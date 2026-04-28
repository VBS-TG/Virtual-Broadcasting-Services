package ctrl

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
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
	"vbs/pkg/showconfig"
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

// Start 啟動 Route 控制面 HTTP 服務（健康檢查、SRT 緩衝參數熱更新）。非資料平面；須防火牆隔離，授權為 Cf-Access-Client-Id/Secret（與本節點 VBS_CF_ACCESS_* 一致）或 Cf-Access-Jwt-Assertion。
func Start(ctx context.Context, cfg config.Config, state *rtstate.Buffer, restart chan<- struct{}, logger *log.Logger, auth *consoleauth.Provider, pipeline *srtla.Pipeline, collector *telemetry.IngestCollector) {
	if logger == nil {
		logger = log.Default()
	}
	bind := cfg.ControlBind
	if bind == "" || bind == "-" || bind == "0" {
		return
	}

	routeStore := newRelayRouteStore()
	runtimeMu := sync.RWMutex{}
	inputCount := 8
	auxCount := 4
	pgmCount := 1
	const maxShowConfigBodyBytes = 512 << 10

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
		runtimeMu.RLock()
		currentAuxCount := auxCount
		runtimeMu.RUnlock()
		items := make([]auxItem, 0, currentAuxCount)
		for i := 1; i <= currentAuxCount; i++ {
			items = append(items, auxItem{
				Channel:      i,
				relaySession: buildRelaySession(cfg, r.Host),
			})
		}
		resp := map[string]any{
			"aux_count": currentAuxCount,
			"sessions":  items,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/v1/route/runtime/config", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		runtimeMu.RLock()
		resp := map[string]any{
			"inputs":    inputCount,
			"pgm_count": pgmCount,
			"aux_count": auxCount,
		}
		runtimeMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"config": resp})
	})

	mux.HandleFunc("/api/v1/route/runtime/config/apply", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body struct {
			Inputs   int `json:"inputs"`
			PGMCount int `json:"pgm_count"`
			AUXCount int `json:"aux_count"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		if body.Inputs < 1 || body.Inputs > 8 {
			http.Error(w, "inputs must be 1..8", http.StatusBadRequest)
			return
		}
		if body.PGMCount != 1 {
			http.Error(w, "pgm_count currently supports only 1", http.StatusBadRequest)
			return
		}
		if body.AUXCount < 0 || body.AUXCount > 4 {
			http.Error(w, "aux_count must be 0..4", http.StatusBadRequest)
			return
		}
		runtimeMu.Lock()
		inputCount = body.Inputs
		pgmCount = body.PGMCount
		auxCount = body.AUXCount
		runtimeMu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied": true,
			"config": map[string]any{
				"inputs":    body.Inputs,
				"pgm_count": body.PGMCount,
				"aux_count": body.AUXCount,
			},
		})
	})

	mux.HandleFunc("/api/v1/show-config/apply", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		raw, err := io.ReadAll(io.LimitReader(r.Body, maxShowConfigBodyBytes))
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}
		var cfgShow showconfig.ShowConfig
		if err := json.Unmarshal(raw, &cfgShow); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}
		showconfig.Normalize(&cfgShow)
		runtimeMu.RLock()
		nIn := inputCount
		runtimeMu.RUnlock()
		if err := showconfig.Validate(cfgShow, nIn); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		logger.Printf("[route][ctrl] show-config applied inputs=%d panel=%s sources=%d", nIn, cfgShow.Switcher.PanelID, len(cfgShow.Sources))
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"applied": true,
			"node":    "route",
			"inputs":  nIn,
		})
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
	// Release policy:
	// 1) Preferred M2M path: Cf-Access-Client-Id/Secret from Console orchestrator.
	// 2) Fallback path: Cf-Access-Jwt-Assertion (Cloudflare JWT), but only "node" identity is accepted.
	if matchInboundAccessServiceToken(r, cfg) {
		return true
	}
	// 2) Cf-Access-Jwt-Assertion fallback
	got := strings.TrimSpace(r.Header.Get("Cf-Access-Jwt-Assertion"))
	if got == "" {
		return false
	}
	if auth == nil {
		return false
	}
	claims, err := auth.VerifyBearer(got)
	if err != nil {
		return false
	}
	role := strings.TrimSpace(strings.ToLower(claims.Role))
	return role == "node"
}

// matchInboundAccessServiceToken 比對節點上設定之 Access Service Token；須與 Console 端
// VBS_ROUTE_ACCESS_CLIENT_ID / SECRET（呼叫 Route 時）一致，部署一次後即自動化。
func matchInboundAccessServiceToken(r *http.Request, cfg config.Config) bool {
	wantID := strings.TrimSpace(cfg.CFAccessClientID)
	wantSecret := strings.TrimSpace(cfg.CFAccessClientSecret)
	if wantID == "" || wantSecret == "" {
		return false
	}
	gotID := strings.TrimSpace(r.Header.Get("Cf-Access-Client-Id"))
	gotSecret := strings.TrimSpace(r.Header.Get("Cf-Access-Client-Secret"))
	if gotID == "" || gotSecret == "" {
		return false
	}
	if len(gotID) != len(wantID) || len(gotSecret) != len(wantSecret) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(gotID), []byte(wantID)) == 1 &&
		subtle.ConstantTimeCompare([]byte(gotSecret), []byte(wantSecret)) == 1
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
