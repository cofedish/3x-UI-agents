#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_FILE="$ROOT/docker/lab/docker-compose.yml"
ARTIFACTS_DIR="$ROOT/artifacts/last-run"
LOG_DIR="$ARTIFACTS_DIR/docker-logs"
DOCKER_BIN=""

resolve_docker() {
  if command -v docker >/dev/null 2>&1; then
    if docker compose version >/dev/null 2>&1; then
      DOCKER_BIN="docker"
    fi
  fi

  if [ -z "$DOCKER_BIN" ]; then
    if command -v docker.exe >/dev/null 2>&1; then
      DOCKER_BIN="docker.exe"
    elif [ -x "/mnt/host/c/Program Files/Docker/Docker/resources/bin/docker.exe" ]; then
      DOCKER_BIN="/mnt/host/c/Program Files/Docker/Docker/resources/bin/docker.exe"
    elif [ -x "/mnt/c/Program Files/Docker/Docker/resources/bin/docker.exe" ]; then
      DOCKER_BIN="/mnt/c/Program Files/Docker/Docker/resources/bin/docker.exe"
    else
      echo "docker not found"
      exit 1
    fi
  fi

  if [ "$DOCKER_BIN" != "docker" ] && command -v wslpath >/dev/null 2>&1; then
    COMPOSE_FILE="$(wslpath -w "$COMPOSE_FILE")"
  fi
}

resolve_docker

compose() {
  if "$DOCKER_BIN" compose version >/dev/null 2>&1; then
    "$DOCKER_BIN" compose -f "$COMPOSE_FILE" "$@"
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose -f "$COMPOSE_FILE" "$@"
  elif command -v docker-compose.exe >/dev/null 2>&1; then
    docker-compose.exe -f "$COMPOSE_FILE" "$@"
  else
    echo "docker compose not found"
    exit 1
  fi
}

log() {
  echo "[lab] $*"
}

rm -rf "$ARTIFACTS_DIR"
mkdir -p "$LOG_DIR"

for dir in "$ROOT/docker/lab/panel-data" "$ROOT/docker/lab/agent-mtls-data" "$ROOT/docker/lab/agent-jwt-data"; do
  rm -rf "$dir"
  mkdir -p "$dir"
done

log "Generating secrets"
"$ROOT/docker/lab/scripts/gen-secrets.sh"

log "Building images"
compose build

log "Starting lab and running e2e"
set +e
compose up --abort-on-container-exit --exit-code-from e2e
status=$?
set -e

for svc in panel agent-mtls agent-jwt e2e; do
  compose logs --no-color "$svc" > "$LOG_DIR/$svc.log" 2>&1 || true
done

{
  echo "Host OS: $(uname -a 2>/dev/null || ver)"
  echo "Docker: $("$DOCKER_BIN" --version 2>/dev/null || echo 'not found')"
  if "$DOCKER_BIN" compose version >/dev/null 2>&1; then
    "$DOCKER_BIN" compose version
  elif command -v docker-compose >/dev/null 2>&1; then
    docker-compose version
  elif command -v docker-compose.exe >/dev/null 2>&1; then
    docker-compose.exe version
  else
    echo "Compose: not found"
  fi
} >> "$ARTIFACTS_DIR/env-summary.txt"

log "Stopping lab"
compose down -v

if [ "$status" -eq 0 ]; then
  echo "PASS - report at $ARTIFACTS_DIR/report.md"
else
  echo "FAIL - report at $ARTIFACTS_DIR/report.md"
fi

exit "$status"
