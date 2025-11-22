# Kokoa Proxy - ユースケース詳細

> Cloudflare Tunnelのような体験をセルフホストで実現する

---

## コアコンセプト

**「ワンライナーでホストを登録 → Web UIでポチポチ設定 → すぐに公開」**

複数のVM・ラズパイ・自宅サーバーで動いているWebサービスを、簡単にインターネットに公開できる仕組み。

---

## 基本的なユースケース

### UC-001: 初めてのサービス公開

**シナリオ:**
自宅のラズパイでGhostブログ（ポート2368）を動かしていて、`blog.example.com` で公開したい。

**ステップ:**

```bash
# 1. ラズパイでワンライナー実行
pi@raspberrypi:~ $ curl -sSL https://control.example.com/install.sh | bash -s -- --name rpi-home

# 出力例:
# ✓ Downloading kokoa-agent...
# ✓ Generating WireGuard keys...
# ✓ Registering with Control Plane...
# ✓ Connecting tunnel...
# ✅ Origin "rpi-home" is now registered!
#
# Next steps:
#   1. Visit https://control.example.com
#   2. Add a route to expose your services
```

```
2. ブラウザでControl Plane Web UIにアクセス
   https://control.example.com

3. ダッシュボードで「Add Route」ボタンクリック

4. フォームに入力
   ┌────────────────────────────────────────────┐
   │ Create New Route                           │
   ├────────────────────────────────────────────┤
   │ Hostname *                                 │
   │ ┌────────────────────────────────────────┐ │
   │ │ blog.example.com                       │ │
   │ └────────────────────────────────────────┘ │
   │                                            │
   │ Origin *                                   │
   │ ┌────────────────────────────────────────┐ │
   │ │ rpi-home                            ▼  │ │
   │ └────────────────────────────────────────┘ │
   │                                            │
   │ Service URL *                              │
   │ ┌────────────────────────────────────────┐ │
   │ │ http://localhost:2368                  │ │
   │ └────────────────────────────────────────┘ │
   │                                            │
   │ SSL Certificate                            │
   │ ☑ Auto-obtain (Let's Encrypt)             │
   │ ☐ Use existing certificate                │
   │                                            │
   │ [Cancel]              [Create Route] ──→   │
   └────────────────────────────────────────────┘

5. 「Create Route」をクリック

6. システムが自動処理（進捗表示）
   ┌────────────────────────────────────────────┐
   │ Creating route for blog.example.com...    │
   ├────────────────────────────────────────────┤
   │ ✓ Validating hostname                     │
   │ ✓ Updating Edge Node configuration        │
   │ ✓ Obtaining SSL certificate               │
   │ ✓ Adding DNS records                      │
   │ ✓ Testing connectivity                    │
   │                                            │
   │ ✅ Route created successfully!             │
   │                                            │
   │ Your service is now live at:              │
   │ https://blog.example.com                  │
   │                                            │
   │ [View Routes]                    [Close]   │
   └────────────────────────────────────────────┘

7. 完了！ブラウザで https://blog.example.com にアクセス可能
```

---

### UC-002: 複数サービスの公開

**シナリオ:**
複数のVM・サーバーで異なるサービスを動かしていて、全て公開したい。

```bash
# VM1（Ubuntu）でAPI、Web、Vaultを稼働
vm1@cloud:~ $ curl -sSL https://control.example.com/install.sh | bash -s -- --name vm1

# VM2（別のクラウド）でデータベース管理画面を稼働
vm2@another:~ $ curl -sSL https://control.example.com/install.sh | bash -s -- --name vm2-db

# 自宅ラズパイで写真管理・ファイル共有を稼働
pi@home:~ $ curl -sSL https://control.example.com/install.sh | bash -s -- --name rpi-home
```

**Web UIでルーティング設定:**

| Hostname | Origin | Service | Status |
|----------|--------|---------|--------|
| `blog.example.com` | rpi-home | `http://localhost:2368` | ✅ Active |
| `api.example.com` | vm1 | `http://localhost:8080` | ✅ Active |
| `vault.example.com` | vm1 | `http://localhost:8200` | ✅ Active |
| `photos.example.com` | rpi-home | `http://localhost:2283` | ✅ Active |
| `files.example.com` | rpi-home | `http://localhost:8080` | ✅ Active |
| `admin.example.com` | vm2-db | `http://localhost:3000` | ✅ Active |

**結果:**
全て自動的にHTTPS化され、Edge Node経由でアクセス可能に。

---

### UC-003: パスベースルーティング

**シナリオ:**
`example.com` の配下で、パスごとに異なるサービスにルーティングしたい。

```
example.com/          → vm1:3000  (フロントエンド)
example.com/api/      → vm1:8080  (API サーバー)
example.com/blog/     → vm2:2368  (ブログ)
example.com/admin/    → vm1:9000  (管理画面)
```

**Web UIでの設定:**

```
Route #1
  Hostname: example.com
  Path:     /
  Origin:   vm1
  Service:  http://localhost:3000

Route #2
  Hostname: example.com
  Path:     /api
  Origin:   vm1
  Service:  http://localhost:8080

Route #3
  Hostname: example.com
  Path:     /blog
  Origin:   vm2
  Service:  http://localhost:2368

Route #4
  Hostname: example.com
  Path:     /admin
  Origin:   vm1
  Service:  http://localhost:9000
```

---

### UC-004: ワイルドカードドメイン

**シナリオ:**
`*.app.example.com` で、各ユーザーごとにサブドメインを動的に割り当てたい。

```
user1.app.example.com → vm1:3000  (ユーザー1の環境)
user2.app.example.com → vm1:3000  (ユーザー2の環境)
user3.app.example.com → vm2:3000  (ユーザー3の環境)
```

**Web UIでの設定:**

```
Route #1
  Hostname: *.app.example.com
  Origin:   vm1
  Service:  http://localhost:3000

  Advanced Options:
    ☑ Forward original Host header
    ☐ Rewrite Host header
```

**実際の動作:**
- `Host: user1.app.example.com` というヘッダーがそのままvm1:3000に転送される
- アプリ側でサブドメインを見て、マルチテナント処理を行う

---

### UC-005: 開発環境の一時公開

**シナリオ:**
ローカルマシンで開発中のアプリを、一時的にチームメンバーに共有したい。

```bash
# ローカルMac（開発マシン）で実行
mac@local:~ $ curl -sSL https://control.example.com/install.sh | bash -s -- --name dev-mac --temporary

# 出力:
# ✅ Temporary origin "dev-mac" registered!
# Expires in: 24 hours
```

**Web UIで設定:**

```
Route #1
  Hostname: demo.example.com
  Origin:   dev-mac
  Service:  http://localhost:3000

  Options:
    ☑ Temporary (Auto-delete after 24 hours)
```

**24時間後:**
- 自動的にルーティングが削除される
- DNSレコードも削除される
- dev-macのトンネルも切断される

---

### UC-006: 障害時の自動フェイルオーバー

**シナリオ:**
メインのOriginがダウンした時、バックアップOriginに自動切り替えたい。

```
Primary:   vm1:3000
Secondary: vm2:3000 (スタンバイ)
```

**Web UIで設定:**

```
Route #1
  Hostname: app.example.com

  Primary Origin:
    Origin:  vm1
    Service: http://localhost:3000

  Failover:
    ☑ Enable automatic failover
    Secondary Origin: vm2
    Service: http://localhost:3000
    Health check interval: 10s
```

**動作:**
1. Control Planeが10秒ごとにvm1:3000をヘルスチェック
2. 3回連続失敗したら、vm2:3000に自動切り替え
3. vm1が復旧したら、自動的に戻す

---

### UC-007: 複数Edge Nodeでの負荷分散

**シナリオ:**
トラフィックが増えてきたので、複数のEdge Nodeで負荷分散したい。

**Web UIでの操作:**

```
1. ダッシュボードで「Edge Nodes」タブを開く

2. 「Add Edge Node」ボタンクリック

3. フォームに入力
   Provider:  DigitalOcean
   Region:    Singapore (sgp1)
   Size:      s-1vcpu-1gb ($6/month)

   [Create]

4. 数分後、新しいEdge Nodeが追加される

5. DNSに自動的に追加される:
   app.example.com  A  203.0.113.10  (Edge Node 1)
   app.example.com  A  203.0.113.11  (Edge Node 2)
   app.example.com  A  198.51.100.5  (Edge Node 3 - 新規)

6. ユーザーのトラフィックが3台に分散される（DNS Round Robin）
```

---

### UC-008: 攻撃を受けたEdge Nodeの即時交換

**シナリオ:**
Edge Node 1がDDoS攻撃を受けて、VPSのCPUが100%に。

**Web UIでの操作:**

```
1. ダッシュボードで警告表示
   ⚠️ Edge Node 1 (203.0.113.10) - High CPU usage (98%)

2. 詳細を確認
   - 異常なトラフィック検知
   - 5000+ req/s from multiple IPs

3. 「Remove & Replace」ボタンをクリック

4. 確認ダイアログ
   ┌────────────────────────────────────────────┐
   │ Remove and replace Edge Node 1?            │
   ├────────────────────────────────────────────┤
   │ This will:                                 │
   │  1. Remove from DNS immediately            │
   │  2. Wait 30s for TTL expiration            │
   │  3. Destroy VPS (203.0.113.10)             │
   │  4. Create new Edge Node in different      │
   │     region with different provider         │
   │                                            │
   │ Estimated downtime: 0s                     │
   │ (Traffic will use remaining nodes)         │
   │                                            │
   │ [Cancel]              [Confirm] ──→        │
   └────────────────────────────────────────────┘

5. 処理開始
   ✓ Removed from DNS
   ✓ Creating new VPS (Vultr, Tokyo)
   ✓ Deploying configuration
   ✓ Tunnel established
   ✓ Added to DNS (198.51.100.20)
   ✓ Destroying old VPS

   ✅ Edge Node replaced successfully!

6. 攻撃を受けていたIPアドレス（203.0.113.10）は完全に破棄
   新しいIP（198.51.100.20）で稼働継続
```

---

## ワンライナー登録の詳細フロー

### インストールスクリプトの動作

```bash
curl -sSL https://control.example.com/install.sh | bash -s -- --name vm1
```

**スクリプトの内部処理:**

```bash
#!/bin/bash
# install.sh

set -e

CONTROL_PLANE_URL="https://control.example.com"
ORIGIN_NAME="${1:-$(hostname)}"

echo "🚀 Installing Kokoa Proxy Agent..."

# 1. システム検出
detect_system() {
  OS=$(uname -s)
  ARCH=$(uname -m)
  echo "✓ Detected: $OS $ARCH"
}

# 2. エージェントバイナリダウンロード
download_agent() {
  echo "📥 Downloading kokoa-agent..."
  curl -sSL "${CONTROL_PLANE_URL}/downloads/kokoa-agent-${OS}-${ARCH}" \
    -o /usr/local/bin/kokoa-agent
  chmod +x /usr/local/bin/kokoa-agent
  echo "✓ Agent downloaded"
}

# 3. WireGuard鍵生成
generate_keys() {
  echo "🔑 Generating WireGuard keys..."
  PRIVATE_KEY=$(wg genkey)
  PUBLIC_KEY=$(echo "$PRIVATE_KEY" | wg pubkey)
  echo "✓ Keys generated"
}

# 4. Control Planeに登録
register_origin() {
  echo "📝 Registering with Control Plane..."

  RESPONSE=$(curl -sSL -X POST "${CONTROL_PLANE_URL}/api/v1/origins/register" \
    -H "Content-Type: application/json" \
    -d "{
      \"name\": \"${ORIGIN_NAME}\",
      \"public_key\": \"${PUBLIC_KEY}\",
      \"platform\": \"${OS}\",
      \"arch\": \"${ARCH}\"
    }")

  # レスポンスからトークンと設定を取得
  TOKEN=$(echo "$RESPONSE" | jq -r '.token')
  WG_IP=$(echo "$RESPONSE" | jq -r '.wireguard_ip')
  WG_SERVER=$(echo "$RESPONSE" | jq -r '.wireguard_server')
  WG_SERVER_PUBLIC_KEY=$(echo "$RESPONSE" | jq -r '.wireguard_server_public_key')

  echo "✓ Registered as: $ORIGIN_NAME ($WG_IP)"
}

# 5. WireGuard設定作成
setup_wireguard() {
  echo "🔧 Setting up WireGuard..."

  cat > /etc/wireguard/kokoa.conf <<EOF
[Interface]
PrivateKey = $PRIVATE_KEY
Address = $WG_IP/24

[Peer]
PublicKey = $WG_SERVER_PUBLIC_KEY
Endpoint = $WG_SERVER
AllowedIPs = 10.0.0.0/24
PersistentKeepalive = 25
EOF

  # WireGuardインターフェース起動
  wg-quick up kokoa
  systemctl enable wg-quick@kokoa

  echo "✓ WireGuard tunnel established"
}

# 6. エージェント設定・起動
setup_agent() {
  echo "🎯 Starting agent..."

  cat > /etc/kokoa/agent.conf <<EOF
control_plane_url = "${CONTROL_PLANE_URL}"
origin_name = "${ORIGIN_NAME}"
token = "${TOKEN}"
wireguard_interface = "kokoa"
heartbeat_interval = 30
EOF

  # systemdサービス作成
  cat > /etc/systemd/system/kokoa-agent.service <<EOF
[Unit]
Description=Kokoa Proxy Agent
After=network.target wg-quick@kokoa.service

[Service]
Type=simple
ExecStart=/usr/local/bin/kokoa-agent --config /etc/kokoa/agent.conf
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable kokoa-agent
  systemctl start kokoa-agent

  echo "✓ Agent started"
}

# 7. 実行
detect_system
download_agent
generate_keys
register_origin
setup_wireguard
setup_agent

echo ""
echo "✅ Origin \"${ORIGIN_NAME}\" is now registered!"
echo ""
echo "Next steps:"
echo "  1. Visit ${CONTROL_PLANE_URL}"
echo "  2. Add a route to expose your services"
echo ""
```

---

## データモデル

### ルーティング設定のデータ構造

```sql
-- ルーティングテーブル
CREATE TABLE routes (
  id UUID PRIMARY KEY,
  hostname VARCHAR(255) NOT NULL,  -- blog.example.com
  path VARCHAR(255) DEFAULT '/',   -- /, /api, /admin
  origin_id UUID REFERENCES origins(id) NOT NULL,
  service_url VARCHAR(500) NOT NULL,  -- http://localhost:3000

  -- SSL設定
  ssl_enabled BOOLEAN DEFAULT true,
  ssl_cert_type VARCHAR(20) DEFAULT 'auto',  -- auto, manual, cloudflare
  ssl_cert_id UUID REFERENCES ssl_certificates(id),

  -- 高度な設定
  strip_path BOOLEAN DEFAULT false,  -- /api/users → /users
  preserve_host BOOLEAN DEFAULT true,
  websocket_enabled BOOLEAN DEFAULT true,

  -- フェイルオーバー
  failover_enabled BOOLEAN DEFAULT false,
  failover_origin_id UUID REFERENCES origins(id),
  failover_service_url VARCHAR(500),

  -- 一時公開
  temporary BOOLEAN DEFAULT false,
  expires_at TIMESTAMP,

  -- メタデータ
  status VARCHAR(20) DEFAULT 'active',  -- active, inactive, error
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW(),
  created_by UUID REFERENCES users(id)
);

-- インデックス
CREATE INDEX idx_routes_hostname ON routes(hostname);
CREATE INDEX idx_routes_origin ON routes(origin_id);
CREATE INDEX idx_routes_status ON routes(status);

-- ユニーク制約（同じホスト名・パスの組み合わせは1つのみ）
CREATE UNIQUE INDEX idx_routes_unique ON routes(hostname, path)
  WHERE status = 'active';
```

### ルーティングのJSON例

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "hostname": "blog.example.com",
  "path": "/",
  "origin": {
    "id": "660e8400-e29b-41d4-a716-446655440001",
    "name": "rpi-home",
    "status": "connected",
    "wireguard_ip": "10.0.0.5"
  },
  "service_url": "http://localhost:2368",
  "ssl": {
    "enabled": true,
    "type": "auto",
    "certificate": {
      "issuer": "Let's Encrypt",
      "expires_at": "2025-04-21T10:30:00Z"
    }
  },
  "advanced": {
    "strip_path": false,
    "preserve_host": true,
    "websocket_enabled": true
  },
  "failover": {
    "enabled": false
  },
  "status": "active",
  "created_at": "2025-01-21T10:30:00Z",
  "updated_at": "2025-01-21T10:30:00Z"
}
```

---

## ルーティングの種類と優先度

### 1. ホストベースルーティング（最も基本）

```
blog.example.com  → Origin: rpi-home, Service: http://localhost:2368
api.example.com   → Origin: vm1, Service: http://localhost:8080
```

**Nginx設定例（Edge Node）:**

```nginx
server {
  listen 443 ssl http2;
  server_name blog.example.com;

  ssl_certificate /etc/letsencrypt/live/blog.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/blog.example.com/privkey.pem;

  location / {
    proxy_pass http://10.0.0.5:2368;  # rpi-home
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;
  }
}

server {
  listen 443 ssl http2;
  server_name api.example.com;

  ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;

  location / {
    proxy_pass http://10.0.0.10:8080;  # vm1
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto https;
  }
}
```

---

### 2. パスベースルーティング

```
example.com/        → Origin: vm1, Service: http://localhost:3000
example.com/api/    → Origin: vm1, Service: http://localhost:8080
example.com/blog/   → Origin: vm2, Service: http://localhost:2368
```

**Nginx設定例（優先度順）:**

```nginx
server {
  listen 443 ssl http2;
  server_name example.com;

  ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

  # 長いパスほど優先度が高い（最長一致）

  location /api/ {
    proxy_pass http://10.0.0.10:8080/;  # vm1 API
    # /api/users → http://10.0.0.10:8080/users (パス除去)
  }

  location /blog/ {
    proxy_pass http://10.0.0.11:2368/;  # vm2 Blog
    # /blog/post/123 → http://10.0.0.11:2368/post/123
  }

  location / {
    proxy_pass http://10.0.0.10:3000;  # vm1 Frontend
    # すべての未マッチパス
  }
}
```

**パス除去（strip_path）のオプション:**

- `strip_path: true` → `/api/users` → `/users`
- `strip_path: false` → `/api/users` → `/api/users`

---

### 3. ワイルドカードルーティング

```
*.app.example.com  → Origin: vm1, Service: http://localhost:3000
```

**Nginx設定例:**

```nginx
server {
  listen 443 ssl http2;
  server_name *.app.example.com;

  # ワイルドカード証明書
  ssl_certificate /etc/letsencrypt/live/app.example.com/fullchain.pem;
  ssl_certificate_key /etc/letsencrypt/live/app.example.com/privkey.pem;

  location / {
    proxy_pass http://10.0.0.10:3000;
    proxy_set_header Host $host;  # 元のサブドメインを保持
    proxy_set_header X-Real-IP $remote_addr;
  }
}
```

**使用例:**
- `user1.app.example.com` → vm1:3000（Host: user1.app.example.com）
- `user2.app.example.com` → vm1:3000（Host: user2.app.example.com）
- アプリ側で `Host` ヘッダーを見て、マルチテナント処理

---

### 4. 優先度ルール

複数のルートが競合する場合の優先順位:

1. **完全一致ホスト名 + 最長パス一致**
2. **完全一致ホスト名 + 短いパス**
3. **ワイルドカードホスト名**
4. **デフォルトルート**

**例:**

```
Route 1: api.example.com /v2/  → vm1:8080  (優先度: 最高)
Route 2: api.example.com /     → vm1:3000  (優先度: 高)
Route 3: *.example.com   /     → vm2:3000  (優先度: 中)
Route 4: *               /     → default   (優先度: 最低)
```

**リクエストのマッチング:**

- `api.example.com/v2/users` → Route 1（vm1:8080）
- `api.example.com/health` → Route 2（vm1:3000）
- `blog.example.com/` → Route 3（vm2:3000）
- `unknown.com/` → Route 4（default）

---

## Web UI設計

### ダッシュボード画面

```
┌──────────────────────────────────────────────────────────────┐
│ Kokoa Proxy                                    admin@example │
├──────────────────────────────────────────────────────────────┤
│  Dashboard   Routes   Origins   Edge Nodes   Settings        │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Overview                                                     │
│  ┌────────────┐  ┌────────────┐  ┌────────────┐             │
│  │   Routes   │  │  Origins   │  │ Edge Nodes │             │
│  │     12     │  │      3     │  │      3     │             │
│  │  ✓ Active  │  │ ✓ Connected│  │  ✓ Healthy │             │
│  └────────────┘  └────────────┘  └────────────┘             │
│                                                               │
│  Quick Actions                                                │
│  [+ Add Route]  [+ Add Origin]  [+ Add Edge Node]           │
│                                                               │
│  Recent Activity                                              │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ ✓ Route created: photos.example.com → rpi-home        │  │
│  │   2 minutes ago                                        │  │
│  │                                                        │  │
│  │ ⚠ Edge Node 2 high CPU usage (87%)                    │  │
│  │   10 minutes ago                                       │  │
│  │                                                        │  │
│  │ ✓ SSL certificate renewed: api.example.com            │  │
│  │   1 hour ago                                           │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  Active Routes                                                │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Hostname             Origin    Service         Status  │  │
│  ├────────────────────────────────────────────────────────┤  │
│  │ blog.example.com     rpi-home  :2368          ✓ Active│  │
│  │ api.example.com      vm1       :8080          ✓ Active│  │
│  │ vault.example.com    vm1       :8200          ✓ Active│  │
│  │ photos.example.com   rpi-home  :2283          ✓ Active│  │
│  │ files.example.com    rpi-home  :8080          ✓ Active│  │
│  │ admin.example.com    vm2-db    :3000          ✓ Active│  │
│  │                                        [View All Routes]│  │
│  └────────────────────────────────────────────────────────┘  │
└──────────────────────────────────────────────────────────────┘
```

### Routes管理画面

```
┌──────────────────────────────────────────────────────────────┐
│  Dashboard   Routes   Origins   Edge Nodes   Settings        │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Routes                                       [+ Add Route]  │
│                                                               │
│  🔍 [Search routes...]                    Filter: [All ▼]   │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Hostname             Path  Origin   Service    Actions │  │
│  ├────────────────────────────────────────────────────────┤  │
│  │ blog.example.com     /     rpi-home :2368      [⚙][🗑] │  │
│  │ 🔒 SSL: Auto (Let's Encrypt) | Status: ✓ Active        │  │
│  │ Traffic: 1.2K req/day | Uptime: 99.8%                  │  │
│  │                                                        │  │
│  │ api.example.com      /     vm1      :8080      [⚙][🗑] │  │
│  │ 🔒 SSL: Auto | Status: ✓ Active                        │  │
│  │ Traffic: 15.3K req/day | Uptime: 99.9%                 │  │
│  │                                                        │  │
│  │ example.com          /     vm1      :3000      [⚙][🗑] │  │
│  │ example.com          /api  vm1      :8080      [⚙][🗑] │  │
│  │ example.com          /blog vm2      :2368      [⚙][🗑] │  │
│  │ 🔒 SSL: Auto | Status: ✓ Active                        │  │
│  │                                                        │  │
│  │ *.app.example.com    /     vm1      :3000      [⚙][🗑] │  │
│  │ 🔒 SSL: Wildcard | Status: ✓ Active                    │  │
│  │ Matched subdomains: user1, user2, user3, ...           │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  Showing 12 routes                               [1] 2 3 >   │
└──────────────────────────────────────────────────────────────┘
```

### ルート追加画面

```
┌──────────────────────────────────────────────────────────────┐
│  Routes > Add Route                                   [✕]    │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Basic Settings                                               │
│                                                               │
│  Hostname *                                                   │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ blog.example.com                                     │    │
│  └──────────────────────────────────────────────────────┘    │
│  ℹ️ Supports wildcards: *.app.example.com                     │
│                                                               │
│  Path                                                         │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ /                                                    │    │
│  └──────────────────────────────────────────────────────┘    │
│  ℹ️ Leave as / for root, or specify /api, /admin, etc.        │
│                                                               │
│  Origin *                                                     │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ rpi-home                                          ▼  │    │
│  │ ├─ rpi-home       (Connected, 10.0.0.5)             │    │
│  │ ├─ vm1            (Connected, 10.0.0.10)            │    │
│  │ └─ vm2-db         (Connected, 10.0.0.11)            │    │
│  └──────────────────────────────────────────────────────┘    │
│                                                               │
│  Service URL *                                                │
│  ┌──────────────────────────────────────────────────────┐    │
│  │ http://localhost:2368                                │    │
│  └──────────────────────────────────────────────────────┘    │
│  ℹ️ Example: http://localhost:3000 or http://127.0.0.1:8080  │
│                                                               │
│  ─────────────────────────────────────────────────────────   │
│                                                               │
│  SSL/TLS Settings                                             │
│                                                               │
│  ☑ Enable HTTPS                                              │
│  ○ Auto-obtain certificate (Let's Encrypt)                   │
│  ○ Use existing certificate                                  │
│  ○ Cloudflare Origin Certificate                             │
│                                                               │
│  ─────────────────────────────────────────────────────────   │
│                                                               │
│  Advanced Settings        [▼ Show]                           │
│                                                               │
│  ☑ Enable WebSocket support                                  │
│  ☑ Preserve Host header                                      │
│  ☐ Strip path prefix                                         │
│                                                               │
│  Failover                 [▼ Show]                           │
│                                                               │
│  ☐ Enable automatic failover                                 │
│                                                               │
│  Temporary Route          [▼ Show]                           │
│                                                               │
│  ☐ Auto-delete after: [24 ▼] hours                           │
│                                                               │
│  ─────────────────────────────────────────────────────────   │
│                                                               │
│  [Cancel]                              [Test] [Create Route] │
└──────────────────────────────────────────────────────────────┘
```

---

## エージェントの動作

### kokoa-agent の責務

1. **ハートビート送信**（30秒間隔）
   ```json
   POST /api/v1/heartbeat
   {
     "origin_id": "660e8400-...",
     "status": "healthy",
     "wireguard_status": "connected",
     "services": [
       {
         "url": "http://localhost:2368",
         "status": "up",
         "response_time_ms": 23
       },
       {
         "url": "http://localhost:8080",
         "status": "up",
         "response_time_ms": 12
       }
     ]
   }
   ```

2. **設定受信・適用**（WebSocket経由）
   ```json
   {
     "type": "config_update",
     "version": "v2",
     "wireguard": {
       "peers": [
         {
           "public_key": "...",
           "allowed_ips": "10.0.0.10/32"
         }
       ]
     }
   }
   ```

3. **ローカルサービスのヘルスチェック**
   - 登録されたサービスURLに定期的にリクエスト
   - 結果をControl Planeに報告

4. **ログ送信**（オプション）
   - サービスのアクセスログ
   - エラーログ

---

## コンテナ環境との統合

### UC-009: Kubernetes クラスタ内のサービス公開

**シナリオ:**
自宅のK8sクラスタで動いているアプリを、Kokoa Proxy経由で外部公開したい。

#### パターン1: サイドカーパターン

**Deployment例:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
  namespace: default
spec:
  replicas: 1
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      # メインアプリケーション
      - name: app
        image: my-app:latest
        ports:
        - containerPort: 3000

      # Kokoa Agent (サイドカー)
      - name: kokoa-agent
        image: kokoaproxy/agent:latest
        env:
        - name: KOKOA_CONTROL_PLANE
          value: "https://control.example.com"
        - name: KOKOA_ORIGIN_NAME
          value: "k8s-cluster-home"
        - name: KOKOA_TOKEN
          valueFrom:
            secretKeyRef:
              name: kokoa-token
              key: token
        securityContext:
          capabilities:
            add:
            - NET_ADMIN  # WireGuard用
        volumeMounts:
        - name: wireguard
          mountPath: /etc/wireguard

      volumes:
      - name: wireguard
        emptyDir: {}
```

**使用方法:**

```bash
# 1. Kokoa Tokenを取得（Web UIまたはAPI）
kubectl create secret generic kokoa-token \
  --from-literal=token='eyJhbGc...'

# 2. Deploymentをデプロイ
kubectl apply -f my-app-deployment.yaml

# 3. Web UIでルーティング設定
# app.example.com → k8s-cluster-home (http://localhost:3000)
```

---

#### パターン2: DaemonSet（クラスタ全体で1つのトンネル）

**DaemonSet例:**

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kokoa-agent
  namespace: kube-system
spec:
  selector:
    matchLabels:
      app: kokoa-agent
  template:
    metadata:
      labels:
        app: kokoa-agent
    spec:
      hostNetwork: true  # ホストネットワークを使用
      containers:
      - name: agent
        image: kokoaproxy/agent:latest
        env:
        - name: KOKOA_CONTROL_PLANE
          value: "https://control.example.com"
        - name: KOKOA_ORIGIN_NAME
          valueFrom:
            fieldRef:
              fieldPath: spec.nodeName  # ノード名を使用
        - name: KOKOA_TOKEN
          valueFrom:
            secretKeyRef:
              name: kokoa-token
              key: token
        securityContext:
          privileged: true  # WireGuard用
        volumeMounts:
        - name: wireguard
          mountPath: /etc/wireguard

      volumes:
      - name: wireguard
        hostPath:
          path: /etc/wireguard
          type: DirectoryOrCreate
```

**ルーティング設定:**

```
# Web UIで各Serviceをルーティング
app.example.com      → k8s-node-1 (http://my-app-service.default.svc:3000)
api.example.com      → k8s-node-1 (http://api-service.default.svc:8080)
grafana.example.com  → k8s-node-1 (http://grafana.monitoring.svc:3000)
```

---

#### パターン3: Kubernetes Ingress統合

**CRD (Custom Resource Definition) 例:**

```yaml
apiVersion: kokoa.io/v1alpha1
kind: KokoaRoute
metadata:
  name: my-app-route
  namespace: default
spec:
  hostname: app.example.com
  service:
    name: my-app-service
    port: 3000
  ssl:
    enabled: true
    type: auto
  origin:
    name: k8s-cluster-home
```

**Kokoa Operator がこれを検知して:**
1. Control Plane APIにルーティング作成
2. Serviceの変更を監視
3. 自動的にルーティング更新

---

#### パターン4: Helm Chartで簡単デプロイ

```bash
# Helm リポジトリ追加
helm repo add kokoa https://charts.kokoaproxy.io
helm repo update

# インストール
helm install kokoa-agent kokoa/agent \
  --set controlPlane.url=https://control.example.com \
  --set origin.name=k8s-cluster-home \
  --set token=eyJhbGc...

# アプリと一緒にデプロイ
helm install my-app ./my-app-chart \
  --set kokoa.enabled=true \
  --set kokoa.hostname=app.example.com \
  --set kokoa.origin=k8s-cluster-home
```

**values.yaml:**

```yaml
# my-app-chart/values.yaml
kokoa:
  enabled: true
  hostname: app.example.com
  origin: k8s-cluster-home
  service:
    port: 3000
  ssl:
    enabled: true
```

---

### UC-010: Docker Compose での使用

**シナリオ:**
Docker Composeで複数のサービスを動かしていて、全て公開したい。

**docker-compose.yml:**

```yaml
version: '3.8'

services:
  # メインアプリ
  web:
    image: nginx:alpine
    ports:
      - "3000:80"

  api:
    image: my-api:latest
    ports:
      - "8080:8080"

  db:
    image: postgres:15
    environment:
      POSTGRES_PASSWORD: secret

  # Kokoa Agent
  kokoa-agent:
    image: kokoaproxy/agent:latest
    environment:
      KOKOA_CONTROL_PLANE: https://control.example.com
      KOKOA_ORIGIN_NAME: docker-home
      KOKOA_TOKEN: ${KOKOA_TOKEN}
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun
    volumes:
      - wireguard:/etc/wireguard
    restart: unless-stopped

volumes:
  wireguard:
```

**.env:**

```bash
KOKOA_TOKEN=eyJhbGc...
```

**起動:**

```bash
# 1. トークンを.envに設定
echo "KOKOA_TOKEN=eyJhbGc..." > .env

# 2. docker-compose起動
docker-compose up -d

# 3. Web UIでルーティング設定
# web.example.com  → docker-home (http://web:80)
# api.example.com  → docker-home (http://api:8080)
```

---

#### Docker Composeでの自動検出（ラベルベース）

**docker-compose.yml（拡張版）:**

```yaml
version: '3.8'

services:
  web:
    image: nginx:alpine
    labels:
      kokoa.enable: "true"
      kokoa.hostname: "web.example.com"
      kokoa.port: "80"
      kokoa.ssl: "auto"
    ports:
      - "3000:80"

  api:
    image: my-api:latest
    labels:
      kokoa.enable: "true"
      kokoa.hostname: "api.example.com"
      kokoa.port: "8080"
      kokoa.path: "/api"  # パスベースルーティング
    ports:
      - "8080:8080"

  grafana:
    image: grafana/grafana:latest
    labels:
      kokoa.enable: "true"
      kokoa.hostname: "grafana.example.com"
      kokoa.port: "3000"
    ports:
      - "3001:3000"

  kokoa-agent:
    image: kokoaproxy/agent:latest
    environment:
      KOKOA_CONTROL_PLANE: https://control.example.com
      KOKOA_ORIGIN_NAME: docker-home
      KOKOA_TOKEN: ${KOKOA_TOKEN}
      KOKOA_AUTO_DISCOVER: "true"  # 自動検出を有効化
    cap_add:
      - NET_ADMIN
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock  # Docker API接続
      - wireguard:/etc/wireguard
    restart: unless-stopped

volumes:
  wireguard:
```

**動作:**
1. `kokoa-agent` がDocker APIを監視
2. `kokoa.enable=true` のコンテナを検出
3. 自動的にControl Plane APIでルーティング作成
4. コンテナが停止したら、自動的にルーティング削除

---

### UC-011: Docker単体コンテナとしての使用

**シナリオ:**
既存のDockerコンテナを、Kokoa Proxy経由で公開したい。

```bash
# 1. 既存のアプリコンテナ
docker run -d --name my-app \
  -p 3000:3000 \
  my-app:latest

# 2. Kokoa Agent を起動（ネットワーク共有）
docker run -d --name kokoa-agent \
  --network container:my-app \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  -e KOKOA_CONTROL_PLANE=https://control.example.com \
  -e KOKOA_ORIGIN_NAME=docker-container \
  -e KOKOA_TOKEN=eyJhbGc... \
  kokoaproxy/agent:latest

# 3. Web UIでルーティング設定
# app.example.com → docker-container (http://localhost:3000)
```

**または、Docker Networkを使用:**

```bash
# 1. 専用ネットワーク作成
docker network create kokoa-net

# 2. アプリコンテナを起動
docker run -d --name my-app \
  --network kokoa-net \
  my-app:latest

# 3. Kokoa Agent を同じネットワークで起動
docker run -d --name kokoa-agent \
  --network kokoa-net \
  --cap-add NET_ADMIN \
  --device /dev/net/tun \
  -e KOKOA_CONTROL_PLANE=https://control.example.com \
  -e KOKOA_ORIGIN_NAME=docker-network \
  -e KOKOA_TOKEN=eyJhbGc... \
  kokoaproxy/agent:latest

# 4. Web UIでルーティング設定
# app.example.com → docker-network (http://my-app:3000)
```

---

### UC-012: Kubernetes Operator による完全自動化

**シナリオ:**
Kubernetesのアノテーションだけで、自動的にルーティングを作成したい。

**Deployment with annotations:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-app
spec:
  replicas: 2
  selector:
    matchLabels:
      app: my-app
  template:
    metadata:
      labels:
        app: my-app
    spec:
      containers:
      - name: app
        image: my-app:latest
        ports:
        - containerPort: 3000

---
apiVersion: v1
kind: Service
metadata:
  name: my-app-service
  annotations:
    kokoa.io/expose: "true"
    kokoa.io/hostname: "app.example.com"
    kokoa.io/origin: "k8s-cluster"
    kokoa.io/ssl: "auto"
spec:
  selector:
    app: my-app
  ports:
  - port: 80
    targetPort: 3000
```

**Kokoa Operatorの動作:**

```
1. Serviceの作成を検知
   ↓
2. アノテーションを解析
   kokoa.io/expose: "true"
   kokoa.io/hostname: "app.example.com"
   ↓
3. Control Plane APIを呼び出し
   POST /api/v1/routes
   {
     "hostname": "app.example.com",
     "origin_id": "k8s-cluster",
     "service_url": "http://my-app-service.default.svc:80"
   }
   ↓
4. ルーティング自動作成
   ↓
5. Serviceが削除されたら、ルーティングも自動削除
```

**インストール:**

```bash
# Operator インストール
kubectl apply -f https://raw.githubusercontent.com/kokoa-proxy/operator/main/install.yaml

# Operator設定
kubectl create secret generic kokoa-operator-config \
  --from-literal=control-plane-url=https://control.example.com \
  --from-literal=origin-name=k8s-cluster \
  --from-literal=token=eyJhbGc...

# これで、アノテーション付きServiceが自動的に公開される
```

---

### コンテナ環境でのデータモデル

#### Origin登録情報の拡張

```sql
-- origins テーブルに platform_type を追加
ALTER TABLE origins ADD COLUMN platform_type VARCHAR(20) DEFAULT 'host';
-- 'host', 'docker', 'k8s', 'docker-compose'

-- K8s固有の情報
CREATE TABLE origin_k8s_metadata (
  id UUID PRIMARY KEY,
  origin_id UUID REFERENCES origins(id),
  cluster_name VARCHAR(255),
  namespace VARCHAR(255),
  node_name VARCHAR(255),
  kubeconfig TEXT,  -- 暗号化保存
  created_at TIMESTAMP DEFAULT NOW()
);

-- Docker固有の情報
CREATE TABLE origin_docker_metadata (
  id UUID PRIMARY KEY,
  origin_id UUID REFERENCES origins(id),
  docker_host VARCHAR(255),  -- unix:///var/run/docker.sock
  compose_project VARCHAR(255),
  auto_discover BOOLEAN DEFAULT false,
  created_at TIMESTAMP DEFAULT NOW()
);
```

#### ルーティングの拡張

```sql
-- routes テーブルに service_type を追加
ALTER TABLE routes ADD COLUMN service_type VARCHAR(20) DEFAULT 'url';
-- 'url', 'k8s-service', 'docker-container'

-- K8s Service参照
CREATE TABLE route_k8s_service (
  id UUID PRIMARY KEY,
  route_id UUID REFERENCES routes(id),
  namespace VARCHAR(255) NOT NULL,
  service_name VARCHAR(255) NOT NULL,
  port INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT NOW()
);

-- Docker Container参照
CREATE TABLE route_docker_container (
  id UUID PRIMARY KEY,
  route_id UUID REFERENCES routes(id),
  container_name VARCHAR(255),
  container_id VARCHAR(64),
  port INTEGER NOT NULL,
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

### コンテナイメージ

#### kokoa-agent イメージの構成

**Dockerfile:**

```dockerfile
FROM alpine:3.19

# WireGuard & 必要なパッケージ
RUN apk add --no-cache \
    wireguard-tools \
    iptables \
    ca-certificates \
    curl

# kokoa-agent バイナリ
COPY kokoa-agent /usr/local/bin/kokoa-agent
RUN chmod +x /usr/local/bin/kokoa-agent

# 設定ディレクトリ
RUN mkdir -p /etc/kokoa /etc/wireguard

# ヘルスチェック
HEALTHCHECK --interval=30s --timeout=3s \
  CMD kokoa-agent health || exit 1

ENTRYPOINT ["/usr/local/bin/kokoa-agent"]
CMD ["run"]
```

**マルチアーキテクチャ対応:**

```bash
docker buildx build --platform linux/amd64,linux/arm64,linux/arm/v7 \
  -t kokoaproxy/agent:latest \
  -t kokoaproxy/agent:v1.0.0 \
  --push .
```

**イメージサイズ最適化:**

- ベースイメージ: Alpine Linux（5MB）
- WireGuard: 1MB
- kokoa-agent バイナリ: 10-15MB
- **合計: 約20MB**

---

### Helm Chart 構成

**Chart.yaml:**

```yaml
apiVersion: v2
name: kokoa-agent
description: Kokoa Proxy Agent for Kubernetes
version: 1.0.0
appVersion: 1.0.0
```

**values.yaml:**

```yaml
# Control Plane設定
controlPlane:
  url: https://control.example.com
  token: ""  # secretから取得推奨

# Origin設定
origin:
  name: ""  # 空の場合はクラスタ名を使用
  platformType: k8s

# デプロイモード
deploymentMode: daemonset  # daemonset, deployment, sidecar

# DaemonSet設定
daemonset:
  enabled: true
  hostNetwork: true
  privileged: true

# リソース制限
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

# 自動検出
autoDiscovery:
  enabled: false
  namespaces: []  # 空の場合は全namespace

# Operator
operator:
  enabled: false
  rbac:
    create: true
```

**templates/daemonset.yaml:**

```yaml
{{- if eq .Values.deploymentMode "daemonset" }}
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: {{ include "kokoa-agent.fullname" . }}
  labels:
    {{- include "kokoa-agent.labels" . | nindent 4 }}
spec:
  selector:
    matchLabels:
      {{- include "kokoa-agent.selectorLabels" . | nindent 6 }}
  template:
    metadata:
      labels:
        {{- include "kokoa-agent.selectorLabels" . | nindent 8 }}
    spec:
      hostNetwork: {{ .Values.daemonset.hostNetwork }}
      containers:
      - name: agent
        image: "kokoaproxy/agent:{{ .Values.image.tag | default .Chart.AppVersion }}"
        env:
        - name: KOKOA_CONTROL_PLANE
          value: {{ .Values.controlPlane.url }}
        - name: KOKOA_TOKEN
          valueFrom:
            secretKeyRef:
              name: {{ include "kokoa-agent.fullname" . }}-token
              key: token
        - name: KOKOA_ORIGIN_NAME
          value: {{ .Values.origin.name | default .Release.Name }}
        securityContext:
          privileged: {{ .Values.daemonset.privileged }}
        resources:
          {{- toYaml .Values.resources | nindent 10 }}
{{- end }}
```

---

### Web UI での表示

#### Origin一覧画面（コンテナ対応）

```
┌──────────────────────────────────────────────────────────────┐
│  Origins                                      [+ Add Origin]  │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Name             Platform    Status      IP          Uptime │
│  ─────────────────────────────────────────────────────────── │
│  🖥️ rpi-home       Host        Connected   10.0.0.5    15d   │
│  🖥️ vm1            Host        Connected   10.0.0.10   30d   │
│  🐳 docker-home    Docker      Connected   10.0.0.15   7d    │
│     └─ 3 containers discovered                                │
│  ☸️  k8s-cluster   Kubernetes  Connected   10.0.0.20   45d   │
│     └─ 12 services in 3 namespaces                            │
│  🐳 docker-dev     Docker      Temporary   10.0.0.25   2h    │
│     └─ Expires in 22 hours                                    │
│                                                               │
└──────────────────────────────────────────────────────────────┘
```

#### Route作成画面（コンテナ対応）

```
┌──────────────────────────────────────────────────────────────┐
│  Create Route                                                 │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Hostname: app.example.com                                    │
│                                                               │
│  Origin:   [k8s-cluster ▼] ☸️                                 │
│                                                               │
│  Service Type:                                                │
│  ○ URL (http://localhost:3000)                               │
│  ● Kubernetes Service                                         │
│  ○ Docker Container                                           │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Kubernetes Service                                     │  │
│  │                                                        │  │
│  │ Namespace: [default ▼]                                │  │
│  │                                                        │  │
│  │ Service:   [my-app-service ▼]                         │  │
│  │            └─ Ports: 80 → 3000                        │  │
│  │                                                        │  │
│  │ Port:      [80 ▼]                                     │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  Preview:                                                     │
│  app.example.com → k8s-cluster                                │
│                    (http://my-app-service.default.svc:80)     │
│                                                               │
│  [Cancel]                                    [Create Route]   │
└──────────────────────────────────────────────────────────────┘
```

---

## Outbound Traffic（外向きトラフィック）のルーティング

### UC-013: OriginからのOutboundトラフィックをEdge経由で送信

**シナリオ:**
K8sやDockerから外部API（例: OpenAI API）にアクセスする際、Origin（自宅）のIPではなく、Edge NodeのIPから出ていくようにしたい。

**理由:**
- Origin（自宅）のIPアドレスを隠したい
- 複数Edge Nodeでoutboundトラフィックも冗長化
- 特定の国/リージョンからのアクセスが必要（GeoIP制限回避）
- 送信元IPを固定したい（API制限対策）

---

#### パターン1: フルトンネルモード（全トラフィック）

**Origin側の設定:**

```bash
# /etc/wireguard/kokoa.conf
[Interface]
PrivateKey = <origin_private_key>
Address = 10.0.0.5/24

# DNS設定（Edge経由）
DNS = 1.1.1.1, 8.8.8.8

[Peer]
PublicKey = <edge_node_public_key>
Endpoint = edge1.example.com:51820
AllowedIPs = 0.0.0.0/0  # 全トラフィックをトンネル経由
PersistentKeepalive = 25
```

**Edge Node側の設定:**

```bash
# /etc/wireguard/wg0.conf
[Interface]
Address = 10.0.0.10/24
PrivateKey = <edge_private_key>
ListenPort = 51820

# NAT設定（自動）
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT
PostUp = iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE
PostDown = iptables -D FORWARD -i wg0 -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

[Peer]
PublicKey = <origin_public_key>
AllowedIPs = 10.0.0.5/32
```

**動作:**
```
K8s Pod → WireGuard Tunnel → Edge Node → インターネット
         (10.0.0.5)         (10.0.0.10)   (203.0.113.10)

外部から見た送信元IP: 203.0.113.10（Edge NodeのIP）
```

---

#### パターン2: スプリットトンネル（特定の宛先のみ）

**シナリオ:**
特定のAPI（例: `api.openai.com`）へのアクセスのみEdge経由にしたい。

**Origin側の設定:**

```bash
# /etc/wireguard/kokoa.conf
[Interface]
PrivateKey = <origin_private_key>
Address = 10.0.0.5/24

# テーブル設定
Table = 100

[Peer]
PublicKey = <edge_node_public_key>
Endpoint = edge1.example.com:51820
AllowedIPs = 10.0.0.0/24  # WireGuardネットワークのみ
PersistentKeepalive = 25

# カスタムルーティング
# api.openai.com (例: 104.18.7.192/32) のみEdge経由
PostUp = ip route add 104.18.7.0/24 dev kokoa table 100
PostUp = ip rule add to 104.18.7.0/24 lookup 100
PostDown = ip route del 104.18.7.0/24 dev kokoa table 100
PostDown = ip rule del to 104.18.7.0/24 lookup 100
```

**Web UIでの設定:**

```
┌──────────────────────────────────────────────────────────────┐
│  Origin: k8s-cluster > Outbound Routing                       │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Outbound Mode: ○ None  ○ Full Tunnel  ● Split Tunnel       │
│                                                               │
│  Route specific destinations via Edge Nodes:                  │
│                                                               │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Destination                  Via Edge Node             │  │
│  ├────────────────────────────────────────────────────────┤  │
│  │ api.openai.com              edge-sgp1        [🗑]      │  │
│  │ api.anthropic.com           edge-sgp1        [🗑]      │  │
│  │ 104.18.0.0/16               edge-us1         [🗑]      │  │
│  │                                                        │  │
│  │ [+ Add Destination]                                    │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  Default Route: ● Direct (Origin's IP)                        │
│                 ○ Via Edge Node: [Select ▼]                  │
│                                                               │
│  [Cancel]                                    [Save Changes]   │
└──────────────────────────────────────────────────────────────┘
```

---

#### パターン3: Kubernetesでのアプリケーション単位の設定

**シナリオ:**
K8sクラスタ内の特定のPodのみ、outboundトラフィックをEdge経由にしたい。

**NetworkPolicyを使用:**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: api-client
  annotations:
    kokoa.io/outbound: "true"
    kokoa.io/outbound-edge: "edge-sgp1"
spec:
  containers:
  - name: app
    image: my-api-client:latest
    env:
    - name: HTTP_PROXY
      value: "http://10.0.0.10:3128"  # Edge NodeのSquid Proxy
    - name: HTTPS_PROXY
      value: "http://10.0.0.10:3128"
```

**または、サイドカープロキシ:**

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: api-client
spec:
  containers:
  # メインアプリ
  - name: app
    image: my-api-client:latest

  # Kokoa Sidecar Proxy
  - name: kokoa-proxy
    image: kokoaproxy/sidecar-proxy:latest
    env:
    - name: KOKOA_OUTBOUND_ENABLED
      value: "true"
    - name: KOKOA_EDGE_NODE
      value: "edge-sgp1"
    ports:
    - containerPort: 3128  # HTTP Proxy
```

**アプリからの利用:**

```python
# Python例
import requests

# 自動的にEdge経由でリクエスト（環境変数HTTP_PROXYを参照）
response = requests.get('https://api.openai.com/v1/models')

# 送信元IP: Edge NodeのIP（203.0.113.10）
```

---

#### パターン4: Docker Composeでの設定

**docker-compose.yml:**

```yaml
version: '3.8'

services:
  app:
    image: my-app:latest
    environment:
      HTTP_PROXY: http://kokoa-proxy:3128
      HTTPS_PROXY: http://kokoa-proxy:3128
      NO_PROXY: localhost,127.0.0.1
    depends_on:
      - kokoa-proxy

  kokoa-proxy:
    image: kokoaproxy/outbound-proxy:latest
    environment:
      KOKOA_CONTROL_PLANE: https://control.example.com
      KOKOA_TOKEN: ${KOKOA_TOKEN}
      KOKOA_EDGE_NODE: edge-sgp1  # 使用するEdge Node
    ports:
      - "3128:3128"  # HTTP Proxy port
    cap_add:
      - NET_ADMIN
    devices:
      - /dev/net/tun
```

**動作:**

```
app → kokoa-proxy → WireGuard → Edge Node → インターネット
     (HTTP Proxy)   (10.0.0.5)   (10.0.0.10)  (203.0.113.10)
```

---

### Outbound用のデータモデル

```sql
-- outbound_routing テーブル
CREATE TABLE outbound_routes (
  id UUID PRIMARY KEY,
  origin_id UUID REFERENCES origins(id) NOT NULL,

  -- ルーティングモード
  mode VARCHAR(20) NOT NULL,  -- 'none', 'full', 'split'

  -- 宛先指定（split modeの場合）
  destination_type VARCHAR(20),  -- 'hostname', 'ip', 'cidr'
  destination_value VARCHAR(255),  -- api.openai.com, 104.18.7.192, 104.18.0.0/16

  -- Edge Node指定
  edge_node_id UUID REFERENCES edge_nodes(id),

  -- フォールバック
  fallback_mode VARCHAR(20) DEFAULT 'direct',  -- 'direct', 'another_edge'
  fallback_edge_node_id UUID REFERENCES edge_nodes(id),

  -- 優先度
  priority INTEGER DEFAULT 100,

  status VARCHAR(20) DEFAULT 'active',
  created_at TIMESTAMP DEFAULT NOW(),
  updated_at TIMESTAMP DEFAULT NOW()
);

-- インデックス
CREATE INDEX idx_outbound_routes_origin ON outbound_routes(origin_id);
CREATE INDEX idx_outbound_routes_priority ON outbound_routes(priority DESC);
```

---

### 複数Edge Nodeでのロードバランシング

**シナリオ:**
Outboundトラフィックを複数のEdge Nodeに分散したい。

**Web UIでの設定:**

```
┌──────────────────────────────────────────────────────────────┐
│  Outbound Load Balancing                                      │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Strategy: ● Round Robin  ○ Least Connections  ○ Random     │
│                                                               │
│  Edge Nodes:                                                  │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Edge Node          Weight    Status                    │  │
│  ├────────────────────────────────────────────────────────┤  │
│  │ ☑ edge-sgp1        50%       ✓ Active                 │  │
│  │ ☑ edge-us1         30%       ✓ Active                 │  │
│  │ ☑ edge-eu1         20%       ✓ Active                 │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  Health Check:                                                │
│  ☑ Automatic failover on edge failure                        │
│  Interval: [10] seconds                                       │
│                                                               │
│  [Save]                                                       │
└──────────────────────────────────────────────────────────────┘
```

**動作:**

```
Origin → Edge Selector → Edge Node 1 (50%) → Internet
                      → Edge Node 2 (30%) → Internet
                      → Edge Node 3 (20%) → Internet

Edge Node 1が停止
↓
自動的にEdge Node 2, 3にフェイルオーバー（60%/40%に再分配）
```

---

### 実装の技術詳細

#### Edge Node側のNAT設定（自動化）

**Control Planeが自動生成するEdge Node設定:**

```bash
#!/bin/bash
# edge-node-setup.sh

# WireGuard設定
cat > /etc/wireguard/wg0.conf <<EOF
[Interface]
Address = 10.0.0.10/24
PrivateKey = $(cat /etc/kokoa/edge-private.key)
ListenPort = 51820

# NAT for outbound traffic
PostUp = sysctl -w net.ipv4.ip_forward=1
PostUp = iptables -A FORWARD -i wg0 -j ACCEPT
PostUp = iptables -A FORWARD -o wg0 -j ACCEPT
PostUp = iptables -t nat -A POSTROUTING -o eth0 -j MASQUERADE

PostDown = iptables -D FORWARD -i wg0 -j ACCEPT
PostDown = iptables -D FORWARD -o wg0 -j ACCEPT
PostDown = iptables -t nat -D POSTROUTING -o eth0 -j MASQUERADE

[Peer]
PublicKey = <origin_public_key>
AllowedIPs = 10.0.0.5/32
EOF

# WireGuard起動
wg-quick up wg0
systemctl enable wg-quick@wg0

# Squid Proxy（オプション、HTTP/HTTPSプロキシ用）
apt install -y squid

cat > /etc/squid/squid.conf <<EOF
http_port 3128
acl localnet src 10.0.0.0/24
http_access allow localnet
http_access deny all
forwarded_for delete
via off
EOF

systemctl restart squid
```

---

#### K8s環境での実装（CNI統合）

**Cilium CNIを使用した例:**

```yaml
apiVersion: cilium.io/v2
kind: CiliumNetworkPolicy
metadata:
  name: kokoa-outbound-routing
spec:
  endpointSelector:
    matchLabels:
      kokoa.io/outbound: "true"
  egress:
  # 全outboundトラフィックをWireGuardトンネルへ
  - toEndpoints:
    - matchLabels:
        app: kokoa-agent
```

**または、iptablesベースのルーティング（汎用）:**

```bash
# kokoa-agent が実行するスクリプト（DaemonSet内）

# Pod CIDR取得
POD_CIDR=$(kubectl get nodes -o jsonpath='{.items[0].spec.podCIDR}')

# 特定のPodのトラフィックをWireGuardへ
for POD_IP in $(kubectl get pods -l kokoa.io/outbound=true -o jsonpath='{.items[*].status.podIP}'); do
  iptables -t nat -A PREROUTING -s $POD_IP -j DNAT --to-destination 10.0.0.10
done
```

---

### パフォーマンス考慮事項

#### レイテンシ

```
【Direct（通常）】
Origin → インターネット
レイテンシ: 10-50ms

【Edge経由（フルトンネル）】
Origin → WireGuard → Edge Node → インターネット
レイテンシ: +20-100ms（Edge Nodeの場所による）

【最適化】
- Edge Nodeを複数リージョンに配置
- 地理的に近いEdge Nodeを自動選択
- 低レイテンシが必要なトラフィックはDirectルーティング
```

#### スループット

```
WireGuardのオーバーヘッド: ~5-10%
Edge NodeのNATオーバーヘッド: ~2-5%

推奨Edge Nodeスペック（outbound用）:
- CPU: 2 core以上
- メモリ: 2GB以上
- ネットワーク: 1Gbps以上
```

---

### ユースケース例

#### 1. OpenAI APIアクセス（レート制限回避）

```yaml
# K8s Pod設定
apiVersion: v1
kind: Pod
metadata:
  name: openai-client
  annotations:
    kokoa.io/outbound: "true"
    kokoa.io/outbound-destination: "api.openai.com"
    kokoa.io/outbound-edge: "edge-us1"  # US IPから
spec:
  containers:
  - name: app
    image: openai-client:latest
```

**結果:**
- `api.openai.com`へのリクエストは全てEdge US1経由
- 送信元IP: Edge US1のアメリカのIP
- レート制限が個別に適用される

---

#### 2. 複数リージョンからのスクレイピング

```yaml
# Web UIで設定
Outbound Route #1
  Destination: www.example.com
  Edge Node: edge-us1 (weight: 33%)
  Edge Node: edge-eu1 (weight: 33%)
  Edge Node: edge-asia1 (weight: 34%)
```

**結果:**
- 同じスクレイパーが異なるIPから分散アクセス
- IP BAN回避

---

#### 3. 自宅IPの秘匿化

```yaml
# 全outboundトラフィックをEdge経由
Outbound Mode: Full Tunnel
Default Edge: edge-sgp1
```

**結果:**
- Origin（自宅）のIPアドレスが一切外部に漏れない
- 全てEdge NodeのIPから出ていく

---

### モニタリング

**Web UIでのOutbound統計表示:**

```
┌──────────────────────────────────────────────────────────────┐
│  Origin: k8s-cluster > Outbound Traffic                       │
├──────────────────────────────────────────────────────────────┤
│                                                               │
│  Last 24 hours                                                │
│                                                               │
│  Total Outbound: 15.2 GB                                      │
│  ├─ via edge-sgp1:  8.5 GB (56%)                             │
│  ├─ via edge-us1:   4.2 GB (28%)                             │
│  ├─ via edge-eu1:   2.5 GB (16%)                             │
│  └─ Direct:         0 GB (0%)                                │
│                                                               │
│  Top Destinations:                                            │
│  ┌────────────────────────────────────────────────────────┐  │
│  │ Destination              Traffic    Edge Node          │  │
│  ├────────────────────────────────────────────────────────┤  │
│  │ api.openai.com          5.2 GB      edge-us1          │  │
│  │ api.anthropic.com       3.8 GB      edge-us1          │  │
│  │ github.com              2.1 GB      edge-sgp1         │  │
│  │ docker.io               1.9 GB      edge-sgp1         │  │
│  │ registry.k8s.io         1.2 GB      edge-eu1          │  │
│  └────────────────────────────────────────────────────────┘  │
│                                                               │
│  [Export CSV]                                   [Refresh]     │
└──────────────────────────────────────────────────────────────┘
```

---

### セキュリティ考慮事項

#### 1. DNS Leak防止

```bash
# Origin側でDNSもWireGuard経由に
[Interface]
DNS = 10.0.0.10  # Edge NodeのDNSリゾルバー

# Edge Node側でUnboundなどのDNSサーバー起動
apt install unbound
systemctl enable unbound
```

#### 2. IPv6 Leak防止

```bash
# IPv6を完全無効化（必要に応じて）
sysctl -w net.ipv6.conf.all.disable_ipv6=1

# またはIPv6もWireGuard経由
[Interface]
Address = 10.0.0.5/24, fd00::5/64

[Peer]
AllowedIPs = 0.0.0.0/0, ::/0
```

#### 3. Kill Switch（トンネル切断時の保護）

```bash
# WireGuardトンネルが切断したら、全通信を遮断
PostUp = iptables -I OUTPUT -o eth0 -j DROP
PostUp = iptables -I OUTPUT -o wg0 -j ACCEPT
PostDown = iptables -D OUTPUT -o eth0 -j DROP
PostDown = iptables -D OUTPUT -o wg0 -j ACCEPT
```

---

## まとめ

### このシステムで実現すること

✅ **簡単な公開**: ワンライナー実行 + Web UIポチポチで即公開
✅ **複数サービス対応**: 無制限のルーティング設定
✅ **柔軟なルーティング**: ホスト名、パス、ワイルドカード
✅ **自動SSL**: Let's Encrypt統合
✅ **高可用性**: 複数Edge Nodeで冗長化
✅ **防弾ホスト**: 攻撃を受けたノードを即座に交換
✅ **完全セルフホスト**: Cloudflareに依存しない

### Cloudflare Tunnelとの比較

| 機能 | Cloudflare Tunnel | Kokoa Proxy |
|------|-------------------|-------------|
| ワンライナー登録 | ✅ `cloudflared tunnel` | ✅ `curl ... \| bash` |
| Web UI管理 | ✅ Zero Trust Dashboard | ✅ Self-hosted UI |
| 自動SSL | ✅ | ✅ Let's Encrypt |
| 複数Origin対応 | ✅ | ✅ |
| パスルーティング | ✅ | ✅ |
| セルフホスト | ❌ SaaS必須 | ✅ 完全セルフホスト |
| Edge Node制御 | ❌ Cloudflare管理 | ✅ 完全制御 |
| コスト | $0（制限あり）〜 | VPS代のみ |
| データ主権 | ❌ Cloudflare経由 | ✅ 自分で管理 |

---

## 次のステップ

このユースケースに基づいて：

1. **claude.mdの更新**
   - 機能要件のMUSTセクションを具体化
   - UI/UXフローに詳細を追加

2. **API設計の詳細化**
   - ルーティングCRUD API
   - Origin登録API
   - リアルタイム通知（WebSocket）

3. **プロトタイプ実装**
   - Phase 1: 基本的なルーティング機能
   - インストールスクリプトの実装

どの部分から進めますか？
