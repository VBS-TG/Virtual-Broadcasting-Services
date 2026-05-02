# Console 部署與驗證（ZTA / Human-Machine Separation）

本文件對齊發布版：Console 前台只走同源 API（經 **BFF Worker 純轉發** 至 `vbsapi`），Console 後端負責應用層身分與節點編排；**Cloudflare Access** 負責網路層連線資格。

## 前置

- 於 repo 根目錄建立 `.env.console`（或以 `env/.env.console` 內容覆寫）。
- 至少設定：
  - `VBS_CF_ACCESS_AUD=<cloudflare-access-audience>`
  - `VBS_CF_ACCESS_TEAM_DOMAIN=<team>.cloudflareaccess.com`（或改填 `VBS_CF_ACCESS_JWKS_URL`）
- 建議設定：
  - `VBS_CF_JWKS_CACHE_TTL_SEC=3600`
  - `VBS_CONSOLE_NODE_OFFLINE_TTL_SEC=10`
  - `VBS_CONSOLE_TELEMETRY_MAX_BYTES=255`
  - `VBS_NTP_CHECK_URL=https://vbsapi.cyblisswisdom.org/healthz`
  - `VBS_NTP_MAX_SKEW_SEC=5`
  - `VBS_NTP_ENFORCE=1`
  - `VBS_JWT_CLOCK_SKEW_SEC=30`

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

瀏覽器／前台正式慣例為 **`X-VBS-Authorization: <raw JWT>`**（無 `Bearer`）。`curl` 驗證範例：

```bash
curl -sS http://127.0.0.1:5000/api/v1/telemetry/latest \
  -H "X-VBS-Authorization: <console-or-cloudflare-jwt>"
```

（後端仍相容 `Authorization: Bearer`，但請勿與前台規格混用。）

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
  -H "X-VBS-Authorization: <console-or-cloudflare-jwt>"
```

當節點超過 `VBS_CONSOLE_NODE_OFFLINE_TTL_SEC` 未更新遙測時，會收到 `node_offline`；恢復後收到 `node_online`。

## 6) Route / Engine 對接重點

- Console 對 Route / Engine 控制面下發一律優先使用 `Cf-Access-Client-Id` / `Cf-Access-Client-Secret`（M2M）。
- `Cf-Access-Jwt-Assertion` 僅作控制面後備路徑，且節點端僅接受 `node` 身分。

## 備註

- 生產環境應由安全管道注入 Access JWT、Service Token 與對應策略。
- JWKS 採記憶體快取；未知 `kid` 會觸發立即刷新。
- 上線前需確認主機 NTP 同步狀態正常（chrony/systemd-timesyncd），避免 JWT `nbf/exp` 因時鐘漂移失敗。
