# Implementation Plan (MVP, architectureベース)

## 前提とスコープ
- `architecture.md` / `mvp-definition.md`に基づき、Control Planeはデータプレーンに入らず、EdgeがPullで設定取得するモデル
- 単一DNSゾーン前提。マルチゾーンのVPS作成/DNS自動化はMVP外（将来拡張でmodulesを有効化）
- 構成: Go製Control Plane + SQLite、最小Web UI、Bash Edge Agent + Nginx map、Let's Encrypt + certbot、手動DNS CLI、WireGuardでOrigin接続

## 実装チェックリスト
1) リポジトリ骨格
- `control-plane/`にGoモジュール初期化、`cmd/kokoa-cp/main.go`、`internal/{api,db,generator,web}`を作成
- Dockerfileとdocker-composeでCP+SQLiteを起動。80/443公開、DBはローカルボリューム
2) データレイヤ
- SQLiteスキーマをMVP通り実装（`origins`,`routes`,`edge_nodes`）。起動時に自動作成/マイグレーション
- IDはUUIDv4と作成時刻保存。WireGuard秘密鍵は暗号化フィールドを確保（鍵管理は後述）
3) 認証・トークン
- ブートストラップトークンを環境変数/設定で管理し、`/edge-nodes/register`で永続トークンを払い出し、DBにはハッシュ保存
- Bearerトークン認証ミドルウェアをAPIに適用。`last_seen`更新と簡易Rate Limit/ログを追加
- `config_hash`(payloadのSHA256)を返し、Agent側で無変更スキップ可能に
4) Control Plane API
- `POST /api/v1/origins`: name/WGパラメータを登録。入力バリデーションとWG IP重複防止
- `POST /api/v1/routes`: hostname, origin_id, target_port。重複存在チェック
- `POST /api/v1/edge-nodes/register`: bootstrap_token検証→永続トークン発行と返却
- `GET /api/v1/edge-nodes/me/config`: 認証→`routes` mapと`hostnames`配列、`config_hash`を返す
- `GET /healthz`等ヘルスチェック追加
5) 設定生成
- `internal/generator/nginx.go`: Routesから`map $host $kokoa_backend`リストとホスト名リストを決定論的に生成
- 空ルート/重複/ソートをテストするユニットテストを用意
6) Web UI (最小)
- Origins作成フォーム（name, wg_ip, pub/privキー）。Routes作成フォーム（hostname, origin選択, port）
- 一覧テーブル表示。API呼び出しはシンプルなfetchで実装。認証は暫定でなし（管理者のみ前提 or Basic Authを設定で選択可能）
7) Edge Agent (`scripts/kokoa-edge-agent/poll_config.sh`)
- 環境変数: `CONTROL_PLANE_URL`, `NODE_TOKEN`, `CONFIG_DIR`, `EMAIL`, `NGINX_BIN`
- Config取得し`config_hash`で差分判定。nginx mapファイルを一時ファイル経由で原子的に置換
- `nginx -t`で検証後にreload。失敗時は旧設定を保持しログ
- certbotはホストごとに存在確認して取得。連続取得を避けるため逐次実行とstagingオプション用フラグを用意
- HTTP/ネットワーク失敗時はバックオフ（例 5s→30s）。ログはsyslog/標準出力
8) インストールスクリプト
- `install-edge.sh`: 必要パッケージ(wg, nginx, certbot, jq, curl)導入→Agent配置→`/edge-nodes/register`で永続トークン取得→cron/systemd timer設定→初回実行
- `install-origin.sh`: WireGuardサーバ設定テンプレート展開（前回仕様踏襲）、防火壁開放(UDP 51820)
9) DNS CLI (`scripts/kokoa-dns/`)
- `kokoa-dns add --name <sub> --ip <addr>`など既定仕様を実装。単一ゾーン前提で、ベースドメインは設定ファイル/環境変数から取得
10) テスト・運用確認
- Goユニットテスト: generator, APIバリデーション, トークン検証
- ビルド/静的チェック: `go test ./...`, `go vet`, 可能なら`staticcheck`
- 手動E2E: CP起動→edge登録→origin/route登録→agent実行→nginx map更新→cert取得→実サービス到達

## 懸念・検討事項
- **WG鍵/秘密鍵管理**: `wireguard_private_key_encrypted`の暗号化方式（環境変数鍵でAES? KMS統一?）を決める必要  
    > 環境変数キーでAES-256-GCM暗号化。`WIREGUARD_KEY_ENC_SECRET`未設定ならCP起動エラー。レコードごとにランダムIVと認証タグを保存し、将来KMS差し替え可能なインタフェースを切る
- **WGアドレス割当**: IP衝突防止の割当ポリシーが未定。シンプルな手動入力＋重複検知で開始  
    > 問題なし
- **TLS/Certbot運用**: LE rate limit回避のためstaging切替と取得間隔制御が必要。複数ホスト証明書の同時取得によるnginx reload競合に注意  
    > EdgeがHTTP-01で各ホストを取得。CPは対象ホストとstaging/本番フラグだけ配布。stagingトグルは設定に保持
- **Nginx証明書設定**: `server_name _;`に対しcertbotで増える証明書パスをどう持つか？map生成だけでなく証明書列挙のテンプレをagentで管理  
    > エージェントが`/etc/nginx/kokoa/servers/<hostname>.conf`を生成し、`/etc/letsencrypt/live/<hostname>/...`を参照。未取得は共通デフォルト証明書でフォールバック。メイン`nginx.conf`は`include /etc/nginx/kokoa/servers/*.conf;`で列挙不要
- **多ゾーン対応**: 現状一つのDNSゾーンのみ。別ゾーンは手動設定か将来のモジュール化で対応  
    > そうだね、ENSはモジュールであるけど、Aレコードを手で設定できれば問題ないと思う
- **モジュール化**: architectureのDNS Manager/Edge Provisioner/SSL Manager/Health CheckerはMVPでは無効化。将来有効化を見越し、設定フラグとinternal APIの拡張余地を残す  
    > Edge用のSSLもHTTP-01で取得。`EDGE_FQDN`と`EMAIL`を受け取り初回にcertbot実行→`servers/<edge>.conf`生成。将来CP制御できるようフラグ化
- **観測性/ログ**: 最低限の構造化ログとリクエストIDを入れる。メトリクスは後回し  
    > OK
