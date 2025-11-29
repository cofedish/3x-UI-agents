// Package service provides ServerManagementService for CRUD operations on servers.
package service

import (
	"fmt"
	"time"

	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
)

// ServerManagementService manages the list of servers (local and remote).
type ServerManagementService struct{}

// GetAllServers returns all servers.
func (s *ServerManagementService) GetAllServers() ([]*model.Server, error) {
	db := database.GetDB()
	var servers []*model.Server

	err := db.Order("id").Find(&servers).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get servers: %w", err)
	}

	return servers, nil
}

// GetEnabledServers returns only enabled servers.
func (s *ServerManagementService) GetEnabledServers() ([]*model.Server, error) {
	db := database.GetDB()
	var servers []*model.Server

	err := db.Where("enabled = ?", true).Order("id").Find(&servers).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get enabled servers: %w", err)
	}

	return servers, nil
}

// GetServer returns a server by ID.
func (s *ServerManagementService) GetServer(id int) (*model.Server, error) {
	db := database.GetDB()
	var server model.Server

	err := db.First(&server, id).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	return &server, nil
}

// GetLocalServer returns the default local server (ID=1).
func (s *ServerManagementService) GetLocalServer() (*model.Server, error) {
	return s.GetServer(1)
}

// AddServer creates a new server.
func (s *ServerManagementService) AddServer(server *model.Server) error {
	db := database.GetDB()

	// Set timestamps
	now := time.Now().Unix()
	server.CreatedAt = now
	server.UpdatedAt = now

	// Set default status
	if server.Status == "" {
		server.Status = "pending"
	}

	err := db.Create(server).Error
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}

	return nil
}

// UpdateServer updates an existing server.
func (s *ServerManagementService) UpdateServer(server *model.Server) error {
	db := database.GetDB()

	// Update timestamp
	server.UpdatedAt = time.Now().Unix()

	err := db.Save(server).Error
	if err != nil {
		return fmt.Errorf("failed to update server: %w", err)
	}

	return nil
}

// DeleteServer deletes a server by ID.
// Note: Cannot delete local server (ID=1).
func (s *ServerManagementService) DeleteServer(id int) error {
	if id == 1 {
		return fmt.Errorf("cannot delete local server")
	}

	db := database.GetDB()

	// Check if server has associated data
	var inboundCount int64
	db.Model(&model.Inbound{}).Where("server_id = ?", id).Count(&inboundCount)

	if inboundCount > 0 {
		return fmt.Errorf("cannot delete server with existing inbounds")
	}

	err := db.Delete(&model.Server{}, id).Error
	if err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	return nil
}

// UpdateServerStatus updates server status and last seen timestamp.
func (s *ServerManagementService) UpdateServerStatus(id int, status string, lastError string) error {
	db := database.GetDB()

	updates := map[string]interface{}{
		"status":     status,
		"last_seen":  time.Now().Unix(),
		"updated_at": time.Now().Unix(),
	}

	if lastError != "" {
		updates["last_error"] = lastError
	} else {
		updates["last_error"] = ""
	}

	err := db.Model(&model.Server{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return fmt.Errorf("failed to update server status: %w", err)
	}

	return nil
}

// UpdateServerMetadata updates server version and OS info.
func (s *ServerManagementService) UpdateServerMetadata(id int, version, xrayVersion, osInfo string) error {
	db := database.GetDB()

	updates := map[string]interface{}{
		"version":      version,
		"xray_version": xrayVersion,
		"os_info":      osInfo,
		"updated_at":   time.Now().Unix(),
	}

	err := db.Model(&model.Server{}).Where("id = ?", id).Updates(updates).Error
	if err != nil {
		return fmt.Errorf("failed to update server metadata: %w", err)
	}

	return nil
}

// IsSingleServerMode checks if only one server exists (backward compatibility mode).
func (s *ServerManagementService) IsSingleServerMode() (bool, error) {
	db := database.GetDB()
	var count int64

	err := db.Model(&model.Server{}).Where("enabled = ?", true).Count(&count).Error
	if err != nil {
		return false, err
	}

	return count == 1, nil
}

// GetConnector returns the appropriate ServerConnector for a given server ID.
func (s *ServerManagementService) GetConnector(serverId int) (ServerConnector, error) {
	server, err := s.GetServer(serverId)
	if err != nil {
		return nil, err
	}

	// Check auth type
	if server.AuthType == "local" {
		// Local connector
		return NewLocalConnector(serverId), nil
	}

	// Remote connector (to be implemented)
	return nil, fmt.Errorf("remote connectors not yet implemented")
}

// GetDefaultServerId returns the server ID to use when none is specified.
// In single-server mode, always returns 1.
// In multi-server mode, returns the first enabled server.
func (s *ServerManagementService) GetDefaultServerId() (int, error) {
	isSingle, err := s.IsSingleServerMode()
	if err != nil {
		return 0, err
	}

	if isSingle {
		return 1, nil
	}

	// Return first enabled server
	servers, err := s.GetEnabledServers()
	if err != nil {
		return 0, err
	}

	if len(servers) == 0 {
		return 0, fmt.Errorf("no enabled servers found")
	}

	return servers[0].Id, nil
}
