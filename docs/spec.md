# HAなWebサービスのトンネリング・防弾ホスティング要件まとめ

## 要件の整理

### 主要な目的
一般的なWebサービスを自宅サーバーでセルフホストし、安全かつ高可用性で公開したい。

### 具体的な要件

#### 1. 高可用性（HA: High Availability）
- 複数の入口ノードによる冗長化
- 1つのノードが停止しても、サービスは継続稼働
- **自動フェイルオーバー**: DNS自動更新により障害ノードを30秒以内に除外
- **自動VPS交換**: オプション機能（設定で有効化可能）

#### 2. トンネリング
- 自宅サーバー（プライベートネットワーク）からVPS経由で公開
- ポートフォワード不要
- 暗号化された通信経路

#### 3. 防弾ホスト（Bulletproof Hosting）
- DDoS攻撃やその他の攻撃を受けた際、入口ノード（VPS）を即座に破棄可能
- 新しい入口ノードを迅速に作成・展開
- DNS切り替えによる自動/半自動のトラフィック移行
- 攻撃を受けても、バックエンド（自宅サーバー）は保護される

#### 4. 完全セルフホスト
- サードパーティのコントロールプレーンに依存しない
- オープンソースソリューション
- データ主権の確保

#### 5. 技術的制約
- **Cloudflareのリバースプロキシは使わない**（制限や依存を避けるため）
- Cloudflareは**DNSのみ**で利用（プロキシOFF）
- データとトラフィックは自分で完全に制御

---

## 検討したソリューション

### 1. Pangolin
**概要**: セルフホスト可能なトンネル型リバースプロキシ

**特徴**:
- Traefik + WireGuardベース
- Web UIによる管理
- SSO、2FA対応

**制限**:
- ❌ 完全セルフホストで複数ノードの自動フェイルオーバーは不可
- Remote Node機能にはPangolin Cloudのコントロールプレーンが必要

**結論**: 要件を満たさない

---


### 2. Firezone
**概要**: WireGuardベースのゼロトラストVPN

**特徴**:
- 複数Gatewayによる自動フェイルオーバー・ロードバランシング
- エンタープライズグレードの機能

**制限**:
- ⚠️ セルフホストは可能だが、公式サポート外
- 内部APIが急速に変化しており、本番環境での使用は推奨されない

**結論**: 現時点では不安定

---


### 3. Wiredoor
**概要**: セルフホストIngress-as-a-Serviceプラットフォーム

**特徴**:
- WireGuard + NGINXベース
- Gateway Node対応
- Kubernetes統合
- Apache-2.0ライセンス

**制限**:
- ⚠️ DNS自動フェイルオーバーは自分で実装が必要
- ⚠️ VPS自動作成・破棄機能は含まれていない

**結論**: 使用可能だが、DNS統合とVPS管理の自動化を追加実装する必要あり

---


### 4. Netmaker
**概要**: WireGuardベースのメッシュネットワークプラットフォーム

**特徴**:
- ロードバランシング・フェイルオーバー機能
- 複数のRemote Access Gateway展開可能
- カーネルWireGuardで高速

**制限**:
- ❌ Failover機能はProfessional Edition（有料）のみ
- Community Editionでは自動フェイルオーバー不可

**結論**: 有料版なら要件を満たす、無料版は不十分

---


### 5. NetBird
**概要**: Tailscaleのセルフホスト代替

**特徴**:
- 完全セルフホスト可能
- アイデンティティ管理統合
- 使いやすいWeb UI

**制限**:
- 複数リレーサーバーの構成が文書化されていない
- 入口ノード管理には不向き

**結論**: メッシュVPNとしては優秀だが、今回の用途には適さない

---


### 6. Headscale
**概要**: Tailscaleのコントロールサーバーのオープンソース実装

**特徴**:
- 完全セルフホスト
- Tailscaleクライアントと互換性

**制限**:
- フェイルオーバーは手動実装が必要
- 入口ノード管理機能なし

**結論**: ベースとして使えるが、追加実装が必要

---

## 推奨ソリューション: 3層アーキテクチャ

### アーキテクチャ図

```
[ユーザー]
    ↓ DNS (GeoDNS/Round Robin for app.hoge.com, wiki.fuga.com)
    
【Layer 1: 使い捨て入口ノード (VPS)】
[VPS #1 - Nginx + WireGuard Client + Kokoa Agent]
[VPS #2 - Nginx + WireGuard Client + Kokoa Agent]

    ↓ WireGuardトンネル (暗号化)
    
【Layer 2: 自宅サーバー (オリジン)】
[Origin #1] ─ Webサービス A, B
[Origin #2] ─ Webサービス C

【Layer 3: Control Plane】
[Kokoa Control Plane]
 - DB (Origins, Routes)
 - Config API (for Agents)
 - Web UI
```

---

## 実装の核心

### 1. VPS側（入口ノード）

#### Nginx設定（マルチドメイン・マルチサブドメイン対応）
```nginx
# /etc/nginx/conf.d/kokoa-map.conf
# このファイルはControl Planeから動的に生成・配布される
map $host $kokoa_backend {
    "app.hoge.com"       "10.100.1.1:8080";
    "wiki.fuga.com"      "10.100.1.1:8081";
    "metrics.hoge.com"   "10.100.2.1:9090"; # 別のOriginへ
    default              "";
}

# /etc/nginx/sites-available/default
server {
    listen 80;
    listen 443 ssl http2;

    # このサーバーブロックがデフォルトとしてすべてのリクエストを受ける
    # server_name は指定しないか、"_" を指定する
    server_name _;

    # SSL証明書はSNIにより自動で選択される
    # この部分はControl Planeが管理するホスト名に応じて動的に更新される
    ssl_certificate /etc/letsencrypt/live/app.hoge.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/app.hoge.com/privkey.pem;
    ssl_certificate /etc/letsencrypt/live/wiki.fuga.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/wiki.fuga.com/privkey.pem;

    # DDoS軽減（アプリケーションレイヤー / Layer 7）
    limit_req_zone $binary_remote_addr zone=one:10m rate=10r/s;
    limit_req zone=one burst=20 nodelay;
    
    location / {
        # mapでバックエンドが定義されていないホストからのリクエストは404を返す
        if ($kokoa_backend = "") {
            return 404; 
        }
        proxy_pass http://$kokoa_backend;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

---

### 2. 自宅サーバー側（オリジン）

#### WireGuard Server
```bash
# /etc/wireguard/wg0.conf on Origin #1
[Interface]
Address = 10.100.1.1/24
PrivateKey = <origin1_private_key>
ListenPort = 51820

# Edge Node #1
[Peer]
PublicKey = <vps1_public_key>
AllowedIPs = 10.100.1.10/32

# Edge Node #2
[Peer]
PublicKey = <vps2_public_key>
AllowedIPs = 10.100.1.11/32
```

---

### 3. DNS管理

Control Planeに統合されたDNS Managerは、**単一のDNSゾーン**（例: `hoge.com`）の管理を前提とします。`fuga.com`のような異なるゾーンのDNSレコードは、別途手動で設定するか、将来的にControl Planeがマルチゾーンの認証情報を管理できるように拡張する必要があります。

```bash
# Control Planeの機能 or kokoa-dns CLIを利用
# 'app.hoge.com' ホスト名に新しいIPを追加
$ kokoa-dns add --name app --ip <new_ip_address>
```

---

## SSL/TLS証明書の処理

Edge Nodeに常駐するエージェントが、Control Planeから受け取ったホスト名リスト（例: `app.hoge.com`, `wiki.fuga.com`）に基づき、HTTP-01(webroot)で`certbot certonly`を実行し、証明書を取得・管理します。CPはホスト名リストとstaging/本番フラグを渡すだけで、チャレンジはEdgeで完結します。異なるベースドメインでも問題なく動作します。

```bash
# Edge Node上のエージェントが実行するコマンドの例
HOSTNAMES_FROM_CP=("app.hoge.com" "wiki.fuga.com")
EMAIL="admin@example.com"
WEBROOT="/var/www/letsencrypt"
mkdir -p "$WEBROOT"

for HOST in "${HOSTNAMES_FROM_CP[@]}"; do
  # 証明書がなければ新規取得
  if [ ! -f "/etc/letsencrypt/live/$HOST/fullchain.pem" ]; then
    echo "Obtaining certificate for $HOST..."
    certbot certonly --webroot -w "$WEBROOT" -d "$HOST" --non-interactive --agree-tos -m "$EMAIL"
  fi
done
```
