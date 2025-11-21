# Kokoa Proxy - モジュラーアーキテクチャ設計 (マルチドメイン対応)

## 概要

Cloudflare Tunnelのような体験を、モジュラーかつセルフホストで実現する高可用性トンネリングプロキシシステム。`app.hoge.com`や`wiki.fuga.com`のように、**複数の異なるベースドメインにまたがる**サブドメインを、単一または複数のバックエンドサービスに安全に公開します。

**設計思想:**
- **モジュラー**: 各機能を独立したモジュールに分離
- **プラガブル**: DNS Provider、VPS Providerなどを差し替え可能
- **スケーラブル**: 各モジュールを独立してスケール可能
- **保守性**: モジュール単位でアップデート・交換可能

---

## 3つの主要な登場人物

### 1. Control Plane（コントロールプレーン）
**何者？** 管理・オーケストレーション役
- Web UI/APIを提供
- Edge、Origin、Route（ホスト名とバックエンドの紐付け）を管理
- **DNS管理の責務**: MVPでは**単一のDNSゾーン**を管理。マルチゾーン対応は将来的な拡張。

**配置:** 信頼できるVPS or 自宅
**役割:** すべての指示を出す司令塔

---

### 2. Edge Node（エッジノード）
**何者？** インターネット側の入口（使い捨てVPS）
- **動的ルーティング**: Control Planeから取得した設定に基づき、リクエストされたホスト名（例: `app.hoge.com`）に応じて適切なOriginにトラフィックを転送する。
- **SSL終端**: 複数の異なるドメインの証明書をSNIで管理し、SSLを終端する。
- **エージェント**: 定期的にControl Planeに設定を問い合わせ、自身のNginx設定や証明書を更新する。

**配置:** 複数のVPS（DigitalOcean、Vultr等）
**役割:** 動的な防弾盾、攻撃を受けたら即破棄

---

### 3. Origin（オリジン）
**何者？** 実際のサービスが動いているバックエンドホスト
- 自宅サーバー、VM、K8s、Docker等
- WireGuardでEdgeからの接続を受け入れ
- 複数のアプリケーションを実行可能

**配置:** 自宅、プライベートネットワーク
**役割:** 保護されたバックエンド

---

## アーキテクチャ概要図（マルチドメイン対応）

```
        ┌──────────────────────────────────────┐
        │         インターネットユーザー          │
        └────────────────┬─────────────────────┘
                         │ HTTPS (app.hoge.com, wiki.fuga.com)
              ┌──────────┴──────────┐
              │                     │
              v                     v
    ┌─────────────────┐   ┌─────────────────┐
    │  Edge Node #1   │   │  Edge Node #2   │
    │  - Nginx (map)  │   │  - Nginx (map)  │
    │  - WG Client    │   │  - WG Client    │
    │  - Kokoa Agent  │   │  - Kokoa Agent  │
    └────────┬────────┘   └────────┬────────┘
             │ WireGuard Tunnel    │
             └─────────┬───────────┘
                       │
            ┌──────────┴──────────┐
            │                     │
            v                     v
 ┌──────────────────────┐  ┌──────────────────────┐
 │   Origin #1 (hoge)    │  │   Origin #2 (fuga)    │
 │  - WG Server         │  │  - WG Server         │
 │  - App "app" (8080)  │  │  - App "wiki" (8000) │
 └──────────────────────┘  └──────────────────────┘


    ┌──────────────────────────────────────────┐
    │      Control Plane (管理サーバー)         │
    │ ┌────────────────┐ ┌──────────────────┐ │
    │ │ Database       │ │ API (for Edges)  │ │
    │ │ - Origins      │ │ GET /config      │ │
    │ │ - Routes       │ └──────────────────┘ │
    │ │   - app.hoge.com -> O1:8080            │ │
    │ │   - wiki.fuga.com -> O2:8000           │ │
    │ └────────────────┘                      │
    └──────────────────┬─────────────────────┘
                       │ ▲
                       │ │ 設定問い合わせ (Pull)
                       └─┤
           ┌─────────────┘
           │
    ┌─────────────────┐
    │  Edge Node      │
    │ (Kokoa Agent)   │
    └─────────────────┘
```

---

## 通信フロー（Pullモデル）

1.  **Edge Nodeのエージェント**が定期的にControl Planeに設定を問い合わせる (`GET /api/v1/edge-nodes/me/config`)。
2.  **Control Plane**は、DBから全ルート情報を取得し、Nginxの`map`設定と管理対象のホスト名リスト（例: `["app.hoge.com", "wiki.fuga.com"]`）をJSONで返す。
3.  **Edgeエージェント**は受信した設定を適用する。
    *   Nginxの`map`ファイルを更新し、`nginx -s reload`を実行。
    *   ホスト名リストに新しいホストがあれば`certbot`で証明書を取得。

---

## データモデル（マルチドメイン対応）

### Control Plane DB Schema

```sql
-- オリジンサーバー（単なるWireGuardピア）
CREATE TABLE origins (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  wireguard_ip TEXT NOT NULL UNIQUE,
  wireguard_public_key TEXT NOT NULL,
  wireguard_private_key_encrypted TEXT NOT NULL,
  created_at INTEGER NOT NULL
);

-- ルート（ホスト名とバックエンドサービスの紐付け）
CREATE TABLE routes (
  id TEXT PRIMARY KEY,
  -- app.hoge.com のような完全なホスト名を格納
  hostname TEXT NOT NULL UNIQUE,
  origin_id TEXT NOT NULL,
  -- Origin内の転送先ポート
  target_port INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(origin_id) REFERENCES origins(id)
);

-- エッジノード
CREATE TABLE edge_nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    auth_token_hash TEXT NOT NULL UNIQUE, -- 永続的な認証トークン
    last_seen INTEGER
);
```

---

## API設計（マルチドメイン対応）

### Control Plane REST API

#### Route管理
```http
# 新規ルート作成
POST /api/v1/routes
{
  "hostname": "app.hoge.com",
  "origin_id": "origin-id-1",
  "target_port": 8080
}
Response: 201 Created
```

#### Edge Node用API
```http
# Edge Nodeが設定をプルするためのエンドポイント
GET /api/v1/edge-nodes/me/config
Authorization: Bearer <persistent-jwt-for-this-node>

Response: 200 OK
{
  "config_hash": "sha256-of-this-payload",
  "routes": {
    "app.hoge.com": "10.100.1.1:8080",
    "wiki.fuga.com": "10.100.2.1:8000"
  },
  "hostnames": [
    "app.hoge.com",
    "wiki.fuga.com"
  ]
}
```

---

## Nginx設定（`map`を利用した動的ルーティング）

Edge Node上のエージェントが、Control Planeから取得した`routes`オブジェクトを元に、以下の様な`map`ファイルを生成します。

```nginx
# /etc/nginx/conf.d/kokoa-map.conf
map $host $kokoa_backend {
    "app.hoge.com"       "10.100.1.1:8080";
    "wiki.fuga.com"      "10.100.2.1:8000";
    default              "";
}
```

メインのNginx設定 (`/etc/nginx/sites-available/default`) は、`server_name`を省略するか`_`（デフォルトサーバー）にすることで、IPアドレスに到達したすべてのリクエストを受け付けます。

```nginx
# /etc/nginx/sites-available/default
server {
    listen 443 ssl http2;
    listen 80;

    # すべてのホスト名リクエストを受ける
    server_name _;

    # SSL証明書はSNIにより自動で選択される
    # この部分はエージェントがcertbotで証明書を取得するたびに更新・追加される
    ssl_certificate /etc/letsencrypt/live/app.hoge.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/app.hoge.com/privkey.pem;
    ssl_certificate /etc/letsencrypt/live/wiki.fuga.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/wiki.fuga.com/privkey.pem;

    location / {
        if ($kokoa_backend = "") {
            return 404; # mapに定義されていないホストは404
        }
        proxy_pass http://$kokoa_backend;
        # ... proxyヘッダー設定
    }
}
```

---

## DNSとSSLの管理スコープ

- **ルーティングとSSL**: Edge Nodeは**任意のドメイン**のホスト名に対応可能です。`app.hoge.com`と`wiki.fuga.com`を同時に処理し、それぞれの証明書を個別に管理します。
- **DNSレコード自動更新**: Control Planeに統合された`DNS Manager`モジュールは、MVPの段階では**単一のDNSゾーン**（例: `hoge.com`）の管理を前提とします。
  - `app.hoge.com`のAレコードはControl Planeで自動管理できます。
  - `wiki.fuga.com`のAレコードは、`fuga.com`のDNS管理画面で**手動で**Edge NodeのIPアドレスを設定する必要があります。
  - 将来的な拡張として、Control Planeが複数のDNSゾーンの認証情報を管理できるようにすることが考えられます。

---

## トラフィックフロー（重要！）

### データプレーン vs コントロールプレーン

**❗重要:** Control Planeは実際のユーザートラフィックを通りません。

```
┌─────────────────────────────────────────────────────────┐
│                   データプレーン                         │
│         （実際のユーザートラフィック）                    │
└─────────────────────────────────────────────────────────┘

【Inbound（外部→サービス）】
ユーザー (1.2.3.4)
  ↓ HTTPS
Edge Node (203.0.113.10)
  ↓ WireGuard (暗号化、直接接続)
Origin (10.0.1.1)
  ↓ HTTP
アプリ (localhost:3000)

※ Control Planeは経由しない！


【Outbound（サービス→外部）】※オプション
アプリ (localhost:3000)
  ↓
Origin (10.0.1.1)
  ↓ WireGuard (暗号化、直接接続)
Edge Node (10.0.1.10)
  ↓ NAT
インターネット (送信元: 203.0.113.10)

※ Control Planeは経由しない！


┌─────────────────────────────────────────────────────────┐
│                 コントロールプレーン                      │
│              （管理・設定・監視のみ）                     │
└─────────────────────────────────────────────────────────┘

【管理・設定】
管理者
  ↓ HTTPS
Control Plane (Web UI/API)
  ↓ API呼び出し
Edge Provisioner Module → VPS作成
DNS Manager Module → DNS更新
SSL Manager Module → 証明書取得

【監視】
Edge Node ──┐
Origin     ──┼→ ハートビート → Control Plane
Health Check ┘

※ ユーザートラフィックは通らない！
```

---

### なぜControl Planeを経由しないのか？

**理由1: 単一障害点の回避**
```
❌ Control Plane経由の場合:
ユーザー → Edge → Control Plane → Origin
                      ↑
                  ここが止まると全滅
```

**理由2: ボトルネックの回避**
```
Control Planeが全トラフィックを処理
→ 高スペックサーバーが必要
→ コスト増
→ スケールしない
```

**理由3: レイテンシの最小化**
```
✅ 現在の設計:
Edge → Origin (1ホップ、10-30ms)

❌ Control Plane経由:
Edge → Control Plane → Origin (2ホップ、20-60ms)
```

**理由4: シンプルさ**
```
WireGuardの暗号化トンネルで直接接続
→ Control Planeが落ちても既存の通信は継続
```

---

### Control Planeの役割（データは通さない）

**Control Planeがやること:**
- ✅ Edge Node作成・削除
- ✅ WireGuard設定生成・配布
- ✅ DNS レコード管理
- ✅ SSL証明書管理
- ✅ ヘルスチェック・監視
- ✅ ルーティング設定管理

**Control Planeがやらないこと:**
- ❌ ユーザートラフィックの転送
- ❌ プロキシ・ロードバランシング
- ❌ SSL終端

--- 

### 具体例: ユーザーがapp.example.comにアクセス

```
ステップ1: DNS問い合わせ
ユーザー → DNS (app.example.com?)
← 203.0.113.10, 203.0.113.11 (Edge NodeのIP)

ステップ2: HTTPSリクエスト
ユーザー (1.2.3.4)
  → Edge Node 1 (203.0.113.10:443)
    GET https://app.example.com/

ステップ3: Edge NodeがSSL終端
Edge Node 1:
  - SSL終端（証明書検証）
  - Nginxがupstream選択
    upstream origin_a { server 10.0.1.1:80; }

ステップ4: WireGuard経由で転送
Edge Node 1 (10.0.1.10)
  → WireGuardトンネル (暗号化)
    → Origin (10.0.1.1:80)
      GET / HTTP/1.1
      Host: app.example.com
      X-Real-IP: 1.2.3.4

ステップ5: Originがアプリに転送
Origin:
  - HAProxy/Nginxがアプリに転送
    → localhost:3000

ステップ6: レスポンス（逆方向）
アプリ → Origin → WireGuard → Edge → ユーザー

※ 全プロセスでControl Planeは関与しない
```

---

### Control Planeが停止した場合の影響

**既存のトラフィック:** 
```
✅ 影響なし
Edge ↔ Origin のWireGuardトンネルは継続
ユーザーは普通にアクセス可能
```

**できなくなること:** 
```
❌ 新しいEdge Node追加
❌ DNS変更
❌ SSL証明書更新
❌ ヘルスチェック（自動フェイルオーバー停止）
❌ Web UI/API操作
```

**復旧:** 
```
Control Planeを再起動すれば元通り
既存のEdge/Originには影響なし
```

---

### アーキテクチャ図（データプレーン分離版）

```
┌────────────────────────────────────────────────────┐
│                データプレーン                       │
│           （ユーザートラフィック経路）               │
└────────────────────────────────────────────────────┘

  [ユーザー]
      ↓ HTTPS
  [Edge Node] ←─┐
      ↓ WG      │ 直接接続
  [Origin]  ←───┘ (Control Plane経由しない)
      ↓
  [アプリ]


┌────────────────────────────────────────────────────┐
│              コントロールプレーン                    │
│          （管理・設定・監視のみ）                    │
└────────────────────────────────────────────────────┘

  [管理者]
      ↓
  [Control Plane]
      ├─→ [Edge Node] (設定配布、ヘルスチェック)
      ├─→ [Origin]    (設定配布、ヘルスチェック)
      ├─→ [DNS API]   (レコード管理)
      └─→ [VPS API]   (Edge作成・削除)
```

---

### まとめ

| 項目 | 経由するもの | 経由しないもの |
|------|------------|--------------|
| **ユーザートラフィック** | Edge Node, Origin | ❌ Control Plane |
| **WireGuard設定** | ✅ Control Plane | Edge, Origin |
| **DNS更新** | ✅ Control Plane | Edge, Origin |
| **ヘルスチェック** | ✅ Control Plane | ユーザートラフィック |

**結論:**
- **データプレーン**: Edge ↔ Origin（直接、高速）
- **コントロールプレーン**: Control Plane → すべて（管理のみ）

これにより、Control Planeが停止してもサービスは継続します。

---

## コアコンポーネント

### Control Plane Core

**役割:**
- 全体のオーケストレーション
- 状態管理（DB）
- Web UI/API提供
- モジュール間の調整

**責務:**
- Route管理（CRUD）
- Origin/Edge Node管理（CRUD）
- モジュールへのタスク委譲
- イベント配信（WebSocket）
- 認証・認可

**技術スタック:**
- バックエンド: Go/Python/Node.js
- DB: PostgreSQL/SQLite
- キュー: Redis/Memory Queue
- WebSocket: Socket.io/native

**含まない機能（モジュールに委譲）:**
- DNS操作（DNS Manager Moduleへ）
- VPS操作（Edge Provisioner Moduleへ）
- 証明書操作（SSL Manager Moduleへ）
- ヘルスチェック実行（Health Checker Moduleへ）

---

## モジュール設計

### 1. DNS Manager Module

**役割:**
DNSレコードの自動管理

**プロバイダー実装:**
- Cloudflare（優先）
- AWS Route53
- Google Cloud DNS
- PowerDNS（セルフホスト）
- Manual（API経由で手動実行）

**API Interface:**

```go
type DNSManager interface {
    // レコード追加
    AddRecord(hostname string, recordType string, value string) error

    // レコード削除
    RemoveRecord(hostname string, recordID string) error

    // レコード一覧取得
    ListRecords(hostname string) ([]DNSRecord, error)

    // ヘルスチェック統合（オプション）
    EnableHealthCheck(recordID string, healthCheckConfig HealthCheckConfig) error
}
```

**設定例:**

```yaml
# config.yaml
dns:
  provider: cloudflare
  cloudflare:
    api_token: ${CLOUDFLARE_API_TOKEN}
    zone_id: ${CLOUDFLARE_ZONE_ID}
  ttl: 60  # 秒
  proxy: false  # Cloudflare Proxyを使わない
```

**デプロイ:**
```bash
# 独立したサービスとして起動
docker run -d \
  --name kokoa-dns-manager \
  -e DNS_PROVIDER=cloudflare \
  -e CLOUDFLARE_API_TOKEN=xxx \
  kokoaproxy/dns-manager:latest
```

---


### 2. Edge Provisioner Module

**役割:**
Edge Nodeの自動作成・削除

**プロバイダー実装:**
- DigitalOcean
- Vultr
- Hetzner
- AWS EC2
- Manual（既存VPSを手動登録）

**API Interface:**

```go
type EdgeProvisioner interface {
    // VPS作成
    CreateInstance(config InstanceConfig) (Instance, error)

    // VPS削除
    DestroyInstance(instanceID string) error

    // VPSステータス取得
    GetInstanceStatus(instanceID string) (InstanceStatus, error)

    // SSH接続情報取得
    GetSSHConfig(instanceID string) (SSHConfig, error)
}

type InstanceConfig struct {
    Provider   string // "digitalocean", "vultr"
    Region     string // "sgp1", "nrt"
    Size       string // "s-1vcpu-1gb"
    Image      string // カスタムイメージID（WireGuard等プリインストール）
    SSHKeyID   string
    UserData   string // cloud-init script
}
```

**設定例:**

```yaml
# config.yaml
edge_provisioner:
  provider: digitalocean
  digitalocean:
    api_token: ${DO_API_TOKEN}
    default_region: sgp1
    default_size: s-1vcpu-1gb
    ssh_key_id: ${DO_SSH_KEY_ID}
  auto_provision: true  # 自動作成を有効化
  min_nodes: 2
  max_nodes: 10
```

**デプロイ:**
```bash
docker run -d \
  --name kokoa-edge-provisioner \
  -e PROVIDER=digitalocean \
  -e DO_API_TOKEN=xxx \
  kokoaproxy/edge-provisioner:latest
```

---


### 3. SSL Manager Module

**役割:**
SSL/TLS証明書の自動取得・更新・配布

**プロバイダー実装:**
- Let's Encrypt (ACME)
- Cloudflare Origin Certificate
- ZeroSSL
- Manual（手動アップロード）

**API Interface:**

```go
type SSLManager interface {
    // 証明書取得
    ObtainCertificate(domains []string, method string) (Certificate, error)

    // 証明書更新
    RenewCertificate(certID string) (Certificate, error)

    // 証明書配布（Edge Nodeへ）
    DistributeCertificate(certID string, nodeIDs []string) error

    // 証明書状態確認
    GetCertificateStatus(certID string) (CertStatus, error)
}

type Certificate struct {
    ID          string
    Domains     []string
    Issuer      string
    ExpiresAt   time.Time
    Certificate []byte
    PrivateKey  []byte
}
```

**設定例:**

```yaml
# config.yaml
ssl:
  provider: letsencrypt
  letsencrypt:
    email: admin@example.com
    challenge_method: http-01 # or dns-01
  auto_renew: true
  renew_days_before: 30
```

**デプロイ:**
```bash
docker run -d \
  --name kokoa-ssl-manager \
  -e SSL_PROVIDER=letsencrypt \
  -e ACME_EMAIL=admin@example.com \
  kokoaproxy/ssl-manager:latest
```

---


### 4. Health Checker Module

**役割:**
Edge Node・Originのヘルスチェック

**チェック方法:**
- HTTP/HTTPS (status code check)
- TCP (port check)
- ICMP (ping)
- Custom script

---

### ヘルスチェック対象の詳細

#### Edge Node のヘルスチェック

**アクティブチェック（Control Planeから定期実行）:**

1. **HTTPエンドポイントチェック**
   ```
   GET https://<edge_node_ip>/health
   期待結果: 200 OK
   頻度: 10秒ごと
   タイムアウト: 5秒
   ```

2. **WireGuardトンネルチェック**
   ```
   Ping 10.0.X.10 (Edge NodeのWireGuard IP)
   期待結果: ICMP Echo Reply
   頻度: 10秒ごと
   タイムアウト: 3秒
   ```

3. **SSL証明書有効性チェック**
   ```
   openssl s_client -connect <edge_node_ip>:443 -servername <hostname>
   期待結果: 証明書が有効（期限切れでない, ホスト名が一致）
   頻度: 1日1回
   ```

4. **バックエンド接続性チェック**
   ```
   Edge Node経由でOriginへの接続をテスト
   GET https://<hostname>/health (via Edge)
   期待結果: 200 OK（Originからの応答）
   頻度: 30秒ごと
   ```

**パッシブチェック（Edge Nodeからハートビート）:**

```go
// Edge Node → Control Plane (5秒ごと)
type EdgeHeartbeat struct {
    NodeID           string
    Status           string  // "healthy", "degraded", "unhealthy"
    TunnelStatus     string  // "up", "down"
    ActiveConnections int
    CPUPercent       float64
    MemoryPercent    float64
    TunnelRTT_ms     int     // Originへのping遅延
    RequestsPerSecond float64
    ErrorRate        float64 // 5xx errors / total requests
}
```

**異常判定基準:**

| 条件 | アクション |
|------|----------|
| HTTPエンドポイント 3回連続失敗 | DNS除外 → UNHEALTHY状態 |
| WireGuardトンネルダウン | 即座にDNS除外 |
| CPU > 90% が5分継続 | アラート送信（まだDNS除外しない）|
| ErrorRate > 10% | アラート送信 + 調査 |
| ハートビート 30秒途絶 | DNS除外 → UNHEALTHY状態 |

---

#### Origin のヘルスチェック

**アクティブチェック（Control Planeから定期実行）:**

1. **WireGuardトンネルチェック**
   ```
   Ping 10.0.X.1 (OriginのWireGuard IP)
   期待結果: ICMP Echo Reply
   頻度: 10秒ごと
   ```

2. **アプリケーションヘルスチェック**
   ```
   GET http://10.0.X.1/health (WireGuard経経由)
   期待結果: 200 OK + 正常なレスポンスボディ
   頻度: 30秒ごと
   ```

3. **個別サービスチェック**
   ```
   各origin_serviceごとに設定されたヘルスチェック
   例: GET http://10.0.X.1:8001/api/health
   頻度: 30秒ごと
   ```

**パッシブチェック（OriginからハートビートWebSocket）:**

```go
// Origin → Control Plane (30秒ごと)
type OriginHeartbeat struct {
    OriginID     string
    Status       string  // "healthy", "degraded", "unhealthy"
    Services     []ServiceStatus
    CPUPercent   float64
    MemoryPercent float64
    DiskPercent  float64
    EdgeNodesConnected int  // 接続中のEdge Node数
}

type ServiceStatus struct {
    Name   string
    Port   int
    Status string  // "up", "down"
}
```

**異常判定基準:**

| 条件 | アクション |
|------|----------|
| WireGuardトンネルダウン | 即座に全Edge Nodeに通知 → 503エラーページ表示 |
| アプリケーション応答なし（3回連続） | 全Edge Nodeに通知 → 503エラーページ |
| 個別サービスダウン | そのサービスのみ503エラー、他は正常動作 |
| ハートビート 60秒途絶 | アラート送信 + 調査 |
| Disk > 90% | アラート送信（Critical） |

---

**API Interface:**

```go
type HealthChecker interface {
    // ヘルスチェック登録
    RegisterCheck(target string, config HealthCheckConfig) (string, error)

    // ヘルスチェック実行
    ExecuteCheck(checkID string) (HealthCheckResult, error)

    // ヘルスチェック削除
    RemoveCheck(checkID string) error

    // ステータス取得
    GetCheckStatus(checkID string) (CheckStatus, error)
}

type HealthCheckConfig struct {
    Type      string // "http", "tcp", "icmp", "wireguard", "ssl_cert"
    Interval  int    // 秒
    Timeout   int    // 秒
    Retries   int    // リトライ回数
    Threshold int    // 連続失敗回数（この回数に達したら異常判定）

    // HTTP/HTTPSチェック用
    URL            string
    ExpectedStatus int    // 期待するHTTPステータスコード（デフォルト: 200）
    ExpectedBody   string // 期待するレスポンスボディ（部分一致）

    // TCPチェック用
    Host string
    Port int

    // WireGuardチェック用
    WireGuardIP string
}
```

**設定例:**

```yaml
# config.yaml
health_checker:
  default_interval: 10  # 秒
  default_timeout: 5
  default_retries: 3
  default_threshold: 3
  workers: 10  # 並列実行数
```

**デプロイ:**
```bash
docker run -d \
  --name kokoa-health-checker \
  kokoaproxy/health-checker:latest
```

---

### 5. Metrics Collector Module（オプション）

**役割:**
メトリクス収集・保存・可視化

**機能:**
- トラフィック統計
- レスポンスタイム
- エラーレート
- リソース使用率

**技術スタック:**
- Prometheus（メトリクス保存）
- Grafana（可視化）
- または、シンプルなカスタム実装

**デプロイ:**
```bash
docker run -d \
  --name kokoa-metrics \
  kokoaproxy/metrics-collector:latest
```

---


### 6. Outbound Router Module（オプション）

**役割:**
Outbound Trafficのルーティング管理

**機能:**
- スプリットトンネル設定
- ロードバランシング
- フェイルオーバー

**デプロイ:**
```bash
docker run -d \
  --name kokoa-outbound-router \
  kokoaproxy/outbound-router:latest
```

---

## モジュール間通信

### イベント駆動アーキテクチャ

**Control Plane → Module:**

```json
// Edge Node追加イベント
{
  "event": "edge.created",
  "edge_node_id": "uuid",
  "public_ip": "203.0.113.10",
  "routes": [
    {
      "hostname": "app.example.com",
      "origin_id": "uuid"
    }
  ]
}
```

**処理フロー:**

```
1. Control Plane: Edge Node作成決定
   ↓
2. Edge Provisioner Module: VPS作成
   ↓ (VPS作成完了)
3. Edge Provisioner → Control Plane: edge.provisioned イベント
   ↓
4. Control Plane → DNS Manager: DNS追加リクエスト
   ↓
5. DNS Manager: Aレコード追加
   ↓ (DNS追加完了)
6. DNS Manager → Control Plane: dns.record.added イベント
   ↓
7. Control Plane → Health Checker: ヘルスチェック登録
   ↓
8. Health Checker: 定期チェック開始
   ↓
9. Control Plane → User: edge.ready イベント（WebSocket）
```

---

## 登場人物（コンポーネント）

### 1. コントロールプレーン（管理サーバー）

**役割:**
- Web UI/API提供
- ノード管理（エッジノード・オリジンの登録・削除）
- WireGuard鍵ペアの生成・管理
- 設定の一元管理
- ユーザー認証（SSO/2FA）
- ヘルスチェック・監視
- DNS自動更新

**配置:**
- 信頼できるVPS or 自宅サーバー
- DBが必要（SQLite/PostgreSQL）

**技術スタック例:**
- バックエンド: Go/Python/Node.js
- フロントエンド: React/Vue
- DB: PostgreSQL/SQLite
- 認証: OAuth2/OIDC

**主な機能:** 
```
- ノード登録・削除API
- 設定生成エンジン
- ヘルスチェッカー（5秒間隔）
- DNS更新API（Cloudflare/Route53）
- WebSocket（リアルタイム監視）
- 証明書管理（Let's Encrypt統合）
```

---


### 2. エッジノード（入口ノード）

**役割:**
- インターネット側の入口
- リバースプロキシ（Nginx/Traefik）
- WireGuard Client（オリジンに接続）
- SSL/TLS終端
- DDoS軽減（rate limiting）

**配置:**
-使い捨てVPS × 2-3台以上
- 異なるプロバイダー・リージョン推奨

**起動シーケエンス:**
```
1. コントロールプレーンに登録リクエスト（node_token使用）
2. WireGuard設定を受信（鍵、IP、Peer情報）
3. WireGuardトンネル確立
4. プロキシ設定を受信（upstream、SSL証明書）
5. Nginx/Traefik起動
6. ヘルスチェックエンドポイント公開
7. コントロールプレーンに稼働報告
```

**ライフサイクル:**
```
起動 → 登録 → 稼働 → 監視 → (攻撃検知) → 削除 → 破棄
```

---


### 3. オリジンサーバー（自宅サーバー）

**役割:**
- WireGuard Server（エッジノードを受け入れ）
- 実際のWebサービス/アプリケーション
- ロードバランサー（複数サービスの場合）
- コントロールプレーンとの通信（ハートビート）

**配置:**
- 自宅のプライベートネットワーク
- 固定IPや動的DNS不要（WireGuardはアウトバウンド接続も可）

**WireGuard設定例:**
```ini
[Interface]
Address = 10.0.0.1/24
PrivateKey = <origin_private_key>
ListenPort = 51820

# エッジノード自動追加（コントロールプレーン経由）
[Peer]
PublicKey = <edge_node_1_public_key>
AllowedIPs = 10.0.0.10/32

[Peer]
PublicKey = <edge_node_2_public_key>
AllowedIPs = 10.0.0.11/32
```

---


### 4. DNS管理

**役割:**
- エッジノードのIPをAレコードとして登録
- ヘルスチェックに基づく自動切り替え
- ラウンドロビン/GeoDNS

**実装オプション:**
- Cloudflare API経由
- PowerDNS + Lua script
- Route53 + Health Check

**レコード例:**
```
app.example.com    A    203.0.113.10  (Edge Node 1)
app.example.com    A    203.0.113.11  (Edge Node 2)
app.example.com    A    203.0.113.12  (Edge Node 3)
TTL: 30秒（フェイルオーバー高速化）
```

---

## システムアーキテクチャ図

```
┌─────────────────────────────────────────────────────┐
│             インターネットユーザー                      │
└─────────────┬───────────────────────────────────────┘
              │ DNS Query (Round Robin for app.example.com)
              │
    ┌─────────┴──────────┬──────────────┐
    │                    │              │
    v                    v              v
┌───────────┐      ┌───────────┐  ┌───────────┐
│Edge Node 1│      │Edge Node 2│  │Edge Node 3│
│(VPS #1)   │      │(VPS #2)   │  │(VPS #3)   │
│           │      │           │  │           │
│ Nginx     │      │ Nginx     │  │ Nginx     │
│ WG Client │      │ WG Client │  │ WG Client │
└─────┬─────┘      └─────┬─────┘  └─────┬─────┘
      │                  │              │
      │  WireGuard Tunnel (暗号化)      │
      │  10.0.0.10-12 → 10.0.0.1       │
      │                  │              │
      └──────────────────┼──────────────┘
                         │
                         v
              ┌──────────────────┐
              │ Origin Server    │
              │ (自宅サーバー)    │
              │                  │
              │ WG Server        │
              │ 10.0.0.1/24      │
              │ HAProxy/Nginx    │
              │ Web Services     │
              └────────┬─────────┘
                       │
              ┌────────┴─────────┐
              │                  │
              v                  v
         ┌─────────┐       ┌─────────┐
         │ App #1  │       │ App #2  │
         │ :8001   │       │ :8002   │
         └─────────┘       └─────────┘


              ┌──────────────────┐
              │ Control Plane    │
              │ (管理サーバー)    │
              │                  │
              │ Web UI/API       │
              │ Node Manager     │
              │ Config DB        │
              │ Health Checker   │
              │ DNS Updater      │
              └──────────────────┘
                       ▲
                       │ 管理・設定・監視
        ┌──────────────┼──────────────┐
        │              │              │
        v              v              v
   Edge Node 1   Edge Node 2   Edge Node 3
   Origin Server
```

---

## 通信フロー

### A. 初期セットアップフロー

```
1. 管理者 → Control Plane (Web UI)
   POST /api/v1/origins
   {
     "name": "home-server",
     "domain": "app.example.com",
     "services": [
       {"name": "web", "port": 8001},
       {"name": "api", "port": 8002}
     ]
   }

2. Control Plane
   - WireGuard鍵ペア生成
   - 設定ファイル生成
   - DBに保存

3. Control Plane → 管理者
   - origin-config.tar.gz ダウンロード
   - インストールスクリプト提供

4. 管理者 → Origin Server
   $ scp origin-config.tar.gz home:~
   $ ssh home
   $ sudo ./install-origin.sh

5. Origin Server起動
   - WireGuardインターフェース作成
   - Control Planeに接続開始（WebSocket）
   - ハートビート送信開始（30秒間隔）

6. 管理者 → Control Plane (Web UI)
   POST /api/edge-nodes
   {
     "provider": "digitalocean",
     "region": "sgp1",
     "size": "s-1vcpu-1gb",
     "ssh_key": "..."
   }

7. Control Plane → VPS Provider API
   - VPS作成
   - SSH鍵登録

8. Control Plane → Edge Node (SSH)
   - 初期セットアップスクリプト実行
   - WireGuard設定配布
   - Nginx設定配布
   - SSL証明書配布（または取得）

9. Edge Node起動
   - WireGuard起動（Originに接続）
   - Nginx起動
   - ヘルスチェックエンドポイント公開 (http://localhost/health)
   - Control Planeに登録完了報告

10. Control Plane → DNS Provider
    - 新しいエッジノードのIPをAレコードに追加
    - app.example.com A 203.0.113.10

11. Control Plane → 管理者 (Web UI通知)
    ✅ Edge Node 1 is now live!
```

---


### B. 通常運用時のフロー

```
【リクエスト処理】
1. ユーザー → DNS
   Query: app.example.com A?

2. DNS → ユーザー
   Response: 203.0.113.10, 203.0.113.11, 203.0.113.12
   (Round Robin)

3. ユーザー → Edge Node 1 (203.0.113.10)
   GET https://app.example.com/api/users
   Host: app.example.com

4. Edge Node 1 (Nginx)
   - SSL終端
   - プロキシヘッダー追加
   - WireGuardトンネル経由で転送

5. Edge Node 1 (10.0.0.10) → Origin (10.0.0.1)
   GET http://10.0.0.1:80/api/users
   X-Real-IP: 1.2.3.4
   X-Forwarded-For: 1.2.3.4
   X-Forwarded-Proto: https

6. Origin Server (HAProxy)
   - バックエンド選択（least_conn）
   - App #2 (port 8002)に転送

7. App #2 → Origin → Edge → ユーザー
   Response: 200 OK

【ヘルスチェック（並行動作）】
Edge Node 1, 2, 3 (every 5s)
  → Control Plane
  POST /api/heartbeat
  {
    "node_id": "edge-1",
    "status": "healthy",
    "connections": 42,
    "cpu": 12.3,
    "memory": 45.6,
    "tunnel_status": "up"
  }

Origin Server (every 30s)
  → Control Plane
  WebSocket message
  {
    "type": "heartbeat",
    "services": [
      {"name": "web", "status": "up"},
      {"name": "api", "status": "up"}
    ]
  }

Control Plane (every 10s)
  → Edge Node 1, 2, 3
  HTTP GET https://edge-node/health
  Expected: 200 OK + tunnel connectivity test
```

---

### C. ノード障害・攻撃時のフロー

```
【シナリオ: Edge Node 1がDDoS攻撃を受ける】

1. Edge Node 1
   - 異常なトラフィック検知
   - CPU 95%
   - rate limit超過多発

2. Control Plane (Health Checker)
   - Edge Node 1へのヘルスチェック失敗（3回連続）
   - タイムアウト or 500 エラー

3. Control Plane → DNS Provider API
   DELETE /zones/{zone_id}/dns_records/{edge1_record_id}
   - Edge Node 1のAレコード削除
   - TTL: 30秒 → 新しいクエリは即座に反映

4. Control Plane → 管理者 (通知)
   ⚠️ Edge Node 1 removed from DNS due to health check failure
   📊 Traffic now distributed to Edge Node 2 & 3

5. ユーザー → DNS (新規クエリ)
   Response: 203.0.113.11, 203.0.113.12 のみ
   - トラフィックが残りのノードに分散

6. 管理者判断 or 自動スクリプト
   → Control Plane (Web UI or API)
   DELETE /api/edge-nodes/edge-1

7. Control Plane → VPS Provider API
   DELETE /droplets/{edge1_droplet_id}
   - VPS完全削除（攻撃されたノード破棄）

8. 管理者 → Control Plane (Web UI)
   POST /api/edge-nodes
   {
     "provider": "vultr",  # プロバイダー変更
     "region": "nrt",      # リージョン変更
     ...
   }

9. 新規Edge Node起動 (5分以内)
   - 自動的にWireGuard設定受信
   - 自動的にNginx設定受信
   - トンネル確立
   - ヘルスチェック成功

10. Control Plane → DNS Provider
    POST /zones/{zone_id}/dns_records
    {
      "type": "A",
      "name": "app",
      "content": "198.51.100.20"  # 新ノードIP
    }

11. システム復旧完了
    ✅ 3ノード体制に復帰
    📊 トラフィック正常分散
    🛡️ 攻撃されたノードは完全削除済み
```

---

## VPS自動交換の完全なエンドツーエンドフロー

### シーケンス図（詳細版）

```
┌─────────────────────────────────────────────────────────────────┐
│ Phase 1: 障害検知・DNS除外（自動）                                │
└─────────────────────────────────────────────────────────────────┘

T+0s    Health Checker → Edge Node 1
        GET https://203.0.113.10/health
        → Timeout (5秒)

T+10s   Health Checker → Edge Node 1 (2回目)
        → Timeout

T+20s   Health Checker → Edge Node 1 (3回目)
        → Timeout
        ✗ 3回連続失敗 → 異常判定

T+21s   Health Checker → Control Plane
        Event: edge_node_unhealthy
        {
          "edge_node_id": "edge-1",
          "failure_count": 3,
          "last_error": "connection timeout"
        }

T+22s   Control Plane → DNS Manager Module
        Action: remove_dns_record
        {
          "edge_node_id": "edge-1",
          "record_id": "abc123"
        }

T+23s   DNS Manager → Cloudflare API
        DELETE /zones/{zone}/dns_records/abc123
        Response: 200 OK

T+24s   Control Plane → Database
        UPDATE edge_nodes SET status='unhealthy' WHERE id='edge-1'

T+25s   Control Plane → Alert System
        Send Notification:
        - Slack: "⚠️ Edge Node 1 (203.0.113.10) removed from DNS"
        - Email: admin@example.com
        - WebSocket: Web UI更新

┌─────────────────────────────────────────────────────────────────┐
│ Phase 2: VPS破棄（手動 or 自動）                                 │
└─────────────────────────────────────────────────────────────────┘

T+60s   管理者判断 (手動の場合)
        or
        Auto-Replace Policy (自動の場合)
          if config.auto_replace_unhealthy_nodes == true:
            trigger vps replacement

T+61s   Control Plane API
        DELETE /api/v1/edge-nodes/edge-1

T+62s   Control Plane → Edge Provisioner Module
        Action: destroy_instance
        {
          "edge_node_id": "edge-1",
          "provider_instance_id": "droplet-12345"
        }

T+63s   Edge Provisioner → DigitalOcean API
        DELETE /v2/droplets/12345
        Response: 204 No Content

T+64s   Control Plane → Database
        UPDATE edge_nodes SET status='deleted', deleted_at=NOW()
        WHERE id='edge-1'

┌─────────────────────────────────────────────────────────────────┐
│ Phase 3: 新VPS作成（自動）                                       │
└─────────────────────────────────────────────────────────────────┘

T+65s   Control Plane API (自動 or 手動トリガー)
        POST /api/v1/edge-nodes
        {
          "origin_id": "origin-a",
          "provider": "vultr",  # プロバイダー変更で攻撃回避
          "region": "nrt",
          "size": "vc2-1c-1gb"
        }

T+66s   Control Plane → Edge Provisioner Module
        Action: create_instance
        + WireGuard鍵ペア生成
        + WireGuard IP割り当て (10.0.1.13)

T+67s   Edge Provisioner → Vultr API
        POST /v2/instances
        {
          "region": "nrt",
          "plan": "vc2-1c-1gb",
          "os_id": 387,  # Ubuntu 24.04
          "user_data": "<cloud-init script>"
        }
        Response: 201 Created
        {
          "id": "instance-67890",
          "main_ip": "198.51.100.20"
        }

T+68s   Control Plane → Database
        INSERT INTO edge_nodes (
          id, origin_id, provider, public_ip, wireguard_ip, status
        ) VALUES (
          'edge-4', 'origin-a', 'vultr', '198.51.100.20',
          '10.0.1.13', 'provisioning'
        )

T+90s   VPS起動完了（約25秒）
        Cloud-init実行中...

┌─────────────────────────────────────────────────────────────────┐
│ Phase 4: 設定配布・サービス起動（自動）                           │
└─────────────────────────────────────────────────────────────────┘

T+91s   Cloud-init Script (VPS上で実行)
        #!/bin/bash
        # 1. 基本パッケージインストール
        apt-get update
        apt-get install -y wireguard nginx certbot

        # 2. Control Planeから設定取得
        EDGE_ID="edge-4"
        TOKEN="<node_registration_token>"
        curl -H "Authorization: Bearer $TOKEN" \
          https://control.example.com/api/v1/edge-nodes/$EDGE_ID/config \
          -o /tmp/edge-config.tar.gz

        # 3. 設定展開
        tar -xzf /tmp/edge-config.tar.gz -C /

        # 4. WireGuard起動
        systemctl enable wg-quick@wg-origin-a
        systemctl start wg-quick@wg-origin-a

        # 5. Nginx起動
        nginx -t && systemctl start nginx

        # 6. ヘルスチェックエンドポイント確認
        sleep 5
        curl http://localhost/health

        # 7. Control Planeに登録完了報告
        curl -X POST \
          https://control.example.com/api/v1/edge-nodes/$EDGE_ID/ready \
          -H "Authorization: Bearer $TOKEN"

T+180s  Edge Node 4起動完了（約90秒）

T+181s  Edge Node 4 → Control Plane
        POST /api/v1/heartbeat
        {
          "node_id": "edge-4",
          "status": "healthy",
          "tunnel_status": "up"
        }

T+182s  Control Plane → Database
        UPDATE edge_nodes SET status='active' WHERE id='edge-4'

┌─────────────────────────────────────────────────────────────────┐
│ Phase 5: SSL証明書取得（自動）                                   │
└─────────────────────────────────────────────────────────────────┘

T+183s  Control Plane → SSL Manager Module
        Action: obtain_certificate_for_node
        {
          "edge_node_id": "edge-4",
          "domains": ["app.example.com"]
        }

T+184s  SSL Manager → Edge Node 4 (SSH)
        certbot --nginx --non-interactive \
          -d app.example.com -m admin@example.com --agree-tos

T+240s  証明書取得完了（約60秒）

T+241s  Edge Node 4
        nginx -s reload  # 新証明書適用

┌─────────────────────────────────────────────────────────────────┐
│ Phase 6: ヘルスチェック・DNS追加（自動）                          │
└─────────────────────────────────────────────────────────────────┘

T+242s  Health Checker → Edge Node 4
        GET https://198.51.100.20/health
        Response: 200 OK
        ✓ ヘルスチェック成功

T+252s  Health Checker → Edge Node 4 (2回目)
        Response: 200 OK

T+262s  Health Checker → Edge Node 4 (3回目)
        Response: 200 OK
        ✓ 3回連続成功 → 正常判定

T+263s  Control Plane → DNS Manager Module
        Action: add_dns_record
        {
          "edge_node_id": "edge-4",
          "hostname": "app.example.com",
          "ip": "198.51.100.20",
          "type": "A",
          "ttl": 30
        }

T+264s  DNS Manager → Cloudflare API
        POST /zones/{zone}/dns_records
        {
          "type": "A",
          "name": "app",
          "content": "198.51.100.20",
          "ttl": 30
        }
        Response: 201 Created

T+265s  Control Plane → Alert System
        Send Notification:
        - Slack: "✅ Edge Node 4 (198.51.100.20) added to DNS"
        - Email: admin@example.com
        - WebSocket: Web UI更新

T+295s  DNS伝播完了（TTL 30秒経過）
        新しいユーザーは Edge Node 4 にも振り分けられる

┌─────────────────────────────────────────────────────────────────┐
│ 完了: システム復旧                                               │
└─────────────────────────────────────────────────────────────────┘

Total Time: 約5分 (T+0s → T+295s)

状態:
✅ Edge Node 2: Active (203.0.113.11)
✅ Edge Node 3: Active (203.0.113.12)
✅ Edge Node 4: Active (198.51.100.20) ← 新規追加
❌ Edge Node 1: Deleted (攻撃を受けたノード)

DNS:
app.example.com A 203.0.113.11
app.example.com A 203.0.113.12
app.example.com A 198.51.100.20

トラフィック分散: 33% / 33% / 33%
```

---

### 自動化レベルの設定

**config.yaml:**

```yaml
edge_node_management:
  # 自動DNS除外（常に有効）
  auto_remove_unhealthy_from_dns: true
  health_check_failure_threshold: 3
  health_check_interval: 10  # 秒

  # 自動VPS破棄（オプション）
  auto_destroy_unhealthy_vps: true
  destroy_delay: 60  # DNS除外から何秒後に破棄するか

  # 自動VPS交換（オプション）
  auto_replace_unhealthy_nodes: true  # ✅ これを true にすると完全自動化
  replace_delay: 5  # VPS破棄から何秒後に新規作成するか

  # 新VPS作成時の設定
  replacement_strategy:
    change_provider: true   # プロバイダーをローテーション
    change_region: true     # リージョンをローテーション
    preferred_providers: ["vultr", "digitalocean", "hetzner"]
    preferred_regions: ["nrt", "sgp", "fra"]

  # 最小ノード数の維持
  min_active_nodes: 2
  # この数を下回ると自動的に新ノード作成
```

--- 

## データモデル

### Control Plane DB Schema

```sql
-- オリジンサーバー
CREATE TABLE origins (
  id UUID PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  -- app.example.com のような完全なホスト名を格納
  domain VARCHAR(255) NOT NULL UNIQUE,
  wireguard_ip VARCHAR(15) NOT NULL,  -- 10.0.0.1
  wireguard_public_key TEXT NOT NULL,
  wireguard_private_key_encrypted TEXT NOT NULL,
  status VARCHAR(20) NOT NULL,  -- active, inactive
  last_heartbeat TIMESTAMP,
  created_at TIMESTAMP DEFAULT NOW()
);

-- オリジン内のサービス
CREATE TABLE origin_services (
  id UUID PRIMARY KEY,
  origin_id UUID REFERENCES origins(id),
  name VARCHAR(255) NOT NULL,
  port INTEGER NOT NULL,
  protocol VARCHAR(10) DEFAULT 'http',  -- http, https, tcp
  health_check_path VARCHAR(255),
  created_at TIMESTAMP DEFAULT NOW()
);

-- エッジノード
CREATE TABLE edge_nodes (
  id UUID PRIMARY KEY,
  origin_id UUID REFERENCES origins(id),
  name VARCHAR(255) NOT NULL,
  provider VARCHAR(50),  -- digitalocean, vultr, hetzner
  region VARCHAR(50),
  public_ip VARCHAR(45) NOT NULL,
  wireguard_ip VARCHAR(15) NOT NULL,  -- 10.0.0.10, .11, .12...
  wireguard_public_key TEXT NOT NULL,
  wireguard_private_key_encrypted TEXT NOT NULL,
  status VARCHAR(20) NOT NULL,  -- provisioning, active, unhealthy, deleted
  last_heartbeat TIMESTAMP,
  health_check_failures INTEGER DEFAULT 0,
  created_at TIMESTAMP DEFAULT NOW(),
  deleted_at TIMESTAMP
);

-- DNS レコード
CREATE TABLE dns_records (
  id UUID PRIMARY KEY,
  edge_node_id UUID REFERENCES edge_nodes(id),
  -- app.example.com のような完全なホスト名を格納
  hostname VARCHAR(255) NOT NULL,
  record_type VARCHAR(10) NOT NULL,  -- A, AAAA
  record_id VARCHAR(255),  -- Provider側のレコードID
  provider VARCHAR(50),  -- cloudflare, route53
  created_at TIMESTAMP DEFAULT NOW()
);

-- ヘルスチェック履歴
CREATE TABLE health_checks (
  id UUID PRIMARY KEY,
  edge_node_id UUID REFERENCES edge_nodes(id),
  status VARCHAR(20),  -- success, failure
  response_time_ms INTEGER,
  error_message TEXT,
  checked_at TIMESTAMP DEFAULT NOW()
);

-- ユーザー（管理者）
CREATE TABLE users (
  id UUID PRIMARY KEY,
  email VARCHAR(255) UNIQUE NOT NULL,
  password_hash TEXT,
  totp_secret TEXT,
  role VARCHAR(20) DEFAULT 'admin',
  created_at TIMESTAMP DEFAULT NOW()
);
```

---

## API設計

### Control Plane REST API

#### オリジン管理

```http
# オリジン作成
POST /api/v1/origins
Content-Type: application/json
Authorization: Bearer <token>

{
  "name": "home-server",
  "domain": "app.example.com",
  "services": [
    {"name": "web", "port": 8001, "protocol": "http"},
    {"name": "api", "port": 8002, "protocol": "http"}
  ]
}

Response: 201 Created
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "home-server",
  "domain": "app.example.com",
  "wireguard_ip": "10.0.1.1",
  "wireguard_public_key": "...",
  "config_download_url": "/api/v1/origins/{id}/config"
}

# オリジン設定ダウンロード
GET /api/v1/origins/{id}/config
Authorization: Bearer <token>

Response: 200 OK (application/gzip)
[origin-config.tar.gz]
  ├── wireguard/wg0.conf
  ├── nginx/nginx.conf
  ├── scripts/install.sh
  └── credentials.json

# オリジン一覧
GET /api/v1/origins
Authorization: Bearer <token>

Response: 200 OK
{
  "origins": [
    {
      "id": "...",
      "name": "home-server",
      "domain": "app.example.com",
      "status": "active",
      "edge_nodes_count": 3,
      "last_heartbeat": "2025-01-21T10:30:00Z"
    }
  ]
}
```

#### エッジノード管理

```http
# エッジノード作成
POST /api/v1/edge-nodes
Content-Type: application/json
Authorization: Bearer <token>

{
  "origin_id": "550e8400-e29b-41d4-a716-446655440000",
  "provider": "digitalocean",
  "region": "sgp1",
  "size": "s-1vcpu-1gb",
  "ssh_key": "..."
}

Response: 202 Accepted
{
  "id": "660e8400-e29b-41d4-a716-446655440001",
  "status": "provisioning",
  "estimated_time_seconds": 120
}

# エッジノード一覧
GET /api/v1/origins/{origin_id}/edge-nodes
Authorization: Bearer <token>

Response: 200 OK
{
  "edge_nodes": [
    {
      "id": "...",
      "name": "edge-sgp1-01",
      "provider": "digitalocean",
      "region": "sgp1",
      "public_ip": "203.0.113.10",
      "status": "active",
      "health": "healthy",
      "uptime_seconds": 86400,
      "last_heartbeat": "2025-01-21T10:29:55Z"
    }
  ]
}

# エッジノード削除
DELETE /api/v1/edge-nodes/{id}
Authorization: Bearer <token>

Response: 202 Accepted
{
  "message": "Edge node deletion initiated",
  "steps": [
    "Remove from DNS",
    "Wait for TTL expiration (30s)",
    "Destroy VPS"
  ]
}
```

#### ヘルスチェック・監視

```http
# エッジノードからのハートビート
POST /api/v1/heartbeat
Content-Type: application/json
Authorization: Bearer <node_token>

{
  "node_id": "660e8400-e29b-41d4-a716-446655440001",
  "status": "healthy",
  "metrics": {
    "connections_active": 42,
    "cpu_percent": 12.3,
    "memory_percent": 45.6,
    "tunnel_rtt_ms": 23
  }
}

Response: 200 OK
{
  "message": "Heartbeat received",
  "next_check_in": 5
}

# ヘルスチェック履歴取得
GET /api/v1/edge-nodes/{id}/health-history?limit=100
Authorization: Bearer <token>

Response: 200 OK
{
  "checks": [
    {
      "timestamp": "2025-01-21T10:30:00Z",
      "status": "success",
      "response_time_ms": 45
    },
    {
      "timestamp": "2025-01-21T10:29:55Z",
      "status": "success",
      "response_time_ms": 42
    }
  ]
}
```

---

## 各コンポーネントの責務まとめ

| コンポーネント | 主な責務 | 状態管理 | 可用性要件 |
|---------------|---------|---------|-----------|
| **Control Plane** | ノード管理、設定配布、監視、DNS更新 | Stateful（DB必須） | 高（冗長化推奨）※下記参照 |
| **Edge Node** | リバースプロキシ、SSL終端、WG Client | Stateless（設定は受信） | 中（複数台で冗長） |
| **Origin Server** | WG Server、アプリホスティング | Stateful（アプリデータ） | 高（単一障害点） |
| **DNS** | レコード管理、ヘルスチェック | Stateless（API経由） | 高（プロバイダー依存） |

---

## Control Plane高可用性（HA）戦略

### なぜControl PlaneのHAが重要か

**Control Planeが停止した場合の影響:**

✅ **既存トラフィックへの影響: なし**
- Edge ↔ Origin間のWireGuardトンネルは継続動作
- ユーザーは通常通りサービスにアクセス可能

❌ **管理機能への影響: あり**
- 新しいEdge Nodeの追加不可
- 障害ノードの自動DNS除外停止
- 自動フェイルオーバー停止
- SSL証明書の自動更新停止
- Web UI/APIアクセス不可

**結論:**
Control Planeが停止しても**既存サービスは継続**するが、**障害対応や新規ノード追加ができなくなる**ため、HAが推奨されます。

---

### HA構成オプション

#### オプション1: Kubernetesデプロイ（最も堅牢）

**構成:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: control-plane
spec:
  replicas: 2  # 冗長化
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 0  # ゼロダウンタイム
      maxSurge: 1

---
apiVersion: v1
kind: Service
metadata:
  name: control-plane
spec:
  type: LoadBalancer  # または Ingress経由
  selector:
    app: control-plane
```

**メリット:**
- ✅ 自動的なヘルスチェック・再起動
- ✅ ローリングアップデート対応
- ✅ ノード障害時の自動リスケジュール
- ✅ 複数レプリカ間のロードバランシング

**デメリット:**
- ❌ K8sクラスタが必要（運用複雑度が高い）
- ❌ 小規模環境にはオーバースペック

---

#### オプション2: Docker Compose + 共有DB（推奨：中規模）

**構成:**

```yaml
version: '3.8'

services:
  # Control Plane Instance 1
  control-plane-1:
    image: kokoaproxy/control-plane:latest
    environment:
      NODE_ID: "control-1"
      DATABASE_URL: postgresql://db-primary:5432/kokoa  # 共有DB
      REDIS_URL: redis://redis-sentinel:6379
      LEADER_ELECTION: "true"  # リーダー選出有効
      HEALTH_CHECK_PORT: 8090
    ports:
      - "8080:8080"
    depends_on:
      - db-primary
      - redis-sentinel

  # Control Plane Instance 2
  control-plane-2:
    image: kokoaproxy/control-plane:latest
    environment:
      NODE_ID: "control-2"
      DATABASE_URL: postgresql://db-primary:5432/kokoa  # 同じDB
      REDIS_URL: redis://redis-sentinel:6379
      LEADER_ELECTION: "true"
      HEALTH_CHECK_PORT: 8090
    ports:
      - "8081:8080"
    depends_on:
      - db-primary
      - redis-sentinel

  # PostgreSQL Primary-Standby構成
  db-primary:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: kokoa
      POSTGRES_REPLICATION_MODE: master
    volumes:
      - db_data:/var/lib/postgresql/data

  db-standby:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: kokoa
      POSTGRES_REPLICATION_MODE: slave
      POSTGRES_MASTER_HOST: db-primary
    volumes:
      - db_standby_data:/var/lib/postgresql/data

  # Redis Sentinel（リーダー選出用）
  redis-sentinel:
    image: redis:7-alpine
    command: redis-sentinel /etc/redis/sentinel.conf

  # Nginx（ロードバランサー）
  nginx:
    image: nginx:alpine
    ports:
      - "443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf
    depends_on:
      - control-plane-1
      - control-plane-2

volumes:
  db_data:
  db_standby_data:
```

**nginx.conf:**
```nginx
upstream control_plane_backend {
    server control-plane-1:8080 max_fails=3 fail_timeout=10s;
    server control-plane-2:8080 max_fails=3 fail_timeout=10s backup;  # バックアップ
}

server {
    listen 443 ssl http2;
    server_name control.example.com;

    location / {
        proxy_pass http://control_plane_backend;
        proxy_next_upstream error timeout http_500 http_502 http_503;
    }
}
```

**リーダー選出ロジック:**
```go
// Control Planeアプリ内でRedis Lockを使用
func electLeader(redisClient *redis.Client, nodeID string) bool {
    key := "kokoa:leader"
    ttl := 10 * time.Second

    // Redisでリーダー選出（先着順）
    success, err := redisClient.SetNX(ctx, key, nodeID, ttl).Result()
    if err != nil || !success {
        return false  // 他のノードがリーダー
    }

    // リーダーとして動作
    go renewLeaderLock(redisClient, nodeID)
    return true
}

// リーダーのみが実行するタスク
if isLeader {
    - ヘルスチェック実行
    - DNS自動更新
    - VPS自動作成・削除
}

// 非リーダーは待機
if !isLeader {
    - Web UI/APIのみ提供
    - DBから読み取り専用
}
```

**メリット:**
- ✅ Docker Composeで比較的簡単に実装
- ✅ 一方が停止しても自動的に切り替わる
- ✅ 共有DBで状態を共有

**デメリット:**
- ⚠️ リーダー選出の実装が必要
- ⚠️ DBのHA構成が追加で必要

---

#### オプション3: アクティブ-パッシブ（推奨：小規模）

**構成:**

```yaml
version: '3.8'

services:
  # Primary Control Plane
  control-plane-primary:
    image: kokoaproxy/control-plane:latest
    environment:
      MODE: "active"
      DATABASE_URL: postgresql://db:5432/kokoa
    ports:
      - "8080:8080"
    restart: unless-stopped

  # Standby Control Plane（手動切り替え）
  control-plane-standby:
    image: kokoaproxy/control-plane:latest
    environment:
      MODE: "standby"  # 待機モード（管理機能無効）
      DATABASE_URL: postgresql://db:5432/kokoa
    ports:
      - "8081:8080"
    restart: unless-stopped
    profiles: 
      - standby  # デフォルトでは起動しない

  db:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: kokoa
    volumes:
      - db_data:/var/lib/postgresql/data
```

**切り替え手順:** 

1. Primaryが停止したことを検知
2. Standbyを起動:
   ```bash
   docker-compose --profile standby up -d control-plane-standby
   ```
3. Edge NodeとOriginの接続先を変更:
   ```bash
   # Edge/Originの設定ファイルを更新
   CONTROL_PLANE_URL=https://control-standby.example.com
   ```

**メリット:**
- ✅ 最もシンプル
- ✅ リーダー選出不要
- ✅ 小規模環境に適している

**デメリット:**
- ❌ 手動切り替えが必要
- ❌ 自動フェイルオーバーなし
- ❌ RPO/RTOが長い（数分〜数時間）

--- 

#### オプション4: 単一インスタンス + 高頻度バックアップ（最小構成）

**構成:**

```yaml
version: '3.8'

services:
  control-plane:
    image: kokoaproxy/control-plane:latest
    environment:
      DATABASE_URL: postgresql://db:5432/kokoa
      BACKUP_ENABLED: "true"
      BACKUP_INTERVAL: "300"  # 5分ごと
      BACKUP_S3_BUCKET: "s3://kokoa-backups/"
    ports:
      - "8080:8080"
    restart: unless-stopped

  db:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: kokoa
    volumes:
      - db_data:/var/lib/postgresql/data
```

**バックアップスクリプト:**
```bash
#!/bin/bash
# 5分ごとに実行
pg_dump -h db -U postgres kokoa | gzip > /backup/kokoa-$(date +%Y%m%d-%H%M%S).sql.gz

# S3にアップロード
aws s3 cp /backup/kokoa-*.sql.gz s3://kokoa-backups/
```

**災害復旧手順:**
1. 新しいVPSを作成
2. Docker Composeで再デプロイ
3. S3から最新バックアップを復元
4. DNS を新VPSに向ける

**RPO/RTO:**
- RPO: 5分（最大5分間のデータ損失）
- RTO: 15〜30分（手動復旧）

**メリット:**
- ✅ 最もシンプル
- ✅ 追加コストなし

**デメリット:**
- ❌ ダウンタイムあり（15〜30分）
- ❌ 手動復旧が必要

---

### 推奨構成まとめ

| 規模 | 推奨構成 | RPO | RTO | 複雑度 | コスト |
|------|---------|-----|-----|--------|--------|
| **大規模**（100+ Edge Nodes） | オプション1: Kubernetes | 0秒 | 数秒 | 高 | 高 |
| **中規模**（10〜100 Edge Nodes） | オプション2: Docker Compose + 共有DB | 0秒 | 数秒 | 中 | 中 |
| **小規模**（1〜10 Edge Nodes） | オプション3: アクティブ-パッシブ | 0秒 | 数分 | 低 | 低 |
| **最小構成**（開発・テスト） | オプション4: 単一 + バックアップ | 5分 | 15〜30分 | 最低 | 最低 |

**本プロジェクトの推奨:**
- 本番環境: **オプション2（Docker Compose + 共有DB）**
- 開発環境: **オプション4（単一 + バックアップ）**

---

## セキュリティ設計

### 1. 認証・認可

**Control Plane:**
- Web UI: OAuth2/OIDC + 2FA (TOTP)
- API: JWT Bearer Token
- Node Token: 長期トークン（ノード登録用、制限付き）

**エッジノードの認証:**
```
初回登録時:
1. 管理者がNode Tokenを発行（Web UI）
2. エッジノードはNode Tokenで登録
3. Control PlaneがNode専用のJWT発行
4. 以降のハートビートはJWT使用
```

### 2. 通信暗号化

```
ユーザー → Edge Node: HTTPS (TLS 1.3)
Edge Node → Origin: WireGuard (ChaCha20-Poly1305)
Control Plane ↔ Node: HTTPS (mutual TLS可)
```

### 3. WireGuard鍵管理

- 秘密鍵はDB内で暗号化保存（AES-256）
- 鍵ローテーション機能（手動/自動）
- 鍵配布はTLS暗号化通信のみ

### 4. DDoS対策

**Edge Node側:**
```nginx
# Nginx設定
limit_req_zone $binary_remote_addr zone=one:10m rate=10r/s;
limit_req zone=one burst=20 nodelay;
limit_conn_zone $binary_remote_addr zone=addr:10m;
limit_conn addr 10;
```

**Control Plane側:**
- ヘルスチェック失敗時の自動DNS除外
- 異常トラフィック検知（閾値設定）
- 自動ノード交換スクリプト

---


### 5. Origin間の通信分離（重要！）

**問題:**
複数のOriginが同じWireGuardネットワーク（例: 10.0.0.0/24）にいると、Origin同士が通信できてしまう。

```
❌ 危険な構成:
Origin A (10.0.0.1) ←→ Origin B (10.0.0.2)  # 直接通信可能！
```

**解決策1: 各Originごとに独立したネットワーク（推奨）**

各Originに専用のサブネットを割り当て、完全に分離します。

```
✅ 安全な構成:
Origin A: 10.0.1.0/24
  └─ Origin A自身: 10.0.1.1
  └─ Edge Node 1: 10.0.1.10
  └─ Edge Node 2: 10.0.1.11

Origin B: 10.0.2.0/24
  └─ Origin B自身: 10.0.2.1
  └─ Edge Node 1: 10.0.2.10
  └─ Edge Node 2: 10.0.2.11
```

**Origin Aの設定:**
```bash
# /etc/wireguard/wg0.conf
[Interface]
Address = 10.0.1.1/24
PrivateKey = <origin_a_private_key>
ListenPort = 51820

# Edge Node 1（Origin A専用IP）
[Peer]
PublicKey = <edge_node_1_public_key>
AllowedIPs = 10.0.1.10/32

# Edge Node 2（Origin A専用IP）
[Peer]
PublicKey = <edge_node_2_public_key>
AllowedIPs = 10.0.1.11/32
```

**Origin Bの設定:**
```bash
# /etc/wireguard/wg0.conf
[Interface]
Address = 10.0.2.1/24
PrivateKey = <origin_b_private_key>
ListenPort = 51820

# Edge Node 1（Origin B専用IP）
[Peer]
PublicKey = <edge_node_1_public_key>
AllowedIPs = 10.0.2.10/32

# Edge Node 2（Origin B専用IP）
[Peer]
PublicKey = <edge_node_2_public_key>
AllowedIPs = 10.0.2.11/32
```

**Edge Nodeの設定（複数WireGuardインターフェース）:**
```bash
# Edge Node 1

# Origin A用トンネル
# /etc/wireguard/wg-origin-a.conf
[Interface]
Address = 10.0.1.10/24
PrivateKey = <edge_private_key_a>
ListenPort = 51820

[Peer]
PublicKey = <origin_a_public_key>
Endpoint = origin-a.example.com:51820
AllowedIPs = 10.0.1.1/32
PersistentKeepalive = 25

# Origin B用トンネル
# /etc/wireguard/wg-origin-b.conf
[Interface]
Address = 10.0.2.10/24
PrivateKey = <edge_private_key_b>
ListenPort = 51821  # 異なるポート

[Peer]
PublicKey = <origin_b_public_key>
Endpoint = origin-b.example.com:51820
AllowedIPs = 10.0.2.1/32
PersistentKeepalive = 25
```

**Nginx設定（Origin別にupstream）:**
```nginx
# Origin A用
upstream origin_a {
    server 10.0.1.1:80;
}

server {
    listen 443 ssl;
    server_name app-a.example.com;

    location / {
        proxy_pass http://origin_a;
    }
}

# Origin B用
upstream origin_b {
    server 10.0.2.1:80;
}

server {
    listen 443 ssl;
    server_name app-b.example.com;

    location / {
        proxy_pass http://origin_b;
    }
}
```

---

**解決策2: AllowedIPsで厳密に制限**

同じネットワーク（10.0.0.0/24）でも、AllowedIPsで制限。

```bash
# Origin A (/etc/wireguard/wg0.conf)
[Interface]
Address = 10.0.0.1/24

# Edge Node 1からのトラフィックのみ許可
[Peer]
PublicKey = <edge_node_1_public_key>
AllowedIPs = 10.0.0.10/32  # Origin Bの10.0.0.2は含まれない

# Edge Node 2からのトラフィックのみ許可
[Peer]
PublicKey = <edge_node_2_public_key>
AllowedIPs = 10.0.0.11/32
```

**iptablesで追加制御:**
```bash
# Origin A側で、他のOriginへの通信をブロック
iptables -A OUTPUT -d 10.0.0.2/32 -j DROP  # Origin Bへの通信を拒否
iptables -A OUTPUT -d 10.0.0.3/32 -j DROP  # Origin Cへの通信を拒否
```

**⚠️ 注意:**
この方法は設定ミスのリスクがあるため、**解決策1（独立ネットワーク）を強く推奨**。

---

**解決策3: WireGuard以外の分離手段**

より複雑ですが、完全分離を保証：

```
各Originが個別のWireGuardサーバーを持ち、
Edge Nodeが複数のWireGuardクライアントを動かす。

Origin A ←→ wg0 ←→ Edge Node
Origin B ←→ wg1 ←→ Edge Node
Origin C ←→ wg2 ←→ Edge Node
```

---

**データモデルへの反映:**

```sql
-- origins テーブルに network_cidr を追加
ALTER TABLE origins ADD COLUMN network_cidr VARCHAR(18) NOT NULL DEFAULT '10.0.0.0/24';

-- 例:
-- Origin A: 10.0.1.0/24
-- Origin B: 10.0.2.0/24
-- Origin C: 10.0.3.0/24

-- IP割り当てロジック
-- Control Planeが自動的に未使用のサブネットを割り当て
```

**Control Planeの自動割り当て:**

```python
def allocate_origin_network():
    # 既存のOriginが使用しているサブネットを取得
    used_subnets = db.query("SELECT network_cidr FROM origins")

    # 10.0.1.0/24, 10.0.2.0/24, ... から未使用のものを探す
    for i in range(1, 255):
        candidate = f"10.0.{i}.0/24"
        if candidate not in used_subnets:
            return candidate

    raise Exception("No available subnets")

# 新しいOrigin作成時
new_origin = {
    "name": "origin-d",
    "network_cidr": allocate_origin_network(),  # "10.0.4.0/24"
    "wireguard_ip": "10.0.4.1",
}
```

---

**推奨構成まとめ:** 

| 項目 | 推奨 |
|------|------|
| **ネットワーク分離** | ✅ 各Originに独立したサブネット（10.0.X.0/24） |
| **Edge Node構成** | ✅ 複数WireGuardインターフェース（wg-origin-a, wg-origin-b...） |
| **AllowedIPs** | ✅ 厳密に制限（Origin自身のIPのみ） |
| **iptables** | ✅ 追加の防御層として設定 |
| **監視** | ✅ 異常な通信パターンを検知 |

これにより、**Origin同士が完全に分離**され、マルチテナント環境でも安全に運用できます。

---

## デプロイ戦略

### Phase 1: Minimal Viable Product (MVP)

**実装範囲:**
- ✅ Control Plane (最小限のWeb UI)
- ✅ 手動でのエッジノード登録
- ✅ WireGuard設定自動生成
- ✅ Nginx設定自動生成
- ✅ 基本的なヘルスチェック
- ✅ 手動DNS更新（CLIツール提供）

**スキップ:**
- ❌ VPS自動作成
- ❌ DNS自動更新
- ❌ SSO/2FA
- ❌ リアルタイム監視ダッシュボード

**期間:** 2-3週間

---


### Phase 2: 自動化

**追加機能:**
- ✅ VPS Provider API統合（DigitalOcean, Vultr）
- ✅ DNS自動更新（Cloudflare API）
- ✅ 自動フェイルオーバー
- ✅ ワンクリックノード追加・削除

**期間:** +2週間

---


### Phase 3: エンタープライズ機能

**追加機能:**
- ✅ SSO/2FA
- ✅ リアルタイムダッシュボード（WebSocket）
- ✅ ログ集約・検索
- ✅ トラフィック分析
- ✅ 複数オリジン対応
- ✅ チーム管理・権限制御

**期間:** +4週間

---

## Pangolinとの比較

| 機能 | Pangolin | 自作Kokoa Proxy |
|------|----------|----------------|
| コントロールプレーン | ✅ Pangolin Cloud（SaaS）<br>⚠️ セルフホストは単一ノードのみ | ✅ 完全セルフホスト<br>✅ 無制限エッジノード |
| 複数エッジノード | ❌ Remote NodeはCloud必須 | ✅ 無制限・完全制御 |
| 自動フェイルオーバー | ❌ | ✅ DNS統合 |
| VPS自動作成 | ❌ | ✅ Provider API統合 |
| Web UI | ✅ フル機能 | ✅ カスタマイズ可能 |
| WireGuard管理 | ✅ 自動 | ✅ 自動 |
| SSL証明書 | ✅ 自動（Let's Encrypt） | ✅ 自動（Certbot） |
| 料金 | 月$15〜（Cloud必須） | $0（セルフホスト）+ VPS代 |

---

## モジュラーデプロイ構成例

### 構成1: All-in-One（開発・小規模）

**docker-compose.yml:**

```yaml
version: '3.8'

services:
  # Control Plane Core
  control-plane:
    image: kokoaproxy/control-plane:latest
    ports:
      - "8080:8080"   # Web UI / API
      - "8081:8081"   # WebSocket
    environment:
      DATABASE_URL: postgresql://postgres:password@db:5432/kokoa
      REDIS_URL: redis://redis:6379
      # モジュール設定
      DNS_MANAGER_URL: http://dns-manager:3000
      EDGE_PROVISIONER_URL: http://edge-provisioner:3000
      SSL_MANAGER_URL: http://ssl-manager:3000
      HEALTH_CHECKER_URL: http://health-checker:3000
    depends_on:
      - db
      - redis
    volumes:
      - ./config:/etc/kokoa

  # Database
  db:
    image: postgres:15-alpine
    environment:
      POSTGRES_DB: kokoa
      POSTGRES_USER: postgres
      POSTGRES_PASSWORD: password
    volumes:
      - db_data:/var/lib/postgresql/data

  # Redis (Queue)
  redis:
    image: redis:7-alpine
    volumes:
      - redis_data:/data

  # DNS Manager Module
  dns-manager:
    image: kokoaproxy/dns-manager:latest
    environment:
      DNS_PROVIDER: cloudflare
      CLOUDFLARE_API_TOKEN: ${CLOUDFLARE_API_TOKEN}
      CLOUDFLARE_ZONE_ID: ${CLOUDFLARE_ZONE_ID}
      CONTROL_PLANE_URL: http://control-plane:8080

  # Edge Provisioner Module
  edge-provisioner:
    image: kokoaproxy/edge-provisioner:latest
    environment:
      PROVIDER: digitalocean
      DO_API_TOKEN: ${DO_API_TOKEN}
      DO_SSH_KEY_ID: ${DO_SSH_KEY_ID}
      CONTROL_PLANE_URL: http://control-plane:8080

  # SSL Manager Module
  ssl-manager:
    image: kokoaproxy/ssl-manager:latest
    environment:
      SSL_PROVIDER: letsencrypt
      ACME_EMAIL: ${ACME_EMAIL}
      CONTROL_PLANE_URL: http://control-plane:8080
    volumes:
      - ssl_certs:/etc/letsencrypt

  # Health Checker Module
  health-checker:
    image: kokoaproxy/health-checker:latest
    environment:
      CONTROL_PLANE_URL: http://control-plane:8080

volumes:
  db_data:
  redis_data:
  ssl_certs:
```

**.env:**

```bash
# Cloudflare (DNS)
CLOUDFLARE_API_TOKEN=your_cloudflare_token
CLOUDFLARE_ZONE_ID=your_zone_id

# DigitalOcean (VPS Provisioner)
DO_API_TOKEN=your_do_token
DO_SSH_KEY_ID=your_ssh_key_id

# SSL
ACME_EMAIL=admin@example.com
```

**起動:**

```bash
docker-compose up -d
```

**アクセス:**
- Web UI: http://localhost:8080
- API: http://localhost:8080/api/v1

---


### 構成2: 分散デプロイ（本番環境）

**Control Plane:**
```bash
# 専用VPS (例: 2GB RAM, 2 vCPU)
docker run -d \
  --name kokoa-control-plane \
  -p 443:8080 \
  -e DATABASE_URL=postgresql://... \
  -e DNS_MANAGER_URL=https://dns.internal.example.com \
  -e EDGE_PROVISIONER_URL=https://provisioner.internal.example.com \
  kokoaproxy/control-plane:latest
```

**DNS Manager (別サーバー):**
```bash
docker run -d \
  --name kokoa-dns-manager \
  -p 3000:3000 \
  -e DNS_PROVIDER=cloudflare \
  -e CLOUDFLARE_API_TOKEN=... \
  kokoaproxy/dns-manager:latest
```

**Edge Provisioner (別サーバー):**
```bash
docker run -d \
  --name kokoa-edge-provisioner \
  -p 3000:3000 \
  -e PROVIDER=digitalocean \
  -e DO_API_TOKEN=... \
  kokoaproxy/edge-provisioner:latest
```

---

### 構成3: Kubernetes デプロイ

**namespace.yaml:**

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: kokoa-system
```

**control-plane-deployment.yaml:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: control-plane
  namespace: kokoa-system
spec:
  replicas: 2  # 冗長化
  selector:
    matchLabels:
      app: control-plane
  template:
    metadata:
      labels:
        app: control-plane
    spec:
      containers:
      - name: control-plane
        image: kokoaproxy/control-plane:latest
        ports:
        - containerPort: 8080
        env:
        - name: DATABASE_URL
          valueFrom:
            secretKeyRef:
              name: kokoa-secrets
              key: database_url
        - name: DNS_MANAGER_URL
          value: "http://dns-manager.kokoa-system.svc:3000"
        - name: EDGE_PROVISIONER_URL
          value: "http://edge-provisioner.kokoa-system.svc:3000"

---
apiVersion: v1
kind: Service
metadata:
  name: control-plane
  namespace: kokoa-system
spec:
  selector:
    app: control-plane
  ports:
  - port: 80
    targetPort: 8080
  type: LoadBalancer
```

**modules-deployment.yaml:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dns-manager
  namespace: kokoa-system
spec:
  replicas: 1  # ⚠️ ステートフル（API Rate Limit共有のため）
  selector:
    matchLabels:
      app: dns-manager
  template:
    metadata:
      labels:
        app: dns-manager
    spec:
      containers:
      - name: dns-manager
        image: kokoaproxy/dns-manager:latest
        env:
        - name: DNS_PROVIDER
          value: "cloudflare"
        - name: CLOUDFLARE_API_TOKEN
          valueFrom:
            secretKeyRef:
              name: kokoa-secrets
              key: cloudflare_token

---
apiVersion: v1
kind: Service
metadata:
  name: dns-manager
  namespace: kokoa-system
spec:
  selector:
    app: dns-manager
  ports:
  - port: 3000
    targetPort: 3000

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: edge-provisioner
  namespace: kokoa-system
spec:
  replicas: 2  # ✅ ステートレス（スケール可能）
  selector:
    matchLabels:
      app: edge-provisioner
  template:
    metadata:
      labels:
        app: edge-provisioner
    spec:
      containers:
      - name: edge-provisioner
        image: kokoaproxy/edge-provisioner:latest
        env:
        - name: PROVIDER
          value: "digitalocean"
        - name: DO_API_TOKEN
          valueFrom:
            secretKeyRef:
              name: kokoa-secrets
              key: do_token
        resources:
          requests:
            cpu: 100m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 512Mi

---
apiVersion: v1
kind: Service
metadata:
  name: edge-provisioner
  namespace: kokoa-system
spec:
  selector:
    app: edge-provisioner
  ports:
  - port: 3000
    targetPort: 3000

---
# Horizontal Pod Autoscaler（オプション）
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: edge-provisioner-hpa
  namespace: kokoa-system
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: edge-provisioner
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
```

---

## モジュールのスケーラビリティ特性

### 各モジュールの分類

| モジュール | ステート | スケーラビリティ | 推奨Replicas | 理由 |
|-----------|----------|----------------|-------------|------|
| **DNS Manager** | ⚠️ 準ステートフル | 制限あり | 1〜2 | API Rate Limit、競合可能性 |
| **Edge Provisioner** | ✅ ステートレス | 水平スケール可 | 2〜10 | VPS作成は独立タスク |
| **SSL Manager** | ⚠️ 準ステートフル | 制限あり | 1〜2 | Let's Encrypt Rate Limit |
| **Health Checker** | ✅ ステートレス | 水平スケール可 | 2〜10 | 各インスタンスが独立チェック |
| **Metrics Collector** | ✅ ステートレス | 水平スケール可 | 2〜5 | データ集約のみ |
| **Outbound Router** | ✅ ステートレス | 水平スケール可 | 2〜10 | ルーティング判定のみ |

---

### 1. DNS Manager Module - スケーラビリティ戦略

**現状の問題:**
- Cloudflare API Rate Limit: 1200 req/5min
- 複数レプリカが同時に証明書取得すると重複リクエスト

**解決策1: 単一インスタンス + キューイング（推奨：小〜中規模）**

```go
// DNS Manager内部でキューを管理
type DNSManager struct {
    queue    chan DNSTask
    workers  int  // ワーカー数（並列度）
}

func (m *DNSManager) Start() {
    for i := 0; i < m.workers; i++ {
        go m.worker()
    }
}

func (m *DNSManager) worker() {
    for task := range m.queue {
        // Rate Limitを考慮してAPI呼び出し
        time.Sleep(rateLimit)
        m.executeTask(task)
    }
}
```

**メリット:**
- ✅ シンプル
- ✅ Rate Limit管理が容易
- ✅ 競合なし

**デメリット:**
- ❌ 単一障害点

---

**解決策2: 複数レプリカ + 分散ロック（推奨：大規模）**

```go
// Redis Lockを使用して排他制御
func (m *DNSManager) AddRecord(hostname, ip string) error {
    // ロック取得（TTL: 30秒）
    lock := redis.Lock("dns:add:"+hostname, 30*time.Second)
    if !lock.Acquire() {
        return errors.New("Another instance is processing")
    }
    defer lock.Release()

    // API呼び出し
    return m.provider.AddRecord(hostname, ip)
}
```

**Kubernetes設定:**
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: dns-manager
spec:
  replicas: 2  # 冗長化
  strategy:
    type: RollingUpdate
```

**メリット:**
- ✅ 高可用性
- ✅ ローリングアップデート可能

**デメリット:**
- ⚠️ やや複雑
- ⚠️ Redis依存

---


### 2. Edge Provisioner Module - スケーラビリティ戦略

**特性:**
- ✅ 完全ステートレス
- ✅ 各VPS作成は独立タスク
- ✅ 水平スケール容易

**推奨構成:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: edge-provisioner
spec:
  replicas: 3  # 通常時
  strategy:
    type: RollingUpdate

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: edge-provisioner-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: edge-provisioner
  minReplicas: 3
  maxReplicas: 20
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Pods
    pods:
      metric:
        name: vps_provision_queue_length
      target:
        type: AverageValue
        averageValue: "10"  # キューが10を超えたらスケール
```

**高負荷時のシナリオ:**

```
通常時:
- 3 replicas
- 処理能力: 3 VPS/min

大規模攻撃時（10台のVPS交換が必要）:
- HPA が自動的にスケール
- → 10 replicas に増加
- 処理能力: 10 VPS/min
- 約2分で全VPS交換完了

攻撃終息後:
- HPA が自動的にスケールダウン
- → 3 replicas に戻る
```

---


### 3. SSL Manager Module - スケーラビリティ戦略

**現状の問題:**
- Let's Encrypt Rate Limit: 50 certs/week/domain
- 複数レプリカが同時に証明書取得すると重複リクエスト

**推奨構成:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: ssl-manager
spec:
  replicas: 2  # アクティブ-パッシブ
  strategy:
    type: Recreate  # 同時起動を避ける
```

**リーダー選出:**
```go
// SSL Managerはリーダーのみが証明書取得
func (m *SSLManager) Start() {
    if m.electLeader() {
        go m.runCertificateRenewer()
        go m.runNewCertificateProcessor()
    } else {
        log.Info("Standby mode, waiting for leader failure")
        m.waitForLeaderFailure()
    }
}
```

---


### 4. Health Checker Module - スケーラビリティ戦略

**特性:**
- ✅ 完全ステートレス
- ✅ 各チェックは独立
- ✅ 負荷分散容易

**推奨構成:**

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: health-checker
spec:
  replicas: 5  # 通常時
  strategy:
    type: RollingUpdate

---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: health-checker-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: health-checker
  minReplicas: 5
  maxReplicas: 50
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 60
```

**チェック分散戦略:**

```go
// Consistent Hashingでチェック対象を分散
func (h *HealthChecker) shouldCheckNode(nodeID string) bool {
    hash := consistentHash(nodeID)
    mySlot := h.instanceID % h.totalInstances
    targetSlot := hash % h.totalInstances
    return mySlot == targetSlot
}
```

**スケーラビリティ例:**

```
100 Edge Nodes の場合:
- 5 replicas → 各インスタンスが 20 nodes をチェック
- 10秒間隔 → 各インスタンスは 2 req/s

1000 Edge Nodes の場合（自動スケール）:
- HPA が 25 replicas に増加
- 各インスタンスが 40 nodes をチェック
- 10秒間隔 → 各インスタンスは 4 req/s
```

---


### 推奨構成まとめ

**小規模（1〜10 Edge Nodes）:**
```yaml
dns-manager: 1 replica
edge-provisioner: 1 replica
ssl-manager: 1 replica
health-checker: 1 replica
```

**中規模（10〜100 Edge Nodes）:**
```yaml
dns-manager: 2 replicas（冗長化）
edge-provisioner: 3 replicas + HPA (max 10)
ssl-manager: 2 replicas（アクティブ-パッシブ）
health-checker: 3 replicas + HPA (max 20)
```

**大規模（100+ Edge Nodes）:**
```yaml
dns-manager: 2 replicas + 分散ロック
edge-provisioner: 5 replicas + HPA (max 50)
ssl-manager: 2 replicas + 分散ロック
health-checker: 10 replicas + HPA (max 100)
```

**デプロイ:**

```bash
kubectl apply -f namespace.yaml
kubectl apply -f control-plane-deployment.yaml
kubectl apply -f modules-deployment.yaml
```

---

## モジュールの有効/無効化

### config.yaml

```yaml
# Control Plane設定
control_plane:
  listen: "0.0.0.0:8080"
  database: "postgresql://..."

# モジュール設定
modules:
  dns_manager:
    enabled: true
    url: "http://dns-manager:3000"
    provider: "cloudflare"

  edge_provisioner:
    enabled: true
    url: "http://edge-provisioner:3000"
    provider: "digitalocean"

  ssl_manager:
    enabled: true
    url: "http://ssl-manager:3000"
    provider: "letsencrypt"

  health_checker:
    enabled: true
    url: "http://health-checker:3000"

  metrics_collector:
    enabled: false  # 無効化

  outbound_router:
    enabled: false  # 無効化
```

### モジュールの動的な有効/無効化

**Web UIから:**

```
Settings > Modules

┌─────────────────────────────────────────────┐
│ Module Configuration                        │
├─────────────────────────────────────────────┤
│                                             │
│ DNS Manager                  [✓] Enabled   │
│   Provider: Cloudflare                      │
│   Status: ✓ Connected                       │
│   [Configure]                               │
│                                             │
│ Edge Provisioner             [✓] Enabled   │
│   Provider: DigitalOcean                    │
│   Status: ✓ Connected                       │
│   [Configure]                               │
│                                             │
│ SSL Manager                  [✓] Enabled   │
│   Provider: Let's Encrypt                   │
│   Status: ✓ Connected                       │
│   [Configure]                               │
│                                             │
│ Health Checker               [✓] Enabled   │
│   Status: ✓ Running (10 checks)             │
│   [Configure]                               │
│                                             │
│ Metrics Collector            [ ] Disabled  │
│   [Enable]                                  │
│                                             │
│ Outbound Router              [ ] Disabled  │
│   [Enable]                                  │
│                                             │
└─────────────────────────────────────────────┘
```

---

## モジュール開発ガイド

### カスタムモジュールの作成

**例: カスタムDNS Provider実装**

```go
// custom_dns_provider.go
package main

import (
    "github.com/kokoa-proxy/module-sdk"
)

type CustomDNSProvider struct {
    apiKey string
}

func (p *CustomDNSProvider) AddRecord(hostname, recordType, value string) error {
    // カスタムDNS APIを呼び出す
    return nil
}

func (p *CustomDNSProvider) RemoveRecord(hostname, recordID string) error {
    return nil
}

func (p *CustomDNSProvider) ListRecords(hostname string) ([]DNSRecord, error) {
    return nil, nil
}

func main() {
    provider := &CustomDNSProvider{
        apiKey: os.Getenv("CUSTOM_DNS_API_KEY"),
    }

    // モジュールSDKで起動
    module.Serve(module.Config{
        Name:     "custom-dns",
        Version:  "1.0.0",
        Provider: provider,
    })
}
```

**Dockerfile:**

```dockerfile
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY . .
RUN go build -o custom-dns-manager

FROM alpine:3.19
COPY --from=builder /app/custom-dns-manager /usr/local/bin/
ENTRYPOINT ["/usr/local/bin/custom-dns-manager"]
```

**使用:**

```yaml
# config.yaml
modules:
  dns_manager:
    enabled: true
    url: "http://custom-dns-manager:3000"
    provider: "custom"
```

---

## モジュール間通信の詳細

### イベントバス（Redis Pub/Sub）

**Control Plane → Module (イベント発行):**

```go
// Control Plane
redis.Publish("kokoa.events.edge.created", json.Marshal(Event{
    Type: "edge.created",
    Data: map[string]interface{}{
        "edge_node_id": "uuid",
        "public_ip": "203.0.113.10",
    },
}))
```

**Module → Control Plane (イベント購読):**

```go
// DNS Manager Module
sub := redis.Subscribe("kokoa.events.edge.created")

for msg := range sub.Channel() {
    var event Event
    json.Unmarshal([]byte(msg.Payload), &event)

    // DNS レコード追加
    err := dnsProvider.AddRecord(
        event.Data["hostname"],
        "A",
        event.Data["public_ip"],
    )

    // 結果をControl Planeに報告
    redis.Publish("kokoa.events.dns.record.added", ...)
}
```

### REST API（モジュール → Control Plane）

```go
// Module側
resp, err := http.Post(
    controlPlaneURL + "/api/v1/internal/events",
    "application/json",
    json.Marshal(Event{
        Type: "dns.record.added",
        Data: map[string]interface{}{
            "record_id": "xyz",
            "hostname": "app.example.com",
        },
    }),
)
```

---

## 次のステップ

1. **技術スタック決定**
   - バックエンド言語: Go / Python / Node.js?
   - フロントエンド: React / Vue / Svelte?
   - DB: PostgreSQL / SQLite?

2. **プロトタイプ実装**
   - Phase 1のMVPから開始
   - Control Planeの基本機能
   - WireGuard設定生成

3. **テスト環境構築**
   - Control Plane: ローカル or 小型VPS
   - Edge Node: Oracle Free Tier × 2
   - Origin: ローカルDocker

どの部分から実装を始めますか?

```