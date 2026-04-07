// Package httpserver wires HTTP + WebSocket routes for Console MVP-A.
package httpserver

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"io"
	"log"
	"net/http"
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
	s.mux.HandleFunc("GET /vbs/telemetry/ws", s.handleTelemetryWS)
	s.mux.HandleFunc("GET /api/v1/telemetry/latest", s.handleTelemetryLatest)
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

func (s *Server) adminAuthorized(r *http.Request) bool {
	h := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(strings.ToLower(h), "bearer ") {
		return false
	}
	raw := strings.TrimSpace(h[7:])
	if constantTimeEqual(raw, s.cfg.AdminToken) {
		// Bootstrap mode: allow Bearer <admin bootstrap token> to mint first admin JWT.
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
