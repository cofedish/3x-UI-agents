package database

import (
	"github.com/cofedish/3x-UI-agents/logger"
)

// MigrateJWTEndpoints converts HTTPS endpoints to HTTP for JWT auth servers
// This is needed because JWT mode now uses plain HTTP instead of HTTPS
func MigrateJWTEndpoints() error {
	db := GetDB()
	if db == nil {
		return nil
	}

	// Find all JWT servers with https:// endpoints
	result := db.Exec(`
		UPDATE servers
		SET endpoint = REPLACE(endpoint, 'https://', 'http://')
		WHERE auth_type = 'jwt'
		AND endpoint LIKE 'https://%'
	`)

	if result.Error != nil {
		logger.Error("Failed to migrate JWT endpoints:", result.Error)
		return result.Error
	}

	if result.RowsAffected > 0 {
		logger.Info("Migrated", result.RowsAffected, "JWT server endpoints from https:// to http://")
	}

	return nil
}
