package consoleauth

import (
	"context"
	"bytes"
	"crypto/ed25519"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"vbs/apps/route/internal/config"
)

type Claims struct {
	Role           string `json:"role"`
	Scope          string `json:"scope"`
	Email          string `json:"email"`
	CommonName     string `json:"common_name"`
	SessionVersion int    `json:"sv"`
	jwt.RegisteredClaims
}

type Provider struct {
	cfg           config.Config
	client        http.Client
	cfIssuer      string
	aud           string
	adminEmails   map[string]struct{}
	nodeCNPrefix  string
	consoleIssuer string
	consolePubKeys []ed25519.PublicKey

	mu        sync.RWMutex
	keys      map[string]any
	fetchedAt time.Time
}

func NewProvider(cfg config.Config) *Provider {
	issuer := ""
	if td := strings.TrimSpace(cfg.CFAccessTeamDomain); td != "" {
		td = strings.TrimPrefix(td, "https://")
		td = strings.TrimPrefix(td, "http://")
		td = strings.TrimSuffix(td, "/")
		if td != "" {
			issuer = "https://" + td
		}
	}
	adminSet := map[string]struct{}{}
	for _, e := range cfg.AdminEmails {
		adminSet[strings.TrimSpace(strings.ToLower(e))] = struct{}{}
	}
	pubs := make([]ed25519.PublicKey, 0, len(cfg.ConsoleJWTPublicKeys))
	for _, k := range cfg.ConsoleJWTPublicKeys {
		if pub, err := parseEd25519PublicKey(k); err == nil {
			pubs = append(pubs, pub)
		}
	}
	return &Provider{
		cfg: cfg,
		client: http.Client{
			Timeout: 8 * time.Second,
		},
		cfIssuer: issuer,
		aud:      cfg.CFAccessAUD,
		adminEmails: adminSet,
		nodeCNPrefix: strings.TrimSpace(strings.ToLower(cfg.NodeCNPrefix)),
		consoleIssuer: strings.TrimSpace(cfg.ConsoleJWTIssuer),
		consolePubKeys: pubs,
		keys: map[string]any{},
	}
}

func (p *Provider) BearerToken(ctx context.Context) (string, error) {
	_ = ctx
	if p.cfg.CFAccessJWT == "" {
		return "", fmt.Errorf("VBS_CF_ACCESS_JWT is required")
	}
	return p.cfg.CFAccessJWT, nil
}

func (p *Provider) ApplyAccessHeaders(header http.Header) error {
	if header == nil {
		return fmt.Errorf("nil header")
	}
	if jwt := strings.TrimSpace(p.cfg.CFAccessJWT); jwt != "" {
		header.Set("Authorization", "Bearer "+jwt)
		return nil
	}
	clientID := strings.TrimSpace(p.cfg.CFAccessClientID)
	clientSecret := strings.TrimSpace(p.cfg.CFAccessClientSecret)
	if clientID != "" && clientSecret != "" {
		header.Set("Cf-Access-Client-Id", clientID)
		header.Set("Cf-Access-Client-Secret", clientSecret)
		return nil
	}
	return fmt.Errorf("cloudflare access credentials missing")
}

func (p *Provider) VerifyBearer(raw string) (*Claims, error) {
	raw = strings.TrimSpace(raw)
	var claims Claims
	_, _, err := new(jwt.Parser).ParseUnverified(raw, &claims)
	if err != nil {
		return nil, err
	}
	if strings.EqualFold(strings.TrimSpace(claims.Issuer), p.consoleIssuer) {
		return p.verifyConsoleBearer(raw)
	}
	return p.verifyCloudflareBearer(raw)
}

func (p *Provider) verifyCloudflareBearer(raw string) (*Claims, error) {
	var claims Claims
	opts := []jwt.ParserOption{jwt.WithAudience(p.aud)}
	if p.cfIssuer != "" {
		opts = append(opts, jwt.WithIssuer(p.cfIssuer))
	}
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		if strings.TrimSpace(kid) == "" {
			return nil, fmt.Errorf("missing kid")
		}
		return p.lookupKey(kid)
	}, opts...)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	claims.Role = p.mapCloudflareRole(strings.TrimSpace(strings.ToLower(claims.Email)), strings.TrimSpace(strings.ToLower(claims.CommonName)))
	if claims.Role == "" {
		return nil, fmt.Errorf("identity not allowed")
	}
	return &claims, nil
}

func (p *Provider) verifyConsoleBearer(raw string) (*Claims, error) {
	var lastErr error
	for _, pub := range p.consolePubKeys {
		var claims Claims
		parsed, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
			return pub, nil
		}, jwt.WithIssuer(p.consoleIssuer), jwt.WithAudience(p.aud))
		if err != nil {
			lastErr = err
			continue
		}
		if !parsed.Valid {
			lastErr = fmt.Errorf("invalid token")
			continue
		}
		if strings.TrimSpace(strings.ToLower(claims.Role)) == "" {
			lastErr = fmt.Errorf("missing role")
			continue
		}
		role := strings.TrimSpace(strings.ToLower(claims.Role))
		if role == "operator" {
			guestID := strings.TrimPrefix(strings.TrimSpace(claims.Subject), "guest:")
			if guestID == "" || !p.introspectGuest(guestID, claims.SessionVersion) {
				lastErr = fmt.Errorf("guest session revoked")
				continue
			}
		}
		return &claims, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("console token verification failed")
	}
	return nil, lastErr
}

func (p *Provider) introspectGuest(guestID string, sessionVersion int) bool {
	base := strings.TrimRight(strings.TrimSpace(p.cfg.ConsoleBaseURL), "/")
	if base == "" {
		return false
	}
	payload, _ := json.Marshal(map[string]any{
		"guest_id": guestID,
		"session_version": sessionVersion,
	})
	req, err := http.NewRequest(http.MethodPost, base+"/api/v1/guest/introspect", bytes.NewReader(payload))
	if err != nil {
		return false
	}
	req.Header.Set("Content-Type", "application/json")
	if err := p.ApplyAccessHeaders(req.Header); err != nil {
		return false
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return false
	}
	var out struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false
	}
	return out.Active
}

func (p *Provider) mapCloudflareRole(email, commonName string) string {
	if _, ok := p.adminEmails[email]; ok {
		return "admin"
	}
	if strings.HasPrefix(commonName, p.nodeCNPrefix) {
		return "node"
	}
	return ""
}

func (p *Provider) lookupKey(kid string) (any, error) {
	if key, ok := p.cachedKey(kid); ok {
		return key, nil
	}
	if err := p.refreshKeys(); err != nil {
		return nil, err
	}
	if key, ok := p.cachedKey(kid); ok {
		return key, nil
	}
	return nil, fmt.Errorf("unknown kid")
}

func (p *Provider) cachedKey(kid string) (any, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if len(p.keys) == 0 || time.Since(p.fetchedAt) > p.cfg.CFAccessJWKSCacheTTL {
		return nil, false
	}
	key, ok := p.keys[kid]
	return key, ok
}

type jsonWebKey struct {
	KID string `json:"kid"`
	KTY string `json:"kty"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func (p *Provider) refreshKeys() error {
	jwksURL := strings.TrimSpace(p.cfg.CFAccessJWKSURL)
	if jwksURL == "" && p.cfIssuer != "" {
		jwksURL = p.cfIssuer + "/cdn-cgi/access/certs"
	}
	if jwksURL == "" {
		return fmt.Errorf("missing jwks url")
	}
	if _, err := url.Parse(jwksURL); err != nil {
		return err
	}
	req, err := http.NewRequest(http.MethodGet, jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks status=%d", resp.StatusCode)
	}
	var body struct {
		Keys []jsonWebKey `json:"keys"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return err
	}
	next := make(map[string]any, len(body.Keys))
	for _, k := range body.Keys {
		pub, err := toPublicKey(k)
		if err != nil {
			continue
		}
		next[k.KID] = pub
	}
	if len(next) == 0 {
		return fmt.Errorf("no usable jwks keys")
	}
	p.mu.Lock()
	p.keys = next
	p.fetchedAt = time.Now()
	p.mu.Unlock()
	return nil
}

func toPublicKey(k jsonWebKey) (any, error) {
	switch strings.ToUpper(strings.TrimSpace(k.KTY)) {
	case "RSA":
		nb, err := decodeBase64URL(k.N)
		if err != nil {
			return nil, err
		}
		eb, err := decodeBase64URL(k.E)
		if err != nil {
			return nil, err
		}
		n := new(big.Int).SetBytes(nb)
		e := new(big.Int).SetBytes(eb)
		if n.Sign() <= 0 || e.Sign() <= 0 {
			return nil, fmt.Errorf("invalid rsa key")
		}
		return &rsa.PublicKey{N: n, E: int(e.Uint64())}, nil
	case "EC":
		var curve elliptic.Curve
		switch strings.ToUpper(strings.TrimSpace(k.Crv)) {
		case "P-256":
			curve = elliptic.P256()
		case "P-384":
			curve = elliptic.P384()
		case "P-521":
			curve = elliptic.P521()
		default:
			return nil, fmt.Errorf("unsupported curve")
		}
		xb, err := decodeBase64URL(k.X)
		if err != nil {
			return nil, err
		}
		yb, err := decodeBase64URL(k.Y)
		if err != nil {
			return nil, err
		}
		x := new(big.Int).SetBytes(xb)
		y := new(big.Int).SetBytes(yb)
		if !curve.IsOnCurve(x, y) {
			return nil, fmt.Errorf("ec point not on curve")
		}
		return &ecdsa.PublicKey{Curve: curve, X: x, Y: y}, nil
	default:
		return nil, fmt.Errorf("unsupported key type")
	}
}

func decodeBase64URL(raw string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
}

func parseEd25519PublicKey(raw string) (ed25519.PublicKey, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty key")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		ed, ok := key.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not ed25519 key")
		}
		return ed, nil
	}
	buf, err := base64.RawStdEncoding.DecodeString(raw)
	if err != nil {
		buf, err = base64.StdEncoding.DecodeString(raw)
		if err != nil {
			return nil, err
		}
	}
	if len(buf) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("invalid ed25519 key length")
	}
	return ed25519.PublicKey(buf), nil
}
