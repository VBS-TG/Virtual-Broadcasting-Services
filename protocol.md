## VBS 通訊規範與部署清單（v1.2 對齊版）

**節點埠區（規劃）**：Capture `10010…`、Route `20010…`、Engine `30010…`、Console `40010…`。

## 認證遷移策略（JWT/Bearer）

目前倉庫既有功能仍含 `X-VBS-Key`；依 `.cursorrules` 目標改為 JWT/Bearer，採三階段遷移：

| 階段 | 時程 | 允許 Header | 說明 |
| --- | --- | --- | --- |
| Phase 0（現在） | 立即 | `X-VBS-Key` + `Authorization: Bearer` | 雙軌相容，所有新 API 必須優先支援 Bearer。 |
| Phase 1（切換） | 下一個 minor 版 | `Authorization: Bearer` 為預設、`X-VBS-Key` 僅 Legacy | 文件與 Console 介面預設顯示 Bearer；Legacy 需明確標註。 |
| Phase 2（退場） | 下下個 minor 版 | `Authorization: Bearer` | 移除 `X-VBS-Key` 驗證路徑與環境變數依賴。 |

- JWT 規格與授權矩陣：`docs/security/jwt-migration.md`。
- 本文件中的服務表已新增 `Auth Mode` 欄位，便於追蹤退場節點。

---

## VBS-Route（`apps/route`）

### 標準環境變數（Route 行程）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_SRT_PASSPHRASE` | 是 | 全系統 SRT AES-256 密鑰，長度 10–64 字元。 |
| `VBS_CONSOLE_BASE_URL` | 是 | Console 控制平面 HTTPS 原點（例 `https://api.example.com`）。 |
| `VBS_ROUTE_JWT`（或 `VBS_JWT`） | 是 | Route JWT；控制面與遙測均使用 `Authorization: Bearer <JWT>`。 |
| `VBS_NODE_ID` | 否 | 預設 `vbs-route-01`。 |
| `VBS_ROUTE_TELEMETRY_WS_PATH` | 否 | 預設 `/vbs/telemetry/ws`。 |
| `VBS_METRICS_INTERVAL` | 否 | 預設 `1000ms`（1Hz）。 |
| `VBS_ROUTE_CONTROL_BIND` | 否 | 預設 `:20080`；空字串關閉控制面。 |

### v1.2 合規對照（`apps/route`）

| 規範項目 | 狀態 |
| :--- | :--- |
| 公網 SRT + AES-256 + passphrase 注入 | 已實作 |
| 1Hz 遙測、單筆 JSON ≤255 bytes、WSS 上報 | 已實作 |
| ingest 停滯自癒、指數退避重啟 | 已實作 |
| `sysctl` rmem/wmem ≥16MB、可選 MTU | 已實作 |
| `/packages/shared` 共用 schema | 已建立草案（`packages/shared/schemas`） |

### 服務表（Route）

| Service / Port | Protocol | Endpoint / Topic | Auth Mode | Node Context |
| --- | --- | --- | --- | --- |
| Route-SRTLA-Ingest / 20020 | UDP/SRTLA | Listener | 媒體層 passphrase | Capture → Route |
| Route-SRT-Internal / 20021 | UDP/SRT | `127.0.0.1` | 無（內部） | Route 內部 |
| Route-SRT-Out / 20030 | UDP/SRT | `srt://<dns>:20030` Listener | 媒體層 passphrase | Route → Engine |
| Route-Telemetry | WSS | `wss://<console>/vbs/telemetry/ws` | Bearer JWT | Route → Console |
| Route-Control-HTTP / 20080 | HTTP | `/healthz`、`/api/v1/route/buffer` | `/healthz` 無；其餘 Bearer JWT | Console / 維運 → Route |

### Route-Telemetry Payload（暫行）

```json
{
  "node_id": "vbs-route-01",
  "node_type": "route",
  "ts_ms": 1712476800000,
  "metrics": {
    "cpu_pct": 12.3,
    "mem_bytes": 12345678,
    "ingest_mbps": 3.27,
    "reorder_error_pct": 12.51,
    "has_engine_client": true,
    "stream_ok": true
  },
  "auth_mode": "bearer"
}
```

---

## VBS-Engine（`apps/engine`）

基於 BBC Brave（GStreamer）：2 路 SRT 輸入、mixer 合成、WebRTC 監看、PGM 以 SRT 輸出。

### Engine 監看信令：現況 vs 目標

| 項目 | 現況（已上線） | 目標（規範） |
| --- | --- | --- |
| 信令入口 | Brave HTTP/Web UI（`/`、`/api/*`） | WHEP HTTP（`/whep`） |
| 對外埠口 | `5000/tcp` | 可沿用 `5000/tcp` 或獨立 WHEP 埠 |
| Tunnel 用途 | 只代理 HTTP/WebSocket 信令 | 同左 |
| 媒體路徑 | WebRTC ICE/UDP 直連（非 Tunnel 搬運） | 同左 |
| STUN/TURN | STUN 可用（`VBS_ENGINE_STUN_SERVER`） | 補齊 TURN 以提高對稱 NAT 成功率 |

### 環境變數（Engine）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_ENGINE_SRT_INPUT_1_URI` | 是 | 第一路 SRT Caller URI。 |
| `VBS_ENGINE_SRT_INPUT_2_URI` | 是 | 第二路 SRT Caller URI。 |
| `VBS_ENGINE_PGM_SRT_URI` | 是 | PGM 輸出 SRT Caller URI。 |
| `PORT` / `VBS_ENGINE_API_PORT` | 否 | Brave HTTP / 信令埠，預設 `5000`。 |
| `VBS_ENGINE_API_HOST` | 否 | Brave 綁定位址，預設 `0.0.0.0`。 |
| `VBS_ENGINE_STUN_SERVER` | 否 | STUN（亦寫入 `STUN_SERVER`），預設 `stun.l.google.com:19302`。 |
| `VBS_ENGINE_TURN_SERVER` | 否 | TURN（Brave 格式 `user:pass@host:port`；亦寫入 `TURN_SERVER`）。 |
| `VBS_ENGINE_REQUIRE_NVIDIA` | 否 | 預設 `1`：啟動前必須 `nvidia-smi` 成功，設 `0` 僅供無 GPU 除錯。 |
| `VBS_ENGINE_REQUIRE_GST_NVH265DEC` | 否 | 預設 `0`；設 `1` 時強制 `gst-inspect nvh265dec` 存在。 |
| `VBS_ENGINE_MTU_IFACE` / `VBS_ENGINE_MTU` | 否 | 若兩者皆設定，啟動時嘗試 `ip link set dev <iface> mtu <mtu>`（建議 `1400`）。 |
| `VBS_ENGINE_RESTART_INITIAL_SEC` | 否 | 管線崩潰後首次重啟延遲秒數，預設 `1`。 |
| `VBS_ENGINE_RESTART_BACKOFF_MAX_SEC` | 否 | 指數退避上限秒數，預設 `30`。 |
| `VBS_CONSOLE_BASE_URL` | 否 | 若設定且 `VBS_ENGINE_TELEMETRY_ENABLED` 非 `0`，啟動 1Hz WSS 遙測。 |
| `VBS_ENGINE_TELEMETRY_WS_PATH` | 否 | 遙測 WebSocket 路徑，預設 `/vbs/telemetry/ws`（相對 Console 主機）。 |
| `VBS_ENGINE_TELEMETRY_ENABLED` | 否 | 預設 `1`；設 `0` 關閉遙測子進程。 |
| `VBS_NODE_ID` | 否 | 遙測 `node_id`，預設 `vbs-engine`。 |
| `VBS_METRICS_INTERVAL` | 否 | 遙測間隔，預設 `1000ms`（1Hz）。 |
| `VBS_API_KEY` | 否 | 遙測 Phase 0：Header `X-VBS-Key`（若未設定 `VBS_ENGINE_JWT`）。 |
| `VBS_ENGINE_JWT` | 否 | 遙測 Phase 0：Header `Authorization: Bearer`（優先於 API Key）。 |

### 埠（Engine）

| 用途 | 預設 |
| :--- | :--- |
| Brave HTTP + WebRTC 信令 + UI | `5000` TCP（host network 下即主機 5000） |
| 內部 PGM 橋接（ffmpeg→Brave） | `30090` TCP（內部） |

### v1.2 合規對照（`apps/engine`）

| 規範項目 | 狀態 |
| :--- | :--- |
| NVIDIA fail-fast（`nvidia-smi`） | 已實作（`entrypoint.sh`） |
| WebRTC STUN／TURN（Brave `webrtcbin`） | 已實作（環境變數＋`generate_brave_config.py`） |
| 1Hz 遙測、payload 符合 `telemetry.v1.schema.json` | 已實作（可選，`engine_telemetry.py`） |
| 管線自癒（指數退避重啟） | 已實作（Brave＋ffmpeg） |
| 可選 MTU `1400` | 已實作（`VBS_ENGINE_MTU_*`） |
| 解碼鏈 `nvh265dec`→`glupload`→`gles2` | 未強制（Brave 預設 pipeline；見 `schemas/engine-deployment.md`） |
| Brave HTTP JWT 中介 | 未實作（建議 Cloudflare Tunnel／Access 或專用 sidecar 邊界驗證） |

### Cloudflare Tunnel 範本

- Engine（rtc 子網域）：`docs/deploy/cloudflared-rtc.example.yml`
- Console/API（api 子網域）：`docs/deploy/cloudflared-api.example.yml`
- 重要原則：Tunnel 只代理信令，WebRTC 媒體走 ICE/UDP 直連。

---

## VBS-Console（`apps/console`，MVP-A）

Console 為 **JWT 簽發（測試／節點用）**、**遙測 WSS ingest** 與 **節點最新狀態內存快照** 的最小服務；預設 HTTP `:4000`，對外可經 Cloudflare Tunnel（見 `docs/deploy/cloudflared-api.example.yml`）。

### 環境變數（Console 行程）

| 變數 | 必填 | 說明 |
| :--- | :--- | :--- |
| `VBS_CONSOLE_JWT_SECRET` | 是 | HS256 簽章密鑰；Route/Engine 所持 JWT 須由此密鑰簽出。 |
| `VBS_CONSOLE_ADMIN_TOKEN` | 強烈建議 | 發放 JWT（`POST /api/v1/auth/token`）與查詢 `GET /api/v1/telemetry/latest`（`X-Console-Admin` 或 Bearer 等值）之共享密鑰；未設定時發證端點回 503。 |
| `VBS_CONSOLE_HTTP_BIND` | 否 | 預設 `:4000`。 |
| `VBS_CONSOLE_JWT_TTL_SEC` | 否 | 簽發 token 有效期（秒），預設 `3600`，最小 `60`。 |
| `VBS_CONSOLE_TELEMETRY_MAX_BYTES` | 否 | 單筆 WS 訊息上限，預設 `255`（與 1Hz／≤255B 規範一致）。 |

### 服務表（Console）

| Service / Port | Protocol | Endpoint | Auth Mode | Node Context |
| --- | --- | --- | --- | --- |
| Console-HTTP / 4000 | HTTP | `GET /healthz` | 無 | 健康檢查 |
| Console-HTTP / 4000 | HTTP | `POST /api/v1/auth/token` | `X-Console-Admin` 或 `Authorization: Bearer <admin token>` | 簽發節點 JWT（claims：`sub`=node_id、`role`、`exp`） |
| Console-Telemetry | WS/WSS | `GET /vbs/telemetry/ws`（Upgrade） | `Authorization: Bearer <JWT>`；`role` 須為 `capture`／`route`／`engine`／`console` | Route/Engine/Capture → Console |
| Console-HTTP / 4000 | HTTP | `GET /api/v1/telemetry/latest` | `X-Console-Admin` 或 Bearer admin JWT | 讀取每節點最近一次遙測（內存） |

### JWT（MVP-A）

- 演算法：`HS256`。
- 節點權杖：`role` 為上表節點類型或 `admin`（後者不可用於遙測 WS，僅可查詢／維運）。
- 客戶端：Route／Engine 將簽發的 `access_token` 設入 `VBS_ROUTE_JWT`／`VBS_ENGINE_JWT`（或 `VBS_JWT`），與既有 Bearer 遙測路徑相容。

### 本機驗證

- `docker compose -f docker-compose.console.yml up --build`
- 步驟見 `docs/console-mvp-test.md`。

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
