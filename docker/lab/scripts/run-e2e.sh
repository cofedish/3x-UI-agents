#!/usr/bin/env bash
set -euo pipefail

ARTIFACTS_DIR="${ARTIFACTS_DIR:-/artifacts/last-run}"
LAB_SHARED="${LAB_SHARED:-/opt/lab/shared}"
mkdir -p "$ARTIFACTS_DIR" "$ARTIFACTS_DIR/playwright" "$ARTIFACTS_DIR/docker-logs"

{
  echo "Container OS: $(uname -a)"
  echo "Node: $(node -v 2>/dev/null || echo 'not found')"
  echo "NPM: $(npm -v 2>/dev/null || echo 'not found')"
  echo "Playwright: $(npx playwright --version 2>/dev/null || echo 'not found')"
  echo "Curl: $(curl --version 2>/dev/null | head -n1 || echo 'not found')"
} > "$ARTIFACTS_DIR/env-summary.txt"

if [ -f "$LAB_SHARED/panel.env" ]; then
  set -a
  # shellcheck disable=SC1090
  source "$LAB_SHARED/panel.env"
  set +a
fi

export PANEL_URL="${PANEL_URL:-http://panel:2053/lab}"
export PANEL_USERNAME="${PANEL_USERNAME:-admin}"
export PANEL_PASSWORD="${PANEL_PASSWORD:-admin123}"

for _ in {1..60}; do
  if curl -fsS "$PANEL_URL/" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

wait_status="PASS"
if ! /work/docker/lab/scripts/wait-ready.sh >"$ARTIFACTS_DIR/wait-ready.log" 2>&1; then
  wait_status="FAIL"
fi

api_status="PASS"
if ! /work/docker/lab/scripts/api-smoke.sh >"$ARTIFACTS_DIR/api-smoke.log" 2>&1; then
  api_status="FAIL"
fi

probe_status="PASS"
if ! /work/docker/lab/scripts/protocol-probes.sh >"$ARTIFACTS_DIR/protocol-probes.log" 2>&1; then
  probe_status="FAIL"
fi

contract_status="PASS"
if ! /work/docker/lab/scripts/api-contract.sh >"$ARTIFACTS_DIR/api-contract.log" 2>&1; then
  contract_status="FAIL"
fi

cd /work/test/e2e
if [ ! -d node_modules ]; then
  npm install --no-fund --no-audit >/dev/null
fi

pw_status="PASS"
if ! npx playwright test --config=playwright.config.js >"$ARTIFACTS_DIR/playwright.log" 2>&1; then
  pw_status="FAIL"
fi

dashboard_fix="$contract_status"
connections_fix="$probe_status"
remote_xray_fix="$probe_status"
translations_fix="$pw_status"
servers_icon_fix="$pw_status"

cat > "$ARTIFACTS_DIR/report.md" <<EOF
# Lab Test Report

- Panel URL: $PANEL_URL
- Wait ready: $wait_status
- API Smoke: $api_status
- API Contract: $contract_status
- Protocol Probes: $probe_status
- Playwright: $pw_status

## Known bugs
- Dashboard metrics/logs/xray uptime: $dashboard_fix
- Connections traffic non-zero: $connections_fix
- Remote Xray apply/reload: $remote_xray_fix
- Translations completeness: $translations_fix
- Servers tab icon: $servers_icon_fix

## Checklist
- Dashboard: status cards + xray controls
- Servers: both agents online
- Inbounds: CRUD + clients
- Logs: panel + xray
- Backup/Restore: export + import
- Settings: safe toggle + revert
- Xray: restart/reload
- UI crawl: all routes + no 404/500
- Protocol probes: vmess/vmess-apply/vless/trojan/shadowsocks/mixed/http/tunnel/wireguard

Artifacts:
- api-smoke.log
- api-contract.log
- playwright.log
- playwright/ (screenshots, traces)
- protocol-probes.log
- protocol-probes-summary.md
- docker-logs/ (container logs)
- env-summary.txt
EOF

if [ -f "$ARTIFACTS_DIR/protocol-probes-summary.md" ]; then
  echo "" >> "$ARTIFACTS_DIR/report.md"
  echo "## Protocol probe summary" >> "$ARTIFACTS_DIR/report.md"
  cat "$ARTIFACTS_DIR/protocol-probes-summary.md" >> "$ARTIFACTS_DIR/report.md"
fi

if [ "$wait_status" != "PASS" ] || [ "$api_status" != "PASS" ] || [ "$contract_status" != "PASS" ] || [ "$probe_status" != "PASS" ] || [ "$pw_status" != "PASS" ]; then
  exit 1
fi
