#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[lab-agent] $*"
}

load_container_env() {
  local env_file="/proc/1/environ"
  [ -r "$env_file" ] || return 0
  while IFS= read -r -d '' entry; do
    case "$entry" in
      LAB_SHARED=*|AGENT_AUTH_TYPE=*|AGENT_CERT_NAME=*|AGENT_HOST_IP=*|AGENT_SERVER_ID=*|AGENT_SERVER_NAME=*)
        export "$entry"
        ;;
    esac
  done < "$env_file"
}

load_container_env

LAB_SHARED="${LAB_SHARED:-/opt/lab/shared}"
AGENT_AUTH_TYPE="${AGENT_AUTH_TYPE:-mtls}"
AGENT_CERT_NAME="${AGENT_CERT_NAME:-agent-mtls}"
AGENT_HOST_IP="${AGENT_HOST_IP:-127.0.0.1}"
AGENT_SERVER_ID="${AGENT_SERVER_ID:-$AGENT_CERT_NAME}"
AGENT_SERVER_NAME="${AGENT_SERVER_NAME:-$AGENT_CERT_NAME}"
CERT_DIR="$LAB_SHARED/certs"
JWT_TOKEN_FILE="$LAB_SHARED/jwt/token.txt"

mkdir -p "$LAB_SHARED"

if [ ! -x /usr/local/bin/x-ui-agent ] && [ ! -x /usr/local/x-ui-agent/x-ui ]; then
  log "Installing agent"
  MODE=agent \
    AGENT_AUTH_TYPE="$AGENT_AUTH_TYPE" \
    AGENT_SERVER_ID="$AGENT_SERVER_ID" \
    AGENT_SERVER_NAME="$AGENT_SERVER_NAME" \
    AGENT_HOST_IP="$AGENT_HOST_IP" \
    AGENT_CERT_FILE="$CERT_DIR/${AGENT_CERT_NAME}.crt" \
    AGENT_KEY_FILE="$CERT_DIR/${AGENT_CERT_NAME}.key" \
    AGENT_CA_FILE="$CERT_DIR/ca.crt" \
    AGENT_JWT_SECRET="$(cat "$JWT_TOKEN_FILE" 2>/dev/null || true)" \
    bash -lc "cd /opt/3x-ui && bash ./install-noninteractive.sh"
else
  log "Agent already installed"
fi

if [ -d /etc/x-ui-agent/certs ]; then
  cp "$CERT_DIR/${AGENT_CERT_NAME}.crt" /etc/x-ui-agent/certs/agent.crt
  cp "$CERT_DIR/${AGENT_CERT_NAME}.key" /etc/x-ui-agent/certs/agent.key
  cp "$CERT_DIR/ca.crt" /etc/x-ui-agent/certs/ca.crt
  chmod 600 /etc/x-ui-agent/certs/agent.key
  chmod 644 /etc/x-ui-agent/certs/agent.crt /etc/x-ui-agent/certs/ca.crt
fi

if [ "$AGENT_AUTH_TYPE" = "jwt" ] && [ -f "$JWT_TOKEN_FILE" ]; then
  mkdir -p /etc/x-ui-agent
  cp "$JWT_TOKEN_FILE" /etc/x-ui-agent/agent.jwt
  chmod 600 /etc/x-ui-agent/agent.jwt
fi

systemctl restart x-ui-agent || true

log "Bootstrap complete"
