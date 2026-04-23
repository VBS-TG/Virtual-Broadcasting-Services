# Console 部署與驗證（ZTA / Cloudflare JWT-only）

本文件只適用於新版 ZTA：**所有 API 與 WS 一律使用 Cloudflare Access JWT**，Console 不再提供 `register` / `refresh` / `token mint` 端點。

## 前置

- 於 repo 根目錄建立 `.env.console`（Compose 會自動載入）。
- 至少設定：
  - `VBS_CF_ACCESS_MODE=jwt`
  - `VBS_CF_ACCESS_AUD=<cloudflare-access-audience>`
  - `VBS_CF_ACCESS_TEAM_DOMAIN=<team>.cloudflareaccess.com`（或改填 `VBS_CF_ACCESS_JWKS_URL`）
- 建議設定：
  - `VBS_CF_JWKS_CACHE_TTL_SEC=3600`
  - `VBS_CONSOLE_NODE_OFFLINE_TTL_SEC=10`
  - `VBS_CONSOLE_TELEMETRY_MAX_BYTES=255`

## 1) 啟動 Console

```bash
docker compose -f docker-compose.console.yml up --build
```

## 2) Health 檢查

```bash
curl -sS http://127.0.0.1:5000/healthz
# 預期：{"status":"ok"}
```

## 3) 驗證管理端查詢（admin JWT）

```bash
curl -sS http://127.0.0.1:5000/api/v1/telemetry/latest \
  -H "Authorization: Bearer <cloudflare-access-jwt-with-role-admin>"
```

成功時回傳每個節點最新遙測與 `presence` 狀態。

## 4) 驗證節點遙測上報（node JWT）

節點需連到 `GET /vbs/telemetry/ws`，Upgrade 時攜帶：

- `Authorization: Bearer <cloudflare-access-jwt-with-role-route|engine|capture|console>`

驗證通過後，Console 會更新節點快照並標記 `online`。

## 5) 驗證 Graceful Offline 事件

管理端（`role=admin`）訂閱：

```bash
wscat -c ws://127.0.0.1:5000/vbs/telemetry/events/ws \
  -H "Authorization: Bearer <cloudflare-access-jwt-with-role-admin>"
```

當節點超過 `VBS_CONSOLE_NODE_OFFLINE_TTL_SEC` 未更新遙測時，會收到 `node_offline` 事件；恢復後會收到 `node_online`。

## 6) Route / Engine 對接重點

- Route / Engine 對 Console 一律帶 `Authorization: Bearer <VBS_CF_ACCESS_JWT>`。
- Route / Engine 控制面由 Console 代理時，Console 會轉發呼叫者原始 JWT（不再使用任何固定 control token）。

## 備註

- 生產環境應由安全管道注入 `VBS_CF_ACCESS_JWT` 與 Access 策略。
- JWKS 採記憶體快取，避免每請求拉取公鑰；未知 `kid` 會觸發立即刷新。
