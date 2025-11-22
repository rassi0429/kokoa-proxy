package web

import "net/http"

// Handler serves a minimal single-page UI for managing origins and routes.
func Handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(indexHTML))
	})
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
    @media (max-width: 600px) { header { flex-direction: column; align-items: flex-start; gap: 8px; } }
  </style>
</head>
<body>
  <header>
    <div>
      <h1>kokoa control plane</h1>
      <div class="muted">Origins/Routes を管理し、Edge がPullする設定を配布します。</div>
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
        div.innerHTML = '<strong>' + o.Name + '</strong> <span class="pill">' + o.ID + '</span><br><span class="muted">WG IP: ' + o.WireguardIP + '</span>';
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
        await Promise.all([loadOrigins(), loadRoutes()]);
        statusEl.textContent = 'ready';
      } catch (e) {
        statusEl.textContent = 'error: ' + e.message;
      }
    }
    init();
  </script>
</body>
</html>`
