# 3x-ui Multi-Server Deployment Guide

This guide covers deploying 3x-ui in a multi-server architecture with a central controller panel and multiple remote agents.

## Architecture Overview

```
┌─────────────────┐
│   Controller    │  ← Web Panel (Port 2053)
│   (Panel)       │  ← Manages all agents
└────────┬────────┘
         │
    ┌────┴────┬──────────┐
    │         │          │
┌───▼───┐ ┌───▼───┐ ┌───▼───┐
│Agent 1│ │Agent 2│ │Agent N│  ← Remote Xray servers (Port 2054)
│ mTLS  │ │ mTLS  │ │ mTLS  │  ← Managed by controller
└───────┘ └───────┘ └───────┘
```

## Prerequisites

- **Controller Server**: Ubuntu/Debian/CentOS, 1GB+ RAM, sudo access
- **Agent Servers**: Ubuntu/Debian/CentOS, 512MB+ RAM, sudo access
- **Network**: Controller must reach agents on port 2054
- **Certificates**: mTLS certificates (generated during setup)

## Quick Start

### 1. Deploy Controller (Panel)

```bash
# Install controller with standard installation
bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/master/install.sh)

# Access panel at: http://your-server-ip:2053
```

### 2. Generate mTLS Certificates

On your local machine (with OpenSSL installed):

```bash
# Clone repository
git clone https://github.com/cofedish/3x-UI-agents.git
cd 3xui-agents/3x-ui/deploy/certs

# Generate CA certificate (once)
openssl genrsa -out ca/ca.key 4096
openssl req -x509 -new -nodes -key ca/ca.key -sha256 -days 3650 \
  -out ca/ca.crt -subj "/CN=3x-ui-ca"

# Generate controller client certificate
openssl genrsa -out controller/controller.key 2048
openssl req -new -key controller/controller.key \
  -out controller/controller.csr -subj "/CN=3x-ui-controller"

cat > controller/controller.ext <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = clientAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
EOF

openssl x509 -req -in controller/controller.csr \
  -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial \
  -out controller/controller.crt -days 365 \
  -extfile controller/controller.ext

# Generate agent server certificate for each agent
# Replace AGENT_NAME and AGENT_IP with actual values
AGENT_NAME="agent-server-1"
AGENT_IP="198.51.100.10"

mkdir -p agent-${AGENT_NAME}
openssl genrsa -out agent-${AGENT_NAME}/agent.key 2048
openssl req -new -key agent-${AGENT_NAME}/agent.key \
  -out agent-${AGENT_NAME}/agent.csr -subj "/CN=${AGENT_NAME}"

cat > agent-${AGENT_NAME}/agent.ext <<EOF
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth
subjectKeyIdentifier = hash
authorityKeyIdentifier = keyid,issuer
subjectAltName = @alt_names

[alt_names]
IP.1 = ${AGENT_IP}
DNS.1 = ${AGENT_NAME}
EOF

openssl x509 -req -in agent-${AGENT_NAME}/agent.csr \
  -CA ca/ca.crt -CAkey ca/ca.key -CAcreateserial \
  -out agent-${AGENT_NAME}/agent.crt -days 365 \
  -extfile agent-${AGENT_NAME}/agent.ext

# Verify certificates
openssl verify -CAfile ca/ca.crt controller/controller.crt
openssl verify -CAfile ca/ca.crt agent-${AGENT_NAME}/agent.crt
```

**Windows Users**: Use the PowerShell script:
```powershell
cd deploy
powershell -ExecutionPolicy Bypass -File gen-prod-certs.ps1
```

### 3. Deploy Agents

For each remote agent server:

```bash
# Download and install agent
curl -sSL https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/scripts/agent/install.sh | sudo bash

# Or see scripts/agent/README.md for detailed manual installation
```

Copy certificates to agent:

```bash
# From your local machine:
AGENT_HOST="agent-server-1"  # SSH hostname
scp -r agent-${AGENT_HOST}/* root@${AGENT_HOST}:/etc/x-ui-agent/certs/
scp ca/ca.crt root@${AGENT_HOST}:/etc/x-ui-agent/certs/

# Set permissions on agent
ssh root@${AGENT_HOST} "chmod 600 /etc/x-ui-agent/certs/agent.key"
ssh root@${AGENT_HOST} "chmod 644 /etc/x-ui-agent/certs/*.crt"
```

Configure agent systemd service:

```bash
# On agent server:
cat > /etc/systemd/system/x-ui-agent.service <<'EOF'
[Unit]
Description=3x-ui Agent
After=network.target
Wants=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/x-ui-agent
ExecStart=/usr/local/bin/x-ui-agent
Environment="AGENT_SERVER_ID=agent-server-1"
Environment="AGENT_SERVER_NAME=Agent Server 1"
Environment="AGENT_LISTEN_ADDR=0.0.0.0:2054"
Environment="AGENT_AUTH_TYPE=mtls"
Environment="AGENT_CERT_FILE=/etc/x-ui-agent/certs/agent.crt"
Environment="AGENT_KEY_FILE=/etc/x-ui-agent/certs/agent.key"
Environment="AGENT_CA_FILE=/etc/x-ui-agent/certs/ca.crt"
Environment="AGENT_LOG_LEVEL=info"
Restart=on-failure
RestartSec=10s
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
EOF

# Enable and start
systemctl daemon-reload
systemctl enable x-ui-agent
systemctl start x-ui-agent
systemctl status x-ui-agent
```

### 4. Register Agents in Controller Panel

1. Access controller panel: `http://controller-ip:2053`
2. Login with default credentials (see install script output)
3. Navigate to **Servers** page
4. Click **Add Server**
5. Configure:
   - **Name**: Agent Server 1
   - **Host**: `https://agent-ip:2054`
   - **Auth Type**: mTLS
   - **Upload Certificates**:
     - Client Cert: `controller/controller.crt`
     - Client Key: `controller/controller.key`
     - CA Cert: `ca/ca.crt`
6. Click **Test Connection** → Should show "✓ Connected"
7. Click **Save**

### 5. Verify Setup

```bash
# Check agent status
curl -k https://agent-ip:2054/api/v1/health
# Expected: {"status":"ok"}

# Check authenticated endpoint (from controller)
curl -k --cert controller.crt --key controller.key --cacert ca.crt \
  https://agent-ip:2054/api/v1/info
# Expected: {"serverId":"agent-server-1",...}
```

## Production Deployment

### Security Hardening

1. **Firewall Configuration**

```bash
# On agent servers - ONLY allow controller IP
ufw allow from <controller-ip> to any port 2054 proto tcp
ufw deny 2054
ufw enable

# Verify
ufw status
```

2. **Certificate Rotation**

Rotate certificates every 90 days:

```bash
# Re-run certificate generation steps
# Transfer new certs to servers
# Restart services: systemctl restart x-ui-agent
```

3. **Strong Credentials**

Change default panel credentials immediately after installation.

4. **HTTPS for Panel**

Configure TLS certificate for controller web panel (Port 2053).

### Monitoring

Monitor agent health:

```bash
# Check service status
systemctl status x-ui-agent

# View logs
journalctl -u x-ui-agent -f

# Check connectivity
curl -k https://agent-ip:2054/api/v1/health
```

### Backup

Critical files to backup:

**Controller:**
- Database: `/etc/x-ui/x-ui.db`
- Certificates: `controller/*.crt`, `controller/*.key`, `ca/ca.crt`

**Agents:**
- Certificates: `/etc/x-ui-agent/certs/`
- Configuration: `/etc/systemd/system/x-ui-agent.service`

### Upgrading

**Controller:**
```bash
bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/master/install.sh)
```

**Agents:**
```bash
# Download new version
VERSION="v2.1.0"
wget https://github.com/cofedish/3x-UI-agents/releases/download/$VERSION/x-ui-linux-amd64
sudo systemctl stop x-ui-agent
sudo mv x-ui-linux-amd64 /usr/local/bin/x-ui-agent
sudo chmod +x /usr/local/bin/x-ui-agent
sudo systemctl start x-ui-agent
```

## Troubleshooting

### Agent Can't Connect

```bash
# Check if agent is running
systemctl status x-ui-agent

# Check if listening on port
ss -tlnp | grep 2054

# Check firewall
ufw status
iptables -L -n | grep 2054

# Test TLS connection
openssl s_client -connect agent-ip:2054 \
  -cert controller.crt -key controller.key -CAfile ca.crt
```

### Certificate Errors

```bash
# Verify certificate chain
openssl verify -CAfile ca/ca.crt agent-server-1/agent.crt

# Check certificate expiry
openssl x509 -in agent-server-1/agent.crt -noout -dates

# Check certificate details
openssl x509 -in agent-server-1/agent.crt -noout -text
```

### Panel Can't Reach Agent

1. Check agent is running: `systemctl status x-ui-agent`
2. Check firewall allows controller IP
3. Verify certificates are correct
4. Check network connectivity: `ping agent-ip`
5. Test port: `telnet agent-ip 2054`

## Architecture Details

- **Controller**: Main web panel, manages all agents, stores configuration
- **Agents**: Remote servers running Xray, accept commands from controller via mTLS API
- **Communication**: All communication over HTTPS with mutual TLS authentication
- **Port 2053**: Controller web panel (HTTP/HTTPS)
- **Port 2054**: Agent API (HTTPS with mTLS)

## Documentation

- **Agent Installation**: `scripts/agent/README.md`
- **Certificate Management**: `scripts/certs/README.md`
- **API Documentation**: `docs/multiserver/ARCHITECTURE.md`
- **Main README**: `README.md`

## Support

- **Issues**: https://github.com/cofedish/3x-UI-agents/issues
- **Discussions**: https://github.com/cofedish/3x-UI-agents/discussions

## License

GPL-3.0 - See LICENSE file
