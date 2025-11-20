# HAなWebサービスのトンネリング・防弾ホスティング要件まとめ

## 要件の整理

### 主要な目的
一般的なWebサービスを自宅サーバーでセルフホストし、安全かつ高可用性で公開したい。

### 具体的な要件

#### 1. 高可用性（HA: High Availability）
- 複数の入口ノードによる冗長化
- 1つのノードが停止しても、サービスは継続稼働
- 自動フェイルオーバー（手動でも可）

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
- ⚠️ 自動フェイルオーバーは手動実装が必要

**結論**: 使用可能だが、フェイルオーバーを自作する必要あり

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

## 推奨ソリューション: シンプルな3層アーキテクチャ

### アーキテクチャ図

```
[ユーザー]
    ↓ DNS (GeoDNS/Round Robin)
    
【Layer 1: 使い捨て入口ノード (VPS)】
[VPS #1 - Nginx + WireGuard Client] ← メイン
[VPS #2 - Nginx + WireGuard Client] ← フェイルオーバー
[VPS #3 - Nginx + WireGuard Client] ← 予備

    ↓ WireGuardトンネル (暗号化)
    
【Layer 2: 自宅サーバー (オリジン)】
[HAProxy/Nginx] ← ロードバランサー
    ├─ Webサービス #1
    ├─ Webサービス #2
    └─ Webサービス #3

【Layer 3: DNS監視・自動切り替え】
[PowerDNS/Health Check Script]
```

### なぜこの構成か

1. **シンプル**: 複雑なソリューションを使わず、枯れた技術のみ
2. **柔軟**: VPSプロバイダーやWebサービスの種類に依存しない
3. **低コスト**: 月$10-15程度（Oracle Free Tierなら$0）
4. **完全制御**: すべてのコンポーネントを自分で管理
5. **使い捨て可能**: VPSは数分で作成・破棄可能

---

## 実装の核心

### 1. VPS側（入口ノード）

#### WireGuard設定
```bash
# /etc/wireguard/wg0.conf
[Interface]
PrivateKey = <VPS秘密鍵>
Address = 10.0.0.10/24  # VPS #1は.10, #2は.11...

[Peer]
PublicKey = <自宅サーバー公開鍵>
Endpoint = home.yourdyndns.net:51820
AllowedIPs = 10.0.0.1/32
PersistentKeepalive = 25
```

#### Nginx設定
```nginx
upstream backend {
    server 10.0.0.1:80;
    keepalive 32;
}

server {
    listen 80;
    listen 443 ssl http2;
    server_name *.yourdomain.com;
    
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    
    # DDoS軽減
    limit_req_zone $binary_remote_addr zone=one:10m rate=10r/s;
    limit_req zone=one burst=20 nodelay;
    
    # ファイルサイズ制限を大きく
    client_max_body_size 500M;
    
    location / {
        proxy_pass http://backend;
        proxy_http_version 1.1;
        proxy_set_header Connection "";
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        
        proxy_connect_timeout 5s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}
```

---

### 2. 自宅サーバー側（オリジン）

#### WireGuard Server
```bash
# /etc/wireguard/wg0.conf
[Interface]
Address = 10.0.0.1/24
PrivateKey = <サーバー秘密鍵>
ListenPort = 51820

# VPS #1
[Peer]
PublicKey = <VPS1公開鍵>
AllowedIPs = 10.0.0.10/32

# VPS #2
[Peer]
PublicKey = <VPS2公開鍵>
AllowedIPs = 10.0.0.11/32

# VPS #3
[Peer]
PublicKey = <VPS3公開鍵>
AllowedIPs = 10.0.0.12/32
```

#### HAProxy/Nginxでロードバランシング
```nginx
upstream app_servers {
    least_conn;
    server 127.0.0.1:8001 max_fails=3 fail_timeout=30s;
    server 127.0.0.1:8002 max_fails=3 fail_timeout=30s;
    server 127.0.0.1:8003 max_fails=3 fail_timeout=30s;
}

server {
    listen 80;
    
    location / {
        proxy_pass http://app_servers;
    }
}
```

---

### 3. DNS自動フェイルオーバー

#### オプション1: PowerDNS + Health Check
PowerDNSは5秒ごとにポート443をチェックし、ノードがダウンした場合、自動的にDNSレスポンスから除外します。

#### オプション2: DNSラウンドロビン + 手動切り替え
```bash
# Cloudflare API経由でDNS更新
# VPS #1が攻撃されたら削除
curl -X DELETE "https://api.cloudflare.com/client/v4/zones/$ZONE/dns_records/$VPS1_RECORD" \
  -H "Authorization: Bearer $TOKEN"

# 新しいVPSを追加
curl -X POST "https://api.cloudflare.com/client/v4/zones/$ZONE/dns_records" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"type":"A","name":"@","content":"新しいIP"}'
```

---

### 4. VPS自動作成・破棄スクリプト

```bash
#!/bin/bash
# rotate-vps.sh

VPS_PROVIDER="digitalocean"
TEMPLATE_IMAGE="ubuntu-24-04-wireguard"

echo "VPS under attack! Rotating..."

# 新しいVPS作成
NEW_VPS_ID=$(doctl compute droplet create edge-node-new \
    --image $TEMPLATE_IMAGE \
    --size s-1vcpu-1gb \
    --region sgp1 \
    --wait \
    --format ID \
    --no-header)

NEW_IP=$(doctl compute droplet get $NEW_VPS_ID --format PublicIPv4 --no-header)

# 設定デプロイ
ansible-playbook -i "$NEW_IP," deploy-edge.yml

# DNSレコード更新
update-dns-record "$NEW_IP"

# 古いVPS削除（3時間後）
(sleep 10800 && doctl compute droplet delete $OLD_VPS_ID -f) &

echo "New VPS $NEW_IP is now live!"
```

---

## コスト試算

| 項目 | 選択肢 | 月額コスト |
|------|--------|-----------|
| **入口ノード（VPS）** | DigitalOcean/Vultr/Hetzner × 2-3台 | $10-15 |
| | Oracle Cloud Free Tier × 2台 | **$0** |
| **トンネル** | WireGuard（セルフホスト） | $0 |
| **オリジン** | 自宅サーバー | 既存 |
| **DNS** | Cloudflare（プロキシOFF） | $0 |
| **監視** | Uptime Kuma（セルフホスト） | $0 |

**合計: $10-15/月（Oracle Free Tierなら$0）**

---

## SSL/TLS証明書の処理

### オプション1: VPS側でSSL終端（推奨）

**メリット:**
- 自宅サーバーはHTTPのみで済む
- 証明書更新が簡単（各VPSで独立）
- VPS間で証明書を共有する必要がない

**デメリット:**
- VPS追加時に証明書取得が必要
- VPSごとにLet's Encrypt設定が必要

#### 実装方法

**各VPSでCertbot使用:**
```bash
# VPSにCertbotインストール
apt install certbot python3-certbot-nginx

# ワイルドカード証明書取得（DNS認証）
certbot certonly --dns-cloudflare \
  --dns-cloudflare-credentials ~/.secrets/cloudflare.ini \
  -d yourdomain.com \
  -d *.yourdomain.com

# 自動更新設定
echo "0 3 * * * certbot renew --quiet" | crontab -
```

**Cloudflare DNS認証用の設定:**
```ini
# ~/.secrets/cloudflare.ini
dns_cloudflare_api_token = YOUR_CLOUDFLARE_API_TOKEN
```

**Nginx設定（VPS側）:**
```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com *.yourdomain.com;
    
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    
    # Mozilla Modern設定
    ssl_protocols TLSv1.3 TLSv1.2;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    ssl_prefer_server_ciphers off;
    
    # HSTS
    add_header Strict-Transport-Security "max-age=63072000" always;
    
    location / {
        proxy_pass http://10.0.0.1:80;  # 自宅サーバー（HTTP）
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto https;
    }
}

# HTTPからHTTPSへリダイレクト
server {
    listen 80;
    server_name yourdomain.com *.yourdomain.com;
    return 301 https://$host$request_uri;
}
```

---

### オプション2: 自宅サーバーでSSL終端（E2E暗号化）

**メリット:**
- エンドツーエンド暗号化
- VPSは完全にプロキシのみ（証明書不要）
- VPS追加時の設定が簡単

**デメリット:**
- WireGuardトンネル内でさらにTLS暗号化（二重暗号化）
- 証明書更新を自宅サーバーで管理
- VPSでSSLインスペクション不可

#### 実装方法

**自宅サーバーでCertbot:**
```bash
# 自宅サーバーにCertbotインストール
apt install certbot

# DNS認証でワイルドカード証明書取得
certbot certonly --manual --preferred-challenges dns \
  -d yourdomain.com \
  -d *.yourdomain.com

# または、Cloudflare DNS Plugin使用
certbot certonly --dns-cloudflare \
  --dns-cloudflare-credentials ~/.secrets/cloudflare.ini \
  -d yourdomain.com \
  -d *.yourdomain.com
```

**Nginx設定（自宅サーバー）:**
```nginx
server {
    listen 80;
    listen 443 ssl http2;
    server_name yourdomain.com *.yourdomain.com;
    
    ssl_certificate /etc/letsencrypt/live/yourdomain.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/yourdomain.com/privkey.pem;
    
    ssl_protocols TLSv1.3 TLSv1.2;
    ssl_ciphers ECDHE-ECDSA-AES128-GCM-SHA256:ECDHE-RSA-AES128-GCM-SHA256;
    
    location / {
        proxy_pass http://localhost:8080;  # バックエンドアプリ
        # ... その他の設定
    }
}
```

**VPS設定（TCPプロキシのみ）:**
```nginx
stream {
    upstream backend_ssl {
        server 10.0.0.1:443;
    }
    
    server {
        listen 443;
        proxy_pass backend_ssl;
        proxy_protocol on;  # オプション
    }
}

server {
    listen 80;
    return 301 https://$host$request_uri;
}
```

---

### オプション3: ハイブリッド（証明書共有）

**メリット:**
- VPSでSSL終端（パフォーマンス）
- 証明書は自宅サーバーで一元管理
- 証明書更新は1箇所のみ

**デメリット:**
- 証明書をVPSに配布する仕組みが必要
- やや複雑

#### 実装方法

**自宅サーバーで証明書取得・配布:**
```bash
#!/bin/bash
# sync-certs.sh - 証明書をVPSに配布

CERT_DIR="/etc/letsencrypt/live/yourdomain.com"
VPS_LIST=("vps1.example.com" "vps2.example.com" "vps3.example.com")

# 証明書更新
certbot renew --quiet

# 各VPSに配布
for VPS in "${VPS_LIST[@]}"; do
    rsync -avz --delete \
        -e "ssh -i ~/.ssh/vps_key" \
        $CERT_DIR/ \
        root@$VPS:/etc/letsencrypt/live/yourdomain.com/
    
    # Nginxリロード
    ssh -i ~/.ssh/vps_key root@$VPS "nginx -s reload"
done
```

**cron設定（自宅サーバー）:**
```bash
# 毎日3時に証明書更新と配布
0 3 * * * /root/sync-certs.sh
```

---

### オプション4: Cloudflare Origin Certificate（最も簡単）

**メリット:**
- 証明書管理が不要
- Cloudflare側で自動更新
- 15年間有効

**デメリット:**
- Cloudflareに依存
- Cloudflare以外からのアクセスでは警告
- プロキシモードOFFでも使用可能

#### 実装方法

**Cloudflare Dashboardで証明書作成:**
1. Cloudflare Dashboard → SSL/TLS → Origin Server
2. "Create Certificate"をクリック
3. ワイルドカード証明書を選択: `*.yourdomain.com`, `yourdomain.com`
4. 証明書と秘密鍵をダウンロード

**VPSまたは自宅サーバーに配置:**
```bash
# 証明書配置
cat > /etc/ssl/cloudflare/cert.pem << 'EOF'
-----BEGIN CERTIFICATE-----
[Cloudflareからコピー]
-----END CERTIFICATE-----
EOF

cat > /etc/ssl/cloudflare/key.pem << 'EOF'
-----BEGIN PRIVATE KEY-----
[Cloudflareからコピー]
-----END PRIVATE KEY-----
EOF

chmod 600 /etc/ssl/cloudflare/key.pem
```

**Nginx設定:**
```nginx
server {
    listen 443 ssl http2;
    server_name yourdomain.com *.yourdomain.com;
    
    ssl_certificate /etc/ssl/cloudflare/cert.pem;
    ssl_certificate_key /etc/ssl/cloudflare/key.pem;
    
    # Cloudflare IPからのみアクセス許可（オプション）
    # https://www.cloudflare.com/ips/
    allow 173.245.48.0/20;
    allow 103.21.244.0/22;
    # ... その他のCloudflare IP
    deny all;
    
    location / {
        proxy_pass http://10.0.0.1:80;
        # ...
    }
}
```

---

## SSL処理の推奨構成

### 小規模・シンプル重視
**オプション1（VPS側でSSL終端） + Certbot DNS認証**
- 各VPSで独立して証明書管理
- VPS追加時は新規取得が必要だが、自動化可能

### セキュリティ重視
**オプション2（自宅サーバーでSSL終端）**
- エンドツーエンド暗号化
- VPSは完全にダムプロキシ

### 運用重視
**オプション4（Cloudflare Origin Certificate）**
- 証明書管理が最も簡単
- Cloudflareへの依存は許容範囲内

### 自動化スクリプトの例

**VPS自動作成時の証明書取得:**
```bash
#!/bin/bash
# deploy-edge-node.sh

NEW_IP=$1
DOMAIN="yourdomain.com"

# VPSにSSH接続可能になるまで待機
until ssh root@$NEW_IP "echo ready"; do
    sleep 5
done

# Ansible経由で設定デプロイ
ansible-playbook -i "$NEW_IP," \
    -e "certbot_email=admin@$DOMAIN" \
    -e "domain=$DOMAIN" \
    deploy-edge.yml

# deploy-edge.ymlの内容（抜粋）
# - name: Install Certbot
#   apt:
#     name:
#       - certbot
#       - python3-certbot-nginx
#       - python3-certbot-dns-cloudflare
#
# - name: Create Cloudflare credentials
#   copy:
#     content: |
#       dns_cloudflare_api_token = {{ cloudflare_api_token }}
#     dest: /root/.secrets/cloudflare.ini
#     mode: 0600
#
# - name: Obtain SSL certificate
#   command: >
#     certbot certonly --dns-cloudflare
#     --dns-cloudflare-credentials /root/.secrets/cloudflare.ini
#     -d {{ domain }} -d *.{{ domain }}
#     --non-interactive --agree-tos
#     -m {{ certbot_email }}
```

---

## 実装のステップ

### Phase 1: 基本構築
1. 自宅サーバーにWireGuard Serverをセットアップ
2. VPS 1台にWireGuard Client + Nginxをセットアップ
3. DNS設定（Aレコード1つ）
4. 動作確認

### Phase 2: 冗長化
1. VPS 2台目をセットアップ
2. DNSにAレコード追加（ラウンドロビン）
3. フェイルオーバーテスト

### Phase 3: 自動化
1. VPS作成スクリプト（Terraform/Ansible）
2. DNS更新スクリプト
3. ヘルスチェック・監視設定

### Phase 4: 防弾化
1. 攻撃検知（fail2ban等）
2. 自動VPS切り替えスクリプト
3. 運用手順書の作成

---

## 代替案・将来的な拡張

### より高度な構成が必要になった場合

1. **Netmaker Professional**
   - 有料だが、エンタープライズグレードの自動フェイルオーバー
   - 完全セルフホスト可能

2. **Kubernetes + Ingress Controller**
   - より大規模な展開に対応
   - 学習コストが高い

3. **Cloudflare Tunnel（緊急時）**
   - 大規模DDoS時の一時的なフェイルバック先
   - 平時はセルフホスト入口ノードを使用

---

## まとめ

### 推奨構成の利点
- ✅ HAなWebサービスを実現
- ✅ トンネリングで自宅サーバーを保護
- ✅ 防弾ホスト（VPS使い捨て可能）
- ✅ 完全セルフホスト
- ✅ シンプルで枯れた技術のみ
- ✅ 低コスト（月$0-15）

### トレードオフ
- 自動フェイルオーバーは手動実装が必要
- VPS管理の運用コストがかかる
- 初期セットアップに時間がかかる

### 最終推奨
**WireGuard + Nginx + DNS管理のシンプルな3層構成**が、現時点で最もバランスの取れた解決策です。

複雑なソリューションを導入するよりも、枯れた技術を組み合わせて自動化スクリプトで補完する方が、長期的なメンテナンス性と柔軟性の面で優れています。