// SPDX-License-Identifier: GPL-3.0-or-later

//go:build integration

// Package integration_test contains end-to-end tests that require a live
// Snowflake account.  They are intentionally excluded from regular "go test"
// runs and must be opted into with the integration build tag:
//
//	go test -v -tags integration -timeout 5m ./internal/integration/
//
// # Required environment variables
//
//	SNOWFLAKE_ACCOUNT      Account identifier, e.g. myorg-myaccount
//	SNOWFLAKE_USER         Login name
//	SNOWFLAKE_PRIVATE_KEY  PEM-encoded PKCS#8 RSA private key (unencrypted)
//	SNOWFLAKE_WAREHOUSE    Warehouse to use, e.g. COMPUTE_WH
//
// # Permissions required
//
// These tests are designed to run with a freshly created Snowflake user that
// has no custom grants — only the PUBLIC role and whatever the account grants
// by default.  No CREATE DATABASE or elevated privileges are required.
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

// ─── helpers ─────────────────────────────────────────────────────────────────

// keyPairConnFromEnv builds a Snowflake client using key-pair (JWT) auth.
// The test is skipped — not failed — when any required variable is absent,
// so the suite degrades gracefully in CI environments without Snowflake access.
func keyPairConnFromEnv(t *testing.T) *snowflake.Client {
	t.Helper()

	required := []string{
		"SNOWFLAKE_ACCOUNT",
		"SNOWFLAKE_USER",
		"SNOWFLAKE_PRIVATE_KEY",
		"SNOWFLAKE_WAREHOUSE",
	}
	for _, key := range required {
		if os.Getenv(key) == "" {
			t.Skipf("integration test skipped: %s is not set", key)
		}
	}

	// Write PEM content to a temp file.  ConnectParams.PrivateKeyPath expects
	// a file path, not raw PEM bytes.
	pemContent := strings.TrimSpace(os.Getenv("SNOWFLAKE_PRIVATE_KEY"))
	tmpFile, err := os.CreateTemp(t.TempDir(), "snowflake-key-*.pem")
	if err != nil {
		t.Fatalf("create temp key file: %v", err)
	}
	if _, err := tmpFile.WriteString(pemContent); err != nil {
		t.Fatalf("write temp key file: %v", err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatalf("close temp key file: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := snowflake.NewClient(ctx, snowflake.ConnectParams{
		Account:        os.Getenv("SNOWFLAKE_ACCOUNT"),
		User:           os.Getenv("SNOWFLAKE_USER"),
		Role:           os.Getenv("SNOWFLAKE_ROLE"), // optional; empty = account default
		Warehouse:      os.Getenv("SNOWFLAKE_WAREHOUSE"),
		Authenticator:  "snowflake_jwt",
		PrivateKeyPath: tmpFile.Name(),
	})
	if err != nil {
		t.Fatalf("connect to Snowflake: %v", err)
	}
	t.Cleanup(func() { client.Close() })
	return client
}

// mustQuery executes a single SQL statement and fails the test on any error.
func mustQuery(t *testing.T, client *snowflake.Client, query string) *snowflake.QueryResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	result, err := client.Execute(ctx, query)
	if err != nil {
		t.Fatalf("query failed: %v\n  SQL: %s", err, query)
	}
	return result
}

// ─── connectivity ─────────────────────────────────────────────────────────────

// TestBasicConnectivity verifies that the client can connect and run a trivial
// SELECT.  This is the minimal smoke test — if it fails, no other tests will
// pass either.
func TestBasicConnectivity(t *testing.T) {
	client := keyPairConnFromEnv(t)

	result := mustQuery(t, client, "SELECT 1 AS num")

	if len(result.Columns) != 1 {
		t.Fatalf("want 1 column, got %d", len(result.Columns))
	}
	if len(result.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(result.Rows))
	}
}

// ─── session metadata ─────────────────────────────────────────────────────────

// TestSessionFunctions verifies that session context functions return
// non-empty strings.  These functions are always available regardless of role
// or privilege level.
func TestSessionFunctions(t *testing.T) {
	client := keyPairConnFromEnv(t)

	cases := []struct {
		name  string
		query string
	}{
		{"CURRENT_USER", "SELECT CURRENT_USER()"},
		{"CURRENT_ROLE", "SELECT CURRENT_ROLE()"},
		{"CURRENT_ACCOUNT", "SELECT CURRENT_ACCOUNT()"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result := mustQuery(t, client, tc.query)
			if len(result.Rows) != 1 || len(result.Rows[0]) != 1 {
				t.Fatalf("expected 1 row, 1 column")
			}
			val, ok := result.Rows[0][0].(string)
			if !ok {
				t.Fatalf("expected string value, got %T", result.Rows[0][0])
			}
			if strings.TrimSpace(val) == "" {
				t.Errorf("%s returned empty string", tc.name)
			}
			t.Logf("%s = %q", tc.name, val)
		})
	}
}

// TestCurrentUserMatchesEnv verifies that the connected user matches the
// SNOWFLAKE_USER environment variable (case-insensitive).
func TestCurrentUserMatchesEnv(t *testing.T) {
	client := keyPairConnFromEnv(t)

	result := mustQuery(t, client, "SELECT CURRENT_USER()")
	if len(result.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(result.Rows))
	}
	got, _ := result.Rows[0][0].(string)
	want := strings.ToUpper(strings.TrimSpace(os.Getenv("SNOWFLAKE_USER")))
	if strings.ToUpper(strings.TrimSpace(got)) != want {
		t.Errorf("CURRENT_USER() = %q, want %q", got, want)
	}
}

// TestWarehouseActive verifies that CURRENT_WAREHOUSE() returns the warehouse
// specified in SNOWFLAKE_WAREHOUSE (case-insensitive).
func TestWarehouseActive(t *testing.T) {
	client := keyPairConnFromEnv(t)

	result := mustQuery(t, client, "SELECT CURRENT_WAREHOUSE()")
	if len(result.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(result.Rows))
	}
	got, _ := result.Rows[0][0].(string)
	want := strings.ToUpper(strings.TrimSpace(os.Getenv("SNOWFLAKE_WAREHOUSE")))
	if strings.ToUpper(strings.TrimSpace(got)) != want {
		t.Errorf("CURRENT_WAREHOUSE() = %q, want %q", got, want)
	}
}

// ─── result shape ─────────────────────────────────────────────────────────────

// TestMultiRowResults verifies that a multi-row SELECT returns all rows with
// correct column names.
func TestMultiRowResults(t *testing.T) {
	client := keyPairConnFromEnv(t)

	result := mustQuery(t, client,
		"SELECT * FROM VALUES (1, 'alpha'), (2, 'beta'), (3, 'gamma') AS t(id, label)")

	if len(result.Columns) != 2 {
		t.Fatalf("want 2 columns, got %d: %v", len(result.Columns), result.Columns)
	}
	if len(result.Rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(result.Rows))
	}

	// Verify column names are non-empty.
	for i, col := range result.Columns {
		if strings.TrimSpace(col) == "" {
			t.Errorf("column[%d] name is empty", i)
		}
	}

	t.Logf("columns: %v", result.Columns)
	for i, row := range result.Rows {
		t.Logf("row[%d]: %v", i, row)
	}
}

// TestNullHandling verifies that NULL SQL values are returned as nil in Go.
func TestNullHandling(t *testing.T) {
	client := keyPairConnFromEnv(t)

	result := mustQuery(t, client,
		"SELECT NULL::VARCHAR AS null_val, 'present'::VARCHAR AS non_null_val")

	if len(result.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(result.Rows))
	}
	row := result.Rows[0]
	if len(row) != 2 {
		t.Fatalf("want 2 columns in row, got %d", len(row))
	}

	if row[0] != nil {
		t.Errorf("null_val: want nil, got %v (%T)", row[0], row[0])
	}
	nonNull, ok := row[1].(string)
	if !ok || nonNull != "present" {
		t.Errorf("non_null_val: want \"present\", got %v (%T)", row[1], row[1])
	}
}

// TestMultipleNullableColumns verifies NULL handling across several columns of
// different types in the same row.
func TestMultipleNullableColumns(t *testing.T) {
	client := keyPairConnFromEnv(t)

	result := mustQuery(t, client,
		"SELECT NULL::NUMBER AS n, NULL::BOOLEAN AS b, NULL::TIMESTAMP_NTZ AS ts, 1 AS present")

	if len(result.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(result.Rows))
	}
	row := result.Rows[0]
	if len(row) < 4 {
		t.Fatalf("want at least 4 columns, got %d", len(row))
	}
	for i := 0; i < 3; i++ {
		if row[i] != nil {
			t.Errorf("column[%d]: expected nil, got %v (%T)", i, row[i], row[i])
		}
	}
}

// ─── error handling ───────────────────────────────────────────────────────────

// TestInvalidSQLReturnsError verifies that syntactically invalid SQL causes
// Execute to return an error (not a panic or an empty result).
func TestInvalidSQLReturnsError(t *testing.T) {
	client := keyPairConnFromEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Execute(ctx, "SELECT FROM WHERE")
	if err == nil {
		t.Error("expected an error for invalid SQL, got nil")
	}
	t.Logf("error (expected): %v", err)
}

// TestUnknownTableReturnsError verifies that a reference to a non-existent
// table produces an error.
func TestUnknownTableReturnsError(t *testing.T) {
	client := keyPairConnFromEnv(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	_, err := client.Execute(ctx,
		"SELECT * FROM THIS_DATABASE_DOES_NOT_EXIST_THAW_TEST.PUBLIC.NO_TABLE")
	if err == nil {
		t.Error("expected an error for unknown table, got nil")
	}
	t.Logf("error (expected): %v", err)
}

// TestContextAlreadyExpired verifies that Execute returns an error immediately
// when the caller provides an already-cancelled context.
func TestContextAlreadyExpired(t *testing.T) {
	client := keyPairConnFromEnv(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before calling Execute

	_, err := client.Execute(ctx, "SELECT 1")
	if err == nil {
		t.Error("expected an error for cancelled context, got nil")
	}
	t.Logf("error (expected): %v", err)
}

// ─── arithmetic and expressions ──────────────────────────────────────────────

// TestArithmeticExpressions verifies that the driver correctly handles simple
// arithmetic results.
func TestArithmeticExpressions(t *testing.T) {
	client := keyPairConnFromEnv(t)

	cases := []struct {
		expr string
		desc string
	}{
		{"SELECT 2 + 2", "addition"},
		{"SELECT 100 - 1", "subtraction"},
		{"SELECT 6 * 7", "multiplication"},
		{"SELECT 10 / 4", "division"},
	}

	for _, tc := range cases {
		t.Run(tc.desc, func(t *testing.T) {
			result := mustQuery(t, client, tc.expr)
			if len(result.Rows) != 1 || len(result.Rows[0]) != 1 {
				t.Fatalf("want 1x1 result, got %d rows, %d cols",
					len(result.Rows), func() int {
						if len(result.Rows) > 0 {
							return len(result.Rows[0])
						}
						return 0
					}())
			}
			t.Logf("%s = %v", tc.expr, result.Rows[0][0])
		})
	}
}

// ─── string functions ─────────────────────────────────────────────────────────

// TestStringFunctions verifies that Snowflake built-in string functions return
// expected results.
func TestStringFunctions(t *testing.T) {
	client := keyPairConnFromEnv(t)

	result := mustQuery(t, client,
		"SELECT UPPER('hello') AS u, LOWER('WORLD') AS l, LENGTH('thaw') AS n")

	if len(result.Rows) != 1 {
		t.Fatalf("want 1 row, got %d", len(result.Rows))
	}
	row := result.Rows[0]
	if len(row) != 3 {
		t.Fatalf("want 3 columns, got %d", len(row))
	}

	upper, _ := row[0].(string)
	lower, _ := row[1].(string)
	if upper != "HELLO" {
		t.Errorf("UPPER('hello') = %q, want %q", upper, "HELLO")
	}
	if lower != "world" {
		t.Errorf("LOWER('WORLD') = %q, want %q", lower, "world")
	}
	t.Logf("UPPER='%s', LOWER='%s', LENGTH=%v", upper, lower, row[2])
}

// ─── large result set ─────────────────────────────────────────────────────────

// TestLargeResultSet verifies that the driver correctly returns a result set
// with many rows.  We use a generator query (no table needed).
func TestLargeResultSet(t *testing.T) {
	client := keyPairConnFromEnv(t)

	const rowCount = 1000
	result := mustQuery(t, client,
		fmt.Sprintf("SELECT SEQ4() AS n FROM TABLE(GENERATOR(ROWCOUNT => %d))", rowCount))

	if len(result.Rows) != rowCount {
		t.Errorf("want %d rows, got %d", rowCount, len(result.Rows))
	}
}
