# Backup and Restore Guide

Protect your 3X-UI data with regular backups.

## What to Backup

### Control Panel

| Item | Path | Critical | Notes |
|------|------|----------|-------|
| Database | `/etc/x-ui/x-ui.db` | ✅ Yes | All config, users, inbounds |
| Config | `/etc/x-ui/config.json` | ✅ Yes | Panel settings |
| TLS Certs | `/etc/x-ui/certs/` | ⚠️ Optional | Can regenerate with Let's Encrypt |

### Agent Servers

| Item | Path | Critical | Notes |
|------|------|----------|-------|
| Agent Certs | `/etc/x-ui/agent/certs/` | ✅ Yes (mTLS) | Required for mTLS agents |
| Agent Config | `/etc/x-ui/agent/config.json` | ⚠️ Optional | Can be recreated |
| Xray Config | `/etc/xray/config.json` | ✅ Yes | Inbound configurations |

## Manual Backup

### Panel Backup

```bash
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$HOME/x-ui-backups"
mkdir -p $BACKUP_DIR

# Create backup archive
sudo tar -czf $BACKUP_DIR/panel-backup-$DATE.tar.gz \
  /etc/x-ui/x-ui.db \
  /etc/x-ui/config.json \
  /etc/x-ui/certs/ 2>/dev/null

echo "Panel backup created: $BACKUP_DIR/panel-backup-$DATE.tar.gz"
```

### Agent Backup

```bash
#!/bin/bash
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_DIR="$HOME/agent-backups"
mkdir -p $BACKUP_DIR

# Create backup archive
sudo tar -czf $BACKUP_DIR/agent-backup-$DATE.tar.gz \
  /etc/x-ui/agent/ \
  /etc/xray/config.json 2>/dev/null

echo "Agent backup created: $BACKUP_DIR/agent-backup-$DATE.tar.gz"
```

## Automated Backup

### Panel Backup Script

Create `/root/backup-x-ui-panel.sh`:

```bash
#!/bin/bash
set -e

BACKUP_DIR="/backup/x-ui-panel"
RETENTION_DAYS=30
DATE=$(date +%Y%m%d_%H%M%S)
BACKUP_FILE="$BACKUP_DIR/panel-$DATE.tar.gz"

# Create backup directory
mkdir -p $BACKUP_DIR

# Stop panel to ensure consistent backup
systemctl stop x-ui

# Create backup
tar -czf $BACKUP_FILE \
  /etc/x-ui/x-ui.db \
  /etc/x-ui/config.json \
  /etc/x-ui/certs/ 2>/dev/null || true

# Start panel
systemctl start x-ui

# Set permissions
chmod 600 $BACKUP_FILE

# Remove old backups
find $BACKUP_DIR -name "panel-*.tar.gz" -mtime +$RETENTION_DAYS -delete

# Log success
echo "$(date): Backup created successfully: $BACKUP_FILE" >> /var/log/x-ui-backup.log

# Optional: Upload to remote storage
# rclone copy $BACKUP_FILE remote:backups/x-ui/
# aws s3 cp $BACKUP_FILE s3://my-bucket/x-ui-backups/
```

Make executable:
```bash
chmod +x /root/backup-x-ui-panel.sh
```

### Schedule with Cron

```bash
sudo crontab -e
```

Add entries:

```cron
# Panel backup daily at 2 AM
0 2 * * * /root/backup-x-ui-panel.sh >> /var/log/x-ui-backup.log 2>&1

# Agent backup daily at 3 AM (on each agent server)
0 3 * * * /root/backup-x-ui-agent.sh >> /var/log/x-ui-agent-backup.log 2>&1
```

## Remote Backup Storage

### Using rclone (Recommended)

Install rclone:
```bash
curl https://rclone.org/install.sh | sudo bash
```

Configure remote (example with S3):
```bash
rclone config
# Choose: Amazon S3, enter credentials
```

Upload backup:
```bash
rclone copy /backup/x-ui-panel/ remote:my-bucket/x-ui-backups/
```

### Using AWS CLI

```bash
aws s3 sync /backup/x-ui-panel/ s3://my-bucket/x-ui-backups/
```

### Using rsync over SSH

```bash
rsync -avz -e ssh /backup/x-ui-panel/ user@backup-server:/backups/x-ui/
```

## Restore Procedures

### Restore Panel

1. **Stop the panel**:
   ```bash
   sudo systemctl stop x-ui
   ```

2. **Extract backup**:
   ```bash
   sudo tar -xzf panel-backup-20250101_020000.tar.gz -C /
   ```

3. **Verify files**:
   ```bash
   ls -la /etc/x-ui/
   ```

4. **Start panel**:
   ```bash
   sudo systemctl start x-ui
   ```

5. **Verify operation**:
   ```bash
   sudo systemctl status x-ui
   sudo journalctl -u x-ui -n 20
   ```

### Restore Agent

1. **Stop agent and Xray**:
   ```bash
   sudo systemctl stop x-ui-agent
   sudo systemctl stop xray  # if applicable
   ```

2. **Extract backup**:
   ```bash
   sudo tar -xzf agent-backup-20250101_030000.tar.gz -C /
   ```

3. **Restore permissions**:
   ```bash
   sudo chmod 600 /etc/x-ui/agent/certs/*
   sudo chown root:root /etc/x-ui/agent/certs/*
   ```

4. **Start services**:
   ```bash
   sudo systemctl start x-ui-agent
   sudo systemctl start xray  # if applicable
   ```

### Disaster Recovery (Full Server Loss)

If you lose a server completely:

1. **Provision new server** with same OS
2. **Install 3X-UI** using standard installation
3. **Stop services**:
   ```bash
   sudo systemctl stop x-ui
   ```
4. **Restore backup** as shown above
5. **Update IP addresses** in configs if changed
6. **Restart services**

## Database-Only Backup/Restore

### Backup Database Only

```bash
# Stop panel
sudo systemctl stop x-ui

# Copy database
sudo cp /etc/x-ui/x-ui.db /backup/x-ui.db.$(date +%Y%m%d_%H%M%S)

# Start panel
sudo systemctl start x-ui
```

### Restore Database Only

```bash
# Stop panel
sudo systemctl stop x-ui

# Backup current database (safety)
sudo cp /etc/x-ui/x-ui.db /etc/x-ui/x-ui.db.old

# Restore from backup
sudo cp /backup/x-ui.db.20250101_020000 /etc/x-ui/x-ui.db

# Set permissions
sudo chown root:root /etc/x-ui/x-ui.db
sudo chmod 600 /etc/x-ui/x-ui.db

# Start panel
sudo systemctl start x-ui
```

## Testing Backups

**Always test your backups!**

### Test Procedure

1. **Create test environment** (separate VM or Docker)
2. **Install 3X-UI** fresh
3. **Restore from backup**
4. **Verify all data** is present
5. **Test functionality** (login, view servers, etc.)

### Automated Test

```bash
#!/bin/bash
# backup-test.sh

BACKUP_FILE="/backup/x-ui-panel/panel-latest.tar.gz"
TEST_DIR="/tmp/x-ui-test"

mkdir -p $TEST_DIR
tar -xzf $BACKUP_FILE -C $TEST_DIR

# Check database integrity
if command -v sqlite3 &> /dev/null; then
  sqlite3 $TEST_DIR/etc/x-ui/x-ui.db "PRAGMA integrity_check;"
  if [ $? -eq 0 ]; then
    echo "✅ Backup database integrity OK"
  else
    echo "❌ Backup database corrupted!"
    exit 1
  fi
fi

rm -rf $TEST_DIR
```

## Backup Verification

### Verify Backup Integrity

```bash
# Check tar archive integrity
tar -tzf panel-backup-20250101_020000.tar.gz > /dev/null
if [ $? -eq 0 ]; then
  echo "✅ Archive integrity OK"
else
  echo "❌ Archive corrupted"
fi

# List contents
tar -tzf panel-backup-20250101_020000.tar.gz
```

### Verify Database

```bash
# Extract database from backup
tar -xzf panel-backup-20250101_020000.tar.gz etc/x-ui/x-ui.db -O > /tmp/test-db

# Check integrity
sqlite3 /tmp/test-db "PRAGMA integrity_check;"

# Check tables
sqlite3 /tmp/test-db ".tables"

rm /tmp/test-db
```

## Best Practices

1. **Backup frequently**: Daily for production, weekly for testing
2. **Test restores**: Monthly restore drills
3. **Store off-site**: Use remote storage (S3, Backblaze, etc.)
4. **Encrypt backups**: Use GPG for sensitive data
5. **Document procedures**: Keep this guide updated
6. **Automate everything**: Reduce human error
7. **Monitor backup jobs**: Alert on failures
8. **Retention policy**: Keep 30 daily, 12 monthly, 7 yearly

## Encryption (Optional)

### Encrypt Backup with GPG

```bash
# Create backup and encrypt
tar -czf - /etc/x-ui/ | gpg --symmetric --cipher-algo AES256 -o panel-backup-encrypted.tar.gz.gpg

# Decrypt and restore
gpg --decrypt panel-backup-encrypted.tar.gz.gpg | tar -xzf - -C /
```

### Using Password

```bash
# Encrypt with password
tar -czf - /etc/x-ui/ | openssl enc -aes-256-cbc -salt -out panel-backup.tar.gz.enc

# Decrypt
openssl enc -d -aes-256-cbc -in panel-backup.tar.gz.enc | tar -xzf - -C /
```

## Recovery Time Objective (RTO)

Typical recovery times:

- **Panel database only**: 5 minutes
- **Full panel restore**: 15 minutes
- **Agent restore**: 10 minutes
- **Full disaster recovery**: 60 minutes (new server + restore)

Plan accordingly for your SLA requirements.
