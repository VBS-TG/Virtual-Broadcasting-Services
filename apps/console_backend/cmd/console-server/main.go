// Console MVP-A: JWT issuance, telemetry WSS hub, health check.
package main

import (
	"log"
	"time"

	"vbs/apps/console_backend/internal/config"
	httpserver "vbs/apps/console_backend/internal/http"
	"vbs/pkg/clocksync"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	headers := map[string]string{}
	if cfg.BFFProxyAccessClientID != "" && cfg.BFFProxyAccessClientSecret != "" {
		headers["Cf-Access-Client-Id"] = cfg.BFFProxyAccessClientID
		headers["Cf-Access-Client-Secret"] = cfg.BFFProxyAccessClientSecret
	}
	if res, err := clocksync.CheckWithHeaders(cfg.NTPCheckURL, cfg.NTPCheckTimeout, headers); err != nil {
		if cfg.NTPEnforce {
			log.Fatalf("clock sync preflight failed: %v", err)
		}
		log.Printf("clock sync preflight warning err=%v", err)
	} else {
		log.Printf("clock sync preflight remote=%s local=%s skew_ms=%d max_skew_ms=%d",
			res.RemoteDate.Format(time.RFC3339), res.LocalNow.Format(time.RFC3339), res.Skew.Milliseconds(), cfg.NTPMaxSkew.Milliseconds())
		if res.Skew > cfg.NTPMaxSkew {
			if cfg.NTPEnforce {
				log.Fatalf("clock skew too large skew_ms=%d max_skew_ms=%d", res.Skew.Milliseconds(), cfg.NTPMaxSkew.Milliseconds())
			}
			log.Printf("clock skew warning skew_ms=%d max_skew_ms=%d", res.Skew.Milliseconds(), cfg.NTPMaxSkew.Milliseconds())
		}
	}
	srv := httpserver.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
