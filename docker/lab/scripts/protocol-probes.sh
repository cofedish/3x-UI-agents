#!/usr/bin/env bash
set -euo pipefail

log() { echo "[protocol-probes] $*"; }

PANEL_URL="${PANEL_URL:-http://panel:2053/lab}"
PANEL_USERNAME="${PANEL_USERNAME:-admin}"
PANEL_PASSWORD="${PANEL_PASSWORD:-admin123}"
ARTIFACTS_DIR="${ARTIFACTS_DIR:-/artifacts/last-run}"
XRAY_APPLY_MODE="${XRAY_APPLY_MODE:-reload}"
COOKIE_FILE=$(mktemp)
TMP_DIR=$(mktemp -d)
SUMMARY_FILE="$ARTIFACTS_DIR/protocol-probes-summary.md"

trap 'rm -f "$COOKIE_FILE"; rm -rf "$TMP_DIR"' EXIT

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

urlencode() {
  jq -sRr @uri <<<"$1"
}

require_success() {
  local resp="$1"
  local label="$2"
  if ! echo "$resp" | jq -e '.success == true' >/dev/null; then
    log "$label failed"
    echo "$resp" >&2
    return 1
  fi
}

require_inbound_id() {
  local inbound_id="$1"
  local label="$2"
  if [ -z "$inbound_id" ] || [ "$inbound_id" = "null" ] || ! [[ "$inbound_id" =~ ^[0-9]+$ ]]; then
    log "$label missing inbound id ($inbound_id)"
    return 1
  fi
}

get_server_host() {
  local server_id="$1"
  if [ "$server_id" = "1" ]; then
    echo "panel"
    return
  fi
  echo "$SERVERS_JSON" | jq -r --argjson id "$server_id" '.obj.servers[]? | select(.id==$id) | .endpoint' \
    | sed -E 's#^https?://##' | cut -d/ -f1 | cut -d: -f1
}

apply_xray() {
  local server_id="$1"
  local endpoint="/panel/api/server/restartXrayService?server_id=$server_id"
  if [ "$XRAY_APPLY_MODE" = "reload" ]; then
    endpoint="/panel/api/server/reloadXrayService?server_id=$server_id"
  fi
  local resp
  resp=$(json_post "$endpoint" "{}")
  require_success "$resp" "apply xray ($server_id)"
}

create_inbound() {
  local server_id="$1"
  local protocol="$2"
  local port="$3"
  local remark="$4"
  local settings_json="$5"
  local stream_json="$6"
  local sniffing_json="$7"

  local payload
  payload=$(jq -n \
    --arg remark "$remark" \
    --arg tag "$remark" \
    --argjson port "$port" \
    --arg protocol "$protocol" \
    --arg settings "$settings_json" \
    --arg streamSettings "$stream_json" \
    --arg sniffing "$sniffing_json" \
    --argjson serverId "$server_id" \
    '{remark:$remark, tag:$tag, port:$port, protocol:$protocol, settings:$settings, streamSettings:$streamSettings, sniffing:$sniffing, enable:true, total:0, expiryTime:0, listen:"", serverId:$serverId}')

  local resp
  resp=$(json_post "/panel/api/inbounds/add?server_id=$server_id" "$payload")
  if ! require_success "$resp" "create inbound $protocol"; then
    return 1
  fi
  local inbound_id
  inbound_id=$(echo "$resp" | jq -r '.obj.id')
  if [ -z "$inbound_id" ] || [ "$inbound_id" = "null" ]; then
    log "create inbound $protocol missing id"
    echo "$resp" >&2
    return 1
  fi
  echo "$inbound_id"
}

update_inbound_port() {
  local server_id="$1"
  local inbound_id="$2"
  local new_port="$3"
  local new_remark="$4"
  local inbound_json
  inbound_json=$(json_get "/panel/api/inbounds/get/$inbound_id?server_id=$server_id")
  if ! require_success "$inbound_json" "get inbound $inbound_id"; then
    return 1
  fi
  local payload
  payload=$(echo "$inbound_json" | jq -c \
    --argjson port "$new_port" \
    --arg remark "$new_remark" \
    --arg tag "inbound-$new_port" \
    '.obj | .port=$port | .remark=$remark | .tag=$tag')
  local resp
  resp=$(json_post "/panel/api/inbounds/update/$inbound_id?server_id=$server_id" "$payload")
  require_success "$resp" "update inbound $inbound_id"
}

delete_inbound() {
  local server_id="$1"
  local inbound_id="$2"
  json_post "/panel/api/inbounds/del/$inbound_id?server_id=$server_id" "{}" >/dev/null
}

get_inbound_traffic() {
  local server_id="$1"
  local inbound_id="$2"
  local resp
  resp=$(json_get "/panel/api/inbounds/list?server_id=$server_id")
  echo "$resp" | jq -r --argjson id "$inbound_id" '.obj[]? | select(.id==$id) | (.up + .down)'
}

get_client_traffic() {
  local server_id="$1"
  local email="$2"
  local encoded
  encoded=$(urlencode "$email")
  local resp
  resp=$(json_get "/panel/api/inbounds/getClientTraffics/$encoded?server_id=$server_id")
  echo "$resp" | jq -r '.obj | if . == null then 0 else (.up + .down) end'
}

get_server_traffic() {
  local server_id="$1"
  local resp
  resp=$(json_get "/panel/api/server/status?server_id=$server_id")
  echo "$resp" | jq -r '.obj.netTraffic.sent + .obj.netTraffic.recv'
}

normalize_traffic() {
  local val="$1"
  if [ -z "$val" ] || [ "$val" = "null" ]; then
    echo 0
    return
  fi
  echo "$val"
}

wait_for_traffic() {
  local server_id="$1"
  local inbound_id="$2"
  local timeout=30
  local start
  start=$(date +%s)
  while true; do
    local val
    val=$(get_inbound_traffic "$server_id" "$inbound_id")
    if [ -n "$val" ] && [ "$val" != "null" ] && [ "$val" -gt 0 ]; then
      return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      return 1
    fi
    sleep 2
  done
}

wait_for_server_traffic() {
  local server_id="$1"
  local baseline="$2"
  local timeout=30
  local start
  start=$(date +%s)
  while true; do
    local val
    val=$(normalize_traffic "$(get_server_traffic "$server_id")")
    if [ -n "$val" ] && [ "$val" -gt "$baseline" ]; then
      return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      return 1
    fi
    sleep 2
  done
}

wait_for_client_traffic() {
  local server_id="$1"
  local email="$2"
  local timeout=30
  local start
  start=$(date +%s)
  while true; do
    local val
    val=$(get_client_traffic "$server_id" "$email")
    if [ -n "$val" ] && [ "$val" -gt 0 ]; then
      return 0
    fi
    if [ $(( $(date +%s) - start )) -ge "$timeout" ]; then
      return 1
    fi
    sleep 2
  done
}

probe_wait() {
  local server_id="$1"
  local inbound_id="$2"
  local baseline="$3"
  local email="$4"
  wait_for_traffic "$server_id" "$inbound_id" || return 1
  if [ -n "$email" ]; then
    wait_for_client_traffic "$server_id" "$email" || return 1
  fi
  wait_for_server_traffic "$server_id" "$baseline" || return 1
}

ensure_xray() {
  if [ -x "$TMP_DIR/xray/xray" ]; then
    XRAY_BIN="$TMP_DIR/xray/xray"
    return
  fi

  local version
  version=$(json_get "/panel/api/server/status?server_id=1" | jq -r '.obj.xray.version')
  if [ -z "$version" ] || [ "$version" = "Unknown" ] || [ "$version" = "null" ]; then
    version="1.8.8"
  fi
  if [[ "$version" != v* ]]; then
    version="v$version"
  fi

  local arch
  arch=$(uname -m)
  local asset="Xray-linux-64.zip"
  case "$arch" in
    aarch64|arm64) asset="Xray-linux-arm64-v8a.zip" ;;
    x86_64|amd64) asset="Xray-linux-64.zip" ;;
  esac

  download_xray() {
    local ver="$1"
    local url="https://github.com/XTLS/Xray-core/releases/download/${ver}/${asset}"
    log "Downloading xray $ver"
    curl -fSL \
      --retry 3 \
      --retry-delay 2 \
      --retry-all-errors \
      --connect-timeout 10 \
      --max-time 120 \
      -o "$TMP_DIR/xray.zip" "$url"
  }

  if ! download_xray "$version"; then
    local latest
    latest=$(curl -fsS https://api.github.com/repos/XTLS/Xray-core/releases/latest \
      | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/') || true
    if [ -z "$latest" ] || [ "$latest" = "null" ] || [ "$latest" = "$version" ]; then
      log "Failed to download xray $version"
      exit 1
    fi
    download_xray "$latest"
    version="$latest"
  fi
  mkdir -p "$TMP_DIR/xray"
  unzip -q "$TMP_DIR/xray.zip" -d "$TMP_DIR/xray"
  XRAY_BIN="$TMP_DIR/xray/xray"
  chmod +x "$XRAY_BIN"
}

run_xray_proxy() {
  local outbound_json="$1"
  local local_port="$2"
  local config="$TMP_DIR/xray-${local_port}.json"
  cat > "$config" <<EOF
{
  "log": {"loglevel": "warning"},
  "inbounds": [
    {"listen": "127.0.0.1", "port": ${local_port}, "protocol": "socks", "settings": {"udp": false}}
  ],
  "outbounds": [
    ${outbound_json}
  ]
}
EOF
  "$XRAY_BIN" run -c "$config" >/dev/null 2>&1 &
  local pid=$!
  sleep 1
  local curl_status=0
  curl -fsS --max-time 15 --socks5-hostname 127.0.0.1:"$local_port" http://echo:8080/ >/dev/null \
    || curl_status=$?
  kill "$pid" >/dev/null 2>&1 || true
  wait "$pid" >/dev/null 2>&1 || true
  return "$curl_status"
}

probe_result() {
  local server_id="$1"
  local protocol="$2"
  local status="$3"
  printf -- "- server_id=%s protocol=%s: %s\n" "$server_id" "$protocol" "$status" | tee -a "$SUMMARY_FILE"
}

probe_vmess() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local client_id
  client_id=$(cat /proc/sys/kernel/random/uuid)
  local email="lab-vmess@local"
  local settings
  settings=$(jq -c -n --arg id "$client_id" --arg email "$email" '{clients:[{id:$id, alterId:0, email:$email, security:"auto"}]}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:true, destOverride:["http","tls"]}')
  local remark="lab-probe-vmess-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "vmess" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "vmess inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  local outbound
  outbound=$(jq -c -n --arg host "$host" --argjson port "$port" --arg id "$client_id" '{protocol:"vmess", settings:{vnext:[{address:$host, port:$port, users:[{id:$id, alterId:0, security:"auto"}]}]}, streamSettings:{network:"tcp", security:"none"}}')
  run_xray_proxy "$outbound" 1081 || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

probe_vmess_apply_reload() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local client_id
  client_id=$(cat /proc/sys/kernel/random/uuid)
  local email="lab-vmess-apply@local"
  local settings
  settings=$(jq -c -n --arg id "$client_id" --arg email "$email" '{clients:[{id:$id, alterId:0, email:$email, security:"auto"}]}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:true, destOverride:["http","tls"]}')
  local remark="lab-apply-vmess-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "vmess" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "vmess apply inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  local outbound
  outbound=$(jq -c -n --arg host "$host" --argjson port "$port" --arg id "$client_id" '{protocol:"vmess", settings:{vnext:[{address:$host, port:$port, users:[{id:$id, alterId:0, security:"auto"}]}]}, streamSettings:{network:"tcp", security:"none"}}')
  run_xray_proxy "$outbound" 1091 || return 1
  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1

  local updated_port=$((port + 50))
  local updated_remark="lab-apply-vmess-$server_id-$updated_port"
  update_inbound_port "$server_id" "$inbound_id" "$updated_port" "$updated_remark" || return 1
  apply_xray "$server_id" || return 1
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  outbound=$(jq -c -n --arg host "$host" --argjson port "$updated_port" --arg id "$client_id" '{protocol:"vmess", settings:{vnext:[{address:$host, port:$port, users:[{id:$id, alterId:0, security:"auto"}]}]}, streamSettings:{network:"tcp", security:"none"}}')
  run_xray_proxy "$outbound" 1092 || return 1
  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1

  delete_inbound "$server_id" "$inbound_id"
}

probe_vless() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local client_id
  client_id=$(cat /proc/sys/kernel/random/uuid)
  local email="lab-vless@local"
  local settings
  settings=$(jq -c -n --arg id "$client_id" --arg email "$email" '{clients:[{id:$id, flow:"", email:$email, enable:true}], decryption:"none", encryption:"none"}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:true, destOverride:["http","tls"]}')
  local remark="lab-probe-vless-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "vless" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "vless inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  local outbound
  outbound=$(jq -c -n --arg host "$host" --argjson port "$port" --arg id "$client_id" '{protocol:"vless", settings:{vnext:[{address:$host, port:$port, users:[{id:$id, encryption:"none"}]}]}, streamSettings:{network:"tcp", security:"none"}}')
  run_xray_proxy "$outbound" 1082 || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

probe_trojan() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local password
  password=$(openssl rand -hex 8)
  local email="lab-trojan@local"
  local settings
  settings=$(jq -c -n --arg password "$password" --arg email "$email" '{clients:[{password:$password, email:$email, enable:true}], fallbacks:[]}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:true, destOverride:["http","tls"]}')
  local remark="lab-probe-trojan-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "trojan" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "trojan inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  local outbound
  outbound=$(jq -c -n --arg host "$host" --argjson port "$port" --arg password "$password" '{protocol:"trojan", settings:{servers:[{address:$host, port:$port, password:$password}]}, streamSettings:{network:"tcp", security:"none"}}')
  run_xray_proxy "$outbound" 1083 || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

probe_shadowsocks() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local password
  password=$(openssl rand -hex 8)
  local method="chacha20-ietf-poly1305"
  local email="lab-ss@local"
  local settings
  settings=$(jq -c -n --arg method "$method" --arg password "$password" --arg email "$email" '{method:$method, password:$password, network:"tcp", clients:[{method:$method, password:$password, email:$email, enable:true}]}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:true, destOverride:["http","tls"]}')
  local remark="lab-probe-ss-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "shadowsocks" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "shadowsocks inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  local outbound
  outbound=$(jq -c -n --arg host "$host" --argjson port "$port" --arg password "$password" --arg method "$method" '{protocol:"shadowsocks", settings:{servers:[{address:$host, port:$port, method:$method, password:$password}]}}')
  run_xray_proxy "$outbound" 1084 || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

probe_mixed() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local email=""
  local settings
  settings=$(jq -c -n '{auth:"noauth", accounts:[], udp:false, ip:"127.0.0.1"}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:true, destOverride:["http","tls"]}')
  local remark="lab-probe-mixed-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "mixed" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "mixed inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  curl -fsS --max-time 15 --socks5-hostname "$host:$port" http://echo:8080/ >/dev/null \
    || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

probe_http() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local user
  local pass
  user="labuser"
  pass="labpass"
  local email=""
  local settings
  settings=$(jq -c -n --arg user "$user" --arg pass "$pass" '{accounts:[{user:$user, pass:$pass}], allowTransparent:false}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:true, destOverride:["http","tls"]}')
  local remark="lab-probe-http-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "http" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "http inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  curl -fsS --max-time 15 -x "http://$user:$pass@$host:$port" http://echo:8080/ >/dev/null \
    || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

probe_tunnel() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local email=""
  local settings
  settings=$(jq -c -n '{address:"echo", port:8080, portMap:[], network:"tcp", followRedirect:false}')
  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:false}')
  local remark="lab-probe-tunnel-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "tunnel" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "tunnel inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  curl -fsS --max-time 15 "http://$host:$port" >/dev/null || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

probe_wireguard() {
  local server_id="$1"
  local host="$2"
  local port="$3"
  local server_priv client_priv server_pub client_pub
  server_priv=$(wg genkey)
  server_pub=$(echo "$server_priv" | wg pubkey)
  client_priv=$(wg genkey)
  client_pub=$(echo "$client_priv" | wg pubkey)
  local email=""

  local settings
  settings=$(jq -c -n \
    --arg secretKey "$server_priv" \
    --arg clientPriv "$client_priv" \
    --arg clientPub "$client_pub" \
    '{mtu:1420, secretKey:$secretKey, peers:[{privateKey:$clientPriv, publicKey:$clientPub, allowedIPs:["10.0.0.2/32"], keepAlive:5} ], noKernelTun:true}')

  local stream
  stream=$(jq -c -n '{network:"tcp", security:"none"}')
  local sniff
  sniff=$(jq -c -n '{enabled:false}')
  local remark="lab-probe-wireguard-$server_id-$port"
  local inbound_id
  inbound_id=$(create_inbound "$server_id" "wireguard" "$port" "$remark" "$settings" "$stream" "$sniff")
  require_inbound_id "$inbound_id" "wireguard inbound" || return 1
  apply_xray "$server_id" || return 1
  local baseline
  baseline=$(normalize_traffic "$(get_server_traffic "$server_id")")

  local outbound
  outbound=$(jq -c -n \
    --arg secretKey "$client_priv" \
    --arg endpoint "$host:$port" \
    --arg serverPub "$server_pub" \
    '{protocol:"wireguard", settings:{secretKey:$secretKey, address:["10.0.0.2/32"], peers:[{publicKey:$serverPub, allowedIPs:["0.0.0.0/0","::/0"], endpoint:$endpoint, keepAlive:5}], noKernelTun:true}}')
  run_xray_proxy "$outbound" 1085 || return 1

  probe_wait "$server_id" "$inbound_id" "$baseline" "$email" || return 1
  delete_inbound "$server_id" "$inbound_id"
}

login
SERVERS_JSON=$(json_get "/panel/api/servers")
require_success "$SERVERS_JSON" "servers list"

if [ -n "${PROBE_SERVER_IDS:-}" ]; then
  SERVER_IDS="$PROBE_SERVER_IDS"
else
  SERVER_IDS=$(echo "$SERVERS_JSON" | jq -r '[1, (.obj.servers[]?.id // empty)] | unique | .[]')
fi

ensure_xray

: > "$SUMMARY_FILE"

for server_id in $SERVER_IDS; do
  host=$(get_server_host "$server_id")
  if [ -z "$host" ]; then
    log "Missing host for server_id=$server_id"
    exit 1
  fi

  port_base=$((20000 + server_id * 100))

  if probe_vmess "$server_id" "$host" $((port_base + 1)); then probe_result "$server_id" "vmess" "PASS"; else probe_result "$server_id" "vmess" "FAIL"; exit 1; fi
  if probe_vmess_apply_reload "$server_id" "$host" $((port_base + 20)); then probe_result "$server_id" "vmess-apply" "PASS"; else probe_result "$server_id" "vmess-apply" "FAIL"; exit 1; fi
  if probe_vless "$server_id" "$host" $((port_base + 2)); then probe_result "$server_id" "vless" "PASS"; else probe_result "$server_id" "vless" "FAIL"; exit 1; fi
  if probe_trojan "$server_id" "$host" $((port_base + 3)); then probe_result "$server_id" "trojan" "PASS"; else probe_result "$server_id" "trojan" "FAIL"; exit 1; fi
  if probe_shadowsocks "$server_id" "$host" $((port_base + 4)); then probe_result "$server_id" "shadowsocks" "PASS"; else probe_result "$server_id" "shadowsocks" "FAIL"; exit 1; fi
  if probe_mixed "$server_id" "$host" $((port_base + 5)); then probe_result "$server_id" "mixed" "PASS"; else probe_result "$server_id" "mixed" "FAIL"; exit 1; fi
  if probe_http "$server_id" "$host" $((port_base + 6)); then probe_result "$server_id" "http" "PASS"; else probe_result "$server_id" "http" "FAIL"; exit 1; fi
  if probe_tunnel "$server_id" "$host" $((port_base + 7)); then probe_result "$server_id" "tunnel" "PASS"; else probe_result "$server_id" "tunnel" "FAIL"; exit 1; fi
  if probe_wireguard "$server_id" "$host" $((port_base + 8)); then probe_result "$server_id" "wireguard" "PASS"; else probe_result "$server_id" "wireguard" "FAIL"; exit 1; fi

done

log "Protocol probes passed"
