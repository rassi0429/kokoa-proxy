# MVP（Minimal Viable Product）定義の詳細化

`architecture.md`で定義されたMVPフェーズを、より具体的で実行可能なステップに掘り下げて定義します。

## 1. 主要な登場人物

このプロジェクトには、以下の3つの主要なコンポーネントがあります。

### 1.1. Control Plane（コントロールプレーン）
- **役割**: システム全体の管理、オーケストレーション、設定の生成と配布、監視を行う司令塔。ユーザーからのリクエストは直接処理せず、データプレーンには関与しません。
- **MVPでの実装範囲**: Web UI/APIの提供、Origin/Edge Nodeの登録管理、WireGuard/Nginx設定生成、基本的なヘルスチェック、手動DNS更新CLIツールとの連携。

### 1.2. Edge Node（エッジノード）
- **役割**: インターネットからのユーザーリクエストを受け付ける最前線のノード。WireGuardトンネルを通じてOriginにトラフィックを転送し、SSL終端やDDoS軽減（Nginxのレートリミットなど）を行います。攻撃を受けた際に破棄・再構築される使い捨てのコンポーネントです。
- **MVPでの実装範囲**: WireGuardクライアントとしてOriginに接続、Nginxによるリバースプロキシ設定の適用、SSL証明書（Let's Encrypt）の取得と更新。

### 1.3. Origin（オリジン/各ホスト）
- **役割**: 実際のWebサービスやアプリケーションが動作するバックエンドホスト。Edge NodeからのWireGuardトンネル接続を受け入れ、保護された環境でアプリケーションを実行します。
- **MVPでの実装範囲**: WireGuardサーバーとしてEdge Nodeからの接続を受け入れ、アプリケーション（例: 開発用Webサーバー）へのトラフィックを転送。

---

## 2. プロジェクトの土台作り（Control Plane）

### 2.1. ディレクトリ構造

まず、Control Planeの基本的なディレクトリ構造を定義します。技術スタックはGoを想定します。

```
/kokoa-proxy
├── control-plane/
│   ├── cmd/
│   │   └── kokoa-cp/
│   │       └── main.go       # メインアプリケーションのエントリポイント
│   ├── internal/
│   │   ├── api/              # APIハンドラ、ルーティング
│   │   ├── config/           # 設定ファイルの読み込み
│   │   ├── db/               # データベース（SQLite）のセットアップ、クエリ
│   │   ├── generator/        # WireGuard/Nginx設定生成
│   │   ├── model/            # データモデル（Origin, EdgeNode）
│   │   └── web/              # Web UIの静的ファイル（HTML, CSS, JS）
│   ├── go.mod
│   ├── go.sum
│   └── Dockerfile
├── scripts/                  # CLIツールや補助スクリプト
│   └── kokoa-dns/
└── docker-compose.yml
```

### 2.2. 初期セットアップ

- `go mod init github.com/your-username/kokoa-proxy/control-plane` を実行
- `main.go`に基本的なWebサーバー（`net/http`）を実装

## 3. Control Plane - コア機能

### 3.1. 最小DBスキーマ

MVPでは、SQLiteを使用してシンプルに保ちます。`control-plane/internal/db/schema.sql`を作成します。

```sql
CREATE TABLE IF NOT EXISTS origins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    domain TEXT NOT NULL UNIQUE,
    wireguard_ip TEXT NOT NULL UNIQUE,
    wireguard_public_key TEXT NOT NULL,
    wireguard_private_key_encrypted TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS edge_nodes (
    id TEXT PRIMARY KEY,
    origin_id TEXT NOT NULL,
    name TEXT NOT NULL,
    public_ip TEXT NOT NULL UNIQUE,
    wireguard_ip TEXT NOT NULL UNIQUE,
    wireguard_public_key TEXT NOT NULL,
    wireguard_private_key_encrypted TEXT NOT NULL,
    health_status TEXT NOT NULL DEFAULT 'unknown', -- "unknown", "healthy", "unhealthy"
    created_at INTEGER NOT NULL,
    FOREIGN KEY(origin_id) REFERENCES origins(id)
);
```

### 3.2. 最小APIエンドポイント

`internal/api/handlers.go`に以下のハンドラを実装します。

- **`POST /api/v1/origins`**
  - リクエストボディ: `{"name": "my-origin", "domain": "example.com"}`
  - 処理:
    1. WireGuard鍵ペア生成
    2. `10.100.X.1`形式のWireGuard IPを割り当て
    3. DBにOriginを保存
  - レスポンス: `201 Created`

- **`POST /api/v1/origins/{id}/edge-nodes`**
  - リクエストボディ: `{"name": "my-edge", "public_ip": "1.2.3.4"}`
  - 処理:
    1. WireGuard鍵ペア生成
    2. `10.100.X.10-254`形式のWireGuard IPを割り当て
    3. DBにEdgeNodeを保存
  - レスポンス: `201 Created`

- **`GET /api/v1/origins/{id}/config-bundle`**
  - 処理:
    1. DBからOriginと関連する全EdgeNodeを取得
    2. WireGuard設定ファイル（`wg0.conf`）を生成
    3. インストールスクリプト（`install.sh`）を生成
    4. これらを `.tar.gz` 形式で圧縮
  - レスポンス: `200 OK` (Content-Type: `application/gzip`)

- **`GET /api/v1/edge-nodes/{id}/config-bundle`**
  - 処理:
    1. DBからEdgeNodeと関連するOriginを取得
    2. WireGuard設定ファイル（`wg-origin-X.conf`）を生成
    3. Nginx設定ファイル（`nginx.conf`）を生成
    4. インストールスクリプト（`install.sh`）を生成
    5. これらを `.tar.gz` 形式で圧縮
  - レスポンス: `200 OK` (Content-Type: `application/gzip`)

### 3.3. 最小Web UI

`internal/web/` に単一の `index.html` を作成します。

- **機能**:
  - Origin登録フォーム（名前, ドメイン）
  - Edge Node登録フォーム（Origin選択, 名前, Public IP）
  - 登録済みのOriginとEdge Nodeの一覧表示（名前, ドメイン, IP, ヘルスステータス）
  - 各ノードの設定バンドルをダウンロードするリンク
- **実装**:
  - シンプルなHTMLとCSS（フレームワーク不要）
  - JavaScript (`fetch` API) でControl PlaneのAPIを呼び出す

## 4. 設定ファイルの自動生成

`internal/generator/` に実装します。

### 4.1. WireGuard設定 (`wg.go`)

- `GenerateOriginConfig(origin, edgeNodes)`: Origin用の`wg0.conf`を生成
- `GenerateEdgeNodeConfig(edgeNode, origin)`: Edge Node用の`wg-origin-X.conf`を生成

### 4.2. Nginx設定 (`nginx.go`)

- `GenerateNginxConfig(edgeNode, origin)`: Edge Node用の`nginx.conf`を生成
  - `server_name` は Origin のドメインを使用
  - `proxy_pass` は Origin のWireGuard IPアドレス（例: `http://10.100.0.1`）
  - SSL証明書パスは `/etc/letsencrypt/live/<domain>/...` の固定パス

## 5. 基本的なヘルスチェック

- `internal/health/` に実装
- `StartChecker()`:
  - 30秒ごとにDBから全EdgeNodeとOriginを取得
  - Goルーチンで各ノードのWireGuard IPに `ping` を実行
  - 結果（"healthy", "unhealthy"）をDBの `health_status` カラムに更新
  - Pingが失敗した場合でも、MVPでは自動アクションは行わない

## 6. 手動DNS更新CLIツール

`scripts/kokoa-dns/` にGoで実装します。

- **`main.go`**: `cobra` や `flag` パッケージでCLIを実装
- **環境変数**: `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ZONE_ID` を読み込む
- **コマンド**:
  - `kokoa-dns list`: ドメインのAレコードを一覧表示
  - `kokoa-dns add --ip <ip_address>`: Aレコードを追加
  - `kokoa-dns remove --ip <ip_address>`: IPアドレスに一致するAレコードを削除

## 7. インストール/セットアップスクリプト

- **`install-origin.sh`**
  ```bash
  #!/bin/bash
  set -e
  echo "Installing Kokoa Origin..."
  # WireGuardインストール
  apt-get update && apt-get install -y wireguard
  # 設定ファイル配置
  cp ./wg0.conf /etc/wireguard/wg0.conf
  # サービス起動
  systemctl enable wg-quick@wg0
  systemctl restart wg-quick@wg0
  echo "✅ Kokoa Origin installed."
  ```

- **`install-edge.sh`**
  ```bash
  #!/bin/bash
  set -e
  DOMAIN="<domain_from_config>"
  EMAIL="<admin_email_placeholder>"
  echo "Installing Kokoa Edge Node..."
  # Nginx, WireGuard, Certbotインストール
  apt-get update && apt-get install -y wireguard nginx certbot python3-certbot-nginx
  # 設定ファイル配置
  cp ./nginx.conf /etc/nginx/sites-available/default
  cp ./wg-origin-*.conf /etc/wireguard/
  # SSL証明書取得
  certbot --nginx -d $DOMAIN --non-interactive --agree-tos -m $EMAIL
  # サービス起動
  systemctl enable wg-quick@wg-origin-X
  systemctl restart wg-quick@wg-origin-X
  systemctl restart nginx
  echo "✅ Kokoa Edge Node installed."
  ```