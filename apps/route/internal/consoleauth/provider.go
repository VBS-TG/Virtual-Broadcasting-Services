package consoleauth

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"vbs/apps/route/internal/config"
)

// Provider keeps a cached JWT and refreshes it when near expiry.
type Provider struct {
	cfg    config.Config
	client http.Client

	mu      sync.Mutex
	cached  string
	expires time.Time
}

func NewProvider(cfg config.Config) *Provider {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.TelemetryTLSInsecureSkipVerify {
		tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // test-only flag
	}
	return &Provider{
		cfg: cfg,
		client: http.Client{
			Timeout:   8 * time.Second,
			Transport: tr,
		},
		cached: cfg.RouteJWT,
	}
}

func (p *Provider) BearerToken(ctx context.Context, nodeID string) (string, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.cached != "" && time.Until(p.expires) > 5*time.Minute {
		return p.cached, nil
	}
	if p.cached != "" && p.expires.IsZero() {
		if exp, ok := jwtExp(p.cached); ok {
			p.expires = exp
			if time.Until(exp) > 5*time.Minute {
				return p.cached, nil
			}
		}
	}
	if p.cfg.BootstrapToken == "" {
		if p.cached != "" {
			return p.cached, nil
		}
		return "", fmt.Errorf("no route jwt and no bootstrap token")
	}

	token, exp, err := p.fetchToken(ctx, nodeID)
	if err != nil {
		return "", err
	}
	p.cached = token
	p.expires = exp
	return token, nil
}

func (p *Provider) fetchToken(ctx context.Context, nodeID string) (string, time.Time, error) {
	endpoint := strings.TrimRight(p.cfg.ConsoleBaseURL, "/") + "/api/v1/auth/token"
	body := map[string]string{
		"node_id": nodeID,
		"role":    "route",
	}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.cfg.BootstrapToken)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("token endpoint status=%d", resp.StatusCode)
	}
	var out struct {
		AccessToken string `json:"access_token"`
		ExpiresAt   int64  `json:"expires_at_unix"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", time.Time{}, err
	}
	if out.AccessToken == "" {
		return "", time.Time{}, fmt.Errorf("empty access_token")
	}
	exp := time.Unix(out.ExpiresAt, 0)
	if out.ExpiresAt == 0 {
		if t, ok := jwtExp(out.AccessToken); ok {
			exp = t
		}
	}
	return out.AccessToken, exp, nil
}

func jwtExp(raw string) (time.Time, bool) {
	parser := jwt.NewParser(jwt.WithoutClaimsValidation())
	var claims jwt.RegisteredClaims
	_, _, err := parser.ParseUnverified(raw, &claims)
	if err != nil || claims.ExpiresAt == nil {
		return time.Time{}, false
	}
	return claims.ExpiresAt.Time, true
}

