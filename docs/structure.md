# フォルダ構成ガイド

## ルート
- `README.md`: プロジェクト概要とフォルダマップ
- `architecture.md` / `spec.md` / `mvp-definition.md` / `implementation-plan.md` / `implementation-progress.md`: 仕様・計画・進捗ドキュメント
- `docker-compose.yaml`: Control PlaneをDocker Composeで起動する設定
- `install-edge.sh` / `install-origin.sh`: Edge/Origin向けのインストーラ雛形

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
