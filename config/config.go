// Package config provides configuration management utilities for the 3x-ui panel,
// including version information, logging levels, database paths, and environment variable handling.
package config

import (
	_ "embed"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// version is set via ldflags during build: -X 'github.com/cofedish/3x-UI-agents/config.version=vX.Y.Z'
// If not set via ldflags, it will use the embedded version from file
var version string

//go:embed name
var name string

//go:embed version
var embeddedVersion string

// LogLevel represents the logging level for the application.
type LogLevel string

// Logging level constants
const (
	Debug   LogLevel = "debug"
	Info    LogLevel = "info"
	Notice  LogLevel = "notice"
	Warning LogLevel = "warning"
	Error   LogLevel = "error"
)

// GetVersion returns the version string of the 3x-ui application.
// Returns version set via ldflags, or falls back to embedded version file.
func GetVersion() string {
	if version != "" {
		return strings.TrimSpace(version)
	}
	return strings.TrimSpace(embeddedVersion)
}

// GetName returns the name of the 3x-ui application.
func GetName() string {
	return strings.TrimSpace(name)
}

// GetLogLevel returns the current logging level based on environment variables or defaults to Info.
func GetLogLevel() LogLevel {
	if IsDebug() {
		return Debug
	}
	logLevel := os.Getenv("XUI_LOG_LEVEL")
	if logLevel == "" {
		return Info
	}
	return LogLevel(logLevel)
}

// IsDebug returns true if debug mode is enabled via the XUI_DEBUG environment variable.
func IsDebug() bool {
	return os.Getenv("XUI_DEBUG") == "true"
}

// GetBinFolderPath returns the path to the binary folder, defaulting to "bin" if not set via XUI_BIN_FOLDER.
func GetBinFolderPath() string {
	binFolderPath := os.Getenv("XUI_BIN_FOLDER")
	if binFolderPath == "" {
		binFolderPath = "bin"
	}
	return binFolderPath
}

func getBaseDir() string {
	exePath, err := os.Executable()
	if err != nil {
		return "."
	}
	exeDir := filepath.Dir(exePath)
	exeDirLower := strings.ToLower(filepath.ToSlash(exeDir))
	if strings.Contains(exeDirLower, "/appdata/local/temp/") || strings.Contains(exeDirLower, "/go-build") {
		wd, err := os.Getwd()
		if err != nil {
			return "."
		}
		return wd
	}
	return exeDir
}

// GetDBFolderPath returns the path to the database folder based on environment variables or platform defaults.
func GetDBFolderPath() string {
	dbFolderPath := os.Getenv("XUI_DB_FOLDER")
	if dbFolderPath != "" {
		return dbFolderPath
	}
	if runtime.GOOS == "windows" {
		return getBaseDir()
	}
	return "/etc/x-ui"
}

// GetDBPath returns the full path to the database file.
func GetDBPath() string {
	return fmt.Sprintf("%s/%s.db", GetDBFolderPath(), GetName())
}

// GetLogFolder returns the path to the log folder based on environment variables or platform defaults.
func GetLogFolder() string {
	logFolderPath := os.Getenv("XUI_LOG_FOLDER")
	if logFolderPath != "" {
		return logFolderPath
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(".", "log")
	}
	return "/var/log"
}

// GetConfigFolderPath returns the path to the config folder for runtime-generated configs.
// This is separate from BIN folder to allow BIN to be read-only (e.g., under systemd ProtectSystem).
// Defaults to /etc/x-ui-agent on Linux, or current directory on Windows.
// Can be overridden via XUI_CONFIG_FOLDER environment variable.
func GetConfigFolderPath() string {
	configFolderPath := os.Getenv("XUI_CONFIG_FOLDER")
	if configFolderPath != "" {
		return configFolderPath
	}
	if runtime.GOOS == "windows" {
		return getBaseDir()
	}
	// Use /etc/x-ui-agent for agent, fall back to /etc/x-ui for panel
	// Check if running as agent by looking at executable name
	exePath, _ := os.Executable()
	if strings.Contains(strings.ToLower(exePath), "agent") {
		return "/etc/x-ui-agent"
	}
	return "/etc/x-ui"
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Sync()
}

func init() {
	if runtime.GOOS != "windows" {
		return
	}
	if os.Getenv("XUI_DB_FOLDER") != "" {
		return
	}
	oldDBFolder := "/etc/x-ui"
	oldDBPath := fmt.Sprintf("%s/%s.db", oldDBFolder, GetName())
	newDBFolder := GetDBFolderPath()
	newDBPath := fmt.Sprintf("%s/%s.db", newDBFolder, GetName())
	_, err := os.Stat(newDBPath)
	if err == nil {
		return // new exists
	}
	_, err = os.Stat(oldDBPath)
	if os.IsNotExist(err) {
		return // old does not exist
	}
	_ = copyFile(oldDBPath, newDBPath) // ignore error
}
