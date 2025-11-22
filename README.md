# kokoa-proxy

軽量なコントロールプレーン＋エッジエージェントで、ドメインごとのリバースプロキシ設定をPull型で配布するMVPプロジェクトです。Control PlaneはGo+SQLite、EdgeはBash+nginxで構成します。

## フォルダ構成
- `control-plane/` — Go実装のコントロールプレーン（API/DB/設定生成、Dockerfile含む）
- `scripts/` — エッジエージェントとDNS CLIのスクリプト
- `docs/` — 仕様・計画・構成ガイドなどのドキュメント
- `docker-compose.yaml` — Control PlaneをComposeで起動
- `install-edge.sh` / `install-origin.sh` — インストーラ雛形
- `.env.example` — Control Planeの環境変数サンプル
- `Makefile` — 開発用ショートカット（test/run/build/docker-up/down）

詳細は `docs/structure.md` を参照してください。

## クイックスタート（抜粋）
1) `.env.example` を `.env` にコピーし、`CP_BOOTSTRAP_TOKEN` を設定  
2) `docker compose up --build` で起動（または `make run` でローカル起動）  
3) ブートストラップトークンで Edge を登録 → Origin/Route を登録 → Edge トークンで config を取得  
詳しい手順は `docs/quickstart.md` を参照してください。
