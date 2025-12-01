// Package service provides LocalConnector for managing the local Xray server instance.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/cofedish/3x-UI-agents/config"
	"github.com/cofedish/3x-UI-agents/database"
	"github.com/cofedish/3x-UI-agents/database/model"
	"github.com/cofedish/3x-UI-agents/xray"
	"github.com/shirou/gopsutil/v4/cpu"
	"github.com/shirou/gopsutil/v4/disk"
	"github.com/shirou/gopsutil/v4/host"
	"github.com/shirou/gopsutil/v4/mem"
)

// LocalConnector implements ServerConnector for the local Xray instance.
// It delegates operations to existing service layer methods.
type LocalConnector struct {
	serverId       int
	inboundService *InboundService
	xrayService    *XrayService
	serverService  *ServerService
}

// NewLocalConnector creates a new LocalConnector instance.
func NewLocalConnector(serverId int) *LocalConnector {
	return &LocalConnector{
		serverId:       serverId,
		inboundService: &InboundService{},
		xrayService:    &XrayService{},
		serverService:  &ServerService{},
	}
}

// GetServerInfo returns basic server information.
func (c *LocalConnector) GetServerInfo(ctx context.Context) (*ServerInfo, error) {
	xrayVersion := c.xrayService.GetXrayVersion()

	hostInfo, err := host.Info()
	uptime := int64(0)
	kernel := "unknown"
	if err == nil {
		uptime = int64(hostInfo.Uptime)
		kernel = hostInfo.KernelVersion
	}

	return &ServerInfo{
		ServerId:    c.serverId,
		ServerName:  "Default Local Server",
		Version:     config.GetVersion(),
		XrayVersion: xrayVersion,
		OS:          runtime.GOOS,
		Arch:        runtime.GOARCH,
		Kernel:      kernel,
		Uptime:      uptime,
	}, nil
}

// GetHealth returns the current health status.
func (c *LocalConnector) GetHealth(ctx context.Context) (*HealthStatus, error) {
	isRunning := c.xrayService.IsXrayRunning()

	xrayVersion := c.xrayService.GetXrayVersion()

	status := "online"
	if !isRunning {
		status = "offline"
	}

	return &HealthStatus{
		Status:      status,
		XrayRunning: isRunning,
		Version:     config.GetVersion(),
		XrayVersion: xrayVersion,
		Timestamp:   time.Now().Unix(),
	}, nil
}

// ListInbounds retrieves all inbounds for this server.
func (c *LocalConnector) ListInbounds(ctx context.Context) ([]*model.Inbound, error) {
	db := database.GetDB()
	var inbounds []*model.Inbound

	// Get inbounds with server_id = c.serverId
	err := db.Where("server_id = ?", c.serverId).
		Preload("ClientStats").
		Find(&inbounds).Error

	if err != nil {
		return nil, fmt.Errorf("failed to list inbounds: %w", err)
	}

	return inbounds, nil
}

// GetInbound retrieves a specific inbound by ID.
func (c *LocalConnector) GetInbound(ctx context.Context, id int) (*model.Inbound, error) {
	db := database.GetDB()
	var inbound model.Inbound

	err := db.Where("id = ? AND server_id = ?", id, c.serverId).
		Preload("ClientStats").
		First(&inbound).Error

	if err != nil {
		return nil, fmt.Errorf("failed to get inbound: %w", err)
	}

	return &inbound, nil
}

// AddInbound adds a new inbound to the local Xray server.
func (c *LocalConnector) AddInbound(ctx context.Context, inbound *model.Inbound) error {
	// Set server_id
	inbound.ServerId = c.serverId

	// Delegate to inbound service (ignore returned inbound and needRestart)
	_, _, err := c.inboundService.AddInbound(inbound)
	return err
}

// UpdateInbound updates an existing inbound.
func (c *LocalConnector) UpdateInbound(ctx context.Context, inbound *model.Inbound) error {
	// Ensure server_id matches
	inbound.ServerId = c.serverId

	// Delegate to inbound service (ignore needRestart return value)
	_, _, err := c.inboundService.UpdateInbound(inbound)
	return err
}

// DeleteInbound deletes an inbound by ID.
func (c *LocalConnector) DeleteInbound(ctx context.Context, id int) error {
	// Verify ownership
	inbound, err := c.GetInbound(ctx, id)
	if err != nil {
		return err
	}

	// Delegate to inbound service (ignore needRestart return value)
	_, err = c.inboundService.DelInbound(inbound.Id)
	return err
}

// AddClient adds a client to an inbound.
func (c *LocalConnector) AddClient(ctx context.Context, inbound *model.Inbound) error {
	// Ensure server_id
	inbound.ServerId = c.serverId

	// Delegate to inbound service (ignore needRestart return value)
	_, err := c.inboundService.AddInboundClient(inbound)
	return err
}

// UpdateClient updates a client in an inbound.
func (c *LocalConnector) UpdateClient(ctx context.Context, inbound *model.Inbound, clientIndex int) error {
	// Ensure server_id
	inbound.ServerId = c.serverId

	// Get existing inbound to find client ID
	existingInbound, err := c.GetInbound(ctx, inbound.Id)
	if err != nil {
		return err
	}

	// Parse clients to get client ID
	clients, err := c.inboundService.GetClients(existingInbound)
	if err != nil {
		return err
	}

	if clientIndex < 0 || clientIndex >= len(clients) {
		return fmt.Errorf("client index %d out of range", clientIndex)
	}

	// Determine client ID based on protocol
	var clientId string
	switch existingInbound.Protocol {
	case "trojan":
		clientId = clients[clientIndex].Password
	case "shadowsocks":
		clientId = clients[clientIndex].Email
	default:
		clientId = clients[clientIndex].ID
	}

	// Delegate to inbound service (ignore needRestart return value)
	_, err = c.inboundService.UpdateInboundClient(inbound, clientId)
	return err
}

// DeleteClient deletes a client from an inbound.
func (c *LocalConnector) DeleteClient(ctx context.Context, inboundId int, clientEmail string) error {
	// Verify inbound ownership
	_, err := c.GetInbound(ctx, inboundId)
	if err != nil {
		return err
	}

	// Delegate to inbound service (ignore needRestart return value)
	_, err = c.inboundService.DelInboundClient(inboundId, clientEmail)
	return err
}

// ResetClientTraffic resets traffic for a specific client.
func (c *LocalConnector) ResetClientTraffic(ctx context.Context, inboundId int, email string) error {
	// Verify ownership
	_, err := c.GetInbound(ctx, inboundId)
	if err != nil {
		return err
	}

	// Delegate to inbound service (ignore needRestart return value)
	_, err = c.inboundService.ResetClientTraffic(inboundId, email)
	return err
}

// GetOnlineClients returns list of currently online client emails.
func (c *LocalConnector) GetOnlineClients(ctx context.Context) ([]string, error) {
	// Delegate to inbound service (returns []string directly)
	emails := c.inboundService.GetOnlineClients()
	return emails, nil
}

// GetTraffic retrieves current traffic statistics.
func (c *LocalConnector) GetTraffic(ctx context.Context, reset bool) (*xray.Traffic, error) {
	// Delegate to xray service
	traffics, _, err := c.xrayService.GetXrayTraffic()
	if err != nil {
		return nil, fmt.Errorf("failed to get traffic: %w", err)
	}

	// Return aggregated traffic or first entry
	if len(traffics) > 0 {
		return traffics[0], nil
	}

	return &xray.Traffic{}, nil
}

// GetClientTraffics retrieves traffic stats for all clients.
func (c *LocalConnector) GetClientTraffics(ctx context.Context) ([]*xray.ClientTraffic, error) {
	db := database.GetDB()
	var traffics []*xray.ClientTraffic

	err := db.Where("server_id = ?", c.serverId).Find(&traffics).Error
	if err != nil {
		return nil, fmt.Errorf("failed to get client traffics: %w", err)
	}

	return traffics, nil
}

// StartXray starts the local Xray process.
func (c *LocalConnector) StartXray(ctx context.Context) error {
	return c.xrayService.RestartXray(true)
}

// StopXray stops the local Xray process.
func (c *LocalConnector) StopXray(ctx context.Context) error {
	return c.xrayService.StopXray()
}

// RestartXray restarts the local Xray process.
func (c *LocalConnector) RestartXray(ctx context.Context) error {
	return c.xrayService.RestartXray(false)
}

// GetXrayVersion returns the installed Xray version.
func (c *LocalConnector) GetXrayVersion(ctx context.Context) (string, error) {
	version := c.xrayService.GetXrayVersion()
	return version, nil
}

// GetXrayConfig returns the current Xray configuration as JSON.
func (c *LocalConnector) GetXrayConfig(ctx context.Context) (string, error) {
	config, err := c.xrayService.GetXrayConfig()
	if err != nil {
		return "", err
	}

	// Convert to JSON string
	configBytes, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return "", err
	}

	return string(configBytes), nil
}

// GetSystemStats retrieves system resource usage statistics.
func (c *LocalConnector) GetSystemStats(ctx context.Context) (*SystemStats, error) {
	stats := &SystemStats{}

	// CPU
	cpuPercents, err := cpu.Percent(time.Second, false)
	if err == nil && len(cpuPercents) > 0 {
		stats.CPUUsage = cpuPercents[0]
	}
	cpuCounts, err := cpu.Counts(false)
	if err == nil {
		stats.CPUCores = cpuCounts
	}

	// Memory
	vmem, err := mem.VirtualMemory()
	if err == nil {
		stats.MemTotal = vmem.Total
		stats.MemUsed = vmem.Used
		stats.MemUsage = vmem.UsedPercent
	}

	swap, err := mem.SwapMemory()
	if err == nil {
		stats.SwapTotal = swap.Total
		stats.SwapUsed = swap.Used
		stats.SwapUsage = swap.UsedPercent
	}

	// Disk (root partition)
	diskStat, err := disk.Usage("/")
	if err == nil {
		stats.DiskTotal = diskStat.Total
		stats.DiskUsed = diskStat.Used
		stats.DiskUsage = diskStat.UsedPercent
	}

	// System uptime
	hostInfo, err := host.Info()
	if err == nil {
		stats.Uptime = int64(hostInfo.Uptime)
		// Load average is Linux-specific
		if runtime.GOOS == "linux" {
			// Note: gopsutil v4 doesn't expose load average directly
			// We can read it from /proc/loadavg on Linux
			if loadData, err := os.ReadFile("/proc/loadavg"); err == nil {
				stats.LoadAverage = strings.TrimSpace(string(loadData))
			}
		}
	}

	// Network connections
	// Note: This would require additional implementation
	stats.TCPConnections = 0
	stats.UDPConnections = 0
	stats.XrayConnections = 0

	// Public IPs - TODO: implement GetPublicIP in ServerService
	stats.PublicIPv4 = ""
	stats.PublicIPv6 = ""

	return stats, nil
}

// GetLogs retrieves the last N lines of Xray logs.
func (c *LocalConnector) GetLogs(ctx context.Context, count int) ([]string, error) {
	logPath, err := xray.GetAccessLogPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get log path: %w", err)
	}

	// Read log file
	data, err := os.ReadFile(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read log file: %w", err)
	}

	// Split into lines
	lines := strings.Split(string(data), "\n")

	// Get last N lines
	start := len(lines) - count
	if start < 0 {
		start = 0
	}

	return lines[start:], nil
}

// UpdateGeoFiles updates Xray geo files (geoip.dat, geosite.dat).
func (c *LocalConnector) UpdateGeoFiles(ctx context.Context) error {
	// Note: ServerService has UpdateGeofile (singular) method
	// Update both geoip and geosite files
	err := c.serverService.UpdateGeofile("geoip.dat")
	if err != nil {
		return fmt.Errorf("failed to update geoip: %w", err)
	}
	err = c.serverService.UpdateGeofile("geosite.dat")
	if err != nil {
		return fmt.Errorf("failed to update geosite: %w", err)
	}
	return nil
}

// InstallXray installs a specific version of Xray.
func (c *LocalConnector) InstallXray(ctx context.Context, version string) error {
	return c.serverService.UpdateXray(version)
}

// GenerateCert generates an X25519 certificate (not TLS cert).
func (c *LocalConnector) GenerateCert(ctx context.Context, domain string) (*CertInfo, error) {
	// Note: The existing GenerateX25519Keys generates keypairs, not domain certs
	// This is a placeholder - actual cert generation would need ACME/Let's Encrypt
	return nil, fmt.Errorf("certificate generation not implemented for local connector")
}

// GetCerts returns information about installed certificates.
func (c *LocalConnector) GetCerts(ctx context.Context) ([]*CertInfo, error) {
	// Get certificate paths from settings
	db := database.GetDB()
	var settings []model.Setting

	certKeys := []string{
		"webCertFile",
		"webKeyFile",
		"subCertFile",
		"subKeyFile",
	}

	err := db.Where("key IN ?", certKeys).Find(&settings).Error
	if err != nil {
		return nil, err
	}

	certs := make([]*CertInfo, 0)

	// Parse certificate files and extract info
	// This is a simplified implementation
	for _, setting := range settings {
		if strings.HasSuffix(setting.Key, "CertFile") && setting.Value != "" {
			certInfo := &CertInfo{
				Domain:   "local",
				CertPath: setting.Value,
				IsValid:  true,
			}
			certs = append(certs, certInfo)
		}
	}

	return certs, nil
}

// BackupDatabase creates a database backup.
func (c *LocalConnector) BackupDatabase(ctx context.Context) ([]byte, error) {
	dbPath := config.GetDBPath()

	data, err := os.ReadFile(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read database: %w", err)
	}

	return data, nil
}

// RestoreDatabase restores database from backup.
func (c *LocalConnector) RestoreDatabase(ctx context.Context, data []byte) error {
	dbPath := config.GetDBPath()

	// Write backup
	err := os.WriteFile(dbPath+".backup", data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}

	// Validate backup
	if err := database.ValidateSQLiteDB(dbPath + ".backup"); err != nil {
		return fmt.Errorf("invalid database backup: %w", err)
	}

	// Replace database
	if err := os.Rename(dbPath+".backup", dbPath); err != nil {
		return fmt.Errorf("failed to restore database: %w", err)
	}

	return nil
}
