# Console 部署與驗證（ZTA / Human-Machine Separation）

本文件對齊發布版：Console 前台只走同源 API，Console 後端負責人機分離與節點編排。

## 前置

- 於 repo 根目錄建立 `.env.console`（Compose 會自動載入）。
- 至少設定：
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

## 3) 驗證管理端查詢（admin/guest JWT）

```bash
curl -sS http://127.0.0.1:5000/api/v1/telemetry/latest \
  -H "Authorization: Bearer <access-or-console-jwt>"
```

成功時回傳每個節點最新遙測與 `presence` 狀態。

## 4) 驗證節點遙測上報（node）

節點連到 `GET /vbs/telemetry/ws` 時可採其中一種：

- `Authorization: Bearer <cloudflare-access-jwt-with-node-identity>`
- `Cf-Access-Client-Id` + `Cf-Access-Client-Secret`

驗證通過後，Console 會更新節點快照並標記 `online`。

## 5) 驗證 Graceful Offline 事件

管理端（`role=admin`）訂閱：

```bash
wscat -c ws://127.0.0.1:5000/vbs/telemetry/events/ws \
  -H "Authorization: Bearer <cloudflare-or-console-jwt>"
```

當節點超過 `VBS_CONSOLE_NODE_OFFLINE_TTL_SEC` 未更新遙測時，會收到 `node_offline`；恢復後收到 `node_online`。

## 6) Route / Engine 對接重點

- Console 對 Route / Engine 控制面下發一律優先使用 `Cf-Access-Client-Id` / `Cf-Access-Client-Secret`（M2M）。
- `Cf-Access-Jwt-Assertion` 僅作控制面後備路徑，且節點端僅接受 `node` 身分。

## 備註

- 生產環境應由安全管道注入 Access JWT、Service Token 與對應策略。
- JWKS 採記憶體快取；未知 `kid` 會觸發立即刷新。
