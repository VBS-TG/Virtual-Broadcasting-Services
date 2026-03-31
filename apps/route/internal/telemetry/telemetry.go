package telemetry

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"runtime"
	"time"

	"vbs/apps/route/internal/config"
)

// Metrics 描述 Route 節點在 .cursorrules 中要求的核心遙測欄位。
type Metrics struct {
	NodeID string `json:"node_id"`

	// 雲端系統負載
	CPUPercent float64 `json:"cpu_percent"`
	MemBytes   uint64  `json:"mem_bytes"`

	// 串流接收品質（MVP 階段先以預留欄位與 0 值表示）
	TotalIngestMbps float64 `json:"total_ingest_mbps"`
	ReorderErrorPct float64 `json:"reorder_error_pct"`

	// 與 Engine 的連線狀態（MVP 先用布林占位）
	HasEngineClient bool `json:"has_engine_client"`
}

// StartLocalLogger 會依照設定的 MetricsInterval 週期性輸出單行 JSON，
// 後續可將同一份資料改為送往 Console WebSocket Hub。
func StartLocalLogger(ctx context.Context, cfg config.Config, logger *log.Logger) {
	if logger == nil {
		logger = log.Default()
	}

	ticker := time.NewTicker(cfg.MetricsInterval)
	defer ticker.Stop()
	collector := newIngestCollector(cfg.IngestIface)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ingestMbps := collector.sampleMbps()
			m := collectMetrics(cfg, ingestMbps)
			buf, err := json.Marshal(m)
			if err != nil {
				logger.Printf("[route][telemetry] marshal error err=%v", err)
				continue
			}

			// 確保單筆 JSON 長度控制在 255 bytes 內，符合 .cursorrules 要求。
			if len(buf) > 255 {
				logger.Printf("[route][telemetry] telemetry payload too large len=%d, skip", len(buf))
				continue
			}

			logger.Printf("[route][telemetry] %s", string(buf))
		}
	}
}

func collectMetrics(cfg config.Config, ingestMbps float64) Metrics {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)

	return Metrics{
		NodeID: cfg.NodeID,

		CPUPercent: 0,           // MVP 階段暫不精準計算 CPU，比率保留欄位
		MemBytes:   ms.Alloc,    // 使用 Go runtime 目前配置的記憶體數量
		TotalIngestMbps: ingestMbps,
		ReorderErrorPct: 0,      // 同上
		HasEngineClient: false,  // 未整合實際連線追蹤前，先以 false 佔位
	}
}

type ingestCollector struct {
	iface    string
	prevRx   uint64
	prevTime time.Time
	ready    bool
}

func newIngestCollector(iface string) *ingestCollector {
	return &ingestCollector{
		iface: iface,
	}
}

func (c *ingestCollector) sampleMbps() float64 {
	rx, err := readRXBytes(c.iface)
	now := time.Now()
	if err != nil {
		return 0
	}

	if !c.ready {
		c.prevRx = rx
		c.prevTime = now
		c.ready = true
		return 0
	}

	deltaBytes := int64(rx) - int64(c.prevRx)
	deltaSec := now.Sub(c.prevTime).Seconds()
	c.prevRx = rx
	c.prevTime = now

	if deltaBytes <= 0 || deltaSec <= 0 {
		return 0
	}

	mbps := float64(deltaBytes*8) / deltaSec / 1_000_000
	// 壓縮小數位，避免 telemetry JSON 超過 255 bytes。
	return round2(mbps)
}

func readRXBytes(iface string) (uint64, error) {
	path := fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", iface)
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return 0, err
	}
	return n, nil
}

func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}

