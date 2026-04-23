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
	CFAccessClientID     string
	CFAccessClientSecret string

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
	// ControlToken：Route 控制面固定 Bearer token；設定後優先以此驗證。
	ControlToken string

	// PGM Relay（SRT StreamID 路由）
	PGMRelayPublishPrefix string
	PGMRelayReadPrefix    string
	PGMRelayPublicHost    string
	PGMRelayPublicPort    int
	PGMRelayLatencyMs     int
}

const (
	envNodeID        = "VBS_NODE_ID"
	envSRTPassphrase = "VBS_SRT_PASSPHRASE"
	envLogLevel      = "VBS_LOG_LEVEL"

	envConsoleBaseURL = "VBS_CONSOLE_BASE_URL"
	envCFAccessClientID = "VBS_CF_ACCESS_CLIENT_ID"
	envCFAccessClientSecret = "VBS_CF_ACCESS_CLIENT_SECRET"
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
	envControlToken = "VBS_ROUTE_CONTROL_TOKEN"

	envPGMRelayPublishPrefix = "VBS_ROUTE_PGM_STREAMID_PUBLISH_PREFIX"
	envPGMRelayReadPrefix    = "VBS_ROUTE_PGM_STREAMID_READ_PREFIX"
	envPGMRelayPublicHost    = "VBS_ROUTE_PGM_PUBLIC_HOST"
	envPGMRelayPublicPort    = "VBS_ROUTE_PGM_PUBLIC_PORT"
	envPGMRelayLatencyMs     = "VBS_ROUTE_PGM_READ_LATENCY_MS"
)

// Load 讀取環境變數。
func Load() Config {
	cfg := Config{
		NodeID:        getenvOrDefault(envNodeID, "vbs-route-01"),
		SRTPassphrase: os.Getenv(envSRTPassphrase),
		LogLevel:      getenvOrDefault(envLogLevel, "info"),

		ConsoleBaseURL: strings.TrimSpace(os.Getenv(envConsoleBaseURL)),
		CFAccessClientID: strings.TrimSpace(os.Getenv(envCFAccessClientID)),
		CFAccessClientSecret: strings.TrimSpace(os.Getenv(envCFAccessClientSecret)),
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
	cfg.ControlToken = strings.TrimSpace(os.Getenv(envControlToken))

	cfg.PGMRelayPublishPrefix = strings.Trim(strings.TrimSpace(getenvOrDefault(envPGMRelayPublishPrefix, "publish")), "/")
	cfg.PGMRelayReadPrefix = strings.Trim(strings.TrimSpace(getenvOrDefault(envPGMRelayReadPrefix, "read")), "/")
	cfg.PGMRelayPublicHost = strings.TrimSpace(os.Getenv(envPGMRelayPublicHost))
	cfg.PGMRelayPublicPort = getenvIntOrDefault(envPGMRelayPublicPort, cfg.SRTOutputPort)
	cfg.PGMRelayLatencyMs = getenvIntOrDefault(envPGMRelayLatencyMs, 200)

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
	if c.CFAccessClientID == "" || c.CFAccessClientSecret == "" {
		return fmt.Errorf("需設定 VBS_CF_ACCESS_CLIENT_ID 與 VBS_CF_ACCESS_CLIENT_SECRET（Cloudflare Access service token）")
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
	if c.PGMRelayPublishPrefix == "" || strings.Contains(c.PGMRelayPublishPrefix, " ") {
		return fmt.Errorf("VBS_ROUTE_PGM_STREAMID_PUBLISH_PREFIX 格式不合法")
	}
	if c.PGMRelayReadPrefix == "" || strings.Contains(c.PGMRelayReadPrefix, " ") {
		return fmt.Errorf("VBS_ROUTE_PGM_STREAMID_READ_PREFIX 格式不合法")
	}
	if c.PGMRelayPublicPort <= 0 {
		return fmt.Errorf("VBS_ROUTE_PGM_PUBLIC_PORT 必須為正整數")
	}
	if c.PGMRelayLatencyMs <= 0 {
		return fmt.Errorf("VBS_ROUTE_PGM_READ_LATENCY_MS 必須為正整數")
	}
	return nil
}

func getenvOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
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
