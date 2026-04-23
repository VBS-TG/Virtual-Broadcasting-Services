package router

import (
	"context"
	"log"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/consoleauth"
	"vbs/apps/route/internal/ctrl"
	"vbs/apps/route/internal/rtstate"
	"vbs/apps/route/internal/srtla"
	"vbs/apps/route/internal/telemetry"
)

// Run 啟動 Route：SRTLA→SRT 管線、遙測上報、可選控制面 HTTP。
func Run(ctx context.Context, cfg config.Config) {
	logger := log.Default()

	buf := rtstate.NewBuffer(cfg.LossMaxTTL, cfg.LatencyMs)
	restartCh := make(chan struct{}, 1)

	getCfg := func() srtla.PipelineConfig {
		loss, lat := buf.Snapshot()
		return srtla.PipelineConfig{
			NodeID:          cfg.NodeID,
			SRTPassphrase:   cfg.SRTPassphrase,
			SRTLAIngestPort: cfg.SRTLAIngestPort,
			SRTOutputPort:   cfg.SRTOutputPort,
			InternalSRTPort: cfg.InternalSRTPort,
			LossMaxTTL:      loss,
			LatencyMs:       lat,
		}
	}

	pipeline := srtla.NewPipeline(getCfg, logger)
	go pipeline.Run(ctx, restartCh)

	collector := telemetry.NewIngestCollector(cfg.IngestIface)
	auth := consoleauth.NewProvider(cfg)
	go telemetry.StartReporter(ctx, cfg, logger, pipeline, collector, restartCh, auth)

	ctrl.Start(ctx, cfg, buf, restartCh, logger, auth, pipeline, collector)

	<-ctx.Done()
}
