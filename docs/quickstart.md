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

3. Edge ノードを登録（ブートストラップトークンを使用）  
   ```bash
   BOOT=yourtoken
   curl -X POST http://localhost:8080/api/v1/edge-nodes/register \
     -H "Authorization: Bearer ${BOOT}" \
     -H "Content-Type: application/json" \
     -d '{"name":"edge-1"}'
   # レスポンス: {"edge_node_id":"...","token":"<NODE_TOKEN>"}
   ```

4. Origin を登録  
   ```bash
   curl -X POST http://localhost:8080/api/v1/origins \
     -H "Content-Type: application/json" \
     -d '{"name":"origin-1","wg_ip":"10.0.0.2"}'
   # レスポンスから origin_id を取得
   ```

5. Route を登録  
   ```bash
   ORIGIN_ID=<origin-id>
   curl -X POST http://localhost:8080/api/v1/routes \
     -H "Content-Type: application/json" \
     -d "{\"hostname\":\"app.example.com\",\"origin_id\":\"${ORIGIN_ID}\",\"target_port\":8080}"
   ```

6. Edge エージェントを起動（テスト用）  
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
