// SPDX-License-Identifier: GPL-3.0-or-later

// Package fnmeta provides a local SQLite cache for Snowflake function metadata
// used by the SQL editor autocomplete and hover tooltip features.
//
// Data flows through three tiers:
//  1. An embedded snowflake_builtin_fallback.json provides an offline-first baseline.
//  2. A per-user SQLite database (fn_metadata.db) is the primary read path for the UI.
//  3. A live Snowflake connection is queried in the background (see sync.go) to refresh
//     the cache and add user-defined functions (UDFs).
package fnmeta

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	_ "modernc.org/sqlite" // pure-Go SQLite driver; no CGO required
)

// FunctionMeta describes a single Snowflake function entry.
type FunctionMeta struct {
	FunctionName      string `json:"functionName"`
	FunctionSignature string `json:"functionSignature"`
	Description       string `json:"description"`
	FunctionType      string `json:"functionType"` // "BUILTIN" or "UDF"
}

// Store wraps the local SQLite cache.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) the SQLite database at <dir>/fn_metadata.db.
// The directory is created if it does not exist.
// The caller should call Close when done.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("fnmeta: mkdir %s: %w", dir, err)
	}
	dbPath := filepath.Join(dir, "fn_metadata.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("fnmeta: open %s: %w", dbPath, err)
	}
	s := &Store{db: db}
	if err := s.init(); err != nil {
		db.Close() //nolint:errcheck
		return nil, err
	}
	return s, nil
}

// Close closes the underlying database connection.
func (s *Store) Close() error { return s.db.Close() }

// schemaVersion is incremented whenever a breaking change (e.g. data-quality
// fix) requires the cache to be rebuilt from the embedded fallback.
const schemaVersion = 3

// init creates (or migrates) the schema. A user_version pragma is used as a
// lightweight version tag: when the stored version is older than schemaVersion
// the table is dropped and recreated, triggering a clean re-seed from the
// fallback on the next LoadFallback call.
func (s *Store) init() error {
	var version int
	_ = s.db.QueryRow("PRAGMA user_version").Scan(&version)
	if version < schemaVersion {
		if _, err := s.db.Exec(`DROP TABLE IF EXISTS function_metadata`); err != nil {
			return err
		}
	}
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS function_metadata (
			function_name      TEXT NOT NULL,
			function_signature TEXT NOT NULL,
			description        TEXT,
			function_type      TEXT NOT NULL,
			last_synced_at     DATETIME DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (function_name, function_signature)
		);
		CREATE INDEX IF NOT EXISTS idx_autocomplete_name
			ON function_metadata(function_name);
	`); err != nil {
		return err
	}
	_, err := s.db.Exec(fmt.Sprintf("PRAGMA user_version = %d", schemaVersion))
	return err
}

// Search returns up to 50 functions whose name begins with prefix (case-insensitive).
// Results are ordered shortest-name-first then alphabetically.
func (s *Store) Search(prefix string) ([]FunctionMeta, error) {
	rows, err := s.db.Query(`
		SELECT function_name, function_signature, description, function_type
		FROM   function_metadata
		WHERE  function_name LIKE ?
		ORDER  BY LENGTH(function_name) ASC, function_name ASC
		LIMIT  50`,
		prefix+"%",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	return scanRows(rows)
}

// GetAllNames returns every distinct (function_name, function_type) pair in
// the cache. It is used to build the editor's decoration set for syntax
// highlighting. Only FunctionName and FunctionType are populated.
func (s *Store) GetAllNames() ([]FunctionMeta, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT function_name, function_type
		FROM   function_metadata
		ORDER  BY function_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	var result []FunctionMeta
	for rows.Next() {
		var m FunctionMeta
		if err := rows.Scan(&m.FunctionName, &m.FunctionType); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// Lookup returns all overloads for the given exact function name.
// The caller should upper-case the name before calling.
func (s *Store) Lookup(name string) ([]FunctionMeta, error) {
	rows, err := s.db.Query(`
		SELECT function_name, function_signature, description, function_type
		FROM   function_metadata
		WHERE  function_name = ?`,
		name,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close() //nolint:errcheck
	return scanRows(rows)
}

// Upsert inserts or updates the given function metadata entries in a single
// transaction. Existing rows (matched on function_name + function_signature)
// have their description, type, and last_synced_at refreshed.
func (s *Store) Upsert(metas []FunctionMeta) error {
	if len(metas) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	stmt, err := tx.Prepare(`
		INSERT INTO function_metadata
			(function_name, function_signature, description, function_type, last_synced_at)
		VALUES (?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(function_name, function_signature) DO UPDATE SET
			description    = excluded.description,
			function_type  = excluded.function_type,
			last_synced_at = CURRENT_TIMESTAMP`)
	if err != nil {
		tx.Rollback() //nolint:errcheck
		return err
	}
	defer stmt.Close() //nolint:errcheck
	for _, m := range metas {
		if _, err := stmt.Exec(m.FunctionName, m.FunctionSignature, m.Description, m.FunctionType); err != nil {
			tx.Rollback() //nolint:errcheck
			return err
		}
	}
	return tx.Commit()
}

// LoadFallback reads the embedded built-in function JSON and upserts it into
// the store. It is a no-op if the JSON is already up-to-date (ON CONFLICT).
func (s *Store) LoadFallback() error {
	var metas []FunctionMeta
	if err := json.Unmarshal(fallbackData, &metas); err != nil {
		return fmt.Errorf("fnmeta: parse fallback JSON: %w", err)
	}
	return s.Upsert(metas)
}

// scanRows is a helper that scans sql.Rows into a []FunctionMeta slice.
func scanRows(rows *sql.Rows) ([]FunctionMeta, error) {
	var result []FunctionMeta
	for rows.Next() {
		var m FunctionMeta
		if err := rows.Scan(&m.FunctionName, &m.FunctionSignature, &m.Description, &m.FunctionType); err != nil {
			return nil, err
		}
		result = append(result, m)
	}
	return result, rows.Err()
}
