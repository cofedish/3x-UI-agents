# Installation Guide

This guide covers installing the 3X-UI control panel and agents.

## Prerequisites

- Linux server (Ubuntu 20.04+, Debian 10+, CentOS 7+)
- Root or sudo access
- Port 54321 open for panel (configurable)
- Port 2054 open for each agent

## Install Control Panel

Run this on your **main server** (the one you'll access via web browser):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/install.sh)
# Select option 1: Install Control Panel
```

### Post-Installation

1. Access panel at `https://your-server:54321`
2. Default credentials: `admin` / `admin`
3. **Change default password immediately** via Settings → Panel Settings

### Configuration

The panel configuration is stored in `/etc/x-ui/config.json`. Key settings:

- `webPort`: Panel web interface port (default: 54321)
- `webBasePath`: URL base path (default: `/`)
- `webCertFile`, `webKeyFile`: TLS certificate paths

## Install Agent

Run this on each **remote VPN server** you want to manage:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/install.sh)
# Select option 2: Install Agent
```

### Agent Configuration

You'll be prompted for:

1. **Authentication type**: Choose mTLS (recommended) or JWT
2. **Agent name**: Descriptive name (e.g., "US-East-1")
3. **Agent address**: Public IP or domain
4. **Listen port**: Port for agent API (default: 2054)

### Non-Interactive Installation

For automation, set environment variables:

```bash
export AGENT_AUTH_TYPE=mtls  # or jwt
export AGENT_NAME="US-Server-1"
export AGENT_HOST_IP="203.0.113.10"
export AGENT_LISTEN_ADDR="0.0.0.0:2054"

bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/install.sh) install-agent
```

## Next Steps

- [Connect agents to panel](Agent-Setup.md)
- [Configure authentication](Authentication.md)
- [Set up your first inbound](../README.md#quick-start)

## Troubleshooting

See [Troubleshooting Guide](Troubleshooting.md) for common issues.
