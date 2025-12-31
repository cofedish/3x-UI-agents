#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[lab-panel] $*"
}

LAB_SHARED="${LAB_SHARED:-/opt/lab/shared}"
PANEL_PORT="${PANEL_PORT:-2053}"
PANEL_BASE_PATH="${PANEL_BASE_PATH:-lab}"
PANEL_USERNAME="${PANEL_USERNAME:-admin}"
PANEL_PASSWORD="${PANEL_PASSWORD:-admin123}"
PANEL_LISTEN_IP="${PANEL_LISTEN_IP:-0.0.0.0}"
PANEL_HOST="${PANEL_HOST:-panel}"
CERT_DIR="$LAB_SHARED/certs"

mkdir -p "$LAB_SHARED"

if [ ! -x /usr/local/x-ui/x-ui ]; then
  log "Installing panel"
  (cd /opt/3x-ui && \
    MODE=panel \
    PANEL_PORT="$PANEL_PORT" \
    PANEL_USER="$PANEL_USERNAME" \
    PANEL_PASS="$PANEL_PASSWORD" \
    PANEL_BASE_PATH="$PANEL_BASE_PATH" \
    PANEL_LISTEN_IP="$PANEL_LISTEN_IP" \
    bash ./install-noninteractive.sh)
else
  log "Panel already installed"
fi

if [ -f "$CERT_DIR/ca.crt" ]; then
  log "Installing lab CA into trust store"
  cp "$CERT_DIR/ca.crt" /usr/local/share/ca-certificates/lab-ca.crt
  update-ca-certificates >/dev/null 2>&1 || true
fi

BASE_URL="http://127.0.0.1:${PANEL_PORT}/${PANEL_BASE_PATH}"
PANEL_URL="http://${PANEL_HOST}:${PANEL_PORT}/${PANEL_BASE_PATH}"
log "Waiting for panel at $BASE_URL"
for _ in {1..60}; do
  if curl -fsS "$BASE_URL/" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

/usr/local/bin/register-agents.sh

cat > "$LAB_SHARED/panel.env" <<EOF
PANEL_URL=${PANEL_URL}
PANEL_USERNAME=${PANEL_USERNAME}
PANEL_PASSWORD=${PANEL_PASSWORD}
PANEL_BASE_PATH=${PANEL_BASE_PATH}
PANEL_PORT=${PANEL_PORT}
EOF

log "Bootstrap complete"
