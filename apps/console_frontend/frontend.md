# VBS Console Frontend - 正式版開發規範（純前端）

本文件定義 `apps/console_frontend` 的正式版前端開發範圍，供另一個 AI 或工程師直接依規格實作。  
本版聚焦 **純前端**：頁面、互動、狀態管理、錯誤處理、驗收標準；不含後端實作與部署細節。

---

## 1. 專案目標與邊界

### 1.1 目標
- 建立可上線的 VBS Console Web 前端。
- 提供導播控制、Runtime 設定、節點狀態監控、系統健康檢查與操作回饋。
- 支援正式域名環境。
- 前端技術棧固定使用 **React + Tailwind CSS**（正式規範）。

### 1.2 邊界（本文件不做）
- 不實作任何後端 API。
- 不處理媒體轉碼、OpenLive 核心流程；僅做前端呈現與控制。

---

## 2. 角色與權限（前端視角）

- `admin`：可用所有頁面與所有控制按鈕。
- `operator`：可用監看與導播切換；不可變更高風險系統設定（如 Runtime 儲存/套用策略可由產品決定是否允許）。
- 未授權：只能看到登入/授權頁，不可進入主控台。

前端必須根據 token 解出的角色（或後端回應）做 UI 權限控管（隱藏或禁用）。

---

## 3. 資訊架構（頁面清單）

正式版至少包含以下頁面：

1. `Login / Access`（登入與授權頁）
2. `Dashboard`（總覽）
3. `Runtime Config`（活動配置）
4. `Switcher`（導播控制）
5. `Multiviewer`（多視窗監看）
6. `Telemetry`（節點遙測）
7. `System / Health`（系統健康與連線檢查）
8. `Operation Log`（前端操作紀錄）
9. `Settings`（前端偏好設定）

---

## 4. 頁面需求（逐頁）

## 4.1 Login / Access
- 功能：
  - 顯示登入狀態與目前環境（Production/Staging）。
  - 支援貼入或帶入 Bearer Token（正式環境可改 SSO/Access 流程）。
  - 驗證 token 格式與到期時間（前端基礎檢查）。
  - 成功後跳轉 Dashboard，失敗顯示可讀錯誤。
- UI 元件：
  - Token 輸入框（可顯示/隱藏）
  - Login 按鈕
  - 錯誤訊息區塊

## 4.2 Dashboard
- 功能：
  - 顯示三端節點狀態（Console/Route/Engine）。
  - 顯示最近一次 Runtime 配置摘要（inputs/pgm/aux）。
  - 顯示最近一次 apply 結果摘要（成功/失敗/回滾）。
  - 顯示快捷操作入口（前往 Switcher、Runtime Config）。
- UI 元件：
  - Node Status Cards
  - Runtime Summary Card
  - Quick Action Buttons

## 4.3 Runtime Config
- 功能：
  - 讀取目前 runtime config。
  - 編輯 `inputs`、`pgm_count`、`aux_count`。
  - 編輯 `input_sources[]` 與 `aux_sources`。
  - 支援：
    - `Save`（PUT runtime config）
    - `Apply`（POST apply）
  - 顯示 apply 詳細結果（route/engine/rolled_back）。
- 驗證規則（前端先擋）：
  - `inputs`: 1~8 整數
  - `pgm_count`: 固定 1
  - `aux_count`: 0~4 整數
  - `input_sources[i]` 必須是 `srt://...`
  - `aux_sources[k]` 只能是 `inputN` 或 `srt://...`
- UI 元件：
  - 數量設定區（inputs/pgm/aux）
  - Input Sources 可增減列編輯器
  - AUX 對應表單
  - Save / Apply 按鈕
  - Apply Response JSON 展示框

## 4.4 Switcher
- 功能：
  - 切換 Program（`/switch/program`）
  - 切換 Preview（`/switch/preview`）
  - 切換 AUX（`/switch/aux`）
  - 顯示目前切換狀態（`/switch/state`）
  - 支援 optimistic UI（送出即先高亮，失敗再回滾）
- UI 元件：
  - Source Buttons（input1..input8 動態）
  - Program/Preview 區塊（紅/綠狀態）
  - AUX channel tabs（1..4）
  - State Refresh 按鈕

## 4.5 Multiviewer
- 功能：
  - 顯示多視窗監看區塊（可先用 placeholder/video component）。
  - 支援布局切換（2x2、3x3、自動）。
  - 每個視窗顯示 source label 與連線狀態。
- 備註：
  - 現階段可先做前端容器，不必阻塞於 WebRTC 實流整合。

## 4.6 Telemetry
- 功能：
  - 讀取 `telemetry/latest` 並以卡片/表格顯示。
  - 顯示節點在線狀態、CPU/MEM、吞吐等關鍵欄位。
  - 支援自動刷新（預設 1 秒）與暫停刷新。
- UI 元件：
  - Node Telemetry Cards
  - 指標表格
  - Auto Refresh Toggle

## 4.7 System / Health
- 功能：
  - 健康檢查按鈕（Console API、Engine switcher domain）。
  - 顯示 HTTP code、耗時、最後檢查時間。
  - 顯示 Access/Tunnel 常見問題提示（302/401/502）。

## 4.8 Operation Log
- 功能：
  - 前端記錄每次操作（時間、操作、payload 摘要、結果）。
  - 支援關鍵字搜尋、清除、匯出 JSON。
  - 本地儲存（`localStorage`）即可。

## 4.9 Settings
- 功能：
  - 前端基礎設定：
    - API Base URL（預設 `https://vbsapi.cyblisswisdom.org`）
    - 自動刷新間隔
    - 主題（暗色/亮色）
  - 設定本地持久化（`localStorage`）。

---

## 5. 導航與路由

- 建議路由：
  - `/login`
  - `/dashboard`
  - `/runtime`
  - `/switcher`
  - `/multiviewer`
  - `/telemetry`
  - `/system`
  - `/logs`
  - `/settings`
- 未登入一律導向 `/login`。
- 404 導向 `/dashboard`（已登入）或 `/login`（未登入）。

---

## 6. API 契約（前端呼叫面）

Base URL：由前端設定（預設 `https://vbsapi.cyblisswisdom.org`）

共通：
- Header: `Authorization: Bearer <token>`
- Header: `Content-Type: application/json`

端點：
- `GET /api/v1/runtime/config`
- `PUT /api/v1/runtime/config`
- `POST /api/v1/runtime/config/apply`
- `POST /api/v1/switch/program`
- `POST /api/v1/switch/preview`
- `POST /api/v1/switch/aux`
- `GET /api/v1/switch/state`
- `GET /api/v1/telemetry/latest`
- `GET /healthz`

---

## 7. 狀態管理規範

- 建議使用集中式 store（例如 Zustand/Redux 擇一）。
- 最少 store 切分：
  - `authStore`
  - `runtimeStore`
  - `switcherStore`
  - `telemetryStore`
  - `settingsStore`
  - `operationLogStore`
- 所有 API 請求必須統一走 `apiClient`，集中處理：
  - token 注入
  - timeout
  - 錯誤轉譯

---

## 8. 錯誤處理與 UX 規範

- 錯誤分類：
  - 4xx：使用者可修正（權限、輸入錯誤）
  - 5xx：系統錯誤（顯示追蹤 ID/時間）
  - network timeout：提示重試與檢查 tunnel/domain
- 每個操作必須有：
  - loading 狀態
  - success toast
  - error toast（含可讀原因）
- `runtime apply` 與 `switch` 類操作必須提供「最後一次請求/回應」檢視。

---

## 9. UI/樣式規範

- 響應式：至少支援 1920 寬桌面與 768 平板。
- 色彩語意：
  - Program = 紅
  - Preview = 綠
  - Warning = 黃
  - Error = 紅色高亮
- 元件一致性：
  - Button、Input、Card、Modal 使用統一樣式 tokens。
- 文字：
  - 介面語言以繁中為主，術語可保留英文（Program/Preview/AUX）。

---

## 10. 安全規範（前端）

- 不將 token 寫入 URL query。
- `localStorage` 可暫存 token，但需提供「登出即清除」。
- console log 不輸出完整 token（僅顯示前 6 + 後 4）。
- 所有外部請求必須是 HTTPS 正式域名。

---

## 11. 測試與驗收標準

## 11.1 功能驗收
- 可成功登入並進入 Dashboard。
- Runtime Config 可讀取、儲存、套用。
- Switcher 可對 Program/Preview/AUX 送出控制並更新狀態。
- Telemetry 頁可持續刷新且可暫停。
- System 頁可辨識常見錯誤（302/401/502/timeout）。

## 11.2 非功能驗收
- 首屏可用時間 < 3 秒（內網環境）。
- API timeout 預設 10 秒，可配置。
- 不得出現未捕捉 Promise 錯誤導致白屏。

---

## 12. 開發交付清單

必交：
- 完整路由頁面與導航
- 共用 API client
- 共用型別定義（TS）
- 操作日誌與錯誤提示機制
- 基礎 E2E/互動測試（至少關鍵流程）

建議：
- Storybook 或元件展示頁（可選）
- CI lint + build + test

---

## 13. 實作優先順序（給另一個 AI）

1. `Login` + `Dashboard`
2. `Runtime Config`
3. `Switcher`
4. `Telemetry`
5. `System`
6. `Operation Log`
7. `Multiviewer`（先占位，再接實流）
8. `Settings` 與整體 UX 優化

---

## 14. 正式版結語

此文件即 `apps/console_frontend` 正式前端規範基準。  
後續若有功能調整，請直接更新本文件並維持「單一真相」。
