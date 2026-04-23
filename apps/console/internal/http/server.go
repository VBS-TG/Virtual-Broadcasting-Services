// Package httpserver wires HTTP + WebSocket routes for Console MVP-A.
package httpserver

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
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
	jwt    *auth.Manager
	access *auth.CFAccessVerifier
	hub    *telemetry.Hub
	http   *http.Server
	mux    *http.ServeMux
	upgrader websocket.Upgrader
}

// New constructs a Server from config.
func New(cfg *config.Config) *Server {
	s := &Server{
		cfg: cfg,
		jwt: auth.NewManager(cfg.JWTSecret, cfg.JWTTTL),
		hub: telemetry.NewHub(),
		mux: http.NewServeMux(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
	}
	access, err := auth.NewCFAccessVerifier(cfg.CFAccessMode, cfg.CFAccessTeamDomain, cfg.CFAccessAUD, cfg.CFAccessClientsRaw)
	if err != nil {
		log.Printf("access verifier parse warning: %v", err)
	}
	s.access = access
	s.routes()
	s.http = &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.handleHealthz)
	s.mux.HandleFunc("POST /api/v1/auth/token", s.handleAuthToken)
	s.mux.HandleFunc("POST /api/v1/auth/register", s.handleNodeRegister)
	s.mux.HandleFunc("POST /api/v1/auth/refresh", s.handleAuthRefresh)
	s.mux.HandleFunc("GET /vbs/telemetry/ws", s.handleTelemetryWS)
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
}

// ListenAndServe starts the HTTP server.
func (s *Server) ListenAndServe() error {
	log.Printf("console-server listening on %s", s.cfg.ListenAddr)
	return s.http.ListenAndServe()
}

// Shutdown gracefully stops the server.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.http.Shutdown(ctx)
}

func (s *Server) handleHealthz(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"status":"ok"}`))
}

type tokenRequest struct {
	NodeID string `json:"node_id"`
	Role   string `json:"role"`
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	ExpiresAt   int64  `json:"expires_at_unix"`
}

type registerRequest struct {
	NodeID             string `json:"node_id"`
	Role               string `json:"role"`
	AccessClientID     string `json:"access_client_id"`
	AccessClientSecret string `json:"access_client_secret"`
}

func (s *Server) handleAuthToken(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AdminToken == "" {
		http.Error(w, `{"error":"admin token not configured"}`, http.StatusServiceUnavailable)
		return
	}
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxTokenBodyBytes))
	if err != nil {
		http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
		return
	}
	var req tokenRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	token, exp, err := s.jwt.Mint(req.NodeID, req.Role)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.cfg.JWTTTL.Seconds()),
		ExpiresAt:   exp.Unix(),
	})
}

func (s *Server) handleNodeRegister(w http.ResponseWriter, r *http.Request) {
	if s.access == nil || s.access.Mode() == "" || s.access.Mode() == "disabled" {
		http.Error(w, `{"error":"node registration requires Cloudflare Access (VBS_CF_ACCESS_MODE=service_token)"}`, http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxTokenBodyBytes))
	if err != nil {
		http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
		return
	}
	var req registerRequest
	if len(body) > 0 {
		if err := json.Unmarshal(body, &req); err != nil {
			http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
			return
		}
	}
	// Fallback when edge/proxy strips auth headers: accept same credentials in JSON body.
	if strings.TrimSpace(r.Header.Get("CF-Access-Client-Id")) == "" && strings.TrimSpace(req.AccessClientID) != "" {
		r.Header.Set("CF-Access-Client-Id", strings.TrimSpace(req.AccessClientID))
	}
	if strings.TrimSpace(r.Header.Get("CF-Access-Client-Secret")) == "" && strings.TrimSpace(req.AccessClientSecret) != "" {
		r.Header.Set("CF-Access-Client-Secret", strings.TrimSpace(req.AccessClientSecret))
	}
	if strings.TrimSpace(r.Header.Get("X-VBS-Node-ID")) == "" && strings.TrimSpace(req.NodeID) != "" {
		r.Header.Set("X-VBS-Node-ID", strings.TrimSpace(req.NodeID))
	}
	identity, err := s.access.VerifyRequest(r)
	if err != nil {
		log.Printf(
			"[console][auth/register] unauthorized access identity: %v remote=%s ua=%q has_cf_id=%t has_cf_secret=%t has_body_id=%t has_body_secret=%t node_id_hdr=%q node_id_body=%q",
			err,
			r.RemoteAddr,
			r.UserAgent(),
			strings.TrimSpace(r.Header.Get("CF-Access-Client-Id")) != "",
			strings.TrimSpace(r.Header.Get("CF-Access-Client-Secret")) != "",
			strings.TrimSpace(req.AccessClientID) != "",
			strings.TrimSpace(req.AccessClientSecret) != "",
			strings.TrimSpace(r.Header.Get("X-VBS-Node-ID")),
			strings.TrimSpace(req.NodeID),
		)
		http.Error(w, `{"error":"unauthorized access identity"}`, http.StatusUnauthorized)
		return
	}
	token, exp, err := s.jwt.Mint(identity.NodeID, identity.Role)
	if err != nil {
		http.Error(w, `{"error":"mint failed"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.cfg.JWTTTL.Seconds()),
		ExpiresAt:   exp.Unix(),
	})
}

func (s *Server) handleAuthRefresh(w http.ResponseWriter, r *http.Request) {
	claims, err := s.jwt.ParseBearer(r.Header.Get("Authorization"))
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	token, exp, err := s.jwt.Mint(claims.NodeID, claims.Role)
	if err != nil {
		http.Error(w, `{"error":"refresh failed"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(tokenResponse{
		AccessToken: token,
		TokenType:   "Bearer",
		ExpiresIn:   int64(s.cfg.JWTTTL.Seconds()),
		ExpiresAt:   exp.Unix(),
	})
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
	if s.access != nil && s.access.Mode() != "" && s.access.Mode() != "disabled" {
		if identity, err := s.access.VerifyRequest(r); err == nil && strings.EqualFold(identity.Role, "admin") {
			return true
		}
	}
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return false
	}
	raw := strings.TrimSpace(h[7:])
	if constantTimeEqual(raw, s.cfg.AdminToken) {
		return true
	}
	claims, err := s.jwt.Parse(raw)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(claims.Role), "admin")
}

func constantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}

func (s *Server) handleTelemetryLatest(w http.ResponseWriter, r *http.Request) {
	if s.cfg.AdminToken != "" {
		if s.adminAuthorized(r) {
			s.writeLatest(w)
			return
		}
	}
	claims, err := s.jwt.ParseBearer(r.Header.Get("Authorization"))
	if err != nil || strings.ToLower(strings.TrimSpace(claims.Role)) != "admin" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	s.writeLatest(w)
}

func (s *Server) writeLatest(w http.ResponseWriter) {
	snap := s.hub.Snapshot()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"latest": snap,
	})
}

func (s *Server) handleTelemetryWS(w http.ResponseWriter, r *http.Request) {
	claims, err := s.jwt.ParseBearer(r.Header.Get("Authorization"))
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
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.RouteControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ROUTE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	respBody, status, err := s.routeControlPOST("/api/v1/route/pgm/session", []byte(`{}`))
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
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.cfg.RouteControlBaseURL == "" {
		http.Error(w, `{"error":"VBS_ROUTE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	respBody, status, err := s.routeControlPOST("/api/v1/route/aux/session", []byte(`{}`))
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
	if !s.adminAuthorized(r) {
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
	if !s.adminAuthorized(r) {
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
	if token := strings.TrimSpace(s.cfg.RouteControlToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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
	if !s.adminAuthorized(r) {
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
	if token := strings.TrimSpace(s.cfg.EngineControlToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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
	if !s.adminAuthorized(r) {
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
	if token := strings.TrimSpace(s.cfg.EngineControlToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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

func (s *Server) routeControlPOST(path string, body []byte) ([]byte, int, error) {
	base := strings.TrimRight(s.cfg.RouteControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token := strings.TrimSpace(s.cfg.RouteControlToken); token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
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
