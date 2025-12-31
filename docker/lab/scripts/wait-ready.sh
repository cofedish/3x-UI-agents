#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[lab-wait] $*"
}

PANEL_URL="${PANEL_URL:-http://panel:2053/lab}"
PANEL_USERNAME="${PANEL_USERNAME:-admin}"
PANEL_PASSWORD="${PANEL_PASSWORD:-admin123}"
PANEL_WAIT_SECS="${PANEL_WAIT_SECS:-600}"
COOKIE_FILE=$(mktemp)
trap 'rm -f "$COOKIE_FILE"' EXIT

login() {
  curl -fsS -c "$COOKIE_FILE" -b "$COOKIE_FILE" \
    -X POST "$PANEL_URL/login" \
    -d "username=$PANEL_USERNAME" \
    -d "password=$PANEL_PASSWORD" >/dev/null
}

get_servers() {
  curl -fsS -b "$COOKIE_FILE" "$PANEL_URL/panel/api/servers"
}

wait_for_panel_login() {
  local start
  start=$(date +%s)
  while true; do
    if login; then
      return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$PANEL_WAIT_SECS" ]; then
      log "Panel login not ready after ${PANEL_WAIT_SECS}s"
      return 1
    fi
    sleep 5
  done
}

wait_for_online() {
  local endpoint="$1"
  local timeout="$2"
  local start
  start=$(date +%s)

  while true; do
    local servers_json
    servers_json=$(get_servers)
    local status
    status=$(echo "$servers_json" | jq -r --arg endpoint "$endpoint" '.obj.servers[]? | select(.endpoint==$endpoint) | .status' | head -n1)
    if [ "$status" = "online" ]; then
      log "Online: $endpoint"
      return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      log "Timeout waiting for $endpoint (last status: ${status:-missing})"
      return 1
    fi
    sleep 5
  done
}

if ! wait_for_panel_login; then
  exit 1
fi

for _ in {1..60}; do
  if get_servers | jq -e '.success == true' >/dev/null 2>&1; then
    break
  fi
  sleep 5
done

if ! get_servers | jq -e '.success == true' >/dev/null 2>&1; then
  log "Panel API not ready"
  exit 1
fi

wait_for_online "https://agent-mtls:2054" 180
wait_for_online "https://agent-jwt:2054" 180

log "All servers online"
