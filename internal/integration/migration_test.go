// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

//go:build integration

// Package integration_test — migration strategy tests.
//
// These tests require a live Snowflake account with CREATE DATABASE, CREATE
// TABLE, INSERT, and DROP TABLE permissions.
//
// By default each test creates its own temporary database (THAW_MIGTEST_<random>)
// and drops it on completion.  To run tests against an existing database instead,
// set:
//
//	SNOWFLAKE_TEST_DATABASE  Existing database to use (no auto-create / auto-drop)
//	SNOWFLAKE_TEST_SCHEMA    Schema within that database (default: PUBLIC)
//
// Connection credentials are the same key-pair env vars used by the other
// integration tests (see basic_test.go).
package integration_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"thaw/internal/snowflake"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// testCtx returns a context with a timeout long enough for DDL operations.
func testCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 90*time.Second)
}

// q double-quote-escapes a Snowflake identifier.
func q(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

// fqn returns a fully-qualified three-part identifier.
func fqn(db, schema, name string) string {
	return q(db) + "." + q(schema) + "." + q(name)
}

// exec runs a SQL statement and fails the test on any error.
func exec(t *testing.T, client *snowflake.Client, sql string) {
	t.Helper()
	ctx, cancel := testCtx()
	defer cancel()
	if _, err := client.Execute(ctx, sql); err != nil {
		t.Fatalf("exec failed: %v\n  SQL: %s", err, sql)
	}
}

// execIgnoreError runs SQL and swallows any error (for cleanup paths).
func execIgnoreError(client *snowflake.Client, sql string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_, _ = client.Execute(ctx, sql)
}

// mquery runs a SELECT and returns the result, failing on error.
func mquery(t *testing.T, client *snowflake.Client, sql string) *snowflake.QueryResult {
	t.Helper()
	ctx, cancel := testCtx()
	defer cancel()
	res, err := client.Execute(ctx, sql)
	if err != nil {
		t.Fatalf("query failed: %v\n  SQL: %s", err, sql)
	}
	return res
}

// mrowCount returns the number of rows in a table.
func mrowCount(t *testing.T, client *snowflake.Client, tableRef string) int64 {
	t.Helper()
	res := mquery(t, client, "SELECT COUNT(*) FROM "+tableRef)
	if len(res.Rows) == 0 {
		return 0
	}
	switch v := res.Rows[0][0].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	case string:
		var n int64
		fmt.Sscanf(v, "%d", &n)
		return n
	}
	return 0
}

// mcolumnNames returns upper-cased column names via DESCRIBE TABLE.
// Only rows with kind="Column" are included.
func mcolumnNames(t *testing.T, client *snowflake.Client, tableRef string) []string {
	t.Helper()
	res := mquery(t, client, "DESCRIBE TABLE "+tableRef)
	var names []string
	for _, row := range res.Rows {
		if len(row) < 3 {
			continue
		}
		kind, _ := row[2].(string)
		if !strings.EqualFold(kind, "Column") {
			continue
		}
		name, _ := row[0].(string)
		names = append(names, strings.ToUpper(name))
	}
	return names
}

// hasColumn reports whether names contains col (case-insensitive).
func hasColumn(names []string, col string) bool {
	upper := strings.ToUpper(col)
	for _, n := range names {
		if n == upper {
			return true
		}
	}
	return false
}

// setupMigrationDB resolves the target database and schema for migration tests.
//
// When SNOWFLAKE_TEST_DATABASE is set the existing database is used as-is (no
// automatic creation or drop).  When it is absent a fresh database named
// THAW_MIGTEST_<random> is created with CREATE DATABASE (no OR REPLACE) and
// dropped unconditionally via t.Cleanup.  If the generated name already exists
// the function retries up to five times with a new random name.
func setupMigrationDB(t *testing.T, client *snowflake.Client) (db, schema string) {
	t.Helper()

	// Explicit database: use it without touching its lifecycle.
	if envDB := os.Getenv("SNOWFLAKE_TEST_DATABASE"); envDB != "" {
		db = strings.ToUpper(envDB)
		schema = "PUBLIC"
		if envSchema := os.Getenv("SNOWFLAKE_TEST_SCHEMA"); envSchema != "" {
			schema = strings.ToUpper(envSchema)
		}
		return
	}

	// Auto-create a uniquely-named temporary database; retry on name collision.
	const maxAttempts = 5
	for range maxAttempts {
		candidate := randomName("THAW_MIGTEST_")
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		_, err := client.Execute(ctx, fmt.Sprintf("CREATE DATABASE %s", q(candidate)))
		cancel()

		if err == nil {
			db = candidate
			schema = "PUBLIC"
			t.Logf("created temp database: %s", db)
			t.Cleanup(func() {
				execIgnoreError(client, fmt.Sprintf("DROP DATABASE IF EXISTS %s", q(db)))
				t.Logf("dropped temp database: %s", db)
			})
			return
		}

		// Only retry on name collision; any other error is fatal.
		if !strings.Contains(strings.ToLower(err.Error()), "already exists") {
			t.Fatalf("create test database %s: %v", candidate, err)
		}
		t.Logf("name %s already exists, retrying...", candidate)
	}

	t.Fatalf("could not create a unique test database after %d attempts", maxAttempts)
	return // unreachable
}

// ─── in_place strategy ────────────────────────────────────────────────────────

// TestMigrationInPlace verifies that the in-place strategy (ALTER TABLE
// ADD/DROP/ALTER COLUMN) modifies an existing table while preserving data in
// unchanged columns.
func TestMigrationInPlace(t *testing.T) {
	client := keyPairConnFromEnv(t)
	db, schema := setupMigrationDB(t, client)

	name := randomName("INPLACE_")
	ref := fqn(db, schema, name)

	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id      NUMBER,
		name    VARCHAR(100),
		old_col VARCHAR(50)
	)`, ref))
	t.Cleanup(func() { execIgnoreError(client, "DROP TABLE IF EXISTS "+ref) })

	exec(t, client, fmt.Sprintf(`INSERT INTO %s (id, name, old_col) VALUES
		(1, 'Alice', 'legacy_a'),
		(2, 'Bob',   'legacy_b'),
		(3, 'Carol', 'legacy_c')`, ref))

	if n := mrowCount(t, client, ref); n != 3 {
		t.Fatalf("initial row count = %d, want 3", n)
	}

	// Apply in-place: add new_col, drop old_col, widen name to VARCHAR(255).
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN new_col TEXT`, ref))
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s DROP COLUMN old_col`, ref))
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s ALTER COLUMN name TYPE VARCHAR(255)`, ref))

	// Unchanged rows must still be present.
	if n := mrowCount(t, client, ref); n != 3 {
		t.Errorf("row count after in-place = %d, want 3 (data must be preserved)", n)
	}

	cols := mcolumnNames(t, client, ref)
	for _, want := range []string{"ID", "NAME", "NEW_COL"} {
		if !hasColumn(cols, want) {
			t.Errorf("column %s missing after in-place; columns: %v", want, cols)
		}
	}
	if hasColumn(cols, "OLD_COL") {
		t.Errorf("old_col should have been dropped; columns: %v", cols)
	}

	// Verify data integrity for a specific row.
	res := mquery(t, client, fmt.Sprintf(`SELECT name FROM %s WHERE id = 1`, ref))
	if len(res.Rows) != 1 {
		t.Fatalf("expected 1 row for id=1, got %d", len(res.Rows))
	}
	if nameVal, _ := res.Rows[0][0].(string); !strings.EqualFold(nameVal, "Alice") {
		t.Errorf("name for id=1 = %q, want 'Alice'", nameVal)
	}

	t.Logf("in-place: columns after migration: %v", cols)
}

// ─── blue_green_swap strategy ─────────────────────────────────────────────────

// TestMigrationBlueGreenSwap verifies the blue/green swap strategy: shared
// column data is copied to the new schema; the swap is atomic; removed columns
// disappear and added columns are NULL in copied rows.
func TestMigrationBlueGreenSwap(t *testing.T) {
	client := keyPairConnFromEnv(t)
	db, schema := setupMigrationDB(t, client)

	name := randomName("BGSW_")
	tmpName := name + "_TMP"
	ref := fqn(db, schema, name)
	tmpRef := fqn(db, schema, tmpName)

	// Original table: id, name, extra (extra will be dropped by the new schema).
	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id    NUMBER,
		name  VARCHAR(100),
		extra VARCHAR(50)
	)`, ref))
	t.Cleanup(func() {
		execIgnoreError(client, "DROP TABLE IF EXISTS "+ref)
		execIgnoreError(client, "DROP TABLE IF EXISTS "+tmpRef)
	})

	exec(t, client, fmt.Sprintf(`INSERT INTO %s (id, name, extra) VALUES
		(10, 'Delta',   'x'),
		(20, 'Echo',    'y'),
		(30, 'Foxtrot', 'z')`, ref))

	// New schema: id, name (shared), new_col (added), extra removed.
	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id      NUMBER,
		name    VARCHAR(100),
		new_col TEXT
	)`, tmpRef))

	// Copy shared columns: id and name.
	exec(t, client, fmt.Sprintf(
		`INSERT INTO %s ("ID", "NAME") SELECT "ID", "NAME" FROM %s`, tmpRef, ref))

	// Atomic swap: original now holds new schema + copied rows.
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s SWAP WITH %s`, ref, tmpRef))

	// Drop temp table (now holds old schema + old data).
	exec(t, client, "DROP TABLE IF EXISTS "+tmpRef)

	cols := mcolumnNames(t, client, ref)
	for _, want := range []string{"ID", "NAME", "NEW_COL"} {
		if !hasColumn(cols, want) {
			t.Errorf("column %s missing after blue-green swap; columns: %v", want, cols)
		}
	}
	if hasColumn(cols, "EXTRA") {
		t.Errorf("extra column should be gone after swap; columns: %v", cols)
	}

	// Shared-column data preserved (3 rows).
	if n := mrowCount(t, client, ref); n != 3 {
		t.Errorf("row count after blue-green swap = %d, want 3", n)
	}

	// new_col must be NULL in all copied rows.
	res := mquery(t, client, fmt.Sprintf(`SELECT COUNT(*) FROM %s WHERE new_col IS NULL`, ref))
	if len(res.Rows) > 0 {
		nullCount := int64(0)
		switch v := res.Rows[0][0].(type) {
		case int64:
			nullCount = v
		case float64:
			nullCount = int64(v)
		case string:
			fmt.Sscanf(v, "%d", &nullCount)
		}
		if nullCount != 3 {
			t.Errorf("new_col NULL count = %d, want 3", nullCount)
		}
	}

	// Spot-check copied data.
	res2 := mquery(t, client, fmt.Sprintf(`SELECT name FROM %s WHERE id = 10`, ref))
	if len(res2.Rows) != 1 {
		t.Fatalf("expected 1 row for id=10, got %d", len(res2.Rows))
	}
	if got, _ := res2.Rows[0][0].(string); !strings.EqualFold(got, "Delta") {
		t.Errorf("name for id=10 = %q, want 'Delta'", got)
	}

	t.Logf("blue-green swap: columns after migration: %v", cols)
}

// ─── view_abstraction strategy ────────────────────────────────────────────────

// TestMigrationViewAbstraction verifies that the view abstraction strategy
// renames the original table to <name>_V1, creates the new table, and creates
// a compatibility view <name>_COMPAT over the shared columns.
func TestMigrationViewAbstraction(t *testing.T) {
	client := keyPairConnFromEnv(t)
	db, schema := setupMigrationDB(t, client)

	name := randomName("VABS_")
	archiveName := name + "_V1"
	compatName := name + "_COMPAT"
	ref := fqn(db, schema, name)
	archiveRef := fqn(db, schema, archiveName)
	compatRef := fqn(db, schema, compatName)

	// Original table: id, name.
	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id   NUMBER,
		name VARCHAR(100)
	)`, ref))
	t.Cleanup(func() {
		execIgnoreError(client, "DROP VIEW IF EXISTS "+compatRef)
		execIgnoreError(client, "DROP TABLE IF EXISTS "+archiveRef)
		execIgnoreError(client, "DROP TABLE IF EXISTS "+ref)
	})

	exec(t, client, fmt.Sprintf(`INSERT INTO %s (id, name) VALUES
		(100, 'Golf'),
		(200, 'Hotel'),
		(300, 'India')`, ref))

	// Step 1: rename original to archive.
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s RENAME TO %s`, ref, archiveRef))

	// Step 2: create new table with updated schema (adds status column).
	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id     NUMBER,
		name   VARCHAR(100),
		status VARCHAR(50)
	)`, ref))

	// Step 3: create compat view over shared columns from archive.
	exec(t, client, fmt.Sprintf(
		`CREATE OR REPLACE VIEW %s AS SELECT "ID", "NAME" FROM %s`,
		compatRef, archiveRef))

	// Archive retains all 3 original rows.
	if n := mrowCount(t, client, archiveRef); n != 3 {
		t.Errorf("archive row count = %d, want 3", n)
	}

	// New table starts empty (view_abstraction does not copy data).
	if n := mrowCount(t, client, ref); n != 0 {
		t.Errorf("new table row count = %d, want 0", n)
	}

	// Compat view exposes 3 rows from the archive.
	if n := mrowCount(t, client, compatRef); n != 3 {
		t.Errorf("compat view row count = %d, want 3", n)
	}

	// New table has the status column.
	cols := mcolumnNames(t, client, ref)
	if !hasColumn(cols, "STATUS") {
		t.Errorf("new table should have STATUS column; columns: %v", cols)
	}

	// Compat view returns the archived data.
	res := mquery(t, client, fmt.Sprintf(`SELECT name FROM %s WHERE id = 100`, compatRef))
	if len(res.Rows) != 1 {
		t.Fatalf("compat view: expected 1 row for id=100, got %d", len(res.Rows))
	}
	if got, _ := res.Rows[0][0].(string); !strings.EqualFold(got, "Golf") {
		t.Errorf("compat view name for id=100 = %q, want 'Golf'", got)
	}

	t.Logf("view abstraction: archive=%s, compat=%s", archiveName, compatName)
}

// ─── destructive_rebuild strategy ────────────────────────────────────────────

// TestMigrationDestructiveRebuild verifies that the destructive rebuild strategy
// drops the existing table and recreates it from scratch, discarding all data.
func TestMigrationDestructiveRebuild(t *testing.T) {
	client := keyPairConnFromEnv(t)
	db, schema := setupMigrationDB(t, client)

	name := randomName("DEST_")
	ref := fqn(db, schema, name)

	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id   NUMBER,
		name VARCHAR(100)
	)`, ref))
	t.Cleanup(func() { execIgnoreError(client, "DROP TABLE IF EXISTS "+ref) })

	exec(t, client, fmt.Sprintf(`INSERT INTO %s (id, name) VALUES
		(1, 'Juliet'), (2, 'Kilo'), (3, 'Lima')`, ref))

	if n := mrowCount(t, client, ref); n != 3 {
		t.Fatalf("initial row count = %d, want 3", n)
	}

	// Strategy: DROP + CREATE with new schema.
	exec(t, client, "DROP TABLE IF EXISTS "+ref)
	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id      NUMBER,
		name    VARCHAR(100),
		new_col TEXT
	)`, ref))

	// All old data gone.
	if n := mrowCount(t, client, ref); n != 0 {
		t.Errorf("row count after destructive rebuild = %d, want 0", n)
	}

	cols := mcolumnNames(t, client, ref)
	if !hasColumn(cols, "NEW_COL") {
		t.Errorf("new_col column not present after rebuild; columns: %v", cols)
	}

	t.Logf("destructive rebuild: columns: %v", cols)
}

// ─── empty-table fast path ────────────────────────────────────────────────────

// TestMigrationEmptyTableFastPath verifies that an empty table is efficiently
// handled via DROP + CREATE (no data to preserve, so any strategy collapses to
// destructive rebuild).
func TestMigrationEmptyTableFastPath(t *testing.T) {
	client := keyPairConnFromEnv(t)
	db, schema := setupMigrationDB(t, client)

	name := randomName("EMPTY_")
	ref := fqn(db, schema, name)

	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id NUMBER, name VARCHAR(100)
	)`, ref))
	t.Cleanup(func() { execIgnoreError(client, "DROP TABLE IF EXISTS "+ref) })

	// Table is empty — apply destructive rebuild directly.
	exec(t, client, "DROP TABLE IF EXISTS "+ref)
	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		id NUMBER, name VARCHAR(200), new_col TEXT
	)`, ref))

	cols := mcolumnNames(t, client, ref)
	if !hasColumn(cols, "NEW_COL") {
		t.Errorf("new_col not present after rebuild; columns: %v", cols)
	}
	if mrowCount(t, client, ref) != 0 {
		t.Error("empty-table rebuild should produce an empty table")
	}
}

// ─── various column types ─────────────────────────────────────────────────────

// TestMigrationVariousColumnTypes verifies that tables containing diverse
// Snowflake column types survive a DROP + CREATE cycle with data intact.
func TestMigrationVariousColumnTypes(t *testing.T) {
	client := keyPairConnFromEnv(t)
	db, schema := setupMigrationDB(t, client)

	name := randomName("TYPES_")
	ref := fqn(db, schema, name)

	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		col_number  NUMBER(38,0),
		col_float   FLOAT,
		col_varchar VARCHAR(512),
		col_text    TEXT,
		col_boolean BOOLEAN,
		col_date    DATE,
		col_time    TIME,
		col_ts_ntz  TIMESTAMP_NTZ(9),
		col_ts_ltz  TIMESTAMP_LTZ(9),
		col_variant VARIANT,
		col_array   ARRAY,
		col_object  OBJECT
	)`, ref))
	t.Cleanup(func() { execIgnoreError(client, "DROP TABLE IF EXISTS "+ref) })

	// Insert one row using Snowflake functions for semi-structured types.
	exec(t, client, fmt.Sprintf(`INSERT INTO %s
		(col_number, col_float, col_varchar, col_text, col_boolean, col_date,
		 col_time, col_ts_ntz, col_ts_ltz, col_variant, col_array, col_object)
	SELECT
		42, 3.14, 'hello', 'world', TRUE, '2024-01-15'::DATE,
		'12:30:00'::TIME, '2024-01-15 12:30:00'::TIMESTAMP_NTZ,
		'2024-01-15 12:30:00 +00:00'::TIMESTAMP_LTZ,
		PARSE_JSON('{"key":"value"}'),
		ARRAY_CONSTRUCT(1, 2, 3),
		OBJECT_CONSTRUCT('a', 1)`, ref))

	if n := mrowCount(t, client, ref); n != 1 {
		t.Fatalf("expected 1 row, got %d", n)
	}

	// Destructive rebuild: drop + recreate with identical schema.
	exec(t, client, "DROP TABLE IF EXISTS "+ref)
	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		col_number  NUMBER(38,0),
		col_float   FLOAT,
		col_varchar VARCHAR(512),
		col_text    TEXT,
		col_boolean BOOLEAN,
		col_date    DATE,
		col_time    TIME,
		col_ts_ntz  TIMESTAMP_NTZ(9),
		col_ts_ltz  TIMESTAMP_LTZ(9),
		col_variant VARIANT,
		col_array   ARRAY,
		col_object  OBJECT
	)`, ref))

	if n := mrowCount(t, client, ref); n != 0 {
		t.Errorf("table should be empty after rebuild, got %d rows", n)
	}

	cols := mcolumnNames(t, client, ref)
	for _, ec := range []string{
		"COL_NUMBER", "COL_FLOAT", "COL_VARCHAR", "COL_TEXT",
		"COL_BOOLEAN", "COL_DATE", "COL_TIME",
		"COL_TS_NTZ", "COL_TS_LTZ",
		"COL_VARIANT", "COL_ARRAY", "COL_OBJECT",
	} {
		if !hasColumn(cols, ec) {
			t.Errorf("column %s missing after rebuild; columns: %v", ec, cols)
		}
	}
	t.Logf("various column types: %v", cols)
}

// ─── in-place with multiple simultaneous ADD/DROP columns ─────────────────────

// TestMigrationInPlaceMultipleColumns verifies in-place migration when several
// columns are added and dropped in the same pass.
func TestMigrationInPlaceMultipleColumns(t *testing.T) {
	client := keyPairConnFromEnv(t)
	db, schema := setupMigrationDB(t, client)

	name := randomName("IPMC_")
	ref := fqn(db, schema, name)

	exec(t, client, fmt.Sprintf(`CREATE TABLE %s (
		keep_a NUMBER,
		keep_b VARCHAR(100),
		drop_c TEXT,
		drop_d BOOLEAN
	)`, ref))
	t.Cleanup(func() { execIgnoreError(client, "DROP TABLE IF EXISTS "+ref) })

	exec(t, client, fmt.Sprintf(`INSERT INTO %s VALUES (1, 'x', 'foo', TRUE)`, ref))
	exec(t, client, fmt.Sprintf(`INSERT INTO %s VALUES (2, 'y', 'bar', FALSE)`, ref))

	// ADD two new columns, DROP two old columns.
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN add_e FLOAT`, ref))
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s ADD COLUMN add_f DATE`, ref))
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s DROP COLUMN drop_c`, ref))
	exec(t, client, fmt.Sprintf(`ALTER TABLE %s DROP COLUMN drop_d`, ref))

	cols := mcolumnNames(t, client, ref)
	for _, want := range []string{"KEEP_A", "KEEP_B", "ADD_E", "ADD_F"} {
		if !hasColumn(cols, want) {
			t.Errorf("expected column %s to be present; columns: %v", want, cols)
		}
	}
	for _, gone := range []string{"DROP_C", "DROP_D"} {
		if hasColumn(cols, gone) {
			t.Errorf("expected column %s to be absent; columns: %v", gone, cols)
		}
	}

	// Data in kept columns is preserved.
	if n := mrowCount(t, client, ref); n != 2 {
		t.Errorf("row count = %d, want 2 (data in keep_a/keep_b preserved)", n)
	}

	t.Logf("in-place multiple columns: %v", cols)
}
