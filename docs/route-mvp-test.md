# VBS-Route 部署與驗證說明（正式執行）

本文件說明在 AWS EC2（linux/amd64）上以正式設定驗證 Route 節點的建議步驟。

## 前置條件

- 已準備 **Console** 端可接受之 **HTTPS** 基底網址（`VBS_CONSOLE_BASE_URL`）及對應 **WSS** 遙測端點（見 `protocol.md`）。
- 已設定 Route 認證方式：  
  - 直接注入 **`VBS_ROUTE_JWT`**（或 `VBS_JWT`），或  
  - 提供 **`VBS_ROUTE_BOOTSTRAP_TOKEN`** 讓 Route 啟動後自動向 Console 換取短效 JWT。
- **`VBS_SRT_PASSPHRASE`** 長度 10–64 字元（全系統 SRT AES-256）。
- 安全群組／防火牆已放行 Route 埠區 UDP（預設 `20020`、`20030` 等）及控制面 TCP（預設 `20080`，若啟用）。

## 啟動

### 方式 A：使用 CI 推送之映像（建議於測試機）

1. 確認 `main` 已合併且 GitHub Actions **Publish VBS-Route (GHCR)** 成功，或已打標籤 `v*` 並完成建置。
2. `docker login ghcr.io`（需可讀取該套件之權限）。
3. 拉取映像（請將路徑換成你們實際的 owner/repo，小寫）：
   - `docker pull ghcr.io/vbs-tg/virtual-broadcasting-services/vbs-route:latest`
4. 以 `docker run --network host -e VBS_CONSOLE_BASE_URL=... -e VBS_ROUTE_JWT=... -e VBS_SRT_PASSPHRASE=... ... ghcr.io/.../vbs-route:latest`（或改用 `VBS_ROUTE_BOOTSTRAP_TOKEN`）啟動，亦可自行撰寫 compose **image:** 欄位指向上述映像。

### 方式 B：於建置機本地 build

1. 於專案根目錄設定環境變數（至少 `VBS_CONSOLE_BASE_URL`、`VBS_SRT_PASSPHRASE`，以及 `VBS_ROUTE_JWT` 或 `VBS_ROUTE_BOOTSTRAP_TOKEN`）。
2. 執行：`docker compose -f docker-compose.route.yml up --build -d`
3. 檢查日誌：應見 `[route] 啟動`、sysctl／可選 MTU、`[route][telemetry]` JSON，以及 WSS 上報成功或錯誤訊息。

## 健康檢查

- `GET http://<主機>:20080/healthz`（無需 Key）應回 `{"status":"ok"}`。
- 動態調整緩衝：`POST http://<主機>:20080/api/v1/route/buffer`，標頭 `Authorization: Bearer <JWT 或 bootstrap token>`，JSON 見 `protocol.md`。

## 串流驗證

1. 由 Capture 或測試端向 Route **SRTLA ingest**（預設 `20020`）送流。
2. 由 Engine 或測試端以 SRT Caller 連 **SRT 輸出**（預設 `20030`），`passphrase` 與全系統一致。

## 遙測

- 每秒至多一筆 JSON（≤255 bytes）送往 Console **WSS**，並寫入容器日誌。
- **ingest 停滯自癒**：預設在曾偵測到 ≥0.5 Mbps 後，若連續 5 秒近零 ingest，會觸發管線重啟；可設 `VBS_ROUTE_STALL_INGEST_SECONDS=0` 關閉。

完成以上項目後，即可視為 Route 節點在正式條件下可運作；後續再串接其他節點與 `/packages/shared` 協定。
