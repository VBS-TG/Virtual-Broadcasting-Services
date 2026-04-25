package httpserver

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type runtimeStore struct {
	db *sql.DB
}

func openRuntimeStore(path string) (*runtimeStore, error) {
	if path == "" {
		return nil, fmt.Errorf("runtime db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &runtimeStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *runtimeStore) Close() error {
	return s.db.Close()
}

func (s *runtimeStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS runtime_config (
  id INTEGER PRIMARY KEY CHECK (id = 1),
  payload TEXT NOT NULL,
  updated_at INTEGER NOT NULL
);
`)
	return err
}

func (s *runtimeStore) Save(cfg runtimeConfig, updatedAt int64) error {
	raw, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	if updatedAt <= 0 {
		updatedAt = time.Now().UTC().Unix()
	}
	_, err = s.db.Exec(
		`INSERT INTO runtime_config (id, payload, updated_at) VALUES (1, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET payload=excluded.payload, updated_at=excluded.updated_at`,
		string(raw), updatedAt,
	)
	return err
}

func (s *runtimeStore) Load() (runtimeConfig, int64, error) {
	var payload string
	var updatedAt int64
	err := s.db.QueryRow(`SELECT payload, updated_at FROM runtime_config WHERE id=1`).Scan(&payload, &updatedAt)
	if err != nil {
		return runtimeConfig{}, 0, err
	}
	var cfg runtimeConfig
	if err := json.Unmarshal([]byte(payload), &cfg); err != nil {
		return runtimeConfig{}, 0, err
	}
	return cfg, updatedAt, nil
}
