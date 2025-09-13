#!/usr/bin/env bash
# 🚀 Ultimate Safe CI/local pipeline for v2config-manager
# - 100% automatic, self-healing, robust
# - Ensures Docker + Compose v2 available (auto-install if missing)
# - Runs stack, waits for health, validates outputs
# - Collects logs/artifacts always, cleans up cleanly
# - Designed for GitHub Actions + local Linux/macOS with Docker
set -Eeuo pipefail

# ---------- config (override via env) ----------
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.yml}"
CONFIG_FILE="${CONFIG_FILE:-config.yaml}"
CONFIG_EXAMPLE="${CONFIG_EXAMPLE:-config.example.yaml}"
HEALTH_URL="${HEALTH_URL:-http://localhost:8080/healthz}"
HEALTH_RETRIES="${HEALTH_RETRIES:-60}"          
HEALTH_SLEEP_SECONDS="${HEALTH_SLEEP_SECONDS:-2}"

OUTPUT_PLAIN="${OUTPUT_PLAIN:-output/merged_nodes.txt}"
OUTPUT_B64="${OUTPUT_B64:-output/merged_sub_base64.txt}"
OUTPUT_RETRIES="${OUTPUT_RETRIES:-90}"          
OUTPUT_SLEEP_SECONDS="${OUTPUT_SLEEP_SECONDS:-2}"

ARTIFACT_DIR="${ARTIFACT_DIR:-artifact}"
ARTIFACT_NAME="${ARTIFACT_NAME:-clean-sub}"
LOG_FILE="${LOG_FILE:-compose.logs.txt}"
PS_FILE="${PS_FILE:-compose.ps.txt}"

COMPOSE_CMD="${COMPOSE_CMD:-}"                  
COMPOSE_PROJECT_NAME="${COMPOSE_PROJECT_NAME:-v2mgr-ci}"

# ---------- utils ----------
ts() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }
log() { echo "[$(ts)] $*"; }
die() { echo "[$(ts)] ❌ ERROR: $*" >&2; exit 1; }

# ---------- preflight ----------
command -v docker >/dev/null 2>&1 || die "Docker not installed."
docker info >/dev/null 2>&1 || die "Docker daemon unreachable."

# ---------- ensure compose ----------
ensure_compose() {
  if [ -n "$COMPOSE_CMD" ]; then return; fi
  if docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD="docker compose"
  elif command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_CMD="docker-compose"
  else
    log "Docker Compose missing → installing…"
    mkdir -p ~/.docker/cli-plugins
    case "$(uname -m)" in
      x86_64|amd64) ARCH_TAG="x86_64" ;;
      aarch64|arm64) ARCH_TAG="aarch64" ;;
      *) ARCH_TAG="x86_64" ;;
    esac
    curl -fsSL -o ~/.docker/cli-plugins/docker-compose \
      "https://github.com/docker/compose/releases/latest/download/docker-compose-$(uname -s)-${ARCH_TAG}" \
      || die "Failed to download Compose"
    chmod +x ~/.docker/cli-plugins/docker-compose
    COMPOSE_CMD="docker compose"
  fi
  $COMPOSE_CMD version >/dev/null 2>&1 || die "Docker Compose invalid."
}
ensure_compose
export COMPOSE_PROJECT_NAME

[ -f "$COMPOSE_FILE" ] || die "Missing compose file: $COMPOSE_FILE"

# ---------- prepare config ----------
if [ ! -f "$CONFIG_FILE" ]; then
  [ -f "$CONFIG_EXAMPLE" ] || die "Missing $CONFIG_FILE and $CONFIG_EXAMPLE."
  cp -f "$CONFIG_EXAMPLE" "$CONFIG_FILE"
  log "Config initialized from $CONFIG_EXAMPLE"
fi

# control char check
if LC_ALL=C grep -qaP '[\x00-\x08\x0B\x0C\x0E-\x1F]' "$CONFIG_FILE"; then
  die "Invalid control chars in $CONFIG_FILE"
fi

# ensure dirs
mkdir -p "$(dirname "$OUTPUT_PLAIN")" "$(dirname "$OUTPUT_B64")" "$ARTIFACT_DIR"

# ---------- cleanup ----------
cleanup() {
  set +e
  log "Collecting logs…"
  { $COMPOSE_CMD logs --no-color || true; } > "$ARTIFACT_DIR/$LOG_FILE" 2>&1
  { $COMPOSE_CMD ps || true; } > "$ARTIFACT_DIR/$PS_FILE" 2>&1
  log "Stopping stack…"
  $COMPOSE_CMD down -v --remove-orphans || true
  if command -v zip >/dev/null 2>&1; then
    (cd "$ARTIFACT_DIR" && zip -qr "${ARTIFACT_NAME}.zip" .) || true
    log "📦 Artifacts: $ARTIFACT_DIR/${ARTIFACT_NAME}.zip"
  else
    tar -C "$ARTIFACT_DIR" -czf "$ARTIFACT_DIR/${ARTIFACT_NAME}.tar.gz" . || true
    log "📦 Artifacts: $ARTIFACT_DIR/${ARTIFACT_NAME}.tar.gz"
  fi
}
trap cleanup EXIT

# ---------- run stack ----------
log "🚀 Starting stack: $COMPOSE_CMD up -d --build"
$COMPOSE_CMD up -d --build

# ---------- health check ----------
log "⏳ Waiting for health: $HEALTH_URL"
for ((i=1; i<=HEALTH_RETRIES; i++)); do
  if curl -fsS --max-time 5 "$HEALTH_URL" >/dev/null 2>&1; then
    log "✅ Service healthy"
    break
  fi
  if [ "$i" -eq "$HEALTH_RETRIES" ]; then
    $COMPOSE_CMD ps || true
    $COMPOSE_CMD logs --tail 200 || true
    die "Health check failed at $HEALTH_URL"
  fi
  sleep "$HEALTH_SLEEP_SECONDS"
done

# ---------- wait for outputs ----------
log "⏳ Waiting for outputs: $OUTPUT_PLAIN & $OUTPUT_B64"
for ((i=1; i<=OUTPUT_RETRIES; i++)); do
  if [ -s "$OUTPUT_PLAIN" ] && [ -s "$OUTPUT_B64" ]; then
    log "✅ Outputs ready"
    break
  fi
  if [ "$i" -eq "$OUTPUT_RETRIES" ]; then
    ls -lah output || true
    die "Outputs not generated"
  fi
  sleep "$OUTPUT_SLEEP_SECONDS"
done

# ---------- sanity checks ----------
NODE_COUNT=$(grep -Ec '^(vmess://|vless://|trojan://|ss://|socks5://)' "$OUTPUT_PLAIN" || true)
log "ℹ️ Node count: $NODE_COUNT"

if head -c 200000 "$OUTPUT_B64" | base64 -d >/dev/null 2>&1; then
  log "✅ Base64 valid"
else
  die "Invalid Base64 output"
fi

# copy outputs
cp -f "$OUTPUT_PLAIN" "$ARTIFACT_DIR/merged_nodes.txt"
cp -f "$OUTPUT_B64" "$ARTIFACT_DIR/merged_sub_base64.txt"

log "🎉 Pipeline completed successfully"
