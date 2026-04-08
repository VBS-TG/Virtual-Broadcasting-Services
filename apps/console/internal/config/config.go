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
	AdminToken   string
	TelemetryMax int // max raw WS message size (bytes), default 255
	NodeCredentialsRaw string
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
	admin := strings.TrimSpace(os.Getenv("VBS_CONSOLE_ADMIN_TOKEN"))
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
		AdminToken:   admin,
		TelemetryMax: maxPayload,
		NodeCredentialsRaw: strings.TrimSpace(os.Getenv("VBS_CONSOLE_NODE_CREDENTIALS")),
	}, nil
}
