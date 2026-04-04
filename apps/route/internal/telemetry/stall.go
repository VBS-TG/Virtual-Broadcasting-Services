package telemetry

import (
	"vbs/apps/route/internal/config"
)

// StallTracker 依 ingest Mbps 偵測「曾有流量後長時間歸零」之停滯（單執行緒每週期呼叫 Observe）。
type StallTracker struct {
	cfg              config.Config
	consecutiveZero  int
	sawTraffic       bool
	restartPending   bool
	thresholdTraffic float64
}

func NewStallTracker(cfg config.Config) *StallTracker {
	return &StallTracker{
		cfg:              cfg,
		thresholdTraffic: cfg.StallTrafficMbps,
	}
}

// Observe 傳入本週期 ingest Mbps；若應觸發管線重啟則回傳 true（呼叫端負責非阻塞寫入 restart channel）。
func (t *StallTracker) Observe(mbps float64) (shouldRestart bool) {
	if t.cfg.StallIngestSeconds <= 0 {
		return false
	}

	if mbps >= t.thresholdTraffic {
		t.sawTraffic = true
		t.consecutiveZero = 0
		return false
	}
	if mbps < 0.05 {
		t.consecutiveZero++
	} else {
		t.consecutiveZero = 0
	}
	if t.sawTraffic && t.consecutiveZero >= t.cfg.StallIngestSeconds {
		t.sawTraffic = false
		t.consecutiveZero = 0
		return true
	}
	return false
}
