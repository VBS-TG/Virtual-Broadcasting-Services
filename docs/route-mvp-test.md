# VBS-Route MVP 測試流程說明

本文件描述在 MVP 階段驗證 Route 節點的建議測試步驟，實際操作時請依部署環境調整指令。

## 前置條件

- 已在 AWS EC2（linux/amd64）節點部署本專案程式碼。
- 節點已加入 Tailscale，並可透過 MagicDNS 由其他節點存取。
- 已安裝 Docker 與 Docker Compose。

## 啟動 Route 節點

1. 在 EC2 節點上進入專案根目錄。
2. 設定必要環境變數（例）：
   - `VBS_SRT_PASSPHRASE`：SRT/SRTLA AES-256 passphrase。
3. 以 docker-compose 啟動：
   - `docker-compose -f docker-compose.route.yml up --build -d`

預期結果：

- 服務啟動後，在容器 log 中可看到 Route 啟動訊息與 sysctl 套用結果。
- 每 `VBS_METRICS_INTERVAL` 週期可看到一行 `[route][telemetry] { ... }` JSON。

## 模擬 Capture 端推送 SRTLA/SRT 流

> 視實際工具而定，此處僅提供概念示意。

1. 在測試機或 Capture 節點，使用對應的 SRTLA Sender 或 ffmpeg/srt-live-transmit 對 `VBS-Route` 的 `Route-SRTLA-Ingest / 10020` 發送測試流。
2. 確認網路安全性與防火牆已允許對應 UDP 連線。

## 從 Engine 端驗證 SRT 輸出

1. 在 Engine 節點或任一可達到 Route 的節點上，使用 SRT Caller 連線：
   - 目標：`srt://vbs-route:10030?mode=caller`
2. 可使用支援 SRT 的播放器（如 ffplay、VLC、OBS 等）確認是否能看到連續影像或測試圖樣。

預期結果：

- 當 Capture 端持續送流時，SRT Caller 能穩定接收到視訊訊號。
- 若中斷 Capture 端，Route 不應崩潰，待恢復推流後，SRT Caller 能重新看到畫面。

## Watchdog 行為驗證

1. 進入 Route 容器的 shell。
2. 手動終止由 Route 啟動的 SRTLA/SRT 子進程（例如使用 `kill` 指令）。
3. 觀察 Route 容器 log：
   - 應可看到 pipeline 結束的錯誤訊息。
   - 並在一段退避時間後重新啟動該子進程。

## 遙測輸出驗證

1. 透過 `docker logs vbs-route` 觀察 `[route][telemetry]` 行。
2. 驗證：
   - JSON 格式正確，包含 `node_id`、`mem_bytes` 等欄位。
   - 單行長度不超過 255 bytes（可視覺檢查或另行統計）。

以上測試完成後，即可確認 Route MVP 在「SRTLA 輸入 → SRT 輸出」、「基本自癒能力」以及「本地遙測輸出」三個面向上達到預期行為，後續即可再接上 Console WebSocket Hub 與更進階的參數調整功能。
