#!/usr/bin/env bash
set -euo pipefail

CONTROL_PLANE_URL="${CONTROL_PLANE_URL:-}"
NODE_TOKEN="${NODE_TOKEN:-}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nginx/kokoa}"
EMAIL="${EMAIL:-}"
NGINX_BIN="${NGINX_BIN:-nginx}"
BACKOFF="${BACKOFF:-5}"
MAX_BACKOFF="${MAX_BACKOFF:-120}"

log() {
  echo "[kokoa-edge] $*"
}

fail_if_empty() {
  local name="$1" value="$2"
  if [[ -z "$value" ]]; then
    log "env $name is required"
    exit 1
  fi
}

fail_if_empty CONTROL_PLANE_URL "$CONTROL_PLANE_URL"
fail_if_empty NODE_TOKEN "$NODE_TOKEN"

mkdir -p "$CONFIG_DIR"

previous_hash=""
if [[ -f "$CONFIG_DIR/config_hash" ]]; then
  previous_hash="$(cat "$CONFIG_DIR/config_hash")"
fi

while true; do
  tmp_map="$(mktemp)"
  response="$(curl -fsS -H "Authorization: Bearer ${NODE_TOKEN}" "${CONTROL_PLANE_URL}/api/v1/edge-nodes/me/config" || true)"
  if [[ -z "$response" ]]; then
    log "failed to fetch config, backing off ${BACKOFF}s"
    sleep "$BACKOFF"
    BACKOFF=$(( BACKOFF * 2 ))
    if (( BACKOFF > MAX_BACKOFF )); then BACKOFF="$MAX_BACKOFF"; fi
    continue
  fi
  BACKOFF="${BACKOFF:-5}"

  config_hash="$(echo "$response" | jq -r '.config_hash')"
  if [[ -n "$previous_hash" && "$config_hash" == "$previous_hash" ]]; then
    sleep 30
    continue
  fi

  echo "$response" | jq -r '.nginx_map' > "$tmp_map"
  if ! $NGINX_BIN -t >/dev/null 2>&1; then
    log "nginx config test failed, keeping previous config"
    rm -f "$tmp_map"
    sleep 30
    continue
  fi

  mv "$tmp_map" "${CONFIG_DIR}/map.conf"
  echo "$config_hash" > "${CONFIG_DIR}/config_hash"
  log "applied new config hash=${config_hash}"
  $NGINX_BIN -s reload >/dev/null 2>&1 || true

  # TLS retrieval is left as a TODO placeholder for certbot integration.
  sleep 30
done
