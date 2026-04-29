package ctrl

import (
	"context"
	"crypto/subtle"
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

const maxPGMCount = 5
const maxAUXCount = 20
const fixedInputSlots = 8

func newRelayRouteStore() *relayRouteStore {
	routes := map[string]string{}
	routes["pgm"] = ""
	for i := 1; i <= maxAUXCount; i++ {
		routes[fmt.Sprintf("aux%d", i)] = ""
	}
	return &relayRouteStore{
		routes: routes,
	}
}

func isValidRouteOutput(output string) bool {
	o := strings.ToLower(strings.TrimSpace(output))
	if o == "pgm" {
		return true
	}
	if !strings.HasPrefix(o, "aux") {
		return false
	}
	n, err := strconv.Atoi(strings.TrimPrefix(o, "aux"))
	return err == nil && n >= 1 && n <= maxAUXCount
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

type discoveredInput struct {
	ID     string `json:"id"`
	Class  string `json:"class"` // capture | other
	Label  string `json:"label,omitempty"`
	Origin string `json:"origin,omitempty"`
	Source string `json:"source,omitempty"` // route_dynamic_routes | external_registry
	Online bool   `json:"online"`
}

func discoverInputsFromRoutes(routes map[string]string) []discoveredInput {
	seen := map[string]discoveredInput{}
	for _, target := range routes {
		t := strings.TrimSpace(target)
		if t == "" {
			continue
		}
		id, className, label := classifyInputTarget(t)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = discoveredInput{
			ID:     id,
			Class:  className,
			Label:  label,
			Origin: t,
			Source: "route_dynamic_routes",
			Online: true,
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]discoveredInput, 0, len(keys))
	for _, k := range keys {
		out = append(out, seen[k])
	}
	return out
}

func discoverInputsFromExternal(rawList []string) []discoveredInput {
	seen := map[string]discoveredInput{}
	for _, raw := range rawList {
		target := strings.TrimSpace(raw)
		if target == "" {
			continue
		}
		id, className, label := classifyInputTarget(target)
		if id == "" {
			continue
		}
		seen[id] = discoveredInput{
			ID:     id,
			Class:  className,
			Label:  label,
			Origin: target,
			Source: "external_registry",
			Online: true,
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]discoveredInput, 0, len(keys))
	for _, k := range keys {
		out = append(out, seen[k])
	}
	return out
}

func mergeDiscoveredInputs(groups ...[]discoveredInput) []discoveredInput {
	seen := map[string]discoveredInput{}
	for _, items := range groups {
		for _, item := range items {
			item.ID = strings.TrimSpace(item.ID)
			if item.ID == "" {
				continue
			}
			if _, ok := seen[item.ID]; ok {
				continue
			}
			seen[item.ID] = item
		}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]discoveredInput, 0, len(keys))
	for _, k := range keys {
		out = append(out, seen[k])
	}
	return out
}

func classifyInputTarget(raw string) (id, className, label string) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", "", ""
	}
	l := strings.ToLower(s)
	switch {
	case strings.HasPrefix(l, "capture:"):
		base := strings.TrimSpace(s[len("capture:"):])
		if base == "" {
			base = "unknown"
		}
		return "capture:" + base, "capture", base
	case strings.HasPrefix(l, "capture-"):
		return s, "capture", strings.TrimPrefix(s, "capture-")
	case strings.HasPrefix(l, "input"):
		return s, "other", s
	case strings.HasPrefix(l, "srt://"):
		return s, "other", "external"
	default:
		return s, "other", s
	}
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
		resp := buildRelaySession(cfg, r.Host, "pgm")
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
				relaySession: buildRelaySession(cfg, r.Host, fmt.Sprintf("aux%d", i)),
			})
		}
		resp := map[string]any{
			"aux_count": currentAuxCount,
			"sessions":  items,
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	})

	mux.HandleFunc("/api/v1/route/input/sessions", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		type inputItem struct {
			Slot string `json:"slot"`
			relaySession
		}
		items := make([]inputItem, 0, fixedInputSlots)
		for i := 1; i <= fixedInputSlots; i++ {
			slot := fmt.Sprintf("input%d", i)
			items = append(items, inputItem{
				Slot:         slot,
				relaySession: buildRelaySession(cfg, r.Host, slot),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"inputs": items,
		})
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
		if body.PGMCount < 1 || body.PGMCount > maxPGMCount {
			http.Error(w, "pgm_count must be 1..5", http.StatusBadRequest)
			return
		}
		if body.AUXCount < 0 || body.AUXCount > maxAUXCount {
			http.Error(w, "aux_count must be 0..20", http.StatusBadRequest)
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
			if !isValidRouteOutput(output) {
				http.Error(w, "output must be pgm or aux1..aux20", http.StatusBadRequest)
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

	mux.HandleFunc("/api/v1/route/inputs", func(w http.ResponseWriter, r *http.Request) {
		if !authorizedControlPlane(r, cfg, auth) {
			logger.Printf("[route][ctrl] 未授權的 API 請求 remote=%s path=%s", r.RemoteAddr, r.URL.Path)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		routes := routeStore.snapshot()
		inputs := mergeDiscoveredInputs(
			discoverInputsFromRoutes(routes),
			discoverInputsFromExternal(cfg.ExternalInputs),
		)
		sources := []string{"route_dynamic_routes"}
		if len(cfg.ExternalInputs) > 0 {
			sources = append(sources, "external_registry")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"inputs":              inputs,
			"runtime_auto_inputs": true,
			"sources":             sources,
		})
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
	// 2) Fallback path: Cf-Access-Jwt-Assertion (Cloudflare JWT), but only "console" identity is accepted.
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
	return role == "console"
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

func buildRelaySession(cfg config.Config, requestHost, slot string) relaySession {
	slot = strings.ToLower(strings.TrimSpace(slot))
	if slot == "" {
		slot = "pgm"
	}
	publish := fmt.Sprintf("%s/%s", cfg.PGMRelayPublishPrefix, slot)
	read := fmt.Sprintf("%s/%s", cfg.PGMRelayReadPrefix, slot)
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
		StreamUUID:      slot,
		PublishStreamID: publish,
		ReadStreamID:    read,
		PlaybackSRTURL:  readURL,
		RelayHost:       host,
	}
}
