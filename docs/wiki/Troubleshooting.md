# Troubleshooting Guide

Common issues and their solutions.

## Agent Connection Issues

### Agent Not Connecting

**Symptoms**: Agent shows as "offline" or "error" in panel

**Solutions**:

1. **Check firewall**: Port 2054 must be open on agent server
   ```bash
   sudo netstat -tlnp | grep 2054
   sudo ufw status | grep 2054
   ```

2. **Verify endpoint** in panel matches agent server IP/domain
   - Panel → Servers → Edit Server
   - Endpoint should be `http://AGENT-IP:2054` (JWT) or `https://AGENT-IP:2054` (mTLS)

3. **Check agent logs**:
   ```bash
   sudo journalctl -u x-ui-agent -n 50
   ```

4. **Test connectivity** from control panel server:
   ```bash
   curl http://agent-server:2054/api/v1/health
   ```

### Certificate Issues (mTLS)

**Symptoms**: "certificate verify failed", "TLS handshake error"

**Solutions**:

1. **Ensure complete certificate bundle** was copied (all three blocks):
   - Agent certificate
   - Agent private key
   - CA certificate

2. **Check certificate expiry**:
   ```bash
   openssl x509 -in /etc/x-ui/agent/certs/agent.crt -noout -enddate
   ```
   Default expiry is 365 days

3. **Regenerate certificates**:
   ```bash
   # Re-run agent installer
   bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/install.sh)
   # Select option 2: Install Agent
   # Choose mTLS when prompted
   ```

### Agent Service Not Starting

**Symptoms**: `systemctl status x-ui-agent` shows "failed" or "inactive"

**Solutions**:

1. **Check service status**:
   ```bash
   sudo systemctl status x-ui-agent
   ```

2. **View detailed logs**:
   ```bash
   sudo journalctl -u x-ui-agent -n 100
   ```

3. **Common issues**:
   - Missing configuration file: `/etc/x-ui/agent/config.json`
   - Invalid permissions on cert files
   - Port already in use

4. **Restart manually**:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable x-ui-agent
   sudo systemctl restart x-ui-agent
   ```

## Panel Issues

### Cannot Access Web Interface

**Symptoms**: Browser can't connect to `https://server:54321`

**Solutions**:

1. **Check panel is running**:
   ```bash
   sudo systemctl status x-ui
   ```

2. **Verify port is open**:
   ```bash
   sudo netstat -tlnp | grep 54321
   sudo ufw allow 54321/tcp
   ```

3. **Check panel logs**:
   ```bash
   sudo journalctl -u x-ui -n 50
   ```

### Database Locked Errors

**Symptoms**: "database is locked", operations fail

**Solutions**:

1. **Stop panel**:
   ```bash
   sudo systemctl stop x-ui
   ```

2. **Check for stale locks**:
   ```bash
   fuser /etc/x-ui/x-ui.db
   ```

3. **Restart panel**:
   ```bash
   sudo systemctl start x-ui
   ```

## Inbound/Client Issues

### Clients Can't Connect

**Symptoms**: VPN clients fail to connect through Xray

**Solutions**:

1. **Check Xray is running** on agent:
   ```bash
   ps aux | grep xray
   sudo systemctl status xray  # if using systemd
   ```

2. **Verify inbound configuration**:
   - Correct protocol (VLESS/VMess/Trojan)
   - Correct port and UUID
   - TLS settings match client

3. **Check firewall** on agent server:
   ```bash
   # Example for inbound on port 443
   sudo ufw allow 443/tcp
   ```

4. **Test Xray config**:
   ```bash
   /usr/local/bin/xray -test -config /etc/xray/config.json
   ```

### Traffic Not Updating

**Symptoms**: Usage statistics don't update in panel

**Solutions**:

1. **Check agent connectivity** (see above)

2. **Verify agent can read Xray data**:
   ```bash
   # Check Xray API is accessible
   curl http://localhost:10085/stats/query
   ```

3. **Restart agent**:
   ```bash
   sudo systemctl restart x-ui-agent
   ```

## Port Conflicts

### Default Ports

- **Panel**: 54321 (configurable in `/etc/x-ui/config.json`)
- **Agent**: 2054 (change via `AGENT_LISTEN_ADDR` env variable)

### Changing Agent Port

1. Edit systemd service:
   ```bash
   sudo nano /etc/systemd/system/x-ui-agent.service
   ```

2. Update `AGENT_LISTEN_ADDR`:
   ```ini
   Environment="AGENT_LISTEN_ADDR=0.0.0.0:3054"
   ```

3. Reload and restart:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl restart x-ui-agent
   ```

4. Update endpoint in panel to match new port

## Getting Help

If you're still experiencing issues:

1. **Gather logs**:
   ```bash
   # Panel logs
   sudo journalctl -u x-ui -n 200 > panel-logs.txt

   # Agent logs
   sudo journalctl -u x-ui-agent -n 200 > agent-logs.txt
   ```

2. **Open an issue**: [GitHub Issues](https://github.com/cofedish/3x-UI-agents/issues)
   - Include OS version, installation method
   - Attach relevant logs (redact sensitive info)
   - Describe steps to reproduce

3. **Join discussions**: [GitHub Discussions](https://github.com/cofedish/3x-UI-agents/discussions)
