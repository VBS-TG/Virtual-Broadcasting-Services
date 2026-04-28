package httpserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"vbs/pkg/showconfig"

	_ "modernc.org/sqlite"
)

type showConfigStore struct {
	db *sql.DB
}

func openShowConfigStore(path string) (*showConfigStore, error) {
	if path == "" {
		return nil, fmt.Errorf("show config db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &showConfigStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *showConfigStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *showConfigStore) migrate() error {
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS show_config_state (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  draft_payload TEXT NOT NULL,
  draft_updated_at INTEGER NOT NULL DEFAULT 0,
  effective_payload TEXT NOT NULL,
  effective_version INTEGER NOT NULL DEFAULT 0,
  effective_updated_at INTEGER NOT NULL DEFAULT 0
);`); err != nil {
		return err
	}
	if _, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS show_config_history (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  version INTEGER NOT NULL,
  payload TEXT NOT NULL,
  applied_at INTEGER NOT NULL,
  downstream_result TEXT NOT NULL DEFAULT '{}'
);`); err != nil {
		return err
	}
	_, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_show_hist_version ON show_config_history(version);`)
	return err
}

func (s *showConfigStore) ensureSeed() error {
	var n int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM show_config_state WHERE id=1`).Scan(&n)
	if n > 0 {
		return nil
	}
	def := showconfig.Default()
	raw, err := json.Marshal(def)
	if err != nil {
		return err
	}
	now := time.Now().UTC().Unix()
	_, err = s.db.Exec(
		`INSERT INTO show_config_state (id, draft_payload, draft_updated_at, effective_payload, effective_version, effective_updated_at)
		 VALUES (1, ?, ?, ?, 0, ?)`,
		string(raw), now, string(raw), now,
	)
	return err
}

func (s *showConfigStore) loadState() (draft showconfig.ShowConfig, draftAt int64, eff showconfig.ShowConfig, version int, effAt int64, err error) {
	if err = s.ensureSeed(); err != nil {
		return
	}
	var draftRaw, effRaw string
	var dv, ev int64
	var ver int
	err = s.db.QueryRow(
		`SELECT draft_payload, draft_updated_at, effective_payload, effective_version, effective_updated_at FROM show_config_state WHERE id=1`,
	).Scan(&draftRaw, &dv, &effRaw, &ver, &ev)
	if err != nil {
		return
	}
	if err = json.Unmarshal([]byte(draftRaw), &draft); err != nil {
		return
	}
	if err = json.Unmarshal([]byte(effRaw), &eff); err != nil {
		return
	}
	draftAt, version, effAt = dv, ver, ev
	return
}

func (s *showConfigStore) saveDraft(cfg showconfig.ShowConfig, updatedAt int64) error {
	if err := s.ensureSeed(); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if updatedAt <= 0 {
		updatedAt = time.Now().UTC().Unix()
	}
	_, err = s.db.Exec(`UPDATE show_config_state SET draft_payload = ?, draft_updated_at = ? WHERE id = 1`, string(raw), updatedAt)
	return err
}

func (s *showConfigStore) saveEffective(cfg showconfig.ShowConfig, version int, updatedAt int64) error {
	if err := s.ensureSeed(); err != nil {
		return err
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if updatedAt <= 0 {
		updatedAt = time.Now().UTC().Unix()
	}
	_, err = s.db.Exec(
		`UPDATE show_config_state SET effective_payload = ?, effective_version = ?, effective_updated_at = ? WHERE id = 1`,
		string(raw), version, updatedAt,
	)
	return err
}

func (s *showConfigStore) appendHistory(version int, cfg showconfig.ShowConfig, appliedAt int64, downstreamJSON string) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if appliedAt <= 0 {
		appliedAt = time.Now().UTC().Unix()
	}
	if downstreamJSON == "" {
		downstreamJSON = "{}"
	}
	_, err = s.db.Exec(
		`INSERT INTO show_config_history (version, payload, applied_at, downstream_result) VALUES (?, ?, ?, ?)`,
		version, string(raw), appliedAt, downstreamJSON,
	)
	return err
}

func (s *showConfigStore) listHistory(limit int) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := s.db.Query(
		`SELECT version, applied_at, downstream_result FROM show_config_history ORDER BY version DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]map[string]any, 0)
	for rows.Next() {
		var ver int
		var at int64
		var dr string
		if err := rows.Scan(&ver, &at, &dr); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"version":            ver,
			"applied_at":         at,
			"downstream_result":  json.RawMessage(dr),
		})
	}
	return out, rows.Err()
}

// previousEffective 傳回比 currentVersion 小的最大 version 之 payload（無則 sql.ErrNoRows）。
func (s *showConfigStore) previousEffectivePayload(currentVersion int) ([]byte, int, error) {
	var payload string
	var ver int
	err := s.db.QueryRow(
		`SELECT payload, version FROM show_config_history WHERE version < ? ORDER BY version DESC LIMIT 1`,
		currentVersion,
	).Scan(&payload, &ver)
	if err != nil {
		return nil, 0, err
	}
	return []byte(payload), ver, nil
}

// nextHistoryVersion 傳回下一個流水號（依 history 表 MAX(version)+1）。
func (s *showConfigStore) nextHistoryVersion() (int, error) {
	var max sql.NullInt64
	err := s.db.QueryRow(`SELECT MAX(version) FROM show_config_history`).Scan(&max)
	if err != nil {
		return 1, err
	}
	if !max.Valid || max.Int64 == 0 {
		return 1, nil
	}
	return int(max.Int64) + 1, nil
}
