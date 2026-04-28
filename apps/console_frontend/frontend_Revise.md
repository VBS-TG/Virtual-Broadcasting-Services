# Console Frontend 增減需求（純前端、無 API 變更）

> 目標：只調整前端資訊架構、畫面與狀態模型；不新增或修改任何後端 API。

## 一、掃描結果（目前現況）

- 路由主幹已收斂為：`dashboard`、`switcher`、`pipeline`、`rental-sessions`、`telemetry`、`logs`。
- 已刪頁面：`RuntimeConfig`、`Settings`、`ShowControlPage`、`SystemHealth`。
- 桌機/手機導覽已對齊 6 項主導覽（`Sidebar.tsx`、`BottomNav.tsx`）。
- 仍有前端型別殘留舊頁面語意（`src/types.ts` 的 `PageKey` 仍含 `runtime/system/settings`）。
- `runtimeStore` 仍保留舊操作心智（`save/apply` 以可編輯 runtime 為中心），與「前端只顯示、自動化配置」目標不一致。

## 二、增減需求（本輪必做）

### A. 導覽與路由（減）

- 移除前端內所有已刪頁面的文案與代碼殘留：
  - `runtime`
  - `settings`
  - `system`
- 統一導航名稱（中文一致）：
  - 總覽 / 導播 / 鏈路 / 租賃 / 遙測 / 日誌

### B. 型別模型（減）

- 精簡 `src/types.ts` 的 `PageKey`：移除 `runtime`、`system`、`settings`。
- 清掉不再使用的 Health/Settings 型別（若已無實際引用）。
- 將 Runtime 相關前端型別改為「顯示導向」：
  - 只保留頁面需要展示的欄位
  - 移除使用者可編輯設定的語意欄位

### C. 狀態管理（改）

- `runtimeStore` 轉為唯讀資訊模型：
  - 保留：讀取、載入狀態、錯誤狀態
  - 移除：前端可編輯/可套用流程狀態（`save/apply` 導向）
- `operationLogStore` 聚焦 UI 操作事件，不再承載「設定提交成功/失敗」這類配置流程敘事。

### D. 頁面資訊架構（增）

- 在既有頁面補齊「新架構說明區塊」：
  - `Dashboard` 顯示目前系統分區（導播 / 鏈路 / 租賃 / 遙測 / 日誌）。
  - `PipelinePage` 顯示「自動化輸入來源」為唯讀資訊卡，不提供手動編輯控件。
- `SwitcherPage` 補「控制通道狀態」前端 UI（連線中/已斷線/重連中）與視覺提示，不調整 API。

## 三、建議本輪一起做（純前端優化）

### E. 元件層整併

- 抽出共用的 `NavItem` 規格（桌機側欄/手機底欄共用同一份資料來源），避免雙份定義分歧。
- 抽出 `PageShell`（標題、說明、內容區）作為六個主頁共用骨架，統一視覺節奏。

### F. 文案與語意一致性

- 全站文案統一繁中術語：
  - Pipeline -> 鏈路
  - Rental Sessions -> 租賃
  - Telemetry -> 遙測
- 刪除過時文案：`Runtime Config`、`System Health`、`Settings`。


