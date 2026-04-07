package ctrl

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/rtstate"
)

// Start 啟動 Route 控制面 HTTP 服務（健康檢查、SRT 緩衝參數熱更新）。非資料平面，須搭配防火牆與 Bearer JWT。
func Start(ctx context.Context, cfg config.Config, state *rtstate.Buffer, restart chan<- struct{}, logger *log.Logger) {
	if logger == nil {
		logger = log.Default()
	}
	bind := cfg.ControlBind
	if bind == "" || bind == "-" || bind == "0" {
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		// 運行中 UDP 埠由 srt 占用，不可再以「可綁定」判斷；能回應即視為進程就緒。
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"ready":true}`))
	})

	mux.HandleFunc("/api/v1/route/buffer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorizedBearer(r.Header.Get("Authorization"), cfg.JWTToken) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		var body struct {
			LatencyMs  *int `json:"latency_ms"`
			LossMaxTTL *int `json:"loss_max_ttl"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		curLoss, curLat := state.Snapshot()
		if body.LatencyMs != nil {
			curLat = *body.LatencyMs
		}
		if body.LossMaxTTL != nil {
			curLoss = *body.LossMaxTTL
		}
		if curLat < 20 || curLat > 10000 {
			http.Error(w, "latency_ms must be in [20,10000]", http.StatusBadRequest)
			return
		}
		if curLoss < 0 || curLoss > 255 {
			http.Error(w, "loss_max_ttl must be in [0,255]", http.StatusBadRequest)
			return
		}
		state.Update(curLoss, curLat)
		select {
		case restart <- struct{}{}:
		default:
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"applied":true}`))
	})

	srv := &http.Server{
		Addr:              bind,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Printf("[route][ctrl] HTTP 控制面監聽 %s", bind)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Printf("[route][ctrl] ListenAndServe err=%v", err)
		}
	}()

	go func() {
		<-ctx.Done()
		shCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shCtx)
	}()
}

func authorizedBearer(headerValue, expectedToken string) bool {
	if expectedToken == "" {
		return false
	}
	if !strings.HasPrefix(headerValue, "Bearer ") {
		return false
	}
	received := strings.TrimSpace(strings.TrimPrefix(headerValue, "Bearer "))
	return received == expectedToken
}
