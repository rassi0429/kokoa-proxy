# 実装状況 (MVP)

## 完了した実装
- Control Planeスケルトン（`control-plane/`）: エントリーポイント、HTTPサーバ配線、環境変数設定、Graceful shutdown。
- 永続化: SQLiteスキーマ（`origins`, `routes`, `edge_nodes`）、起動時マイグレーション、作成系ヘルパー（origin/route、edge登録、トークン検索、last_seen更新、route一覧）。
- API: `POST /api/v1/origins`, `POST /api/v1/routes`, `POST /api/v1/edge-nodes/register`（ブートストラップトークン必須）、`GET /api/v1/edge-nodes/me/config`（Bearerトークン）、`GET /healthz`。簡易ログとプレースホルダWeb UI。
- 設定生成: nginx `map`決定論生成＋config hash/hostnameソート、順序性テストあり。
- コンテナ/Compose: `control-plane/Dockerfile`（linux/amd64静的ビルド、Alpineランタイム、`/data`ボリューム）と`docker-compose.yaml`（8080公開、永続ボリューム）。
- Edge/Originツール: エージェントループ（`scripts/kokoa-edge-agent/poll_config.sh`）でconfig取得→nginx map原子的反映→reload＋バックオフ。インストーラ雛形（`install-edge.sh`, `install-origin.sh`）、DNS CLIスタブ（`scripts/kokoa-dns/kokoa-dns.sh`）。
- 追加整備: `go mod tidy`で`go.sum`生成、APIバリデーション（hostname/IP/port）、JSON Content-Typeチェック、制約違反時409返却、レートリミット（IPごと60req/分）、構造化ログ風のステータス出力、APIユニットテスト（バリデーション/ルート作成）、`go test ./...`実行。

## 残タスク / 次にやること
- staticcheck導入、SQLite CLI整備。
- API強化: hostname/IPの詳細バリデーション、エラーの統一レスポンススキーマ、より厳密なRate Limit/認証設定。
- WireGuard鍵暗号化（`WIREGUARD_KEY_ENC_SECRET`）、WG IP衝突チェック強化、Origin Peer管理APIの追加。
- Edgeエージェント拡張: certbot連携、nginx per-hostサーバsnippet生成、staging切替、失敗時ハンドリング/メトリクス。
- Web UI拡充: origins/routesのCRUD、Basic Authオプション、API統合。
- E2E/開発スクリプト（サンプルenv/Makefile）、ヘルスチェック、CI（`go test`, `go vet`, `staticcheck`）整備。
