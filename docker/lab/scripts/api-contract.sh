#!/usr/bin/env bash
set -euo pipefail

log() { echo "[api-contract] $*"; }

PANEL_URL="${PANEL_URL:-http://panel:2053/lab}"
PANEL_USERNAME="${PANEL_USERNAME:-admin}"
PANEL_PASSWORD="${PANEL_PASSWORD:-admin123}"
COOKIE_FILE=$(mktemp)
trap 'rm -f "$COOKIE_FILE"' EXIT

login() {
  curl -fsS -c "$COOKIE_FILE" -b "$COOKIE_FILE" \
    -X POST "$PANEL_URL/login" \
    -d "username=$PANEL_USERNAME" \
    -d "password=$PANEL_PASSWORD" >/dev/null
}

json_get() {
  curl -fsS -b "$COOKIE_FILE" "$PANEL_URL$1"
}

json_post() {
  local path="$1"
  local payload="$2"
  curl -fsS -b "$COOKIE_FILE" -H "Content-Type: application/json" -d "$payload" "$PANEL_URL$path"
}

require_field() {
  local json="$1"
  local jq_expr="$2"
  local label="$3"
  if ! echo "$json" | jq -e "$jq_expr" >/dev/null; then
    log "Missing/invalid field: $label"
    exit 1
  fi
}

login

servers_json=$(json_get "/panel/api/servers")
if ! echo "$servers_json" | jq -e '.success == true' >/dev/null; then
  log "Servers list failed"
  exit 1
fi

server_ids=$(echo "$servers_json" | jq -r '[1, (.obj.servers[]?.id // empty)] | unique | .[]')

for server_id in $server_ids; do
  status_json=$(json_get "/panel/api/server/status?server_id=$server_id")
  if ! echo "$status_json" | jq -e '.success == true' >/dev/null; then
    log "Status failed for server_id=$server_id"
    exit 1
  fi

  require_field "$status_json" '.obj.uptime >= 0' "uptime ($server_id)"
  require_field "$status_json" '.obj.netIO.up >= 0 and .obj.netIO.down >= 0' "netIO ($server_id)"
  require_field "$status_json" '.obj.netTraffic.sent >= 0 and .obj.netTraffic.recv >= 0' "netTraffic ($server_id)"
  require_field "$status_json" '.obj.appStats.threads >= 0' "appStats.threads ($server_id)"
  require_field "$status_json" '.obj.appStats.mem >= 0' "appStats.mem ($server_id)"
  require_field "$status_json" '.obj.appStats.uptime > 0' "appStats.uptime ($server_id)"
  require_field "$status_json" '.obj.xray.state == "running"' "xray.state ($server_id)"

  logs_resp=$(json_post "/panel/api/server/logs/50?server_id=$server_id" "{}")
  if ! echo "$logs_resp" | jq -e '.success == true' >/dev/null; then
    log "Logs failed for server_id=$server_id"
    exit 1
  fi
  if ! echo "$logs_resp" | jq -e '.obj | length > 0' >/dev/null; then
    log "Logs empty for server_id=$server_id"
    exit 1
  fi

  xray_logs_resp=$(json_post "/panel/api/server/xraylogs/50?server_id=$server_id" "{}")
  if ! echo "$xray_logs_resp" | jq -e '.success == true' >/dev/null; then
    log "Xray logs failed for server_id=$server_id"
    exit 1
  fi
  if ! echo "$xray_logs_resp" | jq -e '.obj | length > 0' >/dev/null; then
    log "Xray logs empty for server_id=$server_id"
    exit 1
  fi

done

aggregated_json=$(json_get "/panel/api/server/aggregatedStatus")
if ! echo "$aggregated_json" | jq -e '.success == true' >/dev/null; then
  log "Aggregated status failed"
  exit 1
fi
require_field "$aggregated_json" '.obj._aggregated.totalServers >= 1' "aggregated totalServers"
require_field "$aggregated_json" '.obj.netTraffic.sent >= 0' "aggregated netTraffic"

log "API contract checks passed"
