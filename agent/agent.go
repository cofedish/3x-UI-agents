// Package agent provides the agent mode entry point for 3x-ui.
package agent

import (
	"fmt"

	"github.com/cofedish/3x-UI-agents/agent/api"
	"github.com/cofedish/3x-UI-agents/agent/config"
	xrayConfig "github.com/cofedish/3x-UI-agents/config"
	"github.com/cofedish/3x-UI-agents/database"
	"github.com/cofedish/3x-UI-agents/logger"
)

// Run starts the agent in server mode.
func Run() error {
	logger.Info("=== Starting 3x-ui Agent ===")

	// Load configuration
	cfg, err := config.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	logger.Info(fmt.Sprintf("Agent ID: %s", cfg.ServerID))
	logger.Info(fmt.Sprintf("Listen Address: %s", cfg.ListenAddr))
	logger.Info(fmt.Sprintf("Auth Type: %s", cfg.AuthType))

	// Initialize database (agent needs local DB for inbounds/clients)
	dbPath := xrayConfig.GetDBPath()
	logger.Info(fmt.Sprintf("Initializing database: %s", dbPath))

	if err := database.InitDB(dbPath); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	// Setup router
	router := api.SetupRouter(cfg)

	// Start server
	logger.Info("Starting agent API server...")
	if err := api.StartServer(cfg, router); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	return nil
}
