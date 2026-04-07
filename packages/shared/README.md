# packages/shared

跨節點通訊欄位單一真相（Single Source of Truth）。

## 目前 schema

- `schemas/telemetry.v1.schema.json`
- `schemas/control.route-buffer.v1.schema.json`
- `schemas/node-status.v1.schema.json`
- `schemas/engine-deployment.md`（Engine 與 `.cursorrules` 對齊狀態）

## 使用規範

1. 新增或變更跨節點 JSON 欄位時，先改 schema。
2. 再同步更新 `protocol.md` 的 payload 範例與 auth mode。
3. 最後再修改各節點程式碼。
