# Pangolin型トンネリングプロキシのアーキテクチャ

## 概要

Pangolinのような、コントロールプレーンで管理される複数エッジノードによる高可用性トンネリングプロキシシステムのアーキテクチャ設計。

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
