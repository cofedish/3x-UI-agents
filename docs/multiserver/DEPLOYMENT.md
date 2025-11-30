# Multi-Server Deployment Guide

## Current Status

⚠️ **This is a work-in-progress implementation.**

The core infrastructure is complete, but UI and Agent components are still pending.

**What Works:**
- ✅ Database schema and migrations
- ✅ ServerConnector abstraction layer
- ✅ LocalConnector (local server operations)
- ✅ RemoteConnector (HTTP client for agents)
- ✅ Health monitoring job
- ✅ Backward compatibility with single-server mode

**What's Pending:**
- ⏳ Agent API endpoints (agent-side server)
- ⏳ UI adaptation (server management page, selector)
- ⏳ Telegram bot multi-server support
- ⏳ API controller updates
- ⏳ Installation scripts

---

## Single-Server Mode (Current Behavior)

The current implementation **fully supports backward compatibility**. If you build and run this branch:

### On Fresh Install

```bash
# Build
cd 3x-ui
go build -o x-ui main.go

# Run
./x-ui run
```

**What Happens:**
1. Database initializes
2. Default local server created (ID=1)
3. All operations work on local server
4. UI shows single-server interface (no selector)
5. Everything works exactly as before

### On Upgrade from Old Version

```bash
# Backup database first!
cp /etc/x-ui/3x-ui.db /etc/x-ui/3x-ui.db.backup

# Replace binary with new version
./x-ui run
```

**What Happens:**
1. Migration `runMultiserverMigration()` runs automatically
2. Checks if already migrated (idempotent)
3. Creates default local server if not exists
4. Adds server_id columns to existing tables
5. Sets server_id=1 for all existing data
6. Creates indexes
7. Records migration in history_of_seeders

**Result:**
- All existing inbounds work
- All traffic stats preserved
- All clients active
- No configuration changes needed
- UI unchanged (single-server mode)

---

## Multi-Server Mode (Pending Implementation)

⚠️ **Agent and UI not yet implemented**

### Architecture Overview

```
┌────────────────────────────────┐
│       Controller               │
│   (Your current panel)         │
│   + ServerConnector layer      │
│   + Health monitoring          │
│   + mTLS/JWT client            │
└───────────┬────────────────────┘
            │
            │ HTTPS + mTLS
            │
    ┌───────┴────────┬──────────────┐
    │                │              │
    ▼                ▼              ▼
┌─────────┐    ┌─────────┐    ┌─────────┐
│ Agent 1 │    │ Agent 2 │    │ Agent N │
│ (VPN-1) │    │ (VPN-2) │    │ (VPN-N) │
└─────────┘    └─────────┘    └─────────┘
```

### When Agent is Implemented

#### Step 1: Install Agent on VPN Server

```bash
# On VPN server (remote)
curl -sSL https://example.com/install-agent.sh | bash

# Or manually:
wget https://github.com/cofedish/3xui-agents/releases/download/vX.X.X/x-ui-agent-linux-amd64
chmod +x x-ui-agent-linux-amd64
mv x-ui-agent-linux-amd64 /usr/local/bin/x-ui-agent

# Create config
mkdir -p /etc/x-ui-agent/certs
vi /etc/x-ui-agent/config.yaml
```

#### Step 2: Generate mTLS Certificates

**On Controller:**
```bash
# Generate CA and agent certificate
./x-ui cert-authority init
./x-ui cert-authority issue --name vpn-us-1 --output /tmp/vpn-us-1-certs/

# Transfer to agent server
scp /tmp/vpn-us-1-certs/* root@vpn-us-1:/etc/x-ui-agent/certs/
```

#### Step 3: Configure Agent

**On Agent Server (`/etc/x-ui-agent/config.yaml`):**

```yaml
controller:
  endpoint: "https://panel.example.com:2053"
  auth:
    type: "mtls"
    cert_file: "/etc/x-ui-agent/certs/agent.crt"
    key_file: "/etc/x-ui-agent/certs/agent.key"
    ca_file: "/etc/x-ui-agent/certs/ca.crt"

agent:
  listen_addr: "0.0.0.0:2054"
  server_id: "vpn-us-1"
  tags: ["us", "production"]

xray:
  bin_folder: "/usr/local/x-ui/bin"
  config_folder: "/etc/x-ui"

logging:
  level: "info"
  file: "/var/log/x-ui-agent/agent.log"
```

#### Step 4: Start Agent

```bash
# Start agent
systemctl start x-ui-agent
systemctl enable x-ui-agent

# Check status
systemctl status x-ui-agent
journalctl -u x-ui-agent -f
```

#### Step 5: Add Server in Controller UI

⚠️ **UI not implemented yet**

When UI is ready:
1. Go to `Settings > Servers` (or `/panel/servers`)
2. Click "Add Server"
3. Fill in:
   - Name: "VPN US East"
   - Endpoint: "https://vpn-us-1.example.com:2054"
   - Region: "us-east"
   - Tags: production, us
   - Auth Type: mTLS
   - Upload certificates
4. Save
5. Server status should show "online" within 30s

---

## Testing Current Implementation

### Test 1: Database Migration

```bash
# On existing 3x-ui installation
./x-ui run &

# Check logs for migration
tail -f /var/log/x-ui/access.log

# Expected output:
# Running multi-server migration...
# Created default local server (ID=1)
# Added server_id column to inbounds
# ...
# Multi-server migration completed successfully
```

### Test 2: Query Database

```bash
# Open database
sqlite3 /etc/x-ui/3x-ui.db

# Check servers table
SELECT * FROM servers;
# Should show 1 row: Default Local Server

# Check inbounds have server_id
PRAGMA table_info(inbounds);
# Should show server_id column

# Check existing inbounds migrated
SELECT id, remark, server_id FROM inbounds LIMIT 5;
# All should have server_id = 1
```

### Test 3: Health Monitoring

```bash
# Check cron job registered
ps aux | grep x-ui

# Check logs
tail -f /var/log/x-ui/access.log | grep "server health"

# Query server status
sqlite3 /etc/x-ui/3x-ui.db "SELECT id, name, status, last_seen FROM servers;"

# Local server should always be online
```

---

## Development Setup

### Build from Source

```bash
cd 3x-ui

# Install dependencies
go mod download

# Build
go build -o x-ui main.go

# Run
./x-ui run
```

### Database Schema Inspection

```bash
sqlite3 /etc/x-ui/3x-ui.db

.schema servers
.schema server_tasks
.schema inbounds
.schema client_traffics
.schema outbound_traffics
.schema inbound_client_ips
```

Expected schema additions:

```sql
-- New tables
CREATE TABLE servers (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    endpoint TEXT NOT NULL,
    -- ... (see model.Server for full schema)
);

CREATE TABLE server_tasks (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    server_id INTEGER NOT NULL,
    operation TEXT NOT NULL,
    -- ... (see model.ServerTask for full schema)
);

-- Modified tables (server_id column added)
ALTER TABLE inbounds ADD COLUMN server_id INTEGER;
ALTER TABLE client_traffics ADD COLUMN server_id INTEGER;
ALTER TABLE outbound_traffics ADD COLUMN server_id INTEGER;
ALTER TABLE inbound_client_ips ADD COLUMN server_id INTEGER;
```

---

## Troubleshooting

### Migration Fails

**Problem:** Migration fails on startup

**Solution:**
```bash
# Check seeder history
sqlite3 /etc/x-ui/3x-ui.db "SELECT * FROM history_of_seeders;"

# If MultiServerMigration is missing, it didn't run
# Check logs for error
tail -100 /var/log/x-ui/access.log

# If columns already exist, migration skips adding them (safe)
# If errors persist, check database permissions
```

### Server ID Not Set

**Problem:** Existing inbounds have NULL server_id

**Solution:**
```bash
# Migration should have set server_id=1 for all existing records
# If not, run manually:
sqlite3 /etc/x-ui/3x-ui.db <<EOF
UPDATE inbounds SET server_id = 1 WHERE server_id IS NULL;
UPDATE client_traffics SET server_id = 1 WHERE server_id IS NULL;
UPDATE outbound_traffics SET server_id = 1 WHERE server_id IS NULL;
UPDATE inbound_client_ips SET server_id = 1 WHERE server_id IS NULL;
EOF
```

### Health Check Not Running

**Problem:** Server health not updating

**Solution:**
```bash
# Check cron job is registered
# Look for: AddJob("@every 30s", job.NewServerHealthJob())

# Check logs
journalctl -u x-ui -f | grep -i "health"

# Manually trigger (if method exists)
# Not implemented yet - job runs automatically every 30s
```

---

## Security Considerations

### Current Implementation

**Secure:**
- mTLS certificate validation in RemoteConnector
- TLS 1.3 minimum version
- Server-side authentication required

**Pending:**
- Certificate rotation mechanism
- Certificate revocation (CRL/OCSP)
- JWT token refresh
- Rate limiting on agent API
- Input validation on agent API

### Best Practices

1. **Certificates:**
   - Use strong keypairs (4096-bit RSA or Ed25519)
   - Set reasonable expiration (1 year)
   - Store private keys securely (chmod 600)
   - Use dedicated CA for agents

2. **Network:**
   - Use firewall to restrict agent port (2054)
   - Only allow controller IP to access agents
   - Use private networks when possible
   - Monitor failed authentication attempts

3. **Database:**
   - Encrypt auth_data field (future enhancement)
   - Regular backups
   - Restrict database file permissions

---

## Rollback Procedure

If you need to rollback to single-server version:

### Option 1: Use Old Binary

```bash
# Stop current version
systemctl stop x-ui

# Restore old binary
cp /usr/local/bin/x-ui.backup /usr/local/bin/x-ui

# Start
systemctl start x-ui
```

**Result:**
- Extra columns (server_id) remain but are harmless
- servers and server_tasks tables exist but unused
- Everything works as before

### Option 2: Remove Columns (Risky)

⚠️ **Only if absolutely necessary**

```bash
# Backup first!
cp /etc/x-ui/3x-ui.db /etc/x-ui/3x-ui.db.backup

# SQLite doesn't support DROP COLUMN easily
# Need to recreate tables:

sqlite3 /etc/x-ui/3x-ui.db <<'EOF'
BEGIN TRANSACTION;

-- Create temp table without server_id
CREATE TABLE inbounds_backup AS
SELECT id, user_id, up, down, total, remark, enable, expiry_time,
       listen, port, protocol, settings, stream_settings, tag, sniffing
FROM inbounds;

-- Drop and recreate
DROP TABLE inbounds;
CREATE TABLE inbounds (...); -- Use old schema

-- Restore data
INSERT INTO inbounds SELECT * FROM inbounds_backup;
DROP TABLE inbounds_backup;

COMMIT;
EOF

# Restart
systemctl restart x-ui
```

**Not Recommended:** Keep the extra columns, they don't hurt.

---

## Performance Benchmarks

⏳ **Not tested yet**

Expected performance:
- Health checks: 30s interval per server
- HTTP round-trip: ~50-200ms per operation
- Database queries: <10ms (with indexes)
- Concurrent operations: Limited by Go scheduler

Scalability:
- Tested: 1 server (local)
- Expected max: 100-200 servers with current design
- Bottleneck: HTTP client connections

---

## Future Enhancements

### Planned
1. Agent auto-update mechanism
2. WebSocket for real-time updates
3. Agent→Controller push notifications
4. Distributed backups
5. Load balancing policies
6. High availability (HA) mode
7. Prometheus metrics export

### Under Consideration
1. Controller clustering
2. Database replication (PostgreSQL)
3. Agent service mesh
4. Multi-region support
5. Advanced monitoring (Grafana)

---

## Support

### Getting Help

**For current implementation:**
- Check [IMPLEMENTATION_STATUS.md](./IMPLEMENTATION_STATUS.md)
- Review [ARCHITECTURE.md](./ARCHITECTURE.md)
- Check git commit history

**For issues:**
- Create issue on GitHub (when published)
- Include logs, database schema, steps to reproduce
- Specify if single or multi-server mode

**For contributions:**
- See [ARCHITECTURE.md](./ARCHITECTURE.md) for design
- Follow existing code patterns
- Add tests for new features
- Update documentation

---

## Changelog

### 2025-11-30 - v0.1.0-multiserver (WIP)

**Added:**
- Multi-server database schema
- ServerConnector abstraction layer
- LocalConnector for local operations
- RemoteConnector for agent communication
- Health monitoring job
- Automatic database migration
- Backward compatibility with single-server mode

**Changed:**
- Database models: Added ServerId field
- Cron jobs: Added ServerHealthJob

**Deprecated:**
- None

**Removed:**
- None

**Fixed:**
- None

**Security:**
- mTLS support for agent communication
- TLS 1.3 minimum version

---

**Last Updated:** 2025-11-30
**Status:** Work in Progress
**Branch:** feature/multiserver-controller-agent
