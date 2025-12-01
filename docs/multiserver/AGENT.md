# 3x-ui Agent Configuration Guide

## Overview

The 3x-ui agent is a lightweight service that runs on each managed VPN server, exposing a secure HTTPS API for the controller to manage Xray inbounds, clients, and system operations.

**Key Features:**
- **Production-Grade mTLS**: Mutual TLS authentication with client certificates (TLS 1.3)
- **Secure API**: RESTful HTTP API with rate limiting and request logging
- **Xray Management**: Full inbound/client lifecycle management
- **System Monitoring**: CPU, memory, disk, network stats
- **Log Access**: Secure, restricted log file reading
- **Health Checks**: Status reporting for monitoring

---

## Table of Contents
1. [Quick Start](#quick-start)
2. [Authentication Methods](#authentication-methods)
3. [mTLS Configuration (Recommended)](#mtls-configuration-recommended)
4. [JWT Configuration (Not Recommended)](#jwt-configuration-not-recommended)
5. [Environment Variables](#environment-variables)
6. [Security Best Practices](#security-best-practices)
7. [Troubleshooting](#troubleshooting)
8. [API Reference](#api-reference)

---

## Quick Start

### Installation

```bash
# Download and install agent binary
wget https://github.com/cofedish/3xui-agents/releases/latest/download/x-ui-linux-amd64.tar.gz
tar -xzf x-ui-linux-amd64.tar.gz
sudo mv x-ui /usr/local/bin/
sudo chmod +x /usr/local/bin/x-ui
```

### Generate Certificates (on secure workstation)

```bash
# Clone repository
git clone https://github.com/cofedish/3xui-agents.git
cd 3xui-agents/scripts/certs

# Generate CA (once)
./gen-ca.sh

# Generate agent certificate
./gen-agent-cert.sh agent-01

# Generate controller certificate
./gen-controller-cert.sh
```

### Deploy Certificates to Agent

```bash
# Create directory
sudo mkdir -p /etc/x-ui-agent/certs

# Copy certificates to agent server
scp ca.crt agent-01.crt agent-01.key user@agent-server:/tmp/

# On agent server, move to proper location
sudo mv /tmp/agent-01.crt /etc/x-ui-agent/certs/agent.crt
sudo mv /tmp/agent-01.key /etc/x-ui-agent/certs/agent.key
sudo mv /tmp/ca.crt /etc/x-ui-agent/certs/ca.crt

# Set secure permissions
sudo chmod 600 /etc/x-ui-agent/certs/agent.key
sudo chmod 644 /etc/x-ui-agent/certs/*.crt
```

### Configure Agent

```bash
# Copy environment template
sudo cp deploy/agent/agent.env.example /etc/x-ui-agent/.env

# Edit configuration
sudo nano /etc/x-ui-agent/.env
```

**Minimal configuration:**
```bash
AGENT_SERVER_ID=agent-01
AGENT_SERVER_NAME=Production Agent 01
AGENT_AUTH_TYPE=mtls
AGENT_CERT_FILE=/etc/x-ui-agent/certs/agent.crt
AGENT_KEY_FILE=/etc/x-ui-agent/certs/agent.key
AGENT_CA_FILE=/etc/x-ui-agent/certs/ca.crt
```

### Start Agent

```bash
# Load environment variables
source /etc/x-ui-agent/.env

# Start agent
sudo x-ui agent

# Or run as systemd service (see below)
```

---

## Authentication Methods

### Comparison

| Feature | mTLS | JWT |
|---------|------|-----|
| Security Level | **Excellent** | Good |
| Certificate Management | Required | Not required |
| Mutual Authentication | ✅ Yes | ❌ No |
| Token Theft Protection | ✅ Yes | ❌ No |
| Replay Attack Protection | ✅ Yes | ⚠️ Limited |
| TLS Version | 1.3 (enforced) | 1.3 (enforced) |
| Certificate Rotation | Yes (annually) | N/A |
| Setup Complexity | Medium | Low |
| Production Recommended | **✅ Yes** | ❌ No |

### Recommendation

**Always use mTLS in production.** JWT is provided for testing/development only.

---

## mTLS Configuration (Recommended)

### What is mTLS?

Mutual TLS (mTLS) provides **bidirectional authentication**:
1. **Agent authenticates controller**: Verifies controller presents valid client certificate
2. **Controller authenticates agent**: Verifies agent presents valid server certificate
3. **Both signed by trusted CA**: Prevents unauthorized access

### Certificate Architecture

```
┌─────────────┐
│   CA Root   │  <- Generated once, signs all certs
│   ca.crt    │     KEEP ca.key SECURE!
│   ca.key    │
└──────┬──────┘
       │
       ├──────────────────┬──────────────────┐
       │                  │                  │
       ▼                  ▼                  ▼
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│  Agent 01   │    │  Agent 02   │    │ Controller  │
│   (Server)  │    │   (Server)  │    │   (Client)  │
│             │    │             │    │             │
│ agent-01.crt│    │ agent-02.crt│    │controller.crt│
│ agent-01.key│    │ agent-02.key│    │controller.key│
└─────────────┘    └─────────────┘    └─────────────┘
```

### Step-by-Step Setup

#### 1. Generate CA (Once)

```bash
cd scripts/certs
./gen-ca.sh

# Output:
# - ca.key (KEEP SECURE - offline storage recommended)
# - ca.crt (distribute to all agents and controller)
```

**⚠️ CRITICAL:** Store `ca.key` securely. Anyone with this key can generate valid certificates!

#### 2. Generate Agent Certificate (Per Agent)

```bash
./gen-agent-cert.sh agent-01

# With IP SAN (if accessing by IP)
./gen-agent-cert.sh agent-01 ./certs ./certs 192.168.1.100

# With DNS SAN (if accessing by hostname)
./gen-agent-cert.sh agent-01 ./certs ./certs agent.example.com

# Output:
# - agent-agent-01.key (private key)
# - agent-agent-01.crt (certificate)
```

#### 3. Generate Controller Certificate (Once)

```bash
./gen-controller-cert.sh

# Output:
# - controller.key (private key)
# - controller.crt (certificate)
```

#### 4. Deploy to Agent

```bash
# On agent server
sudo mkdir -p /etc/x-ui-agent/certs

# Copy files (from workstation to agent)
scp ca.crt agent-agent-01.crt agent-agent-01.key user@agent:/tmp/

# On agent server
sudo mv /tmp/agent-agent-01.crt /etc/x-ui-agent/certs/agent.crt
sudo mv /tmp/agent-agent-01.key /etc/x-ui-agent/certs/agent.key
sudo mv /tmp/ca.crt /etc/x-ui-agent/certs/ca.crt

# Secure permissions
sudo chmod 600 /etc/x-ui-agent/certs/agent.key
sudo chmod 644 /etc/x-ui-agent/certs/*.crt
```

#### 5. Configure Agent

Edit `/etc/x-ui-agent/.env`:

```bash
AGENT_AUTH_TYPE=mtls
AGENT_CERT_FILE=/etc/x-ui-agent/certs/agent.crt
AGENT_KEY_FILE=/etc/x-ui-agent/certs/agent.key
AGENT_CA_FILE=/etc/x-ui-agent/certs/ca.crt
```

#### 6. Start Agent

```bash
source /etc/x-ui-agent/.env
sudo x-ui agent
```

Expected log output:
```
=== Starting 3x-ui Agent ===
Agent ID: agent-01
Listen Address: 0.0.0.0:2054
Auth Type: mtls
Starting agent API server...
Starting mTLS server (TLS 1.3 + client certificate required)...
```

#### 7. Test mTLS Connection

```bash
cd scripts/certs
./test-mtls.sh https://agent-server:2054

# Or manual test:
curl --cert controller.crt --key controller.key --cacert ca.crt \
  https://agent-server:2054/api/v1/health
```

Expected response:
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "xray_running": true
  }
}
```

### Certificate Rotation

Certificates should be rotated before expiration (default: 1 year).

**Rotation Process:**
1. Generate new certificates with same CA
2. Deploy new certificates to agent server (don't delete old ones yet)
3. Restart agent with new certificates
4. Verify connection
5. Remove old certificates

```bash
# Generate new cert
./gen-agent-cert.sh agent-01-renewed

# Deploy
scp agent-agent-01-renewed.crt agent-agent-01-renewed.key user@agent:/tmp/
sudo mv /tmp/agent-agent-01-renewed.crt /etc/x-ui-agent/certs/agent.crt
sudo mv /tmp/agent-agent-01-renewed.key /etc/x-ui-agent/certs/agent.key

# Restart agent
sudo systemctl restart x-ui-agent

# Verify
curl --cert controller.crt --key controller.key --cacert ca.crt \
  https://agent-server:2054/api/v1/health
```

---

## JWT Configuration (Not Recommended)

⚠️ **JWT is NOT recommended for production.** It provides only static token authentication without certificate-based mutual trust.

### When to Use JWT

- Development/testing environments
- Internal networks with strict firewall rules
- Environments where certificate management is impractical

### Setup

```bash
# Generate a strong secret (32+ characters)
export AGENT_JWT_SECRET=$(openssl rand -hex 32)

# Configure agent
echo "AGENT_AUTH_TYPE=jwt" >> /etc/x-ui-agent/.env
echo "AGENT_JWT_SECRET=$AGENT_JWT_SECRET" >> /etc/x-ui-agent/.env

# Start agent
source /etc/x-ui-agent/.env
sudo x-ui agent
```

### Security Limitations

- ❌ No mutual authentication
- ❌ Token can be intercepted and reused
- ❌ No certificate rotation
- ❌ Single point of failure (one token for all agents)

**Use mTLS instead for production deployments.**

---

## Environment Variables

See [deploy/agent/agent.env.example](../../deploy/agent/agent.env.example) for complete reference.

### Required Variables

```bash
AGENT_SERVER_ID       # Unique agent identifier
AGENT_AUTH_TYPE       # "mtls" or "jwt"
```

### mTLS Variables (when AGENT_AUTH_TYPE=mtls)

```bash
AGENT_CERT_FILE       # Path to agent server certificate
AGENT_KEY_FILE        # Path to agent private key
AGENT_CA_FILE         # Path to CA certificate
```

### Optional Variables

```bash
AGENT_LISTEN_ADDR     # Default: 0.0.0.0:2054
AGENT_SERVER_NAME     # Human-readable name
AGENT_TAGS            # Comma-separated tags
AGENT_LOG_LEVEL       # debug, info, warning, error
AGENT_RATE_LIMIT      # Requests per minute (default: 100)
```

---

## Security Best Practices

### File Permissions

```bash
# Private keys: read-only by owner
chmod 600 /etc/x-ui-agent/certs/*.key

# Certificates: readable by all
chmod 644 /etc/x-ui-agent/certs/*.crt

# Configuration file
chmod 600 /etc/x-ui-agent/.env
```

### Network Security

```bash
# Firewall: Allow only controller IP
sudo ufw allow from <controller-ip> to any port 2054 proto tcp

# Or use iptables
sudo iptables -A INPUT -p tcp -s <controller-ip> --dport 2054 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 2054 -j DROP
```

### Certificate Storage

- **ca.key**: Store offline, encrypted, with access control
- **agent.key**: Keep on agent server only, chmod 600
- **ca.crt**: Can be distributed freely (public)

### Monitoring

```bash
# Check agent logs for authentication failures
tail -f /var/log/x-ui-agent/agent.log | grep "authentication\|certificate"

# Monitor failed connection attempts
journalctl -u x-ui-agent -f | grep "Unauthorized\|CERT"
```

---

## Troubleshooting

### Issue: Connection Refused

**Symptoms:**
```
curl: (7) Failed to connect to agent-server port 2054: Connection refused
```

**Solutions:**
1. Verify agent is running: `sudo systemctl status x-ui-agent`
2. Check firewall: `sudo ufw status`
3. Verify listening port: `sudo netstat -tlnp | grep 2054`

### Issue: Certificate Verification Failed

**Symptoms:**
```
Client certificate verification failed
```

**Solutions:**
1. Verify CA is the same on both sides:
   ```bash
   md5sum /etc/x-ui-agent/certs/ca.crt
   md5sum /etc/x-ui/certs/ca.crt
   ```

2. Check certificate validity:
   ```bash
   openssl x509 -in /etc/x-ui-agent/certs/agent.crt -noout -dates
   ```

3. Verify certificate chain:
   ```bash
   openssl verify -CAfile /etc/x-ui-agent/certs/ca.crt /etc/x-ui-agent/certs/agent.crt
   ```

### Issue: TLS Handshake Timeout

**Symptoms:**
```
TLS handshake timeout
```

**Solutions:**
1. Check network latency: `ping agent-server`
2. Verify TLS configuration is compatible
3. Check for MTU issues (especially with VPNs)

### Issue: Log Reading Permission Denied

**Symptoms:**
```
Unable to read logs
```

**Solutions:**
1. Verify AGENT_LOG_FILE path is in allowlist
2. Check file permissions: `ls -l /var/log/x-ui-agent/agent.log`
3. Ensure agent process has read permission

---

## API Reference

### Base URL

```
https://<agent-endpoint>:2054/api/v1
```

### Authentication

All endpoints (except `/health`) require mTLS client certificate or JWT Bearer token.

### Endpoints

#### Health Check (Public)

```bash
GET /health
```

Response:
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "xray_running": true,
    "uptime": 3600
  }
}
```

#### Server Info (Protected)

```bash
GET /info
```

Response:
```json
{
  "success": true,
  "data": {
    "server_id": "agent-01",
    "hostname": "vpn-server-1",
    "os": "linux",
    "arch": "amd64",
    "version": "2.0.0"
  }
}
```

#### List Inbounds

```bash
GET /inbounds
```

#### Add Inbound

```bash
POST /inbounds
Content-Type: application/json

{
  "remark": "MyInbound",
  "port": 443,
  "protocol": "vless",
  ...
}
```

#### Xray Control

```bash
POST /xray/start
POST /xray/stop
POST /xray/restart
GET /xray/version
```

#### System Stats

```bash
GET /system/stats
```

Response:
```json
{
  "cpu_percent": 15.2,
  "memory_percent": 45.8,
  "disk_percent": 60.1,
  "network_rx": 1048576,
  "network_tx": 2097152
}
```

#### Logs

```bash
GET /logs?count=100
```

For complete API documentation, see [API.md](./API.md).

---

## Systemd Service

Create `/etc/systemd/system/x-ui-agent.service`:

```ini
[Unit]
Description=3x-ui Agent Service
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/etc/x-ui-agent
EnvironmentFile=/etc/x-ui-agent/.env
ExecStart=/usr/local/bin/x-ui agent
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl daemon-reload
sudo systemctl enable x-ui-agent
sudo systemctl start x-ui-agent
sudo systemctl status x-ui-agent
```

---

## Next Steps

1. ✅ Configure agent with mTLS
2. ✅ Test connection with test-mtls.sh
3. → [Add agent to controller](./DEPLOYMENT.md#adding-remote-agents)
4. → Monitor agent logs and health status
5. → Configure periodic certificate rotation reminders
