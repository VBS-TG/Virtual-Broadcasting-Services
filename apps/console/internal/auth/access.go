package auth

import (
	"context"
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
)

// AccessClaims contains normalized identity data from Cloudflare Access JWT.
type AccessClaims struct {
	Subject string
	NodeID  string
	Role    string
	Scope   string
	Audience []string
}

type AccessJWTVerifier struct {
	mode       string
	issuer     string
	aud        string
	jwksURL    string
	cacheTTL   time.Duration
	httpClient *http.Client

	mu        sync.RWMutex
	keys      map[string]any
	fetchedAt time.Time
}

type accessJWTClaims struct {
	NodeID string `json:"node_id"`
	Role   string `json:"role"`
	Scope  string `json:"scope"`
	jwt.RegisteredClaims
}

func NewAccessJWTVerifier(mode, teamDomain, aud, jwksURL string, cacheTTL time.Duration) (*AccessJWTVerifier, error) {
	mode = strings.TrimSpace(strings.ToLower(mode))
	if mode == "" {
		mode = "jwt"
	}
	if mode != "jwt" {
		return nil, fmt.Errorf("unsupported VBS_CF_ACCESS_MODE=%q (expected jwt)", mode)
	}
	if strings.TrimSpace(aud) == "" {
		return nil, fmt.Errorf("VBS_CF_ACCESS_AUD is required")
	}
	issuer, resolvedJWKSURL, err := resolveIssuerAndJWKS(teamDomain, jwksURL)
	if err != nil {
		return nil, err
	}
	if cacheTTL <= 0 {
		cacheTTL = time.Hour
	}
	return &AccessJWTVerifier{
		mode:       mode,
		issuer:     issuer,
		aud:        strings.TrimSpace(aud),
		jwksURL:    resolvedJWKSURL,
		cacheTTL:   cacheTTL,
		httpClient: &http.Client{Timeout: 8 * time.Second},
		keys:       map[string]any{},
	}, nil
}

func (v *AccessJWTVerifier) Mode() string { return v.mode }

func (v *AccessJWTVerifier) VerifyRequest(r *http.Request) (*AccessClaims, error) {
	return v.VerifyBearer(r.Header.Get("Authorization"))
}

func (v *AccessJWTVerifier) VerifyBearer(header string) (*AccessClaims, error) {
	header = strings.TrimSpace(header)
	if !strings.HasPrefix(strings.ToLower(header), "bearer ") {
		return nil, fmt.Errorf("missing bearer token")
	}
	raw := strings.TrimSpace(header[7:])
	if raw == "" {
		return nil, fmt.Errorf("empty bearer token")
	}
	return v.VerifyToken(raw)
}

func (v *AccessJWTVerifier) VerifyToken(raw string) (*AccessClaims, error) {
	var claims accessJWTClaims
	opts := []jwt.ParserOption{jwt.WithAudience(v.aud)}
	if v.issuer != "" {
		opts = append(opts, jwt.WithIssuer(v.issuer))
	}
	parsed, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		if strings.TrimSpace(kid) == "" {
			return nil, fmt.Errorf("missing kid")
		}
		return v.lookupKey(kid)
	}, opts...)
	if err != nil {
		return nil, err
	}
	if !parsed.Valid {
		return nil, fmt.Errorf("invalid token")
	}
	role := strings.TrimSpace(strings.ToLower(claims.Role))
	sub := strings.TrimSpace(claims.Subject)
	nodeID := strings.TrimSpace(claims.NodeID)
	if nodeID == "" {
		nodeID = sub
	}
	if sub == "" || role == "" {
		return nil, fmt.Errorf("missing required claims")
	}
	return &AccessClaims{
		Subject:  sub,
		NodeID:   nodeID,
		Role:     role,
		Scope:    strings.TrimSpace(claims.Scope),
		Audience: []string(claims.Audience),
	}, nil
}

func (v *AccessJWTVerifier) lookupKey(kid string) (any, error) {
	if key, ok := v.cachedKey(kid); ok {
		return key, nil
	}
	if err := v.refreshKeys(context.Background()); err != nil {
		return nil, err
	}
	if key, ok := v.cachedKey(kid); ok {
		return key, nil
	}
	return nil, fmt.Errorf("unknown kid")
}

func (v *AccessJWTVerifier) cachedKey(kid string) (any, bool) {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if len(v.keys) == 0 {
		return nil, false
	}
	key, ok := v.keys[kid]
	if !ok {
		return nil, false
	}
	if time.Since(v.fetchedAt) > v.cacheTTL {
		return nil, false
	}
	return key, true
}

func (v *AccessJWTVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}
	resp, err := v.httpClient.Do(req)
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
	v.mu.Lock()
	v.keys = next
	v.fetchedAt = time.Now()
	v.mu.Unlock()
	return nil
}

type jsonWebKey struct {
	KID string `json:"kid"`
	KTY string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
	Crv string `json:"crv"`
	X   string `json:"x"`
	Y   string `json:"y"`
}

func toPublicKey(k jsonWebKey) (any, error) {
	switch strings.ToUpper(strings.TrimSpace(k.KTY)) {
	case "RSA":
		nBytes, err := decodeBase64URL(k.N)
		if err != nil {
			return nil, err
		}
		eBytes, err := decodeBase64URL(k.E)
		if err != nil {
			return nil, err
		}
		nb := new(big.Int).SetBytes(nBytes)
		eb := new(big.Int).SetBytes(eBytes)
		if nb.Sign() <= 0 || eb.Sign() <= 0 {
			return nil, fmt.Errorf("invalid rsa key values")
		}
		return &rsa.PublicKey{N: nb, E: int(eb.Uint64())}, nil
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
			return nil, fmt.Errorf("unsupported curve %q", k.Crv)
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
		return nil, fmt.Errorf("unsupported kty %q", k.KTY)
	}
}

func decodeBase64URL(raw string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimSpace(raw))
}

func resolveIssuerAndJWKS(teamDomain, jwksURL string) (string, string, error) {
	td := strings.TrimSpace(teamDomain)
	if td != "" {
		td = strings.TrimPrefix(td, "https://")
		td = strings.TrimPrefix(td, "http://")
		td = strings.TrimSuffix(td, "/")
		if td == "" {
			return "", "", fmt.Errorf("invalid VBS_CF_ACCESS_TEAM_DOMAIN")
		}
		issuer := "https://" + td
		if strings.TrimSpace(jwksURL) == "" {
			return issuer, issuer + "/cdn-cgi/access/certs", nil
		}
	}
	jwksURL = strings.TrimSpace(jwksURL)
	if jwksURL == "" {
		return "", "", fmt.Errorf("VBS_CF_ACCESS_JWKS_URL is required")
	}
	u, err := url.Parse(jwksURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return "", "", fmt.Errorf("invalid VBS_CF_ACCESS_JWKS_URL")
	}
	if td == "" {
		return "", jwksURL, nil
	}
	return "https://" + td, jwksURL, nil
}

