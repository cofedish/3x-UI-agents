# 3x-ui Agent Installation Guide

## Quick Install

```bash
# Download and run installer
curl -sSL https://raw.githubusercontent.com/MHSanaei/3x-ui/main/scripts/agent/install.sh | sudo bash
```

## Manual Installation

### 1. Download Agent Binary

```bash
# Set version (or use 'latest')
VERSION="v2.0.0"

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64) ARCH="amd64" ;;
  aarch64) ARCH="arm64" ;;
  armv7l) ARCH="armv7" ;;
esac

# Download
wget https://github.com/MHSanaei/3x-ui/releases/download/$VERSION/x-ui-linux-$ARCH
chmod +x x-ui-linux-$ARCH
sudo mv x-ui-linux-$ARCH /usr/local/bin/x-ui-agent
```

### 2. Create Directories

```bash
sudo mkdir -p /etc/x-ui-agent/certs
sudo mkdir -p /var/log/x-ui-agent
sudo chmod 700 /etc/x-ui-agent/certs
```

### 3. Copy Certificates

Transfer certificates from controller to agent server:

```bash
# On controller, generate agent certificate:
./x-ui cert-authority issue --name vpn-us-1 --output /tmp/vpn-us-1-certs/

# Transfer to agent server (use scp or manual copy):
scp /tmp/vpn-us-1-certs/* root@vpn-us-1:/etc/x-ui-agent/certs/

# On agent server, set permissions:
sudo chmod 600 /etc/x-ui-agent/certs/agent.key
sudo chmod 644 /etc/x-ui-agent/certs/agent.crt
sudo chmod 644 /etc/x-ui-agent/certs/ca.crt
```

### 4. Install Systemd Service

```bash
# Copy service file
sudo cp deploy/systemd/x-ui-agent.service /etc/systemd/system/

# Edit environment variables
sudo nano /etc/systemd/system/x-ui-agent.service

# Reload systemd
sudo systemctl daemon-reload

# Enable and start
sudo systemctl enable x-ui-agent
sudo systemctl start x-ui-agent
```

### 5. Verify Installation

```bash
# Check service status
sudo systemctl status x-ui-agent

# View logs
sudo journalctl -u x-ui-agent -f

# Test health endpoint
curl -k https://localhost:2054/api/v1/health
```

## Configuration

### Environment Variables

All configuration via environment variables. See `deploy/agent.env.example` for full list.

**Required:**
- `AGENT_LISTEN_ADDR` - Listen address (default: `0.0.0.0:2054`)
- `AGENT_AUTH_TYPE` - Authentication type: `mtls` or `jwt`
- `AGENT_CERT_FILE`, `AGENT_KEY_FILE`, `AGENT_CA_FILE` - Certificate paths (for mTLS)

**Optional:**
- `AGENT_SERVER_ID` - Unique server identifier
- `AGENT_TAGS` - Comma-separated tags
- `AGENT_RATE_LIMIT` - Requests per minute (default: 100)
- `AGENT_LOG_LEVEL` - Log level: debug/info/warning/error

### Firewall Configuration

**Important:** Only controller should access agent port!

```bash
# UFW
sudo ufw allow from <controller-ip> to any port 2054 proto tcp

# Firewalld
sudo firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="<controller-ip>" port protocol="tcp" port="2054" accept'
sudo firewall-cmd --reload

# iptables
sudo iptables -A INPUT -p tcp -s <controller-ip> --dport 2054 -j ACCEPT
sudo iptables -A INPUT -p tcp --dport 2054 -j DROP
```

## Certificate Management

### mTLS Certificates

1. **On Controller:** Generate CA and agent certificate
2. **Transfer:** Securely copy certs to agent server
3. **Permissions:** Ensure private key is secure (chmod 600)
4. **Rotation:** Rotate certificates regularly (recommended: every 90 days)

### Certificate Rotation

```bash
# On controller: Generate new certificate
./x-ui cert-authority issue --name vpn-us-1 --renew

# Transfer new certs to agent
scp /tmp/vpn-us-1-certs/* root@vpn-us-1:/etc/x-ui-agent/certs/

# Restart agent
sudo systemctl restart x-ui-agent
```

## Troubleshooting

### Agent Won't Start

```bash
# Check logs
sudo journalctl -u x-ui-agent -n 50

# Common issues:
# 1. Certificate paths incorrect
# 2. Certificate permissions wrong
# 3. Port already in use
# 4. Missing Xray binary
```

### Connection Refused

```bash
# Check if agent is listening
sudo ss -tlnp | grep 2054

# Check firewall
sudo ufw status
sudo iptables -L -n | grep 2054

# Test from controller
curl -k --cert /path/to/controller.crt --key /path/to/controller.key https://agent-ip:2054/api/v1/health
```

### Certificate Errors

```bash
# Verify certificate chain
openssl verify -CAfile /etc/x-ui-agent/certs/ca.crt /etc/x-ui-agent/certs/agent.crt

# Check certificate expiry
openssl x509 -in /etc/x-ui-agent/certs/agent.crt -noout -dates

# Test TLS connection
openssl s_client -connect localhost:2054 -CAfile /etc/x-ui-agent/certs/ca.crt
```

## Upgrading

```bash
# Stop agent
sudo systemctl stop x-ui-agent

# Download new version
wget https://github.com/MHSanaei/3x-ui/releases/download/v2.1.0/x-ui-linux-amd64
sudo mv x-ui-linux-amd64 /usr/local/bin/x-ui-agent
sudo chmod +x /usr/local/bin/x-ui-agent

# Start agent
sudo systemctl start x-ui-agent

# Verify version
curl -k https://localhost:2054/api/v1/info | jq .version
```

## Uninstalling

```bash
# Stop and disable service
sudo systemctl stop x-ui-agent
sudo systemctl disable x-ui-agent

# Remove files
sudo rm /usr/local/bin/x-ui-agent
sudo rm /etc/systemd/system/x-ui-agent.service
sudo rm -rf /etc/x-ui-agent
sudo rm -rf /var/log/x-ui-agent

# Reload systemd
sudo systemctl daemon-reload
```

## Security Best Practices

1. **Use mTLS** - Always prefer mTLS over JWT for production
2. **Restrict Firewall** - Only controller IP should access port 2054
3. **Rotate Certificates** - Regular rotation (every 90 days recommended)
4. **Secure Private Keys** - chmod 600, never commit to git
5. **Monitor Logs** - Watch for unauthorized access attempts
6. **Keep Updated** - Apply security patches promptly
7. **Audit Access** - Review controller access logs regularly

## API Documentation

See `docs/multiserver/ARCHITECTURE.md` for complete API specification.

### Example API Calls

```bash
# Health check (no auth required)
curl -k https://agent-ip:2054/api/v1/health

# Get server info (requires auth)
curl -k --cert controller.crt --key controller.key \
  https://agent-ip:2054/api/v1/info

# List inbounds
curl -k --cert controller.crt --key controller.key \
  https://agent-ip:2054/api/v1/inbounds

# Get traffic stats
curl -k --cert controller.crt --key controller.key \
  https://agent-ip:2054/api/v1/traffic
```

## Support

- **Issues:** https://github.com/MHSanaei/3x-ui/issues
- **Documentation:** https://github.com/MHSanaei/3x-ui/tree/main/docs
- **Community:** Telegram group

## License

Same as 3x-ui main project.
