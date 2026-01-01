# 3X-UI Documentation

Welcome to the 3X-UI documentation. This wiki provides comprehensive guides for installing, configuring, and managing your multi-server VPN panel.

## Quick Links

- [Installation Guide](Installation.md) - Get started with control panel and agents
- [Agent Setup](Agent-Setup.md) - Deploy agents on remote servers
- [Authentication](Authentication.md) - mTLS and JWT configuration
- [Troubleshooting](Troubleshooting.md) - Common issues and solutions
- [Security Best Practices](Security.md) - Hardening your deployment
- [Backup and Restore](Backup-Restore.md) - Data protection strategies

## What is 3X-UI?

3X-UI is an advanced, open-source web-based control panel for managing Xray-core servers with multi-server agent support. Manage multiple remote VPN servers from a single centralized panel.

### Key Features

- **Multi-Server Architecture**: Control multiple remote servers from one dashboard
- **Agent-Based Design**: Lightweight agents on remote servers
- **Dual Authentication**: mTLS for production, JWT for development
- **Unified Monitoring**: Real-time statistics across all servers
- **Xray Protocol Support**: VLESS, VMess, Trojan, Shadowsocks, and more

## Getting Started

1. [Install the control panel](Installation.md#install-control-panel) on your main server
2. [Install agents](Installation.md#install-agent) on each remote VPN server
3. [Connect agents to panel](Agent-Setup.md#connecting-agents) using mTLS or JWT
4. Start managing inbounds, clients, and traffic across all servers

## Architecture Overview

```
┌─────────────────┐
│  Control Panel  │  ← Your main server (web UI)
│   (Port 54321)  │
└────────┬────────┘
         │
    ┌────┴────┬────────┬────────┐
    │         │        │        │
┌───▼───┐ ┌──▼───┐ ┌──▼───┐ ┌──▼───┐
│Agent 1│ │Agent2│ │Agent3│ │Agent4│  ← Remote VPN servers
│:2054  │ │:2054 │ │:2054 │ │:2054 │
└───────┘ └──────┘ └──────┘ └──────┘
```

## Support

- **Issues**: [GitHub Issues](https://github.com/cofedish/3x-UI-agents/issues)
- **Discussions**: [GitHub Discussions](https://github.com/cofedish/3x-UI-agents/discussions)
- **Sponsor**: [GitHub Sponsors](https://github.com/sponsors/cofedish)
