# クイックスタート

前提: Go 1.22+、Docker/Docker Compose、curl、(任意で jq)。

1. ブートストラップトークンを決める  
   `.env.example` をコピーして `.env` を作成し、`CP_BOOTSTRAP_TOKEN` を任意の値に変更。

2. Control Plane を起動  
   ```bash
   docker compose up --build
   # またはローカルで:
   # cd control-plane && CP_BOOTSTRAP_TOKEN=yourtoken go run ./cmd/kokoa-cp
   ```
   **ボリューム権限で失敗する場合**: Dockerfileを更新したので再ビルドが必要です。既存の `cp-data` ボリュームを削除すると権限がリセットされます。例: `docker compose down -v && docker compose up --build`

3. Web UI から操作（ブラウザで `http://localhost:8080/`）  
   Origins/Routesの登録がブラウザ上で可能です。

4. Edge ノードを登録（ワンライナー配布）  
   ```bash
   # bash の環境に渡す場合（パイプラインでも確実に変数を渡せる方法）
   curl -fsSL http://localhost:8080/edge/install.sh | BOOTSTRAP_TOKEN=<CP_BOOTSTRAP_TOKEN> bash
   # または引数指定
   curl -fsSL http://localhost:8080/edge/install.sh | bash -s -- --bootstrap-token <CP_BOOTSTRAP_TOKEN>
   # WireGuardも自動セットアップする場合（例）
   curl -fsSL http://localhost:8080/edge/install.sh | BOOTSTRAP_TOKEN=<CP_BOOTSTRAP_TOKEN> WG_ADDR=10.0.0.3/32 WG_ENDPOINT=origin.example.com:51820 WG_PEER_PUBKEY=<origin_pubkey> bash
   ```
   ※systemdがない環境では最後に表示されるコマンドを手動で実行してください。

   Edge ノードを登録（curlで直接）  
   ```bash
   BOOT=yourtoken
   curl -X POST http://localhost:8080/api/v1/edge-nodes/register \
     -H "Authorization: Bearer ${BOOT}" \
     -H "Content-Type: application/json" \
     -d '{"name":"edge-1"}'
   # レスポンス: {"edge_node_id":"...","token":"<NODE_TOKEN>"}
   ```

5. Origin を登録  
   ```bash
   curl -X POST http://localhost:8080/api/v1/origins \
     -H "Content-Type: application/json" \
     -d '{"name":"origin-1","wg_ip":"10.0.0.2"}'
   # レスポンスから origin_id を取得
   ```
   Origin インストールワンライナー（登録済みoriginのWG_IPを使う）:
   ```bash
   curl -fsSL http://localhost:8080/origin/install.sh | WG_ADDR=10.0.0.2/32 bash
   # もしくは UI の Origins リストに表示されるコマンドを利用
   ```

6. Route を登録  
   ```bash
   ORIGIN_ID=<origin-id>
   curl -X POST http://localhost:8080/api/v1/routes \
     -H "Content-Type: application/json" \
     -d "{\"hostname\":\"app.example.com\",\"origin_id\":\"${ORIGIN_ID}\",\"target_port\":8080}"
   ```

7. Edge エージェントを起動（テスト用）  
   ```bash
   CONTROL_PLANE_URL=http://localhost:8080 \
   NODE_TOKEN=<上で受け取った NODE_TOKEN> \
   CONFIG_DIR=/tmp/kokoa-nginx \
   EMAIL=you@example.com \
   scripts/kokoa-edge-agent/poll_config.sh
   ```
   `CONFIG_DIR` 配下に `map.conf` と `config_hash` が配置される。nginx が入っていない場合は `NGINX_BIN=true` を指定してバリデーションをスキップ。

7. DNS スタブの利用（任意）  
   ```bash
   KOKOA_ZONE=example.com scripts/kokoa-dns/kokoa-dns.sh add --name app --ip 1.2.3.4
   scripts/kokoa-dns/kokoa-dns.sh list
   ```

よく使うコマンド:  
- `make test` — ユニットテスト  
- `make run` — ローカル起動（デフォルト: `:8080`）  
- `make docker-up` — Compose で起動  
- `make docker-down` — Compose を停止
- `scripts/dev/seed.sh` — CP起動後にEdge/Origin/Routeを自動登録してconfigを取得
