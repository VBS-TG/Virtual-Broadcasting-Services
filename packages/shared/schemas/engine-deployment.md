# VBS-Engine 部署與規範對齊（v1）

本檔與根目錄 `protocol.md` 之 Engine 章節同步更新；變更通訊／環境變數時請兩處一併修改。

## 已對齊項目（`apps/engine`）

- NVIDIA：`nvidia-smi` fail-fast（`VBS_ENGINE_REQUIRE_NVIDIA`）
- WebRTC：STUN／TURN（`VBS_ENGINE_STUN_SERVER`、`VBS_ENGINE_TURN_SERVER` → Brave `stun-server`／`turn-server`）
- 遙測：可選 1Hz WSS，payload 符合 `telemetry.v1.schema.json`
- 自登：Brave＋ffmpeg 管線結束後指數退避重啟（上限可調）
- MTU：可選 `ip link set`（`VBS_ENGINE_MTU_IFACE`／`VBS_ENGINE_MTU`）

## 尚未以程式強制（後續）

- Brave 內部解碼鏈全面改為 `nvh265dec`→`glupload`→`gles2`（需 fork／深度改 Brave pipeline）
- 8 路靜態 VRAM 預配（需與 Brave／CUDA 配置聯動）
- HTTP API 層 JWT 中介層（建議由 Nginx／Cloudflare Access 或專用 sidecar 處理）
