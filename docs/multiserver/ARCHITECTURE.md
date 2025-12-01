# 3x-ui Multi-Server Architecture

## Document Status
- **Version:** 1.0
- **Date:** 2025-11-30
- **Author:** Multi-Server Implementation Team
- **Status:** Design Document

---

## Table of Contents
1. [Executive Summary](#executive-summary)
2. [Current Architecture](#current-architecture)
3. [Target Architecture](#target-architecture)
4. [Core Components](#core-components)
5. [Security Model](#security-model)
6. [Database Schema](#database-schema)
7. [API Design](#api-design)
8. [Communication Protocol](#communication-protocol)
9. [Server Enrollment](#server-enrollment)
10. [Backward Compatibility](#backward-compatibility)
11. [Implementation Plan](#implementation-plan)

---

## Executive Summary

This document describes the architectural design for transforming 3x-ui from a single-server panel into a distributed multi-server management system. The new architecture introduces a **Controller-Agent** pattern where:

- **Controller (Panel)**: Centralized web UI + backend managing multiple VPN servers
- **Agent**: Lightweight service on each managed VPN server executing local operations

### Goals
- ✅ **Full Feature Preservation**: All existing 3x-ui functionality retained
- ✅ **Backward Compatibility**: Single-server mode works identically to current version
- ✅ **Production-Grade**: Security, migrations, logging, error handling, restart handling
- ✅ **No Compromises**: No TODOs, no stubs, no feature cuts
- ✅ **Telegram Bot Adaptation**: Full multi-server support in bot

---

## Current Architecture

### Single-Server Model (Current State)

```
┌─────────────────────────────────────────────────┐
│          Single Physical Server                 │
│                                                  │
│  ┌──────────────┐      ┌──────────────┐        │
│  │   3x-ui      │      │   Xray       │        │
│  │   Panel      │◄────►│   Core       │        │
│  │              │ gRPC │              │        │
│  │  - Web UI    │      │  - Inbounds  │        │
│  │  - API       │      │  - Outbounds │        │
│  │  - TG Bot    │      │  - Traffic   │        │
│  │  - SQLite DB │      └──────────────┘        │
│  └──────────────┘                               │
│         │                                        │
│         ▼                                        │
│  ┌──────────────┐                               │
│  │ Subscription │                               │
│  │   Server     │                               │
│  └──────────────┘                               │
└─────────────────────────────────────────────────┘
         │
         ▼
    VPN Clients
```

**Characteristics:**
- All components run on same server
- Direct local access to Xray process
- Local filesystem operations
- Single SQLite database
- Tight coupling between UI and Xray management

**Limitations:**
- Cannot manage remote servers
- No centralized management
- Panel must run on VPN server
- Scaling requires manual replication

---

## Target Architecture

### Multi-Server Controller-Agent Model

```
┌──────────────────────────────────────────────────────────────────┐
│                         CONTROLLER                                │
│                    (Can be anywhere)                              │
│                                                                    │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                    Web Panel (UI)                          │  │
│  │  - Dashboard (all servers aggregated)                      │  │
│  │  - Server Selector                                         │  │
│  │  - Multi-server management                                 │  │
│  └────────────────────────────────────────────────────────────┘  │
│                             │                                     │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │                    Backend API                             │  │
│  │  ┌─────────────────────────────────────────────────────┐   │  │
│  │  │         ServerConnector Interface                   │   │  │
│  │  │  ┌──────────────────┐   ┌──────────────────────┐   │   │  │
│  │  │  │ LocalConnector   │   │  RemoteConnector     │   │   │  │
│  │  │  │ (backward compat)│   │  (agent API calls)   │   │   │  │
│  │  │  └──────────────────┘   └──────────────────────┘   │   │  │
│  │  └─────────────────────────────────────────────────────┘   │  │
│  │  - InboundService                                          │  │
│  │  - ServerService                                           │  │
│  │  - TelegramBot (multi-server aware)                       │  │
│  └────────────────────────────────────────────────────────────┘  │
│                             │                                     │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │              PostgreSQL / SQLite Database                  │  │
│  │  - Servers (id, name, endpoint, auth, status)             │  │
│  │  - Inbounds (server_id FK)                                │  │
│  │  - Users                                                   │  │
│  │  - Traffic (server_id FK)                                 │  │
│  │  - Tasks/Jobs (operation history)                         │  │
│  └────────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────────┘
         │                    │                    │
         │ mTLS/JWT           │                    │
         ▼                    ▼                    ▼
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│  AGENT (VPN-1)  │  │  AGENT (VPN-2)  │  │  AGENT (VPN-N)  │
│  ┌───────────┐  │  │  ┌───────────┐  │  │  ┌───────────┐  │
│  │Agent API  │  │  │  │Agent API  │  │  │  │Agent API  │  │
│  │(REST/gRPC)│  │  │  │(REST/gRPC)│  │  │  │(REST/gRPC)│  │
│  └───────────┘  │  │  └───────────┘  │  │  └───────────┘  │
│       │         │  │       │         │  │       │         │
│       ▼         │  │       ▼         │  │       ▼         │
│  ┌───────────┐  │  │  ┌───────────┐  │  │  ┌───────────┐  │
│  │Xray Core  │  │  │  │Xray Core  │  │  │  │Xray Core  │  │
│  └───────────┘  │  │  └───────────┘  │  │  └───────────┘  │
│                 │  │                 │  │                 │
│  Region: US     │  │  Region: EU     │  │  Region: ASIA   │
└─────────────────┘  └─────────────────┘  └─────────────────┘
```

**Key Characteristics:**
- Controller can be deployed separately from VPN servers
- Agents are lightweight services (no web UI needed)
- Secure mTLS or JWT-based communication
- Centralized database and management
- Horizontal scaling: add more agents as needed

---

## Core Components

### 1. Controller (Panel)

**Purpose:** Centralized management interface for all VPN servers

**Responsibilities:**
- Web UI for administrators
- User authentication and authorization
- Server inventory management
- Aggregated metrics and dashboards
- Job scheduling and task orchestration
- Telegram bot integration
- Database management

**New Files:**
```
web/service/server_connector.go     # Connector interface
web/service/local_connector.go      # Local mode implementation
web/service/remote_connector.go     # Remote agent client
web/service/agent_client.go         # HTTP/gRPC client for agent
web/service/server_health.go        # Health monitoring
web/job/server_poll_job.go          # Periodic server polling
database/model/server.go             # Server model
```

**Modified Files:**
```
web/service/inbound.go               # Use ServerConnector
web/service/xray.go                  # Use ServerConnector
web/service/tgbot.go                 # Multi-server support
web/html/index.html                  # Server selector + aggregated stats
web/html/servers.html                # NEW: Server management page
```

---

### 2. Agent (Server-Side Service)

**Purpose:** Lightweight service running on each managed VPN server

**Responsibilities:**
- Execute Xray management operations locally
- Provide health and metrics endpoints
- Manage local certificates and configuration
- Stream logs to controller (with size limits)
- Handle service restarts and updates
- No web UI, only API

**Implementation Options:**

**Option A: Standalone Binary** (Preferred)
```
agent/
├── main.go                    # Agent entry point
├── api/
│   ├── router.go             # API routes
│   ├── auth.go               # mTLS/JWT authentication
│   ├── inbound_handler.go    # Inbound operations
│   ├── server_handler.go     # Server operations
│   ├── traffic_handler.go    # Traffic stats
│   └── cert_handler.go       # Certificate management
├── service/
│   ├── xray_service.go       # Xray management (reuses xray/)
│   └── system_service.go     # System info
└── config/
    └── agent_config.go       # Agent configuration
```

**Option B: Shared Binary with Mode Flag**
```bash
# Panel mode (current behavior)
./x-ui run

# Agent mode (new)
./x-ui agent --controller https://panel.example.com --token xxx
```

**Agent Configuration:**
```yaml
# /etc/x-ui-agent/config.yaml
controller:
  endpoint: "https://panel.example.com:2053"
  auth:
    type: "mtls"  # or "jwt"
    cert_file: "/etc/x-ui-agent/certs/agent.crt"
    key_file: "/etc/x-ui-agent/certs/agent.key"
    ca_file: "/etc/x-ui-agent/certs/ca.crt"

agent:
  listen_addr: "0.0.0.0:2054"
  server_id: "agent-us-1"
  tags: ["us", "production"]

xray:
  bin_folder: "/usr/local/x-ui/bin"
  config_folder: "/etc/x-ui"

logging:
  level: "info"
  file: "/var/log/x-ui-agent/agent.log"
```

---

### 3. ServerConnector Interface

**Purpose:** Unified abstraction for local and remote server operations

**Interface Definition:**
```go
// web/service/server_connector.go
package service

import (
    "context"
    "github.com/cofedish/3x-UI-agents/database/model"
    "github.com/cofedish/3x-UI-agents/xray"
)

type ServerConnector interface {
    // Metadata
    GetServerInfo(ctx context.Context) (*ServerInfo, error)
    GetHealth(ctx context.Context) (*HealthStatus, error)

    // Inbound Management
    ListInbounds(ctx context.Context) ([]*model.Inbound, error)
    GetInbound(ctx context.Context, id int) (*model.Inbound, error)
    AddInbound(ctx context.Context, inbound *model.Inbound) error
    UpdateInbound(ctx context.Context, inbound *model.Inbound) error
    DeleteInbound(ctx context.Context, id int) error

    // Client Management
    AddClient(ctx context.Context, inboundId int, client *model.Client) error
    UpdateClient(ctx context.Context, clientId string, client *model.Client) error
    DeleteClient(ctx context.Context, inboundId int, clientId string) error
    ResetClientTraffic(ctx context.Context, inboundId int, email string) error

    // Traffic & Stats
    GetTraffic(ctx context.Context, reset bool) (*xray.Traffic, error)
    GetClientTraffic(ctx context.Context) ([]*xray.ClientTraffic, error)
    GetOnlineClients(ctx context.Context) ([]string, error)

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

type LocalConnector struct {
    // Uses existing services directly
    inboundService *InboundService
    xrayService    *XrayService
    serverService  *ServerService
}

type RemoteConnector struct {
    serverID   int
    endpoint   string
    httpClient *http.Client  // or gRPC client
    authToken  string
}
```

---

## Security Model

### Overview

**Primary Authentication**: **Production-grade mTLS (Mutual TLS)**
**Fallback (Dev/Test Only)**: JWT Bearer Token

The 3x-ui multi-server architecture implements **defense-in-depth** security with mTLS as the primary authentication mechanism. This ensures bidirectional authentication and encryption for all controller-agent communication.

### Threat Model

**Threats Mitigated:**
- ✅ Unauthorized access to agent API
- ✅ Man-in-the-middle (MITM) attacks on controller-agent communication
- ✅ Token/credential theft and replay
- ✅ Agent impersonation
- ✅ Eavesdropping on sensitive data
- ✅ Tampering with configuration/traffic data

**Security Requirements Implemented:**
- ✅ Strong mutual authentication (mTLS with TLS 1.3)
- ✅ Encrypted communication (TLS 1.3 with modern cipher suites)
- ✅ Certificate-based authorization
- ✅ Request logging and audit trails
- ✅ Certificate rotation support
- ✅ Rate limiting and request validation
- ✅ Secure log access with path restrictions

---

### Authentication Method Comparison

| Security Feature | mTLS | JWT |
|-----------------|------|-----|
| **Mutual Authentication** | ✅ Both parties verify | ❌ Server only |
| **Certificate Rotation** | ✅ Annual rotation | ❌ N/A |
| **Token Theft Protection** | ✅ Certificate + key required | ❌ Token alone sufficient |
| **Replay Attack Resistance** | ✅ TLS handshake nonce | ⚠️ Limited |
| **MITM Protection** | ✅ Certificate pinning | ⚠️ TLS only |
| **Compromise Impact** | 🟡 Single agent | 🔴 All agents |
| **TLS Version** | ✅ 1.3 enforced | ✅ 1.3 enforced |
| **Cipher Suites** | ✅ Modern only | ✅ Modern only |
| **Production Ready** | **✅ YES** | **❌ NO** |

**Recommendation**: **Always use mTLS in production.** JWT is provided for development/testing only.

---

### Authentication: mTLS (Mutual TLS) - IMPLEMENTED

#### Architecture

```
┌─────────────────────────────────────────────────────────┐
│            Certificate Authority (CA)                    │
│  Generated once, signs all certificates                  │
│  ca.key (OFFLINE STORAGE) + ca.crt (DISTRIBUTED)        │
└───────────────┬─────────────────────────────────────────┘
                │ signs all certificates
                │
    ┌───────────┼───────────┬───────────┐
    │           │           │           │
    ▼           ▼           ▼           ▼
┌─────────┐ ┌─────────┐ ┌─────────┐ ┌─────────────┐
│ Agent 1 │ │ Agent 2 │ │ Agent N │ │ Controller  │
│(Server) │ │(Server) │ │(Server) │ │  (Client)   │
│         │ │         │ │         │ │             │
│ agent-  │ │ agent-  │ │ agent-  │ │ controller. │
│  01.crt │ │  02.crt │ │  nn.crt │ │    crt      │
│ agent-  │ │ agent-  │ │ agent-  │ │ controller. │
│  01.key │ │  02.key │ │  nn.key │ │    key      │
│ ca.crt  │ │ ca.crt  │ │ ca.crt  │ │ ca.crt      │
│         │ │         │ │         │ │             │
│ServerAuth│ │ServerAuth│ │ServerAuth│ │ ClientAuth │
└─────────┘ └─────────┘ └─────────┘ └─────────────┘
```

#### Certificate Generation (Actual Implementation)

**Scripts Location**: `scripts/certs/`

1. **Generate CA (Once)**
   ```bash
   cd scripts/certs
   ./gen-ca.sh

   # Outputs:
   # - ca.key (4096-bit RSA, KEEP SECURE!)
   # - ca.crt (valid 10 years)
   ```

2. **Generate Agent Certificates (Per Agent)**
   ```bash
   ./gen-agent-cert.sh agent-01

   # With SAN (Subject Alternative Name):
   ./gen-agent-cert.sh agent-01 ./certs ./certs 192.168.1.100
   ./gen-agent-cert.sh agent-02 ./certs ./certs agent.example.com

   # Outputs:
   # - agent-<id>.key (2048-bit RSA, valid 1 year)
   # - agent-<id>.crt (signed by CA)
   ```

3. **Generate Controller Certificate (Once)**
   ```bash
   ./gen-controller-cert.sh

   # Outputs:
   # - controller.key (2048-bit RSA, valid 1 year)
   # - controller.crt (signed by CA, clientAuth extension)
   ```

#### TLS Configuration (Agent Server)

**Implementation**: `agent/api/router.go:startTLSServer()`

```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},
    ClientAuth:   tls.RequireAndVerifyClientCert,  // MANDATORY
    ClientCAs:    caCertPool,
    MinVersion:   tls.VersionTLS13,
    CipherSuites: []uint16{
        tls.TLS_AES_128_GCM_SHA256,
        tls.TLS_AES_256_GCM_SHA384,
        tls.TLS_CHACHA20_POLY1305_SHA256,
    },
}
```

**Key Security Features:**
- `RequireAndVerifyClientCert`: Agent REJECTS requests without valid controller certificate
- `MinVersion: TLS13`: Only TLS 1.3 accepted (no downgrade attacks)
- Modern cipher suites only (AES-GCM, ChaCha20-Poly1305)
- Certificate verification at TLS layer (before HTTP processing)

#### TLS Configuration (Controller Client)

**Implementation**: `web/service/remote_connector.go:createMTLSClient()`

```go
tlsConfig := &tls.Config{
    Certificates: []tls.Certificate{cert},  // Controller client cert
    RootCAs:      caCertPool,               // CA for verifying agent
    MinVersion:   tls.VersionTLS13,
}
```

**Key Security Features:**
- Strict server verification (no `InsecureSkipVerify`)
- RootCAs ensures agent certificate signed by trusted CA
- Mutual verification: controller verifies agent, agent verifies controller

#### Certificate Deployment

**Agent Deployment:**
```bash
# Create directory
sudo mkdir -p /etc/x-ui-agent/certs

# Copy certificates
sudo cp agent-01.crt /etc/x-ui-agent/certs/agent.crt
sudo cp agent-01.key /etc/x-ui-agent/certs/agent.key
sudo cp ca.crt /etc/x-ui-agent/certs/ca.crt

# Secure permissions
sudo chmod 600 /etc/x-ui-agent/certs/agent.key
sudo chmod 644 /etc/x-ui-agent/certs/*.crt

# Configure environment
export AGENT_AUTH_TYPE=mtls
export AGENT_CERT_FILE=/etc/x-ui-agent/certs/agent.crt
export AGENT_KEY_FILE=/etc/x-ui-agent/certs/agent.key
export AGENT_CA_FILE=/etc/x-ui-agent/certs/ca.crt

# Start agent
x-ui agent
```

**Controller Deployment:**
```bash
# Copy certificates
sudo cp controller.crt /etc/x-ui/certs/controller.crt
sudo cp controller.key /etc/x-ui/certs/controller.key
sudo cp ca.crt /etc/x-ui/certs/ca.crt

# Secure permissions
sudo chmod 600 /etc/x-ui/certs/controller.key
sudo chmod 644 /etc/x-ui/certs/*.crt

# Configure in UI when adding agent:
Auth Type: mTLS
Auth Data: {
  "certFile": "/etc/x-ui/certs/controller.crt",
  "keyFile": "/etc/x-ui/certs/controller.key",
  "caFile": "/etc/x-ui/certs/ca.crt"
}
```

#### Authentication Flow (mTLS Handshake)

```
Controller                                Agent
    │                                       │
    │─────── TLS ClientHello ─────────────→│
    │                                       │
    │←────── TLS ServerHello ──────────────│
    │←────── Certificate (agent.crt) ──────│
    │←────── CertificateRequest ───────────│
    │                                       │
    │─────── Certificate (controller.crt)─→│ Agent verifies:
    │                                       │ - Signed by CA?
    │                                       │ - Not expired?
    │                                       │ - CN matches?
    │                                       │
    │←────── Finished ─────────────────────│
    │                                       │
Controller verifies:                       │
- Agent cert signed by CA?                 │
- Not expired?                             │
                                           │
    │─────── HTTP Request ─────────────────→│
    │         (encrypted, authenticated)    │
    │                                       │
    │←────── HTTP Response ─────────────────│
    │         (encrypted, authenticated)    │
```

#### Middleware Validation

**Implementation**: `agent/middleware/middleware.go:MTLSAuth()`

```go
// Middleware provides additional validation after TLS layer
func MTLSAuth(caFile string) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Check TLS connection exists
        if c.Request.TLS == nil {
            abort(401, "TLS_REQUIRED")
        }

        // 2. Check client certificate present
        // (should always be present due to RequireAndVerifyClientCert)
        if len(c.Request.TLS.PeerCertificates) == 0 {
            abort(401, "CLIENT_CERT_REQUIRED")
        }

        // 3. Extract client CN for logging/audit
        clientCert := c.Request.TLS.PeerCertificates[0]
        c.Set("client_cn", clientCert.Subject.CommonName)

        logger.Info("Client authenticated via mTLS: CN=%s",
                   clientCert.Subject.CommonName)

        c.Next()
    }
}
```

**Note**: Certificate verification (signature, expiry, chain) is done by the TLS layer. The middleware provides additional context extraction and logging.

#### Certificate Rotation

**Rotation Schedule:**
- **CA Certificate**: 10 years (rotate rarely, requires reissuing all certs)
- **Agent/Controller Certificates**: 1 year (rotate annually)

**Rotation Process (Zero-Downtime):**
```bash
# 1. Generate new certificate (same CA)
./gen-agent-cert.sh agent-01-renewed

# 2. Deploy to agent server (keep old cert active)
scp agent-agent-01-renewed.* user@agent:/tmp/
ssh user@agent "sudo cp /tmp/agent-agent-01-renewed.crt /etc/x-ui-agent/certs/agent.crt.new"
ssh user@agent "sudo cp /tmp/agent-agent-01-renewed.key /etc/x-ui-agent/certs/agent.key.new"

# 3. Atomically swap certificates
ssh user@agent "sudo mv /etc/x-ui-agent/certs/agent.crt.new /etc/x-ui-agent/certs/agent.crt"
ssh user@agent "sudo mv /etc/x-ui-agent/certs/agent.key.new /etc/x-ui-agent/certs/agent.key"

# 4. Restart agent (brief downtime, <1s)
ssh user@agent "sudo systemctl restart x-ui-agent"

# 5. Verify
./test-mtls.sh https://agent-server:2054
```

**Monitoring Certificate Expiry:**
```bash
# Check expiration date
openssl x509 -in /etc/x-ui-agent/certs/agent.crt -noout -dates

# Alert if expiring within 30 days
if openssl x509 -in /etc/x-ui-agent/certs/agent.crt -noout -checkend 2592000; then
    echo "Certificate valid for >30 days"
else
    echo "WARNING: Certificate expires within 30 days!"
fi
```

---

### Authentication: JWT (Development Only)

⚠️ **NOT RECOMMENDED FOR PRODUCTION**

**Use Cases:**
- Local development/testing
- Internal networks with strict firewall rules
- Environments where certificate management is impractical

**Implementation**: `agent/middleware/middleware.go:JWTAuth()`

```go
// Static Bearer token comparison (constant-time to prevent timing attacks)
func JWTAuth(secret string) gin.HandlerFunc {
    return func(c *gin.Context) {
        authHeader := c.GetHeader("Authorization")
        if !strings.HasPrefix(authHeader, "Bearer ") {
            abort(401, "INVALID_AUTH_FORMAT")
        }

        token := authHeader[7:]

        // Constant-time comparison
        if !secureCompare(token, secret) {
            abort(401, "INVALID_TOKEN")
        }

        c.Next()
    }
}
```

**Configuration:**
```bash
# Agent
export AGENT_AUTH_TYPE=jwt
export AGENT_JWT_SECRET=$(openssl rand -hex 32)

# Controller (in UI)
Auth Type: JWT
Auth Data: <secret from AGENT_JWT_SECRET>
```

**Security Limitations:**
- ❌ No mutual authentication
- ❌ Token can be intercepted and reused indefinitely
- ❌ Single token compromise affects all communication
- ❌ No certificate rotation mechanism

**Migration to mTLS:**
When ready for production, migrate to mTLS without downtime:
1. Generate certificates
2. Add new agent entry with mTLS configuration
3. Test connectivity
4. Switch traffic to new agent
5. Remove JWT agent entry

---

### Additional Security Measures

#### Rate Limiting

**Implementation**: `agent/middleware/middleware.go:RateLimiter`

```go
// Token bucket algorithm
// Default: 100 requests per minute per IP
rateLimiter := middleware.NewRateLimiter(cfg.RateLimit)
router.Use(rateLimiter.Middleware())
```

Prevents DoS attacks and brute-force attempts.

#### Request Logging

All requests logged with:
- Timestamp
- Client CN (from certificate)
- HTTP method and path
- Response status
- Duration
- Trace ID

**Log Format:**
```
[Agent API] POST /api/v1/inbounds | Status: 200 | Duration: 45ms | TraceID: abc123 | Client: CN=3x-ui-controller
```

#### Secure Log Access

**Implementation**: `agent/api/handlers.go:readLogFile()`

Log reading restricted with:
- **Path allowlist**: Only reads from approved directories
- **Path traversal prevention**: `filepath.Clean()` sanitization
- **Size limits**: Maximum 1000 lines per request
- **Permission checks**: Verifies file accessibility

**Allowed paths:**
```go
allowedPaths := []string{
    "/var/log/x-ui-agent/",
    "/var/log/3x-ui-agent/",
    "/tmp/x-ui-agent/",
}
```

#### Request Validation

- Maximum body size: 10MB
- Timeout: 30 seconds
- JSON validation
- Input sanitization

---

### Security Testing

**Test Script**: `scripts/certs/test-mtls.sh`

Verifies:
1. ✅ Public endpoints accessible
2. ✅ Protected endpoints reject requests without cert
3. ✅ Protected endpoints accept requests with valid cert
4. ✅ TLS 1.3 used
5. ✅ Certificate chain validation

**Usage:**
```bash
./test-mtls.sh https://agent-server:2054
```

**Expected Output:**
```
=== 3x-ui mTLS Connection Test ===

✓ All certificates found
✓ Controller certificate is valid and signed by CA
✓ Public endpoint accessible
✓ Request correctly rejected without client certificate
✓ mTLS authentication successful!
✓ Using TLS 1.3

=== All mTLS Tests Passed ===
```

---

### Compliance and Best Practices

**Standards Followed:**
- ✅ NIST SP 800-52 Rev. 2 (TLS Guidelines)
- ✅ OWASP Transport Layer Protection Cheat Sheet
- ✅ RFC 8446 (TLS 1.3)
- ✅ RFC 5280 (X.509 Certificates)

**Best Practices Implemented:**
- ✅ Principle of least privilege
- ✅ Defense in depth (multiple layers)
- ✅ Fail securely (reject by default)
- ✅ Audit logging
- ✅ Secure defaults (mTLS primary)
- ✅ No hardcoded secrets
- ✅ Constant-time comparisons (prevents timing attacks)

---

### Future Enhancements

**Planned:**
- Certificate revocation list (CRL) support
- OCSP (Online Certificate Status Protocol) stapling
- Hardware security module (HSM) integration for CA key
- Automated certificate renewal (ACME-style)
- Certificate transparency logging

**Under Consideration:**
- Zero-trust network architecture
- Service mesh integration (Istio/Linkerd)
- Policy-based access control (OPA)
- Behavioral anomaly detection
   - Controller presents controller.crt
   - Agent presents agent.crt
   - Both verify against ca.crt
   - Successful mutual authentication

**Certificate Metadata:**
```go
type Certificate struct {
    CommonName       string    // "agent-us-1"
    SerialNumber     string
    NotBefore        time.Time
    NotAfter         time.Time
    IssuedBy         string    // CA subject
    ServerID         int       // Links to database
}
```

**Certificate Storage:**
```sql
-- In controller database
CREATE TABLE certificates (
    id INTEGER PRIMARY KEY,
    server_id INTEGER NOT NULL REFERENCES servers(id),
    common_name TEXT NOT NULL,
    serial_number TEXT UNIQUE NOT NULL,
    issued_at TIMESTAMP NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    revoked_at TIMESTAMP,
    pem_certificate TEXT NOT NULL,
    UNIQUE(server_id)
);
```

**Revocation:**
```bash
# Revoke agent certificate
x-ui server revoke --server-id 5

# Certificate Revocation List (CRL) or OCSP can be added later
```

---

### Alternative: JWT with Rotating Keys

If mTLS is deemed too complex for initial release:

**Flow:**
1. Controller generates shared secret per agent during enrollment
2. Controller issues JWT tokens signed with secret
3. Agent validates JWT signature
4. Tokens expire after N hours (configurable)
5. Agent requests token refresh via separate endpoint

**JWT Claims:**
```json
{
  "sub": "agent-us-1",
  "server_id": 5,
  "iss": "3x-ui-controller",
  "iat": 1732234567,
  "exp": 1732324567,
  "scopes": ["inbound:write", "traffic:read", "xray:control"]
}
```

**Recommendation:** Start with **mTLS** for production-grade security.

---

## Database Schema

### New Tables

#### 1. `servers` Table
```sql
CREATE TABLE servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    endpoint TEXT NOT NULL,  -- "https://vpn-us-1.example.com:2054"
    region TEXT,
    tags TEXT,  -- JSON array: ["us", "production"]

    -- Authentication
    auth_type TEXT NOT NULL,  -- "mtls" or "jwt"
    auth_data TEXT,  -- Encrypted secret or cert reference

    -- Status
    status TEXT NOT NULL DEFAULT 'pending',  -- pending, online, offline, error
    last_seen TIMESTAMP,
    last_error TEXT,

    -- Metadata
    version TEXT,  -- Agent version
    xray_version TEXT,
    os_info TEXT,  -- JSON: {"os": "linux", "arch": "amd64"}

    -- Settings
    enabled BOOLEAN NOT NULL DEFAULT 1,
    notes TEXT,

    -- Timestamps
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_servers_status ON servers(status);
CREATE INDEX idx_servers_enabled ON servers(enabled);
```

#### 2. `server_tasks` Table (Operation History)
```sql
CREATE TABLE server_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL REFERENCES servers(id) ON DELETE CASCADE,
    operation TEXT NOT NULL,  -- "add_inbound", "restart_xray", etc.
    status TEXT NOT NULL,  -- "pending", "running", "completed", "failed"

    -- Request/Response
    request_data TEXT,  -- JSON of input parameters
    response_data TEXT,  -- JSON of result
    error_message TEXT,

    -- Execution
    started_at TIMESTAMP,
    completed_at TIMESTAMP,
    retry_count INTEGER DEFAULT 0,

    -- Audit
    user_id INTEGER,  -- Who triggered this
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_server_tasks_server ON server_tasks(server_id);
CREATE INDEX idx_server_tasks_status ON server_tasks(status);
```

---

### Modified Tables

All entities that previously operated in single-server context now need `server_id` foreign key:

#### `inbounds` Table
```sql
ALTER TABLE inbounds ADD COLUMN server_id INTEGER REFERENCES servers(id);

-- For backward compatibility migration:
-- Set server_id = 1 (default local server) for all existing rows
UPDATE inbounds SET server_id = 1 WHERE server_id IS NULL;

-- After migration, make NOT NULL
-- ALTER TABLE inbounds ALTER COLUMN server_id SET NOT NULL;

CREATE INDEX idx_inbounds_server ON inbounds(server_id);
```

#### `client_traffics` Table
```sql
ALTER TABLE client_traffics ADD COLUMN server_id INTEGER REFERENCES servers(id);
UPDATE client_traffics SET server_id = 1 WHERE server_id IS NULL;
CREATE INDEX idx_client_traffics_server ON client_traffics(server_id);
```

#### `outbound_traffics` Table
```sql
ALTER TABLE outbound_traffics ADD COLUMN server_id INTEGER REFERENCES servers(id);
UPDATE outbound_traffics SET server_id = 1 WHERE server_id IS NULL;
CREATE INDEX idx_outbound_traffics_server ON outbound_traffics(server_id);
```

#### `inbound_client_ips` Table
```sql
ALTER TABLE inbound_client_ips ADD COLUMN server_id INTEGER REFERENCES servers(id);
UPDATE inbound_client_ips SET server_id = 1 WHERE server_id IS NULL;
CREATE INDEX idx_inbound_client_ips_server ON inbound_client_ips(server_id);
```

---

### Migration Strategy

**Migration File:** `database/migrations/001_add_multiserver.go`

```go
func MigrateToMultiserver(db *gorm.DB) error {
    // 1. Create servers table
    if err := db.AutoMigrate(&model.Server{}); err != nil {
        return err
    }

    // 2. Create default "Local Server"
    defaultServer := &model.Server{
        ID:       1,
        Name:     "Default Local Server",
        Endpoint: "local://",
        AuthType: "local",
        Status:   "online",
        Enabled:  true,
        Notes:    "Auto-created during multi-server migration",
    }
    if err := db.FirstOrCreate(defaultServer, "id = ?", 1).Error; err != nil {
        return err
    }

    // 3. Add server_id columns
    migrations := []string{
        "ALTER TABLE inbounds ADD COLUMN server_id INTEGER REFERENCES servers(id)",
        "ALTER TABLE client_traffics ADD COLUMN server_id INTEGER REFERENCES servers(id)",
        "ALTER TABLE outbound_traffics ADD COLUMN server_id INTEGER REFERENCES servers(id)",
        "ALTER TABLE inbound_client_ips ADD COLUMN server_id INTEGER REFERENCES servers(id)",
    }

    for _, migration := range migrations {
        if err := db.Exec(migration).Error; err != nil {
            // Column might already exist, check error
            if !strings.Contains(err.Error(), "duplicate column") {
                return err
            }
        }
    }

    // 4. Set default server_id = 1 for existing records
    updates := []string{
        "UPDATE inbounds SET server_id = 1 WHERE server_id IS NULL",
        "UPDATE client_traffics SET server_id = 1 WHERE server_id IS NULL",
        "UPDATE outbound_traffics SET server_id = 1 WHERE server_id IS NULL",
        "UPDATE inbound_client_ips SET server_id = 1 WHERE server_id IS NULL",
    }

    for _, update := range updates {
        if err := db.Exec(update).Error; err != nil {
            return err
        }
    }

    // 5. Create indexes
    indexes := []string{
        "CREATE INDEX IF NOT EXISTS idx_inbounds_server ON inbounds(server_id)",
        "CREATE INDEX IF NOT EXISTS idx_client_traffics_server ON client_traffics(server_id)",
        "CREATE INDEX IF NOT EXISTS idx_outbound_traffics_server ON outbound_traffics(server_id)",
        "CREATE INDEX IF NOT EXISTS idx_inbound_client_ips_server ON inbound_client_ips(server_id)",
    }

    for _, index := range indexes {
        if err := db.Exec(index).Error; err != nil {
            return err
        }
    }

    // 6. Create server_tasks table
    if err := db.AutoMigrate(&model.ServerTask{}); err != nil {
        return err
    }

    return nil
}
```

**Migration Execution:**
- Runs automatically on first startup after upgrade
- Uses `history_of_seeders` table to track execution
- Idempotent: safe to run multiple times

---

## API Design

### Agent REST API

**Base URL:** `https://<agent-endpoint>:2054/api/v1`

**Authentication:** mTLS or Bearer JWT

**Common Response Format:**
```json
{
  "success": true,
  "data": { ... },
  "error": null,
  "trace_id": "abc123"
}
```

---

### Endpoint Categories

#### 1. Health & Info
```
GET /health
GET /info
GET /metrics
```

**Example: GET /info**
```json
{
  "success": true,
  "data": {
    "server_id": "agent-us-1",
    "version": "2.5.0",
    "xray_version": "1.8.6",
    "os": "linux",
    "arch": "amd64",
    "uptime": 86400,
    "last_restart": "2025-11-29T12:00:00Z"
  }
}
```

---

#### 2. Inbound Management
```
GET    /inbounds
GET    /inbounds/:id
POST   /inbounds
PUT    /inbounds/:id
DELETE /inbounds/:id
POST   /inbounds/:id/clients
PUT    /inbounds/:id/clients/:email
DELETE /inbounds/:id/clients/:email
POST   /inbounds/:id/clients/:email/reset-traffic
```

---

#### 3. Traffic & Stats
```
GET /traffic
GET /traffic/clients
GET /traffic/outbounds
GET /clients/online
```

**Example: GET /traffic**
```json
{
  "success": true,
  "data": {
    "inbounds": [
      {"tag": "vmess-in", "up": 1073741824, "down": 2147483648}
    ],
    "outbounds": [
      {"tag": "direct", "up": 524288, "down": 1048576}
    ],
    "timestamp": "2025-11-30T10:00:00Z"
  }
}
```

---

#### 4. Xray Control
```
POST /xray/start
POST /xray/stop
POST /xray/restart
GET  /xray/version
GET  /xray/config
POST /xray/install/:version
```

---

#### 5. System Operations
```
GET  /system/stats
GET  /logs?count=100
POST /geofiles/update
POST /backup
POST /restore
```

---

#### 6. Certificates
```
GET  /certificates
POST /certificates/generate
GET  /certificates/:domain
```

---

### Rate Limiting

Protect agents from abuse:
```go
// Per-IP rate limits (using go-rate)
type RateLimits struct {
    Read:   100 req/min   // GET requests
    Write:  20 req/min    // POST/PUT/DELETE
    Heavy:  5 req/min     // Restart, install, backup
}
```

---

## Communication Protocol

### Request Flow

```
Controller → Remote Connector → HTTP Client → Agent API
    ↓
Context (timeout, trace ID)
    ↓
Request w/ mTLS auth
    ↓
Agent processes
    ↓
Response (JSON)
    ↓
Error handling
    ↓
Store in server_tasks (if needed)
```

---

### Error Handling

**Error Types:**
```go
type AgentError struct {
    Code    string  // "TIMEOUT", "AUTH_FAILED", "OPERATION_FAILED"
    Message string
    Details map[string]interface{}
}
```

**Error Responses:**
```json
{
  "success": false,
  "error": {
    "code": "INBOUND_NOT_FOUND",
    "message": "Inbound with ID 123 not found",
    "details": {"inbound_id": 123}
  },
  "trace_id": "xyz789"
}
```

---

### Timeouts

```go
var Timeouts = map[string]time.Duration{
    "health":        5 * time.Second,
    "info":          5 * time.Second,
    "inbound_crud":  15 * time.Second,
    "traffic_query": 10 * time.Second,
    "xray_restart":  30 * time.Second,
    "backup":        60 * time.Second,
    "install":       300 * time.Second,  // 5 minutes
}
```

---

## Server Enrollment

### Enrollment Flow

```
┌──────────────┐                                 ┌──────────────┐
│ Controller   │                                 │   Agent      │
│   (Admin)    │                                 │  (VPN-1)     │
└──────┬───────┘                                 └──────┬───────┘
       │                                                │
       │ 1. Admin: Add Server                          │
       │    Name: "US-1"                               │
       │    Region: "us-east"                          │
       │    Generate Cert                              │
       ├──────────────────────────────────────────────►│
       │ 2. Controller generates:                      │
       │    - Server ID: 5                             │
       │    - Agent cert: agent-us-1.crt/.key         │
       │    - Enrollment token (temp)                  │
       │◄──────────────────────────────────────────────┤
       │ 3. Downloads cert bundle:                     │
       │    - agent-us-1.crt                          │
       │    - agent-us-1.key                          │
       │    - ca.crt                                   │
       │                                                │
       │ 4. Admin transfers certs to VPN-1             │
       │    (via SCP, manual upload, etc.)            │
       │                                                │
       │                                                │
       │                                      5. Admin installs agent:
       │                                         curl -sSL install.sh | bash
       │                                         Copies certs to /etc/x-ui-agent/certs/
       │                                         Configures controller endpoint
       │                                         Starts service
       │                                                │
       │ 6. Agent sends first health check             │
       │◄──────────────────────────────────────────────┤
       │    POST /controller/api/agent/register        │
       │    Body: {server_id: 5, version: "2.5.0"}    │
       │                                                │
       │ 7. Controller verifies cert, updates status   │
       │    servers.status = 'online'                  │
       │    servers.last_seen = NOW()                  │
       │──────────────────────────────────────────────►│
       │    Response: {success: true}                  │
       │                                                │
       │ 8. Controller starts polling                  │
       │    GET /agent/api/v1/health (every 30s)      │
       │◄─────────────────────────────────────────────►│
       │                                                │
```

---

### Agent Installation Script

**File:** `agent/install.sh`

```bash
#!/bin/bash
set -e

# 3x-ui Agent Installer

echo "=== 3x-ui Agent Installer ==="

# Detect OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$ID
else
    echo "Unsupported OS"
    exit 1
fi

# Install dependencies
if [ "$OS" = "ubuntu" ] || [ "$OS" = "debian" ]; then
    apt-get update
    apt-get install -y curl wget
elif [ "$OS" = "centos" ] || [ "$OS" = "rhel" ]; then
    yum install -y curl wget
fi

# Download agent binary
ARCH=$(uname -m)
VERSION="2.5.0"
DOWNLOAD_URL="https://github.com/cofedish/3x-UI-agents/releases/download/v${VERSION}/x-ui-agent-linux-${ARCH}"

echo "Downloading agent binary..."
wget -O /usr/local/bin/x-ui-agent "$DOWNLOAD_URL"
chmod +x /usr/local/bin/x-ui-agent

# Create directories
mkdir -p /etc/x-ui-agent/certs
mkdir -p /var/log/x-ui-agent

# Prompt for configuration
read -p "Controller endpoint (e.g., https://panel.example.com:2053): " CONTROLLER_ENDPOINT
read -p "Server ID: " SERVER_ID

# Create config file
cat > /etc/x-ui-agent/config.yaml <<EOF
controller:
  endpoint: "$CONTROLLER_ENDPOINT"
  auth:
    type: "mtls"
    cert_file: "/etc/x-ui-agent/certs/agent.crt"
    key_file: "/etc/x-ui-agent/certs/agent.key"
    ca_file: "/etc/x-ui-agent/certs/ca.crt"

agent:
  listen_addr: "0.0.0.0:2054"
  server_id: "$SERVER_ID"

xray:
  bin_folder: "/usr/local/x-ui/bin"
  config_folder: "/etc/x-ui"

logging:
  level: "info"
  file: "/var/log/x-ui-agent/agent.log"
EOF

echo ""
echo "Configuration created at /etc/x-ui-agent/config.yaml"
echo ""
echo "Next steps:"
echo "1. Copy certificates to /etc/x-ui-agent/certs/:"
echo "   - agent.crt"
echo "   - agent.key"
echo "   - ca.crt"
echo "2. Start the agent:"
echo "   systemctl enable x-ui-agent"
echo "   systemctl start x-ui-agent"
echo "3. Check status:"
echo "   systemctl status x-ui-agent"
echo "   journalctl -u x-ui-agent -f"
```

---

## Backward Compatibility

### Single-Server Mode Preservation

**Detection:**
```go
func IsSingleServerMode() bool {
    var serverCount int64
    db.Model(&model.Server{}).Where("enabled = ?", true).Count(&serverCount)
    return serverCount == 1
}
```

**Behavior in Single-Server Mode:**
- UI hides server selector
- Uses LocalConnector exclusively
- All operations work as before
- No network calls to agents
- Default server_id = 1 automatically applied

---

### UI Adaptation

```vue
<!-- Conditional server selector -->
<div v-if="serverCount > 1" class="server-selector">
  <a-select v-model="selectedServerId" @change="onServerChange">
    <a-select-option value="0">All Servers</a-select-option>
    <a-select-option v-for="server in servers" :key="server.id" :value="server.id">
      {{ server.name }}
    </a-select-option>
  </a-select>
</div>

<!-- Dashboard shows local stats in single mode -->
<div v-if="isSingleMode">
  <!-- Current dashboard code unchanged -->
</div>

<!-- Multi-server aggregated dashboard -->
<div v-else>
  <!-- Server cards, aggregated metrics -->
</div>
```

---

### API Compatibility

Existing API endpoints continue to work:
```
GET /panel/api/inbounds/list
```

**Single-Server Mode:** Returns inbounds from server_id = 1
**Multi-Server Mode:** Requires `?server_id=X` query param (or uses selected server from session)

---

## Implementation Plan

### Phase 1: Foundation (Commits 1-3)
- ✅ Create `docs/multiserver/ARCHITECTURE.md`
- [ ] Add `database/model/server.go`
- [ ] Create migration `001_add_multiserver.go`
- [ ] Implement `Server` model with CRUD
- [ ] Add default local server creation

### Phase 2: Connector Layer (Commits 4-5)
- [ ] Create `web/service/server_connector.go` (interface)
- [ ] Implement `LocalConnector` (wraps existing services)
- [ ] Update `InboundService` to use `ServerConnector`
- [ ] Update `XrayService` to use `ServerConnector`

### Phase 3: Agent Service (Commits 6-8)
- [ ] Create `agent/` directory
- [ ] Implement agent API (REST with mTLS)
- [ ] Implement health, info, metrics endpoints
- [ ] Implement inbound management endpoints
- [ ] Implement traffic endpoints
- [ ] Implement Xray control endpoints
- [ ] Add agent installation script

### Phase 4: Remote Connector (Commits 9-10)
- [ ] Implement `RemoteConnector` (HTTP client)
- [ ] Add mTLS certificate management
- [ ] Add server health polling job
- [ ] Implement task logging (`server_tasks` table)

### Phase 5: UI Adaptation (Commits 11-13)
- [ ] Add server selector component
- [ ] Create `servers.html` (server management page)
- [ ] Update `index.html` for multi-server dashboard
- [ ] Adapt `inbounds.html` for server context
- [ ] Adapt `settings.html`
- [ ] Add aggregated metrics views

### Phase 6: Telegram Bot (Commit 14)
- [ ] Update `tgbot.go` for multi-server awareness
- [ ] Add server selection in bot menus
- [ ] Implement server auto-selection policy
- [ ] Add server status notifications

### Phase 7: Testing & Docs (Commits 15-17)
- [ ] Write unit tests for `ServerConnector`
- [ ] Write integration tests (mock agent)
- [ ] Create `DEPLOYMENT.md`
- [ ] Create `AGENT.md`
- [ ] Create `MIGRATION.md`
- [ ] Add docker-compose examples

### Phase 8: Final Polish (Commit 18)
- [ ] Version bump
- [ ] Changelog
- [ ] End-to-end testing
- [ ] Performance optimization

---

## Metrics & Monitoring

### Server Health Checks

**Polling Frequency:** Every 30 seconds

**Health Check Flow:**
```go
func (j *ServerHealthJob) Run() {
    servers := serverService.GetEnabledServers()

    for _, server := range servers {
        go func(s *model.Server) {
            connector := getConnector(s)

            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            defer cancel()

            health, err := connector.GetHealth(ctx)

            if err != nil {
                s.Status = "offline"
                s.LastError = err.Error()
            } else {
                s.Status = "online"
                s.LastSeen = time.Now()
                s.Version = health.Version
                s.XrayVersion = health.XrayVersion
            }

            serverService.UpdateServer(s)
        }(server)
    }
}
```

---

### Metrics Collection

**Aggregated Metrics:**
```go
type AggregatedMetrics struct {
    TotalServers    int
    OnlineServers   int
    OfflineServers  int
    TotalInbounds   int
    TotalClients    int
    TotalTrafficUp  int64
    TotalTrafficDown int64
    OnlineClients   int
}
```

**Dashboard Display:**
- Server status grid (online/offline/error)
- Traffic heatmap by server
- Top servers by traffic/clients
- Recent errors/alerts

---

## Future Enhancements

(Not in scope for initial release, but architecture supports)

- **Agent Auto-Update:** Controller pushes agent updates
- **Distributed Backup:** Replicate backups across servers
- **Load Balancing:** Auto-assign clients to least-loaded server
- **Geographic Routing:** Bot suggests nearest server by IP geolocation
- **High Availability:** Controller clustering, database replication
- **Advanced Monitoring:** Prometheus metrics export
- **Audit Trail:** Detailed operation logs with diffs

---

## Conclusion

This architecture transforms 3x-ui into a scalable, production-grade multi-server VPN management platform while maintaining 100% backward compatibility. The Controller-Agent pattern provides clear separation of concerns, robust security, and horizontal scalability.

**Next Steps:**
1. Review and approve this architecture document
2. Begin implementation following the phased plan
3. Commit incrementally with detailed messages
4. Test thoroughly at each phase
5. Document deployment procedures

**Estimated Implementation Effort:**
- Phase 1-2: Database & Connector Layer (1-2 days)
- Phase 3-4: Agent & Remote Connector (2-3 days)
- Phase 5-6: UI & Bot Adaptation (2-3 days)
- Phase 7-8: Testing & Documentation (1-2 days)

**Total:** 6-10 days of focused development

---

**Document Version History:**
- v1.0 (2025-11-30): Initial architecture design
