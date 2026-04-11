# Console 後端部署與驗證說明

本文件描述如何驗證 **healthz**、**Cloudflare Access 節點註冊／JWT 續約**、**遙測 WebSocket** 與 **latest 快照**。

## 前置

- `VBS_CONSOLE_JWT_SECRET`：HS256 簽章密鑰（必填）。
- `VBS_CONSOLE_ADMIN_TOKEN`：管理用共享密鑰（強烈建議；未設定時 `POST /api/v1/auth/token` 回 503）。
- `VBS_CF_ACCESS_MODE=service_token` 與 `VBS_CF_ACCESS_CLIENTS`：節點註冊映射（必填）。
- 服務進程預設監聽 `:4000`；`docker-compose.console.yml` 將主機埠映射為 **5000**。

## 1. 啟動

```bash
docker compose -f docker-compose.console.yml up --build
```

## 2. Health

```bash
curl -sS http://127.0.0.1:5000/healthz
# 預期：{"status":"ok"}
```

## 3. 取得 JWT（管理者，除錯／維運）

```bash
curl -sS -X POST http://127.0.0.1:5000/api/v1/auth/token \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer <VBS_CONSOLE_ADMIN_TOKEN>" \
  -d '{"node_id":"vbs-route-01","role":"route"}'
```

## 4. 節點註冊（Cloudflare Access）

```bash
curl -sS -X POST http://127.0.0.1:5000/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -H "CF-Access-Client-Id: <與 VBS_CF_ACCESS_CLIENTS 一致之 client id>" \
  -H "CF-Access-Client-Secret: <對應 secret>" \
  -H "X-VBS-Node-ID: vbs-route-01" \
  -d '{"node_id":"vbs-route-01","role":"route"}'
```

續約：

```bash
curl -sS -X POST http://127.0.0.1:5000/api/v1/auth/refresh \
  -H "Authorization: Bearer <old_jwt>"
```

## 5. WebSocket 遙測

Upgrade 請求帶 `Authorization: Bearer <access_token>`；單筆 JSON ≤255 bytes（見 `protocol.md`）。

## 6. 查詢最新快照

```bash
curl -sS http://127.0.0.1:5000/api/v1/telemetry/latest \
  -H "Authorization: Bearer <VBS_CONSOLE_ADMIN_TOKEN 或 admin JWT>"
```

## 7. Route 對接

- `VBS_CONSOLE_BASE_URL=http://127.0.0.1:5000`（或 Tunnel 後的 `https://api.example.com`）
- `VBS_CF_ACCESS_CLIENT_ID` / `VBS_CF_ACCESS_CLIENT_SECRET`（與本 Console 的 `VBS_CF_ACCESS_CLIENTS` 映射一致）

---

**限制**：無完整 RBAC、持久化；生產環境請以安全管道注入密鑰並縮短 TTL。

---

## 驗證備註

- 有 **Go 1.22+** 時：`go build -o console-server ./apps/console/cmd/console-server`（於 repo 根目錄）。
