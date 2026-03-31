package router

import (
	"context"
	"log"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/srtla"
	"vbs/apps/route/internal/telemetry"
)

// Run 啟動 Route 節點的核心處理流程。
// MVP 階段僅負責啟動單一 SRTLA → SRT pipeline，並交由其內建 watchdog 管理。
func Run(ctx context.Context, cfg config.Config) {
	logger := log.Default()

	pipeline := srtla.NewPipeline(srtla.PipelineConfig{
		NodeID:          cfg.NodeID,
		SRTPassphrase:   cfg.SRTPassphrase,
		SRTLAIngestPort: cfg.SRTLAIngestPort,
		SRTOutputPort:   cfg.SRTOutputPort,
		InternalSRTPort:  cfg.InternalSRTPort,
		LossMaxTTL:       cfg.LossMaxTTL,
		LatencyMs:        cfg.LatencyMs,
	}, logger)

	go pipeline.Run(ctx)
	go telemetry.StartLocalLogger(ctx, cfg, logger, pipeline)

	// 後續可在此擴充：例如多條 pipeline、健康檢查、對 Console WebSocket Hub 的遙測上報等。
	<-ctx.Done()
}

