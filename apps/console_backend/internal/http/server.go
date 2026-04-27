// Package httpserver wires HTTP + WebSocket routes for Console MVP-A.
package httpserver

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"vbs/apps/console_backend/internal/auth"
	"vbs/apps/console_backend/internal/config"
	"vbs/apps/console_backend/internal/telemetry"

	"github.com/gorilla/websocket"
)

const maxTokenBodyBytes = 4096

// Server is the console HTTP server.
type Server struct {
	cfg    *config.Config
	access *auth.AccessJWTVerifier
	hub    *telemetry.Hub
	http   *http.Server
	mux    *http.ServeMux
	upgrader websocket.Upgrader

	eventMu    sync.Mutex
	eventConns map[*websocket.Conn]struct{}
	guestStore *guestStore
	runtimeStore *runtimeStore

	runtimeMu        sync.RWMutex
	runtimeCfg       runtimeConfig
	runtimeUpdatedAt int64
}

type guestSession struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	PIN            string `json:"pin"`
	SessionVersion int    `json:"session_version"`
	Revoked        bool   `json:"revoked"`
	CreatedAt      int64  `json:"created_at"`
	ExpiresAt      int64  `json:"expires_at"`
}

type runtimeConfig struct {
	Inputs      int               `json:"inputs"`
	PGMCount    int               `json:"pgm_count"`
	AUXCount    int               `json:"aux_count"`
	InputSources []string         `json:"input_sources,omitempty"`
	AUXSources  map[string]string `json:"aux_sources,omitempty"`
}

// New constructs a Server from config.
func New(cfg *config.Config) *Server {
	s := &Server{
		cfg: cfg,
		hub: telemetry.NewHub(cfg.NodeOfflineTTL),
		mux: http.NewServeMux(),
		eventConns: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		runtimeCfg: runtimeConfig{
			Inputs:   8,
			PGMCount: 1,
			AUXCount: 4,
		},
		runtimeUpdatedAt: time.Now().UTC().Unix(),
	}
	store, err := openGuestStore(cfg.GuestDBPath)
	if err != nil {
		log.Fatalf("guest store init failed: %v", err)
	}
	s.guestStore = store
	rtStore, err := openRuntimeStore(cfg.RuntimeDBPath)
	if err != nil {
		log.Fatalf("runtime store init failed: %v", err)
	}
	s.runtimeStore = rtStore
	if savedCfg, savedAt, err := s.runtimeStore.Load(); err == nil {
		if err := validateRuntimeConfig(savedCfg); err == nil {
			s.runtimeCfg = savedCfg
			s.runtimeUpdatedAt = savedAt
		}
	}
	access, err := auth.NewAccessJWTVerifier(
		cfg.CFAccessMode,
		cfg.CFAccessTeamDomain,
		cfg.CFAccessAUD,
		cfg.CFAccessJWKSURL,
		cfg.CFAccessJWKSCacheTTL,
		cfg.AdminEmails,
		cfg.NodeCNPrefix,
		cfg.ConsoleJWTIssuer,
		cfg.ConsoleJWTPrivateKey,
		cfg.ConsoleJWTPublicKeys,
	)
	if err != nil {
		log.Fatalf("access verifier init failed: %v", err)
	}
	s.access = access
	s.routes()
	go s.fanoutStatusEvents()
	s.http = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           s.withCORS(s.mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := strings.TrimSpace(r.Header.Get("Origin"))
		if origin != "" && s.isAllowedOrigin(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, Cf-Access-Jwt-Assertion")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) isAllowedOrigin(origin string) bool {
	if s == nil || s.cfg == nil {
		return false
	}
	for _, allowed := range s.cfg.CORSAllowedOrigins {
		if strings.EqualFold(strings.TrimSpace(allowed), origin) {
			return true
		}
	}
	return false
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /api/v1/auth/admin/email-login", s.handleAdminEmailLogin)
	s.mux.HandleFunc("GET /vbs/telemetry/ws", s.handleTelemetryWS)
	s.mux.HandleFunc("GET /vbs/telemetry/events/ws", s.handleTelemetryEventsWS)
	s.mux.HandleFunc("GET /api/v1/telemetry/latest", s.handleTelemetryLatest)
	s.mux.HandleFunc("POST /api/v1/stream/session-key", s.handleSessionKey)
	s.mux.HandleFunc("POST /api/v1/pgm/route-buffer", s.handlePGMRouteBuffer)
	s.mux.HandleFunc("GET /api/v1/route/metrics", s.handleRouteMetrics)
	s.mux.HandleFunc("POST /api/v1/switch/program", s.handleSwitchProgram)
	s.mux.HandleFunc("POST /api/v1/switch/preview", s.handleSwitchPreview)
	s.mux.HandleFunc("POST /api/v1/switch/aux", s.handleSwitchAUX)
	s.mux.HandleFunc("GET /api/v1/switch/state", s.handleSwitchState)
	s.mux.HandleFunc("POST /api/v1/guest/sessions", s.handleGuestSessionCreate)
	s.mux.HandleFunc("DELETE /api/v1/guest/sessions/{id}", s.handleGuestSessionDelete)
	s.mux.HandleFunc("POST /api/v1/guest/exchange-pin", s.handleGuestExchangePIN)
	s.mux.HandleFunc("POST /api/v1/guest/introspect", s.handleGuestIntrospect)
	s.mux.HandleFunc("GET /api/v1/runtime/config", s.handleRuntimeConfigGet)
	s.mux.HandleFunc("PUT /api/v1/runtime/config", s.handleRuntimeConfigPut)
	s.mux.HandleFunc("POST /api/v1/runtime/config/apply", s.handleRuntimeConfigApply)
}

func (s *Server) handleAdminEmailLogin(w http.ResponseWriter, r *http.Request) {
	claims, err := s.access.VerifyRequest(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if !auth.IsAdminRole(claims.Role) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxTokenBodyBytes)).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	email := strings.TrimSpace(strings.ToLower(body.Email))
	if email == "" {
		http.Error(w, `{"error":"email required"}`, http.StatusBadRequest)
		return
	}
	if claims.Email == "" || !strings.EqualFold(strings.TrimSpace(claims.Email), email) {
		http.Error(w, `{"error":"email mismatch with access identity"}`, http.StatusForbidden)
		return
	}
	ttl := 8 * time.Hour
	token, err := s.access.MintAdminToken("admin:"+email, ttl)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"mint admin token failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": token,
		"token_type": "Bearer",
		"expires_at": time.Now().UTC().Add(ttl).Unix(),
		"role": "admin",
		"email": email,
	})
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	log.Printf("console-server listening on %s", s.cfg.ListenAddr)
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	s.hub.Close()
	s.closeEventConns()
	if s.guestStore != nil {
		_ = s.guestStore.Close()
	}
	if s.runtimeStore != nil {
		_ = s.runtimeStore.Close()
	}
	return s.http.Shutdown(ctx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

func (s *Server) handleSessionKey(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	buf := make([]byte, 24) // 32 chars base64url-ish
	if _, err := rand.Read(buf); err != nil {
		http.Error(w, `{"error":"keygen failed"}`, http.StatusInternalServerError)
		return
	}
	key := base64.RawURLEncoding.EncodeToString(buf)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"passphrase": key,
		"algorithm":  "SRT-AES-256",
		"expires_in": "session",
	})
}

func (s *Server) adminAuthorized(r *http.Request) bool {
	claims, err := s.access.VerifyRequest(r)
	if err != nil {
		return false
	}
	return auth.IsAdminRole(claims.Role)
}

func (s *Server) controlAuthorized(r *http.Request) bool {
	claims, err := s.access.VerifyRequest(r)
	if err != nil {
		return false
	}
	if !auth.CanControlPlane(claims.Role) {
		return false
	}
	if claims.Role == "operator" {
		guestID := strings.TrimPrefix(strings.TrimSpace(claims.Subject), "guest:")
		return s.guestStore.ValidateTokenSession(guestID, claims.SessionVersion)
	}
	return true
}

func (s *Server) handleTelemetryLatest(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	s.writeLatest(w)
}

func (s *Server) writeLatest(w http.ResponseWriter) {
	snap := s.hub.Snapshot()
	presence := s.hub.PresenceSnapshot()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"latest":    snap,
		"presence":  presence,
		"ts_ms":     time.Now().UTC().UnixMilli(),
	})
}

func (s *Server) handleTelemetryWS(w http.ResponseWriter, r *http.Request) {
	claims, err := s.access.VerifyRequest(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !auth.IsTelemetryRole(claims.Role) {
		http.Error(w, "forbidden: role cannot send telemetry", http.StatusForbidden)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	for {
		mt, payload, err := conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("telemetry ws read: %v", err)
			}
			break
		}
		if mt != websocket.TextMessage && mt != websocket.BinaryMessage {
			continue
		}
		if len(payload) > s.cfg.TelemetryMax {
			_ = conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "payload too large"),
				time.Now().Add(5*time.Second))
			break
		}
		if err := s.hub.ValidateAndStore(payload); err != nil {
			_ = conn.WriteMessage(websocket.TextMessage, []byte(`{"error":"`+trimErr(err)+`"}`))
			continue
		}
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
	}
}

func (s *Server) handleTelemetryEventsWS(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("events websocket upgrade: %v", err)
		return
	}
	s.eventMu.Lock()
	s.eventConns[conn] = struct{}{}
	s.eventMu.Unlock()

	defer func() {
		s.eventMu.Lock()
		delete(s.eventConns, conn)
		s.eventMu.Unlock()
		_ = conn.Close()
	}()

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})
	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (s *Server) fanoutStatusEvents() {
	events := s.hub.SubscribeStatusEvents()
	for ev := range events {
		raw, err := json.Marshal(ev)
		if err != nil {
			continue
		}
		s.eventMu.Lock()
		for conn := range s.eventConns {
			_ = conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, raw); err != nil {
				_ = conn.Close()
				delete(s.eventConns, conn)
			}
		}
		s.eventMu.Unlock()
	}
}

func (s *Server) closeEventConns() {
	s.eventMu.Lock()
	defer s.eventMu.Unlock()
	for conn := range s.eventConns {
		_ = conn.Close()
		delete(s.eventConns, conn)
	}
}

func trimErr(err error) string {
	s := err.Error()
	s = strings.ReplaceAll(s, `"`, `'`)
	if len(s) > 200 {
		s = s[:200]
	}
	return s
}

func (s *Server) handlePGMRouteBuffer(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.RouteControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ROUTE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxTokenBodyBytes))
	if err != nil {
		http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		http.Error(w, `{"error":"body required"}`, http.StatusBadRequest)
		return
	}
	respBody, status, err := s.routeControlPOST("/api/v1/route/buffer", body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"route buffer proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	if status > 0 {
		w.WriteHeader(status)
	}
	_, _ = w.Write(respBody)
}

func (s *Server) handleRouteMetrics(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.RouteControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ROUTE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	base := strings.TrimRight(s.cfg.RouteControlBaseURL, "/")
	target := base + "/metrics"
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		http.Error(w, `{"error":"build metrics request failed"}`, http.StatusInternalServerError)
		return
	}
	s.attachRouteServiceToken(req)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"route metrics proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"route metrics read failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}

func (s *Server) handleSwitchProgram(w http.ResponseWriter, r *http.Request) {
	s.proxyEngineControl(w, r, "/api/v1/switch/program")
}

func (s *Server) handleSwitchPreview(w http.ResponseWriter, r *http.Request) {
	s.proxyEngineControl(w, r, "/api/v1/switch/preview")
}

func (s *Server) handleSwitchAUX(w http.ResponseWriter, r *http.Request) {
	s.proxyEngineControl(w, r, "/api/v1/switch/aux")
}

func (s *Server) handleSwitchState(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.EngineControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ENGINE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	base := strings.TrimRight(s.cfg.EngineControlBaseURL, "/")
	target := base + "/api/v1/switch/state"
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		http.Error(w, `{"error":"build engine state request failed"}`, http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	s.attachEngineServiceToken(req)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"engine switch state proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"engine switch state read failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}

func (s *Server) proxyEngineControl(w http.ResponseWriter, r *http.Request, path string) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.EngineControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ENGINE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxTokenBodyBytes))
	if err != nil {
		http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		http.Error(w, `{"error":"body required"}`, http.StatusBadRequest)
		return
	}
	base := strings.TrimRight(s.cfg.EngineControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, `{"error":"build engine request failed"}`, http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	s.attachEngineServiceToken(req)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"engine switch proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"engine switch read failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}

func (s *Server) handleGuestSessionCreate(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var body struct {
		Name string `json:"name"`
		TTLSeconds int `json:"ttl_seconds"`
	}
	_ = json.NewDecoder(io.LimitReader(r.Body, maxTokenBodyBytes)).Decode(&body)
	name := strings.TrimSpace(body.Name)
	if name == "" {
		name = "guest"
	}
	ttl := s.cfg.GuestTokenTTL
	if body.TTLSeconds > 0 {
		ttl = time.Duration(body.TTLSeconds) * time.Second
	}
	id := randomToken(12)
	pin := randomDigits(6)
	now := time.Now().UTC()
	session := guestSession{
		ID: id,
		Name: name,
		PIN: pin,
		SessionVersion: 1,
		CreatedAt: now.Unix(),
		ExpiresAt: now.Add(ttl).Unix(),
	}
	if err := s.guestStore.Create(session); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"store guest session failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	token, err := s.access.MintGuestToken("guest:"+id, "control:basic telemetry:read", ttl, session.SessionVersion)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"mint guest token failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": session.ID,
		"name": session.Name,
		"pin": session.PIN,
		"expires_at": session.ExpiresAt,
		"access_token": token,
		"token_type": "Bearer",
		"magic_link": fmt.Sprintf("/guest?pin=%s", session.PIN),
	})
}

func (s *Server) handleGuestSessionDelete(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	id := strings.TrimSpace(r.PathValue("id"))
	if id == "" {
		http.Error(w, `{"error":"id required"}`, http.StatusBadRequest)
		return
	}
	if err := s.guestStore.Delete(id); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"delete guest session failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"deleted":true}`))
}

func (s *Server) handleGuestExchangePIN(w http.ResponseWriter, r *http.Request) {
	var body struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxTokenBodyBytes)).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	pin := strings.TrimSpace(body.PIN)
	if len(pin) != 6 {
		http.Error(w, `{"error":"invalid pin"}`, http.StatusBadRequest)
		return
	}
	session, err := s.guestStore.GetByPIN(pin)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, `{"error":"pin expired or invalid"}`, http.StatusUnauthorized)
			return
		}
		http.Error(w, fmt.Sprintf(`{"error":"lookup pin failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	if session.Revoked || time.Now().UTC().Unix() > session.ExpiresAt {
		http.Error(w, `{"error":"pin expired or invalid"}`, http.StatusUnauthorized)
		return
	}
	ttl := time.Until(time.Unix(session.ExpiresAt, 0))
	token, err := s.access.MintGuestToken("guest:"+session.ID, "control:basic telemetry:read", ttl, session.SessionVersion)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"mint guest token failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": token,
		"token_type": "Bearer",
		"expires_at": session.ExpiresAt,
	})
}

func (s *Server) handleGuestIntrospect(w http.ResponseWriter, r *http.Request) {
	claims, err := s.access.VerifyRequest(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if !(auth.IsTelemetryRole(claims.Role) || auth.CanControlPlane(claims.Role)) {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	var body struct {
		GuestID        string `json:"guest_id"`
		SessionVersion int    `json:"session_version"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxTokenBodyBytes)).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	ok := s.guestStore.ValidateTokenSession(strings.TrimSpace(body.GuestID), body.SessionVersion)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"active": ok,
	})
}

func (s *Server) handleRuntimeConfigGet(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	cfg, updatedAt := s.getRuntimeConfig()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"config":     cfg,
		"updated_at": updatedAt,
	})
}

func (s *Server) handleRuntimeConfigPut(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var body runtimeConfig
	if err := json.NewDecoder(io.LimitReader(r.Body, maxTokenBodyBytes)).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	if err := validateRuntimeConfig(body); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, trimErr(err)), http.StatusBadRequest)
		return
	}
	s.setRuntimeConfig(body)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saved": true,
		"config": body,
		"updated_at": s.runtimeUpdatedAt,
	})
}

func (s *Server) handleRuntimeConfigApply(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	cfg, updatedAt := s.getRuntimeConfig()
	cfgBody, _ := json.Marshal(cfg)
	result := map[string]any{
		"config":      cfg,
		"updated_at":  updatedAt,
		"route":       map[string]any{"ok": false},
		"engine":      map[string]any{"ok": false},
		"applied_at":  time.Now().UTC().Unix(),
	}

	var prevRoute json.RawMessage
	var prevEngine json.RawMessage
	if s.cfg.RouteControlBaseURL != "" {
		if raw, status, err := s.routeControlGET("/api/v1/route/runtime/config"); err == nil && status < 400 {
			prevRoute = extractConfigPayload(raw)
		}
	}
	if s.cfg.EngineControlBaseURL != "" {
		if raw, status, err := s.engineControlGET("/api/v1/runtime/config"); err == nil && status < 400 {
			prevEngine = extractConfigPayload(raw)
		}
	}

	routeApplied := false
	if s.cfg.RouteControlBaseURL != "" {
		applyBody, applyStatus, applyErr := s.routeControlPOST("/api/v1/route/runtime/config/apply", cfgBody)
		routeApplied = applyErr == nil && applyStatus < 400
		result["route"] = map[string]any{
			"ok":         routeApplied,
			"apply_status": applyStatus,
			"apply_raw":  string(applyBody),
			"error":      firstErr(applyErr),
		}
	}

	engineApplied := false
	if s.cfg.EngineControlBaseURL != "" {
		applyBody, applyStatus, applyErr := s.engineControlPOST("/api/v1/runtime/config/apply", cfgBody)
		engineApplied = applyErr == nil && applyStatus < 400
		stateRaw, stateStatus, stateErr := s.engineControlGET("/api/v1/switch/state")
		result["engine"] = map[string]any{
			"ok":           engineApplied && stateErr == nil && stateStatus < 400,
			"apply_status": applyStatus,
			"apply_raw":    string(applyBody),
			"state_status": stateStatus,
			"state":        json.RawMessage(stateRaw),
			"error":        firstErr(applyErr, stateErr),
		}
	}

	if !routeApplied || !engineApplied {
		rolled := map[string]bool{}
		if routeApplied && len(prevRoute) > 0 {
			_, status, err := s.routeControlPOST("/api/v1/route/runtime/config/apply", prevRoute)
			rolled["route"] = err == nil && status < 400
		}
		if engineApplied && len(prevEngine) > 0 {
			_, status, err := s.engineControlPOST("/api/v1/runtime/config/apply", prevEngine)
			rolled["engine"] = err == nil && status < 400
		}
		result["rolled_back"] = rolled
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(result)
}

func validateRuntimeConfig(cfg runtimeConfig) error {
	if cfg.Inputs < 1 || cfg.Inputs > 8 {
		return fmt.Errorf("inputs must be between 1 and 8")
	}
	if cfg.PGMCount < 1 || cfg.PGMCount > 1 {
		return fmt.Errorf("pgm_count currently supports only 1")
	}
	if cfg.AUXCount < 0 || cfg.AUXCount > 4 {
		return fmt.Errorf("aux_count must be between 0 and 4")
	}
	if len(cfg.InputSources) > 8 {
		return fmt.Errorf("input_sources cannot exceed 8 entries")
	}
	for i, src := range cfg.InputSources {
		source := strings.TrimSpace(src)
		if source == "" {
			return fmt.Errorf("input_sources[%d] is empty", i)
		}
		if !strings.HasPrefix(source, "srt://") {
			return fmt.Errorf("input_sources[%d] must be srt:// URI", i)
		}
	}
	for k, v := range cfg.AUXSources {
		if k != "1" && k != "2" && k != "3" && k != "4" {
			return fmt.Errorf("aux_sources keys must be 1..4")
		}
		source := strings.TrimSpace(v)
		if source == "" {
			return fmt.Errorf("aux_sources[%s] is empty", k)
		}
		if strings.HasPrefix(source, "srt://") {
			continue
		}
		if !strings.HasPrefix(source, "input") {
			return fmt.Errorf("aux_sources[%s] must be inputN or srt:// URI", k)
		}
		idxRaw := strings.TrimPrefix(source, "input")
		idx, err := strconv.Atoi(idxRaw)
		if err != nil {
			return fmt.Errorf("aux_sources[%s] must be inputN or srt:// URI", k)
		}
		if idx < 1 || idx > cfg.Inputs {
			return fmt.Errorf("aux_sources[%s] input index out of range", k)
		}
	}
	return nil
}

func (s *Server) setRuntimeConfig(cfg runtimeConfig) {
	s.runtimeMu.Lock()
	defer s.runtimeMu.Unlock()
	s.runtimeCfg = cfg
	s.runtimeUpdatedAt = time.Now().UTC().Unix()
	if s.runtimeStore != nil {
		_ = s.runtimeStore.Save(s.runtimeCfg, s.runtimeUpdatedAt)
	}
}

func (s *Server) getRuntimeConfig() (runtimeConfig, int64) {
	s.runtimeMu.RLock()
	defer s.runtimeMu.RUnlock()
	return s.runtimeCfg, s.runtimeUpdatedAt
}

func firstErr(errs ...error) string {
	for _, err := range errs {
		if err != nil {
			return err.Error()
		}
	}
	return ""
}

func extractConfigPayload(raw []byte) json.RawMessage {
	var wrapped struct {
		Config json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal(raw, &wrapped); err == nil && len(wrapped.Config) > 0 {
		return wrapped.Config
	}
	return json.RawMessage(raw)
}

func (s *Server) routeControlGET(path string) ([]byte, int, error) {
	base := strings.TrimRight(s.cfg.RouteControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	s.attachRouteServiceToken(req)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func (s *Server) engineControlGET(path string) ([]byte, int, error) {
	base := strings.TrimRight(s.cfg.EngineControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodGet, target, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.attachEngineServiceToken(req)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func (s *Server) engineControlPOST(path string, body []byte) ([]byte, int, error) {
	base := strings.TrimRight(s.cfg.EngineControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.attachEngineServiceToken(req)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func randomToken(n int) string {
	if n <= 0 {
		n = 12
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	}
	return base64.RawURLEncoding.EncodeToString(buf)
}

func randomDigits(n int) string {
	if n <= 0 {
		n = 6
	}
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return "000000"
	}
	for i := range b {
		b[i] = '0' + (b[i] % 10)
	}
	return string(b)
}

func (s *Server) routeControlPOST(path string, body []byte) ([]byte, int, error) {
	base := strings.TrimRight(s.cfg.RouteControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.attachRouteServiceToken(req)
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}

func (s *Server) attachRouteServiceToken(req *http.Request) {
	if req == nil {
		return
	}
	if id := strings.TrimSpace(s.cfg.RouteAccessClientID); id != "" {
		req.Header.Set("Cf-Access-Client-Id", id)
	}
	if secret := strings.TrimSpace(s.cfg.RouteAccessClientSecret); secret != "" {
		req.Header.Set("Cf-Access-Client-Secret", secret)
	}
}

func (s *Server) attachEngineServiceToken(req *http.Request) {
	if req == nil {
		return
	}
	if id := strings.TrimSpace(s.cfg.EngineAccessClientID); id != "" {
		req.Header.Set("Cf-Access-Client-Id", id)
	}
	if secret := strings.TrimSpace(s.cfg.EngineAccessClientSecret); secret != "" {
		req.Header.Set("Cf-Access-Client-Secret", secret)
	}
}
