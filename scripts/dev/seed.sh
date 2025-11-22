#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://localhost:8080}"
BOOT_TOKEN="${BOOT_TOKEN:-changeme-bootstrap-token}"

require() {
  local name="$1" value="$2"
  if [[ -z "$value" ]]; then
    echo "$name is required"
    exit 1
  fi
}

require "BOOT_TOKEN" "$BOOT_TOKEN"

echo "registering edge..."
edge_resp="$(curl -fsS -X POST "${BASE_URL}/api/v1/edge-nodes/register" \
  -H "Authorization: Bearer ${BOOT_TOKEN}" \
  -H "Content-Type: application/json" \
  -d '{"name":"edge-seed"}')"
echo "edge response: $edge_resp"
node_token="$(echo "$edge_resp" | python -c "import sys, json; print(json.load(sys.stdin)['token'])")"

echo "creating origin..."
origin_resp="$(curl -fsS -X POST "${BASE_URL}/api/v1/origins" \
  -H "Content-Type: application/json" \
  -d '{"name":"origin-seed","wg_ip":"10.0.0.2"}')"
echo "origin response: $origin_resp"
origin_id="$(echo "$origin_resp" | python -c "import sys, json; print(json.load(sys.stdin)['ID'])")"

echo "creating route..."
route_resp="$(curl -fsS -X POST "${BASE_URL}/api/v1/routes" \
  -H "Content-Type: application/json" \
  -d "{\"hostname\":\"seed.example.com\",\"origin_id\":\"${origin_id}\",\"target_port\":8080}")"
echo "route response: $route_resp"

echo "fetching config..."
config="$(curl -fsS "${BASE_URL}/api/v1/edge-nodes/me/config" \
  -H "Authorization: Bearer ${node_token}")"
echo "config: $config"
