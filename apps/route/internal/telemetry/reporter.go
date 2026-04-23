package telemetry

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/consoleauth"
	"vbs/apps/route/internal/srtla"
	"vbs/apps/route/internal/system"
)

// TelemetryPayload aligns with console telemetry.v1 envelope.
type TelemetryPayload struct {
	NodeID   string                 `json:"node_id"`
	NodeType string                 `json:"node_type"`
	TsMs     int64                  `json:"ts_ms"`
	Metrics  map[string]interface{} `json:"metrics"`
	AuthMode string                 `json:"auth_mode,omitempty"`
}

// StartReporter 以 MetricsInterval 週期取樣、送 WSS、並寫入結構化日誌；可選觸發管線重啟。
func StartReporter(ctx context.Context, cfg config.Config, logger *log.Logger, pipeline *srtla.Pipeline, collector *IngestCollector, restart chan<- struct{}, auth *consoleauth.Provider) {
	if logger == nil {
		logger = log.Default()
	}

	wss := NewWSSClient(cfg, auth)
	stall := NewStallTracker(cfg)
	ticker := time.NewTicker(cfg.MetricsInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			mbps := collector.SampleMbps()
			if stall.Observe(mbps) {
				select {
				case restart <- struct{}{}:
					logger.Printf("[route][stall] ingest 停滯達閾值，已送出管線重啟信號")
				default:
				}
			}

			s := pipeline.Snapshot()
			m := collectMetrics(cfg, mbps, s)
			buf, err := json.Marshal(m)
			if err != nil {
				logger.Printf("[route][telemetry] JSON 序列化失敗 err=%v", err)
				continue
			}
			if len(buf) > 255 {
				logger.Printf("[route][telemetry] 遙測超過 255 bytes（len=%d），略過本筆", len(buf))
				continue
			}

			logger.Printf("[route][telemetry] %s", string(buf))

			payload := append([]byte(nil), buf...)
			go func() {
				sendCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()
				if err := wss.SendOne(sendCtx, payload); err != nil {
					logger.Printf("[route][telemetry] WSS 上報失敗 err=%v（非阻塞重試由下次週期再送）", err)
				}
			}()
		}
	}
}

func collectMetrics(cfg config.Config, ingestMbps float64, s srtla.Stats) TelemetryPayload {
	cpuPct := 0.0
	if v, err := system.HostCPUPercent(); err == nil {
		cpuPct = v
	}
	memUsed := uint64(0)
	if v, err := system.HostMemUsedBytes(); err == nil {
		memUsed = v
	}

	reorderErrPct := 0.0
	if s.BytesSent > 0 {
		reorderErrPct = round2(float64(s.BytesLost) * 100.0 / float64(s.BytesSent))
	}

	hasEngineClient := false
	if s.LastUpdate.After(time.Now().Add(-5*time.Second)) && s.BytesSent > 0 {
		hasEngineClient = true
	}

	return TelemetryPayload{
		NodeID:   cfg.NodeID,
		NodeType: "route",
		TsMs:     time.Now().UnixMilli(),
		Metrics: map[string]interface{}{
			"cpu_percent":       round2(cpuPct),
			"mem_bytes":         memUsed,
			"total_ingest_mbps": ingestMbps,
			"reorder_error_pct": reorderErrPct,
			"has_engine_client": hasEngineClient,
		},
		AuthMode: "cf_jwt",
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
