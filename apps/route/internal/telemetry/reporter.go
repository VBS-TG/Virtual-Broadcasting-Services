package telemetry

import (
	"context"
	"encoding/json"
	"log"
	"time"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/srtla"
	"vbs/apps/route/internal/system"
)

// Metrics 為 telemetry.v1 schema 的 metrics 區塊。
type Metrics struct {
	CPUPct         float64 `json:"cpu_pct"`
	MemBytes       uint64  `json:"mem_bytes"`
	IngestMbps     float64 `json:"ingest_mbps"`
	ReorderErrorPct float64 `json:"reorder_error_pct"`
	HasEngineClient bool    `json:"has_engine_client"`
	StreamOK        bool    `json:"stream_ok"`
}

// Payload 為 Route 節點上報 Console 之 JSON（單筆 ≤255 bytes）。
type Payload struct {
	NodeID   string  `json:"node_id"`
	NodeType string  `json:"node_type"`
	TSMS     int64   `json:"ts_ms"`
	Metrics  Metrics `json:"metrics"`
	AuthMode string  `json:"auth_mode"`
}

// StartReporter 以 MetricsInterval 週期取樣、送 WSS、並寫入結構化日誌；可選觸發管線重啟。
func StartReporter(ctx context.Context, cfg config.Config, logger *log.Logger, pipeline *srtla.Pipeline, collector *IngestCollector, restart chan<- struct{}) {
	if logger == nil {
		logger = log.Default()
	}

	wss := NewWSSClient(cfg)
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

func collectMetrics(cfg config.Config, ingestMbps float64, s srtla.Stats) Payload {
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

	return Payload{
		NodeID:   cfg.NodeID,
		NodeType: "route",
		TSMS:     time.Now().UnixMilli(),
		AuthMode: "bearer",
		Metrics: Metrics{
			CPUPct:          round2(cpuPct),
			MemBytes:        memUsed,
			IngestMbps:      ingestMbps,
			ReorderErrorPct: reorderErrPct,
			HasEngineClient: hasEngineClient,
			StreamOK:        hasEngineClient,
		},
	}
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
