// Package api provides HTTP handlers for the agent API.
package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/cofedish/3xui-agents/config"
	"github.com/cofedish/3xui-agents/database"
	"github.com/cofedish/3xui-agents/database/model"
	"github.com/cofedish/3xui-agents/logger"
	"github.com/cofedish/3xui-agents/web/service"
	"github.com/cofedish/3xui-agents/xray"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// AgentHandlers contains all agent API handlers.
type AgentHandlers struct {
	inboundService *service.InboundService
	xrayService    *service.XrayService
	serverService  *service.ServerService
}

// NewAgentHandlers creates a new AgentHandlers instance.
func NewAgentHandlers() *AgentHandlers {
	return &AgentHandlers{
		inboundService: &service.InboundService{},
		xrayService:    &service.XrayService{},
		serverService:  &service.ServerService{},
	}
}

// StandardResponse is the standard API response format.
type StandardResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   *ErrorInfo  `json:"error,omitempty"`
	TraceID string      `json:"trace_id,omitempty"`
}

// ErrorInfo contains error details.
type ErrorInfo struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// respondSuccess sends a successful response.
func respondSuccess(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, StandardResponse{
		Success: true,
		Data:    data,
		TraceID: c.GetString("trace_id"),
	})
}

// respondError sends an error response.
func respondError(c *gin.Context, code string, message string, statusCode int) {
	c.JSON(statusCode, StandardResponse{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
		TraceID: c.GetString("trace_id"),
	})
}

// Health returns the agent health status.
// GET /api/v1/health
func (h *AgentHandlers) Health(c *gin.Context) {
	isRunning := h.xrayService.IsXrayRunning()

	xrayVersion := h.xrayService.GetXrayVersion()

	status := "online"
	if !isRunning {
		status = "degraded"
	}

	respondSuccess(c, gin.H{
		"status":       status,
		"xray_running": isRunning,
		"version":      config.GetVersion(),
		"xray_version": xrayVersion,
		"timestamp":    time.Now().Unix(),
	})
}

// Info returns detailed server information.
// GET /api/v1/info
func (h *AgentHandlers) Info(c *gin.Context) {
	xrayVersion := h.xrayService.GetXrayVersion()

	hostInfo, err := host.Info()
	uptime := int64(0)
	kernel := "unknown"
	if err == nil {
		uptime = int64(hostInfo.Uptime)
		kernel = hostInfo.KernelVersion
	}

	respondSuccess(c, gin.H{
		"version":      config.GetVersion(),
		"xray_version": xrayVersion,
		"os":           runtime.GOOS,
		"arch":         runtime.GOARCH,
		"kernel":       kernel,
		"uptime":       uptime,
	})
}

// ListInbounds returns all inbounds.
// GET /api/v1/inbounds
func (h *AgentHandlers) ListInbounds(c *gin.Context) {
	db := database.GetDB()
	var inbounds []*model.Inbound

	// Get all inbounds (agent manages local server only)
	err := db.Preload("ClientStats").Find(&inbounds).Error
	if err != nil {
		logger.Error("Failed to list inbounds:", err)
		respondError(c, "DB_ERROR", "Failed to list inbounds", http.StatusInternalServerError)
		return
	}

	respondSuccess(c, inbounds)
}

// GetInbound returns a specific inbound.
// GET /api/v1/inbounds/:id
func (h *AgentHandlers) GetInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respondError(c, "INVALID_ID", "Invalid inbound ID", http.StatusBadRequest)
		return
	}

	db := database.GetDB()
	var inbound model.Inbound

	err = db.Where("id = ?", id).Preload("ClientStats").First(&inbound).Error
	if err != nil {
		if database.IsNotFound(err) {
			respondError(c, "NOT_FOUND", "Inbound not found", http.StatusNotFound)
		} else {
			logger.Error("Failed to get inbound:", err)
			respondError(c, "DB_ERROR", "Failed to get inbound", http.StatusInternalServerError)
		}
		return
	}

	respondSuccess(c, inbound)
}

// AddInbound creates a new inbound.
// POST /api/v1/inbounds
func (h *AgentHandlers) AddInbound(c *gin.Context) {
	var inbound model.Inbound

	if err := c.ShouldBindJSON(&inbound); err != nil {
		respondError(c, "INVALID_INPUT", "Invalid inbound data: "+err.Error(), http.StatusBadRequest)
		return
	}

	_, _, err := h.inboundService.AddInbound(&inbound)
	if err != nil {
		logger.Error("Failed to add inbound:", err)
		respondError(c, "OPERATION_FAILED", "Failed to add inbound: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"id": inbound.Id})
}

// UpdateInbound updates an existing inbound.
// PUT /api/v1/inbounds/:id
func (h *AgentHandlers) UpdateInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respondError(c, "INVALID_ID", "Invalid inbound ID", http.StatusBadRequest)
		return
	}

	var inbound model.Inbound
	if err := c.ShouldBindJSON(&inbound); err != nil {
		respondError(c, "INVALID_INPUT", "Invalid inbound data: "+err.Error(), http.StatusBadRequest)
		return
	}

	inbound.Id = id

	_, _, err = h.inboundService.UpdateInbound(&inbound)
	if err != nil {
		logger.Error("Failed to update inbound:", err)
		respondError(c, "OPERATION_FAILED", "Failed to update inbound: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// DeleteInbound deletes an inbound.
// DELETE /api/v1/inbounds/:id
func (h *AgentHandlers) DeleteInbound(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respondError(c, "INVALID_ID", "Invalid inbound ID", http.StatusBadRequest)
		return
	}

	_, err = h.inboundService.DelInbound(id)
	if err != nil {
		logger.Error("Failed to delete inbound:", err)
		respondError(c, "OPERATION_FAILED", "Failed to delete inbound: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// AddClient adds a client to an inbound.
// POST /api/v1/inbounds/:id/clients
func (h *AgentHandlers) AddClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respondError(c, "INVALID_ID", "Invalid inbound ID", http.StatusBadRequest)
		return
	}

	var inbound model.Inbound
	if err := c.ShouldBindJSON(&inbound); err != nil {
		respondError(c, "INVALID_INPUT", "Invalid client data: "+err.Error(), http.StatusBadRequest)
		return
	}

	inbound.Id = id

	_, err = h.inboundService.AddInboundClient(&inbound)
	if err != nil {
		logger.Error("Failed to add client:", err)
		respondError(c, "OPERATION_FAILED", "Failed to add client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// DeleteClient deletes a client from an inbound.
// DELETE /api/v1/inbounds/:id/clients/:email
func (h *AgentHandlers) DeleteClient(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		respondError(c, "INVALID_ID", "Invalid inbound ID", http.StatusBadRequest)
		return
	}

	email := c.Param("email")
	if email == "" {
		respondError(c, "INVALID_EMAIL", "Email is required", http.StatusBadRequest)
		return
	}

	_, err = h.inboundService.DelInboundClient(id, email)
	if err != nil {
		logger.Error("Failed to delete client:", err)
		respondError(c, "OPERATION_FAILED", "Failed to delete client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// GetTraffic returns traffic statistics.
// GET /api/v1/traffic
func (h *AgentHandlers) GetTraffic(c *gin.Context) {
	// Use XrayService to get traffic
	traffics, clientTraffics, err := h.xrayService.GetXrayTraffic()
	if err != nil {
		logger.Error("Failed to get traffic:", err)
		respondError(c, "OPERATION_FAILED", "Failed to get traffic: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{
		"traffics":       traffics,
		"clientTraffics": clientTraffics,
	})
}

// GetClientTraffics returns client traffic statistics.
// GET /api/v1/traffic/clients
func (h *AgentHandlers) GetClientTraffics(c *gin.Context) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic

	err := db.Find(&traffics).Error
	if err != nil {
		logger.Error("Failed to get client traffics:", err)
		respondError(c, "DB_ERROR", "Failed to get client traffics", http.StatusInternalServerError)
		return
	}

	respondSuccess(c, traffics)
}

// GetOnlineClients returns list of online clients.
// GET /api/v1/clients/online
func (h *AgentHandlers) GetOnlineClients(c *gin.Context) {
	// GetOnlineClients returns []string directly
	emails := h.inboundService.GetOnlineClients()
	respondSuccess(c, emails)
}

// StartXray starts the Xray service.
// POST /api/v1/xray/start
func (h *AgentHandlers) StartXray(c *gin.Context) {
	if err := h.xrayService.RestartXray(true); err != nil {
		logger.Error("Failed to start Xray:", err)
		respondError(c, "OPERATION_FAILED", "Failed to start Xray: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// StopXray stops the Xray service.
// POST /api/v1/xray/stop
func (h *AgentHandlers) StopXray(c *gin.Context) {
	if err := h.xrayService.StopXray(); err != nil {
		logger.Error("Failed to stop Xray:", err)
		respondError(c, "OPERATION_FAILED", "Failed to stop Xray: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// RestartXray restarts the Xray service.
// POST /api/v1/xray/restart
func (h *AgentHandlers) RestartXray(c *gin.Context) {
	if err := h.xrayService.RestartXray(false); err != nil {
		logger.Error("Failed to restart Xray:", err)
		respondError(c, "OPERATION_FAILED", "Failed to restart Xray: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// GetXrayVersion returns Xray version.
// GET /api/v1/xray/version
func (h *AgentHandlers) GetXrayVersion(c *gin.Context) {
	version := h.xrayService.GetXrayVersion()
	respondSuccess(c, gin.H{"version": version})
}

// GetXrayConfig returns current Xray configuration.
// GET /api/v1/xray/config
func (h *AgentHandlers) GetXrayConfig(c *gin.Context) {
	xrayConfig, err := h.xrayService.GetXrayConfig()
	if err != nil {
		logger.Error("Failed to get Xray config:", err)
		respondError(c, "OPERATION_FAILED", "Failed to get Xray config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Convert to JSON string
	configBytes, err := json.MarshalIndent(xrayConfig, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal Xray config:", err)
		respondError(c, "OPERATION_FAILED", "Failed to marshal Xray config: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"config": string(configBytes)})
}

// GetSystemStats returns system resource statistics.
// GET /api/v1/system/stats
func (h *AgentHandlers) GetSystemStats(c *gin.Context) {
	stats := make(map[string]interface{})

	// CPU
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercents) > 0 {
		stats["cpu_usage"] = cpuPercents[0]
	}

	cpuCounts, err := cpu.Counts(false)
	if err == nil {
		stats["cpu_cores"] = cpuCounts
	}

	// Memory
	vmem, err := mem.VirtualMemory()
	if err == nil {
		stats["mem_total"] = vmem.Total
		stats["mem_used"] = vmem.Used
		stats["mem_usage"] = vmem.UsedPercent
	}

	swap, err := mem.SwapMemory()
	if err == nil {
		stats["swap_total"] = swap.Total
		stats["swap_used"] = swap.Used
		stats["swap_usage"] = swap.UsedPercent
	}

	// Disk
	diskStat, err := disk.Usage("/")
	if err == nil {
		stats["disk_total"] = diskStat.Total
		stats["disk_used"] = diskStat.Used
		stats["disk_usage"] = diskStat.UsedPercent
	}

	// System uptime
	hostInfo, err := host.Info()
	if err == nil {
		stats["uptime"] = hostInfo.Uptime
	}

	// Public IPs - TODO: implement GetPublicIP in ServerService
	stats["public_ipv4"] = ""
	stats["public_ipv6"] = ""

	respondSuccess(c, stats)
}

// GetLogs returns recent log entries.
// GET /api/v1/logs
func (h *AgentHandlers) GetLogs(c *gin.Context) {
	count := 100
	if countStr := c.Query("count"); countStr != "" {
		if parsedCount, err := strconv.Atoi(countStr); err == nil {
			count = parsedCount
		}
	}

	// Limit count to prevent abuse (max 1000 lines)
	if count > 1000 {
		count = 1000
	}
	if count < 1 {
		count = 10
	}

	// Read logs from configured log file with security restrictions
	logs, err := h.readLogFile(count)
	if err != nil {
		logger.Warning("Failed to read log file:", err)
		// Don't expose internal errors to API clients
		respondError(c, "LOG_READ_ERROR", "Unable to read logs", http.StatusInternalServerError)
		return
	}

	respondSuccess(c, logs)
}

// readLogFile securely reads the last N lines from the agent log file.
// Only reads from the configured log file path to prevent path traversal attacks.
func (h *AgentHandlers) readLogFile(count int) ([]string, error) {
	logFile := os.Getenv("AGENT_LOG_FILE")
	if logFile == "" {
		logFile = "/var/log/x-ui-agent/agent.log"
	}

	// Security: Use filepath.Clean to prevent path traversal
	logFile = filepath.Clean(logFile)

	// Security: Verify the log file path matches expected pattern
	// This prevents reading arbitrary files even if AGENT_LOG_FILE is manipulated
	allowedPaths := []string{
		"/var/log/x-ui-agent/",
		"/var/log/3x-ui-agent/",
		"/tmp/x-ui-agent/",
		"./logs/",
	}

	allowed := false
	for _, prefix := range allowedPaths {
		cleanPrefix := filepath.Clean(prefix)
		if strings.HasPrefix(logFile, cleanPrefix) {
			allowed = true
			break
		}
	}

	if !allowed {
		return nil, fmt.Errorf("log file path not in allowlist: %s", logFile)
	}

	// Check if file exists
	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		// If log file doesn't exist, return empty logs (not an error)
		return []string{"Log file not found. Logs may be directed to stdout."}, nil
	}

	// Use tail command for efficient reading of last N lines
	cmd := exec.Command("tail", "-n", strconv.Itoa(count), logFile)
	output, err := cmd.Output()
	if err != nil {
		// Fallback: try reading file directly if tail fails
		return h.readLogFileDirect(logFile, count)
	}

	// Split lines and reverse (most recent first)
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	// Reverse array to show most recent first
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return lines, nil
}

// readLogFileDirect reads log file directly when tail command is unavailable.
// Fallback implementation for Windows or systems without tail.
func (h *AgentHandlers) readLogFileDirect(logFile string, count int) ([]string, error) {
	data, err := os.ReadFile(logFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Split into lines
	allLines := strings.Split(string(data), "\n")

	// Get last N lines
	start := len(allLines) - count
	if start < 0 {
		start = 0
	}

	lines := allLines[start:]

	// Reverse to show most recent first
	for i, j := 0, len(lines)-1; i < j; i, j = i+1, j-1 {
		lines[i], lines[j] = lines[j], lines[i]
	}

	return lines, nil
}

// UpdateGeoFiles triggers geo file update.
// POST /api/v1/geofiles/update
func (h *AgentHandlers) UpdateGeoFiles(c *gin.Context) {
	// ServerService has UpdateGeofile (singular) method
	// Update both geoip and geosite files
	if err := h.serverService.UpdateGeofile("geoip.dat"); err != nil {
		logger.Error("Failed to update geoip:", err)
		respondError(c, "OPERATION_FAILED", "Failed to update geoip: "+err.Error(), http.StatusInternalServerError)
		return
	}
	if err := h.serverService.UpdateGeofile("geosite.dat"); err != nil {
		logger.Error("Failed to update geosite:", err)
		respondError(c, "OPERATION_FAILED", "Failed to update geosite: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}
