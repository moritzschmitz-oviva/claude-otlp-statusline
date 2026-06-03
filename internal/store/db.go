package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moritzschmitz-oviva/claude-otlp-statusline/internal/otlp"
	_ "modernc.org/sqlite"
)

const dbDir = ".local/share/claude-otlp"
const dbFile = "telemetry.db"

type DB struct {
	db *sql.DB
}

func Open() (*DB, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("home dir: %w", err)
	}
	dir := filepath.Join(home, dbDir)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	path := filepath.Join(dir, dbFile)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	store := &DB{db: db}
	if err := store.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return store, nil
}

func (s *DB) Close() error {
	return s.db.Close()
}

func (s *DB) migrate() error {
	_, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS api_requests (
		session_id             TEXT NOT NULL,
		request_id             TEXT NOT NULL,
		cost_usd               REAL NOT NULL DEFAULT 0,
		input_tokens           INTEGER NOT NULL DEFAULT 0,
		output_tokens          INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens      INTEGER NOT NULL DEFAULT 0,
		cache_creation_tokens  INTEGER NOT NULL DEFAULT 0,
		model                  TEXT NOT NULL DEFAULT '',
		ts                     INTEGER NOT NULL DEFAULT 0,
		UNIQUE(request_id)
	)`)
	return err
}

func (s *DB) Insert(r otlp.ApiRequest) error {
	_, err := s.db.Exec(`
		INSERT OR IGNORE INTO api_requests
			(session_id, request_id, cost_usd, input_tokens, output_tokens,
			 cache_read_tokens, cache_creation_tokens, model, ts)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		r.SessionID, r.RequestID, r.CostUSD,
		r.InputTokens, r.OutputTokens,
		r.CacheReadTokens, r.CacheCreationTokens,
		r.Model, r.TimeUnixNano,
	)
	return err
}

type SessionStats struct {
	CostUSD     float64
	TotalTokens int64
}

func (s *DB) QuerySession(sessionID string) (*SessionStats, error) {
	row := s.db.QueryRow(`
		SELECT
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens), 0)
		FROM api_requests
		WHERE session_id = ?`,
		sessionID,
	)
	var stats SessionStats
	if err := row.Scan(&stats.CostUSD, &stats.TotalTokens); err != nil {
		return nil, err
	}
	return &stats, nil
}

func (s *DB) QueryMonthly(month string) (*SessionStats, error) {
	row := s.db.QueryRow(`
		SELECT
			COALESCE(SUM(cost_usd), 0),
			COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens), 0)
		FROM api_requests
		WHERE strftime('%Y-%m', ts / 1000000000, 'unixepoch') = ?`,
		month,
	)
	var stats SessionStats
	if err := row.Scan(&stats.CostUSD, &stats.TotalTokens); err != nil {
		return nil, err
	}
	return &stats, nil
}

func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dbDir, dbFile), nil
}
