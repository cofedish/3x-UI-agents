#!/bin/bash

#================================================================
# 3x-ui Agent Installer
# Installs and configures 3x-ui agent on remote VPN servers
#================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
AGENT_VERSION="${AGENT_VERSION:-latest}"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/x-ui-agent"
CERT_DIR="$CONFIG_DIR/certs"
LOG_DIR="/var/log/x-ui-agent"
SERVICE_NAME="x-ui-agent"

echo -e "${GREEN}=== 3x-ui Agent Installer ===${NC}"
echo ""

# Check if running as root
if [ "$EUID" -ne 0 ]; then
  echo -e "${RED}Error: This script must be run as root${NC}"
  exit 1
fi

# Detect OS and architecture
detect_system() {
  echo -e "${YELLOW}Detecting system...${NC}"

  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)

  case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l) ARCH="armv7" ;;
    *) echo -e "${RED}Unsupported architecture: $ARCH${NC}"; exit 1 ;;
  esac

  echo -e "${GREEN}System: $OS-$ARCH${NC}"
}

# Install dependencies
install_dependencies() {
  echo -e "${YELLOW}Installing dependencies...${NC}"

  if command -v apt-get &> /dev/null; then
    apt-get update -qq
    apt-get install -y wget curl ca-certificates
  elif command -v yum &> /dev/null; then
    yum install -y wget curl ca-certificates
  elif command -v dnf &> /dev/null; then
    dnf install -y wget curl ca-certificates
  else
    echo -e "${YELLOW}Warning: Could not detect package manager${NC}"
  fi
}

# Download agent binary
download_agent() {
  echo -e "${YELLOW}Downloading agent binary...${NC}"

  BINARY_NAME="x-ui-$OS-$ARCH"
  ARCHIVE_NAME="$BINARY_NAME.tar.gz"
  DOWNLOAD_URL="https://github.com/cofedish/3x-UI-agents/releases/download/$AGENT_VERSION/$ARCHIVE_NAME"

  if [ "$AGENT_VERSION" = "latest" ]; then
    # Get latest release URL
    DOWNLOAD_URL=$(curl -s https://api.github.com/repos/cofedish/3x-UI-agents/releases/latest | grep "browser_download_url.*$ARCHIVE_NAME" | cut -d '"' -f 4)
  fi

  echo "Downloading from: $DOWNLOAD_URL"

  # Create temporary directory for extraction
  TMP_DIR=$(mktemp -d)

  # Download archive
  wget -q --show-progress -O "$TMP_DIR/$ARCHIVE_NAME" "$DOWNLOAD_URL" || {
    echo -e "${RED}Error: Failed to download agent binary${NC}"
    rm -rf "$TMP_DIR"
    exit 1
  }

  # Extract archive
  echo -e "${YELLOW}Extracting archive...${NC}"
  tar -xzf "$TMP_DIR/$ARCHIVE_NAME" -C "$TMP_DIR" || {
    echo -e "${RED}Error: Failed to extract archive${NC}"
    rm -rf "$TMP_DIR"
    exit 1
  }

  # Move binary to install directory (archive contains x-ui/ directory with binary inside)
  if [ -f "$TMP_DIR/x-ui/x-ui" ]; then
    mv "$TMP_DIR/x-ui/x-ui" "$INSTALL_DIR/x-ui-agent"
  elif [ -f "$TMP_DIR/x-ui" ]; then
    mv "$TMP_DIR/x-ui" "$INSTALL_DIR/x-ui-agent"
  else
    echo -e "${RED}Error: Binary not found in archive${NC}"
    rm -rf "$TMP_DIR"
    exit 1
  fi

  # Cleanup
  rm -rf "$TMP_DIR"

  chmod +x "$INSTALL_DIR/x-ui-agent"
  echo -e "${GREEN}Agent binary installed to $INSTALL_DIR/x-ui-agent${NC}"
}

# Create directories
create_directories() {
  echo -e "${YELLOW}Creating directories...${NC}"

  mkdir -p "$CONFIG_DIR"
  mkdir -p "$CERT_DIR"
  mkdir -p "$LOG_DIR"

  # Agent needs /etc/x-ui for database (shared with panel if co-located)
  mkdir -p "/etc/x-ui"

  chmod 700 "$CERT_DIR"
  chmod 755 "$CONFIG_DIR"
  chmod 755 "$LOG_DIR"
  chmod 755 "/etc/x-ui"

  echo -e "${GREEN}Directories created${NC}"
}

# Create systemd service
create_service() {
  echo -e "${YELLOW}Creating systemd service...${NC}"

  cat > /etc/systemd/system/$SERVICE_NAME.service <<EOF
[Unit]
Description=3x-ui Agent
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/root
ExecStart=$INSTALL_DIR/x-ui-agent agent
Restart=on-failure
RestartSec=10s

# Environment variables (customize as needed)
Environment="AGENT_LISTEN_ADDR=0.0.0.0:2054"
Environment="AGENT_AUTH_TYPE=mtls"
Environment="AGENT_CERT_FILE=$CERT_DIR/agent.crt"
Environment="AGENT_KEY_FILE=$CERT_DIR/agent.key"
Environment="AGENT_CA_FILE=$CERT_DIR/ca.crt"
Environment="AGENT_LOG_LEVEL=info"
Environment="AGENT_LOG_FILE=$LOG_DIR/agent.log"
Environment="AGENT_RATE_LIMIT=100"

# Security
ProtectSystem=strict
ReadWritePaths=/etc/x-ui /var/log/x-ui-agent
ProtectHome=true
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  echo -e "${GREEN}Systemd service created${NC}"
}

# Configure firewall
configure_firewall() {
  echo -e "${YELLOW}Configuring firewall...${NC}"

  AGENT_PORT=2054

  if command -v ufw &> /dev/null; then
    ufw allow $AGENT_PORT/tcp comment '3x-ui Agent'
    echo -e "${GREEN}UFW firewall rule added${NC}"
  elif command -v firewall-cmd &> /dev/null; then
    firewall-cmd --permanent --add-port=$AGENT_PORT/tcp
    firewall-cmd --reload
    echo -e "${GREEN}Firewalld rule added${NC}"
  else
    echo -e "${YELLOW}Warning: No firewall detected. Please manually open port $AGENT_PORT${NC}"
  fi
}

# Display next steps
display_next_steps() {
  echo ""
  echo -e "${GREEN}=== Installation Complete ===${NC}"
  echo ""
  echo -e "${YELLOW}Next Steps:${NC}"
  echo ""
  echo "1. Copy certificates to $CERT_DIR/:"
  echo "   - agent.crt (agent certificate)"
  echo "   - agent.key (agent private key)"
  echo "   - ca.crt (CA certificate)"
  echo ""
  echo "2. Set proper permissions:"
  echo "   chmod 600 $CERT_DIR/agent.key"
  echo "   chmod 644 $CERT_DIR/agent.crt"
  echo "   chmod 644 $CERT_DIR/ca.crt"
  echo ""
  echo "3. Start the agent:"
  echo "   systemctl start $SERVICE_NAME"
  echo "   systemctl enable $SERVICE_NAME"
  echo ""
  echo "4. Check status:"
  echo "   systemctl status $SERVICE_NAME"
  echo "   journalctl -u $SERVICE_NAME -f"
  echo ""
  echo "5. Test agent API:"
  echo "   curl -k https://localhost:2054/api/v1/health"
  echo ""
  echo -e "${YELLOW}Important:${NC}"
  echo "- Agent listens on port 2054 by default"
  echo "- Only controller should have access to this port"
  echo "- Configure firewall to restrict access"
  echo ""
}

# Main installation flow
main() {
  detect_system
  install_dependencies
  download_agent
  create_directories
  create_service
  configure_firewall
  display_next_steps
}

# Run main function
main

exit 0
