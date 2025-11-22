# フォルダ構成ガイド

## ルート
- `README.md`: プロジェクト概要とフォルダマップ
- `docker-compose.yaml`: Control PlaneをDocker Composeで起動する設定
- `install-edge.sh` / `install-origin.sh`: Edge/Origin向けのインストーラ雛形
- `.env.example`: Control Plane起動用の環境変数サンプル
- `Makefile`: 開発用ショートカット（test/run/build/docker-up/down）

## control-plane/
- `cmd/kokoa-cp/`: Control Planeバイナリのエントリーポイント
- `internal/api/`: HTTP APIルーティングとハンドラ
- `internal/db/`: SQLiteスキーマとDBアクセス
- `internal/generator/`: nginx map生成とconfig hash
- `internal/web/`: 簡易Web UIプレースホルダ
- `Dockerfile`: Control Planeコンテナイメージのビルド定義
- `go.mod`, `go.sum`: Goモジュール定義

## scripts/
- `kokoa-edge-agent/`: エッジ側の設定ポーリングスクリプト
- `kokoa-dns/`: 単一ゾーン向けDNS CLIスタブ
- `dev/`: ローカル起動後にシード投入する補助スクリプト

## docs/
- 仕様・計画・進捗ドキュメント: `architecture.md`, `spec.md`, `mvp-definition.md`, `implementation-plan.md`, `implementation-progress.md`, `use-cases.md`, `GEMINI.md`, `gemini-problem.md`, `claude.md`
- 運用ガイド: `structure.md`（このファイル）, `quickstart.md`
