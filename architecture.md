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
(以降のHA戦略、セキュリティ設計などのセクションの基本思想は維持されますが、実装の細部は上記マルチドメイン設計に基づきます。)