package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/router"
	"vbs/apps/route/internal/system"
)

func main() {
	// 初始化設定
	cfg := config.Load()

	log.Printf("[route] start node_id=%s log_level=%s metrics_interval=%s ingest_port=%d srt_out_port=%d",
		cfg.NodeID, cfg.LogLevel, cfg.MetricsInterval, cfg.SRTLAIngestPort, cfg.SRTOutputPort)

	// 套用 Route 節點需求的基礎網路緩衝調整（若失敗僅記錄警告，不中斷啟動）。
	system.ApplyNetTuning(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 啟動 Route 核心路由流程（內含 SRTLA → SRT pipeline 與 watchdog）
	go router.Run(ctx, cfg)

	// 等待系統訊號以便優雅關閉
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	log.Printf("[route] shutting down")
	cancel()

	// 預留一點時間給子協程清理資源
	time.Sleep(2 * time.Second)
}

