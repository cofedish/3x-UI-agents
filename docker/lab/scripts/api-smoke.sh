#!/usr/bin/env bash
set -euo pipefail

log() { echo "[api-smoke] $*"; }

PANEL_URL="${PANEL_URL:-http://panel:2053/lab}"
PANEL_USERNAME="${PANEL_USERNAME:-admin}"
PANEL_PASSWORD="${PANEL_PASSWORD:-admin123}"
COOKIE_FILE=$(mktemp)
trap 'rm -f "$COOKIE_FILE"' EXIT

login() {
  curl -sS -c "$COOKIE_FILE" -b "$COOKIE_FILE" \
    -X POST "$PANEL_URL/login" \
    -d "username=$PANEL_USERNAME" \
    -d "password=$PANEL_PASSWORD" >/dev/null
}

json_get() {
  curl -sS -b "$COOKIE_FILE" "$PANEL_URL$1"
}

json_post() {
  local path="$1"
  local payload="$2"
  curl -sS -b "$COOKIE_FILE" -H "Content-Type: application/json" -d "$payload" "$PANEL_URL$path"
}

login

servers_json=$(json_get "/panel/api/servers")
server_total=$(echo "$servers_json" | jq -r '.obj.total')
log "Servers total: $server_total"
if [ "$server_total" -lt 3 ]; then
  log "Expected at least 3 servers (local + 2 agents)"
  exit 1
fi

mtls_id=$(echo "$servers_json" | jq -r '.obj.servers[] | select(.endpoint=="https://agent-mtls:2054") | .id' | head -n1)
jwt_id=$(echo "$servers_json" | jq -r '.obj.servers[] | select(.endpoint=="https://agent-jwt:2054") | .id' | head -n1)

for server_id in 1 "$mtls_id" "$jwt_id"; do
  if [ -z "$server_id" ] || [ "$server_id" = "null" ]; then
    continue
  fi
  status_json=$(json_get "/panel/api/server/status?server_id=$server_id")
  if ! echo "$status_json" | jq -e '.success == true' >/dev/null; then
    log "Status fetch failed for server_id=$server_id"
    exit 1
  fi
  xray_state=$(echo "$status_json" | jq -r '.obj.xray.state')
  if [ "$xray_state" = "error" ]; then
    log "Xray state error for server_id=$server_id"
    exit 1
  fi
  health_json=$(json_get "/panel/api/servers/$server_id/health")
  if ! echo "$health_json" | jq -e '.success == true' >/dev/null; then
    log "Health check failed for server_id=$server_id"
    exit 1
  fi
  info_json=$(json_get "/panel/api/servers/$server_id/info")
  if ! echo "$info_json" | jq -e '.success == true' >/dev/null; then
    log "Info check failed for server_id=$server_id"
    exit 1
  fi
  restart_resp=$(json_post "/panel/api/server/restartXrayService?server_id=$server_id" "{}")
  if ! echo "$restart_resp" | jq -e '.success == true' >/dev/null; then
    log "Xray restart failed for server_id=$server_id"
    exit 1
  fi
  logs_resp=$(json_post "/panel/api/server/logs/50?server_id=$server_id" "{}")
  if ! echo "$logs_resp" | jq -e '.success == true' >/dev/null; then
    log "Logs fetch failed for server_id=$server_id"
    exit 1
  fi
  xray_logs_resp=$(json_post "/panel/api/server/xraylogs/50?server_id=$server_id" "{}")
  if ! echo "$xray_logs_resp" | jq -e '.success == true' >/dev/null; then
    log "Xray logs fetch failed for server_id=$server_id"
    exit 1
  fi
  config_resp=$(json_get "/panel/api/server/getConfigJson?server_id=$server_id")
  if ! echo "$config_resp" | jq -e '.success == true' >/dev/null; then
    log "Config json fetch failed for server_id=$server_id"
    exit 1
  fi
  version_resp=$(json_get "/panel/api/server/getXrayVersion?server_id=$server_id")
  if ! echo "$version_resp" | jq -e '.success == true' >/dev/null; then
    log "Xray version fetch failed for server_id=$server_id"
    exit 1
  fi
  update_geo=$(json_post "/panel/api/server/updateGeofile?server_id=$server_id" "{}")
  if ! echo "$update_geo" | jq -e '.success == true' >/dev/null; then
    log "Geo update failed for server_id=$server_id"
    exit 1
  fi
  install_uuid=$(json_get "/panel/api/server/getNewUUID?server_id=$server_id")
  if ! echo "$install_uuid" | jq -e '.success == true' >/dev/null; then
    log "UUID fetch failed for server_id=$server_id"
    exit 1
  fi

done

aggregated_json=$(json_get "/panel/api/server/aggregatedStatus")
if ! echo "$aggregated_json" | jq -e '.success == true' >/dev/null; then
  log "Aggregated status failed"
  exit 1
fi

stats_json=$(json_get "/panel/api/servers/stats")
if ! echo "$stats_json" | jq -e '.success == true' >/dev/null; then
  log "Server stats failed"
  exit 1
fi

settings_json=$(json_post "/panel/api/setting/all" "{}")
if ! echo "$settings_json" | jq -e '.success == true' >/dev/null; then
  log "Settings fetch failed"
  exit 1
fi

orig_settings=$(echo "$settings_json" | jq '.obj')
new_page_size=$(echo "$orig_settings" | jq -r '.pageSize')
new_page_size=$((new_page_size + 1))
updated_settings=$(echo "$orig_settings" | jq --argjson pageSize "$new_page_size" '.pageSize=$pageSize')
update_resp=$(json_post "/panel/api/setting/update" "$updated_settings")
if ! echo "$update_resp" | jq -e '.success == true' >/dev/null; then
  log "Settings update failed"
  exit 1
fi
revert_resp=$(json_post "/panel/api/setting/update" "$orig_settings")
if ! echo "$revert_resp" | jq -e '.success == true' >/dev/null; then
  log "Settings revert failed"
  exit 1
fi

inbound_port=12345
inbound_uuid=$(cat /proc/sys/kernel/random/uuid)
client_uuid=$(cat /proc/sys/kernel/random/uuid)

create_payload=$(jq -n \
  --arg remark "lab-inbound" \
  --arg tag "lab-inbound" \
  --argjson port "$inbound_port" \
  --arg protocol "vmess" \
  --arg settings "{\"clients\":[{\"id\":\"$inbound_uuid\",\"alterId\":0,\"email\":\"lab1@local\",\"enable\":true}],\"disableInsecureEncryption\":true}" \
  --arg streamSettings "{\"network\":\"tcp\",\"security\":\"none\"}" \
  --arg sniffing "{\"enabled\":true,\"destOverride\":[\"http\",\"tls\"]}" \
  '{remark:$remark, tag:$tag, port:$port, protocol:$protocol, settings:$settings, streamSettings:$streamSettings, sniffing:$sniffing, enable:true, total:0, expiryTime:0, listen:"", serverId:1}')

create_resp=$(json_post "/panel/api/inbounds/add?server_id=1" "$create_payload")
if ! echo "$create_resp" | jq -e '.success == true' >/dev/null; then
  log "Inbound create failed"
  echo "$create_resp"
  exit 1
fi

inbound_id=$(echo "$create_resp" | jq -r '.obj.id')
if [ -z "$inbound_id" ] || [ "$inbound_id" = "null" ]; then
  log "Inbound id missing"
  exit 1
fi

update_payload=$(echo "$create_payload" | jq --arg remark "lab-inbound-updated" '.remark=$remark')
update_resp=$(json_post "/panel/api/inbounds/update/$inbound_id?server_id=1" "$update_payload")
if ! echo "$update_resp" | jq -e '.success == true' >/dev/null; then
  log "Inbound update failed"
  exit 1
fi

add_client_payload=$(jq -n \
  --argjson id "$inbound_id" \
  --arg settings "{\"clients\":[{\"id\":\"$client_uuid\",\"alterId\":0,\"email\":\"lab2@local\",\"enable\":true}],\"disableInsecureEncryption\":true}" \
  '{id:$id, settings:$settings}')

add_client_resp=$(json_post "/panel/api/inbounds/addClient?server_id=1" "$add_client_payload")
if ! echo "$add_client_resp" | jq -e '.success == true' >/dev/null; then
  log "Add client failed"
  exit 1
fi

update_client_payload=$(jq -n \
  --argjson id "$inbound_id" \
  --arg settings "{\"clients\":[{\"id\":\"$client_uuid\",\"alterId\":0,\"email\":\"lab2-updated@local\",\"enable\":true}],\"disableInsecureEncryption\":true}" \
  '{id:$id, settings:$settings}')

update_client_resp=$(json_post "/panel/api/inbounds/updateClient/$client_uuid?server_id=1" "$update_client_payload")
if ! echo "$update_client_resp" | jq -e '.success == true' >/dev/null; then
  log "Update client failed"
  exit 1
fi

delete_client_resp=$(json_post "/panel/api/inbounds/$inbound_id/delClient/$client_uuid?server_id=1" "{}")
if ! echo "$delete_client_resp" | jq -e '.success == true' >/dev/null; then
  log "Delete client failed"
  exit 1
fi

inbounds_list=$(json_get "/panel/api/inbounds/list?server_id=1")
export_payload=$(echo "$inbounds_list" | jq -c '.obj[0]')
delete_inbound_resp=$(json_post "/panel/api/inbounds/del/$inbound_id?server_id=1" "{}")
if ! echo "$delete_inbound_resp" | jq -e '.success == true' >/dev/null; then
  log "Inbound delete failed"
  exit 1
fi

if [ -n "$export_payload" ] && [ "$export_payload" != "null" ]; then
  import_resp=$(curl -sS -b "$COOKIE_FILE" -F "data=$export_payload" "$PANEL_URL/panel/api/inbounds/import")
  if ! echo "$import_resp" | jq -e '.success == true' >/dev/null; then
    log "Inbound import failed"
    exit 1
  fi
  import_id=$(echo "$import_resp" | jq -r '.obj.id')
  if [ -n "$import_id" ] && [ "$import_id" != "null" ]; then
    import_delete=$(json_post "/panel/api/inbounds/del/$import_id?server_id=1" "{}")
    if ! echo "$import_delete" | jq -e '.success == true' >/dev/null; then
      log "Imported inbound delete failed"
      exit 1
    fi
  fi
fi

db_file=$(mktemp)
curl -sS -b "$COOKIE_FILE" "$PANEL_URL/panel/api/server/getDb?server_id=1" -o "$db_file"
if [ ! -s "$db_file" ]; then
  log "Database export is empty"
  exit 1
fi

import_db_resp=$(curl -sS -b "$COOKIE_FILE" -F "db=@$db_file" "$PANEL_URL/panel/api/server/importDB?server_id=1")
if ! echo "$import_db_resp" | jq -e '.success == true' >/dev/null; then
  log "Database import failed"
  exit 1
fi
rm -f "$db_file"

log "API smoke tests passed"
