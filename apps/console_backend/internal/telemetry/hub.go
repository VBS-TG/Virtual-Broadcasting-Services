// Package telemetry implements in-memory latest telemetry per node.
package telemetry

import (
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Payload is the minimal validated telemetry object (aligned with telemetry.v1).
type Payload struct {
	NodeID   string          `json:"node_id"`
	NodeType string          `json:"node_type"`
	TsMs     int64           `json:"ts_ms"`
	Metrics  json.RawMessage `json:"metrics"`
}

// Record stores one accepted telemetry message.
type Record struct {
	NodeID     string          `json:"node_id"`
	NodeType   string          `json:"node_type"`
	TsMs       int64           `json:"ts_ms"`
	Metrics    json.RawMessage `json:"metrics"`
	ReceivedAt time.Time       `json:"received_at"`
	AuthMode   string          `json:"auth_mode,omitempty"`
}

type Presence struct {
	NodeID       string    `json:"node_id"`
	NodeType     string    `json:"node_type"`
	Online       bool      `json:"online"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	LastSeenTsMs int64     `json:"last_seen_ts_ms"`
}

type StatusEvent struct {
	Type       string `json:"type"` // node_online | node_offline
	NodeID     string `json:"node_id"`
	NodeType   string `json:"node_type"`
	Online     bool   `json:"online"`
	OccurredAt int64  `json:"occurred_at_ms"`
}

// Hub keeps the latest telemetry per node_id in memory.
type Hub struct {
	mu sync.RWMutex
	// key: node_id
	latest map[string]Record
	online map[string]bool

	offlineTTL time.Duration
	subsMu     sync.RWMutex
	subs       []chan StatusEvent

	stopCh chan struct{}
	once   sync.Once
}

// NewHub creates an empty hub.
func NewHub(offlineTTL time.Duration) *Hub {
	if offlineTTL <= 0 {
		offlineTTL = 10 * time.Second
	}
	h := &Hub{
		latest:     make(map[string]Record),
		online:     make(map[string]bool),
		offlineTTL: offlineTTL,
		stopCh:     make(chan struct{}),
	}
	go h.offlineWatcher()
	return h
}

// ValidateAndStore checks JSON, required fields, and stores if valid.
func (h *Hub) ValidateAndStore(raw []byte) error {
	if len(raw) == 0 {
		return fmt.Errorf("empty payload")
	}
	var p Payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return fmt.Errorf("invalid json: %w", err)
	}
	p.NodeID = strings.TrimSpace(p.NodeID)
	p.NodeType = strings.TrimSpace(strings.ToLower(p.NodeType))
	if p.NodeID == "" {
		return fmt.Errorf("node_id required")
	}
	if p.NodeType == "" {
		return fmt.Errorf("node_type required")
	}
	switch p.NodeType {
	case "capture", "route", "engine", "console":
	default:
		return fmt.Errorf("node_type must be capture|route|engine|console")
	}
	if p.TsMs < 0 {
		return fmt.Errorf("ts_ms invalid")
	}
	if len(p.Metrics) == 0 || string(p.Metrics) == "null" {
		return fmt.Errorf("metrics required")
	}
	var stub map[string]interface{}
	if err := json.Unmarshal(p.Metrics, &stub); err != nil || len(stub) == 0 {
		return fmt.Errorf("metrics must be an object")
	}

	authMode := ""
	var aux struct {
		AuthMode string `json:"auth_mode"`
	}
	_ = json.Unmarshal(raw, &aux)
	if aux.AuthMode != "" {
		authMode = aux.AuthMode
	}

	rec := Record{
		NodeID:     p.NodeID,
		NodeType:   p.NodeType,
		TsMs:       p.TsMs,
		Metrics:    p.Metrics,
		ReceivedAt: time.Now().UTC(),
		AuthMode:   authMode,
	}
	h.mu.Lock()
	wasOnline := h.online[p.NodeID]
	h.latest[p.NodeID] = rec
	h.online[p.NodeID] = true
	h.mu.Unlock()
	if !wasOnline {
		h.publish(StatusEvent{
			Type:       "node_online",
			NodeID:     rec.NodeID,
			NodeType:   rec.NodeType,
			Online:     true,
			OccurredAt: time.Now().UTC().UnixMilli(),
		})
	}
	return nil
}

// Snapshot returns a copy of the latest records keyed by node_id.
func (h *Hub) Snapshot() map[string]Record {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]Record, len(h.latest))
	for k, v := range h.latest {
		out[k] = v
	}
	return out
}

func (h *Hub) PresenceSnapshot() map[string]Presence {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make(map[string]Presence, len(h.latest))
	for k, v := range h.latest {
		out[k] = Presence{
			NodeID:       v.NodeID,
			NodeType:     v.NodeType,
			Online:       h.online[k],
			LastSeenAt:   v.ReceivedAt,
			LastSeenTsMs: v.TsMs,
		}
	}
	return out
}

func (h *Hub) SubscribeStatusEvents() <-chan StatusEvent {
	ch := make(chan StatusEvent, 32)
	h.subsMu.Lock()
	h.subs = append(h.subs, ch)
	h.subsMu.Unlock()
	return ch
}

func (h *Hub) Close() {
	h.once.Do(func() { close(h.stopCh) })
}

func (h *Hub) offlineWatcher() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-h.stopCh:
			h.closeSubs()
			return
		case now := <-ticker.C:
			h.markOffline(now)
		}
	}
}

func (h *Hub) markOffline(now time.Time) {
	expired := make([]Record, 0, 4)
	h.mu.Lock()
	for nodeID, rec := range h.latest {
		if !h.online[nodeID] {
			continue
		}
		if now.Sub(rec.ReceivedAt) > h.offlineTTL {
			h.online[nodeID] = false
			expired = append(expired, rec)
		}
	}
	h.mu.Unlock()
	for _, rec := range expired {
		h.publish(StatusEvent{
			Type:       "node_offline",
			NodeID:     rec.NodeID,
			NodeType:   rec.NodeType,
			Online:     false,
			OccurredAt: now.UTC().UnixMilli(),
		})
	}
}

func (h *Hub) publish(ev StatusEvent) {
	h.subsMu.RLock()
	defer h.subsMu.RUnlock()
	for _, ch := range h.subs {
		select {
		case ch <- ev:
		default:
		}
	}
}

func (h *Hub) closeSubs() {
	h.subsMu.Lock()
	defer h.subsMu.Unlock()
	for _, ch := range h.subs {
		close(ch)
	}
	h.subs = nil
}
