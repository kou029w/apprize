package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// SQLite is a Store backed by modernc.org/sqlite (pure Go, no cgo).
type SQLite struct {
	db *sql.DB
}

// OpenSQLite opens (creating if needed) a sqlite database at path and runs
// migrations. Pass ":memory:" for an ephemeral database.
func OpenSQLite(path string) (*SQLite, error) {
	memory := path == ":memory:"
	var dsn string
	if memory {
		// A bare ":memory:" gives every pooled connection its own private
		// database, so the table created during migration is invisible to the
		// next connection. A shared cache makes one in-memory database visible
		// across the pool.
		dsn = "file::memory:?cache=shared"
	} else {
		// Enable WAL and a busy timeout for concurrent access.
		dsn = fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", path)
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	if memory {
		// Keep at least one connection alive; the shared in-memory database is
		// dropped once the last connection closes.
		db.SetMaxOpenConns(1)
	}
	s := &SQLite{db: db}
	if err := s.migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *SQLite) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS configs (
    key        TEXT PRIMARY KEY,
    format     TEXT NOT NULL,
    config     TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);`)
	return err
}

func (s *SQLite) Get(ctx context.Context, key string) (Entry, bool, error) {
	var e Entry
	err := s.db.QueryRowContext(ctx,
		`SELECT key, format, config, updated_at FROM configs WHERE key = ?`, key).
		Scan(&e.Key, &e.Format, &e.Config, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return Entry{}, false, nil
	}
	if err != nil {
		return Entry{}, false, err
	}
	return e, true, nil
}

func (s *SQLite) Put(ctx context.Context, e Entry) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO configs (key, format, config, updated_at)
VALUES (?, ?, ?, ?)
ON CONFLICT(key) DO UPDATE SET
    format = excluded.format,
    config = excluded.config,
    updated_at = excluded.updated_at;`,
		e.Key, e.Format, e.Config, e.UpdatedAt)
	return err
}

func (s *SQLite) Delete(ctx context.Context, key string) (bool, error) {
	res, err := s.db.ExecContext(ctx, `DELETE FROM configs WHERE key = ?`, key)
	if err != nil {
		return false, err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *SQLite) List(ctx context.Context) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT key FROM configs ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

func (s *SQLite) Close() error { return s.db.Close() }
