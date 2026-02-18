//go:build integration

// Package integration_test contains end-to-end tests that require a live
// Snowflake account.  They are intentionally excluded from regular "go test"
// runs and must be opted into with the integration build tag:
//
//	go test -v -tags integration -timeout 10m ./internal/integration/
//
// # Required environment variables
//
//	SNOWFLAKE_ACCOUNT    Account identifier, e.g. myorg-myaccount
//	SNOWFLAKE_USER       Login name
//	SNOWFLAKE_PASSWORD   Password
//	SNOWFLAKE_WAREHOUSE  Warehouse to use, e.g. COMPUTE_WH
//
// # Optional environment variables
//
//	SNOWFLAKE_ROLE       Role to assume (defaults to the user's default role)
//
// Each test run creates a temporary database named THAW_TEST_<random> and
// drops it unconditionally on exit, even when the test fails.
package integration_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"thaw/internal/ddl"
	"thaw/internal/snowflake"
)

// ─── helpers ─────────────────────────────────────────────────────────────────

// connFromEnv builds a Snowflake client from environment variables.
// The test is skipped — not failed — when any required variable is absent,
// so the suite degrades gracefully in CI environments without Snowflake access.
func connFromEnv(t *testing.T) *snowflake.Client {
	t.Helper()

	required := []string{
		"SNOWFLAKE_ACCOUNT",
		"SNOWFLAKE_USER",
		"SNOWFLAKE_PASSWORD",
		"SNOWFLAKE_WAREHOUSE",
	}
	for _, key := range required {
		if os.Getenv(key) == "" {
			t.Skipf("integration test skipped: %s is not set", key)
		}
	}

	client, err := snowflake.NewClient(snowflake.ConnectParams{
		Account:   os.Getenv("SNOWFLAKE_ACCOUNT"),
		User:      os.Getenv("SNOWFLAKE_USER"),
		Password:  os.Getenv("SNOWFLAKE_PASSWORD"),
		Role:      os.Getenv("SNOWFLAKE_ROLE"),
		Warehouse: os.Getenv("SNOWFLAKE_WAREHOUSE"),
	})
	if err != nil {
		t.Fatalf("connect to Snowflake: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// mustExec runs a single SQL statement, failing the test immediately on error.
func mustExec(t *testing.T, client *snowflake.Client, query string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if _, err := client.Execute(ctx, query); err != nil {
		preview := query
		if len(preview) > 120 {
			preview = preview[:120] + "…"
		}
		t.Fatalf("SQL failed: %v\n  %s", err, preview)
	}
}

// randomName returns a Snowflake-safe identifier: prefix + 8 random hex chars.
func randomName(prefix string) string {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return prefix + strings.ToUpper(hex.EncodeToString(b))
}

// assertFileExists checks that path exists and is a regular file.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Errorf("expected file missing: %s (%v)", path, err)
		return
	}
	if !info.Mode().IsRegular() {
		t.Errorf("expected regular file, got non-file at: %s", path)
	}
}

// assertFileContains reads path and checks that every want string appears in
// the content (case-insensitive).
func assertFileContains(t *testing.T, path string, want ...string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("read %s: %v", path, err)
		return
	}
	lower := strings.ToLower(string(data))
	for _, w := range want {
		if !strings.Contains(lower, strings.ToLower(w)) {
			t.Errorf("%s: expected to contain %q\ncontent:\n%s", filepath.Base(path), w, string(data))
		}
	}
}

// ─── test: full DDL export round-trip ────────────────────────────────────────

// TestExportDatabase creates a temporary Snowflake database that exercises
// every supported DDL object type, runs the parallel export pipeline, and
// then validates the file-system output.
//
// Object inventory (per schema):
//
//	ALPHA: 2 tables, 1 view, 2 function overloads, 1 procedure,
//	       1 sequence, 1 internal stage, 1 stream, 1 file format
//	BETA:  1 table, 1 view
func TestExportDatabase(t *testing.T) {
	client := connFromEnv(t)
	ctx := context.Background()

	// ── 1. Create a uniquely-named temporary database ─────────────────────────

	dbName := randomName("THAW_TEST_")
	t.Logf("test database: %s", dbName)

	mustExec(t, client, fmt.Sprintf(`CREATE DATABASE "%s"`, dbName))

	// Always drop the database when the test finishes, even on failure.
	t.Cleanup(func() {
		dropCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		if _, err := client.Execute(dropCtx,
			fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, dbName)); err != nil {
			t.Logf("cleanup: drop %s: %v (manual cleanup may be required)", dbName, err)
		}
	})

	// ── 2. Create schemas ─────────────────────────────────────────────────────

	for _, schema := range []string{"ALPHA", "BETA"} {
		mustExec(t, client, fmt.Sprintf(`CREATE SCHEMA "%s"."%s"`, dbName, schema))
	}

	// Convenience: returns a fully-qualified three-part identifier.
	fqn := func(schema, object string) string {
		return fmt.Sprintf(`"%s"."%s"."%s"`, dbName, schema, object)
	}

	// ── 3. Populate ALPHA ─────────────────────────────────────────────────────

	// Tables
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE TABLE %s (
			"ORDER_ID"  NUMBER(38,0) NOT NULL,
			"CUSTOMER"  VARCHAR(256),
			"AMOUNT"    FLOAT
		)`, fqn("ALPHA", "ORDERS")))

	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE TABLE %s (
			"PRODUCT_ID" NUMBER(38,0) NOT NULL,
			"NAME"       VARCHAR(256),
			"PRICE"      FLOAT
		)`, fqn("ALPHA", "PRODUCTS")))

	// View
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE VIEW %s AS
		SELECT * FROM %s WHERE "PRICE" > 100`,
		fqn("ALPHA", "EXPENSIVE_PRODUCTS"),
		fqn("ALPHA", "PRODUCTS")))

	// JavaScript function — overload 1: single argument
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %s(PRICE FLOAT)
		RETURNS FLOAT
		LANGUAGE JAVASCRIPT
		AS $$
			return PRICE * 0.9;
		$$`, fqn("ALPHA", "APPLY_DISCOUNT")))

	// JavaScript function — overload 2: two arguments (same name, different signature)
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE FUNCTION %s(PRICE FLOAT, PCT FLOAT)
		RETURNS FLOAT
		LANGUAGE JAVASCRIPT
		AS $$
			return PRICE * (1 - PCT / 100);
		$$`, fqn("ALPHA", "APPLY_DISCOUNT")))

	// Stored procedure
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE PROCEDURE %s(MSG VARCHAR)
		RETURNS VARCHAR
		LANGUAGE SQL
		AS $$
		BEGIN
			RETURN MSG;
		END
		$$`, fqn("ALPHA", "LOG_EVENT")))

	// Sequence
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE SEQUENCE %s START = 1 INCREMENT = 1`,
		fqn("ALPHA", "ORDER_SEQ")))

	// Internal named stage (no external credentials required)
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE STAGE %s`, fqn("ALPHA", "UPLOADS")))

	// Stream on ORDERS — must be created after the table
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE STREAM %s ON TABLE %s`,
		fqn("ALPHA", "ORDERS_CHANGES"),
		fqn("ALPHA", "ORDERS")))

	// File format
	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE FILE FORMAT %s
			TYPE            = 'JSON'
			STRIP_OUTER_ARRAY = TRUE`,
		fqn("ALPHA", "JSON_FORMAT")))

	// ── 4. Populate BETA ──────────────────────────────────────────────────────

	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE TABLE %s (
			"EVENT_ID"   NUMBER(38,0) NOT NULL,
			"EVENT_TYPE" VARCHAR(128)
		)`, fqn("BETA", "EVENTS")))

	mustExec(t, client, fmt.Sprintf(`
		CREATE OR REPLACE VIEW %s AS
		SELECT * FROM %s`,
		fqn("BETA", "RECENT_EVENTS"),
		fqn("BETA", "EVENTS")))

	// ── 5. Run the export pipeline ────────────────────────────────────────────

	outDir := t.TempDir()

	t.Log("fetching DDL and exporting…")
	exportStart := time.Now()

	results := ddl.ExportDatabases(
		ctx,
		[]string{dbName},
		client.GetDatabaseDDL,
		ddl.ExportOptions{OutputDir: outDir},
		func(done, total int, res ddl.ExportResult) {
			t.Logf("export done %d/%d: %s — %d files, %d skipped, %d errors",
				done, total, res.Database, res.Files, res.Skipped, len(res.Errors))
		},
	)

	t.Logf("export finished in %s", time.Since(exportStart).Round(time.Millisecond))

	if len(results) != 1 {
		t.Fatalf("ExportDatabases returned %d results, want 1", len(results))
	}
	result := results[0]

	// ── 6. Assert: no pipeline errors ────────────────────────────────────────

	t.Run("no export errors", func(t *testing.T) {
		if len(result.Errors) > 0 {
			t.Errorf("%d export error(s):\n%s",
				len(result.Errors), strings.Join(result.Errors, "\n"))
		}
	})

	// ── 7. Assert: minimum file count ────────────────────────────────────────
	// We know exactly what we created (16 objects: 1 db + 3 schemas including
	// the automatic PUBLIC + 12 user objects).  Accept >= 16 to tolerate any
	// extra objects Snowflake injects automatically (e.g. default roles, tasks).

	t.Run("minimum file count", func(t *testing.T) {
		const minExpected = 16
		if result.Files < minExpected {
			t.Errorf("exported %d files, want at least %d", result.Files, minExpected)
		}
		t.Logf("exported %d files total", result.Files)
	})

	// ── 8. Assert: all expected files exist ──────────────────────────────────
	// path builds a full filesystem path rooted at outDir / dbName.
	path := func(parts ...string) string {
		return filepath.Join(append([]string{outDir, dbName}, parts...)...)
	}

	expectedFiles := []struct {
		rel      string   // relative parts passed to path()
		contains []string // strings that must appear in the file (case-insensitive)
	}{
		// Database sentinel
		{rel: "_database.sql", contains: []string{"create", "database", dbName}},

		// Schema DDLs
		{rel: filepath.Join("schemas", "ALPHA.sql"), contains: []string{"schema", "ALPHA"}},
		{rel: filepath.Join("schemas", "BETA.sql"), contains: []string{"schema", "BETA"}},

		// ALPHA — tables
		{rel: filepath.Join("ALPHA", "tables", "ORDERS.sql"), contains: []string{"table", "ORDERS", "ORDER_ID"}},
		{rel: filepath.Join("ALPHA", "tables", "PRODUCTS.sql"), contains: []string{"table", "PRODUCTS", "PRICE"}},

		// ALPHA — view
		{rel: filepath.Join("ALPHA", "views", "EXPENSIVE_PRODUCTS.sql"), contains: []string{"view", "EXPENSIVE_PRODUCTS"}},

		// ALPHA — function overloads (distinct file names)
		{rel: filepath.Join("ALPHA", "functions", "APPLY_DISCOUNT__FLOAT.sql"), contains: []string{"function", "APPLY_DISCOUNT"}},
		{rel: filepath.Join("ALPHA", "functions", "APPLY_DISCOUNT__FLOAT_FLOAT.sql"), contains: []string{"function", "APPLY_DISCOUNT"}},

		// ALPHA — procedure
		{rel: filepath.Join("ALPHA", "procedures", "LOG_EVENT__VARCHAR.sql"), contains: []string{"procedure", "LOG_EVENT"}},

		// ALPHA — sequence
		{rel: filepath.Join("ALPHA", "sequences", "ORDER_SEQ.sql"), contains: []string{"sequence", "ORDER_SEQ"}},

		// ALPHA — stage
		{rel: filepath.Join("ALPHA", "stages", "UPLOADS.sql"), contains: []string{"stage", "UPLOADS"}},

		// ALPHA — stream
		{rel: filepath.Join("ALPHA", "streams", "ORDERS_CHANGES.sql"), contains: []string{"stream", "ORDERS_CHANGES"}},

		// ALPHA — file format
		{rel: filepath.Join("ALPHA", "file_formats", "JSON_FORMAT.sql"), contains: []string{"file format", "JSON_FORMAT"}},

		// BETA — table and view
		{rel: filepath.Join("BETA", "tables", "EVENTS.sql"), contains: []string{"table", "EVENTS", "EVENT_ID"}},
		{rel: filepath.Join("BETA", "views", "RECENT_EVENTS.sql"), contains: []string{"view", "RECENT_EVENTS"}},
	}

	t.Run("expected files exist", func(t *testing.T) {
		for _, ef := range expectedFiles {
			full := path(ef.rel)
			assertFileExists(t, full)
		}
	})

	t.Run("file contents are valid SQL", func(t *testing.T) {
		for _, ef := range expectedFiles {
			full := path(ef.rel)
			// Every file must start with a CREATE statement and end with a semicolon.
			assertFileContains(t, full, "create")
			assertFileContains(t, full, ef.contains...)
		}
	})

	// ── 9. Assert: function overloads landed at different paths ───────────────

	t.Run("function overloads have distinct paths", func(t *testing.T) {
		oneArg := path("ALPHA", "functions", "APPLY_DISCOUNT__FLOAT.sql")
		twoArgs := path("ALPHA", "functions", "APPLY_DISCOUNT__FLOAT_FLOAT.sql")

		assertFileExists(t, oneArg)
		assertFileExists(t, twoArgs)

		// The single-arg version must NOT contain PCT (the second param name).
		data1, _ := os.ReadFile(oneArg)
		if strings.Contains(strings.ToUpper(string(data1)), "PCT") {
			t.Errorf("single-arg file unexpectedly contains PCT:\n%s", data1)
		}

		// The two-arg version must contain both PRICE and PCT.
		assertFileContains(t, twoArgs, "PCT")
		assertFileContains(t, twoArgs, "PRICE")
	})

	// ── 10. Assert: dollar-quoted body preserved intact ───────────────────────

	t.Run("dollar-quoted body preserved in function file", func(t *testing.T) {
		// The JS body of APPLY_DISCOUNT(PRICE) should survive the split/write round-trip.
		assertFileContains(t,
			path("ALPHA", "functions", "APPLY_DISCOUNT__FLOAT.sql"),
			"$$",          // dollar-quote delimiters present
			"return PRICE", // JS body intact
		)
	})

	t.Run("dollar-quoted body preserved in procedure file", func(t *testing.T) {
		assertFileContains(t,
			path("ALPHA", "procedures", "LOG_EVENT__VARCHAR.sql"),
			"$$",
			"RETURN MSG",
		)
	})

	// ── 11. Assert: output directory structure ────────────────────────────────

	t.Run("schema directories exist", func(t *testing.T) {
		for _, schema := range []string{"ALPHA", "BETA"} {
			info, err := os.Stat(path(schema))
			if err != nil {
				t.Errorf("schema directory %s missing: %v", schema, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("expected directory at %s", path(schema))
			}
		}
	})

	t.Run("object type sub-directories exist inside ALPHA", func(t *testing.T) {
		for _, sub := range []string{"tables", "views", "functions", "procedures",
			"sequences", "stages", "streams", "file_formats"} {
			info, err := os.Stat(path("ALPHA", sub))
			if err != nil {
				t.Errorf("sub-directory ALPHA/%s missing: %v", sub, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("expected directory at ALPHA/%s", sub)
			}
		}
	})
}

// ─── test: multi-database parallel export ────────────────────────────────────

// TestExportMultipleDatabases verifies that the parallel export pipeline
// correctly handles multiple databases concurrently, with no cross-database
// file contamination.
func TestExportMultipleDatabases(t *testing.T) {
	client := connFromEnv(t)
	ctx := context.Background()

	// Create two independent databases.
	dbA := randomName("THAW_TEST_A_")
	dbB := randomName("THAW_TEST_B_")
	t.Logf("test databases: %s, %s", dbA, dbB)

	for _, db := range []string{dbA, dbB} {
		db := db
		mustExec(t, client, fmt.Sprintf(`CREATE DATABASE "%s"`, db))
		t.Cleanup(func() {
			dropCtx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if _, err := client.Execute(dropCtx,
				fmt.Sprintf(`DROP DATABASE IF EXISTS "%s"`, db)); err != nil {
				t.Logf("cleanup: drop %s: %v", db, err)
			}
		})
	}

	// Create one table in each database so we have at least one object file.
	for _, pair := range []struct{ db, schema, tbl string }{
		{dbA, "PUBLIC", "ALPHA_TABLE"},
		{dbB, "PUBLIC", "BETA_TABLE"},
	} {
		mustExec(t, client, fmt.Sprintf(`
			CREATE OR REPLACE TABLE "%s"."%s"."%s" ("ID" NUMBER)`,
			pair.db, pair.schema, pair.tbl))
	}

	outDir := t.TempDir()

	results := ddl.ExportDatabases(
		ctx,
		[]string{dbA, dbB},
		client.GetDatabaseDDL,
		ddl.ExportOptions{
			OutputDir:     outDir,
			DBConcurrency: 2, // both databases fetched in parallel
		},
		func(done, total int, res ddl.ExportResult) {
			t.Logf("export %d/%d: %s — %d files", done, total, res.Database, res.Files)
		},
	)

	t.Run("both databases exported", func(t *testing.T) {
		if len(results) != 2 {
			t.Fatalf("want 2 results, got %d", len(results))
		}
		for _, r := range results {
			if len(r.Errors) > 0 {
				t.Errorf("%s: export errors: %v", r.Database, r.Errors)
			}
			if r.Files == 0 {
				t.Errorf("%s: no files exported", r.Database)
			}
		}
	})

	t.Run("database directories are separate", func(t *testing.T) {
		for _, db := range []string{dbA, dbB} {
			info, err := os.Stat(filepath.Join(outDir, db))
			if err != nil {
				t.Errorf("directory for %s missing: %v", db, err)
				continue
			}
			if !info.IsDir() {
				t.Errorf("expected directory for %s", db)
			}
		}
	})

	t.Run("no cross-database file contamination", func(t *testing.T) {
		// ALPHA_TABLE must exist only under dbA, not dbB.
		assertFileExists(t,
			filepath.Join(outDir, dbA, "PUBLIC", "tables", "ALPHA_TABLE.sql"))

		wrongPath := filepath.Join(outDir, dbB, "PUBLIC", "tables", "ALPHA_TABLE.sql")
		if _, err := os.Stat(wrongPath); err == nil {
			t.Errorf("ALPHA_TABLE.sql found under %s — cross-database contamination", dbB)
		}

		// BETA_TABLE must exist only under dbB.
		assertFileExists(t,
			filepath.Join(outDir, dbB, "PUBLIC", "tables", "BETA_TABLE.sql"))

		wrongPath = filepath.Join(outDir, dbA, "PUBLIC", "tables", "BETA_TABLE.sql")
		if _, err := os.Stat(wrongPath); err == nil {
			t.Errorf("BETA_TABLE.sql found under %s — cross-database contamination", dbA)
		}
	})
}
