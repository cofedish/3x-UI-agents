// Package controller provides HTTP handlers for server management.
package controller

import (
	"encoding/json"
	"strconv"

	"github.com/cofedish/3x-UI-agents/database/model"
	"github.com/cofedish/3x-UI-agents/logger"
	"github.com/cofedish/3x-UI-agents/web/service"
	"github.com/gin-gonic/gin"
)

// ServerManagementController handles server CRUD operations.
type ServerManagementController struct {
	serverMgmt *service.ServerManagementService
}

// NewServerManagementController creates a new controller instance.
func NewServerManagementController() *ServerManagementController {
	return &ServerManagementController{
		serverMgmt: &service.ServerManagementService{},
	}
}

// ListServers returns paginated list of servers with filters.
// GET /panel/api/servers
// Query params: page, limit, status, search, tags
func (c *ServerManagementController) ListServers(ctx *gin.Context) {
	// Parse pagination
	page, _ := strconv.Atoi(ctx.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(ctx.DefaultQuery("limit", "20"))

	// Validate limits to prevent abuse
	if limit > 100 {
		limit = 100
	}
	if page < 1 {
		page = 1
	}

	// Get filters
	status := ctx.Query("status")   // online, offline, error, pending
	search := ctx.Query("search")   // search by name or endpoint
	tagsFilter := ctx.Query("tags") // comma-separated tags

	// Get all servers for filtering
	servers, err := c.serverMgmt.GetAllServers()
	if err != nil {
		logger.Error("Failed to get servers:", err)
		jsonMsg(ctx, "Failed to get servers", err)
		return
	}

	// Apply filters (simplified implementation)
	filtered := make([]*model.Server, 0)
	for _, server := range servers {
		// Status filter
		if status != "" && server.Status != status {
			continue
		}

		// Search filter (name or endpoint contains search term)
		if search != "" {
			if !contains(server.Name, search) && !contains(server.Endpoint, search) {
				continue
			}
		}

		// Tags filter (at least one tag matches)
		if tagsFilter != "" {
			// Parse tags from JSON
			var serverTags []string
			json.Unmarshal([]byte(server.Tags), &serverTags)

			matched := false
			for _, tag := range serverTags {
				if contains(tag, tagsFilter) {
					matched = true
					break
				}
			}

			if !matched {
				continue
			}
		}

		filtered = append(filtered, server)
	}

	// Apply pagination
	start := (page - 1) * limit
	end := start + limit

	if start > len(filtered) {
		start = len(filtered)
	}
	if end > len(filtered) {
		end = len(filtered)
	}

	paginated := filtered[start:end]

	jsonObj(ctx, gin.H{
		"servers": paginated,
		"total":   len(filtered),
		"page":    page,
		"limit":   limit,
	}, nil)
}

// GetServer returns a specific server by ID.
// GET /panel/api/servers/:id
func (c *ServerManagementController) GetServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		jsonMsg(ctx, "Invalid server ID", err)
		return
	}

	server, err := c.serverMgmt.GetServer(id)
	if err != nil {
		logger.Error("Failed to get server:", err)
		jsonMsg(ctx, "Server not found", err)
		return
	}

	jsonObj(ctx, server, nil)
}

// AddServer creates a new server.
// POST /panel/api/servers
func (c *ServerManagementController) AddServer(ctx *gin.Context) {
	var server model.Server

	if err := ctx.ShouldBindJSON(&server); err != nil {
		jsonMsg(ctx, "Invalid server data", err)
		return
	}

	// Validate required fields
	if server.Name == "" {
		jsonMsg(ctx, "Server name is required", nil)
		return
	}

	if server.Endpoint == "" {
		jsonMsg(ctx, "Server endpoint is required", nil)
		return
	}

	if server.AuthType != "mtls" && server.AuthType != "jwt" && server.AuthType != "local" {
		jsonMsg(ctx, "Invalid auth type (must be: mtls, jwt, or local)", nil)
		return
	}

	// Set initial status
	if server.Status == "" {
		server.Status = "pending"
	}

	if err := c.serverMgmt.AddServer(&server); err != nil {
		logger.Error("Failed to add server:", err)
		jsonMsg(ctx, "Failed to add server", err)
		return
	}

	jsonObj(ctx, gin.H{"id": server.Id, "message": "Server added successfully"}, nil)
}

// UpdateServer updates an existing server.
// PUT /panel/api/servers/:id
func (c *ServerManagementController) UpdateServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		jsonMsg(ctx, "Invalid server ID", err)
		return
	}

	var server model.Server
	if err := ctx.ShouldBindJSON(&server); err != nil {
		jsonMsg(ctx, "Invalid server data", err)
		return
	}

	server.Id = id

	if err := c.serverMgmt.UpdateServer(&server); err != nil {
		logger.Error("Failed to update server:", err)
		jsonMsg(ctx, "Failed to update server", err)
		return
	}

	jsonMsg(ctx, "Server updated successfully", nil)
}

// DeleteServer deletes a server.
// DELETE /panel/api/servers/:id
func (c *ServerManagementController) DeleteServer(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		jsonMsg(ctx, "Invalid server ID", err)
		return
	}

	if err := c.serverMgmt.DeleteServer(id); err != nil {
		logger.Error("Failed to delete server:", err)
		jsonMsg(ctx, "Failed to delete server", err)
		return
	}

	jsonMsg(ctx, "Server deleted successfully", nil)
}

// GetServerHealth tests server connectivity and returns health status.
// GET /panel/api/servers/:id/health
func (c *ServerManagementController) GetServerHealth(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		jsonMsg(ctx, "Invalid server ID", err)
		return
	}

	connector, err := c.serverMgmt.GetConnector(id)
	if err != nil {
		logger.Error("Failed to get connector:", err)
		jsonMsg(ctx, "Failed to connect to server", err)
		return
	}

	health, err := connector.GetHealth(ctx.Request.Context())
	if err != nil {
		logger.Warning("Server health check failed:", err)
		jsonObj(ctx, gin.H{
			"status": "error",
			"error":  err.Error(),
		}, nil)
		return
	}

	jsonObj(ctx, health, nil)
}

// GetServerInfo returns detailed server information.
// GET /panel/api/servers/:id/info
func (c *ServerManagementController) GetServerInfo(ctx *gin.Context) {
	id, err := strconv.Atoi(ctx.Param("id"))
	if err != nil {
		jsonMsg(ctx, "Invalid server ID", err)
		return
	}

	connector, err := c.serverMgmt.GetConnector(id)
	if err != nil {
		logger.Error("Failed to get connector:", err)
		jsonMsg(ctx, "Failed to connect to server", err)
		return
	}

	info, err := connector.GetServerInfo(ctx.Request.Context())
	if err != nil {
		logger.Error("Failed to get server info:", err)
		jsonMsg(ctx, "Failed to get server info", err)
		return
	}

	jsonObj(ctx, info, nil)
}

// GetServerStats returns aggregated statistics.
// GET /panel/api/servers/stats
func (c *ServerManagementController) GetServerStats(ctx *gin.Context) {
	servers, err := c.serverMgmt.GetAllServers()
	if err != nil {
		logger.Error("Failed to get servers:", err)
		jsonMsg(ctx, "Failed to get server stats", err)
		return
	}

	stats := gin.H{
		"total":   len(servers),
		"online":  0,
		"offline": 0,
		"error":   0,
		"pending": 0,
	}

	for _, server := range servers {
		switch server.Status {
		case "online":
			stats["online"] = stats["online"].(int) + 1
		case "offline":
			stats["offline"] = stats["offline"].(int) + 1
		case "error":
			stats["error"] = stats["error"].(int) + 1
		case "pending":
			stats["pending"] = stats["pending"].(int) + 1
		}
	}

	jsonObj(ctx, stats, nil)
}

// Helper function to check if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && caseInsensitiveContains(s, substr)))
}

func caseInsensitiveContains(s, substr string) bool {
	// Simplified case-insensitive search
	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}

	return false
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + ('a' - 'A')
		} else {
			result[i] = c
		}
	}
	return string(result)
}
