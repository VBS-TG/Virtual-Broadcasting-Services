// Package auth provides HS256 JWT minting and verification for Console MVP-A.
package auth

import (
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TelemetrySenderRoles are allowed to open the telemetry WebSocket.
var TelemetrySenderRoles = map[string]struct{}{
	"capture": {},
	"route":   {},
	"engine":  {},
	"console": {},
}

// Claims is the JWT payload for node tokens.
type Claims struct {
	NodeID string `json:"node_id"`
	Role string `json:"role"`
	jwt.RegisteredClaims
}

// Manager signs and verifies tokens with a shared HS256 secret.
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager creates a JWT manager.
func NewManager(secret string, ttl time.Duration) *Manager {
	return &Manager{secret: []byte(secret), ttl: ttl}
}

// Mint issues a short-lived JWT with subject = nodeID and custom role claim.
func (m *Manager) Mint(nodeID, role string) (string, time.Time, error) {
	nodeID = strings.TrimSpace(nodeID)
	role = strings.TrimSpace(strings.ToLower(role))
	if nodeID == "" {
		return "", time.Time{}, fmt.Errorf("node_id is required")
	}
	if !isAllowedIssuedRole(role) {
		return "", time.Time{}, fmt.Errorf("unsupported role %q", role)
	}
	now := time.Now()
	exp := now.Add(m.ttl)
	claims := Claims{
		NodeID: nodeID,
		Role: role,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   nodeID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := tok.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// ParseBearer extracts "Bearer <token>" and verifies the JWT.
func (m *Manager) ParseBearer(header string) (*Claims, error) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return nil, fmt.Errorf("missing bearer token")
	}
	raw := strings.TrimSpace(header[7:])
	return m.Parse(raw)
}

// Parse verifies a raw JWT string.
func (m *Manager) Parse(raw string) (*Claims, error) {
	var claims Claims
	tok, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return m.secret, nil
	})
	if err != nil {
		return nil, err
	}
	if !tok.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	if claims.Subject == "" {
		return nil, fmt.Errorf("missing sub claim")
	}
	if claims.NodeID == "" {
		claims.NodeID = claims.Subject
	}
	return &claims, nil
}

func isAllowedIssuedRole(role string) bool {
	if role == "admin" {
		return true
	}
	_, ok := TelemetrySenderRoles[role]
	return ok
}

// IsTelemetryRole returns true if the claim allows telemetry ingest.
func IsTelemetryRole(role string) bool {
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "admin" {
		return false
	}
	_, ok := TelemetrySenderRoles[role]
	return ok
}
