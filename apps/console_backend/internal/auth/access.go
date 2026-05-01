package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
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

type AccessClaims struct {
	Subject    string
	NodeID     string
	Role       string
	Scope      string
	Issuer     string
	Email      string
	CommonName string
	TokenID    string
	SessionVersion int
	ExpiresAtUnix int64
	Audience   []string
}

type AccessJWTVerifier struct {
	cfIssuer      string
	cfAud         string
	cfJWKSURL     string
	cacheTTL      time.Duration
	httpClient    *http.Client
	adminEmailSet map[string]struct{}
	nodeCNPrefix  string
	serviceRoleByCN map[string]string
	consoleIssuer string
	consolePubKeys []ed25519.PublicKey
	consolePrivKey ed25519.PrivateKey
	clockSkewLeeway time.Duration

	mu        sync.RWMutex
	keys      map[string]any
	fetchedAt time.Time
}

type accessJWTClaims struct {
	NodeID     string `json:"node_id"`
	Role       string `json:"role"`
	Scope      string `json:"scope"`
	Email      string `json:"email"`
	CommonName string `json:"common_name"`
	SessionVersion int `json:"sv"`
	jwt.RegisteredClaims
}

func NewAccessJWTVerifier(teamDomain, aud, jwksURL string, cacheTTL time.Duration, clockSkewLeeway time.Duration, adminEmails []string, nodeCNPrefix, routeClientID, engineClientID, captureClientID, bffClientID, consoleIssuer, consolePrivateKey string, consolePublicKeys []string) (*AccessJWTVerifier, error) {
	if strings.TrimSpace(aud) == "" {
		return nil, fmt.Errorf("VBS_CF_ACCESS_AUD is required")
	}
	cfIssuer, resolvedJWKSURL, err := resolveIssuerAndJWKS(teamDomain, jwksURL)
	if err != nil {
		return nil, err
	}
	if cacheTTL <= 0 {
		cacheTTL = time.Hour
	}
	if clockSkewLeeway < 0 {
		clockSkewLeeway = 0
	}
	adminSet := make(map[string]struct{}, len(adminEmails))
	for _, e := range adminEmails {
		e = strings.TrimSpace(strings.ToLower(e))
		if e != "" {
			adminSet[e] = struct{}{}
		}
	}
	if len(adminSet) == 0 {
		return nil, fmt.Errorf("admin emails cannot be empty")
	}
	nodeCNPrefix = strings.TrimSpace(strings.ToLower(nodeCNPrefix))
	if nodeCNPrefix == "" {
		return nil, fmt.Errorf("node common_name prefix is required")
	}
	serviceRoleByCN := buildServiceRoleByCN(routeClientID, engineClientID, captureClientID, bffClientID)
	consoleIssuer = strings.TrimSpace(consoleIssuer)
	if consoleIssuer == "" {
		return nil, fmt.Errorf("console issuer is required")
	}
	pubKeys := make([]ed25519.PublicKey, 0, len(consolePublicKeys))
	for _, k := range consolePublicKeys {
		pub, err := parseEd25519PublicKey(k)
		if err == nil {
			pubKeys = append(pubKeys, pub)
		}
	}
	if len(pubKeys) == 0 {
		return nil, fmt.Errorf("console public keys are required")
	}
	priv, err := parseEd25519PrivateKey(consolePrivateKey)
	if err != nil {
		return nil, fmt.Errorf("invalid console private key: %w", err)
	}

	return &AccessJWTVerifier{
		cfIssuer:      cfIssuer,
		cfAud:         strings.TrimSpace(aud),
		cfJWKSURL:     resolvedJWKSURL,
		cacheTTL:      cacheTTL,
		httpClient:    &http.Client{Timeout: 8 * time.Second},
		adminEmailSet: adminSet,
		nodeCNPrefix:  nodeCNPrefix,
		serviceRoleByCN: serviceRoleByCN,
		consoleIssuer: consoleIssuer,
		consolePubKeys: pubKeys,
		consolePrivKey: priv,
		clockSkewLeeway: clockSkewLeeway,
		keys:          map[string]any{},
	}, nil
}

func (v *AccessJWTVerifier) VerifyRequest(r *http.Request) (*AccessClaims, error) {
	if cfJWT := strings.TrimSpace(r.Header.Get("Cf-Access-Jwt-Assertion")); cfJWT != "" {
		return v.VerifyToken(cfJWT)
	}
	return v.VerifyBearer(r.Header.Get("Authorization"))
}

// VerifyRequestPreferBearer prioritizes application-layer bearer tokens.
// This is used by Console control/admin authorization to avoid Cloudflare
// service-token identity overshadowing a valid admin/guest bearer token.
func (v *AccessJWTVerifier) VerifyRequestPreferBearer(r *http.Request) (*AccessClaims, error) {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if authz != "" {
		// If caller provides Authorization header, treat bearer token as the single
		// source of truth for role. Do not silently fall back to Cloudflare identity.
		return v.VerifyBearer(authz)
	}
	if cfJWT := strings.TrimSpace(r.Header.Get("Cf-Access-Jwt-Assertion")); cfJWT != "" {
		return v.VerifyToken(cfJWT)
	}
	return nil, fmt.Errorf("missing authorization")
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
	_, _, err := new(jwt.Parser).ParseUnverified(raw, &claims)
	if err != nil {
		return nil, fmt.Errorf("parse token failed: %w", err)
	}
	if strings.EqualFold(strings.TrimSpace(claims.Issuer), v.consoleIssuer) {
		return v.verifyConsoleToken(raw)
	}
	return v.verifyCloudflareToken(raw)
}

func (v *AccessJWTVerifier) verifyCloudflareToken(raw string) (*AccessClaims, error) {
	var claims accessJWTClaims
	opts := []jwt.ParserOption{jwt.WithAudience(v.cfAud)}
	if v.clockSkewLeeway > 0 {
		opts = append(opts, jwt.WithLeeway(v.clockSkewLeeway))
	}
	if v.cfIssuer != "" {
		opts = append(opts, jwt.WithIssuer(v.cfIssuer))
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

	sub := strings.TrimSpace(claims.Subject)
	email := strings.TrimSpace(strings.ToLower(claims.Email))
	cn := strings.TrimSpace(strings.ToLower(claims.CommonName))
	if sub == "" {
		sub = cn
	}
	if sub == "" && email == "" && cn == "" {
		return nil, fmt.Errorf("missing subject identity")
	}
	role := v.mapCloudflareRole(email, cn)
	if role == "" {
		return nil, fmt.Errorf("identity not allowed")
	}
	nodeID := strings.TrimSpace(claims.NodeID)
	if nodeID == "" {
		nodeID = sub
	}
	return &AccessClaims{
		Subject:    sub,
		NodeID:     nodeID,
		Role:       role,
		Scope:      strings.TrimSpace(claims.Scope),
		Issuer:     strings.TrimSpace(claims.Issuer),
		Email:      email,
		CommonName: cn,
		Audience:   []string(claims.Audience),
	}, nil
}

func (v *AccessJWTVerifier) verifyConsoleToken(raw string) (*AccessClaims, error) {
	var lastErr error
	for _, key := range v.consolePubKeys {
		var claims accessJWTClaims
		parseOpts := []jwt.ParserOption{jwt.WithIssuer(v.consoleIssuer), jwt.WithAudience(v.cfAud)}
		if v.clockSkewLeeway > 0 {
			parseOpts = append(parseOpts, jwt.WithLeeway(v.clockSkewLeeway))
		}
		parsed, err := jwt.ParseWithClaims(raw, &claims, func(token *jwt.Token) (any, error) {
			return key, nil
		}, parseOpts...)
		if err != nil {
			lastErr = err
			continue
		}
		if !parsed.Valid {
			lastErr = fmt.Errorf("invalid token")
			continue
		}
		sub := strings.TrimSpace(claims.Subject)
		role := strings.TrimSpace(strings.ToLower(claims.Role))
		if sub == "" || role == "" {
			lastErr = fmt.Errorf("missing required claims")
			continue
		}
		expUnix := int64(0)
		if claims.ExpiresAt != nil {
			expUnix = claims.ExpiresAt.Time.Unix()
		}
		return &AccessClaims{
			Subject:    sub,
			NodeID:     strings.TrimSpace(claims.NodeID),
			Role:       role,
			Scope:      strings.TrimSpace(claims.Scope),
			Issuer:     strings.TrimSpace(claims.Issuer),
			Email:      strings.TrimSpace(strings.ToLower(claims.Email)),
			CommonName: strings.TrimSpace(strings.ToLower(claims.CommonName)),
			TokenID:    strings.TrimSpace(claims.ID),
			SessionVersion: claims.SessionVersion,
			ExpiresAtUnix: expUnix,
			Audience:   []string(claims.Audience),
		}, nil
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("console token verification failed")
	}
	return nil, lastErr
}

func (v *AccessJWTVerifier) MintGuestToken(subject, scope string, ttl time.Duration, sessionVersion int) (string, error) {
	return v.mintRoleToken(subject, "guest", scope, ttl, sessionVersion)
}

func (v *AccessJWTVerifier) MintAdminToken(subject string, ttl time.Duration) (string, error) {
	return v.mintRoleToken(subject, "admin", "control:admin telemetry:read", ttl, 0)
}

func (v *AccessJWTVerifier) mintRoleToken(subject, role, scope string, ttl time.Duration, sessionVersion int) (string, error) {
	if ttl <= 0 {
		ttl = 10 * time.Minute
	}
	subject = strings.TrimSpace(subject)
	if subject == "" {
		return "", fmt.Errorf("subject required")
	}
	role = strings.TrimSpace(strings.ToLower(role))
	if role == "" {
		return "", fmt.Errorf("role required")
	}
	now := time.Now().UTC()
	notBefore := now
	if v.clockSkewLeeway > 0 {
		notBefore = now.Add(-v.clockSkewLeeway)
	}
	claims := accessJWTClaims{
		Role:  role,
		Scope: strings.TrimSpace(scope),
		SessionVersion: sessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    v.consoleIssuer,
			Subject:   subject,
			ID:        randomID(),
			Audience:  jwt.ClaimStrings{v.cfAud},
			IssuedAt:  jwt.NewNumericDate(now),
			NotBefore: jwt.NewNumericDate(notBefore),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	return token.SignedString(v.consolePrivKey)
}

func (v *AccessJWTVerifier) mapCloudflareRole(email, commonName string) string {
	if email != "" {
		if _, ok := v.adminEmailSet[email]; ok {
			return "admin"
		}
	}
	if role, ok := v.serviceRoleByCN[commonName]; ok {
		return role
	}
	if strings.HasPrefix(commonName, v.nodeCNPrefix) {
		return "node"
	}
	return ""
}

func buildServiceRoleByCN(routeClientID, engineClientID, captureClientID, bffClientID string) map[string]string {
	out := map[string]string{}
	add := func(clientID, role string) {
		id := strings.TrimSpace(strings.ToLower(clientID))
		if id == "" {
			return
		}
		out[id] = role
		if !strings.HasSuffix(id, ".access") {
			out[id+".access"] = role
		}
	}
	add(routeClientID, "route")
	add(engineClientID, "engine")
	add(captureClientID, "capture")
	add(bffClientID, "bff")
	return out
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
	if !ok || time.Since(v.fetchedAt) > v.cacheTTL {
		return nil, false
	}
	return key, true
}

func (v *AccessJWTVerifier) refreshKeys(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.cfJWKSURL, nil)
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

func parseEd25519PrivateKey(raw string) (ed25519.PrivateKey, error) {
	raw = normalizeKeyInput(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty private key")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		ed, ok := key.(ed25519.PrivateKey)
		if !ok {
			return nil, fmt.Errorf("not ed25519 private key")
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
	if len(buf) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid ed25519 private key length")
	}
	return ed25519.PrivateKey(buf), nil
}

func parseEd25519PublicKey(raw string) (ed25519.PublicKey, error) {
	raw = normalizeKeyInput(raw)
	if raw == "" {
		return nil, fmt.Errorf("empty public key")
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		key, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, err
		}
		ed, ok := key.(ed25519.PublicKey)
		if !ok {
			return nil, fmt.Errorf("not ed25519 public key")
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
		return nil, fmt.Errorf("invalid ed25519 public key length")
	}
	return ed25519.PublicKey(buf), nil
}

func normalizeKeyInput(raw string) string {
	v := strings.TrimSpace(raw)
	v = strings.Trim(v, `"'`)
	v = strings.ReplaceAll(v, `\n`, "\n")
	return strings.TrimSpace(v)
}

func randomID() string {
	return fmt.Sprintf("%d", time.Now().UTC().UnixNano())
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

