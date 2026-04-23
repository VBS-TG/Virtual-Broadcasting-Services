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
	defaultListen = ":4000"
	defaultJWTTTL = 3600 // seconds
)

// Config holds runtime settings for the console server.
type Config struct {
	ListenAddr   string
	JWTSecret    string
	JWTTTL       time.Duration
	TelemetryMax int // max raw WS message size (bytes), default 255
	CFAccessMode       string
	CFAccessTeamDomain string
	CFAccessAUD        string
	CFAccessClientsRaw string
	RouteControlBaseURL string
	RouteControlToken   string
	PGMDefaultLatencyMs int
	EngineControlBaseURL string
	EngineControlToken   string
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	listen := strings.TrimSpace(os.Getenv("VBS_CONSOLE_HTTP_BIND"))
	if listen == "" {
		listen = defaultListen
	}
	secret := strings.TrimSpace(os.Getenv("VBS_CONSOLE_JWT_SECRET"))
	if secret == "" {
		return nil, fmt.Errorf("VBS_CONSOLE_JWT_SECRET is required")
	}
	ttlSec := defaultJWTTTL
	if v := strings.TrimSpace(os.Getenv("VBS_CONSOLE_JWT_TTL_SEC")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 60 {
			return nil, fmt.Errorf("VBS_CONSOLE_JWT_TTL_SEC must be an integer >= 60")
		}
		ttlSec = n
	}
	accessMode := strings.TrimSpace(strings.ToLower(getenvDefault("VBS_CF_ACCESS_MODE", "service_token")))
	accessClientsRaw := strings.TrimSpace(os.Getenv("VBS_CF_ACCESS_CLIENTS"))
	if accessMode == "service_token" && accessClientsRaw == "" {
		return nil, fmt.Errorf("VBS_CF_ACCESS_CLIENTS is required when VBS_CF_ACCESS_MODE=service_token")
	}
	maxPayload := 255
	if v := strings.TrimSpace(os.Getenv("VBS_CONSOLE_TELEMETRY_MAX_BYTES")); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 64 || n > 4096 {
			return nil, fmt.Errorf("VBS_CONSOLE_TELEMETRY_MAX_BYTES must be between 64 and 4096")
		}
		maxPayload = n
	}
	return &Config{
		ListenAddr:   listen,
		JWTSecret:    secret,
		JWTTTL:       time.Duration(ttlSec) * time.Second,
		TelemetryMax: maxPayload,
		CFAccessMode:       accessMode,
		CFAccessTeamDomain: strings.TrimSpace(os.Getenv("VBS_CF_ACCESS_TEAM_DOMAIN")),
		CFAccessAUD:        strings.TrimSpace(os.Getenv("VBS_CF_ACCESS_AUD")),
		CFAccessClientsRaw: accessClientsRaw,
		RouteControlBaseURL: strings.TrimSpace(os.Getenv("VBS_ROUTE_CONTROL_BASE_URL")),
		RouteControlToken:   strings.TrimSpace(os.Getenv("VBS_ROUTE_CONTROL_TOKEN")),
		PGMDefaultLatencyMs: getenvIntDefault("VBS_PGM_DEFAULT_LATENCY_MS", 200),
		EngineControlBaseURL: strings.TrimSpace(os.Getenv("VBS_ENGINE_CONTROL_BASE_URL")),
		EngineControlToken:   strings.TrimSpace(os.Getenv("VBS_ENGINE_CONTROL_TOKEN")),
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
