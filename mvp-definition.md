# MVP（Minimal Viable Product）定義の詳細化 (マルチサブドメイン対応版)

`architecture.md`で定義された、より柔軟な「マルチサブドメイン対応」および「プル型設定取得」モデルに基づき、MVPの具体的で実行可能なステップを再定義します。

## 1. 主要な登場人物

このプロジェクトには、以下の3つの主要なコンポーネントがあります。

### 1.1. Control Plane（コントロールプレーン）
- **役割**: システム全体の管理、オーケストレーションを行う司令塔。
- **MVPでの実装範囲**:
  - OriginとRouteを管理するための基本的なWeb UIとAPI。
  - Edge Nodeからの設定取得リクエストに応答するAPI。
  - Nginxの`map`設定と、Edge Nodeが管理すべきホスト名リストを動的に生成する機能。

### 1.2. Edge Node（エッジノード）
- **役割**: インターネットからのリクエストを受け付ける、動的なリバースプロキシ。
- **MVPでの実装範囲**:
  - `kokoa-edge-agent`: 定期的にControl Planeに設定を問い合わせるシンプルなエージェント（cronで実行するシェルスクリプトで実装）。
  - 受信した設定に基づき、Nginxの`map`ファイルを更新し、Nginxをリロードする機能。
  - 受信したホスト名リストに基づき、`certbot`を実行してSSL証明書を個別に取得・管理する機能。

### 1.3. Origin（オリジン）
- **役割**: 実際のWebサービスが動作するバックエンドホスト。
- **MVPでの実装範囲**: WireGuardサーバーとしてEdge Nodeからの接続を受け入れる。

---

## 2. プロジェクトの土台作り（Control Plane）

### 2.1. ディレクトリ構造
Goを想定した基本的なディレクトリ構造。

```
/kokoa-proxy
├── control-plane/
│   ├── cmd/kokoa-cp/main.go
│   ├── internal/
│   │   ├── api/
│   │   ├── db/
│   │   ├── generator/
│   │   └── web/
│   ├── go.mod
│   └── Dockerfile
├── scripts/
│   ├── kokoa-dns/
│   └── kokoa-edge-agent/
└── docker-compose.yml
```

### 2.2. 初期セットアップ
- `go mod init` を実行し、基本的なWebサーバーを実装。

---

## 3. Control Plane - コア機能

### 3.1. 最小DBスキーマ
```sql
-- オリジンサーバー（単なるWireGuardピア）
CREATE TABLE IF NOT EXISTS origins (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    wireguard_ip TEXT NOT NULL UNIQUE,
    wireguard_public_key TEXT NOT NULL,
    wireguard_private_key_encrypted TEXT NOT NULL,
    created_at INTEGER NOT NULL
);

-- ルート（ホスト名とバックエンドサービスの紐付け）
CREATE TABLE IF NOT EXISTS routes (
  id TEXT PRIMARY KEY,
  hostname TEXT NOT NULL UNIQUE,
  origin_id TEXT NOT NULL,
  target_port INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  FOREIGN KEY(origin_id) REFERENCES origins(id)
);

-- エッジノード
CREATE TABLE IF NOT EXISTS edge_nodes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    auth_token_hash TEXT NOT NULL UNIQUE, -- 永続的な認証トークン
    last_seen INTEGER
);
```

### 3.2. 最小APIエンドポイント

- **`POST /api/v1/origins`**: Originを登録（`domain`フィールドは不要に）。
- **`POST /api/v1/routes`**: ホスト名、Origin、転送先ポートを紐付けるルートを作成。
- **`POST /api/v1/edge-nodes/register`**: Edge Nodeが初回起動時に自身を登録し、永続的な認証トークンを取得。
- **`GET /api/v1/edge-nodes/me/config`**: Edge Nodeが認証トークンを使い、自身の動的な設定（Nginxの`map`情報、ホスト名リスト）を取得。

### 3.3. 最小Web UI

- Origin登録フォーム（名前のみ）。
- Route登録フォーム（ホスト名、Origin選択、ポート番号）。
- 登録済みのOriginとRouteの一覧表示。

---

## 4. 設定ファイルの自動生成

### 4.1. Nginx `map` 設定生成 (`generator/nginx.go`)

- `GenerateNginxMapConfig(routes)`: DBの全ルート情報を元に、`map $host $kokoa_backend`ディレクティブを含む設定ファイルを生成する。

---

## 5. Edge Node エージェント (`scripts/kokoa-edge-agent/`)

MVPでは、cronで定期的に実行されるシェルスクリプトとして実装。

**`poll_config.sh`**:
```bash
#!/bin/bash
CONTROL_PLANE_URL="https://control.example.com"
NODE_TOKEN="<this-node-persistent-token>" # 初回登録時に取得・保存
CONFIG_DIR="/etc/kokoa-edge"

# 1. Control Planeから設定を取得
CONFIG_JSON=$(curl -s -H "Authorization: Bearer $NODE_TOKEN" "$CONTROL_PLANE_URL/api/v1/edge-nodes/me/config")

# 2. Nginxのmapファイルを生成・更新
echo "$CONFIG_JSON" | jq -r '.routes | to_entries | map("    \"\(.key)\" \"\(.value)\";") | .[]' | \
  (echo "map \$host \$kokoa_backend {"; cat; echo "}") > "$CONFIG_DIR/kokoa-map.conf"

# Nginx設定をリロード
nginx -s reload

# 3. SSL証明書を管理
HOSTNAMES=$(echo "$CONFIG_JSON" | jq -r '.hostnames[]')
EMAIL="admin@example.com"

for HOST in $HOSTNAMES; do
  if [ ! -f "/etc/letsencrypt/live/$HOST/fullchain.pem" ]; then
    echo "Obtaining certificate for $HOST..."
    certbot --nginx -d "$HOST" --non-interactive --agree-tos -m "$EMAIL"
  fi
done
```

---

## 6. 手動DNS更新CLIツール (`scripts/kokoa-dns/`)

CLIツールの仕様は前回定義したもの（`--name`フラグでサブドメインを指定）で問題ありません。

- `kokoa-dns add --name app1 --ip <ip_address>`
- `kokoa-dns add --name app2 --ip <ip_address>`

---

## 7. インストール/セットアップスクリプト

### `install-edge.sh` (更新)
このスクリプトは、VPSの初回起動時に一度だけ実行されることを想定します。

```bash
#!/bin/bash
set -e

CONTROL_PLANE_URL="https://control.example.com"
BOOTSTRAP_TOKEN="<one-time-bootstrap-token>"

# 必要なパッケージをインストール
apt-get update && apt-get install -y wireguard nginx certbot python3-certbot-nginx jq curl

# エージェントスクリプトを配置
mkdir -p /opt/kokoa
cat << 'EOF' > /opt/kokoa/poll_config.sh
# --- 上記の poll_config.sh の内容をここに挿入 ---
EOF
chmod +x /opt/kokoa/poll_config.sh

# Control Planeに初回登録して永続トークンを取得
PERSISTENT_TOKEN=$(curl -s -d "{"bootstrap_token": "$BOOTSTRAP_TOKEN"}" "$CONTROL_PLANE_URL/api/v1/edge-nodes/register" | jq -r .auth_token)

# 永続トークンをスクリプトに埋め込む
sed -i "s|<this-node-persistent-token>|$PERSISTENT_TOKEN|" /opt/kokoa/poll_config.sh

# cronジョブを設定（例: 1分ごとに実行）
echo "* * * * * root /opt/kokoa/poll_config.sh" > /etc/cron.d/kokoa-edge-agent

# 初回実行
/opt/kokoa/poll_config.sh
```
*`install-origin.sh`は前回定義から大きな変更はありません。*

---

## 8. ホストのセットアップ (MVP)

MVP段階では、以下の3種類のホストを手動で準備することを想定します。

### 8.1. Control Plane ホスト

- **役割**: Kokoa Proxyの管理機能全体を実行するサーバー。
- **要件**:
    - **マシン**: 1台のVPSまたは自宅サーバー。
    - **OS**: Linux (Ubuntu 22.04 LTS推奨)。
    - **ソフトウェア**: DockerおよびDocker Compose。
    - **ネットワーク**:
        - 管理者からのWeb UIアクセスと、Edge/OriginノードからのAPIアクセス用にポートを公開（例: 80/443）。
        - 固定IPアドレスまたはDNS名を持つこと。

### 8.2. Origin ホスト

- **役割**: 公開したい実際のWebアプリケーションが動作しているサーバー。
- **要件**:
    - **マシン**: 自宅ネットワークやプライベートクラウド内の任意のサーバー/VM/PC。
    - **OS**: `systemd`と`wireguard`ツールが動作する標準的なLinux。
    - **ネットワーク**: インターネットへのアウトバウンド接続が可能であること（インバウンドポート開放は不要）。
    - **セットアップ**: Control PlaneのWeb UIからOriginを登録後、提供される設定ファイル（`origin-config.tar.gz`と`install-origin.sh`）をOriginホストにコピーし、`install-origin.sh`を実行することでセットアップします。これによりWireGuardが設定され、Control Planeとの接続が開始されます。

### 8.3. Edge Node ホスト

- **役割**: インターネットからのトラフィックを受け付ける公開エンドポイント。
- **要件**:
    - **マシン**: 1台のクリーンなVPS（DigitalOcean, Vultr, Hetznerなど）。
    - **OS**: Ubuntu 22.04 LTS推奨。
    - **ソフトウェア**: `install-edge.sh`スクリプトによって `wireguard`, `nginx`, `certbot`, `jq` が自動的にインストールされます。
    - **ネットワーク**:
        - グローバルな固定IPアドレスを持つこと。
        - ポート `80` と `443` がインターネットからアクセス可能であること。
    - **セットアップ**: `install-edge.sh`スクリプトを実行してセットアップします。
