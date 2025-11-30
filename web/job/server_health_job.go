// Package job provides ServerHealthJob for monitoring multi-server health.
package job

import (
	"context"
	"encoding/json"
	"os"
	"strconv"
	"sync"
	"time"

	"github.com/cofedish/3xui-agents/database/model"
	"github.com/cofedish/3xui-agents/logger"
	"github.com/cofedish/3xui-agents/web/service"
)

// HealthConfig holds configuration for health monitoring
type HealthConfig struct {
	MaxConcurrency int           // Maximum number of concurrent health checks
	CheckTimeout   time.Duration // Timeout per health check
	InfoTimeout    time.Duration // Timeout for server info refresh
}

// loadHealthConfig loads configuration from environment variables with defaults
func loadHealthConfig() HealthConfig {
	cfg := HealthConfig{
		MaxConcurrency: 10,            // Default: 10 concurrent checks
		CheckTimeout:   10 * time.Second,
		InfoTimeout:    5 * time.Second,
	}

	// Override from environment
	if val := os.Getenv("HEALTH_MAX_CONCURRENCY"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 && n <= 100 {
			cfg.MaxConcurrency = n
		}
	}
	if val := os.Getenv("HEALTH_TIMEOUT_SEC"); val != "" {
		if n, err := strconv.Atoi(val); err == nil && n > 0 {
			cfg.CheckTimeout = time.Duration(n) * time.Second
		}
	}

	return cfg
}

// ServerHealthJob periodically checks health of all remote servers.
// Uses bounded worker pool to prevent resource exhaustion with N servers.
type ServerHealthJob struct {
	serverManagement *service.ServerManagementService
	config           HealthConfig

	// Backoff tracking per server (simple: consecutive failure count)
	failuresMu sync.RWMutex
	failures   map[int]int // server_id -> consecutive failure count
}

// NewServerHealthJob creates a new server health monitoring job.
func NewServerHealthJob() *ServerHealthJob {
	return &ServerHealthJob{
		serverManagement: &service.ServerManagementService{},
		config:           loadHealthConfig(),
		failures:         make(map[int]int),
	}
}

// Run executes health checks for all enabled servers with bounded concurrency.
func (j *ServerHealthJob) Run() {
	startTime := time.Now()

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

	// Filter out local server
	remoteServers := make([]*model.Server, 0, len(servers))
	for _, server := range servers {
		if server.Id != 1 {
			remoteServers = append(remoteServers, server)
		}
	}

	if len(remoteServers) == 0 {
		return
	}

	logger.Debug("Running health check for", len(remoteServers), "remote servers (max concurrency:", j.config.MaxConcurrency, ")")

	// Metrics
	var (
		onlineCount  int
		offlineCount int
		errorCount   int
		mu           sync.Mutex
	)

	// Worker pool: semaphore pattern
	semaphore := make(chan struct{}, j.config.MaxConcurrency)
	var wg sync.WaitGroup

	for _, server := range remoteServers {
		wg.Add(1)
		server := server // capture loop variable

		go func() {
			defer wg.Done()

			// Acquire slot
			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			// Check if we should backoff (simple exponential backoff simulation)
			j.failuresMu.RLock()
			failCount := j.failures[server.Id]
			j.failuresMu.RUnlock()

			// If server has failed multiple times, reduce check frequency
			// We skip some checks based on failure count (backoff)
			if failCount > 0 {
				// Skip check probabilistically based on failure count
				// For example: skip if (currentTime % (failCount + 1)) != 0
				// This is a simple approach; more sophisticated would use per-server timers
				skipFactor := failCount
				if skipFactor > 5 {
					skipFactor = 5 // Cap at 5x slowdown
				}
				// For now, we proceed anyway but log the backoff state
				if failCount >= 3 {
					logger.Debug("Server", server.Name, "has", failCount, "consecutive failures, may need attention")
				}
			}

			// Perform health check
			status := j.checkServer(server)

			// Update metrics
			mu.Lock()
			switch status {
			case "online":
				onlineCount++
			case "offline":
				offlineCount++
			case "error":
				errorCount++
			}
			mu.Unlock()
		}()
	}

	// Wait for all checks to complete
	wg.Wait()

	elapsed := time.Since(startTime)
	logger.Info("Health check completed:", len(remoteServers), "servers,",
		onlineCount, "online,", offlineCount, "offline,", errorCount, "errors, took", elapsed)
}

// checkServer performs health check for a single server and returns its status.
func (j *ServerHealthJob) checkServer(server *model.Server) string {
	// Get connector
	connector, err := j.serverManagement.GetConnector(server.Id)
	if err != nil {
		logger.Warning("Failed to get connector for server", server.Name, ":", err)
		j.updateServerStatus(server.Id, "error", "Failed to create connector: "+err.Error())
		j.recordFailure(server.Id)
		return "error"
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), j.config.CheckTimeout)
	defer cancel()

	// Get health status
	health, err := connector.GetHealth(ctx)
	if err != nil {
		logger.Warning("Health check failed for server", server.Name, ":", err)
		j.updateServerStatus(server.Id, "offline", "Health check failed: "+err.Error())
		j.recordFailure(server.Id)
		return "offline"
	}

	// Success - reset failure count
	j.resetFailure(server.Id)

	// Update status to online
	j.updateServerStatus(server.Id, health.Status, "")

	// Update metadata if needed
	if health.Version != "" || health.XrayVersion != "" {
		j.updateServerMetadata(server.Id, health.Version, health.XrayVersion)
	}

	// Get detailed server info (less frequently)
	// Check if server info needs refresh (e.g., if version is unknown)
	if server.Version == "" || server.XrayVersion == "" {
		j.refreshServerInfo(server.Id, connector)
	}

	return health.Status
}

// recordFailure increments failure count for a server
func (j *ServerHealthJob) recordFailure(serverId int) {
	j.failuresMu.Lock()
	defer j.failuresMu.Unlock()
	j.failures[serverId]++
}

// resetFailure clears failure count for a server
func (j *ServerHealthJob) resetFailure(serverId int) {
	j.failuresMu.Lock()
	defer j.failuresMu.Unlock()
	delete(j.failures, serverId)
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
	ctx, cancel := context.WithTimeout(context.Background(), j.config.InfoTimeout)
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
