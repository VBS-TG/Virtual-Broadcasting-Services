package httpserver

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type guestStore struct {
	db *sql.DB
}

func openGuestStore(path string) (*guestStore, error) {
	if path == "" {
		return nil, fmt.Errorf("guest db path is required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	s := &guestStore{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *guestStore) Close() error {
	return s.db.Close()
}

func (s *guestStore) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS guest_sessions (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  pin TEXT NOT NULL UNIQUE,
  session_version INTEGER NOT NULL DEFAULT 1,
  revoked INTEGER NOT NULL DEFAULT 0,
  created_at INTEGER NOT NULL,
  expires_at INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_guest_sessions_pin ON guest_sessions(pin);
`)
	return err
}

func (s *guestStore) Create(session guestSession) error {
	_, err := s.db.Exec(
		`INSERT INTO guest_sessions (id, name, pin, session_version, revoked, created_at, expires_at) VALUES (?, ?, ?, ?, 0, ?, ?)`,
		session.ID, session.Name, session.PIN, session.SessionVersion, session.CreatedAt, session.ExpiresAt,
	)
	return err
}

func (s *guestStore) Delete(id string) error {
	_, err := s.db.Exec(`UPDATE guest_sessions SET revoked=1, session_version=session_version+1 WHERE id=?`, id)
	return err
}

func (s *guestStore) GetByPIN(pin string) (*guestSession, error) {
	var out guestSession
	var revoked int
	err := s.db.QueryRow(
		`SELECT id, name, pin, session_version, revoked, created_at, expires_at FROM guest_sessions WHERE pin=?`,
		pin,
	).Scan(&out.ID, &out.Name, &out.PIN, &out.SessionVersion, &revoked, &out.CreatedAt, &out.ExpiresAt)
	if err != nil {
		return nil, err
	}
	out.Revoked = revoked == 1
	return &out, nil
}

func (s *guestStore) ValidateTokenSession(id string, sessionVersion int) bool {
	if id == "" {
		return false
	}
	var currentVersion int
	var revoked int
	var expiresAt int64
	err := s.db.QueryRow(
		`SELECT session_version, revoked, expires_at FROM guest_sessions WHERE id=?`,
		id,
	).Scan(&currentVersion, &revoked, &expiresAt)
	if err != nil {
		return false
	}
	if revoked == 1 || sessionVersion != currentVersion {
		return false
	}
	return time.Now().UTC().Unix() <= expiresAt
}
