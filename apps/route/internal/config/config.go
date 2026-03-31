package config

import (
	"os"
	"strconv"
	"time"
)

// Config 為 Route 節點的基本設定結構。
// 所有欄位皆來自環境變數，避免在程式碼中硬編任何敏感資訊或服務位址。
type Config struct {
	NodeID          string
	SRTPassphrase   string
	LogLevel        string
	MetricsInterval time.Duration

	SRTLAIngestPort int
	SRTOutputPort   int

	// 用於 srt-live-transmit 的基本 jitter buffer 參數（MVP 靜態設定，後續可改為 Console 動態控制）。
	// 對照 BELABOX/srtla README：lossmaxttl 為接收視窗大小；latency 為可用的重傳/重排序時間（ms）。
	LossMaxTTL int
	LatencyMs  int

	// srtla_rec 送入 srt-live-transmit 的「內部」SRT listener port。
	// MVP 預設為 SRTLAIngestPort + 1，避免與既定輸出埠衝突。
	InternalSRTPort int

	// 估算 ingest Mbps 使用的網卡名稱（通常在 Tailscale 環境為 tailscale0）。
	IngestIface string
}

// env 變數名稱常數。
const (
	envNodeID          = "VBS_NODE_ID"
	envSRTPassphrase   = "VBS_SRT_PASSPHRASE"
	envLogLevel        = "VBS_LOG_LEVEL"
	envMetricsInterval = "VBS_METRICS_INTERVAL"

	envSRTLAIngestPort = "VBS_ROUTE_SRTLA_INGEST_PORT"
	envSRTOutputPort   = "VBS_ROUTE_SRT_OUTPUT_PORT"

	envLossMaxTTL      = "VBS_ROUTE_LOSS_MAX_TTL"
	envLatencyMs       = "VBS_ROUTE_SRT_LATENCY_MS"
	envInternalSRTPort = "VBS_ROUTE_INTERNAL_SRT_PORT"
	envIngestIface     = "VBS_ROUTE_INGEST_IFACE"
)

// Load 讀取環境變數，並給予安全的預設值。
// 預設值僅用於開發與測試環境，實際部署時應由外部注入正確設定。
func Load() Config {
	cfg := Config{
		NodeID:        getenvOrDefault(envNodeID, "vbs-route-01"),
		SRTPassphrase: os.Getenv(envSRTPassphrase), // 若為空代表未設定，後續使用處需檢查
		LogLevel:      getenvOrDefault(envLogLevel, "info"),
	}

	intervalStr := getenvOrDefault(envMetricsInterval, "1000ms")
	if d, err := time.ParseDuration(intervalStr); err == nil {
		cfg.MetricsInterval = d
	} else {
		cfg.MetricsInterval = time.Second
	}

	// 預設埠號僅供開發與測試，實際部署時應由 protocol.md 規範並由外部注入。
	cfg.SRTLAIngestPort = getenvIntOrDefault(envSRTLAIngestPort, 10020)
	cfg.SRTOutputPort = getenvIntOrDefault(envSRTOutputPort, 10030)

	cfg.LossMaxTTL = getenvIntOrDefault(envLossMaxTTL, 40)
	cfg.LatencyMs = getenvIntOrDefault(envLatencyMs, 2000)

	defaultInternal := cfg.SRTLAIngestPort + 1
	cfg.InternalSRTPort = getenvIntOrDefault(envInternalSRTPort, defaultInternal)
	cfg.IngestIface = getenvOrDefault(envIngestIface, "tailscale0")

	return cfg
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


