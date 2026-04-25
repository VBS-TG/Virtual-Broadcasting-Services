// Console MVP-A: JWT issuance, telemetry WSS hub, health check.
package main

import (
	"log"

	"vbs/apps/console_backend/internal/config"
	httpserver "vbs/apps/console_backend/internal/http"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	srv := httpserver.New(cfg)
	if err := srv.ListenAndServe(); err != nil {
		log.Fatalf("server: %v", err)
	}
}
