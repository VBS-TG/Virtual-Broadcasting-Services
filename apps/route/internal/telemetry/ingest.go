package telemetry

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// IngestCollector 依網卡 RX 位元組估算 ingest 頻寬（Mbps）。
type IngestCollector struct {
	iface    string
	prevRx   uint64
	prevTime time.Time
	ready    bool
}

func NewIngestCollector(iface string) *IngestCollector {
	return &IngestCollector{iface: iface}
}

// SampleMbps 回傳自上次呼叫以來的平均 Mbps（第一次為 0）。
func (c *IngestCollector) SampleMbps() float64 {
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
	return round2(mbps)
}

func readRXBytes(iface string) (uint64, error) {
	path := fmt.Sprintf("/sys/class/net/%s/statistics/rx_bytes", iface)
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	s := strings.TrimSpace(string(b))
	return strconv.ParseUint(s, 10, 64)
}
