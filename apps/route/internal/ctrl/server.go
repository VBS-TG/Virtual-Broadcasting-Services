package ctrl

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/consoleauth"
	"vbs/apps/route/internal/rtstate"
)

// Start 啟動 Route 控制面 HTTP 服務（健康檢查、SRT 緩衝參數熱更新）。非資料平面，須搭配防火牆與 Bearer JWT。
func Start(ctx context.Context, cfg config.Config, state *rtstate.Buffer, restart chan<- struct{}, logger *log.Logger, auth *consoleauth.Provider) {
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
		if !authorizedControlPlane(r, cfg, auth) {
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
		if curLat <= 0 || curLoss <= 0 {
			http.Error(w, "latency_ms and loss_max_ttl must be positive", http.StatusBadRequest)
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

	mux.HandleFunc("/api/v1/route/pgm/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		uuid := randomHex(16)
		publish := fmt.Sprintf("%s/%s", cfg.PGMRelayPublishPrefix, uuid)
		read := fmt.Sprintf("%s/%s", cfg.PGMRelayReadPrefix, uuid)
		host := cfg.PGMRelayPublicHost
		if host == "" {
			host = r.Host
			if i := strings.Index(host, ":"); i > 0 {
				host = host[:i]
			}
			if host == "" {
				host = "route.example.com"
			}
		}
		readURL := fmt.Sprintf("srt://%s:%d?streamid=%s&passphrase=%s&latency=%d",
			host, cfg.PGMRelayPublicPort, read, cfg.SRTPassphrase, cfg.PGMRelayLatencyMs)
		resp := map[string]string{
			"stream_uuid":       uuid,
			"publish_streamid":  publish,
			"read_streamid":     read,
			"playback_srt_url":  readURL,
			"relay_host":        host,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
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

func authorizedControlPlane(r *http.Request, cfg config.Config, auth *consoleauth.Provider) bool {
	if auth == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	want, err := auth.BearerToken(ctx)
	if err != nil || want == "" {
		return false
	}
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return false
	}
	got := strings.TrimSpace(h[7:])
	if got == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}
