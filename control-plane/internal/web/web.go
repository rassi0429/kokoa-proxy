package web

import (
	"fmt"
	"net/http"
)

// Handler serves the UI and helper install scripts.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/edge/install.sh":
			serveInstallScript(w, r)
			return
		case "/edge/poll_config.sh":
			servePollScript(w, r)
			return
		case "/origin/install.sh":
			serveOriginInstall(w, r)
			return
		default:
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(indexHTML))
		}
	})
}

func serveInstallScript(w http.ResponseWriter, r *http.Request) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	baseURL := fmt.Sprintf("%s://%s", scheme, r.Host)
	script := fmt.Sprintf(edgeInstallScript, baseURL, baseURL)
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(script))
}

func servePollScript(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(edgePollScript))
}

func serveOriginInstall(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	_, _ = w.Write([]byte(originInstallScript))
}

const indexHTML = `<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1.0" />
  <title>kokoa control plane</title>
  <style>
    :root { --bg:#0f172a; --card:#111827; --text:#e5e7eb; --muted:#9ca3af; --accent:#06b6d4; --accent2:#22c55e; --danger:#ef4444; }
    * { box-sizing: border-box; }
    body { margin:0; font-family: "Segoe UI", sans-serif; background: radial-gradient(circle at 20% 20%, rgba(6,182,212,0.06), transparent 25%), radial-gradient(circle at 80% 0%, rgba(34,197,94,0.08), transparent 20%), var(--bg); color: var(--text); }
    header { padding: 20px; border-bottom: 1px solid #1f2937; display:flex; justify-content: space-between; align-items:center; }
    h1 { margin:0; font-size: 20px; letter-spacing: 0.4px; }
    main { padding: 20px; display: grid; gap: 20px; grid-template-columns: repeat(auto-fit, minmax(320px, 1fr)); }
    .card { background: linear-gradient(145deg, #0b1220, #0d1628); border: 1px solid #1f2937; border-radius: 12px; padding: 16px; box-shadow: 0 18px 35px rgba(0,0,0,0.35); }
    .card h2 { margin: 0 0 10px 0; font-size: 16px; }
    label { display:block; margin: 10px 0 4px; color: var(--muted); font-size: 13px; }
    input, select { width:100%; padding:10px; border-radius:8px; border:1px solid #1f2937; background:#0b1220; color: var(--text); }
    button { margin-top:12px; padding:10px 12px; border-radius:8px; border:none; cursor:pointer; color:#0b1220; font-weight:600; background: linear-gradient(90deg, var(--accent), var(--accent2)); }
    button:hover { filter: brightness(1.05); }
    .muted { color: var(--muted); font-size:12px; }
    .list { max-height: 360px; overflow-y: auto; }
    .item { padding:10px; border:1px solid #1f2937; border-radius:8px; margin-bottom:8px; background:#0c1322; }
    .item strong { color: var(--accent); }
    .pill { display:inline-block; padding:2px 8px; border-radius:999px; font-size:11px; background:#1f2937; color:var(--muted); margin-left:6px; }
    .error { color: var(--danger); font-size: 12px; margin-top: 6px; }
    .status { font-size: 12px; color: var(--muted); }
    code { background:#0b1220; padding:2px 4px; border-radius:6px; border:1px solid #1f2937; color: var(--muted); }
    @media (max-width: 600px) { header { flex-direction: column; align-items: flex-start; gap: 8px; } }
  </style>
</head>
<body>
  <header>
    <div>
      <h1>kokoa control plane</h1>
      <div class="muted">Origins/Routes/Edges を管理し、Edge がPullする設定を配布します。</div>
    </div>
    <div class="muted status" id="status">loading...</div>
  </header>

  <main>
    <section class="card">
      <h2>Origin 作成</h2>
      <label>名前</label>
      <input id="origin-name" placeholder="origin-1" />
      <label>WireGuard IP</label>
      <input id="origin-wg" placeholder="10.0.0.2" />
      <label>WireGuard 公開鍵 (任意)</label>
      <input id="origin-pub" placeholder="" />
      <label>WireGuard 秘密鍵(暗号化済み) (任意)</label>
      <input id="origin-priv" placeholder="" />
      <button onclick="createOrigin()">作成</button>
      <div class="error" id="origin-error"></div>
    </section>

    <section class="card">
      <h2>Route 作成</h2>
      <label>ホスト名</label>
      <input id="route-host" placeholder="app.example.com" />
      <label>対象 Origin</label>
      <select id="route-origin"></select>
      <label>ターゲットポート</label>
      <input id="route-port" type="number" placeholder="8080" />
      <button onclick="createRoute()">作成</button>
      <div class="error" id="route-error"></div>
    </section>

    <section class="card">
      <h2>Origins</h2>
      <div class="list" id="origins-list"></div>
    </section>

    <section class="card">
      <h2>Routes</h2>
      <div class="list" id="routes-list"></div>
    </section>

    <section class="card">
      <h2>Edge Nodes</h2>
      <div class="list" id="edges-list"></div>
    </section>

    <section class="card">
      <h2>Edge 登録</h2>
      <label>Edge 名</label>
      <input id="edge-name" placeholder="edge-1" />
      <label>ブートストラップトークン</label>
      <input id="edge-bootstrap" placeholder="必須: CP_BOOTSTRAP_TOKEN" />
      <label>WireGuard IP (例: 10.0.0.3/32) - 任意</label>
      <input id="edge-wg-addr" placeholder="10.0.0.3/32" />
      <label>WireGuard Endpoint (例: origin.example.com:51820) - 任意</label>
      <input id="edge-wg-endpoint" placeholder="origin.example.com:51820" />
      <label>Peer Public Key (Originのpubkey) - 任意</label>
      <input id="edge-wg-peer" placeholder="base64 pubkey" />
      <label>Allowed IPs (任意, デフォルト0.0.0.0/0)</label>
      <input id="edge-wg-allowed" placeholder="0.0.0.0/0" />
      <button onclick="registerEdge()">登録してワンライナー生成</button>
      <div class="error" id="edge-error"></div>
      <div class="muted" id="edge-result"></div>
    </section>
  </main>

  <script>
    const statusEl = document.getElementById('status');
    async function fetchJSON(url, opts={}) {
      const res = await fetch(url, opts);
      if (!res.ok) {
        const data = await res.json().catch(() => ({}));
        const msg = data.error || res.statusText;
        throw new Error(msg);
      }
      return res.json();
    }

    async function loadOrigins() {
      const data = await fetchJSON('/api/v1/origins/list');
      const listEl = document.getElementById('origins-list');
      const select = document.getElementById('route-origin');
      listEl.innerHTML = '';
      select.innerHTML = '';
      data.forEach(o => {
        const div = document.createElement('div');
        div.className = 'item';
        const cmd = 'curl -fsSL ' + window.location.origin + '/origin/install.sh | WG_ADDR=' + o.WireguardIP + '/32 bash';
        div.innerHTML = '<strong>' + o.Name + '</strong> <span class="pill">' + o.ID + '</span><br><span class="muted">WG IP: ' + o.WireguardIP + '</span><br><div class="muted">Install: <code style="font-size:11px;">' + cmd + '</code></div>';
        listEl.appendChild(div);
        const opt = document.createElement('option');
        opt.value = o.ID;
        opt.textContent = o.Name + ' (' + o.WireguardIP + ')';
        select.appendChild(opt);
      });
      if (!data.length) {
        listEl.innerHTML = '<div class="muted">まだ登録されていません</div>';
      }
    }

    async function loadRoutes() {
      const data = await fetchJSON('/api/v1/routes/list');
      const listEl = document.getElementById('routes-list');
      listEl.innerHTML = '';
      data.forEach(r => {
        const div = document.createElement('div');
        div.className = 'item';
        div.innerHTML = '<strong>' + r.Hostname + '</strong> ➜ ' + r.WireguardIP + ':' + r.TargetPort + '<br><span class="muted">origin: ' + r.OriginName + '</span>';
        listEl.appendChild(div);
      });
      if (!data.length) {
        listEl.innerHTML = '<div class="muted">まだ登録されていません</div>';
      }
    }

    async function loadEdges() {
      const data = await fetchJSON('/api/v1/edge-nodes/list');
      const listEl = document.getElementById('edges-list');
      listEl.innerHTML = '';
      data.forEach(e => {
        const div = document.createElement('div');
        div.className = 'item';
        const lastSeen = e.LastSeen && e.LastSeen.Time ? new Date(e.LastSeen.Time).toISOString() : 'never';
        const cmd = 'curl -fsSL ' + window.location.origin + '/edge/install.sh | BOOTSTRAP_TOKEN=<token> bash';
        div.innerHTML = '<strong>' + (e.Name || 'edge') + '</strong> <span class="pill">' + e.ID + '</span><br><span class="muted">last_seen: ' + lastSeen + '</span><br><div class="muted">Install: <code style="font-size:11px;">' + cmd + '</code></div>';
        listEl.appendChild(div);
      });
      if (!data.length) {
        listEl.innerHTML = '<div class="muted">まだ登録されていません</div>';
      }
    }

    async function createOrigin() {
      const errEl = document.getElementById('origin-error');
      errEl.textContent = '';
      try {
        const payload = {
          name: document.getElementById('origin-name').value.trim(),
          wg_ip: document.getElementById('origin-wg').value.trim(),
          wireguard_public_key: document.getElementById('origin-pub').value.trim(),
          wireguard_private_key_encrypted: document.getElementById('origin-priv').value.trim(),
        };
        await fetchJSON('/api/v1/origins', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        document.getElementById('origin-name').value = '';
        document.getElementById('origin-wg').value = '';
        document.getElementById('origin-pub').value = '';
        document.getElementById('origin-priv').value = '';
        await loadOrigins();
      } catch (e) {
        errEl.textContent = e.message;
      }
    }

    async function createRoute() {
      const errEl = document.getElementById('route-error');
      errEl.textContent = '';
      try {
        const payload = {
          hostname: document.getElementById('route-host').value.trim(),
          origin_id: document.getElementById('route-origin').value,
          target_port: Number(document.getElementById('route-port').value),
        };
        await fetchJSON('/api/v1/routes', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(payload),
        });
        document.getElementById('route-host').value = '';
        document.getElementById('route-port').value = '';
        await loadRoutes();
      } catch (e) {
        errEl.textContent = e.message;
      }
    }

    async function init() {
      statusEl.textContent = 'loading...';
      try {
        await Promise.all([loadOrigins(), loadRoutes(), loadEdges()]);
        statusEl.textContent = 'ready';
      } catch (e) {
        statusEl.textContent = 'error: ' + e.message;
      }
    }
    init();

    async function registerEdge() {
      const errEl = document.getElementById('edge-error');
      const resEl = document.getElementById('edge-result');
      errEl.textContent = '';
      resEl.textContent = '';
      try {
        const name = document.getElementById('edge-name').value.trim() || 'edge';
        const bootstrap = document.getElementById('edge-bootstrap').value.trim();
        if (!bootstrap) throw new Error('bootstrap token is required');
        const payload = { name };
        const resp = await fetchJSON('/api/v1/edge-nodes/register', {
          method: 'POST',
          headers: {
            'Content-Type': 'application/json',
            'Authorization': 'Bearer ' + bootstrap,
          },
          body: JSON.stringify(payload),
        });
        const nodeToken = resp.token || '';
        const wgAddr = document.getElementById('edge-wg-addr').value.trim();
        const wgEndpoint = document.getElementById('edge-wg-endpoint').value.trim();
        const wgPeer = document.getElementById('edge-wg-peer').value.trim();
        const wgAllowed = document.getElementById('edge-wg-allowed').value.trim();
        let cmd = 'curl -fsSL ' + window.location.origin + '/edge/install.sh | ';
        cmd += 'BOOTSTRAP_TOKEN=' + bootstrap + ' ';
        if (wgAddr) cmd += 'WG_ADDR=' + wgAddr + ' ';
        if (wgEndpoint) cmd += 'WG_ENDPOINT=' + wgEndpoint + ' ';
        if (wgPeer) cmd += 'WG_PEER_PUBKEY=' + wgPeer + ' ';
        if (wgAllowed) cmd += 'WG_ALLOWED_IPS=' + wgAllowed + ' ';
        cmd += 'bash';
        resEl.innerHTML = '登録完了。Edge ID: ' + (resp.edge_node_id || '') + '<br>NODE_TOKEN: <code>' + nodeToken + '</code><br>インストール: <code>' + cmd + '</code>';
        await loadEdges();
      } catch (e) {
        errEl.textContent = e.message;
      }
    }
  </script>
</body>
</html>`

const edgeInstallScript = `#!/usr/bin/env bash
set -euo pipefail

CONTROL_PLANE_URL="${CONTROL_PLANE_URL:-%s}"
BOOTSTRAP_TOKEN="${BOOTSTRAP_TOKEN:-}"
EDGE_NAME="${EDGE_NAME:-$(hostname -s)}"
CONFIG_DIR="${CONFIG_DIR:-/etc/nginx/kokoa}"
EMAIL="${EMAIL:-}"
NGINX_BIN="${NGINX_BIN:-nginx}"
WG_IFACE="${WG_IFACE:-wg0}"
WG_ADDR="${WG_ADDR:-}"
WG_ENDPOINT="${WG_ENDPOINT:-}"
WG_PEER_PUBKEY="${WG_PEER_PUBKEY:-}"
WG_ALLOWED_IPS="${WG_ALLOWED_IPS:-0.0.0.0/0}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bootstrap-token) BOOTSTRAP_TOKEN="$2"; shift 2;;
    --control-plane-url) CONTROL_PLANE_URL="$2"; shift 2;;
    --edge-name) EDGE_NAME="$2"; shift 2;;
    --config-dir) CONFIG_DIR="$2"; shift 2;;
    --nginx-bin) NGINX_BIN="$2"; shift 2;;
    --email) EMAIL="$2"; shift 2;;
    --wg-addr) WG_ADDR="$2"; shift 2;;
    --wg-endpoint) WG_ENDPOINT="$2"; shift 2;;
    --wg-peer-pubkey) WG_PEER_PUBKEY="$2"; shift 2;;
    --wg-iface) WG_IFACE="$2"; shift 2;;
    --wg-allowed-ips) WG_ALLOWED_IPS="$2"; shift 2;;
    *)
      echo "unknown arg: $1" >&2
      exit 1
      ;;
  esac
done

require() {
  local name="$1" value="$2"
  if [[ -z "$value" ]]; then
    echo "$name is required" >&2
    exit 1
  fi
}

if [[ $EUID -ne 0 ]]; then
  echo "run as root (needed for nginx/systemd install)" >&2
  exit 1
fi

command -v curl >/dev/null 2>&1 || { echo "curl is required"; exit 1; }

require "BOOTSTRAP_TOKEN" "$BOOTSTRAP_TOKEN"

echo "[1/5] Installing packages (wireguard, nginx, jq, curl)..."
apt-get update -y
apt-get install -y wireguard nginx jq curl
systemctl enable --now nginx >/dev/null 2>&1 || true

echo "[2/5] Registering edge at ${CONTROL_PLANE_URL}..."
resp="$(curl -fsS -X POST "${CONTROL_PLANE_URL}/api/v1/edge-nodes/register" \
  -H "Authorization: Bearer ${BOOTSTRAP_TOKEN}" \
  -H "Content-Type: application/json" \
  -d "{\"name\":\"${EDGE_NAME}\"}")"

token="$(echo "$resp" | python -c "import sys,json;print(json.load(sys.stdin)['token'])" 2>/dev/null || true)"
if [[ -z "$token" ]]; then
  token="$(echo "$resp" | jq -r '.token' 2>/dev/null || true)"
fi
if [[ -z "$token" ]]; then
  echo "failed to parse token from response: $resp" >&2
  exit 1
fi
echo "[3/5] Received node token."

echo "[4/5] Installing poller script..."
mkdir -p /usr/local/bin
curl -fsS "%s/edge/poll_config.sh" -o /usr/local/bin/kokoa-edge-poll.sh
chmod +x /usr/local/bin/kokoa-edge-poll.sh

echo "[5/5] Writing env file to /etc/kokoa-edge.env"
cat >/etc/kokoa-edge.env <<EOF
CONTROL_PLANE_URL=${CONTROL_PLANE_URL}
NODE_TOKEN=${token}
CONFIG_DIR=${CONFIG_DIR}
EMAIL=${EMAIL}
NGINX_BIN=${NGINX_BIN}
EOF

if [[ -n "$WG_ADDR" ]]; then
  echo "WG_ADDR=${WG_ADDR}" >> /etc/kokoa-edge.env
fi

# Optional WireGuard setup if WG_ADDR, WG_ENDPOINT, WG_PEER_PUBKEY provided
if [[ -n "$WG_ADDR" && -n "$WG_ENDPOINT" && -n "$WG_PEER_PUBKEY" ]]; then
  echo "[wg] configuring ${WG_IFACE}..."
  umask 077
  wg genkey | tee /etc/wireguard/${WG_IFACE}.key | wg pubkey > /etc/wireguard/${WG_IFACE}.pub
  privkey="$(cat /etc/wireguard/${WG_IFACE}.key)"

  cat >/etc/wireguard/${WG_IFACE}.conf <<EOF
[Interface]
Address = ${WG_ADDR}
PrivateKey = ${privkey}
DNS = 1.1.1.1

[Peer]
PublicKey = ${WG_PEER_PUBKEY}
AllowedIPs = ${WG_ALLOWED_IPS}
Endpoint = ${WG_ENDPOINT}
PersistentKeepalive = 25
EOF
  systemctl enable --now wg-quick@${WG_IFACE}
  echo "WireGuard up on ${WG_IFACE} (${WG_ADDR})"
else
  echo "WireGuard packages installed. Skipping config (set WG_ADDR, WG_ENDPOINT, WG_PEER_PUBKEY to auto-configure)."
fi

if command -v systemctl >/dev/null 2>&1; then
  cat >/etc/systemd/system/kokoa-edge.service <<'EOF'
[Unit]
Description=Kokoa Edge Agent
After=network-online.target

[Service]
EnvironmentFile=/etc/kokoa-edge.env
ExecStart=/usr/local/bin/kokoa-edge-poll.sh
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable --now kokoa-edge.service
  echo "Edge service started via systemd."
else
  echo "systemd not found. Run manually:"
  echo "  env \$(cat /etc/kokoa-edge.env | xargs) /usr/local/bin/kokoa-edge-poll.sh"
fi

echo "Done. NODE_TOKEN stored in /etc/kokoa-edge.env"
`

const edgePollScript = `#!/usr/bin/env bash
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
  if [[ "$NGINX_BIN" != "true" ]]; then
    if ! $NGINX_BIN -t >/dev/null 2>&1; then
      log "nginx config test failed, keeping previous config"
      rm -f "$tmp_map"
      sleep 30
      continue
    fi
  fi

  mv "$tmp_map" "${CONFIG_DIR}/map.conf"
  echo "$config_hash" > "${CONFIG_DIR}/config_hash"
  log "applied new config hash=${config_hash}"
  if [[ "$NGINX_BIN" != "true" ]]; then
    $NGINX_BIN -s reload >/dev/null 2>&1 || true
  fi

  # TLS retrieval is left as a TODO placeholder for certbot integration.
  sleep 30
done
`

const originInstallScript = `#!/usr/bin/env bash
set -euo pipefail

WG_IFACE="${WG_IFACE:-wg0}"
WG_PORT="${WG_PORT:-51820}"
WG_ADDR="${WG_ADDR:-10.20.0.1/24}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --iface) WG_IFACE="$2"; shift 2;;
    --port) WG_PORT="$2"; shift 2;;
    --addr) WG_ADDR="$2"; shift 2;;
    *)
      echo "unknown arg: $1" >&2; exit 1;;
  esac
done

if [[ $EUID -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi

apt-get update -y
apt-get install -y wireguard qrencode

umask 077
wg genkey | tee /etc/wireguard/privatekey | wg pubkey > /etc/wireguard/publickey

cat >/etc/wireguard/${WG_IFACE}.conf <<EOF
[Interface]
Address = ${WG_ADDR}
ListenPort = ${WG_PORT}
PrivateKey = $(cat /etc/wireguard/privatekey)
SaveConfig = true

# Peers will be appended by the control plane.
EOF

systemctl enable --now wg-quick@${WG_IFACE}
echo "WireGuard origin ready on ${WG_IFACE} (${WG_ADDR}). Add peers via wg set or the control plane."
`
