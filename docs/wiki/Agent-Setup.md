# Agent Setup Guide

Learn how to deploy and connect agents to your control panel.

## Connecting Agents

After installing an agent, you need to register it in the control panel.

### For mTLS Authentication

1. The agent installer generates certificates automatically
2. Copy the displayed certificate bundle (3 blocks: agent.crt, agent.key, ca.crt)
3. In panel: Navigate to **Servers → Add Server**
4. Select **mTLS** authentication
5. Paste the complete certificate bundle
6. Click **Save**

### For JWT Authentication

1. The agent installer shows a JWT token
2. Copy the token
3. In panel: Navigate to **Servers → Add Server**
4. Select **JWT** authentication
5. Paste the token
6. Click **Save**

## Agent Configuration

Agent settings are configured via environment variables in `/etc/systemd/system/x-ui-agent.service`:

```ini
[Service]
Environment="AGENT_SERVER_ID=server-1"
Environment="AGENT_SERVER_NAME=US Server 1"
Environment="AGENT_LISTEN_ADDR=0.0.0.0:2054"
Environment="AGENT_AUTH_TYPE=mtls"
Environment="AGENT_CERT_FILE=/etc/x-ui/agent/certs/agent.crt"
Environment="AGENT_KEY_FILE=/etc/x-ui/agent/certs/agent.key"
Environment="AGENT_CA_FILE=/etc/x-ui/agent/certs/ca.crt"
Environment="AGENT_LOG_LEVEL=info"
```

### Available Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `AGENT_SERVER_ID` | Unique server identifier | Generated |
| `AGENT_SERVER_NAME` | Display name | Hostname |
| `AGENT_LISTEN_ADDR` | Listen address:port | `0.0.0.0:2054` |
| `AGENT_AUTH_TYPE` | Authentication type (`mtls` or `jwt`) | `mtls` |
| `AGENT_CERT_FILE` | Path to agent certificate (mTLS) | `/etc/x-ui/agent/certs/agent.crt` |
| `AGENT_KEY_FILE` | Path to agent private key (mTLS) | `/etc/x-ui/agent/certs/agent.key` |
| `AGENT_CA_FILE` | Path to CA certificate (mTLS) | `/etc/x-ui/agent/certs/ca.crt` |
| `AGENT_JWT_SECRET` | JWT token (JWT auth) | Generated |
| `AGENT_LOG_LEVEL` | Log verbosity (`debug`, `info`, `warn`, `error`) | `info` |

## Managing Agents

### Check Agent Status

```bash
sudo systemctl status x-ui-agent
```

### View Agent Logs

```bash
# Last 50 lines
sudo journalctl -u x-ui-agent -n 50

# Follow live logs
sudo journalctl -u x-ui-agent -f

# With timestamps
sudo journalctl -u x-ui-agent -n 100 --no-pager
```

### Restart Agent

```bash
sudo systemctl restart x-ui-agent
```

### Update Agent Configuration

1. Edit the systemd service file:
   ```bash
   sudo nano /etc/systemd/system/x-ui-agent.service
   ```

2. Reload systemd and restart:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart x-ui-agent
   ```

## Health Check

Test agent connectivity from the control panel server:

```bash
# For JWT agents (HTTP)
curl http://agent-server:2054/api/v1/health

# For mTLS agents (HTTPS with certificates)
curl --cert /path/to/client.crt \
     --key /path/to/client.key \
     --cacert /path/to/ca.crt \
     https://agent-server:2054/api/v1/health
```

Expected response:
```json
{"status":"ok","version":"1.1.83","uptime":3600}
```

## Firewall Configuration

### UFW (Ubuntu/Debian)

```bash
sudo ufw allow 2054/tcp comment 'X-UI Agent'
```

### Firewalld (CentOS/RHEL)

```bash
sudo firewall-cmd --permanent --add-port=2054/tcp
sudo firewall-cmd --reload
```

### iptables

```bash
sudo iptables -A INPUT -p tcp --dport 2054 -j ACCEPT
sudo iptables-save > /etc/iptables/rules.v4
```

## Next Steps

- [Configure authentication details](Authentication.md)
- [Set up monitoring and alerts](Monitoring.md)
- [Review security best practices](Security.md)
