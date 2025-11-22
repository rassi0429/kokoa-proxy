# 実装状況 (MVP)

## 完了した実装
- Control Planeスケルトン（`control-plane/`）: エントリーポイント（`cmd/kokoa-cp`）、HTTPサーバ配線、環境変数設定（`CP_LISTEN_ADDR`, `CP_DB_PATH`, `CP_BOOTSTRAP_TOKEN`）、Graceful shutdown。
- 永続化: SQLiteスキーマ（`origins`, `routes`, `edge_nodes`）、起動時マイグレーション、作成系ヘルパー（origin/route、edge登録、トークン検索、last_seen更新、route一覧）。
- API: `POST /api/v1/origins`, `POST /api/v1/routes`, `POST /api/v1/edge-nodes/register`（ブートストラップトークン必須）、`GET /api/v1/edge-nodes/me/config`（Bearerトークン）、`GET /healthz`。簡易ログとプレースホルダWeb UI。
- 設定生成: nginx `map`決定論生成＋config hash/hostnameソート、順序性テストあり。
- コンテナ/Compose: `control-plane/Dockerfile`（linux/amd64静的ビルド、Alpineランタイム、`/data`ボリューム）と`docker-compose.yaml`（8080公開、永続ボリューム）。
- Edge/Originツール: エージェントループ（`scripts/kokoa-edge-agent/poll_config.sh`）でconfig取得→nginx map原子的反映→reload＋バックオフ。インストーラ雛形（`install-edge.sh`, `install-origin.sh`）、DNS CLIスタブ（`scripts/kokoa-dns/kokoa-dns.sh`）。

## 残タスク / 次にやること
- Go依存取得（`go mod tidy`）で`go.sum`生成、staticcheckとSQLite CLI導入。
- APIバリデーション強化（hostname/IP形式、target_port範囲、ユニーク制約のエラー整形）とAPI/DBユニットテスト追加。
- 認証/Rate Limitミドルウェア、構造化ログ、エラー応答の整備。
- WireGuard鍵暗号化（`WIREGUARD_KEY_ENC_SECRET`）、WG IP衝突チェック、Origin Peer管理APIの追加。
- Edgeエージェント拡張: certbot連携、nginx per-hostサーバsnippet生成、staging切替、失敗時ハンドリング/メトリクス。
- Web UI拡充: origins/routesのCRUD、Basic Authオプション、API統合。
- E2E/開発スクリプト（サンプルenv/Makefile）、ヘルスチェック、CI（`go test`, `go vet`, `staticcheck`）整備。
