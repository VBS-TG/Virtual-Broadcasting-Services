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

// Hub keeps the latest telemetry per node_id in memory.
type Hub struct {
	mu sync.RWMutex
	// key: node_id
	latest map[string]Record
}

// NewHub creates an empty hub.
func NewHub() *Hub {
	return &Hub{latest: make(map[string]Record)}
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
	h.latest[p.NodeID] = rec
	h.mu.Unlock()
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
