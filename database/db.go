// Package database provides database initialization, migration, and management utilities
// for the 3x-ui panel using GORM with SQLite.
package database

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"log"
	"os"
	"path"
	"slices"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/util/crypto"
	"github.com/mhsanaei/3x-ui/v2/xray"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var db *gorm.DB

const (
	defaultUsername = "admin"
	defaultPassword = "admin"
)

func initModels() error {
	models := []any{
		&model.User{},
		&model.Inbound{},
		&model.OutboundTraffics{},
		&model.Setting{},
		&model.InboundClientIps{},
		&xray.ClientTraffic{},
		&model.HistoryOfSeeders{},
		&model.Server{},
		&model.ServerTask{},
	}
	for _, model := range models {
		if err := db.AutoMigrate(model); err != nil {
			log.Printf("Error auto migrating model: %v", err)
			return err
		}
	}
	return nil
}

// initUser creates a default admin user if the users table is empty.
func initUser() error {
	empty, err := isTableEmpty("users")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}
	if empty {
		hashedPassword, err := crypto.HashPasswordAsBcrypt(defaultPassword)

		if err != nil {
			log.Printf("Error hashing default password: %v", err)
			return err
		}

		user := &model.User{
			Username: defaultUsername,
			Password: hashedPassword,
		}
		return db.Create(user).Error
	}
	return nil
}

// runSeeders migrates user passwords to bcrypt and records seeder execution to prevent re-running.
func runSeeders(isUsersEmpty bool) error {
	empty, err := isTableEmpty("history_of_seeders")
	if err != nil {
		log.Printf("Error checking if users table is empty: %v", err)
		return err
	}

	if empty && isUsersEmpty {
		hashSeeder := &model.HistoryOfSeeders{
			SeederName: "UserPasswordHash",
		}
		return db.Create(hashSeeder).Error
	} else {
		var seedersHistory []string
		db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory)

		if !slices.Contains(seedersHistory, "UserPasswordHash") && !isUsersEmpty {
			var users []model.User
			db.Find(&users)

			for _, user := range users {
				hashedPassword, err := crypto.HashPasswordAsBcrypt(user.Password)
				if err != nil {
					log.Printf("Error hashing password for user '%s': %v", user.Username, err)
					return err
				}
				db.Model(&user).Update("password", hashedPassword)
			}

			hashSeeder := &model.HistoryOfSeeders{
				SeederName: "UserPasswordHash",
			}
			return db.Create(hashSeeder).Error
		}
	}

	return nil
}

// runMultiserverMigration adds multi-server support to the database schema.
// Creates default local server and adds server_id foreign keys to existing tables.
func runMultiserverMigration() error {
	var seedersHistory []string
	db.Model(&model.HistoryOfSeeders{}).Pluck("seeder_name", &seedersHistory)

	if slices.Contains(seedersHistory, "MultiServerMigration") {
		return nil
	}

	log.Println("Running multi-server migration...")

	// 1. Create default local server if servers table is empty
	var serverCount int64
	db.Model(&model.Server{}).Count(&serverCount)

	if serverCount == 0 {
		defaultServer := &model.Server{
			Id:       1,
			Name:     "Default Local Server",
			Endpoint: "local://",
			Region:   "",
			Tags:     "[]",
			AuthType: "local",
			AuthData: "",
			Status:   "online",
			Enabled:  true,
			Notes:    "Auto-created during multi-server migration. This represents the local Xray instance.",
		}

		if err := db.Create(defaultServer).Error; err != nil {
			log.Printf("Error creating default local server: %v", err)
			return err
		}

		log.Println("Created default local server (ID=1)")
	}

	// 2. Add server_id columns to existing tables if they don't exist
	// SQLite doesn't support IF NOT EXISTS for ALTER TABLE, so we check manually
	type ColumnInfo struct {
		Name string
	}

	tablesToMigrate := []struct {
		tableName  string
		columnName string
	}{
		{"inbounds", "server_id"},
		{"client_traffics", "server_id"},
		{"outbound_traffics", "server_id"},
		{"inbound_client_ips", "server_id"},
	}

	for _, table := range tablesToMigrate {
		var columns []ColumnInfo
		db.Raw("PRAGMA table_info(" + table.tableName + ")").Scan(&columns)

		hasColumn := false
		for _, col := range columns {
			if col.Name == table.columnName {
				hasColumn = true
				break
			}
		}

		if !hasColumn {
			// Add column
			alterSQL := "ALTER TABLE " + table.tableName + " ADD COLUMN server_id INTEGER"
			if err := db.Exec(alterSQL).Error; err != nil {
				log.Printf("Error adding server_id to %s: %v", table.tableName, err)
				return err
			}
			log.Printf("Added server_id column to %s", table.tableName)
		}

		// 3. Set default server_id = 1 for existing records
		updateSQL := "UPDATE " + table.tableName + " SET server_id = 1 WHERE server_id IS NULL"
		if err := db.Exec(updateSQL).Error; err != nil {
			log.Printf("Error setting default server_id in %s: %v", table.tableName, err)
			return err
		}
		log.Printf("Set default server_id=1 for existing records in %s", table.tableName)
	}

	// 4. Create indexes
	indexes := []string{
		"CREATE INDEX IF NOT EXISTS idx_inbounds_server ON inbounds(server_id)",
		"CREATE INDEX IF NOT EXISTS idx_client_traffics_server ON client_traffics(server_id)",
		"CREATE INDEX IF NOT EXISTS idx_outbound_traffics_server ON outbound_traffics(server_id)",
		"CREATE INDEX IF NOT EXISTS idx_inbound_client_ips_server ON inbound_client_ips(server_id)",
	}

	for _, indexSQL := range indexes {
		if err := db.Exec(indexSQL).Error; err != nil {
			log.Printf("Error creating index: %v", err)
			return err
		}
	}

	log.Println("Created indexes for server_id columns")

	// 5. Record migration in seeder history
	migrationSeeder := &model.HistoryOfSeeders{
		SeederName: "MultiServerMigration",
	}
	if err := db.Create(migrationSeeder).Error; err != nil {
		log.Printf("Error recording multi-server migration: %v", err)
		return err
	}

	log.Println("Multi-server migration completed successfully")
	return nil
}

// isTableEmpty returns true if the named table contains zero rows.
func isTableEmpty(tableName string) (bool, error) {
	var count int64
	err := db.Table(tableName).Count(&count).Error
	return count == 0, err
}

// InitDB sets up the database connection, migrates models, and runs seeders.
func InitDB(dbPath string) error {
	dir := path.Dir(dbPath)
	err := os.MkdirAll(dir, fs.ModePerm)
	if err != nil {
		return err
	}

	var gormLogger logger.Interface

	if config.IsDebug() {
		gormLogger = logger.Default
	} else {
		gormLogger = logger.Discard
	}

	c := &gorm.Config{
		Logger: gormLogger,
	}
	db, err = gorm.Open(sqlite.Open(dbPath), c)
	if err != nil {
		return err
	}

	if err := initModels(); err != nil {
		return err
	}

	isUsersEmpty, err := isTableEmpty("users")
	if err != nil {
		return err
	}

	if err := initUser(); err != nil {
		return err
	}

	if err := runSeeders(isUsersEmpty); err != nil {
		return err
	}

	return runMultiserverMigration()
}

// CloseDB closes the database connection if it exists.
func CloseDB() error {
	if db != nil {
		sqlDB, err := db.DB()
		if err != nil {
			return err
		}
		return sqlDB.Close()
	}
	return nil
}

// GetDB returns the global GORM database instance.
func GetDB() *gorm.DB {
	return db
}

// IsNotFound checks if the given error is a GORM record not found error.
func IsNotFound(err error) bool {
	return err == gorm.ErrRecordNotFound
}

// IsSQLiteDB checks if the given file is a valid SQLite database by reading its signature.
func IsSQLiteDB(file io.ReaderAt) (bool, error) {
	signature := []byte("SQLite format 3\x00")
	buf := make([]byte, len(signature))
	_, err := file.ReadAt(buf, 0)
	if err != nil {
		return false, err
	}
	return bytes.Equal(buf, signature), nil
}

// Checkpoint performs a WAL checkpoint on the SQLite database to ensure data consistency.
func Checkpoint() error {
	// Update WAL
	err := db.Exec("PRAGMA wal_checkpoint;").Error
	if err != nil {
		return err
	}
	return nil
}

// ValidateSQLiteDB opens the provided sqlite DB path with a throw-away connection
// and runs a PRAGMA integrity_check to ensure the file is structurally sound.
// It does not mutate global state or run migrations.
func ValidateSQLiteDB(dbPath string) error {
	if _, err := os.Stat(dbPath); err != nil { // file must exist
		return err
	}
	gdb, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{Logger: logger.Discard})
	if err != nil {
		return err
	}
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	defer sqlDB.Close()
	var res string
	if err := gdb.Raw("PRAGMA integrity_check;").Scan(&res).Error; err != nil {
		return err
	}
	if res != "ok" {
		return errors.New("sqlite integrity check failed: " + res)
	}
	return nil
}
