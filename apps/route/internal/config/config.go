package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

// Config 為 Route 節點的正式執行設定，欄位皆來自環境變數。
type Config struct {
	NodeID string

	SRTPassphrase string
	LogLevel      string

	// Console 控制平面（HTTPS 基底網址，用於衍生 WSS 遙測 URL）
	ConsoleBaseURL string
	// RouteJWT 可直接注入已簽發 JWT；若未提供可改用 BootstrapToken 自動向 Console 申請。
	RouteJWT string
	// BootstrapToken 為舊版相容用 token（可換取短效節點 JWT）。
	BootstrapToken string
	// Device bootstrap identity（正式版）
	DeviceID     string
	DeviceSecret string

	// WSS 遙測路徑（相對於 Console 主機），預設見 Load。
	TelemetryWSPath string

	// 開發用：略過 TLS 憑證校驗（僅限測試環境）
	TelemetryTLSInsecureSkipVerify bool

	MetricsInterval time.Duration

	SRTLAIngestPort int
	SRTOutputPort   int
	InternalSRTPort int

	LossMaxTTL int
	LatencyMs  int

	IngestIface string

	// StallIngestSeconds：偵測到 ingest 頻寬連續為零達此秒數後，觸發管線重啟；0 表示停用。
	// 僅在曾偵測到顯著流量（高於 StallTrafficMbps）後啟用，避免無送流時誤觸發。
	StallIngestSeconds int
	StallTrafficMbps   float64

	// ControlBind：HTTP 控制面監聽位址，例如「:20080」；空字串表示不啟用。
	ControlBind string
}

const (
	envNodeID        = "VBS_NODE_ID"
	envSRTPassphrase = "VBS_SRT_PASSPHRASE"
	envLogLevel      = "VBS_LOG_LEVEL"

	envConsoleBaseURL = "VBS_CONSOLE_BASE_URL"
	envRouteJWT       = "VBS_ROUTE_JWT"
	envGlobalJWT      = "VBS_JWT"
	envBootstrapToken = "VBS_ROUTE_BOOTSTRAP_TOKEN"
	envDeviceID       = "VBS_ROUTE_DEVICE_ID"
	envDeviceSecret   = "VBS_ROUTE_DEVICE_SECRET"
	envTelemetryPath  = "VBS_ROUTE_TELEMETRY_WS_PATH"
	envTLSInsecure    = "VBS_ROUTE_TELEMETRY_TLS_INSECURE_SKIP_VERIFY"

	envMetricsInterval = "VBS_METRICS_INTERVAL"

	envSRTLAIngestPort = "VBS_ROUTE_SRTLA_INGEST_PORT"
	envSRTOutputPort   = "VBS_ROUTE_SRT_OUTPUT_PORT"
	envLossMaxTTL      = "VBS_ROUTE_LOSS_MAX_TTL"
	envLatencyMs       = "VBS_ROUTE_SRT_LATENCY_MS"
	envInternalSRTPort = "VBS_ROUTE_INTERNAL_SRT_PORT"
	envIngestIface     = "VBS_ROUTE_INGEST_IFACE"

	envStallSeconds = "VBS_ROUTE_STALL_INGEST_SECONDS"
	envStallTraffic = "VBS_ROUTE_STALL_TRAFFIC_MBPS"

	envControlBind = "VBS_ROUTE_CONTROL_BIND"
)

// Load 讀取環境變數。
func Load() Config {
	cfg := Config{
		NodeID:        getenvOrDefault(envNodeID, "vbs-route-01"),
		SRTPassphrase: os.Getenv(envSRTPassphrase),
		LogLevel:      getenvOrDefault(envLogLevel, "info"),

		ConsoleBaseURL: strings.TrimSpace(os.Getenv(envConsoleBaseURL)),
		RouteJWT:       strings.TrimSpace(getenvFirstNonEmpty(envRouteJWT, envGlobalJWT)),
		BootstrapToken: strings.TrimSpace(os.Getenv(envBootstrapToken)),
		DeviceID:       strings.TrimSpace(os.Getenv(envDeviceID)),
		DeviceSecret:   strings.TrimSpace(os.Getenv(envDeviceSecret)),
		TelemetryWSPath: getenvOrDefault(envTelemetryPath, "/vbs/telemetry/ws"),
	}

	if getenvOrDefault(envTLSInsecure, "0") == "1" || strings.EqualFold(os.Getenv(envTLSInsecure), "true") {
		cfg.TelemetryTLSInsecureSkipVerify = true
	}

	intervalStr := getenvOrDefault(envMetricsInterval, "1000ms")
	if d, err := time.ParseDuration(intervalStr); err == nil {
		cfg.MetricsInterval = d
	} else {
		cfg.MetricsInterval = time.Second
	}

	cfg.SRTLAIngestPort = getenvIntOrDefault(envSRTLAIngestPort, 20020)
	cfg.SRTOutputPort = getenvIntOrDefault(envSRTOutputPort, 20030)
	cfg.LossMaxTTL = getenvIntOrDefault(envLossMaxTTL, 40)
	cfg.LatencyMs = getenvIntOrDefault(envLatencyMs, 2000)

	defaultInternal := cfg.SRTLAIngestPort + 1
	cfg.InternalSRTPort = getenvIntOrDefault(envInternalSRTPort, defaultInternal)
	cfg.IngestIface = getenvOrDefault(envIngestIface, "eth0")

	cfg.StallIngestSeconds = getenvIntOrDefault(envStallSeconds, 5)
	cfg.StallTrafficMbps = getenvFloatOrDefault(envStallTraffic, 0.5)

	if v, ok := os.LookupEnv(envControlBind); ok {
		cfg.ControlBind = strings.TrimSpace(v)
	} else {
		cfg.ControlBind = ":20080"
	}

	return cfg
}

// TelemetryWSSURL 由 Console HTTPS 基底與路徑組出 WSS 連線網址。
func (c Config) TelemetryWSSURL() (string, error) {
	if c.ConsoleBaseURL == "" {
		return "", fmt.Errorf("VBS_CONSOLE_BASE_URL 未設定")
	}
	u, err := url.Parse(strings.TrimSpace(c.ConsoleBaseURL))
	if err != nil {
		return "", err
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	case "http":
		u.Scheme = "ws"
	default:
		return "", fmt.Errorf("VBS_CONSOLE_BASE_URL 需為 http 或 https 開頭")
	}
	path := strings.TrimSpace(c.TelemetryWSPath)
	if path == "" {
		path = "/vbs/telemetry/ws"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	ref, err := url.Parse(path)
	if err != nil {
		return "", err
	}
	out := u.ResolveReference(ref)
	out.RawQuery = ""
	out.Fragment = ""
	return out.String(), nil
}

// Validate 驗證正式執行必要條件。
func (c Config) Validate() error {
	if c.SRTPassphrase == "" {
		return fmt.Errorf("VBS_SRT_PASSPHRASE 為必填")
	}
	if n := utf8.RuneCountInString(c.SRTPassphrase); n < 10 || n > 64 {
		return fmt.Errorf("VBS_SRT_PASSPHRASE 長度須為 10–64 字元，目前為 %d", n)
	}
	if c.ConsoleBaseURL == "" {
		return fmt.Errorf("VBS_CONSOLE_BASE_URL 為必填（Console 控制平面 HTTPS 基底）")
	}
	if c.RouteJWT == "" && c.BootstrapToken == "" && (c.DeviceID == "" || c.DeviceSecret == "") {
		return fmt.Errorf("需設定 VBS_ROUTE_JWT（或 VBS_JWT），或 VBS_ROUTE_DEVICE_ID+VBS_ROUTE_DEVICE_SECRET，或舊版 VBS_ROUTE_BOOTSTRAP_TOKEN")
	}
	if c.SRTLAIngestPort <= 0 || c.SRTOutputPort <= 0 || c.InternalSRTPort <= 0 {
		return fmt.Errorf("Route 埠號必須為正整數")
	}
	if _, err := c.TelemetryWSSURL(); err != nil {
		return err
	}
	if c.StallIngestSeconds < 0 {
		return fmt.Errorf("VBS_ROUTE_STALL_INGEST_SECONDS 不可為負數")
	}
	if c.StallTrafficMbps < 0 {
		return fmt.Errorf("VBS_ROUTE_STALL_TRAFFIC_MBPS 不可為負數")
	}
	return nil
}

func getenvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getenvFirstNonEmpty(keys ...string) string {
	for _, k := range keys {
		if v := strings.TrimSpace(os.Getenv(k)); v != "" {
			return v
		}
	}
	return ""
}

func getenvIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getenvFloatOrDefault(key string, def float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return def
}
