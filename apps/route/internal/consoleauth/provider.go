package consoleauth

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
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
	CommonName     string `json:"common_name"`
	jwt.RegisteredClaims
}

type Provider struct {
	cfg           config.Config
	client        http.Client
	cfIssuer      string
	aud           string
	nodeCNPrefix  string

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
	return &Provider{
		cfg: cfg,
		client: http.Client{
			Timeout: 8 * time.Second,
		},
		cfIssuer: issuer,
		aud:      cfg.CFAccessAUD,
		nodeCNPrefix: strings.TrimSpace(strings.ToLower(cfg.NodeCNPrefix)),
		keys: map[string]any{},
	}
}

func (p *Provider) ApplyAccessHeaders(header http.Header) error {
	if header == nil {
		return fmt.Errorf("nil header")
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
	return p.verifyCloudflareBearer(raw)
}

func (p *Provider) verifyCloudflareBearer(raw string) (*Claims, error) {
	var claims Claims
	opts := []jwt.ParserOption{jwt.WithAudience(p.aud)}
	if p.cfg.JWTClockSkewLeeway > 0 {
		opts = append(opts, jwt.WithLeeway(p.cfg.JWTClockSkewLeeway))
	}
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
	claims.Role = p.mapCloudflareRole(strings.TrimSpace(strings.ToLower(claims.CommonName)))
	if claims.Role == "" {
		return nil, fmt.Errorf("identity not allowed")
	}
	return &claims, nil
}

func (p *Provider) mapCloudflareRole(commonName string) string {
	if strings.HasPrefix(commonName, p.nodeCNPrefix) {
		return "node"
	}
	// Cloudflare Service Token 常見以 "<client_id>.access" 形式出現在 common_name。
	if strings.HasSuffix(commonName, ".access") {
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

