# kokoa-proxy

軽量なコントロールプレーン＋エッジエージェントで、ドメインごとのリバースプロキシ設定をPull型で配布するMVPプロジェクトです。Control PlaneはGo+SQLite、EdgeはBash+nginxで構成します。

## フォルダ構成
- `control-plane/` — Go実装のコントロールプレーン（API/DB/設定生成、Dockerfile含む）
- `scripts/` — エッジエージェントとDNS CLIのスクリプト
- `docs/` — 仕様・計画・構成ガイドなどのドキュメント
- `docker-compose.yaml` — Control PlaneをComposeで起動
- `install-edge.sh` / `install-origin.sh` — インストーラ雛形

詳細は `docs/structure.md` を参照してください。
