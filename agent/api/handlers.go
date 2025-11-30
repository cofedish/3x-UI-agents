// Package api provides HTTP handlers for the agent API.
package api

import (
	"encoding/json"
	"net/http"
	"runtime"
	"strconv"
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

	xrayVersion, _ := h.xrayService.GetXrayVersions()

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
	xrayVersion, _ := h.xrayService.GetXrayVersions()

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

	if err := h.inboundService.AddInbound(&inbound); err != nil {
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

	if err := h.inboundService.UpdateInbound(&inbound); err != nil {
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

	if err := h.inboundService.DelInbound(id); err != nil {
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

	if err := h.inboundService.AddInboundClient(&inbound); err != nil {
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

	if err := h.inboundService.DelInboundClient(id, email); err != nil {
		logger.Error("Failed to delete client:", err)
		respondError(c, "OPERATION_FAILED", "Failed to delete client: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}

// GetTraffic returns traffic statistics.
// GET /api/v1/traffic
func (h *AgentHandlers) GetTraffic(c *gin.Context) {
	reset := c.Query("reset") == "true"

	xrayApi := xray.GetXrayAPI()
	if xrayApi == nil {
		respondError(c, "XRAY_NOT_INITIALIZED", "Xray API not initialized", http.StatusServiceUnavailable)
		return
	}

	traffic, err := xrayApi.GetTraffic(reset)
	if err != nil {
		logger.Error("Failed to get traffic:", err)
		respondError(c, "OPERATION_FAILED", "Failed to get traffic: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, traffic)
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
	onlineClients, err := h.inboundService.GetOnlineClients()
	if err != nil {
		logger.Error("Failed to get online clients:", err)
		respondError(c, "OPERATION_FAILED", "Failed to get online clients: "+err.Error(), http.StatusInternalServerError)
		return
	}

	emails := make([]string, 0, len(onlineClients))
	for email := range onlineClients {
		emails = append(emails, email)
	}

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
	version, err := h.xrayService.GetXrayVersions()
	if err != nil {
		logger.Error("Failed to get Xray version:", err)
		respondError(c, "OPERATION_FAILED", "Failed to get Xray version: "+err.Error(), http.StatusInternalServerError)
		return
	}

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

	// Public IPs
	ipv4, _ := h.serverService.GetPublicIP(true)
	ipv6, _ := h.serverService.GetPublicIP(false)
	stats["public_ipv4"] = ipv4
	stats["public_ipv6"] = ipv6

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

	// Limit count to prevent abuse
	if count > 1000 {
		count = 1000
	}

	// TODO: Implement actual log reading
	logs := []string{
		"Log functionality not fully implemented",
		"This is a placeholder",
	}

	respondSuccess(c, logs)
}

// UpdateGeoFiles triggers geo file update.
// POST /api/v1/geofiles/update
func (h *AgentHandlers) UpdateGeoFiles(c *gin.Context) {
	if err := h.serverService.UpdateGeoFiles(); err != nil {
		logger.Error("Failed to update geo files:", err)
		respondError(c, "OPERATION_FAILED", "Failed to update geo files: "+err.Error(), http.StatusInternalServerError)
		return
	}

	respondSuccess(c, gin.H{"success": true})
}
