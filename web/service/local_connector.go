// Package service provides LocalConnector for managing the local Xray server instance.
package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v2/config"
	"github.com/mhsanaei/3x-ui/v2/database"
	"github.com/mhsanaei/3x-ui/v2/database/model"
	"github.com/mhsanaei/3x-ui/v2/logger"
	"github.com/mhsanaei/3x-ui/v2/xray"
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
	xrayVersion, err := c.xrayService.GetXrayVersions()
	if err != nil {
		logger.Warning("Failed to get Xray version:", err)
		xrayVersion = "unknown"
	}

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

	xrayVersion, _ := c.xrayService.GetXrayVersions()

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

	// Delegate to inbound service
	return c.inboundService.AddInbound(inbound)
}

// UpdateInbound updates an existing inbound.
func (c *LocalConnector) UpdateInbound(ctx context.Context, inbound *model.Inbound) error {
	// Ensure server_id matches
	inbound.ServerId = c.serverId

	// Delegate to inbound service
	return c.inboundService.UpdateInbound(inbound)
}

// DeleteInbound deletes an inbound by ID.
func (c *LocalConnector) DeleteInbound(ctx context.Context, id int) error {
	// Verify ownership
	inbound, err := c.GetInbound(ctx, id)
	if err != nil {
		return err
	}

	// Delegate to inbound service
	return c.inboundService.DelInbound(inbound.Id)
}

// AddClient adds a client to an inbound.
func (c *LocalConnector) AddClient(ctx context.Context, inbound *model.Inbound) error {
	// Ensure server_id
	inbound.ServerId = c.serverId

	// Delegate to inbound service
	return c.inboundService.AddInboundClient(inbound)
}

// UpdateClient updates a client in an inbound.
func (c *LocalConnector) UpdateClient(ctx context.Context, inbound *model.Inbound, clientIndex int) error {
	// Ensure server_id
	inbound.ServerId = c.serverId

	// Delegate to inbound service
	clientId, err := c.inboundService.GetClientIdByIndex(inbound.Id, clientIndex)
	if err != nil {
		return err
	}

	return c.inboundService.UpdateInboundClient(inbound, clientId)
}

// DeleteClient deletes a client from an inbound.
func (c *LocalConnector) DeleteClient(ctx context.Context, inboundId int, clientEmail string) error {
	// Verify inbound ownership
	_, err := c.GetInbound(ctx, inboundId)
	if err != nil {
		return err
	}

	// Delegate to inbound service
	return c.inboundService.DelInboundClient(inboundId, clientEmail)
}

// ResetClientTraffic resets traffic for a specific client.
func (c *LocalConnector) ResetClientTraffic(ctx context.Context, inboundId int, email string) error {
	// Verify ownership
	_, err := c.GetInbound(ctx, inboundId)
	if err != nil {
		return err
	}

	// Delegate to inbound service
	return c.inboundService.ResetClientTraffic(inboundId, email)
}

// GetOnlineClients returns list of currently online client emails.
func (c *LocalConnector) GetOnlineClients(ctx context.Context) ([]string, error) {
	// Delegate to inbound service
	onlineClients, err := c.inboundService.GetOnlineClients()
	if err != nil {
		return nil, err
	}

	var emails []string
	for email := range onlineClients {
		emails = append(emails, email)
	}

	return emails, nil
}

// GetTraffic retrieves current traffic statistics.
func (c *LocalConnector) GetTraffic(ctx context.Context, reset bool) (*xray.Traffic, error) {
	// Get Xray API
	xrayApi := xray.GetXrayAPI()
	if xrayApi == nil {
		return nil, fmt.Errorf("xray api not initialized")
	}

	// Get traffic
	traffics, err := xrayApi.GetTraffic(reset)
	if err != nil {
		return nil, fmt.Errorf("failed to get traffic: %w", err)
	}

	return traffics, nil
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
	version, err := c.xrayService.GetXrayVersions()
	if err != nil {
		return "", err
	}
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
			if info, err := host.InfoWithContext(ctx); err == nil {
				// Note: gopsutil v4 doesn't expose load average directly
				// We can read it from /proc/loadavg on Linux
				if loadData, err := os.ReadFile("/proc/loadavg"); err == nil {
					stats.LoadAverage = strings.TrimSpace(string(loadData))
				}
			}
		}
	}

	// Network connections
	// Note: This would require additional implementation
	stats.TCPConnections = 0
	stats.UDPConnections = 0
	stats.XrayConnections = 0

	// Public IPs (delegate to server service if needed)
	ipv4, _ := c.serverService.GetPublicIP(true)
	ipv6, _ := c.serverService.GetPublicIP(false)
	stats.PublicIPv4 = ipv4
	stats.PublicIPv6 = ipv6

	return stats, nil
}

// GetLogs retrieves the last N lines of Xray logs.
func (c *LocalConnector) GetLogs(ctx context.Context, count int) ([]string, error) {
	logPath := path.Join(config.GetLogFolder(), config.GetAccessLogName())

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
	return c.serverService.UpdateGeoFiles()
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
