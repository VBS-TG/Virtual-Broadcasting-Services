# Cloudflare BFF（同源閘道）

將 **`https://<SPA 網域>`** 上的 **`/api/*`、 `/vbs/*`、 `/whep/*`** 等轉發到各 **upstream**（見 `wrangler.toml` 的 `API_ORIGIN`、`RTC_ORIGIN`…），讓瀏覽器可用 **相對路徑** 與 **同源 WebSocket**，無需 CORS 開全網域。

## 必備路由（方案 A：同源 WSS）

若 SPA 主機名為 `vbs.cyblisswisdom.org`，Worker **必須**同時處理：

- `vbs.cyblisswisdom.org/api/*`
- **`vbs.cyblisswisdom.org/vbs/*`** ← 遙測 `wss://…/vbs/telemetry/ws` 依賴此條，否則請求會落到 **Pages** 回 **HTTP 200 HTML**，瀏覽器報 `Unexpected response code: 200`。

`wrangler.toml` 內已設定 `routes`；部署：

```bash
cd infra/cloudflare-bff
npx wrangler deploy
```

若 Dashboard 已手動建立相同 route，請避免與 Wrangler **重複綁定**（刪除重複項）。

## 行為摘要

- **`/vbs/*`、`/api/*` → `API_ORIGIN`**（Console／`vbsapi`）
- **`Upgrade: websocket`**：Worker 以 `fetch` 轉發上游，由執行環境處理 **101 Switching Protocols**
- **不**在 Worker 內解析或注入 JWT；**不**注入 `Cf-Access-Client-*`

## 檔案

- `worker.js` — 轉發邏輯  
- `wrangler.toml` — `vars`、`routes`
