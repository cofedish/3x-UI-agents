#!/usr/bin/env bash
set -euo pipefail

log() {
  echo "[lab-panel-register] $*"
}

LAB_SHARED="${LAB_SHARED:-/opt/lab/shared}"
PANEL_PORT="${PANEL_PORT:-2053}"
PANEL_BASE_PATH="${PANEL_BASE_PATH:-lab}"
PANEL_USERNAME="${PANEL_USERNAME:-admin}"
PANEL_PASSWORD="${PANEL_PASSWORD:-admin123}"
BASE_URL="http://127.0.0.1:${PANEL_PORT}/${PANEL_BASE_PATH}"
CERT_DIR="$LAB_SHARED/certs"
JWT_TOKEN_FILE="$LAB_SHARED/jwt/token.txt"
COOKIE_FILE=$(mktemp)
trap 'rm -f "$COOKIE_FILE"' EXIT

login() {
  curl -sS -c "$COOKIE_FILE" -b "$COOKIE_FILE" \
    -X POST "$BASE_URL/login" \
    -d "username=$PANEL_USERNAME" \
    -d "password=$PANEL_PASSWORD" >/dev/null
}

get_servers() {
  curl -sS -b "$COOKIE_FILE" "$BASE_URL/panel/api/servers"
}

ensure_server() {
  local name="$1"
  local endpoint="$2"
  local auth_type="$3"
  local auth_data="$4"
  local tags="$5"

  local servers_json
  servers_json=$(get_servers)
  local existing_id
  existing_id=$(echo "$servers_json" | jq -r --arg endpoint "$endpoint" '.obj.servers[]? | select(.endpoint==$endpoint) | .id' | head -n1)

  if [ -n "$existing_id" ] && [ "$existing_id" != "null" ]; then
    log "Server already exists: $endpoint (id=$existing_id)"
    return
  fi

  local payload
  payload=$(jq -n \
    --arg name "$name" \
    --arg endpoint "$endpoint" \
    --arg authType "$auth_type" \
    --arg authData "$auth_data" \
    --arg tags "$tags" \
    '{name:$name, endpoint:$endpoint, authType:$authType, authData:$authData, enabled:true, tags:$tags}')

  curl -sS -b "$COOKIE_FILE" -H "Content-Type: application/json" \
    -d "$payload" "$BASE_URL/panel/api/servers" >/dev/null

  log "Added server: $endpoint"
}

wait_for_online() {
  local endpoint="$1"
  local timeout="${2:-120}"
  local start
  start=$(date +%s)

  while true; do
    local servers_json
    servers_json=$(get_servers)
    local status
    status=$(echo "$servers_json" | jq -r --arg endpoint "$endpoint" '.obj.servers[]? | select(.endpoint==$endpoint) | .status' | head -n1)
    if [ "$status" = "online" ]; then
      log "Server online: $endpoint"
      return
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      log "Timeout waiting for $endpoint to become online (last status: $status)"
      return 1
    fi
    sleep 5
  done
}

if [ ! -d "$CERT_DIR" ]; then
  log "Missing cert directory: $CERT_DIR"
  exit 1
fi

if [ ! -f "$CERT_DIR/panel-client.crt" ] || [ ! -f "$CERT_DIR/panel-client.key" ] || [ ! -f "$CERT_DIR/ca.crt" ]; then
  log "Missing panel mTLS materials"
  exit 1
fi

login
for _ in {1..30}; do
  if get_servers | jq -e '.success == true' >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

mtls_auth_json=$(jq -n \
  --arg cert "$(cat "$CERT_DIR/panel-client.crt")" \
  --arg key "$(cat "$CERT_DIR/panel-client.key")" \
  --arg ca "$(cat "$CERT_DIR/ca.crt")" \
  '{certPem:$cert, keyPem:$key, caPem:$ca}')

ensure_server "Lab Agent mTLS" "https://agent-mtls:2054" "mtls" "$mtls_auth_json" "[\"lab\",\"mtls\"]"

if [ -f "$JWT_TOKEN_FILE" ]; then
  jwt_token=$(cat "$JWT_TOKEN_FILE")
  ensure_server "Lab Agent JWT" "https://agent-jwt:2054" "jwt" "$jwt_token" "[\"lab\",\"jwt\"]"
fi

wait_for_online "https://agent-mtls:2054" || exit 1
wait_for_online "https://agent-jwt:2054" || exit 1

log "Server registration complete"
