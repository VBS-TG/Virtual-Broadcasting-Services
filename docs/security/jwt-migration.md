# JWT/Bearer 遷移設計（VBS v1.2）

## 目標

將控制平面與遙測平面的認證從 `X-VBS-Key` 過渡到 `Authorization: Bearer <JWT>`，同時保持既有節點可平滑升級。

## JWT Claims

```json
{
  "iss": "vbs-console",
  "sub": "node:vbs-route-01",
  "aud": "vbs-control-plane",
  "node_id": "vbs-route-01",
  "role": "route",
  "scope": ["telemetry:write", "control:read"],
  "iat": 1712500000,
  "exp": 1712503600,
  "jti": "uuid"
}
```

- `role`：`admin` / `capture` / `route` / `engine` / `console`。
- `scope`：最小權限原則，按 API / Topic 授權。
- `exp`：短效（建議 30-60 分鐘），避免長期憑證暴露。

## Header 規範

- 標準：`Authorization: Bearer <JWT>`。
- 過渡期（Phase 0/1）：允許 `X-VBS-Key` 作為 Legacy。
- 最終（Phase 2）：移除 `X-VBS-Key` 驗證路徑。

## Refresh 與續約

- 節點在 JWT 剩餘有效期 <10% 時主動向 Console 換發。
- 建議 refresh endpoint：`POST /api/v1/auth/refresh`。
- 續約失敗：
  - 控制面請求立即拒絕（401）。
  - 遙測允許短暫重試（exponential backoff），避免風暴。

## 角色授權矩陣（初版）

| 角色 | 允許能力 |
| --- | --- |
| `admin` | 全部 control / telemetry 管理能力 |
| `console` | 下發控制指令、收/讀全節點 telemetry |
| `route` | 上報 route telemetry、接受 route 控制 |
| `engine` | 上報 engine telemetry、接受 engine 控制 |
| `capture` | 上報 capture telemetry、接受 capture 控制 |

## 遷移步驟

1. Console 先具備雙驗證（Bearer + Legacy Key）。
2. Route/Engine/Capture 客戶端先補 Bearer，保留 Legacy fallback。
3. protocol.md 與 shared schema 標註 auth mode。
4. 觀察期結束後移除 `X-VBS-Key`。

## 稽核項目

- 所有 API/WSS 入口可追蹤 auth mode（Bearer/Legacy）。
- 新增 endpoint 一律先實作 Bearer。
- CI 需擋下未同步 protocol/schema 的通訊變更。
