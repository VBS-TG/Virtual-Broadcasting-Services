package rtstate

import "sync"

// Buffer 保存可由控制面 API 熱更新的 SRT 緩衝參數（對應 .cursorrules Route 端動態調整）。
type Buffer struct {
	mu sync.RWMutex

	LossMaxTTL int
	LatencyMs  int
}

func NewBuffer(lossMaxTTL, latencyMs int) *Buffer {
	return &Buffer{LossMaxTTL: lossMaxTTL, LatencyMs: latencyMs}
}

func (s *Buffer) Snapshot() (lossMaxTTL, latencyMs int) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.LossMaxTTL, s.LatencyMs
}

func (s *Buffer) Update(lossMaxTTL, latencyMs int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.LossMaxTTL = lossMaxTTL
	s.LatencyMs = latencyMs
}
