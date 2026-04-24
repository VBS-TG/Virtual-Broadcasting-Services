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
	"net/url"
	"strings"
	"sync"
	"time"

	"vbs/apps/console/internal/auth"
	"vbs/apps/console/internal/config"
	"vbs/apps/console/internal/telemetry"

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
	}
	store, err := openGuestStore(cfg.GuestDBPath)
	if err != nil {
		log.Fatalf("guest store init failed: %v", err)
	}
	s.guestStore = store
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
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("GET /vbs/telemetry/ws", s.handleTelemetryWS)
	s.mux.HandleFunc("GET /vbs/telemetry/events/ws", s.handleTelemetryEventsWS)
	s.mux.HandleFunc("GET /api/v1/telemetry/latest", s.handleTelemetryLatest)
	s.mux.HandleFunc("POST /api/v1/stream/session-key", s.handleSessionKey)
	s.mux.HandleFunc("POST /api/v1/pgm/session", s.handlePGMSession)
	s.mux.HandleFunc("POST /api/v1/aux/session", s.handleAUXSession)
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

type pgmSessionResponse struct {
	StreamUUID      string `json:"stream_uuid"`
	PublishStreamID string `json:"publish_streamid"`
	ReadStreamID    string `json:"read_streamid"`
	PlaybackSRTURL  string `json:"playback_srt_url"`
	RelayHost       string `json:"relay_host,omitempty"`
}

type auxSessionItem struct {
	Channel int `json:"channel"`
	pgmSessionResponse
}

type auxSessionResponse struct {
	AuxCount int              `json:"aux_count"`
	Sessions []auxSessionItem `json:"sessions"`
}

func (s *Server) handlePGMSession(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.RouteControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ROUTE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	respBody, status, err := s.routeControlPOST(r.Header.Get("Authorization"), "/api/v1/route/pgm/session", []byte(`{}`))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"route pgm session proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	if status >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(respBody)
		return
	}
	var out pgmSessionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		http.Error(w, `{"error":"invalid route session payload"}`, http.StatusBadGateway)
		return
	}
	if out.PlaybackSRTURL != "" && s.cfg.PGMDefaultLatencyMs > 0 {
		if u, err := url.Parse(out.PlaybackSRTURL); err == nil {
			q := u.Query()
			if strings.TrimSpace(q.Get("latency")) == "" {
				q.Set("latency", fmt.Sprintf("%d", s.cfg.PGMDefaultLatencyMs))
				u.RawQuery = q.Encode()
				out.PlaybackSRTURL = u.String()
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (s *Server) handleAUXSession(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.RouteControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ROUTE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	respBody, status, err := s.routeControlPOST(r.Header.Get("Authorization"), "/api/v1/route/aux/session", []byte(`{}`))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"route aux session proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	if status >= 400 {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(respBody)
		return
	}
	var out auxSessionResponse
	if err := json.Unmarshal(respBody, &out); err != nil {
		http.Error(w, `{"error":"invalid route aux payload"}`, http.StatusBadGateway)
		return
	}
	if s.cfg.PGMDefaultLatencyMs > 0 {
		for i := range out.Sessions {
			if out.Sessions[i].PlaybackSRTURL == "" {
				continue
			}
			if u, err := url.Parse(out.Sessions[i].PlaybackSRTURL); err == nil {
				q := u.Query()
				if strings.TrimSpace(q.Get("latency")) == "" {
					q.Set("latency", fmt.Sprintf("%d", s.cfg.PGMDefaultLatencyMs))
					u.RawQuery = q.Encode()
					out.Sessions[i].PlaybackSRTURL = u.String()
				}
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
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
	respBody, status, err := s.routeControlPOST(r.Header.Get("Authorization"), "/api/v1/route/buffer", body)
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
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		req.Header.Set("Authorization", auth)
	}
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
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		req.Header.Set("Authorization", auth)
	}
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
	if auth := strings.TrimSpace(r.Header.Get("Authorization")); auth != "" {
		req.Header.Set("Authorization", auth)
	}
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

func (s *Server) routeControlPOST(authorization, path string, body []byte) ([]byte, int, error) {
	base := strings.TrimRight(s.cfg.RouteControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if auth := strings.TrimSpace(authorization); auth != "" {
		req.Header.Set("Authorization", auth)
	}
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
