## VBS-Route 服務清單與通訊規範（正式執行）

**節點埠區（規劃）**：Capture `10010…`、Route `20010…`、Engine `30010…`、Console `40010…`。Route 資料平面預設使用 `20020`（SRTLA 入）、`20021`（內部 SRT）、`20030`（對 Engine SRT 出）；可經環境變數覆寫。

### 標準環境變數（Route 行程）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_SRT_PASSPHRASE` | 是 | 全系統 SRT AES-256 密鑰，長度 10–64 字元。 |
| `VBS_CONSOLE_BASE_URL` | 是 | Console 控制平面 HTTPS **原點**（例 `https://api.example.com`，建議不含路徑前綴）；若 Hub 在子路徑，請改以 `VBS_ROUTE_TELEMETRY_WS_PATH` 指定完整路徑。 |
| `VBS_CF_ACCESS_JWT` | 條件必填 | Route 對 Console telemetry 與控制面請求使用的 Cloudflare Access JWT（Bearer）；未提供時需改用 Service Token。 |
| `VBS_CF_ACCESS_CLIENT_ID` / `VBS_CF_ACCESS_CLIENT_SECRET` | 條件必填 | Route 對 Console telemetry 與控制面請求可用的 Cloudflare Service Token；未提供時需改用 `VBS_CF_ACCESS_JWT`。 |
| `VBS_CF_ACCESS_AUD` | 是 | Cloudflare Access JWT audience（Route 驗簽控制面請求用）。 |
| `VBS_CF_ACCESS_TEAM_DOMAIN` | 條件必填 | Cloudflare team domain（若未提供 `VBS_CF_ACCESS_JWKS_URL` 則必填）。 |
| `VBS_CF_ACCESS_JWKS_URL` | 條件必填 | Cloudflare JWKS URL（若未提供 `VBS_CF_ACCESS_TEAM_DOMAIN` 則必填）。 |
| `VBS_CF_JWKS_CACHE_TTL_SEC` | 否 | JWKS 記憶體快取秒數，預設 `3600`。 |
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
| 遙測 1Hz、單筆 JSON ≤255 bytes、WSS 上報（JWT 或 Service Token） | 已實作（上報為非阻塞 goroutine） |
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
| Route-Telemetry | WSS | 由 `VBS_CONSOLE_BASE_URL` + `VBS_ROUTE_TELEMETRY_WS_PATH` 衍生之 `wss://…` | `Authorization: Bearer <Cloudflare Access JWT>` **或** `Cf-Access-Client-Id/Secret` | Route → Console Hub |
| Route-Control-HTTP | HTTP | `http://<route>:20080`（預設，可關閉） | `/healthz` 無；其餘 **`Cf-Access-Client-Id`／`Cf-Access-Client-Secret`**（與節點 `VBS_CF_ACCESS_*` 一致）**或** `Cf-Access-Jwt-Assertion`／Bearer JWT（映射為 admin/operator） | Console orchestrator／維運 → Route |
| Route-Control-HTTP | HTTP | `POST /api/v1/show-config/apply` | 同上（Orchestrator 自動附 Service Token） | Console orchestrator → Route |

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

**Headers**：`Authorization: Bearer <JWT>`（Cloudflare 或 Console 簽發；需通過本地映射為 admin/operator），`Content-Type: application/json`

```json
{
  "latency_ms": 2000,
  "loss_max_ttl": 40
}
```

可僅送需要變更的欄位；成功後 Route 會重啟媒體管線以套用新參數。

### Route Runtime Config API

- `GET /api/v1/route/runtime/config`：回傳 Route 當前 runtime 配置（目前含 `inputs`、`pgm_count`、`aux_count`）。
- `POST /api/v1/route/runtime/config/apply`：套用 runtime 配置（`inputs:1..8`、`pgm_count:1`、`aux_count:0..4`）。

---

## VBS-Engine（`apps/engine`）

Engine 節點採 **Eyevinn Open Live 官方核心 + 本專案 Adapter**。  
若 Open Live 受限，次選 **Eyevinn 官方 Strom-based flow**，但契約仍由 Adapter 對外提供。  
本專案 **不再實作本地渲染/混流核心**；僅保留控制命令轉譯、權限驗證、健康檢查、Runtime 契約與回滾。  
**WSS 遙測**：已實作（1Hz，單筆 ≤255 bytes），支援 Cloudflare Access JWT 或 Service Token。

### 環境變數（Engine 容器）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_EYEVINN_OPENLIVE_BASE_URL` | 是 | Open Live 控制 API 根位址（例：`https://vbsrtc.cyblisswisdom.org`）。 |
| `VBS_EYEVINN_OPENLIVE_APPLY_PATH` | 否 | Open Live runtime apply 路徑，預設 `/api/v1/runtime/config/apply`。 |
| `VBS_EYEVINN_OPENLIVE_STATE_PATH` | 否 | Open Live switch state 路徑，預設 `/api/v1/switch/state`。 |
| `VBS_EYEVINN_OPENLIVE_HEALTH_PATH` | 否 | Open Live health 路徑，預設 `/healthz`。 |
| `VBS_EYEVINN_OPENLIVE_AUTH_TOKEN` | 否 | Adapter 呼叫 Open Live 時附帶的 Bearer token。 |
| `VBS_ENGINE_MIXER_WIDTH` | 否 | Open Live / Strom flow 的輸出解析度參數（若該官方流程支援）。 |
| `VBS_ENGINE_MIXER_HEIGHT` | 否 | Open Live / Strom flow 的輸出解析度參數（若該官方流程支援）。 |
| `VBS_ENGINE_CONTROL_BIND_HOST` / `VBS_ENGINE_CONTROL_BIND_PORT` | 否 | Engine adapter 控制 API 監聽位址（供 Console 呼叫）。 |
| `VBS_CF_ACCESS_JWT` | 條件必填 | Engine telemetry 與 Console guest introspect 請求可用的 Cloudflare Access JWT（Bearer）；未提供時需改用 Service Token。 |
| `VBS_CF_ACCESS_CLIENT_ID` / `VBS_CF_ACCESS_CLIENT_SECRET` | 條件必填 | Engine telemetry 與 Console guest introspect 可用的 Cloudflare Service Token；未提供時需改用 `VBS_CF_ACCESS_JWT`。 |
| `VBS_CF_ACCESS_AUD` | 是 | Cloudflare Access JWT audience（Single AUD，Engine 控制 API 驗簽用）。 |
| `VBS_CF_ACCESS_TEAM_DOMAIN` | 條件必填 | Cloudflare team domain（若未提供 `VBS_CF_ACCESS_JWKS_URL` 則必填）。 |
| `VBS_CF_ACCESS_JWKS_URL` | 條件必填 | Cloudflare JWKS URL（若未提供 `VBS_CF_ACCESS_TEAM_DOMAIN` 則必填）。 |
| `VBS_CF_JWKS_CACHE_TTL_SEC` | 否 | JWKS 記憶體快取秒數，預設 `3600`。 |
| `VBS_CONSOLE_JWT_PUBLIC_KEYS` | 條件必填 | 驗證 Console 簽發 Guest JWT 的公鑰集合（啟用 Guest 雙軌信任時必填）。 |
| `VBS_CONSOLE_BASE_URL` | 否 | 設定後啟用 Engine telemetry，以上報至 Console `wss://.../vbs/telemetry/ws`。 |
| `VBS_ENGINE_TELEMETRY_ENABLED` | 否 | 預設 `1`；設 `0` 關閉 telemetry。 |
| `VBS_ENGINE_TELEMETRY_WS_PATH` | 否 | 預設 `/vbs/telemetry/ws`。 |
| `VBS_METRICS_INTERVAL_SEC` | 否 | 預設 `1` 秒（1Hz）。 |
| `VBS_NODE_ID` | 否 | telemetry `node_id`，預設 `vbs-engine`。 |

### 埠（預設，Engine 埠區 `30010…`）

| 用途 | 預設 |
| :--- | :--- |
| Engine 控制 API（switch/program/preview/aux） | `5000` TCP（`network_mode: host` 時為主機 `5000`） |
| Engine 媒體資料平面 | 由 Open Live/Strom 官方核心管理（本 Adapter 不直接承載） |

### CI/CD

- Workflow：`.github/workflows/vbs-engine-publish.yml`
- 映像：`ghcr.io/<owner小寫>/<repo小寫>/vbs-engine`

### 控制 API（Engine Adapter）

- `POST /api/v1/switch/program`：切 Program 來源（body：`{"source":"input1..input8|srt://..."}`）。
- `POST /api/v1/switch/preview`：切 Preview 來源（body：同上）。
- `POST /api/v1/switch/aux`：切 AUX 路由（body：`{"channel":"1..4","source":"input1..input8|srt://..."}`）。
- `GET /api/v1/switch/state`：查目前 Program/Preview/AUX 狀態。
- `GET /api/v1/runtime/config`：查 Engine 當前 Runtime 配置（inputs/pgm_count/aux_count）。
- `POST /api/v1/runtime/config/apply`：套用 Runtime 配置（body 範例：`{"inputs":8,"pgm_count":1,"aux_count":4,"input_sources":["srt://..."],"aux_sources":{"1":"input1"}}`），由 adapter 轉譯到 Open Live（或 Strom 備援）官方配置格式。
- `POST /api/v1/show-config/apply`：套用 Show Config（body 與 `packages/shared/schemas/show-config.v1.schema.json`／`pkg/showconfig` 一致）；驗證後由 adapter 記錄並回傳 `applied`（後續可擴充：來源顯示名同步至 Open Live sources 等）。

### 部署

- 於 repo 根目錄建立 `.env.engine`（含 Open Live、Console、`VBS_CF_ACCESS_*` 等），再執行：`docker compose -f docker-compose.engine.yml up --build`（會自動載入 `.env.engine`，無需再於 shell 匯出同名變數）。若曾用舊版 compose 殘留容器（例如 nginx），可加 `--remove-orphans` 一併清掉。
- Engine 容器是 Open Live 官方核心的控制代理層（Adapter）；本專案不再依賴本地自建媒體核心鏈。
- Engine（rtc 子網域）：`docs/deploy/cloudflared-rtc.example.yml`
- Console/API（api 子網域）：`docs/deploy/cloudflared-api.example.yml`
- 重要原則：Tunnel 只代理信令，WebRTC 媒體走 ICE/UDP 直連。

---

## VBS-Console（`apps/console_backend`，MVP-A）

Console 為 **Cloudflare JWT 驗證閘道**、**遙測 WSS ingest**、**Runtime 配置編排中心（Orchestrator）** 與 **節點最新狀態內存快照** 的最小服務；預設 HTTP `:4000`，本倉庫 `docker-compose.console.yml` 將主機埠映射為 **5000**（避免與本機 4000 衝突）；對外可經 Cloudflare Tunnel（見 `docs/deploy/cloudflared-api.example.yml`）。

### 環境變數（Console 行程）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_CF_ACCESS_MODE` | 是 | 固定 `jwt`（ZTA 模式）。 |
| `VBS_CF_ACCESS_AUD` | 是 | Cloudflare Access JWT audience（Single AUD）。 |
| `VBS_CF_ACCESS_TEAM_DOMAIN` | 條件必填 | Cloudflare team domain（若未提供 `VBS_CF_ACCESS_JWKS_URL` 則必填）。 |
| `VBS_CF_ACCESS_JWKS_URL` | 條件必填 | Cloudflare JWKS URL（若未提供 `VBS_CF_ACCESS_TEAM_DOMAIN` 則必填）。 |
| `VBS_CF_JWKS_CACHE_TTL_SEC` | 否 | JWKS 記憶體快取秒數，預設 `3600`。 |
| `VBS_ADMIN_EMAILS` | 是 | Admin 本地映射白名單（Cloudflare JWT 驗簽後以 email 對名冊）。 |
| `VBS_NODE_CN_PREFIX` | 是 | 系統節點本地映射前綴（例如 `vbs-node-`；以 `common_name` 對規則）。 |
| `VBS_CONSOLE_JWT_ISSUER` | 是 | Console 簽發 Guest JWT 的 issuer。 |
| `VBS_CONSOLE_JWT_PRIVATE_KEY` | 是 | Console 簽發 Guest JWT 私鑰（建議非對稱）。 |
| `VBS_CONSOLE_JWT_PUBLIC_KEYS` | 是 | Console 與節點驗證 Guest JWT 的公鑰集合（支援輪替）。 |
| `VBS_GUEST_TOKEN_TTL_SEC` | 否 | Guest JWT 有效期（建議短效，如 `600` 秒）。 |
| `VBS_CONSOLE_HTTP_BIND` | 否 | 預設 `:4000`。 |
| `VBS_CONSOLE_TELEMETRY_MAX_BYTES` | 否 | 單筆 WS 訊息上限，預設 `255`（與 1Hz／≤255B 規範一致）。 |
| `VBS_CONSOLE_NODE_OFFLINE_TTL_SEC` | 否 | 節點離線判定秒數，預設 `10`（最小 `3`）。 |
| `VBS_RUNTIME_DB_PATH` | 否 | Runtime 快照 SQLite 路徑，預設 `data/console-runtime.db`。 |
| `VBS_SHOW_CONFIG_DB_PATH` | 否 | Show Config（製作規格）SQLite 路徑，預設 `data/console-show-config.db`。 |
| `VBS_ROUTE_CONTROL_BASE_URL` | 否 | Route 控制面基底 URL（Console orchestrator 轉發）；未設定時 Runtime／Show Config 之下發該跳為 skip。 |
| `VBS_ENGINE_CONTROL_BASE_URL` | 否 | Engine adapter 控制面基底 URL；未設定時對應之下發為 skip。 |
| `VBS_ROUTE_ACCESS_CLIENT_ID` / `VBS_ROUTE_ACCESS_CLIENT_SECRET` | 否 | Console → Route M2M（Cf-Access-Client-*）；未設定則請求無 Service Token（依部署而定）。 |
| `VBS_ENGINE_ACCESS_CLIENT_ID` / `VBS_ENGINE_ACCESS_CLIENT_SECRET` | 否 | Console → Engine M2M；同上。 |
| `VBS_CAPTURE_CONTROL_BASE_URL` | 否 | Capture 控制面基底 URL（Show Config apply／rollback 轉發）；未設定則該跳 skip。 |
| `VBS_CAPTURE_ACCESS_CLIENT_ID` / `VBS_CAPTURE_ACCESS_CLIENT_SECRET` | 否 | Console → Capture M2M；若留空則套用時退回使用 **Engine** 之 Client Id/Secret（便於同區 Cloudflare Service Token）。 |

> 配置邊界：`.env` 僅承載固定基礎參數（安全、定位、資源上限）；當天活動配置（IN 路數、PGM/AUX 路數、來源綁定）應由 Console Runtime API 下發與熱更新；**製作規格（Show Config：畫質政策、來源名、Switcher／Multiview 編排）**由 Show Config API 持久化與套用，**同樣不得**依賴改 `.env` 手動覆寫。

### 服務表（Console）

| Service / Port | Protocol | Endpoint | Auth Mode | Node Context |
| --- | --- | --- | --- | --- |
| Console-HTTP / 4000 | HTTP | `GET /healthz` | 無 | 健康檢查 |
| Console-Telemetry | WS/WSS | `GET /vbs/telemetry/ws`（Upgrade） | `Authorization: Bearer <Cloudflare Access JWT>`；驗簽後以 `common_name` 前綴映射為 node 身分 | Route/Engine/Capture → Console |
| Console-Telemetry-Events | WS/WSS | `GET /vbs/telemetry/events/ws`（Upgrade） | `Authorization: Bearer <JWT>`（Cloudflare 或 Console 簽發 Guest）；需映射為 admin/operator | Console → UI（online/offline 狀態事件） |
| Console-HTTP / 4000 | HTTP | `GET /api/v1/telemetry/latest` | `Authorization: Bearer <JWT>`（Cloudflare 或 Console 簽發 Guest）；需映射為 admin/operator | 讀取每節點最近一次遙測（內存 + presence） |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/stream/session-key` | `Authorization: Bearer <Cloudflare Access JWT>`；需映射為 admin | 生成當次直播 SRT AES-256 passphrase |
| Console-HTTP / 4000 | HTTP | `GET /api/v1/runtime/config` | `Authorization: Bearer <JWT>`（admin/operator） | 讀取今日 Runtime 配置 |
| Console-HTTP / 4000 | HTTP | `PUT /api/v1/runtime/config` | `Authorization: Bearer <JWT>`（admin） | 儲存今日 Runtime 配置 |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/runtime/config/apply` | `Authorization: Bearer <JWT>`（admin） | 以 staged apply 下發至 Route/Engine（失敗回退） |
| Console-HTTP / 4000 | HTTP | `GET /api/v1/show-config` | `Authorization: Bearer <JWT>`（admin／operator） | 讀取 Show Config **draft**、**effective** 與版本時間戳 |
| Console-HTTP / 4000 | HTTP | `PUT /api/v1/show-config/draft` | `Authorization: Bearer <JWT>`（admin／operator） | 儲存草稿（body 為完整 Show Config JSON；須與目前 `runtime.config.inputs` 交叉驗證） |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/show-config/apply` | `Authorization: Bearer <JWT>`（admin） | 依序嘗試 **Capture／Route／Engine** `POST /api/v1/show-config/apply`（若該基底 URL 未設定則 skip）；**全部成功或 skip** 後才將 draft 設為 Console **effective** 並寫入 history |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/show-config/rollback` | `Authorization: Bearer <JWT>`（admin） | 以上一版 history 快照回滾 effective，並再次轉發各節點 `POST /api/v1/show-config/apply` |
| Console-HTTP / 4000 | HTTP | `GET /api/v1/show-config/history?limit=50` | `Authorization: Bearer <JWT>`（admin／operator） | 列出套用紀錄（version、applied_at、downstream_result） |

### Show Config（節點契約，待各節點實作）

- **Console → 節點**：`POST /api/v1/show-config/apply`  
  - **Headers**：`Content-Type: application/json`；**必須**與下節「M2M Service Token」一致，附帶 `Cf-Access-Client-Id` / `Cf-Access-Client-Secret`（Console 已於 Orchestrator 自動附加）。  
  - **Body**：與 `packages/shared/schemas/show-config.v1.schema.json` 一致之 **Show Config JSON 本體**（與 Console `PUT /api/v1/show-config/draft` 相同形狀）。  
  - **成功**：HTTP 2xx；**失敗**：Console 不更新 effective（apply），rollback 時亦全失敗則不更新。  
- **節點未實作前**：請將對應之 `VBS_*_CONTROL_BASE_URL` **留空**，Console 僅在本地套用 effective／history（不下發 HTTP）。

### Console ↔ Route／Engine 控制面 M2M（免手動 JWT）

- **目的**：Orchestrator（Console 後端）對 Route／Engine 的 **Runtime apply、Show Config apply** 等請求，只應依賴 **環境變數內已配置的 Cloudflare Access Service Token**，不需人工複製 `Cf-Access-Jwt-Assertion`。
- **Console 端**：使用既有 `VBS_ROUTE_ACCESS_CLIENT_ID`／`SECRET`、`VBS_ENGINE_ACCESS_CLIENT_ID`／`SECRET`（見 `apps/console_backend/internal/config`），請求時自動帶入 **`Cf-Access-Client-Id`**、**`Cf-Access-Client-Secret`**。
- **Route／Engine 端**：控制面除驗證 **`Cf-Access-Jwt-Assertion`**（JWT）外，另支援 **與本節點 `VBS_CF_ACCESS_CLIENT_ID`／`VBS_CF_ACCESS_CLIENT_SECRET` 完全一致**（常數時間比對）之 Service Token；通過即視為已授權（與 Console 對該節點設定的 Access 憑證為 **同一組** 即可）。
- **部署約定（上線一次即可）**：  
  - Route 容器：`VBS_CF_ACCESS_CLIENT_ID`／`SECRET` **＝** Console 的 **`VBS_ROUTE_ACCESS_CLIENT_ID`／`SECRET`**（呼叫 Route 控制面專用 Token）。  
  - Engine 容器：`VBS_CF_ACCESS_CLIENT_ID`／`SECRET` **＝** Console 的 **`VBS_ENGINE_ACCESS_CLIENT_ID`／`SECRET`**。  
  - 節點對 Console 遙測出站若沿用同一組 Token，無需額外人工步驟。

### JWT（ZTA）

- 外部門禁：Cloudflare Access Single AUD（Admin/Nodes）。
- 內部臨時授權：Console 可簽發短效 Guest JWT（PIN/魔術連結對應租約）。
- 驗證方式：Cloudflare JWT 以 JWKS 驗簽（記憶體快取預設 1h）；Console JWT 以本地公鑰驗簽。
- 權限決策：先驗 `iss/aud/exp/nbf`，再以本地名冊/規則映射（admin / node / guest-operator）。
- 去註冊化：節點連線即驗證、驗證通過即更新狀態，不再有 register/refresh API。

### 本機驗證

- 建立 `.env.console` 後：`docker compose -f docker-compose.console.yml up --build`（會自動載入該檔）。
- 步驟見 `docs/console-deploy-and-verify.md`。

---

## 共用 Schema（單一真相）

- `packages/shared/schemas/telemetry.v1.schema.json`
- `packages/shared/schemas/control.route-buffer.v1.schema.json`
- `packages/shared/schemas/node-status.v1.schema.json`
- `packages/shared/schemas/console-telemetry-latest.v1.schema.json`（Console `GET /api/v1/telemetry/latest` 回應快照，MVP-A）
- `packages/shared/schemas/control.engine-switch.v1.schema.json`
- `packages/shared/schemas/show-config.v1.schema.json`（Show Config／製作規格）

新增或修改跨節點封包時，必須先更新上述 schema，再更新各節點實作。

---

## CI/CD 與治理

- 發布 workflow：
  - `.github/workflows/vbs-route-publish.yml`
  - `.github/workflows/vbs-engine-publish.yml`
  - `.github/workflows/vbs-console-publish.yml`
- 協定治理 workflow：`.github/workflows/protocol-governance.yml`
  - 若 PR 變更 `protocol.md`，必須同步調整 `packages/shared/schemas/**`；反向亦同。
