// Package config loads Console MVP-A environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultListen            = ":4000"
	defaultJWKSCacheTTLInSec = 3600
)

// Config holds runtime settings for the console server.
type Config struct {
	ListenAddr   string
	TelemetryMax int // max raw WS message size (bytes), default 255
	NodeOfflineTTL time.Duration

	CFAccessMode       string
	CFAccessTeamDomain string
	CFAccessAUD        string
	CFAccessJWKSURL    string
	CFAccessJWKSCacheTTL time.Duration
	AdminEmails        []string
	NodeCNPrefix       string

	ConsoleJWTIssuer     string
	ConsoleJWTPrivateKey string
	ConsoleJWTPublicKeys []string
	GuestTokenTTL        time.Duration
	GuestDBPath          string
	RuntimeDBPath        string

	RouteControlBaseURL  string
	PGMDefaultLatencyMs  int
	EngineControlBaseURL string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	listen := strings.TrimSpace(os.Getenv("VBS_CONSOLE_HTTP_BIND"))
	if listen == "" {
		listen = defaultListen
	}
	accessMode := strings.TrimSpace(strings.ToLower(getenvDefault("VBS_CF_ACCESS_MODE", "jwt")))
	if accessMode == "disabled" {
		return nil, fmt.Errorf("VBS_CF_ACCESS_MODE=disabled is not allowed in ZTA mode")
	}
	if accessMode != "jwt" && accessMode != "" {
		return nil, fmt.Errorf("unsupported VBS_CF_ACCESS_MODE=%q (expected jwt)", accessMode)
	}
	aud := strings.TrimSpace(os.Getenv("VBS_CF_ACCESS_AUD"))
	if aud == "" {
		return nil, fmt.Errorf("VBS_CF_ACCESS_AUD is required")
	}
	teamDomain := strings.TrimSpace(os.Getenv("VBS_CF_ACCESS_TEAM_DOMAIN"))
	jwksURL := strings.TrimSpace(os.Getenv("VBS_CF_ACCESS_JWKS_URL"))
	if teamDomain == "" && jwksURL == "" {
		return nil, fmt.Errorf("either VBS_CF_ACCESS_TEAM_DOMAIN or VBS_CF_ACCESS_JWKS_URL is required")
	}
	jwksTTL := defaultJWKSCacheTTLInSec
	if v := strings.TrimSpace(os.Getenv("VBS_CF_JWKS_CACHE_TTL_SEC")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 60 {
			return nil, fmt.Errorf("VBS_CF_JWKS_CACHE_TTL_SEC must be an integer >= 60")
		}
		jwksTTL = n
	}
	maxPayload := 255
	if v := strings.TrimSpace(os.Getenv("VBS_CONSOLE_TELEMETRY_MAX_BYTES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 64 || n > 4096 {
			return nil, fmt.Errorf("VBS_CONSOLE_TELEMETRY_MAX_BYTES must be between 64 and 4096")
		}
		maxPayload = n
	}
	offlineTTL := 10
	if v := strings.TrimSpace(os.Getenv("VBS_CONSOLE_NODE_OFFLINE_TTL_SEC")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 3 {
			return nil, fmt.Errorf("VBS_CONSOLE_NODE_OFFLINE_TTL_SEC must be an integer >= 3")
		}
		offlineTTL = n
	}
	adminEmails := splitCSVLower(os.Getenv("VBS_ADMIN_EMAILS"))
	if len(adminEmails) == 0 {
		return nil, fmt.Errorf("VBS_ADMIN_EMAILS is required")
	}
	nodePrefix := strings.TrimSpace(os.Getenv("VBS_NODE_CN_PREFIX"))
	if nodePrefix == "" {
		return nil, fmt.Errorf("VBS_NODE_CN_PREFIX is required")
	}
	consoleIssuer := strings.TrimSpace(getenvDefault("VBS_CONSOLE_JWT_ISSUER", "vbs-console"))
	consolePriv := strings.TrimSpace(os.Getenv("VBS_CONSOLE_JWT_PRIVATE_KEY"))
	if consolePriv == "" {
		return nil, fmt.Errorf("VBS_CONSOLE_JWT_PRIVATE_KEY is required")
	}
	consolePub := splitCSVRaw(os.Getenv("VBS_CONSOLE_JWT_PUBLIC_KEYS"))
	if len(consolePub) == 0 {
		return nil, fmt.Errorf("VBS_CONSOLE_JWT_PUBLIC_KEYS is required")
	}
	guestTTL := getenvIntDefault("VBS_GUEST_TOKEN_TTL_SEC", 600)
	if guestTTL < 60 {
		return nil, fmt.Errorf("VBS_GUEST_TOKEN_TTL_SEC must be >= 60")
	}
	guestDBPath := strings.TrimSpace(getenvDefault("VBS_GUEST_DB_PATH", "data/console-guests.db"))
	runtimeDBPath := strings.TrimSpace(getenvDefault("VBS_RUNTIME_DB_PATH", "data/console-runtime.db"))
	if runtimeDBPath == "" {
		return nil, fmt.Errorf("VBS_RUNTIME_DB_PATH is required")
	}

	return &Config{
		ListenAddr:         listen,
		TelemetryMax:       maxPayload,
		NodeOfflineTTL:     time.Duration(offlineTTL) * time.Second,
		CFAccessMode:       accessMode,
		CFAccessTeamDomain: teamDomain,
		CFAccessAUD:        aud,
		CFAccessJWKSURL:    jwksURL,
		CFAccessJWKSCacheTTL: time.Duration(jwksTTL) * time.Second,
		AdminEmails:        adminEmails,
		NodeCNPrefix:       nodePrefix,
		ConsoleJWTIssuer:   consoleIssuer,
		ConsoleJWTPrivateKey: consolePriv,
		ConsoleJWTPublicKeys: consolePub,
		GuestTokenTTL:      time.Duration(guestTTL) * time.Second,
		GuestDBPath:        guestDBPath,
		RuntimeDBPath:      runtimeDBPath,
		RouteControlBaseURL: strings.TrimSpace(os.Getenv("VBS_ROUTE_CONTROL_BASE_URL")),
		PGMDefaultLatencyMs: getenvIntDefault("VBS_PGM_DEFAULT_LATENCY_MS", 200),
		EngineControlBaseURL: strings.TrimSpace(os.Getenv("VBS_ENGINE_CONTROL_BASE_URL")),
	}, nil
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getenvIntDefault(key string, def int) int {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func splitCSVLower(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(strings.ToLower(p))
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

func splitCSVRaw(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
