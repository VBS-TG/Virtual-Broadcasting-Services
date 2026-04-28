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
	"vbs/pkg/clocksync"
)

func main() {
	// 初始化設定
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		log.Fatalf("[route] invalid config err=%v", err)
	}
	headers := map[string]string{}
	if cfg.CFAccessClientID != "" && cfg.CFAccessClientSecret != "" {
		headers["Cf-Access-Client-Id"] = cfg.CFAccessClientID
		headers["Cf-Access-Client-Secret"] = cfg.CFAccessClientSecret
	}
	if res, err := clocksync.CheckWithHeaders(cfg.NTPCheckURL, cfg.NTPCheckTimeout, headers); err != nil {
		if cfg.NTPEnforce {
			log.Fatalf("[route] clock sync preflight failed err=%v", err)
		}
		log.Printf("[route] clock sync warning err=%v", err)
	} else if res.Skew > cfg.NTPMaxSkew {
		if cfg.NTPEnforce {
			log.Fatalf("[route] clock skew too large skew_ms=%d max_skew_ms=%d", res.Skew.Milliseconds(), cfg.NTPMaxSkew.Milliseconds())
		}
		log.Printf("[route] clock skew warning skew_ms=%d max_skew_ms=%d", res.Skew.Milliseconds(), cfg.NTPMaxSkew.Milliseconds())
	} else {
		log.Printf("[route] clock sync ok skew_ms=%d max_skew_ms=%d", res.Skew.Milliseconds(), cfg.NTPMaxSkew.Milliseconds())
	}

	log.Printf("[route] 啟動 node_id=%s log_level=%s metrics_interval=%s ingest_port=%d srt_out_port=%d console_base=%s control_bind=%q",
		cfg.NodeID, cfg.LogLevel, cfg.MetricsInterval, cfg.SRTLAIngestPort, cfg.SRTOutputPort, cfg.ConsoleBaseURL, cfg.ControlBind)

	// 套用 Route 節點需求的基礎網路緩衝調整（若失敗僅記錄警告，不中斷啟動）。
	system.ApplyNetTuning(nil)

	// 啟動前先檢查主要 UDP 埠是否可用，避免進入 watchdog 重啟循環才發現被占用。
	for _, p := range []int{cfg.SRTLAIngestPort, cfg.InternalSRTPort, cfg.SRTOutputPort} {
		if err := system.CheckUDPPortAvailable(p); err != nil {
			log.Fatalf("[route] port preflight failed port=%d err=%v", p, err)
		}
		log.Printf("[route] port preflight ok port=%d", p)
	}

	log.Printf(
		"[route] route config summary ingest_iface=%s encryption=%s",
		cfg.IngestIface,
		func() string {
			if cfg.SRTPassphrase != "" {
				return "enabled"
			}
			return "disabled"
		}(),
	)

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

