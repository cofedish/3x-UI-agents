[English](/README.md) | [فارسی](/README.fa_IR.md) | [العربية](/README.ar_EG.md) |  [中文](/README.zh_CN.md) | [Español](/README.es_ES.md) | [Русский](/README.ru_RU.md)

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./media/3x-ui-dark.png">
    <img alt="3x-ui" src="./media/3x-ui-light.png">
  </picture>
</p>

[![Release](https://img.shields.io/github/v/release/cofedish/3x-UI-agents.svg)](https://github.com/cofedish/3x-UI-agents/releases)
[![Build](https://img.shields.io/github/actions/workflow/status/cofedish/3x-UI-agents/release.yml.svg)](https://github.com/cofedish/3x-UI-agents/actions)
[![GO Version](https://img.shields.io/github/go-mod/go-version/cofedish/3x-UI-agents.svg)](#)
[![Downloads](https://img.shields.io/github/downloads/cofedish/3x-UI-agents/total.svg)](https://github.com/cofedish/3x-UI-agents/releases/latest)
[![License](https://img.shields.io/badge/license-GPL%20V3-blue.svg?longCache=true)](https://www.gnu.org/licenses/gpl-3.0.en.html)

**3X-UI** — advanced, open-source web-based control panel for managing Xray-core servers with multi-server agent support. Manage multiple remote VPN servers from a single panel.

> [!IMPORTANT]
> This project is only for personal usage. Do not use it for illegal purposes or in a production environment without proper security hardening.

## Features

- **Multi-Server Architecture**: Manage multiple remote servers from one control panel
- **Agent-Based**: Deploy lightweight agents on remote servers, control from central panel
- **Authentication Options**:
  - mTLS (mutual TLS) for production
  - JWT tokens for testing/development
- **Unified Dashboard**: View statistics across all servers in one place
- **Xray Protocol Support**: VLESS, VMess, Trojan, Shadowsocks, and more
- **Web UI**: User-friendly interface for configuration and monitoring

## Quick Start

### Install Control Panel

Run this on your **main server** (the one you'll access via web browser):

```bash
bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/install.sh)
# Select option 1: Install Control Panel
```

After installation:
- Access panel at `https://your-server:54321`
- Default credentials: admin / admin

### Install Agent on Remote Server

Run this on each **remote VPN server** you want to manage:

```bash
bash <(curl -Ls https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/install.sh)
# Select option 2: Install Agent
```

You'll be prompted for:
- **Authentication type**: Choose mTLS (recommended) or JWT
- **Agent details**: Name, address, listening port

### Connect Agent to Panel

1. **If using mTLS**:
   - Agent installer generates certificates automatically
   - Copy the displayed certificate bundle
   - In panel: Servers → Add Server → Paste certificate bundle

2. **If using JWT**:
   - Agent installer shows JWT token
   - Copy the token
   - In panel: Servers → Add Server → Select JWT → Paste token

## Download URLs

- **Latest release**: https://github.com/cofedish/3x-UI-agents/releases/latest
- **Install script**: https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/install.sh
- **Agent install script**: https://raw.githubusercontent.com/cofedish/3x-UI-agents/main/scripts/agent/install.sh

## Authentication Methods

### mTLS (Recommended for Production)

- **Security**: Mutual TLS with certificate validation
- **Use case**: Production environments, secure remote servers
- **Setup**: Agent generates certificates, you copy them to panel
- **Protocol**: HTTPS with client certificates

### JWT Tokens (For Testing)

- **Security**: Token-based authentication
- **Use case**: Development, testing, quick setup
- **Setup**: Agent generates token, you paste it in panel
- **Protocol**: HTTP (no TLS overhead)

## Troubleshooting

### Agent not connecting?

1. Check firewall: Port 2054 must be open on agent server
2. Verify endpoint in panel matches agent server IP/domain
3. Check agent logs: `sudo journalctl -u x-ui-agent -n 50`
4. Test connectivity: `curl http://agent-server:2054/api/v1/health`

### Certificate issues (mTLS)?

- Ensure you copied the **complete** certificate bundle (all three blocks)
- Check certificate expiry: default is 365 days
- Regenerate if needed: run agent installer again

### Agent service not starting?

```bash
# Check status
sudo systemctl status x-ui-agent

# View detailed logs
sudo journalctl -u x-ui-agent -n 100

# Restart manually
sudo systemctl restart x-ui-agent
```

### Port conflicts?

Default ports:
- **Panel**: 54321 (configurable)
- **Agent**: 2054 (change via AGENT_LISTEN_ADDR env)

## Documentation

For detailed guides, visit the [project Wiki](https://github.com/cofedish/3x-UI-agents/wiki).

Topics:
- Architecture overview
- Security best practices
- Advanced configuration
- Backup and restore
- Monitoring and logging

## Acknowledgments

- [cofedish](https://github.com/cofedish) - Project maintainer
- [alireza0](https://github.com/alireza0) - Original X-UI creator
- [Iran v2ray rules](https://github.com/chocolate4u/Iran-v2ray-rules) (GPL-3.0)
- [Russia v2ray rules](https://github.com/runetfreedom/russia-v2ray-rules-dat) (GPL-3.0)

## Support

If this project is helpful, please ⭐ star the repository or support via [GitHub Sponsors](https://github.com/sponsors/cofedish).
