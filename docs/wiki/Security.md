# Security Best Practices

Guidelines for securing your 3X-UI deployment.

## Production Deployment Checklist

### Panel Security

- [ ] **Change default credentials** immediately after installation
- [ ] **Use HTTPS** with valid TLS certificates (Let's Encrypt recommended)
- [ ] **Restrict panel access** to specific IPs using firewall rules
- [ ] **Enable two-factor authentication** if available in settings
- [ ] **Use strong passwords** (16+ characters, mixed case, symbols)
- [ ] **Disable root login** and use sudo user
- [ ] **Keep panel updated** to latest stable version

### Agent Security

- [ ] **Use mTLS authentication** for production (not JWT)
- [ ] **Firewall agent ports** to only allow panel IP
- [ ] **Use non-default ports** when possible
- [ ] **Rotate certificates** before expiry
- [ ] **Monitor agent logs** for suspicious activity
- [ ] **Keep agents updated** to match panel version

### Server Hardening

- [ ] **Disable SSH password auth**, use keys only
- [ ] **Enable automatic security updates**
- [ ] **Install fail2ban** or similar intrusion prevention
- [ ] **Use minimal server installation** (no unnecessary packages)
- [ ] **Regular backups** to off-site location
- [ ] **Monitor resource usage** for anomalies

## Firewall Configuration

### Control Panel Server

```bash
# Allow SSH (restrict to your IP)
sudo ufw allow from YOUR_IP to any port 22

# Allow panel web interface (restrict to trusted IPs)
sudo ufw allow from YOUR_IP to any port 54321

# Allow agent connections (if agents connect to panel)
sudo ufw allow from AGENT_IP_1 to any port 54321
sudo ufw allow from AGENT_IP_2 to any port 54321

# Deny all other inbound
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw enable
```

### Agent Servers

```bash
# Allow SSH (restrict to your IP)
sudo ufw allow from YOUR_IP to any port 22

# Allow agent API (restrict to panel IP)
sudo ufw allow from PANEL_IP to any port 2054

# Allow VPN client connections (example for port 443)
sudo ufw allow 443/tcp

# Deny all other inbound
sudo ufw default deny incoming
sudo ufw default allow outgoing
sudo ufw enable
```

## TLS/SSL Configuration

### Panel HTTPS

Use Let's Encrypt for free, auto-renewing certificates:

```bash
# Install certbot
sudo apt install certbot

# Get certificate
sudo certbot certonly --standalone -d panel.yourdomain.com

# Update panel config
sudo nano /etc/x-ui/config.json
```

Set in `config.json`:
```json
{
  "webCertFile": "/etc/letsencrypt/live/panel.yourdomain.com/fullchain.pem",
  "webKeyFile": "/etc/letsencrypt/live/panel.yourdomain.com/privkey.pem"
}
```

Restart panel:
```bash
sudo systemctl restart x-ui
```

### Certificate Auto-Renewal

```bash
# Test renewal
sudo certbot renew --dry-run

# Add renewal hook to restart panel
sudo nano /etc/letsencrypt/renewal-hooks/deploy/restart-x-ui.sh
```

Hook content:
```bash
#!/bin/bash
systemctl restart x-ui
```

Make executable:
```bash
sudo chmod +x /etc/letsencrypt/renewal-hooks/deploy/restart-x-ui.sh
```

## Access Control

### IP Whitelisting

Panel level (in `/etc/x-ui/config.json`):
```json
{
  "allowedIPs": ["1.2.3.4", "5.6.7.8"]
}
```

System level (firewall preferred for defense in depth).

### Reverse Proxy (Nginx)

Use Nginx for additional security layer:

```nginx
server {
    listen 443 ssl http2;
    server_name panel.yourdomain.com;

    ssl_certificate /etc/letsencrypt/live/panel.yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/panel.yourdomain.com/privkey.pem;

    # Security headers
    add_header X-Frame-Options "SAMEORIGIN" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header X-XSS-Protection "1; mode=block" always;
    add_header Strict-Transport-Security "max-age=31536000" always;

    # IP whitelist
    allow YOUR_IP;
    deny all;

    location / {
        proxy_pass http://127.0.0.1:54321;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Backup and Recovery

### What to Backup

1. **Panel database**: `/etc/x-ui/x-ui.db`
2. **Panel config**: `/etc/x-ui/config.json`
3. **Agent certificates**: `/etc/x-ui/agent/certs/` (per agent)
4. **Xray configs**: `/etc/xray/` (per agent)

### Automated Backup Script

```bash
#!/bin/bash
BACKUP_DIR="/backup/x-ui"
DATE=$(date +%Y%m%d_%H%M%S)

mkdir -p $BACKUP_DIR

# Backup panel
tar -czf $BACKUP_DIR/panel-$DATE.tar.gz \
  /etc/x-ui/x-ui.db \
  /etc/x-ui/config.json

# Keep only last 7 days
find $BACKUP_DIR -name "panel-*.tar.gz" -mtime +7 -delete
```

Schedule with cron:
```bash
sudo crontab -e
# Add: 0 2 * * * /root/backup-x-ui.sh
```

### Restore from Backup

```bash
# Stop panel
sudo systemctl stop x-ui

# Extract backup
tar -xzf panel-20250101_020000.tar.gz -C /

# Restart panel
sudo systemctl start x-ui
```

## Monitoring and Logging

### Enable Detailed Logging

Panel (`/etc/x-ui/config.json`):
```json
{
  "logLevel": "info"
}
```

Agent (systemd service):
```ini
Environment="AGENT_LOG_LEVEL=info"
```

### Monitor Logs

```bash
# Watch panel logs
sudo journalctl -u x-ui -f

# Watch agent logs
sudo journalctl -u x-ui-agent -f

# Check for failed auth attempts
sudo journalctl -u x-ui | grep -i "failed\|error\|unauthorized"
```

### External Monitoring

Consider tools like:
- **Prometheus + Grafana** for metrics
- **Loki** for log aggregation
- **Uptime Kuma** for availability monitoring

## Incident Response

If you suspect compromise:

1. **Isolate affected server** (block network access)
2. **Review logs** for unauthorized access
3. **Rotate all credentials** (passwords, tokens, certificates)
4. **Restore from known-good backup**
5. **Update all software** to latest versions
6. **Review and harden firewall rules**

## Security Updates

```bash
# Ubuntu/Debian
sudo apt update && sudo apt upgrade -y

# CentOS/RHEL
sudo yum update -y

# Reboot if kernel updated
sudo reboot
```

Enable automatic security updates:

```bash
# Ubuntu/Debian
sudo apt install unattended-upgrades
sudo dpkg-reconfigure -plow unattended-upgrades
```

## Additional Resources

- [OWASP Top Ten](https://owasp.org/www-project-top-ten/)
- [CIS Benchmarks](https://www.cisecurity.org/cis-benchmarks/)
- [Let's Encrypt Documentation](https://letsencrypt.org/docs/)
