// Package httpserver wires HTTP + WebSocket routes for Console MVP-A.
package httpserver

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"sort"
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
const maxProxyBodyBytes = 1 << 20

// upstreamHTTPClient is used for outbound calls to BFF / engine / route / capture control planes.
// Redirects are not followed so misconfigured upstreams cannot silently change the request target.
func upstreamHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// Server is the console HTTP server.
type Server struct {
	cfg      *config.Config
	access   *auth.AccessJWTVerifier
	hub      *telemetry.Hub
	http     *http.Server
	mux      *http.ServeMux
	upgrader websocket.Upgrader

	eventMu      sync.Mutex
	eventConns   map[*websocket.Conn]struct{}
	guestStore   *guestStore
	runtimeStore *runtimeStore
	showStore    *showConfigStore

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
	Inputs     int               `json:"inputs"`
	PGMCount   int               `json:"pgm_count"`
	AUXCount   int               `json:"aux_count"`
	AUXSources map[string]string `json:"aux_sources,omitempty"`
}

type runtimeInput struct {
	ID     string `json:"id"`
	Class  string `json:"class"` // capture | other
	Label  string `json:"label,omitempty"`
	Origin string `json:"origin,omitempty"`
	Online bool   `json:"online"`
}

type controlError struct {
	Status  int
	Code    string
	Message string
}

func (e *controlError) Error() string {
	if e == nil {
		return ""
	}
	return e.Message
}

// New constructs a Server from config.
func New(cfg *config.Config) *Server {
	s := &Server{
		cfg:        cfg,
		hub:        telemetry.NewHub(cfg.NodeOfflineTTL),
		mux:        http.NewServeMux(),
		eventConns: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
		},
		runtimeCfg: runtimeConfig{
			Inputs:   1,
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
	scStore, err := openShowConfigStore(cfg.ShowConfigDBPath)
	if err != nil {
		log.Fatalf("show config store init failed: %v", err)
	}
	s.showStore = scStore
	if savedCfg, savedAt, err := s.runtimeStore.Load(); err == nil {
		if err := validateRuntimeConfig(savedCfg); err == nil {
			s.runtimeCfg = savedCfg
			s.runtimeUpdatedAt = savedAt
		}
	}
	access, err := auth.NewAccessJWTVerifier(
		cfg.CFAccessTeamDomain,
		cfg.CFAccessAUD,
		cfg.CFAccessJWKSURL,
		cfg.CFAccessJWKSCacheTTL,
		cfg.JWTClockSkewLeeway,
		cfg.AdminEmails,
		cfg.NodeCNPrefix,
		cfg.RouteAccessClientID,
		cfg.EngineAccessClientID,
		cfg.CaptureAccessClientID,
		cfg.BFFProxyAccessClientID,
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
			w.Header().Set("Access-Control-Allow-Headers", "X-VBS-Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		if err := s.authorizeServiceRequest(r); err != nil {
			status := http.StatusForbidden
			if strings.HasPrefix(strings.ToLower(err.Error()), "unauthorized:") {
				status = http.StatusUnauthorized
			}
			http.Error(w, fmt.Sprintf(`{"error":"%s"}`, trimErr(err)), status)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) authorizeServiceRequest(r *http.Request) error {
	path := auth.NormalizeRBACPath(r.URL.Path)
	method := strings.TrimSpace(strings.ToUpper(r.Method))
	if path == "/healthz" || path == "/api/v1/auth/admin/email-login" || path == "/api/v1/guest/exchange-pin" {
		return nil
	}
	authzPreview := headerPreview(r.Header.Get("Authorization"), 20)
	xVBSAuthzPreview := headerPreview(r.Header.Get("X-VBS-Authorization"), 20)
	cfJWTPreview := headerPreview(r.Header.Get("Cf-Access-Jwt-Assertion"), 20)
	hasHumanAuthHeader := strings.TrimSpace(r.Header.Get("X-VBS-Authorization")) != "" || strings.TrimSpace(r.Header.Get("Authorization")) != ""
	claims, err := s.access.VerifyRequestPreferBearer(r)
	if err != nil {
		log.Printf("[auth-debug][middleware] method=%s path=%s authz=%q x_vbs_authz=%q cf_jwt=%q role=<verify_error> err=%v", r.Method, r.URL.Path, authzPreview, xVBSAuthzPreview, cfJWTPreview, err)
		if hasHumanAuthHeader {
			// Fail fast: explicit human token must verify or return 401.
			return fmt.Errorf("unauthorized: %w", err)
		}
		// Preserve legacy behavior for completely unauthenticated paths.
		return nil
	}
	role := strings.TrimSpace(strings.ToLower(claims.Role))
	log.Printf("[auth-debug][middleware] method=%s path=%s authz=%q x_vbs_authz=%q cf_jwt=%q role=%q", r.Method, r.URL.Path, authzPreview, xVBSAuthzPreview, cfJWTPreview, role)
	if role == "guest" {
		return nil
	}
	if role == "admin" {
		if auth.AdminMiddlewareAllowed(method, path) {
			return nil
		}
		log.Printf("[auth-debug] Access Denied: Role=%s, Method=%s, Path=%s", role, method, path)
		return fmt.Errorf("forbidden: role %s cannot access %s %s", role, method, path)
	}
	if role == "" {
		return nil
	}
	if s.servicePathAllowed(role, method, path) {
		return nil
	}
	log.Printf("[auth-debug] Access Denied: Role=%s, Method=%s, Path=%s", role, method, path)
	return fmt.Errorf("forbidden: role %s cannot access %s %s", role, method, path)
}

func (s *Server) servicePathAllowed(role, method, path string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	method = strings.TrimSpace(strings.ToUpper(method))
	path = auth.NormalizeRBACPath(path)
	for _, rule := range serviceACLRules {
		if !rule.matchesRole(role) {
			continue
		}
		if !rule.matchesMethod(method) {
			continue
		}
		if rule.matchesPath(path) {
			return true
		}
	}
	return false
}

type serviceACLRule struct {
	roles         map[string]struct{}
	allowAllRoles bool
	methods       map[string]struct{}
	prefixes      []string
	exacts        []string
}

func (r serviceACLRule) matchesRole(role string) bool {
	if r.allowAllRoles {
		return true
	}
	_, ok := r.roles[role]
	return ok
}

func (r serviceACLRule) matchesMethod(method string) bool {
	if len(r.methods) == 0 {
		return true
	}
	_, ok := r.methods[method]
	return ok
}

func (r serviceACLRule) matchesPath(path string) bool {
	path = auth.NormalizeRBACPath(path)
	for _, exact := range r.exacts {
		if path == auth.NormalizeRBACPath(exact) {
			return true
		}
	}
	for _, prefix := range r.prefixes {
		if strings.HasPrefix(path, auth.NormalizeRBACPath(prefix)) {
			return true
		}
	}
	return false
}

func set(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" {
			out[v] = struct{}{}
		}
	}
	return out
}

func methodSet(values ...string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, v := range values {
		v = strings.TrimSpace(strings.ToUpper(v))
		if v != "" {
			out[v] = struct{}{}
		}
	}
	return out
}

var serviceACLRules = []serviceACLRule{
	{
		roles:    set("bff"),
		prefixes: []string{"/api/proxy/"},
	},
	{
		roles:    set("node", "route", "engine", "capture", "console"),
		methods:  methodSet(http.MethodGet),
		prefixes: []string{"/vbs/telemetry/ws"},
	},
	{
		roles:   set("node", "route", "engine", "capture", "console"),
		methods: methodSet(http.MethodPost),
		exacts:  []string{"/api/v1/guest/introspect"},
	},
	{
		roles:   set("console"),
		methods: methodSet(http.MethodGet),
		exacts:  []string{"/vbs/telemetry/events/ws", "/vbs/control/ws", "/api/v1/telemetry/latest"},
	},
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
	s.mux.HandleFunc("/api/proxy/", s.handleBFFProxy)
	s.mux.HandleFunc("GET /api/v1/auth/session", s.handleAuthSession)
	s.mux.HandleFunc("POST /api/v1/auth/admin/email-login", s.handleAdminEmailLogin)
	s.mux.HandleFunc("GET /vbs/telemetry/ws", s.handleTelemetryWS)
	s.mux.HandleFunc("GET /vbs/telemetry/events/ws", s.handleTelemetryEventsWS)
	s.mux.HandleFunc("GET /vbs/control/ws", s.handleControlWS)
	s.mux.HandleFunc("GET /api/v1/telemetry/latest", s.handleTelemetryLatest)
	s.mux.HandleFunc("POST /api/v1/stream/session-key", s.handleSessionKey)
	s.mux.HandleFunc("POST /api/v1/pgm/route-buffer", s.handlePGMRouteBuffer)
	s.mux.HandleFunc("POST /api/v1/capture/bitrate", s.handleCaptureBitrate)
	s.mux.HandleFunc("POST /api/v1/capture/reboot", s.handleCaptureReboot)
	s.mux.HandleFunc("GET /api/v1/route/metrics", s.handleRouteMetrics)
	s.mux.HandleFunc("POST /api/v1/switch/program", s.handleSwitchProgram)
	s.mux.HandleFunc("POST /api/v1/switch/preview", s.handleSwitchPreview)
	s.mux.HandleFunc("POST /api/v1/switch/aux", s.handleSwitchAUX)
	s.mux.HandleFunc("GET /api/v1/switch/state", s.handleSwitchState)
	s.mux.HandleFunc("POST /api/v1/engine/reset", s.handleEngineReset)
	s.mux.HandleFunc("POST /api/v1/engine/pgm/output", s.handleEnginePGMOutput)
	s.mux.HandleFunc("GET /api/v1/guest/sessions", s.handleGuestSessionList)
	s.mux.HandleFunc("POST /api/v1/guest/sessions", s.handleGuestSessionCreate)
	s.mux.HandleFunc("DELETE /api/v1/guest/sessions/{id}", s.handleGuestSessionDelete)
	s.mux.HandleFunc("POST /api/v1/guest/exchange-pin", s.handleGuestExchangePIN)
	s.mux.HandleFunc("POST /api/v1/guest/introspect", s.handleGuestIntrospect)
	s.mux.HandleFunc("GET /api/v1/runtime/config", s.handleRuntimeConfigGet)
	s.mux.HandleFunc("PUT /api/v1/runtime/config", s.handleRuntimeConfigPut)
	s.mux.HandleFunc("POST /api/v1/runtime/config/apply", s.handleRuntimeConfigApply)
	s.mux.HandleFunc("GET /api/v1/show-config", s.handleShowConfigGet)
	s.mux.HandleFunc("PUT /api/v1/show-config/draft", s.handleShowConfigDraftPut)
	s.mux.HandleFunc("POST /api/v1/show-config/apply", s.handleShowConfigApply)
	s.mux.HandleFunc("POST /api/v1/show-config/rollback", s.handleShowConfigRollback)
	s.mux.HandleFunc("GET /api/v1/show-config/history", s.handleShowConfigHistory)
}

func (s *Server) handleBFFProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost && r.Method != http.MethodPut && r.Method != http.MethodDelete {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	base := strings.TrimSpace(s.cfg.BFFProxyBaseURL)
	if base == "" {
		http.Error(w, `{"error":"VBS_BFF_PROXY_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	trimmedPath := strings.TrimPrefix(r.URL.Path, "/api/proxy")
	if trimmedPath == r.URL.Path || trimmedPath == "" {
		http.Error(w, `{"error":"invalid proxy path"}`, http.StatusBadRequest)
		return
	}
	if !strings.HasPrefix(trimmedPath, "/api/v1/") && trimmedPath != "/healthz" {
		http.Error(w, `{"error":"forbidden proxy target"}`, http.StatusForbidden)
		return
	}
	upstreamURL, err := url.Parse(strings.TrimRight(base, "/") + trimmedPath)
	if err != nil {
		http.Error(w, `{"error":"invalid proxy target"}`, http.StatusInternalServerError)
		return
	}
	upstreamURL.RawQuery = r.URL.RawQuery

	body, err := io.ReadAll(io.LimitReader(r.Body, maxProxyBodyBytes))
	if err != nil {
		http.Error(w, `{"error":"read proxy body failed"}`, http.StatusBadRequest)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, r.Method, upstreamURL.String(), strings.NewReader(string(body)))
	if err != nil {
		http.Error(w, `{"error":"build proxy request failed"}`, http.StatusInternalServerError)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if x := strings.TrimSpace(r.Header.Get("X-VBS-Authorization")); x != "" {
		req.Header.Set("X-VBS-Authorization", x)
	}
	if accept := strings.TrimSpace(r.Header.Get("Accept")); accept != "" {
		req.Header.Set("Accept", accept)
	}
	s.attachBFFProxyServiceToken(req)

	client := upstreamHTTPClient(12 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bff proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxProxyBodyBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"bff proxy read failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	contentType := strings.TrimSpace(resp.Header.Get("Content-Type"))
	if contentType == "" {
		contentType = "application/json"
	}
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(raw)
}

func (s *Server) handleAdminEmailLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
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
	if !containsNormalizedEmail(s.cfg.AdminEmails, email) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var err error
	ttl := 8 * time.Hour
	token, err := s.access.MintAdminToken("admin:"+email, ttl)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"mint admin token failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": token,
		"token_type":   "Bearer",
		"expires_at":   time.Now().UTC().Add(ttl).Unix(),
		"role":         "admin",
		"email":        email,
	})
}

func containsNormalizedEmail(allow []string, email string) bool {
	needle := strings.TrimSpace(strings.ToLower(email))
	if needle == "" {
		return false
	}
	for _, v := range allow {
		if strings.EqualFold(strings.TrimSpace(v), needle) {
			return true
		}
	}
	return false
}

// handleAuthSession returns current authenticated session role for UI gating.
// It prefers application bearer token first (admin/guest), then Cf-Access JWT.
func (s *Server) handleAuthSession(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	claims, err := s.access.VerifyRequestPreferBearer(r)
	if err != nil {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	role := strings.TrimSpace(strings.ToLower(claims.Role))
	if role != "admin" && role != "guest" {
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
		return
	}
	active := true
	if role == "guest" {
		guestID := strings.TrimPrefix(strings.TrimSpace(claims.Subject), "guest:")
		active = s.guestStore != nil && s.guestStore.ValidateTokenSession(guestID, claims.SessionVersion)
		if !active {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      true,
		"role":    role,
		"subject": strings.TrimSpace(claims.Subject),
		"active":  active,
		"email":   strings.TrimSpace(claims.Email),
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
	if s.showStore != nil {
		_ = s.showStore.Close()
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
	claims, err := s.access.VerifyRequestPreferBearer(r)
	if err != nil {
		return false
	}
	return auth.IsAdminRole(claims.Role)
}

func (s *Server) controlAuthorized(r *http.Request) bool {
	claims, err := s.access.VerifyRequestPreferBearer(r)
	if err != nil {
		return false
	}
	role := strings.TrimSpace(strings.ToLower(claims.Role))
	if role == "bff" {
		return strings.HasPrefix(strings.TrimSpace(r.URL.Path), "/api/proxy/")
	}
	if !auth.CanControlPlane(claims.Role) {
		return false
	}
	if auth.IsGuestRole(claims.Role) {
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
		"latest":   snap,
		"presence": presence,
		"ts_ms":    time.Now().UTC().UnixMilli(),
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

func (s *Server) handleControlWS(w http.ResponseWriter, r *http.Request) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("control websocket upgrade: %v", err)
		return
	}
	defer conn.Close()

	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(120 * time.Second))
		return nil
	})

	for {
		_, payload, err := conn.ReadMessage()
		if err != nil {
			return
		}
		resp := s.executeControlCommand(payload)
		_ = conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
		if err := conn.WriteMessage(websocket.TextMessage, resp); err != nil {
			return
		}
	}
}

func (s *Server) executeControlCommand(raw []byte) []byte {
	type controlCommand struct {
		Action  string         `json:"action"`
		Payload map[string]any `json:"payload"`
	}
	out := map[string]any{
		"ok": false,
	}
	var cmd controlCommand
	if err := json.Unmarshal(raw, &cmd); err != nil {
		out["error"] = "invalid json"
		b, _ := json.Marshal(out)
		return b
	}
	action := strings.TrimSpace(strings.ToLower(cmd.Action))
	payload := cmd.Payload
	if payload == nil {
		payload = map[string]any{}
	}
	if verr := validateControlActionPayload(action, payload); verr != nil {
		out["code"] = verr.Code
		out["error"] = verr.Message
		b, _ := json.Marshal(out)
		return b
	}
	callEngine := func(path string) {
		body, _ := json.Marshal(payload)
		proxyRaw, status, err := s.engineControlPOST(path, body)
		if err != nil {
			out["error"] = trimErr(err)
			return
		}
		out["status"] = status
		if status >= 400 {
			out["error"] = fmt.Sprintf("engine status=%d", status)
		} else {
			out["ok"] = true
		}
		out["response"] = json.RawMessage(proxyRaw)
	}
	switch action {
	case "switch_program":
		callEngine("/api/v1/switch/program")
	case "switch_preview":
		callEngine("/api/v1/switch/preview")
	case "switch_aux":
		callEngine("/api/v1/switch/aux")
	case "engine_reset":
		callEngine("/api/v1/engine/reset")
	case "engine_pgm_output":
		callEngine("/api/v1/engine/pgm/output")
	case "route_buffer":
		body, _ := json.Marshal(payload)
		proxyRaw, status, err := s.routeControlPOST("/api/v1/route/buffer", body)
		if err != nil {
			out["error"] = trimErr(err)
			break
		}
		out["status"] = status
		out["response"] = json.RawMessage(proxyRaw)
		out["ok"] = status < 400
	case "capture_bitrate":
		body, _ := json.Marshal(payload)
		proxyRaw, status, err := s.captureControlPOST("/api/v1/capture/bitrate", body)
		if err != nil {
			out["error"] = trimErr(err)
			break
		}
		out["status"] = status
		out["response"] = json.RawMessage(proxyRaw)
		out["ok"] = status < 400
	case "capture_reboot":
		body, _ := json.Marshal(payload)
		proxyRaw, status, err := s.captureControlPOST("/api/v1/capture/reboot", body)
		if err != nil {
			out["error"] = trimErr(err)
			break
		}
		out["status"] = status
		out["response"] = json.RawMessage(proxyRaw)
		out["ok"] = status < 400
	default:
		out["code"] = "unsupported_action"
		out["error"] = "unsupported action"
	}
	b, _ := json.Marshal(out)
	return b
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

func writeControlError(w http.ResponseWriter, err *controlError) {
	status := http.StatusBadRequest
	code := "bad_request"
	msg := "invalid payload"
	if err != nil {
		if err.Status > 0 {
			status = err.Status
		}
		if strings.TrimSpace(err.Code) != "" {
			code = strings.TrimSpace(err.Code)
		}
		if strings.TrimSpace(err.Message) != "" {
			msg = strings.TrimSpace(err.Message)
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":      false,
		"code":    code,
		"error":   msg,
		"message": msg,
	})
}

func parseControlPayload(raw []byte) (map[string]any, *controlError) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, &controlError{Status: http.StatusBadRequest, Code: "invalid_json", Message: "invalid json"}
	}
	if payload == nil {
		payload = map[string]any{}
	}
	return payload, nil
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case float64:
		if n == float64(int(n)) {
			return int(n), true
		}
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		if err == nil {
			return i, true
		}
	}
	return 0, false
}

func validateSourceValue(source string) bool {
	s := strings.TrimSpace(source)
	if s == "" {
		return false
	}
	return strings.HasPrefix(s, "input") ||
		strings.HasPrefix(s, "srt://") ||
		strings.HasPrefix(s, "capture:")
}

func validateSwitchProgramPreviewPayload(payload map[string]any) *controlError {
	source := strings.TrimSpace(fmt.Sprintf("%v", payload["source"]))
	if !validateSourceValue(source) {
		return &controlError{Status: http.StatusBadRequest, Code: "invalid_source", Message: "source must be inputN, capture:* or srt:// URI"}
	}
	return nil
}

func validateSwitchAUXPayload(payload map[string]any) *controlError {
	channel, ok := asInt(payload["channel"])
	if !ok || channel < 1 || channel > 20 {
		return &controlError{Status: http.StatusBadRequest, Code: "invalid_channel", Message: "channel must be 1..20"}
	}
	source := strings.TrimSpace(fmt.Sprintf("%v", payload["source"]))
	if !validateSourceValue(source) {
		return &controlError{Status: http.StatusBadRequest, Code: "invalid_source", Message: "source must be inputN, capture:* or srt:// URI"}
	}
	return nil
}

func validateRouteBufferPayload(payload map[string]any) *controlError {
	latRaw, hasLat := payload["latency_ms"]
	lossRaw, hasLoss := payload["loss_max_ttl"]
	if !hasLat && !hasLoss {
		return &controlError{Status: http.StatusBadRequest, Code: "missing_field", Message: "latency_ms or loss_max_ttl is required"}
	}
	if hasLat {
		lat, ok := asInt(latRaw)
		if !ok || lat <= 0 {
			return &controlError{Status: http.StatusBadRequest, Code: "invalid_latency_ms", Message: "latency_ms must be positive integer"}
		}
	}
	if hasLoss {
		loss, ok := asInt(lossRaw)
		if !ok || loss <= 0 {
			return &controlError{Status: http.StatusBadRequest, Code: "invalid_loss_max_ttl", Message: "loss_max_ttl must be positive integer"}
		}
	}
	return nil
}

func validateEnginePGMOutputPayload(payload map[string]any) *controlError {
	enabledRaw, ok := payload["enabled"]
	if !ok {
		return &controlError{Status: http.StatusBadRequest, Code: "missing_field", Message: "enabled is required"}
	}
	enabled, ok := enabledRaw.(bool)
	if !ok {
		return &controlError{Status: http.StatusBadRequest, Code: "invalid_enabled", Message: "enabled must be boolean"}
	}
	url := strings.TrimSpace(fmt.Sprintf("%v", payload["url"]))
	if enabled && url == "" {
		return &controlError{Status: http.StatusBadRequest, Code: "missing_field", Message: "url is required when enabled=true"}
	}
	return nil
}

func validateCaptureBitratePayload(payload map[string]any) *controlError {
	if len(payload) == 0 {
		return &controlError{Status: http.StatusBadRequest, Code: "missing_field", Message: "payload is required"}
	}
	if v, ok := payload["bitrate_kbps"]; ok {
		bitrate, ok := asInt(v)
		if !ok || bitrate <= 0 {
			return &controlError{Status: http.StatusBadRequest, Code: "invalid_bitrate_kbps", Message: "bitrate_kbps must be positive integer"}
		}
	}
	return nil
}

func validateCaptureRebootPayload(payload map[string]any) *controlError {
	if len(payload) == 0 {
		return &controlError{Status: http.StatusBadRequest, Code: "missing_field", Message: "payload is required"}
	}
	return nil
}

func validateControlActionPayload(action string, payload map[string]any) *controlError {
	switch action {
	case "switch_program", "switch_preview":
		return validateSwitchProgramPreviewPayload(payload)
	case "switch_aux":
		return validateSwitchAUXPayload(payload)
	case "engine_reset":
		if len(payload) != 0 {
			return &controlError{Status: http.StatusBadRequest, Code: "unexpected_payload", Message: "engine_reset does not accept payload"}
		}
		return nil
	case "engine_pgm_output":
		return validateEnginePGMOutputPayload(payload)
	case "route_buffer":
		return validateRouteBufferPayload(payload)
	case "capture_bitrate":
		return validateCaptureBitratePayload(payload)
	case "capture_reboot":
		return validateCaptureRebootPayload(payload)
	default:
		return &controlError{Status: http.StatusBadRequest, Code: "unsupported_action", Message: "unsupported action"}
	}
}

func validateProxyPathPayload(path string, payload map[string]any) *controlError {
	switch path {
	case "/api/v1/switch/program", "/api/v1/switch/preview":
		return validateSwitchProgramPreviewPayload(payload)
	case "/api/v1/switch/aux":
		return validateSwitchAUXPayload(payload)
	case "/api/v1/engine/reset":
		if len(payload) != 0 {
			return &controlError{Status: http.StatusBadRequest, Code: "unexpected_payload", Message: "engine reset does not accept payload"}
		}
		return nil
	case "/api/v1/engine/pgm/output":
		return validateEnginePGMOutputPayload(payload)
	case "/api/v1/route/buffer":
		return validateRouteBufferPayload(payload)
	case "/api/v1/capture/bitrate":
		return validateCaptureBitratePayload(payload)
	case "/api/v1/capture/reboot":
		return validateCaptureRebootPayload(payload)
	default:
		return nil
	}
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
		writeControlError(w, &controlError{Status: http.StatusBadRequest, Code: "read_body_failed", Message: "read body"})
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeControlError(w, &controlError{Status: http.StatusBadRequest, Code: "missing_body", Message: "body required"})
		return
	}
	payload, verr := parseControlPayload(body)
	if verr != nil {
		writeControlError(w, verr)
		return
	}
	if verr := validateProxyPathPayload("/api/v1/route/buffer", payload); verr != nil {
		writeControlError(w, verr)
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
	client := upstreamHTTPClient(10 * time.Second)
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

func (s *Server) handleEngineReset(w http.ResponseWriter, r *http.Request) {
	s.proxyEngineControl(w, r, "/api/v1/engine/reset")
}

func (s *Server) handleEnginePGMOutput(w http.ResponseWriter, r *http.Request) {
	s.proxyEngineControl(w, r, "/api/v1/engine/pgm/output")
}

func (s *Server) handleCaptureBitrate(w http.ResponseWriter, r *http.Request) {
	s.proxyCaptureControl(w, r, "/api/v1/capture/bitrate")
}

func (s *Server) handleCaptureReboot(w http.ResponseWriter, r *http.Request) {
	s.proxyCaptureControl(w, r, "/api/v1/capture/reboot")
}

func (s *Server) handleSwitchState(w http.ResponseWriter, r *http.Request) {
	authzPreview := headerPreview(r.Header.Get("Authorization"), 20)
	xVBSAuthzPreview := headerPreview(r.Header.Get("X-VBS-Authorization"), 20)
	cfJWTPreview := headerPreview(r.Header.Get("Cf-Access-Jwt-Assertion"), 20)
	claims, err := s.access.VerifyRequestPreferBearer(r)
	finalRole := ""
	if err != nil {
		log.Printf("[auth-debug][switch-state] authz=%q x_vbs_authz=%q cf_jwt=%q role=<verify_error> err=%v", authzPreview, xVBSAuthzPreview, cfJWTPreview, err)
	} else {
		finalRole = strings.TrimSpace(strings.ToLower(claims.Role))
		log.Printf("[auth-debug][switch-state] authz=%q x_vbs_authz=%q cf_jwt=%q role=%q", authzPreview, xVBSAuthzPreview, cfJWTPreview, finalRole)
	}

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
	client := upstreamHTTPClient(10 * time.Second)
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

func headerPreview(raw string, n int) string {
	v := strings.TrimSpace(raw)
	if v == "" || n <= 0 {
		return ""
	}
	runes := []rune(v)
	if len(runes) <= n {
		return v
	}
	return string(runes[:n])
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
		writeControlError(w, &controlError{Status: http.StatusBadRequest, Code: "read_body_failed", Message: "read body"})
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeControlError(w, &controlError{Status: http.StatusBadRequest, Code: "missing_body", Message: "body required"})
		return
	}
	payload, verr := parseControlPayload(body)
	if verr != nil {
		writeControlError(w, verr)
		return
	}
	if verr := validateProxyPathPayload(path, payload); verr != nil {
		writeControlError(w, verr)
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
	client := upstreamHTTPClient(10 * time.Second)
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

func (s *Server) proxyCaptureControl(w http.ResponseWriter, r *http.Request, path string) {
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if strings.TrimSpace(s.cfg.CaptureControlBaseURL) == "" {
		http.Error(w, `{"error":"VBS_CAPTURE_CONTROL_BASE_URL not configured"}`, http.StatusServiceUnavailable)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxTokenBodyBytes))
	if err != nil {
		writeControlError(w, &controlError{Status: http.StatusBadRequest, Code: "read_body_failed", Message: "read body"})
		return
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		writeControlError(w, &controlError{Status: http.StatusBadRequest, Code: "missing_body", Message: "body required"})
		return
	}
	payload, verr := parseControlPayload(body)
	if verr != nil {
		writeControlError(w, verr)
		return
	}
	if verr := validateProxyPathPayload(path, payload); verr != nil {
		writeControlError(w, verr)
		return
	}
	raw, status, err := s.captureControlPOST(path, body)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"capture control proxy failed: %s"}`, trimErr(err)), http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}

func (s *Server) handleGuestSessionCreate(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var body struct {
		Name       string `json:"name"`
		TTLSeconds int    `json:"ttl_seconds"`
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
		ID:             id,
		Name:           name,
		PIN:            pin,
		SessionVersion: 1,
		CreatedAt:      now.Unix(),
		ExpiresAt:      now.Add(ttl).Unix(),
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
		"id":           session.ID,
		"name":         session.Name,
		"pin":          session.PIN,
		"expires_at":   session.ExpiresAt,
		"access_token": token,
		"token_type":   "Bearer",
		"magic_link":   fmt.Sprintf("/guest?pin=%s", session.PIN),
	})
}

func (s *Server) handleGuestSessionList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.guestStore == nil {
		http.Error(w, `{"error":"guest store unavailable"}`, http.StatusInternalServerError)
		return
	}
	list, err := s.guestStore.ListActive()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"list guest sessions failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for _, g := range list {
		out = append(out, map[string]any{
			"id":         g.ID,
			"name":       g.Name,
			"pin":        g.PIN,
			"expires_at": g.ExpiresAt,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"sessions": out})
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
		"token_type":   "Bearer",
		"expires_at":   session.ExpiresAt,
	})
}

func (s *Server) handleGuestIntrospect(w http.ResponseWriter, r *http.Request) {
	claims, err := s.access.VerifyRequestPreferBearer(r)
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
	cfg, updatedAt, inputs := s.getRuntimeConfigSnapshot()
	captureCount := 0
	otherCount := 0
	for _, in := range inputs {
		if in.Class == "capture" {
			captureCount++
		} else {
			otherCount++
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"config":              cfg,
		"updated_at":          updatedAt,
		"inputs":              inputs,
		"capture_inputs":      captureCount,
		"other_inputs":        otherCount,
		"runtime_auto_inputs": true,
	})
}

func (s *Server) handleRuntimeConfigPut(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	var body struct {
		PGMCount   *int              `json:"pgm_count"`
		AUXCount   *int              `json:"aux_count"`
		AUXSources map[string]string `json:"aux_sources"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, maxTokenBodyBytes)).Decode(&body); err != nil {
		http.Error(w, `{"error":"invalid body"}`, http.StatusBadRequest)
		return
	}
	current, _ := s.getRuntimeConfig()
	next := runtimeConfig{
		Inputs:     current.Inputs, // auto-discovered, cannot be set manually
		PGMCount:   current.PGMCount,
		AUXCount:   current.AUXCount,
		AUXSources: body.AUXSources,
	}
	if body.PGMCount != nil {
		next.PGMCount = *body.PGMCount
	}
	if body.AUXCount != nil {
		next.AUXCount = *body.AUXCount
	}
	if next.AUXSources == nil {
		next.AUXSources = current.AUXSources
	}
	if err := validateRuntimeConfig(next); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, trimErr(err)), http.StatusBadRequest)
		return
	}
	s.setRuntimeConfig(next)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saved":               true,
		"config":              next,
		"updated_at":          s.runtimeUpdatedAt,
		"runtime_auto_inputs": true,
	})
}

func (s *Server) handleRuntimeConfigApply(w http.ResponseWriter, r *http.Request) {
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	cfg, updatedAt, _ := s.getRuntimeConfigSnapshot()
	cfgBody, _ := json.Marshal(cfg)
	result := map[string]any{
		"config":     cfg,
		"updated_at": updatedAt,
		"route":      map[string]any{"ok": false},
		"engine":     map[string]any{"ok": false},
		"applied_at": time.Now().UTC().Unix(),
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
			"ok":           routeApplied,
			"apply_status": applyStatus,
			"apply_raw":    string(applyBody),
			"error":        firstErr(applyErr),
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

	if strings.TrimSpace(s.cfg.RouteControlBaseURL) == "" {
		result["route"] = map[string]any{"ok": true, "skipped": true, "reason": "VBS_ROUTE_CONTROL_BASE_URL 未設定"}
		routeApplied = true
	}
	if strings.TrimSpace(s.cfg.EngineControlBaseURL) == "" {
		result["engine"] = map[string]any{"ok": true, "skipped": true, "reason": "VBS_ENGINE_CONTROL_BASE_URL 未設定"}
		engineApplied = true
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
	if cfg.Inputs < 1 {
		cfg.Inputs = 1
	}
	if cfg.Inputs > 8 {
		cfg.Inputs = 8
	}
	if cfg.PGMCount < 1 || cfg.PGMCount > 5 {
		return fmt.Errorf("pgm_count must be between 1 and 5")
	}
	if cfg.AUXCount < 0 || cfg.AUXCount > 20 {
		return fmt.Errorf("aux_count must be between 0 and 20")
	}
	for k, v := range cfg.AUXSources {
		ch, err := strconv.Atoi(strings.TrimSpace(k))
		if err != nil || ch < 1 || ch > 20 {
			return fmt.Errorf("aux_sources keys must be 1..20")
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

func (s *Server) getRuntimeConfigSnapshot() (runtimeConfig, int64, []runtimeInput) {
	base, updatedAt := s.getRuntimeConfig()
	inputs := s.discoverRuntimeInputs()
	inputCount := len(inputs)
	if inputCount < 1 {
		inputCount = 1
	}
	if inputCount > 8 {
		inputCount = 8
	}
	base.Inputs = inputCount
	if base.PGMCount == 0 {
		base.PGMCount = 1
	}
	return base, updatedAt, inputs
}

func (s *Server) discoverRuntimeInputs() []runtimeInput {
	inputs := s.discoverRuntimeInputsFromRoute()
	if len(inputs) == 0 {
		inputs = s.discoverRuntimeInputsFromTelemetry()
	}
	seen := map[string]struct{}{}
	out := make([]runtimeInput, 0, len(inputs))
	for _, in := range inputs {
		in.ID = strings.TrimSpace(in.ID)
		if in.ID == "" {
			continue
		}
		if _, ok := seen[in.ID]; ok {
			continue
		}
		seen[in.ID] = struct{}{}
		if in.Class != "capture" {
			in.Class = "other"
		}
		in.Online = true
		out = append(out, in)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Class != out[j].Class {
			return out[i].Class < out[j].Class
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func (s *Server) discoverRuntimeInputsFromRoute() []runtimeInput {
	if strings.TrimSpace(s.cfg.RouteControlBaseURL) == "" {
		return nil
	}
	path := strings.TrimSpace(s.cfg.RouteInputDiscoveryPath)
	if path == "" {
		path = "/api/v1/route/inputs"
	}
	raw, status, err := s.routeControlGET(path)
	if err != nil || status >= 400 {
		return nil
	}
	var body struct {
		Inputs []runtimeInput `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &body); err == nil && len(body.Inputs) > 0 {
		return body.Inputs
	}
	var generic struct {
		Inputs []map[string]any `json:"inputs"`
	}
	if err := json.Unmarshal(raw, &generic); err != nil {
		return nil
	}
	out := make([]runtimeInput, 0, len(generic.Inputs))
	for _, item := range generic.Inputs {
		out = append(out, runtimeInput{
			ID:     strings.TrimSpace(fmt.Sprintf("%v", item["id"])),
			Class:  strings.TrimSpace(strings.ToLower(fmt.Sprintf("%v", item["class"]))),
			Label:  strings.TrimSpace(fmt.Sprintf("%v", item["label"])),
			Origin: strings.TrimSpace(fmt.Sprintf("%v", item["origin"])),
			Online: true,
		})
	}
	return out
}

func (s *Server) discoverRuntimeInputsFromTelemetry() []runtimeInput {
	if s.hub == nil {
		return nil
	}
	pres := s.hub.PresenceSnapshot()
	out := make([]runtimeInput, 0, len(pres))
	for _, p := range pres {
		if !p.Online {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(p.NodeType)) {
		case "capture":
			out = append(out, runtimeInput{
				ID:     "capture:" + strings.TrimSpace(p.NodeID),
				Class:  "capture",
				Label:  strings.TrimSpace(p.NodeID),
				Origin: "capture",
				Online: true,
			})
		case "route", "engine":
			// Reserved for future "other input" discovery hooks from node metrics.
		}
	}
	return out
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
	client := upstreamHTTPClient(10 * time.Second)
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
	client := upstreamHTTPClient(10 * time.Second)
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
	client := upstreamHTTPClient(10 * time.Second)
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
	client := upstreamHTTPClient(10 * time.Second)
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

func (s *Server) attachCaptureServiceToken(req *http.Request) {
	if req == nil {
		return
	}
	id := strings.TrimSpace(s.cfg.CaptureAccessClientID)
	secret := strings.TrimSpace(s.cfg.CaptureAccessClientSecret)
	if id == "" {
		id = strings.TrimSpace(s.cfg.EngineAccessClientID)
	}
	if secret == "" {
		secret = strings.TrimSpace(s.cfg.EngineAccessClientSecret)
	}
	if id != "" {
		req.Header.Set("Cf-Access-Client-Id", id)
	}
	if secret != "" {
		req.Header.Set("Cf-Access-Client-Secret", secret)
	}
}

func (s *Server) attachBFFProxyServiceToken(req *http.Request) {
	if req == nil {
		return
	}
	id := strings.TrimSpace(s.cfg.BFFProxyAccessClientID)
	secret := strings.TrimSpace(s.cfg.BFFProxyAccessClientSecret)
	if id == "" || secret == "" {
		return
	}
	req.Header.Set("Cf-Access-Client-Id", id)
	req.Header.Set("Cf-Access-Client-Secret", secret)
}
