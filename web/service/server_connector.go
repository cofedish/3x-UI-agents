// Package service provides the ServerConnector abstraction for unified local and remote server operations.
package service

import (
	"context"

	"github.com/cofedish/3x-UI-agents/database/model"
	"github.com/cofedish/3x-UI-agents/xray"
)

// ServerConnector provides a unified interface for managing both local and remote servers.
// LocalConnector implements this for the local Xray instance.
// RemoteConnector implements this for agent-managed remote servers.
type ServerConnector interface {
	// Server Metadata
	GetServerInfo(ctx context.Context) (*ServerInfo, error)
	GetHealth(ctx context.Context) (*HealthStatus, error)

	// Inbound Management
	ListInbounds(ctx context.Context) ([]*model.Inbound, error)
	GetInbound(ctx context.Context, id int) (*model.Inbound, error)
	AddInbound(ctx context.Context, inbound *model.Inbound) error
	UpdateInbound(ctx context.Context, inbound *model.Inbound) error
	DeleteInbound(ctx context.Context, id int) error

	// Client Management
	AddClient(ctx context.Context, inbound *model.Inbound) error
	UpdateClient(ctx context.Context, inbound *model.Inbound, clientIndex int) error
	DeleteClient(ctx context.Context, inboundId int, clientEmail string) error
	ResetClientTraffic(ctx context.Context, inboundId int, email string) error
	GetOnlineClients(ctx context.Context) ([]string, error)

	// Traffic & Stats
	GetTraffic(ctx context.Context, reset bool) (*xray.Traffic, error)
	GetClientTraffics(ctx context.Context) ([]*xray.ClientTraffic, error)

	// Xray Control
	StartXray(ctx context.Context) error
	StopXray(ctx context.Context) error
	RestartXray(ctx context.Context) error
	GetXrayVersion(ctx context.Context) (string, error)
	GetXrayConfig(ctx context.Context) (string, error)

	// System Operations
	GetSystemStats(ctx context.Context) (*SystemStats, error)
	GetLogs(ctx context.Context, count int) ([]string, error)
	UpdateGeoFiles(ctx context.Context) error
	InstallXray(ctx context.Context, version string) error

	// Certificates
	GenerateCert(ctx context.Context, domain string) (*CertInfo, error)
	GetCerts(ctx context.Context) ([]*CertInfo, error)

	// Backups
	BackupDatabase(ctx context.Context) ([]byte, error)
	RestoreDatabase(ctx context.Context, data []byte) error
}

// ServerInfo contains basic server metadata and version information.
type ServerInfo struct {
	ServerId    int    `json:"serverId"`
	ServerName  string `json:"serverName"`
	Version     string `json:"version"`      // Panel/Agent version
	XrayVersion string `json:"xray_version"` // Xray core version (agent uses snake_case)
	OS          string `json:"os"`           // Operating system
	Arch        string `json:"arch"`         // Architecture
	Kernel      string `json:"kernel"`       // Kernel version (Linux)
	Uptime      int64  `json:"uptime"`       // Uptime in seconds
}

// HealthStatus represents the current health status of a server.
type HealthStatus struct {
	Status      string `json:"status"`        // "online", "offline", "error"
	XrayRunning bool   `json:"xray_running"`  // Is Xray process running (agent reports snake_case)
	Version     string `json:"version"`
	XrayVersion string `json:"xray_version"`
	LastError   string `json:"lastError,omitempty"`
	Timestamp   int64  `json:"timestamp"` // Unix timestamp of health check
}

// SystemStats contains system resource usage information.
type SystemStats struct {
	// CPU
	CPUUsage float64 `json:"cpuUsage"` // Percentage (0-100)
	CPUCores int     `json:"cpuCores"`

	// Memory
	MemTotal uint64  `json:"memTotal"` // Bytes
	MemUsed  uint64  `json:"memUsed"`  // Bytes
	MemUsage float64 `json:"memUsage"` // Percentage (0-100)

	// Swap
	SwapTotal uint64  `json:"swapTotal"` // Bytes
	SwapUsed  uint64  `json:"swapUsed"`  // Bytes
	SwapUsage float64 `json:"swapUsage"` // Percentage (0-100)

	// Disk
	DiskTotal uint64  `json:"diskTotal"` // Bytes
	DiskUsed  uint64  `json:"diskUsed"`  // Bytes
	DiskUsage float64 `json:"diskUsage"` // Percentage (0-100)

	// Network
	NetInSpeed  int64 `json:"netInSpeed"`  // Bytes/sec
	NetOutSpeed int64 `json:"netOutSpeed"` // Bytes/sec

	// System
	Uptime          int64  `json:"uptime"`          // Seconds
	LoadAverage     string `json:"loadAverage"`     // "0.5, 0.7, 0.9"
	TCPConnections  int    `json:"tcpConnections"`  // Active TCP connections
	UDPConnections  int    `json:"udpConnections"`  // Active UDP connections
	XrayConnections int    `json:"xrayConnections"` // Active Xray client connections
	PublicIPv4      string `json:"publicIPv4"`
	PublicIPv6      string `json:"publicIPv6"`
}

// CertInfo contains SSL/TLS certificate information.
type CertInfo struct {
	Domain    string `json:"domain"`
	CertPath  string `json:"certPath"`
	KeyPath   string `json:"keyPath"`
	IssuedBy  string `json:"issuedBy"`  // CA or "Self-signed"
	NotBefore int64  `json:"notBefore"` // Unix timestamp
	NotAfter  int64  `json:"notAfter"`  // Unix timestamp
	ValidDays int    `json:"validDays"` // Days until expiration
	IsValid   bool   `json:"isValid"`   // Is currently valid
	IsExpired bool   `json:"isExpired"` // Is expired
	AutoRenew bool   `json:"autoRenew"` // Auto-renewal enabled
}
