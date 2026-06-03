package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

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
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`CREATE TABLE IF NOT EXISTS spend_overrides (
		month      TEXT PRIMARY KEY,
		target_usd REAL NOT NULL,
		set_at     DATETIME NOT NULL
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

func (s *DB) SetSpendOverride(month string, targetUSD float64) error {
	if targetUSD == 0 {
		_, err := s.db.Exec(`DELETE FROM spend_overrides WHERE month = ?`, month)
		return err
	}
	setAt := time.Now().UTC().Format(time.RFC3339)
	_, err := s.db.Exec(`
		INSERT INTO spend_overrides (month, target_usd, set_at)
		VALUES (?, ?, ?)
		ON CONFLICT(month) DO UPDATE SET target_usd = excluded.target_usd, set_at = excluded.set_at`,
		month, targetUSD, setAt,
	)
	return err
}

type SessionStats struct {
	CostUSD     float64
	TotalTokens int64
}

type MonthlyStats struct {
	CostUSD          float64
	TotalTokens      int64
	OverrideActive   bool
	OverrideTarget   float64
	OverrideSetAt    time.Time
	PostOverrideCost float64
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

func (s *DB) QueryMonthly(month string) (*MonthlyStats, error) {
	stats := &MonthlyStats{}

	var targetUSD sql.NullFloat64
	var setAtStr sql.NullString
	overrideRow := s.db.QueryRow(`SELECT target_usd, set_at FROM spend_overrides WHERE month = ?`, month)
	if err := overrideRow.Scan(&targetUSD, &setAtStr); err != nil && err != sql.ErrNoRows {
		return nil, fmt.Errorf("query override: %w", err)
	}
	if targetUSD.Valid {
		stats.OverrideActive = true
		stats.OverrideTarget = targetUSD.Float64
		if setAtStr.Valid {
			if t, err := time.Parse(time.RFC3339, setAtStr.String); err == nil {
				stats.OverrideSetAt = t
			}
		}
	}

	if stats.OverrideActive {
		setAtNano := stats.OverrideSetAt.UnixNano()
		row := s.db.QueryRow(`
			SELECT
				COALESCE(SUM(cost_usd), 0),
				COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens), 0),
				COALESCE(SUM(CASE WHEN ts > ? THEN cost_usd ELSE 0 END), 0)
			FROM api_requests
			WHERE strftime('%Y-%m', ts / 1000000000, 'unixepoch') = ?`,
			setAtNano, month,
		)
		if err := row.Scan(&stats.CostUSD, &stats.TotalTokens, &stats.PostOverrideCost); err != nil {
			return nil, err
		}
	} else {
		row := s.db.QueryRow(`
			SELECT
				COALESCE(SUM(cost_usd), 0),
				COALESCE(SUM(input_tokens + output_tokens + cache_read_tokens + cache_creation_tokens), 0)
			FROM api_requests
			WHERE strftime('%Y-%m', ts / 1000000000, 'unixepoch') = ?`,
			month,
		)
		if err := row.Scan(&stats.CostUSD, &stats.TotalTokens); err != nil {
			return nil, err
		}
	}

	return stats, nil
}

func DBPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, dbDir, dbFile), nil
}
