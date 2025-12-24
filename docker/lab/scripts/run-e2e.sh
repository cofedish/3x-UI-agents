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

api_status="PASS"
if ! /work/docker/lab/scripts/api-smoke.sh >"$ARTIFACTS_DIR/api-smoke.log" 2>&1; then
  api_status="FAIL"
fi

cd /work/test/e2e
if [ ! -d node_modules ]; then
  npm install --no-fund --no-audit >/dev/null
fi

pw_status="PASS"
if ! npx playwright test --config=playwright.config.js --reporter=list >"$ARTIFACTS_DIR/playwright.log" 2>&1; then
  pw_status="FAIL"
fi

cat > "$ARTIFACTS_DIR/report.md" <<EOF
# Lab Test Report

- Panel URL: $PANEL_URL
- API Smoke: $api_status
- Playwright: $pw_status

## Checklist
- Dashboard: status cards + xray controls
- Servers: both agents online
- Inbounds: CRUD + clients
- Logs: panel + xray
- Backup/Restore: export + import
- Settings: safe toggle + revert
- Xray: restart/reload
- UI crawl: all routes + no 404/500

Artifacts:
- api-smoke.log
- playwright.log
- playwright/ (screenshots, traces)
- docker-logs/ (container logs)
- env-summary.txt
EOF

if [ "$api_status" != "PASS" ] || [ "$pw_status" != "PASS" ]; then
  exit 1
fi
