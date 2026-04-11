## VBS-Route 服務清單與通訊規範（正式執行）

**節點埠區（規劃）**：Capture `10010…`、Route `20010…`、Engine `30010…`、Console `40010…`。Route 資料平面預設使用 `20020`（SRTLA 入）、`20021`（內部 SRT）、`20030`（對 Engine SRT 出）；可經環境變數覆寫。

### 標準環境變數（Route 行程）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_SRT_PASSPHRASE` | 是 | 全系統 SRT AES-256 密鑰，長度 10–64 字元。 |
| `VBS_CONSOLE_BASE_URL` | 是 | Console 控制平面 HTTPS **原點**（例 `https://api.example.com`，建議不含路徑前綴）；若 Hub 在子路徑，請改以 `VBS_ROUTE_TELEMETRY_WS_PATH` 指定完整路徑。 |
| `VBS_CF_ACCESS_CLIENT_ID` / `VBS_CF_ACCESS_CLIENT_SECRET` | 是 | Route 以 Cloudflare Access service token 呼叫 `POST /api/v1/auth/register` 自動換發/續約 JWT。 |
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
| 遙測 1Hz、單筆 JSON ≤255 bytes、WSS 上報（Bearer JWT） | 已實作（上報為非阻塞 goroutine） |
| ingest 停滯自癒（曾有流量後連續歸零達閾值秒數） | 已實作，可關閉 |
| `sysctl` rmem/wmem ≥16MB、可選 MTU | 已實作 |
| 控制面 HTTP：`/healthz`、`/api/v1/route/buffer`（需 Bearer） | 已實作 |
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

| Service / Port | Protocol | Endpoint / Topic | Auth Mode | Node Context |
| --- | --- | --- | --- | --- |
| Route-SRTLA-Ingest / 20020 | UDP/SRTLA | Listener | —（媒體層 passphrase） | Capture → Route |
| Route-SRT-Internal / 20021 | UDP/SRT | 本機 127.0.0.1 | — | Route 內部 |
| Route-SRT-Out / 20030 | UDP/SRT | `srt://<DNS 或域名>:20030` Listener | —（媒體層 passphrase） | Route → Engine |
| Route-Telemetry | WSS | 由 `VBS_CONSOLE_BASE_URL` + `VBS_ROUTE_TELEMETRY_WS_PATH` 衍生之 `wss://…` | `Authorization: Bearer <JWT>` | Route → Console Hub |
| Route-Control-HTTP | HTTP | `http://<route>:20080`（預設，可關閉） | `/healthz` 無；其餘 `Authorization: Bearer <JWT>` | Console / 維運 → Route |

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

**Headers**：`Authorization: Bearer <JWT>`，`Content-Type: application/json`

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
**WSS 遙測**：已實作（1Hz，Bearer JWT，單筆 ≤255 bytes），以 Cloudflare Access 向 Console 註冊後自動換發/續約 JWT。

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
| `VBS_CONSOLE_BASE_URL` | 否 | 設定後啟用 Engine telemetry，以上報至 Console `wss://.../vbs/telemetry/ws`。 |
| `VBS_CF_ACCESS_CLIENT_ID` / `VBS_CF_ACCESS_CLIENT_SECRET` | 是（啟用 telemetry 時） | Engine 以 Cloudflare Access service token 呼叫 `POST /api/v1/auth/register` 自動換發/續約 JWT。 |
| `VBS_ENGINE_TELEMETRY_ENABLED` | 否 | 預設 `1`；設 `0` 關閉 telemetry。 |
| `VBS_ENGINE_TELEMETRY_WS_PATH` | 否 | 預設 `/vbs/telemetry/ws`。 |
| `VBS_METRICS_INTERVAL_SEC` | 否 | 預設 `1` 秒（1Hz）。 |
| `VBS_NODE_ID` | 否 | telemetry `node_id`，預設 `vbs-engine`。 |

### 埠（預設，Engine 埠區 `30010…`）

| 用途 | 預設 |
| :--- | :--- |
| Brave HTTP / WebRTC 信令與內建 UI | `5000` TCP（`network_mode: host` 時為主機 `5000`） |
| 內部 PGM 橋接（ffmpeg→Brave TCP） | `30090` TCP（僅容器內，可不對外） |

### CI/CD

- Workflow：`.github/workflows/vbs-engine-publish.yml`
- 映像：`ghcr.io/<owner小寫>/<repo小寫>/vbs-engine`

### 部署

- 於 repo 根目錄建立 `.env.engine`（含 SRT URI、Console、`VBS_CF_ACCESS_*` 等），再執行：`docker compose -f docker-compose.engine.yml up --build`（會自動載入 `.env.engine`，無需再於 shell 匯出同名變數）。若曾用舊版 compose 殘留容器（例如 nginx），可加 `--remove-orphans` 一併清掉。
- 需 **NVIDIA Container Toolkit** 與主機驅動；映像基底為 `nvidia/cuda:12.2.0-runtime-ubuntu22.04`。
- Engine（rtc 子網域）：`docs/deploy/cloudflared-rtc.example.yml`
- Console/API（api 子網域）：`docs/deploy/cloudflared-api.example.yml`
- 重要原則：Tunnel 只代理信令，WebRTC 媒體走 ICE/UDP 直連。

---

## VBS-Console（`apps/console`，MVP-A）

Console 為 **JWT 簽發（測試／節點用）**、**遙測 WSS ingest** 與 **節點最新狀態內存快照** 的最小服務；預設 HTTP `:4000`，本倉庫 `docker-compose.console.yml` 將主機埠映射為 **5000**（避免與本機 4000 衝突）；對外可經 Cloudflare Tunnel（見 `docs/deploy/cloudflared-api.example.yml`）。

### 環境變數（Console 行程）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_CONSOLE_JWT_SECRET` | 是 | HS256 簽章密鑰；Route/Engine 所持 JWT 須由此密鑰簽出。 |
| `VBS_CONSOLE_ADMIN_TOKEN` | 強烈建議 | 管理密鑰；以 `Authorization: Bearer <token>` 呼叫 `POST /api/v1/auth/token` 取得 admin JWT。未設定時發證端點回 503。 |
| `VBS_CF_ACCESS_MODE` | 建議 | `service_token`（預設）；設為 `disabled` 時 `POST /api/v1/auth/register` 回 503。 |
| `VBS_CF_ACCESS_CLIENTS` | 是 | Cloudflare Access 許可清單（`node_id:role:client_id[:subject][:email]`），供節點註冊映射。 |
| `VBS_CONSOLE_HTTP_BIND` | 否 | 預設 `:4000`。 |
| `VBS_CONSOLE_JWT_TTL_SEC` | 否 | 簽發 token 有效期（秒），預設 `3600`，最小 `60`。 |
| `VBS_CONSOLE_TELEMETRY_MAX_BYTES` | 否 | 單筆 WS 訊息上限，預設 `255`（與 1Hz／≤255B 規範一致）。 |

### 服務表（Console）

| Service / Port | Protocol | Endpoint | Auth Mode | Node Context |
| --- | --- | --- | --- | --- |
| Console-HTTP / 4000 | HTTP | `GET /healthz` | 無 | 健康檢查 |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/auth/token` | `Authorization: Bearer <VBS_CONSOLE_ADMIN_TOKEN 或 admin JWT>` | 簽發節點 JWT（claims：`node_id`、`role`、`exp`） |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/auth/register` | Cloudflare Access（`CF-Access-Client-Id`/`CF-Access-Client-Secret`） | 節點自動註冊換發 JWT |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/auth/refresh` | `Authorization: Bearer <JWT>` | 節點 JWT 續約 |
| Console-Telemetry | WS/WSS | `GET /vbs/telemetry/ws`（Upgrade） | `Authorization: Bearer <JWT>`；`role` 須為 `capture`／`route`／`engine`／`console` | Route/Engine/Capture → Console |
| Console-HTTP / 4000 | HTTP | `GET /api/v1/telemetry/latest` | `Authorization: Bearer <VBS_CONSOLE_ADMIN_TOKEN 或 admin JWT>` | 讀取每節點最近一次遙測（內存） |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/stream/session-key` | `Authorization: Bearer <VBS_CONSOLE_ADMIN_TOKEN 或 admin JWT>` | 生成當次直播 SRT AES-256 passphrase |

### JWT（MVP-A）

- 演算法：`HS256`。
- 節點權杖：`role` 為上表節點類型或 `admin`（後者不可用於遙測 WS，僅可查詢／維運）。
- 客戶端：Route／Engine 執行期向 Console 註冊/續約取得 `access_token`，以 Bearer 連線 WSS 遙測。

### 本機驗證

- 建立 `.env.console` 後：`docker compose -f docker-compose.console.yml up --build`（會自動載入該檔）。
- 步驟見 `docs/console-deploy-and-verify.md`。

---

## 共用 Schema（單一真相）

- `packages/shared/schemas/telemetry.v1.schema.json`
- `packages/shared/schemas/control.route-buffer.v1.schema.json`
- `packages/shared/schemas/node-status.v1.schema.json`
- `packages/shared/schemas/console-telemetry-latest.v1.schema.json`（Console `GET /api/v1/telemetry/latest` 回應快照，MVP-A）

新增或修改跨節點封包時，必須先更新上述 schema，再更新各節點實作。

---

## CI/CD 與治理

- 發布 workflow：
  - `.github/workflows/vbs-route-publish.yml`
  - `.github/workflows/vbs-engine-publish.yml`
- 協定治理 workflow：`.github/workflows/protocol-governance.yml`
  - 若 PR 變更 `protocol.md`，必須同步調整 `packages/shared/schemas/**`；反向亦同。
