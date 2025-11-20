# Kokoa Proxy - モジュラーアーキテクチャ設計

## 概要

Cloudflare Tunnelのような体験を、モジュラーかつセルフホストで実現する高可用性トンネリングプロキシシステム。

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
- EdgeとOriginを管理
- モジュール群を含む（DNS管理、VPS作成、SSL管理など）

**配置:** 信頼できるVPS or 自宅
**役割:** すべての指示を出す司令塔

---

### 2. Edge Node（エッジノード）
**何者？** インターネット側の入口（使い捨てVPS）
- ユーザーからのリクエストを受け付ける
- WireGuardでOriginに転送
- SSL終端、DDoS対策

**配置:** 複数のVPS（DigitalOcean、Vultr等）
**役割:** 防弾盾、攻撃を受けたら即破棄

---

### 3. Origin（オリジン/各ホスト）
**何者？** 実際のサービスが動いているホスト
- 自宅サーバー、VM、K8s、Docker等
- WireGuardでEdgeからの接続を受け入れ
- 実際のアプリケーションを実行

**配置:** 自宅、プライベートネットワーク
**役割:** 保護されたバックエンド

---

## アーキテクチャ概要図

```
        ┌──────────────────────────────────────┐
        │         インターネットユーザー          │
        └────────────────┬─────────────────────┘
                         │ HTTPS
              ┌──────────┴──────────┐
              │                     │
              v                     v
    ┌─────────────────┐   ┌─────────────────┐
    │  Edge Node #1   │   │  Edge Node #2   │  ← 登場人物②
    │  (VPS/使い捨て)  │   │  (VPS/使い捨て)  │
    │  - Nginx        │   │  - Nginx        │
    │  - WG Client    │   │  - WG Client    │
    └────────┬────────┘   └────────┬────────┘
             │ WireGuard Tunnel    │
             │  (暗号化)            │
             └─────────┬────────────┘
                       │
                       v
            ┌──────────────────────┐
            │   Origin (自宅等)     │  ← 登場人物③
            │  - WG Server         │
            │  - K8s/Docker/VM     │
            │  - 実際のアプリ       │
            └──────────────────────┘


    ┌──────────────────────────────────────────┐
    │      Control Plane (管理サーバー)         │  ← 登場人物①
    │  ┌────────────────────────────────────┐  │
    │  │  Web UI / API                      │  │
    │  └────────────────────────────────────┘  │
    │  ┌────────────────────────────────────┐  │
    │  │  モジュール群（Control Planeの一部）│  │
    │  │  ├─ DNS Manager                    │  │
    │  │  ├─ Edge Provisioner               │  │
    │  │  ├─ SSL Manager                    │  │
    │  │  └─ Health Checker                 │  │
    │  └────────────────────────────────────┘  │
    │  ┌────────────────────────────────────┐  │
    │  │  Database                          │  │
    │  └────────────────────────────────────┘  │
    └──────────────────────────────────────────┘
              │           │           │
              v           v           v
         Edge管理   Origin管理    DNS/VPS API
```

**重要:** モジュール（DNS Manager等）は独立した「登場人物」ではなく、Control Planeの機能を分離したもの。Control Planeの一部として動作します。

---

## トラフィックフロー（シンプル版）

```
【Inbound（外部→サービス）】
ユーザー
  → Edge Node（VPS、203.0.113.10）
    → WireGuardトンネル
      → Origin（自宅、10.0.0.1）
        → アプリ（ポート3000）

【Outbound（サービス→外部）】※オプション
アプリ（ポート3000）
  → Origin（10.0.0.1）
    → WireGuardトンネル
      → Edge Node（10.0.0.10）
        → インターネット（203.0.113.10から出る）

【管理】
管理者
  → Control Plane（Web UI）
    → Edge Node作成指示
    → Origin登録
    → ルーティング設定
```

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
    challenge_method: dns-01  # dns-01, http-01
    dns_provider: cloudflare  # DNS-01の場合
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
    Type      string // "http", "tcp", "icmp"
    Interval  int    // 秒
    Timeout   int    // 秒
    Retries   int
    Threshold int    // 連続失敗回数
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
- 使い捨てVPS × 2-3台以上
- 異なるプロバイダー・リージョン推奨

**起動シーケンス:**
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
example.com    A    203.0.113.10  (Edge Node 1)
example.com    A    203.0.113.11  (Edge Node 2)
example.com    A    203.0.113.12  (Edge Node 3)
TTL: 30秒（フェイルオーバー高速化）
```

---

## システムアーキテクチャ図

```
┌─────────────────────────────────────────────────────┐
│             インターネットユーザー                      │
└─────────────┬───────────────────────────────────────┘
              │ DNS Query (Round Robin)
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
   POST /api/origins
   {
     "name": "home-server",
     "domain": "example.com",
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
    - example.com A 203.0.113.10

11. Control Plane → 管理者 (Web UI通知)
    ✅ Edge Node 1 is now live!
```

---

### B. 通常運用時のフロー

```
【リクエスト処理】
1. ユーザー → DNS
   Query: example.com A?

2. DNS → ユーザー
   Response: 203.0.113.10, 203.0.113.11, 203.0.113.12
   (Round Robin)

3. ユーザー → Edge Node 1 (203.0.113.10)
   GET https://example.com/api/users
   Host: example.com

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
      "name": "@",
      "content": "198.51.100.20"  # 新ノードIP
    }

11. システム復旧完了
    ✅ 3ノード体制に復帰
    📊 トラフィック正常分散
    🛡️ 攻撃されたノードは完全削除済み
```

---

## データモデル

### Control Plane DB Schema

```sql
-- オリジンサーバー
CREATE TABLE origins (
  id UUID PRIMARY KEY,
  name VARCHAR(255) NOT NULL,
  domain VARCHAR(255) NOT NULL,
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
  domain VARCHAR(255) NOT NULL,
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
  "domain": "example.com",
  "services": [
    {"name": "web", "port": 8001, "protocol": "http"},
    {"name": "api", "port": 8002, "protocol": "http"}
  ]
}

Response: 201 Created
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "name": "home-server",
  "domain": "example.com",
  "wireguard_ip": "10.0.0.1",
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
      "domain": "example.com",
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
| **Control Plane** | ノード管理、設定配布、監視、DNS更新 | Stateful（DB必須） | 高（冗長化推奨） |
| **Edge Node** | リバースプロキシ、SSL終端、WG Client | Stateless（設定は受信） | 中（複数台で冗長） |
| **Origin Server** | WG Server、アプリホスティング | Stateful（アプリデータ） | 高（単一障害点） |
| **DNS** | レコード管理、ヘルスチェック | Stateless（API経由） | 高（プロバイダー依存） |

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
|-----|----------|----------------|
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
  replicas: 1
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
  replicas: 1
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
