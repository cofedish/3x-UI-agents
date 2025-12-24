#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'EOF'
Usage: install-noninteractive.sh [options]

Options:
  --mode panel|agent         Install panel or agent (default: panel)
  --panel-port PORT          Panel port (default: 2053)
  --panel-user USER          Panel username (default: admin)
  --panel-pass PASS          Panel password (default: admin123)
  --panel-base-path PATH     Panel base path (default: lab)
  --panel-listen-ip IP       Panel listen IP (default: 0.0.0.0)
  --panel-version VERSION    Install specific panel version
  --agent-auth mtls|jwt       Agent auth type (default: mtls)
  --agent-server-id ID       Agent server ID
  --agent-server-name NAME   Agent server name
  --agent-host-ip IP         Agent host IP for cert SAN
  --agent-cert FILE          Agent cert path (preseed)
  --agent-key FILE           Agent key path (preseed)
  --agent-ca FILE            Agent CA path (preseed)
  --agent-jwt-secret TOKEN   Agent JWT secret/token
  -h, --help                 Show this help

Environment variables mirror flags:
  MODE, PANEL_PORT, PANEL_USER, PANEL_PASS, PANEL_USERNAME, PANEL_PASSWORD,
  PANEL_BASE_PATH, PANEL_LISTEN_IP, PANEL_VERSION
  AGENT_AUTH_TYPE, AGENT_SERVER_ID, AGENT_SERVER_NAME, AGENT_HOST_IP
  AGENT_CERT_FILE, AGENT_KEY_FILE, AGENT_CA_FILE, AGENT_JWT_SECRET
EOF
}

MODE="${MODE:-panel}"
PANEL_PORT="${PANEL_PORT:-2053}"
PANEL_USER="${PANEL_USER:-${PANEL_USERNAME:-admin}}"
PANEL_PASS="${PANEL_PASS:-${PANEL_PASSWORD:-admin123}}"
PANEL_BASE_PATH="${PANEL_BASE_PATH:-lab}"
PANEL_LISTEN_IP="${PANEL_LISTEN_IP:-0.0.0.0}"
PANEL_VERSION="${PANEL_VERSION:-}"

AGENT_AUTH_TYPE="${AGENT_AUTH_TYPE:-mtls}"
AGENT_SERVER_ID="${AGENT_SERVER_ID:-}"
AGENT_SERVER_NAME="${AGENT_SERVER_NAME:-}"
AGENT_HOST_IP="${AGENT_HOST_IP:-}"
AGENT_CERT_FILE="${AGENT_CERT_FILE:-}"
AGENT_KEY_FILE="${AGENT_KEY_FILE:-}"
AGENT_CA_FILE="${AGENT_CA_FILE:-}"
AGENT_JWT_SECRET="${AGENT_JWT_SECRET:-}"

while [ $# -gt 0 ]; do
  case "$1" in
    --mode) MODE="$2"; shift 2 ;;
    --panel-port) PANEL_PORT="$2"; shift 2 ;;
    --panel-user) PANEL_USER="$2"; shift 2 ;;
    --panel-pass) PANEL_PASS="$2"; shift 2 ;;
    --panel-base-path) PANEL_BASE_PATH="$2"; shift 2 ;;
    --panel-listen-ip) PANEL_LISTEN_IP="$2"; shift 2 ;;
    --panel-version) PANEL_VERSION="$2"; shift 2 ;;
    --agent-auth) AGENT_AUTH_TYPE="$2"; shift 2 ;;
    --agent-server-id) AGENT_SERVER_ID="$2"; shift 2 ;;
    --agent-server-name) AGENT_SERVER_NAME="$2"; shift 2 ;;
    --agent-host-ip) AGENT_HOST_IP="$2"; shift 2 ;;
    --agent-cert) AGENT_CERT_FILE="$2"; shift 2 ;;
    --agent-key) AGENT_KEY_FILE="$2"; shift 2 ;;
    --agent-ca) AGENT_CA_FILE="$2"; shift 2 ;;
    --agent-jwt-secret) AGENT_JWT_SECRET="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1"; usage; exit 1 ;;
  esac
  done

install_panel() {
  echo "[install-ci] Installing panel"

  if [ -n "$PANEL_VERSION" ]; then
    printf 'n\n' | bash ./install.sh "$PANEL_VERSION"
  else
    printf '1\nn\n' | bash ./install.sh
  fi

  /usr/local/x-ui/x-ui setting \
    -username "$PANEL_USER" \
    -password "$PANEL_PASS" \
    -port "$PANEL_PORT" \
    -webBasePath "$PANEL_BASE_PATH"

  if [ -n "$PANEL_LISTEN_IP" ]; then
    /usr/local/x-ui/x-ui setting -listenIP "$PANEL_LISTEN_IP"
  fi

  systemctl restart x-ui || true
}

install_agent() {
  echo "[install-ci] Installing agent"
  export AGENT_AUTH_TYPE
  export AGENT_SERVER_ID
  export AGENT_SERVER_NAME
  export AGENT_HOST_IP
  export AGENT_CERT_FILE
  export AGENT_KEY_FILE
  export AGENT_CA_FILE
  export AGENT_JWT_SECRET
  local auth_choice="1"
  if [ "$AGENT_AUTH_TYPE" = "jwt" ]; then
    auth_choice="2"
  fi
  printf "2\n%s\n" "$auth_choice" | bash ./install.sh
}

case "$MODE" in
  panel) install_panel ;;
  agent) install_agent ;;
  *) echo "Unknown MODE: $MODE"; usage; exit 1 ;;
esac
