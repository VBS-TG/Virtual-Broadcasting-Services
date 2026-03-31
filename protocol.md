## VBS-Route 服務清單與通訊規範

| Service Name / Port        | Protocol Type | Endpoint / Topic                          | Payload Definition                                                                 | Node Context                           |
| -------------------------- | ------------- | ----------------------------------------- | ---------------------------------------------------------------------------------- | -------------------------------------- |
| Route-SRTLA-Ingest / 10020 | UDP/SRT       | SRTLA 聚合入口（Listener）                | SRTLA 封包，AES-256 由 `VBS_SRT_PASSPHRASE` 管理，StreamID 依 belabox-srtla 規範 | VBS-Capture -> VBS-Route               |
| Route-SRT-Out / 10030      | UDP/SRT       | `srt://vbs-route:10030` (Listener 模式)   | 單路聚合後的 SRT 視訊流，AES-256 Passphrase 由 `VBS_SRT_PASSPHRASE` 提供          | VBS-Route -> VBS-Engine                |
| Route-Telemetry            | TCP/WebSocket | `vbs/telemetry/route-<nodeId>`            | 單筆 JSON ≤ 255 bytes，包含 CPU、Mem、即時 ingest Mbps、重排/丟包估算、Engine 連線狀態 | VBS-Route -> VBS-Console WebSocket Hub |
| Route-Buffer-Control       | HTTP/REST     | `/api/v1/route/buffer` (規劃中，未實作)   | Request/Response JSON 用於調整 SRT latency、rcvbuf 等參數                         | VBS-Console -> VBS-Route               |

### Route-Telemetry Payload（Current）

```json
{
  "node_id": "vbs-route-01",
  "cpu_percent": 0,
  "mem_bytes": 12345678,
  "total_ingest_mbps": 3.27,
  "reorder_error_pct": 12.51,
  "has_engine_client": true
}
```

