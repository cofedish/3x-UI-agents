// Package job provides ServerHealthJob for monitoring multi-server health.
package job

import (
	"context"
	"encoding/json"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/web/service"
)

// ServerHealthJob periodically checks health of all remote servers.
// Updates server status and metadata in the database.
type ServerHealthJob struct {
	serverManagement *service.ServerManagementService
}

// NewServerHealthJob creates a new server health monitoring job.
func NewServerHealthJob() *ServerHealthJob {
	return &ServerHealthJob{
		serverManagement: &service.ServerManagementService{},
	}
}

// Run executes health checks for all enabled servers.
// Runs concurrently for all servers with individual timeouts.
func (j *ServerHealthJob) Run() {
	// Get all enabled servers
	servers, err := j.serverManagement.GetEnabledServers()
	if err != nil {
		logger.Error("Failed to get servers for health check:", err)
		return
	}

	// Skip health check if only local server exists (ID=1)
	if len(servers) == 1 && servers[0].Id == 1 {
		return
	}

	logger.Debug("Running health check for", len(servers), "servers")

	// Check each server concurrently
	for _, server := range servers {
		go j.checkServer(server)
	}
}

// checkServer performs health check for a single server.
func (j *ServerHealthJob) checkServer(server *model.Server) {
	// Skip local server (always online)
	if server.Id == 1 {
		return
	}

	// Get connector
	connector, err := j.serverManagement.GetConnector(server.Id)
	if err != nil {
		logger.Warning("Failed to get connector for server", server.Name, ":", err)
		j.updateServerStatus(server.Id, "error", "Failed to create connector: "+err.Error())
		return
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get health status
	health, err := connector.GetHealth(ctx)
	if err != nil {
		logger.Warning("Health check failed for server", server.Name, ":", err)
		j.updateServerStatus(server.Id, "offline", "Health check failed: "+err.Error())
		return
	}

	// Update status to online
	j.updateServerStatus(server.Id, health.Status, "")

	// Update metadata if needed
	if health.Version != "" || health.XrayVersion != "" {
		j.updateServerMetadata(server.Id, health.Version, health.XrayVersion)
	}

	// Get detailed server info (less frequently)
	// Check if server info needs refresh (e.g., every 10 health checks or if version is unknown)
	if server.Version == "" || server.XrayVersion == "" {
		j.refreshServerInfo(server.Id, connector)
	}
}

// updateServerStatus updates server status in database.
func (j *ServerHealthJob) updateServerStatus(serverId int, status string, lastError string) {
	err := j.serverManagement.UpdateServerStatus(serverId, status, lastError)
	if err != nil {
		logger.Error("Failed to update server status:", err)
	}
}

// updateServerMetadata updates server version information.
func (j *ServerHealthJob) updateServerMetadata(serverId int, version, xrayVersion string) {
	// Get current server to preserve os_info
	server, err := j.serverManagement.GetServer(serverId)
	if err != nil {
		logger.Error("Failed to get server for metadata update:", err)
		return
	}

	err = j.serverManagement.UpdateServerMetadata(serverId, version, xrayVersion, server.OsInfo)
	if err != nil {
		logger.Error("Failed to update server metadata:", err)
	}
}

// refreshServerInfo fetches and updates complete server information.
func (j *ServerHealthJob) refreshServerInfo(serverId int, connector service.ServerConnector) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	info, err := connector.GetServerInfo(ctx)
	if err != nil {
		logger.Warning("Failed to get server info:", err)
		return
	}

	// Build OS info JSON
	osInfo := map[string]string{
		"os":     info.OS,
		"arch":   info.Arch,
		"kernel": info.Kernel,
	}

	osInfoJson, err := json.Marshal(osInfo)
	if err != nil {
		logger.Error("Failed to marshal OS info:", err)
		return
	}

	// Update metadata
	err = j.serverManagement.UpdateServerMetadata(serverId, info.Version, info.XrayVersion, string(osInfoJson))
	if err != nil {
		logger.Error("Failed to update server metadata:", err)
	}
}
