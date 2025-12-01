// Package config provides configuration management for the 3x-ui agent.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// AgentConfig holds all configuration for the agent.
type AgentConfig struct {
	// Server settings
	ListenAddr string
	ServerID   string
	ServerName string
	Tags       []string

	// Controller settings
	ControllerEndpoint string

	// Authentication
	AuthType  string // "mtls" or "jwt"
	CertFile  string
	KeyFile   string
	CAFile    string
	JWTSecret string

	// Xray settings
	XrayBinFolder    string
	XrayConfigFolder string

	// Logging
	LogLevel string
	LogFile  string

	// Performance
	MaxConcurrentRequests int
	RequestTimeout        int // seconds
	RateLimit             int // requests per minute
}

// LoadConfig loads agent configuration from environment variables.
func LoadConfig() (*AgentConfig, error) {
	cfg := &AgentConfig{
		// Defaults
		ListenAddr:            getEnv("AGENT_LISTEN_ADDR", "0.0.0.0:2054"),
		ServerID:              getEnv("AGENT_SERVER_ID", ""),
		ServerName:            getEnv("AGENT_SERVER_NAME", ""),
		Tags:                  parseTags(getEnv("AGENT_TAGS", "")),
		ControllerEndpoint:    getEnv("AGENT_CONTROLLER_ENDPOINT", ""),
		AuthType:              getEnv("AGENT_AUTH_TYPE", "mtls"),
		CertFile:              getEnv("AGENT_CERT_FILE", "/etc/x-ui-agent/certs/agent.crt"),
		KeyFile:               getEnv("AGENT_KEY_FILE", "/etc/x-ui-agent/certs/agent.key"),
		CAFile:                getEnv("AGENT_CA_FILE", "/etc/x-ui-agent/certs/ca.crt"),
		JWTSecret:             getEnv("AGENT_JWT_SECRET", ""),
		XrayBinFolder:         getEnv("XRAY_BIN_FOLDER", "/usr/local/x-ui/bin"),
		XrayConfigFolder:      getEnv("XRAY_CONFIG_FOLDER", "/etc/x-ui"),
		LogLevel:              getEnv("AGENT_LOG_LEVEL", "info"),
		LogFile:               getEnv("AGENT_LOG_FILE", "/var/log/x-ui-agent/agent.log"),
		MaxConcurrentRequests: getEnvInt("AGENT_MAX_CONCURRENT", 50),
		RequestTimeout:        getEnvInt("AGENT_REQUEST_TIMEOUT", 30),
		RateLimit:             getEnvInt("AGENT_RATE_LIMIT", 100),
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks if configuration is valid.
func (c *AgentConfig) Validate() error {
	if c.AuthType != "mtls" && c.AuthType != "jwt" {
		return fmt.Errorf("invalid auth type: %s (must be 'mtls' or 'jwt')", c.AuthType)
	}

	if c.AuthType == "mtls" {
		if c.CertFile == "" || c.KeyFile == "" || c.CAFile == "" {
			return fmt.Errorf("mTLS requires cert_file, key_file, and ca_file")
		}
	}

	if c.AuthType == "jwt" {
		if c.JWTSecret == "" {
			return fmt.Errorf("JWT auth requires jwt_secret")
		}
	}

	if c.ListenAddr == "" {
		return fmt.Errorf("listen_addr is required")
	}

	return nil
}

// getEnv retrieves environment variable or returns default.
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt retrieves environment variable as int or returns default.
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

// parseTags parses comma-separated tags.
func parseTags(tagsStr string) []string {
	if tagsStr == "" {
		return []string{}
	}

	tags := strings.Split(tagsStr, ",")
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag != "" {
			result = append(result, tag)
		}
	}
	return result
}
