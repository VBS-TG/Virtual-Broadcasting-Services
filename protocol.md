## VBS-Route 服務清單與通訊規範（正式執行）

**節點埠區（規劃）**：Capture `10010…`、Route `20010…`、Engine `30010…`、Console `40010…`。Route 資料平面預設使用 `20020`（SRTLA 入）、`20021`（內部 SRT）、`20030`（對 Engine SRT 出）；可經環境變數覆寫。

### 標準環境變數（Route 行程）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_SRT_PASSPHRASE` | 是 | 全系統 SRT AES-256 密鑰，長度 10–64 字元。 |
| `VBS_CONSOLE_BASE_URL` | 是 | Console 控制平面 HTTPS **原點**（例 `https://api.example.com`，建議不含路徑前綴）；若 Hub 在子路徑，請改以 `VBS_ROUTE_TELEMETRY_WS_PATH` 指定完整路徑。 |
| `VBS_API_KEY` | 是 | 與 Console 約定之 API Key；WSS 與 REST 以標頭 `X-VBS-Key` 傳送。 |
| `VBS_NODE_ID` | 否 | 預設 `vbs-route-01`。 |
| `VBS_ROUTE_TELEMETRY_WS_PATH` | 否 | 預設 `/vbs/telemetry/ws`（相對於 Console 主機）。 |
| `VBS_METRICS_INTERVAL` | 否 | 預設 `1000ms`（1Hz）。 |
| `VBS_ROUTE_CONTROL_BIND` | 否 | 未設定時預設 `:20080`；設為空字串可關閉 HTTP 控制面。 |
| `VBS_ROUTE_STALL_INGEST_SECONDS` | 否 | 預設 `5`；`0` 表示停用 ingest 停滯自癒。 |
| `VBS_ROUTE_STALL_TRAFFIC_MBPS` | 否 | 判定「曾有顯著流量」之 Mbps 下限，預設 `0.5`。 |
| `VBS_ROUTE_TELEMETRY_TLS_INSECURE_SKIP_VERIFY` | 否 | 設為 `1` 或 `true` 僅供測試環境略過 TLS 校驗。 |
| `VBS_ROUTE_MTU_IFACE` / `VBS_ROUTE_MTU` | 否 | 若設定介面名，嘗試 `ip link set dev <iface> mtu <mtu>`（預設 MTU `1400`）。 |

### v1.2 合規對照（`apps/route`）

| 規範項目 | 狀態 |
| :--- | :--- |
| 公網 SRT、Passphrase 環境注入、長度 10–64 | 已實作 |
| 遙測 1Hz、單筆 JSON ≤255 bytes、WSS 上報、`X-VBS-Key` | 已實作（上報為非阻塞 goroutine） |
| ingest 停滯自癒（曾有流量後連續歸零達閾值秒數） | 已實作，可關閉 |
| `sysctl` rmem/wmem ≥16MB、可選 MTU | 已實作 |
| 控制面 HTTP：`/healthz`、`/api/v1/route/buffer`（需 `X-VBS-Key`） | 已實作 |
| Chrony/NTP、Cloudflare Tunnel | 主機／Console 側部署，非 Route 行程內 |
| `/packages/shared` 共用 Schema | 尚未建立（後續與其他節點一併導入） |

### CI/CD 與容器映像（對應 .cursorrules §7.1–§7.4）

- **Workflow**：`.github/workflows/vbs-route-publish.yml`
- **正式發布線（唯一部署映像來源）**：
  - **Merge / push 至 `main`**（且變更路徑含 `apps/route/**` 或 `go.mod` / `go.sum` 等）：推送 **`latest`** 與 **`sha-<短提交>`** 至 GHCR（§7.4「Merge to Main」）。
  - **推送標籤 `v*`**（例 `v1.2.0`）：僅當該標籤所指提交 **屬於 `origin/main` 歷程** 時才建置並推送語意化版本標籤（§7.3、§7.4「Tag Create」）；否則 workflow 失敗，不發布映像。
- **不觸發**：功能分支 **`feature/*`**、**`fix/*`** 等之 push **不會**執行本 workflow（§7.4：該類 push 僅應跑 Lint／測試／受影響模組編譯，由其他 workflow 負責）。
- **Registry**：`ghcr.io`。映像名稱 **`ghcr.io/<owner小寫>/<repo小寫>/vbs-route`**（例：`ghcr.io/vbs-tg/virtual-broadcasting-services/vbs-route`）。
- **機器上測試**：`docker login ghcr.io` 後 `docker pull .../vbs-route:latest`（或對應 **`v*`** 建置之版本標籤）。

### 服務表

| Service / Port | Protocol | Endpoint / Topic | `X-VBS-Key` | Node Context |
| --- | --- | --- | --- | --- |
| Route-SRTLA-Ingest / 20020 | UDP/SRTLA | Listener | —（媒體層 passphrase） | Capture → Route |
| Route-SRT-Internal / 20021 | UDP/SRT | 本機 127.0.0.1 | — | Route 內部 |
| Route-SRT-Out / 20030 | UDP/SRT | `srt://<DNS 或域名>:20030` Listener | —（媒體層 passphrase） | Route → Engine |
| Route-Telemetry | WSS | 由 `VBS_CONSOLE_BASE_URL` + `VBS_ROUTE_TELEMETRY_WS_PATH` 衍生之 `wss://…` | **是** | Route → Console Hub |
| Route-Control-HTTP | HTTP | `http://<route>:20080`（預設，可關閉） | 除 `/healthz` 外 **是** | Console / 維運 → Route |

### Route-Telemetry Payload

```json
{
  "node_id": "vbs-route-01",
  "cpu_percent": 12.3,
  "mem_bytes": 12345678,
  "total_ingest_mbps": 3.27,
  "reorder_error_pct": 12.51,
  "has_engine_client": true
}
```

### Route-Control：`POST /api/v1/route/buffer`

**Headers**：`X-VBS-Key: <VBS_API_KEY>`，`Content-Type: application/json`

```json
{
  "latency_ms": 2000,
  "loss_max_ttl": 40
}
```

可僅送需要變更的欄位；成功後 Route 會重啟媒體管線以套用新參數。

---

## VBS-Engine（`apps/engine`）

基於 [BBC Brave](https://github.com/bbc/brave)（GStreamer）：**2 路 SRT（`uri` 入）**、左右分割 **mixer**、**WebRTC** 監看（Brave 內建網頁/API，**非**標準 WHEP；標準 WHEP 可後續再換）、**TCP MPEG** 接 **ffmpeg** 再以 **SRT Caller** 輸出 **PGM**。  
**WSS 遙測**：本階段未實作，預留後續與 Route 一致之上報。

### 環境變數（Engine 容器）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_ENGINE_SRT_INPUT_1_URI` | 是 | 第一路 SRT Caller URI（例 `srt://route.example.com:20030?mode=caller&latency=2000&passphrase=...&pbkeylen=32`） |
| `VBS_ENGINE_SRT_INPUT_2_URI` | 是 | 第二路（測試可與第一路相同 URI 複製畫面） |
| `VBS_ENGINE_PGM_SRT_URI` | 是 | PGM 輸出之 SRT Caller 完整 URI（ffmpeg 以 mpegts 送出） |
| `VBS_ENGINE_MIXER_WIDTH` | 否 | 預設 `854`（約 480p 16:9 寬） |
| `VBS_ENGINE_MIXER_HEIGHT` | 否 | 預設 `480` |
| `VBS_ENGINE_PGM_TCP_PORT` | 否 | Brave 內部 TCP 伺服器埠，預設 `30090`（僅本機環，`ffmpeg` 連線用） |
| `PORT` / `VBS_ENGINE_API_PORT` | 否 | Brave REST/Web 預設 `5000` |
| `VBS_ENGINE_STUN_SERVER` | 否 | WebRTC 用，預設 `stun.l.google.com:19302` |

### 埠（預設，Engine 埠區 `30010…`）

| 用途 | 預設 |
| :--- | :--- |
| Brave HTTP / WebRTC 信令與內建 UI | `5000` TCP（`network_mode: host` 時為主機 `5000`） |
| 內部 PGM 橋接（ffmpeg→Brave TCP） | `30090` TCP（僅容器內，可不對外） |

### CI/CD

- Workflow：`.github/workflows/vbs-engine-publish.yml`
- 映像：`ghcr.io/<owner小寫>/<repo小寫>/vbs-engine`

### 部署

- `docker compose -f docker-compose.engine.yml --env-file .env.engine up --build`
- 需 **NVIDIA Container Toolkit** 與主機驅動；映像基底為 `nvidia/cuda:12.2.0-runtime-ubuntu22.04`。
