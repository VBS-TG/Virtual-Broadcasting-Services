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
	if p.cached != "" {
		// Try refresh when token exists but nearing expiry.
		if t, exp, err := p.refreshToken(ctx, p.cached); err == nil {
			p.cached, p.expires = t, exp
			return p.cached, nil
		}
	}

	if p.cfg.DeviceID != "" && p.cfg.DeviceSecret != "" {
		token, exp, err := p.registerDevice(ctx)
		if err == nil {
			p.cached, p.expires = token, exp
			return p.cached, nil
		}
	}
	if p.cfg.CFAccessClientID != "" && p.cfg.CFAccessClientSecret != "" {
		token, exp, err := p.registerWithCFAccess(ctx)
		if err == nil {
			p.cached, p.expires = token, exp
			return p.cached, nil
		}
	}

	if p.cfg.BootstrapToken == "" {
		return "", fmt.Errorf("no route jwt and no device credential/bootstrap token")
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

func (p *Provider) registerDevice(ctx context.Context) (string, time.Time, error) {
	endpoint := strings.TrimRight(p.cfg.ConsoleBaseURL, "/") + "/api/v1/auth/register"
	body := map[string]string{
		"node_id":       p.cfg.DeviceID,
		"role":          "route",
		"device_secret": p.cfg.DeviceSecret,
	}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("register endpoint status=%d", resp.StatusCode)
	}
	return decodeTokenResponse(resp)
}

func (p *Provider) registerWithCFAccess(ctx context.Context) (string, time.Time, error) {
	endpoint := strings.TrimRight(p.cfg.ConsoleBaseURL, "/") + "/api/v1/auth/register"
	body := map[string]string{
		"node_id": p.cfg.NodeID,
		"role":    "route",
	}
	buf, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(buf))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("CF-Access-Client-Id", p.cfg.CFAccessClientID)
	req.Header.Set("CF-Access-Client-Secret", p.cfg.CFAccessClientSecret)
	req.Header.Set("X-VBS-Node-ID", p.cfg.NodeID)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("cf access register endpoint status=%d", resp.StatusCode)
	}
	return decodeTokenResponse(resp)
}

func (p *Provider) refreshToken(ctx context.Context, token string) (string, time.Time, error) {
	endpoint := strings.TrimRight(p.cfg.ConsoleBaseURL, "/") + "/api/v1/auth/refresh"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader([]byte("{}")))
	if err != nil {
		return "", time.Time{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := p.client.Do(req)
	if err != nil {
		return "", time.Time{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", time.Time{}, fmt.Errorf("refresh endpoint status=%d", resp.StatusCode)
	}
	return decodeTokenResponse(resp)
}

func decodeTokenResponse(resp *http.Response) (string, time.Time, error) {
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

