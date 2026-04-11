# VBS-Route 部署與驗證說明（正式執行）

## 前置條件

- 已準備 **Console** 之 HTTPS 基底網址（`VBS_CONSOLE_BASE_URL`）及對應 **WSS** 遙測端點（見 `protocol.md`）。
- **`VBS_CF_ACCESS_CLIENT_ID` + `VBS_CF_ACCESS_CLIENT_SECRET`**（Cloudflare Access service token）— 與 Console `VBS_CF_ACCESS_CLIENTS` 內該 `node_id` 映射一致。
- **`VBS_SRT_PASSPHRASE`** 長度 10–64 字元（全系統 SRT AES-256）。
- 安全群組／防火牆已放行 Route 埠區 UDP（預設 `20020`、`20030` 等）及控制面 TCP（預設 `20080`，若啟用）。

## 啟動

### 方式 A：使用 CI 推送之映像（建議於正式機）

1. 確認 `main` 已合併且 GitHub Actions **Publish VBS-Route (GHCR)** 成功。
2. `docker login ghcr.io`。
3. `docker pull ghcr.io/vbs-tg/virtual-broadcasting-services/vbs-route:latest`
4. 以 `docker run --network host` 並注入 `VBS_CONSOLE_BASE_URL`、`VBS_SRT_PASSPHRASE`、`VBS_CF_ACCESS_*` 等環境變數啟動。

### 方式 B：於建置機本地 build

1. 於專案根目錄準備 `.env.route`（變數見 `protocol.md`；Compose 會自動載入該檔）。
2. `docker compose -f docker-compose.route.yml up --build -d`
3. 檢查日誌：應見 `[route][telemetry]` JSON，且 WSS 上報不應持續認證失敗。

## 健康檢查

- `GET http://<主機>:20080/healthz`（無需 Key）應回 `{"status":"ok"}`。
- 動態調整緩衝：`POST http://<主機>:20080/api/v1/route/buffer`，標頭 `Authorization: Bearer <與 Route 目前所持 Console JWT 相同>`（Route 與 Console 以 CF Access 註冊取得之 JWT）。

## 串流驗證

1. 由 Capture 或測試端向 Route **SRTLA ingest**（預設 `20020`）送流。
2. 由 Engine 或測試端以 SRT Caller 連 **SRT 輸出**（預設 `20030`），`passphrase` 與全系統一致。

## 遙測

- 每秒至多一筆 JSON（≤255 bytes）送往 Console **WSS**。
- **ingest 停滯自癒**：預設在曾偵測到 ≥0.5 Mbps 後，若連續 5 秒近零 ingest，會觸發管線重啟；可設 `VBS_ROUTE_STALL_INGEST_SECONDS=0` 關閉。
