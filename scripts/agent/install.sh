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
RESOLVED_AGENT_VERSION="$AGENT_VERSION"
APP_DIR="/usr/local/x-ui-agent"
BIN_DIR="$APP_DIR/bin"
INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="/etc/x-ui-agent"
CERT_DIR="$CONFIG_DIR/certs"
LOG_DIR="/var/log/x-ui-agent"
SERVICE_NAME="x-ui-agent"
AUTH_TYPE=""
JWT_TOKEN=""
AGENT_HOST_IP="${AGENT_HOST_IP:-}"
XRAY_VERSION="${XRAY_VERSION:-v25.10.15}"

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

detect_ip() {
  if [ -n "$AGENT_HOST_IP" ]; then
    echo -e "${GREEN}Using provided AGENT_HOST_IP: $AGENT_HOST_IP${NC}"
    return
  fi
  echo -e "${YELLOW}Detecting public IP...${NC}"
  AGENT_HOST_IP=$(curl -s https://ifconfig.me || true)
  if [ -z "$AGENT_HOST_IP" ]; then
    AGENT_HOST_IP=$(hostname -I 2>/dev/null | awk '{print $1}')
  fi
  if [ -z "$AGENT_HOST_IP" ]; then
    echo -e "${RED}Unable to detect IP automatically. Set AGENT_HOST_IP and rerun.${NC}"
    exit 1
  fi
  echo -e "${GREEN}Detected IP: $AGENT_HOST_IP${NC}"
}

# Install dependencies
install_dependencies() {
  echo -e "${YELLOW}Installing dependencies...${NC}"

  if command -v apt-get &> /dev/null; then
    apt-get update -qq
    apt-get install -y wget curl ca-certificates openssl unzip
  elif command -v yum &> /dev/null; then
    yum install -y wget curl ca-certificates openssl unzip
  elif command -v dnf &> /dev/null; then
    dnf install -y wget curl ca-certificates openssl unzip
  else
    echo -e "${YELLOW}Warning: Could not detect package manager${NC}"
  fi
}

# Stop running service if present (for upgrades)
stop_existing_service() {
  if systemctl list-units --type=service --all 2>/dev/null | grep -q "^$SERVICE_NAME.service"; then
    if systemctl is-active --quiet "$SERVICE_NAME"; then
      echo -e "${YELLOW}Stopping existing $SERVICE_NAME service...${NC}"
      systemctl stop "$SERVICE_NAME" || true
    fi
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
    # Derive resolved version from URL
    RESOLVED_AGENT_VERSION=$(printf "%s" "$DOWNLOAD_URL" | awk -F'/download/' '{print $2}' | cut -d'/' -f1)
  else
    RESOLVED_AGENT_VERSION="$AGENT_VERSION"
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

  # Move full app (with bin/xray) to APP_DIR
  rm -rf "$APP_DIR"
  mkdir -p "$APP_DIR"

  if [ -d "$TMP_DIR/x-ui" ]; then
    cp -a "$TMP_DIR/x-ui/." "$APP_DIR/"
  elif [ -f "$TMP_DIR/x-ui" ]; then
    mkdir -p "$APP_DIR/bin"
    cp "$TMP_DIR/x-ui" "$APP_DIR/x-ui"
  else
    echo -e "${RED}Error: Package contents not found${NC}"
    rm -rf "$TMP_DIR"
    exit 1
  fi

  # Ensure executables are runnable
  chmod +x "$APP_DIR/x-ui" || true
  if [ -d "$BIN_DIR" ]; then
    chmod +x "$BIN_DIR"/* 2>/dev/null || true
  fi

  # Symlink CLI entrypoint for convenience
  ln -sf "$APP_DIR/x-ui" "$INSTALL_DIR/x-ui-agent"

  # Cleanup
  rm -rf "$TMP_DIR"

  echo -e "${GREEN}Agent installed to $APP_DIR (binary: $APP_DIR/x-ui)${NC}"
  echo -e "${GREEN}Agent version: $RESOLVED_AGENT_VERSION${NC}"
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
  mkdir -p "$BIN_DIR"

  echo -e "${GREEN}Directories created${NC}"
}

ensure_xray_assets() {
  local xray_bin="$BIN_DIR/xray-linux-$ARCH"
  if [ -f "$xray_bin" ]; then
    echo -e "${GREEN}Xray binary found: $xray_bin${NC}"
    return
  fi

  echo -e "${YELLOW}Xray core not found, downloading...${NC}"
  local base_url="https://github.com/XTLS/Xray-core/releases/download/${XRAY_VERSION}/"
  local pkg=""
  case "$ARCH" in
    amd64) pkg="Xray-linux-64.zip" ;;
    arm64) pkg="Xray-linux-arm64-v8a.zip" ;;
    armv7) pkg="Xray-linux-arm32-v7a.zip" ;;
    armv6) pkg="Xray-linux-arm32-v6.zip" ;;
    armv5) pkg="Xray-linux-arm32-v5.zip" ;;
    386) pkg="Xray-linux-32.zip" ;;
    s390x) pkg="Xray-linux-s390x.zip" ;;
    *) echo -e "${RED}Unsupported architecture for Xray: $ARCH${NC}"; return ;;
  esac

  TMP_DIR=$(mktemp -d)
  wget -q -O "$TMP_DIR/xray.zip" "${base_url}${pkg}" || {
    echo -e "${RED}Failed to download Xray package${NC}"
    rm -rf "$TMP_DIR"
    return
  }
  unzip -q "$TMP_DIR/xray.zip" -d "$TMP_DIR/xray" || {
    echo -e "${RED}Failed to extract Xray package${NC}"
    rm -rf "$TMP_DIR"
    return
  }
  if [ -f "$TMP_DIR/xray/xray" ]; then
    mv "$TMP_DIR/xray/xray" "$xray_bin"
    chmod +x "$xray_bin"
    echo -e "${GREEN}Installed Xray binary to $xray_bin${NC}"
  fi
  # Geo files (best-effort)
  wget -q -O "$BIN_DIR/geoip.dat" https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geoip.dat || true
  wget -q -O "$BIN_DIR/geosite.dat" https://github.com/Loyalsoldier/v2ray-rules-dat/releases/latest/download/geosite.dat || true
  rm -rf "$TMP_DIR"
}

generate_mtls_certs() {
  echo -e "${YELLOW}Generating mTLS certificates...${NC}"
  # CA
  if [ ! -f "$CERT_DIR/ca.crt" ] || [ ! -f "$CERT_DIR/ca.key" ]; then
    openssl req -x509 -nodes -newkey rsa:4096 -days 365 \
      -subj "/ST=Test/L=Test/O=3x-ui-agent/OU=Agents/CN=agent-ca" \
      -keyout "$CERT_DIR/ca.key" -out "$CERT_DIR/ca.crt"
  fi

  # Agent cert
  openssl req -new -nodes -newkey rsa:2048 \
    -subj "/ST=Test/L=Test/O=3x-ui-agent/OU=Agents/CN=$AGENT_HOST_IP" \
    -keyout "$CERT_DIR/agent.key" -out "$CERT_DIR/agent.csr"

  openssl x509 -req -in "$CERT_DIR/agent.csr" -days 365 \
    -CA "$CERT_DIR/ca.crt" -CAkey "$CERT_DIR/ca.key" -CAcreateserial \
    -out "$CERT_DIR/agent.crt" \
    -extfile <(printf "subjectAltName=IP:%s" "$AGENT_HOST_IP")

  rm -f "$CERT_DIR/agent.csr"
  chmod 600 "$CERT_DIR/agent.key"
  chmod 644 "$CERT_DIR/agent.crt" "$CERT_DIR/ca.crt"
  echo -e "${GREEN}mTLS certificates created in $CERT_DIR${NC}"
}

generate_jwt_token() {
  JWT_TOKEN=$(openssl rand -hex 32)
  echo "$JWT_TOKEN" > "$CONFIG_DIR/agent.jwt"
  chmod 600 "$CONFIG_DIR/agent.jwt"
  echo -e "${GREEN}JWT token generated: $JWT_TOKEN${NC}"
  echo -e "${YELLOW}Token saved to $CONFIG_DIR/agent.jwt${NC}"
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
WorkingDirectory=$APP_DIR
ExecStart=$APP_DIR/x-ui agent
Restart=on-failure
RestartSec=10s

# Environment variables (customize as needed)
Environment="AGENT_LISTEN_ADDR=0.0.0.0:2054"
Environment="AGENT_AUTH_TYPE=$AUTH_TYPE"
Environment="AGENT_CERT_FILE=$CERT_DIR/agent.crt"
Environment="AGENT_KEY_FILE=$CERT_DIR/agent.key"
Environment="AGENT_CA_FILE=$CERT_DIR/ca.crt"
Environment="AGENT_JWT_TOKEN=$JWT_TOKEN"
Environment="AGENT_LOG_LEVEL=info"
Environment="AGENT_LOG_FILE=$LOG_DIR/agent.log"
Environment="AGENT_RATE_LIMIT=100"
Environment="XUI_BIN_FOLDER=$BIN_DIR"
Environment="XUI_DB_FOLDER=/etc/x-ui"
Environment="XUI_LOG_FOLDER=$LOG_DIR"

# Security
ProtectSystem=full
ReadWritePaths=/etc/x-ui /var/log/x-ui-agent /usr/local/x-ui-agent/bin /usr/local/x-ui-agent
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

choose_auth_type() {
  echo -e "${YELLOW}Select authentication method for agent:${NC}"
  echo "1) mTLS (certs will be generated automatically)"
  echo "2) JWT (random token will be generated)"
  read -rp "Choose [1-2]: " auth_choice
  case "$auth_choice" in
    1)
      AUTH_TYPE="mtls"
      generate_mtls_certs
      ;;
    2)
      AUTH_TYPE="jwt"
      generate_jwt_token
      ;;
    *)
      echo -e "${RED}Invalid choice, defaulting to mTLS${NC}"
      AUTH_TYPE="mtls"
      generate_mtls_certs
      ;;
  esac
}

# Display next steps
display_next_steps() {
  echo ""
  echo -e "${GREEN}=== Installation Complete ===${NC}"
  echo ""
  echo "Service: $SERVICE_NAME (port 2054)"
  echo "Agent version: $RESOLVED_AGENT_VERSION"
  echo "Xray version: $XRAY_VERSION"
  echo "Auth: $AUTH_TYPE"
  [ "$AUTH_TYPE" = "jwt" ] && echo "JWT token saved to $CONFIG_DIR/agent.jwt"
  [ "$AUTH_TYPE" = "mtls" ] && echo "mTLS certs in $CERT_DIR (agent.crt/key, ca.crt, SAN IP: $AGENT_HOST_IP)"
}

# Main installation flow
main() {
  detect_system
  detect_ip
  install_dependencies
  stop_existing_service
  download_agent
  create_directories
  ensure_xray_assets
  choose_auth_type
  create_service
  configure_firewall
  display_next_steps
}

# Run main function
main

# Enable and restart service after installation (ensures new binary is picked up)
systemctl daemon-reload
systemctl enable $SERVICE_NAME || true
systemctl restart $SERVICE_NAME || true

exit 0
