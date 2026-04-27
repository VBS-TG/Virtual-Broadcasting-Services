# Frontend Revise (Page Gap Scan)

## 掃描結論（以目前 `App.tsx` 路由與頁面實作為準）

目前已存在頁面：
- `/login`
- `/dashboard`
- `/switcher`
- `/runtime`
- `/telemetry`
- `/system`
- `/logs`
- `/settings`
- `/popout/switcher`
- `/popout/multiviewer`

## 目前仍缺的頁面 / 路由

### 1) Admin 租賃碼管理頁（高優先）
- 依目前規範：Admin 需在後台建立「有時效租賃碼」給 Guest。
- 現況前端沒有專門頁面可建立/撤銷租賃碼（僅有 Team 端輸入 Code 登入）。
- 建議新增：
  - `src/pages/RentalSessions.tsx`（或 `GuestSessions.tsx`）
  - 路由：`/rental-sessions`（admin only）
  - UI 欄位（先前端，不綁 API 也可）：
    - Name / Label
    - TTL 秒數或到期時間
    - 產生按鈕
    - 目前有效租賃碼清單（含 revoke/delete 按鈕）

### 2) 角色導向頁面可見性（中優先）
- 現況 `runtime` / `settings` 對非 admin 只做「唯讀標示」，但仍可進入。
- 建議補強：
  - 以角色控制頁面入口（admin / operator）
  - 非 admin 進入 admin-only 頁面時顯示專用空狀態頁（403 UI）

## 可先不做（目前可接受）
- `SystemHealth` 已有（但目前只檢查 Console + Engine，未包含 Route 健康）
- `OperationLog` 目前是本地 log（`zustand persist`），若後續要正式審計可再改後端來源

## 建議開發順序（先前端，後補 API）
1. `RentalSessions` 後台頁（先做完整 UI 流程與狀態）
2. Role-based route guard（admin-only）
3. 再接 API（`guest/sessions`, `guest/exchange-pin`, revoke/delete）
