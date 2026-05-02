package httpserver

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"vbs/pkg/showconfig"
)

const maxShowConfigBodyBytes = 512 * 1024

func (s *Server) handleShowConfigGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.showStore == nil {
		http.Error(w, `{"error":"show config store unavailable"}`, http.StatusInternalServerError)
		return
	}
	draft, draftAt, eff, ver, effAt, err := s.showStore.loadState()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"load failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"draft":               draft,
		"draft_updated_at":    draftAt,
		"effective":           eff,
		"effective_version":   ver,
		"effective_updated_at": effAt,
	})
}

func (s *Server) handleShowConfigDraftPut(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.showStore == nil {
		http.Error(w, `{"error":"show config store unavailable"}`, http.StatusInternalServerError)
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, maxShowConfigBodyBytes))
	if err != nil {
		http.Error(w, `{"error":"read body"}`, http.StatusBadRequest)
		return
	}
	var cfg showconfig.ShowConfig
	if err := json.Unmarshal(body, &cfg); err != nil {
		http.Error(w, `{"error":"invalid json"}`, http.StatusBadRequest)
		return
	}
	showconfig.Normalize(&cfg)
	rt, _, _ := s.getRuntimeConfigSnapshot()
	if err := showconfig.Validate(cfg, rt.Inputs); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, trimErr(err)), http.StatusBadRequest)
		return
	}
	now := time.Now().UTC().Unix()
	if err := s.showStore.saveDraft(cfg, now); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"save draft: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"saved":            true,
		"draft":            cfg,
		"draft_updated_at": now,
	})
}

func (s *Server) handleShowConfigApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.showStore == nil {
		http.Error(w, `{"error":"show config store unavailable"}`, http.StatusInternalServerError)
		return
	}
	draft, _, _, effVer, _, err := s.showStore.loadState()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"load failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	showconfig.Normalize(&draft)
	rt, _, _ := s.getRuntimeConfigSnapshot()
	if err := showconfig.Validate(draft, rt.Inputs); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, trimErr(err)), http.StatusBadRequest)
		return
	}

	payload, err := json.Marshal(draft)
	if err != nil {
		http.Error(w, `{"error":"marshal"}`, http.StatusInternalServerError)
		return
	}

	downstream := map[string]any{}
	allOK := true

	if u := strings.TrimSpace(s.cfg.CaptureControlBaseURL); u != "" {
		raw, st, err := s.captureControlPOST("/api/v1/show-config/apply", payload)
		downstream["capture"] = map[string]any{"ok": err == nil && st < 400, "status": st, "raw": string(raw), "error": firstErr(err)}
		if err != nil || st >= 400 {
			allOK = false
		}
	} else {
		downstream["capture"] = map[string]any{"ok": true, "skipped": true, "reason": "VBS_CAPTURE_CONTROL_BASE_URL 未設定"}
	}

	if u := strings.TrimSpace(s.cfg.RouteControlBaseURL); u != "" {
		raw, st, err := s.routeControlPOST("/api/v1/show-config/apply", payload)
		downstream["route"] = map[string]any{"ok": err == nil && st < 400, "status": st, "raw": string(raw), "error": firstErr(err)}
		if err != nil || st >= 400 {
			allOK = false
		}
	} else {
		downstream["route"] = map[string]any{"ok": true, "skipped": true, "reason": "VBS_ROUTE_CONTROL_BASE_URL 未設定"}
	}

	if u := strings.TrimSpace(s.cfg.EngineControlBaseURL); u != "" {
		raw, st, err := s.engineControlPOST("/api/v1/show-config/apply", payload)
		downstream["engine"] = map[string]any{"ok": err == nil && st < 400, "status": st, "raw": string(raw), "error": firstErr(err)}
		if err != nil || st >= 400 {
			allOK = false
		}
	} else {
		downstream["engine"] = map[string]any{"ok": true, "skipped": true, "reason": "VBS_ENGINE_CONTROL_BASE_URL 未設定"}
	}

	drj, _ := json.Marshal(downstream)

	if !allOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":          false,
			"downstream":  json.RawMessage(drj),
			"message":     "節點套用失敗；未變更 Console 之 effective 快照",
			"effective_version": effVer,
		})
		return
	}

	nextVer, err := s.showStore.nextHistoryVersion()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"version: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	now := time.Now().UTC().Unix()
	if err := s.showStore.saveEffective(draft, nextVer, now); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"save effective: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	if err := s.showStore.appendHistory(nextVer, draft, now, string(drj)); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"history: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	_ = s.showStore.saveDraft(draft, now)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                 true,
		"effective":        draft,
		"effective_version": nextVer,
		"effective_updated_at": now,
		"downstream":       json.RawMessage(drj),
	})
}

func (s *Server) handleShowConfigRollback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.adminAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.showStore == nil {
		http.Error(w, `{"error":"show config store unavailable"}`, http.StatusInternalServerError)
		return
	}
	_, _, _, effVer, _, err := s.showStore.loadState()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"load failed: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	if effVer <= 0 {
		http.Error(w, `{"error":"無已套用版本可回滾"}`, http.StatusBadRequest)
		return
	}
	prevRaw, prevVer, err := s.showStore.previousEffectivePayload(effVer)
	if err != nil {
		http.Error(w, `{"error":"找不到上一版快照"}`, http.StatusBadRequest)
		return
	}
	var prevCfg showconfig.ShowConfig
	if err := json.Unmarshal(prevRaw, &prevCfg); err != nil {
		http.Error(w, `{"error":"上一版資料損毀"}`, http.StatusInternalServerError)
		return
	}
	showconfig.Normalize(&prevCfg)
	rt, _, _ := s.getRuntimeConfigSnapshot()
	if err := showconfig.Validate(prevCfg, rt.Inputs); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"上一版與目前 runtime 路數不相容: %s"}`, trimErr(err)), http.StatusConflict)
		return
	}

	payload, err := json.Marshal(prevCfg)
	if err != nil {
		http.Error(w, `{"error":"marshal"}`, http.StatusInternalServerError)
		return
	}

	downstream := map[string]any{}
	allOK := true

	if strings.TrimSpace(s.cfg.CaptureControlBaseURL) != "" {
		raw, st, err := s.captureControlPOST("/api/v1/show-config/apply", payload)
		downstream["capture"] = map[string]any{"ok": err == nil && st < 400, "status": st, "raw": string(raw), "error": firstErr(err)}
		if err != nil || st >= 400 {
			allOK = false
		}
	} else {
		downstream["capture"] = map[string]any{"ok": true, "skipped": true}
	}

	if strings.TrimSpace(s.cfg.RouteControlBaseURL) != "" {
		raw, st, err := s.routeControlPOST("/api/v1/show-config/apply", payload)
		downstream["route"] = map[string]any{"ok": err == nil && st < 400, "status": st, "raw": string(raw), "error": firstErr(err)}
		if err != nil || st >= 400 {
			allOK = false
		}
	} else {
		downstream["route"] = map[string]any{"ok": true, "skipped": true}
	}

	if strings.TrimSpace(s.cfg.EngineControlBaseURL) != "" {
		raw, st, err := s.engineControlPOST("/api/v1/show-config/apply", payload)
		downstream["engine"] = map[string]any{"ok": err == nil && st < 400, "status": st, "raw": string(raw), "error": firstErr(err)}
		if err != nil || st >= 400 {
			allOK = false
		}
	} else {
		downstream["engine"] = map[string]any{"ok": true, "skipped": true}
	}

	drj, _ := json.Marshal(downstream)

	if !allOK {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":         false,
			"downstream": json.RawMessage(drj),
			"message":    "節點回滾套用失敗；Console 未變更",
		})
		return
	}

	now := time.Now().UTC().Unix()
	if err := s.showStore.saveEffective(prevCfg, prevVer, now); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"save effective: %s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	_ = s.showStore.saveDraft(prevCfg, now)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"ok":                  true,
		"effective":         prevCfg,
		"effective_version":   prevVer,
		"effective_updated_at": now,
		"downstream":        json.RawMessage(drj),
	})
}

func (s *Server) handleShowConfigHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, `{"error":"method not allowed"}`, http.StatusMethodNotAllowed)
		return
	}
	if !s.controlAuthorized(r) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}
	if s.showStore == nil {
		http.Error(w, `{"error":"show config store unavailable"}`, http.StatusInternalServerError)
		return
	}
	limit := 50
	if v := strings.TrimSpace(r.URL.Query().Get("limit")); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	rows, err := s.showStore.listHistory(limit)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, trimErr(err)), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"history": rows})
}

func (s *Server) captureControlPOST(path string, body []byte) ([]byte, int, error) {
	if strings.TrimSpace(s.cfg.CaptureControlBaseURL) == "" {
		return nil, 0, fmt.Errorf("capture base url not configured")
	}
	base := strings.TrimRight(s.cfg.CaptureControlBaseURL, "/")
	target := base + path
	req, err := http.NewRequest(http.MethodPost, target, strings.NewReader(string(body)))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/json")
	s.attachCaptureServiceToken(req)
	client := upstreamHTTPClient(15 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return raw, resp.StatusCode, nil
}
