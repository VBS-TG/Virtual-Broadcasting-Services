# Console 後端 MVP-A — 手動驗證

本文件描述如何在本機或 Docker 中驗證 **healthz**、**JWT 發放**、**遙測 WebSocket** 與 **latest 快照**。

## 前置

- 環境變數（與 `docker-compose.console.yml` 預設對齊）：
  - `VBS_CONSOLE_JWT_SECRET`：HS256 簽章密鑰（必填）。
  - `VBS_CONSOLE_ADMIN_TOKEN`：發放 JWT 與查詢 `telemetry/latest` 的管理用共享密鑰（建議必填；未設定時 `/api/v1/auth/token` 會回 503）。
- 服務預設監聽 `:4000`（可改 `VBS_CONSOLE_HTTP_BIND`）。

## 1. 啟動

```bash
docker compose -f docker-compose.console.yml up --build
```

## 2. Health

```bash
curl -sS http://127.0.0.1:4000/healthz
# 預期：{"status":"ok"}
```

## 3. 取得節點 JWT

以管理密鑰發放短效 token（`node_id` / `role` 與 Route/Engine 節點類型一致：`route`、`engine`、`capture`、`console`；另可發 `admin` 僅供除錯）：

```bash
curl -sS -X POST http://127.0.0.1:4000/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -H "X-Console-Admin: dev-admin-token-change-me" \
  -d '{"node_id":"vbs-route-01","role":"route"}'
```

預期 JSON 含 `access_token`、`expires_in`。

將 `access_token` 設為環境變數（PowerShell 範例）：

```powershell
$tok = (Invoke-RestMethod -Method POST -Uri http://127.0.0.1:4000/api/v1/auth/token `
  -Headers @{ "X-Console-Admin"="dev-admin-token-change-me" } `
  -ContentType "application/json" `
  -Body '{"node_id":"vbs-route-01","role":"route"}').access_token
```

## 4. WebSocket 上報遙測

單筆 JSON **≤255 bytes**（與協定一致），且含 `node_id`、`node_type`、`ts_ms`、`metrics`。

範例 payload（請自行確認位元組長度 ≤255）：

```json
{"node_id":"vbs-route-01","node_type":"route","ts_ms":1712476800000,"metrics":{"cpu_pct":1,"mem_bytes":1000,"stream_ok":true},"auth_mode":"bearer"}
```

使用 `wscat`（或自寫腳本）連線時 **Upgrade 請求需帶**：

`Authorization: Bearer <access_token>`

（Node 端將 `VBS_ROUTE_JWT` / `VBS_ENGINE_JWT` 設為上述 token 即可與倉庫內 Route/Engine 遙測客戶端對接。）

## 5. 查詢最新快照

需帶管理密鑰 **或** `role=admin` 的 JWT：

```bash
curl -sS http://127.0.0.1:4000/api/v1/telemetry/latest \
  -H "X-Console-Admin: dev-admin-token-change-me"
```

預期：`latest` 物件內以 `node_id` 為鍵，含最近一次成功接收的欄位。

## 6. Route 整體對接

設定 Route 容器／行程：

- `VBS_CONSOLE_BASE_URL=http://127.0.0.1:4000`（或 Tunnel 後的 `https://api.example.com`）
- `VBS_ROUTE_JWT=<上一步取得的 access_token>`（或透過 admin 重新簽發）

確認 Route 日誌中遙測 WSS 連線成功，並以本節 **§5** 驗證快照。

---

**限制（MVP-A）**：無完整 RBAC、refresh token、持久化；生產環境請旋轉密鑰、縮短 TTL，並改以安全管道注入 `VBS_CONSOLE_JWT_SECRET` / `VBS_CONSOLE_ADMIN_TOKEN`。

---

## 驗證備註（開發／CI）

- 於安裝 **Go 1.22+** 的環境可執行：`go build -o console-server ./apps/console/cmd/console-server`（於 repo 根目錄）確認編譯通過。
- 於安裝 **Docker** 的環境依 **§1** 啟動後依序完成 **§2–§5** 即為計畫要求之最小介面驗證。
