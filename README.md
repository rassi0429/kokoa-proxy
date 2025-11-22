# kokoa-proxy

軽量なコントロールプレーン＋エッジエージェントで、ドメインごとのリバースプロキシ設定をPull型で配布するMVPプロジェクトです。Control PlaneはGo+SQLite、EdgeはBash+nginxで構成します。

## フォルダ構成
- `control-plane/` — Go実装のコントロールプレーン（API/DB/設定生成、Dockerfile含む）
- `scripts/` — エッジエージェントとDNS CLIのスクリプト
- `docs/` — フォルダ構成ガイドなどの補助ドキュメント
- `architecture.md` / `spec.md` / `mvp-definition.md` / `implementation-plan.md` / `implementation-progress.md` — 仕様・計画・進捗
- `docker-compose.yaml` — Control PlaneをComposeで起動
- `install-edge.sh` / `install-origin.sh` — インストーラ雛形

より詳しい構成は `docs/structure.md` を参照してください。
