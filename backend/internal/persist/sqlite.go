// Package persist provides the infrastructure adapter for the game layer's Store
// interface (SPEC 13): the game package owns the abstraction; this package depends
// on the game layer, never the reverse. The whole state persists as one snapshot in
// a single save row rather than per-entity tables, keeping the abstraction focused
// on what the application needs.
package persist

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/DanielZ-Org/Postman/backend/internal/game"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements game.Store on a local SQLite database file.
type SQLiteStore struct {
	mu sync.Mutex // serialises save/load against the shared connection
	db *sql.DB
}

// NewSQLite opens (creating if needed) the SQLite database at path and ensures the
// single-row save table exists. It returns an error when the file cannot be created
// or the schema cannot be applied.
func NewSQLite(path string) (*SQLiteStore, error) {
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, fmt.Errorf("persist: create database directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("persist: open database: %w", err)
	}
	// A single writer with a busy timeout keeps the app simple and safe if a second
	// process touches the file.
	schema := `CREATE TABLE IF NOT EXISTS save (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		data TEXT NOT NULL,
		updated_at TEXT NOT NULL
	)`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("persist: create schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Load reads the saved snapshot. It returns (nil, nil) when no save exists yet, so
// a fresh game starts normally.
func (s *SQLiteStore) Load() (*game.Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var data string
	err := s.db.QueryRow(`SELECT data FROM save WHERE id = 1`).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("persist: read save row: %w", err)
	}
	var snap game.Snapshot
	if err := json.Unmarshal([]byte(data), &snap); err != nil {
		return nil, fmt.Errorf("persist: decode snapshot: %w", err)
	}
	return &snap, nil
}

// Save persists the snapshot in one durable statement (single-row replace).
func (s *SQLiteStore) Save(snap *game.Snapshot) error {
	if snap == nil {
		return errors.New("persist: refusing to save a nil snapshot")
	}
	data, err := json.Marshal(snap)
	if err != nil {
		return fmt.Errorf("persist: encode snapshot: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	_, err = s.db.Exec(
		`INSERT INTO save (id, data, updated_at) VALUES (1, ?, datetime('now'))
		 ON CONFLICT(id) DO UPDATE SET data = excluded.data, updated_at = excluded.updated_at`,
		string(data),
	)
	if err != nil {
		return fmt.Errorf("persist: write save row: %w", err)
	}
	return nil
}

// Close releases the database connection.
func (s *SQLiteStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}
