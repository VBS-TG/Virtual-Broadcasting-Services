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
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"vbs/apps/route/internal/config"
	"vbs/apps/route/internal/consoleauth"
	"vbs/apps/route/internal/rtstate"
	"vbs/apps/route/internal/srtla"
	"vbs/apps/route/internal/telemetry"
)

type relaySession struct {
	StreamUUID      string `json:"stream_uuid"`
	PublishStreamID string `json:"publish_streamid"`
	ReadStreamID    string `json:"read_streamid"`
	PlaybackSRTURL  string `json:"playback_srt_url"`
	RelayHost       string `json:"relay_host,omitempty"`
}

type relayRouteStore struct {
	mu     sync.RWMutex
	routes map[string]string
}

func newRelayRouteStore() *relayRouteStore {
	return &relayRouteStore{
		routes: map[string]string{
			"pgm":  "",
			"aux1": "",
			"aux2": "",
			"aux3": "",
			"aux4": "",
		},
	}
}

func (s *relayRouteStore) set(output, target string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes[output] = target
}

func (s *relayRouteStore) snapshot() map[string]string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]string, len(s.routes))
	for k, v := range s.routes {
		out[k] = v
	}
	return out
}

// Start 啟動 Route 控制面 HTTP 服務（健康檢查、SRT 緩衝參數熱更新）。非資料平面，須搭配防火牆與 Bearer JWT。
func Start(ctx context.Context, cfg config.Config, state *rtstate.Buffer, restart chan<- struct{}, logger *log.Logger, auth *consoleauth.Provider, pipeline *srtla.Pipeline, collector *telemetry.IngestCollector) {
	if logger == nil {
		logger = log.Default()
	}
	bind := cfg.ControlBind
	if bind == "" || bind == "-" || bind == "0" {
		return
	}

	routeStore := newRelayRouteStore()

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
		resp := buildRelaySession(cfg, r.Host)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/v1/route/aux/session", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		type auxItem struct {
			Channel int `json:"channel"`
			relaySession
		}
		items := make([]auxItem, 0, 4)
		for i := 1; i <= 4; i++ {
			items = append(items, auxItem{
				Channel:      i,
				relaySession: buildRelaySession(cfg, r.Host),
			})
		}
		resp := map[string]any{
			"aux_count": 4,
			"sessions":  items,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/v1/route/routes", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		switch r.Method {
		case http.MethodGet:
			resp := map[string]any{"routes": routeStore.snapshot()}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		case http.MethodPost:
			var body struct {
				Output string `json:"output"`
				Target string `json:"target"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				http.Error(w, "invalid json", http.StatusBadRequest)
				return
			}
			output := strings.ToLower(strings.TrimSpace(body.Output))
			target := strings.TrimSpace(body.Target)
			switch output {
			case "pgm", "aux1", "aux2", "aux3", "aux4":
			default:
				http.Error(w, "output must be one of: pgm,aux1,aux2,aux3,aux4", http.StatusBadRequest)
				return
			}
			if target == "" {
				http.Error(w, "target required", http.StatusBadRequest)
				return
			}
			routeStore.set(output, target)
			resp := map[string]any{
				"applied": true,
				"route": map[string]string{
					"output": output,
					"target": target,
				},
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		stats := srtla.Stats{}
		if pipeline != nil {
			stats = pipeline.Snapshot()
		}
		ingestMbps := 0.0
		if collector != nil {
			ingestMbps = collector.SampleMbps()
		}

		hasEngineClient := 0
		if stats.LastUpdate.After(time.Now().Add(-5*time.Second)) && stats.BytesSent > 0 {
			hasEngineClient = 1
		}

		routes := routeStore.snapshot()
		keys := make([]string, 0, len(routes))
		for k := range routes {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var sb strings.Builder
		sb.WriteString("# HELP vbs_route_bytes_sent_total SRT bytes sent by relay pipeline.\n")
		sb.WriteString("# TYPE vbs_route_bytes_sent_total counter\n")
		sb.WriteString("vbs_route_bytes_sent_total ")
		sb.WriteString(strconv.FormatUint(stats.BytesSent, 10))
		sb.WriteString("\n")

		sb.WriteString("# HELP vbs_route_bytes_received_total SRT bytes received by relay pipeline.\n")
		sb.WriteString("# TYPE vbs_route_bytes_received_total counter\n")
		sb.WriteString("vbs_route_bytes_received_total ")
		sb.WriteString(strconv.FormatUint(stats.BytesReceived, 10))
		sb.WriteString("\n")

		sb.WriteString("# HELP vbs_route_bytes_lost_total SRT bytes lost by relay pipeline.\n")
		sb.WriteString("# TYPE vbs_route_bytes_lost_total counter\n")
		sb.WriteString("vbs_route_bytes_lost_total ")
		sb.WriteString(strconv.FormatUint(stats.BytesLost, 10))
		sb.WriteString("\n")

		sb.WriteString("# HELP vbs_route_ingest_mbps Current ingest bitrate in Mbps.\n")
		sb.WriteString("# TYPE vbs_route_ingest_mbps gauge\n")
		sb.WriteString("vbs_route_ingest_mbps ")
		sb.WriteString(fmt.Sprintf("%.2f", ingestMbps))
		sb.WriteString("\n")

		sb.WriteString("# HELP vbs_route_has_engine_client Route has active downstream client in recent 5 seconds.\n")
		sb.WriteString("# TYPE vbs_route_has_engine_client gauge\n")
		sb.WriteString("vbs_route_has_engine_client ")
		sb.WriteString(strconv.Itoa(hasEngineClient))
		sb.WriteString("\n")

		sb.WriteString("# HELP vbs_route_dynamic_route_target Dynamic route target status (1 if target configured).\n")
		sb.WriteString("# TYPE vbs_route_dynamic_route_target gauge\n")
		for _, output := range keys {
			target := routes[output]
			v := 0
			if strings.TrimSpace(target) != "" {
				v = 1
			}
			sb.WriteString("vbs_route_dynamic_route_target{output=\"")
			sb.WriteString(output)
			sb.WriteString("\",target=\"")
			sb.WriteString(strings.ReplaceAll(target, "\"", "\\\""))
			sb.WriteString("\"} ")
			sb.WriteString(strconv.Itoa(v))
			sb.WriteString("\n")
		}

		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		_, _ = w.Write([]byte(sb.String()))
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
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return false
	}
	got := strings.TrimSpace(h[7:])
	if got == "" {
		return false
	}
	if want := strings.TrimSpace(cfg.ControlToken); want != "" {
		return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
	}
	if auth == nil {
		return false
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	want, err := auth.BearerToken(ctx)
	if err != nil || want == "" {
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

func buildRelaySession(cfg config.Config, requestHost string) relaySession {
	uuid := randomHex(16)
	publish := fmt.Sprintf("%s/%s", cfg.PGMRelayPublishPrefix, uuid)
	read := fmt.Sprintf("%s/%s", cfg.PGMRelayReadPrefix, uuid)
	host := cfg.PGMRelayPublicHost
	if host == "" {
		host = requestHost
		if i := strings.Index(host, ":"); i > 0 {
			host = host[:i]
		}
		if host == "" {
			host = "route.example.com"
		}
	}
	readURL := fmt.Sprintf("srt://%s:%d?streamid=%s&passphrase=%s&latency=%d",
		host, cfg.PGMRelayPublicPort, read, cfg.SRTPassphrase, cfg.PGMRelayLatencyMs)
	return relaySession{
		StreamUUID:      uuid,
		PublishStreamID: publish,
		ReadStreamID:    read,
		PlaybackSRTURL:  readURL,
		RelayHost:       host,
	}
}
