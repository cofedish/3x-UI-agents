// Package model defines the database models and data structures used by the 3x-ui panel.
package model

import (
	"fmt"

	"github.com/cofedish/3x-UI-agents/util/json_util"
	"github.com/cofedish/3x-UI-agents/xray"
)

// Protocol represents the protocol type for Xray inbounds.
type Protocol string

// Protocol constants for different Xray inbound protocols
const (
	VMESS       Protocol = "vmess"
	VLESS       Protocol = "vless"
	Tunnel      Protocol = "tunnel"
	HTTP        Protocol = "http"
	Trojan      Protocol = "trojan"
	Shadowsocks Protocol = "shadowsocks"
	Mixed       Protocol = "mixed"
	WireGuard   Protocol = "wireguard"
)

// User represents a user account in the 3x-ui panel.
type User struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Inbound represents an Xray inbound configuration with traffic statistics and settings.
type Inbound struct {
	Id                   int                  `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`                                                    // Unique identifier
	UserId               int                  `json:"-"`                                                                                               // Associated user ID
	ServerId             int                  `json:"serverId" form:"serverId" gorm:"index"`                                                           // Foreign key to Server (for multi-server support)
	ServerAddress        string               `json:"serverAddress,omitempty" gorm:"-"`                                                                // Server address/hostname (not stored in DB, populated at runtime)
	Up                   int64                `json:"up" form:"up"`                                                                                    // Upload traffic in bytes
	Down                 int64                `json:"down" form:"down"`                                                                                // Download traffic in bytes
	Total                int64                `json:"total" form:"total"`                                                                              // Total traffic limit in bytes
	AllTime              int64                `json:"allTime" form:"allTime" gorm:"default:0"`                                                         // All-time traffic usage
	Remark               string               `json:"remark" form:"remark"`                                                                            // Human-readable remark
	Enable               bool                 `json:"enable" form:"enable" gorm:"index:idx_enable_traffic_reset,priority:1"`                           // Whether the inbound is enabled
	ExpiryTime           int64                `json:"expiryTime" form:"expiryTime"`                                                                    // Expiration timestamp
	TrafficReset         string               `json:"trafficReset" form:"trafficReset" gorm:"default:never;index:idx_enable_traffic_reset,priority:2"` // Traffic reset schedule
	LastTrafficResetTime int64                `json:"lastTrafficResetTime" form:"lastTrafficResetTime" gorm:"default:0"`                               // Last traffic reset timestamp
	ClientStats          []xray.ClientTraffic `gorm:"foreignKey:InboundId;references:Id" json:"clientStats" form:"clientStats"`                        // Client traffic statistics

	// Xray configuration fields
	Listen         string   `json:"listen" form:"listen"`
	Port           int      `json:"port" form:"port"`
	Protocol       Protocol `json:"protocol" form:"protocol"`
	Settings       string   `json:"settings" form:"settings"`
	StreamSettings string   `json:"streamSettings" form:"streamSettings"`
	Tag            string   `json:"tag" form:"tag" gorm:"unique"`
	Sniffing       string   `json:"sniffing" form:"sniffing"`
}

// OutboundTraffics tracks traffic statistics for Xray outbound connections.
type OutboundTraffics struct {
	Id       int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	ServerId int    `json:"serverId" form:"serverId" gorm:"index"` // Foreign key to Server (for multi-server support)
	Tag      string `json:"tag" form:"tag" gorm:"unique"`
	Up       int64  `json:"up" form:"up" gorm:"default:0"`
	Down     int64  `json:"down" form:"down" gorm:"default:0"`
	Total    int64  `json:"total" form:"total" gorm:"default:0"`
}

// InboundClientIps stores IP addresses associated with inbound clients for access control.
type InboundClientIps struct {
	Id          int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerId    int    `json:"serverId" form:"serverId" gorm:"index"` // Foreign key to Server (for multi-server support)
	ClientEmail string `json:"clientEmail" form:"clientEmail" gorm:"unique"`
	Ips         string `json:"ips" form:"ips"`
}

// HistoryOfSeeders tracks which database seeders have been executed to prevent re-running.
type HistoryOfSeeders struct {
	Id         int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SeederName string `json:"seederName"`
}

// GenXrayInboundConfig generates an Xray inbound configuration from the Inbound model.
func (i *Inbound) GenXrayInboundConfig() *xray.InboundConfig {
	listen := i.Listen
	if listen != "" {
		listen = fmt.Sprintf("\"%v\"", listen)
	}
	return &xray.InboundConfig{
		Listen:         json_util.RawMessage(listen),
		Port:           i.Port,
		Protocol:       string(i.Protocol),
		Settings:       json_util.RawMessage(i.Settings),
		StreamSettings: json_util.RawMessage(i.StreamSettings),
		Tag:            i.Tag,
		Sniffing:       json_util.RawMessage(i.Sniffing),
	}
}

// Setting stores key-value configuration settings for the 3x-ui panel.
type Setting struct {
	Id    int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Key   string `json:"key" form:"key"`
	Value string `json:"value" form:"value"`
}

// Client represents a client configuration for Xray inbounds with traffic limits and settings.
type Client struct {
	ID         string `json:"id"`                           // Unique client identifier
	Security   string `json:"security"`                     // Security method (e.g., "auto", "aes-128-gcm")
	Password   string `json:"password"`                     // Client password
	Flow       string `json:"flow"`                         // Flow control (XTLS)
	Email      string `json:"email"`                        // Client email identifier
	LimitIP    int    `json:"limitIp"`                      // IP limit for this client
	TotalGB    int64  `json:"totalGB" form:"totalGB"`       // Total traffic limit in GB
	ExpiryTime int64  `json:"expiryTime" form:"expiryTime"` // Expiration timestamp
	Enable     bool   `json:"enable" form:"enable"`         // Whether the client is enabled
	TgID       int64  `json:"tgId" form:"tgId"`             // Telegram user ID for notifications
	SubID      string `json:"subId" form:"subId"`           // Subscription identifier
	Comment    string `json:"comment" form:"comment"`       // Client comment
	Reset      int    `json:"reset" form:"reset"`           // Reset period in days
	CreatedAt  int64  `json:"created_at,omitempty"`         // Creation timestamp
	UpdatedAt  int64  `json:"updated_at,omitempty"`         // Last update timestamp
}

// Server represents a managed VPN server in multi-server architecture.
// In single-server mode, there is only one server with ID=1 (local).
type Server struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name     string `json:"name" gorm:"unique;not null"` // Unique server name (e.g., "US-East-1")
	Endpoint string `json:"endpoint" gorm:"not null"`    // Agent endpoint (e.g., "https://vpn1.example.com:2054")
	Region   string `json:"region"`                      // Geographic region (e.g., "us-east")
	Tags     string `json:"tags"`                        // JSON array of tags (e.g., ["production", "us"])

	// Authentication
	AuthType string `json:"authType" gorm:"not null"` // "mtls", "jwt", or "local"
	AuthData string `json:"authData"`                 // Encrypted secret or certificate reference (encrypted)

	// Status
	Status    string `json:"status" gorm:"default:'pending';index"` // "pending", "online", "offline", "error"
	LastSeen  int64  `json:"lastSeen"`                              // Unix timestamp of last successful health check
	LastError string `json:"lastError"`                             // Last error message (if status is "error")

	// Metadata
	Version     string `json:"version"`     // Agent version
	XrayVersion string `json:"xrayVersion"` // Xray version on the server
	OsInfo      string `json:"osInfo"`      // JSON: {"os": "linux", "arch": "amd64", "kernel": "5.15"}

	// Settings
	Enabled bool   `json:"enabled" gorm:"default:true;index"` // Whether this server is enabled
	Notes   string `json:"notes"`                             // Admin notes

	// Timestamps
	CreatedAt int64 `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt int64 `json:"updatedAt" gorm:"autoUpdateTime"`
}

// ServerTask represents an operation executed on a managed server.
// Used for audit logging and async job tracking.
type ServerTask struct {
	Id       int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ServerId int    `json:"serverId" gorm:"not null;index"` // Foreign key to Server
	Server   Server `json:"server" gorm:"foreignKey:ServerId"`

	Operation string `json:"operation" gorm:"not null"`                      // Operation type (e.g., "add_inbound", "restart_xray")
	Status    string `json:"status" gorm:"not null;index;default:'pending'"` // "pending", "running", "completed", "failed"

	// Request/Response
	RequestData  string `json:"requestData"`  // JSON of input parameters
	ResponseData string `json:"responseData"` // JSON of operation result
	ErrorMessage string `json:"errorMessage"` // Error details if failed

	// Execution
	StartedAt   int64 `json:"startedAt"`                   // Unix timestamp when task started
	CompletedAt int64 `json:"completedAt"`                 // Unix timestamp when task completed
	RetryCount  int   `json:"retryCount" gorm:"default:0"` // Number of retry attempts

	// Audit
	UserId int `json:"userId"` // Admin user who triggered this operation

	// Timestamps
	CreatedAt int64 `json:"createdAt" gorm:"autoCreateTime"`
}
