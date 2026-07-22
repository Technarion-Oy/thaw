// SPDX-License-Identifier: GPL-3.0-or-later

package queryhistory

import (
	"context"
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildQueryHistorySQL(t *testing.T) {
	tests := []struct {
		name         string
		filterType   string
		sessionID    string
		userName     string
		warehouse    string
		wantFunc     string
		wantContains []string
	}{
		{
			name:         "session filter",
			filterType:   "session",
			sessionID:    "12345",
			wantFunc:     "QUERY_HISTORY_BY_SESSION",
			wantContains: []string{"SESSION_ID => 12345"},
		},
		{
			name:         "user filter escapes quotes",
			filterType:   "user",
			userName:     "O'Brien",
			wantFunc:     "QUERY_HISTORY_BY_USER",
			wantContains: []string{"USER_NAME => 'O''Brien'"},
		},
		{
			name:         "warehouse filter",
			filterType:   "warehouse",
			warehouse:    "WH1",
			wantFunc:     "QUERY_HISTORY_BY_WAREHOUSE",
			wantContains: []string{"WAREHOUSE_NAME => 'WH1'"},
		},
		{
			name:         "all filter",
			filterType:   "all",
			wantFunc:     "QUERY_HISTORY",
			wantContains: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sql := buildQueryHistorySql(tt.filterType, tt.sessionID, tt.userName, tt.warehouse, "", "", 100, true, QueryHistoryFilters{})
			if !strings.Contains(sql, tt.wantFunc) {
				t.Errorf("expected func %q in SQL:\n%s", tt.wantFunc, sql)
			}
			if !strings.Contains(sql, "RESULT_LIMIT => 100") {
				t.Errorf("expected RESULT_LIMIT in SQL:\n%s", sql)
			}
			if !strings.Contains(sql, "SESSION_ID,") {
				t.Errorf("expected SESSION_ID in projected columns:\n%s", sql)
			}
			if !strings.Contains(sql, "INCLUDE_CLIENT_GENERATED_STATEMENT => TRUE") {
				t.Errorf("expected include-client-generated in SQL:\n%s", sql)
			}
			for _, want := range tt.wantContains {
				if !strings.Contains(sql, want) {
					t.Errorf("expected %q in SQL:\n%s", want, sql)
				}
			}
		})
	}
}

func TestBuildQueryHistorySQLSessionInjection(t *testing.T) {
	// A non-numeric / injection-laden session id violates the builder's
	// precondition and must panic (never be embedded as an argument).
	for _, sid := range []string{
		"1234, RESULT_LIMIT => 10000",
		"1234; DROP TABLE",
		" 1234 ",
		"abc",
		"",
		"12345678901234567890123456789", // longer than int64 max
		"9999999999999999999",           // 19 digits but overflows int64
		"007",                           // leading zeros
	} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("session id %q must panic, not be embedded", sid)
				}
			}()
			buildQueryHistorySql("session", sid, "", "", "", "", 100, false, QueryHistoryFilters{})
		}()
	}

	// A clean numeric id is embedded as-is.
	sql := buildQueryHistorySql("session", "1234567890", "", "", "", "", 100, false, QueryHistoryFilters{})
	if !strings.Contains(sql, "SESSION_ID => 1234567890") {
		t.Errorf("expected numeric SESSION_ID argument:\n%s", sql)
	}
}

func TestBuildQueryHistorySQLTimeRange(t *testing.T) {
	sql := buildQueryHistorySql("all", "", "", "", "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", 50, false, QueryHistoryFilters{})
	if !strings.Contains(sql, "END_TIME_RANGE_START => '2026-01-01T00:00:00Z'::TIMESTAMP_LTZ") {
		t.Errorf("missing range start in SQL:\n%s", sql)
	}
	if !strings.Contains(sql, "END_TIME_RANGE_END => '2026-01-02T00:00:00Z'::TIMESTAMP_LTZ") {
		t.Errorf("missing range end in SQL:\n%s", sql)
	}
	if strings.Contains(sql, "INCLUDE_CLIENT_GENERATED_STATEMENT") {
		t.Errorf("should not include client-generated when false:\n%s", sql)
	}
}

// TestBuildQueryHistorySQLTimeRangeQuoting exercises the QuoteStringLit guard on
// the timestamp args: a value containing a single-quote must be doubled, not
// embedded verbatim (which would break out of the literal). A clean RFC3339
// string can't distinguish QuoteStringLit from a bare '%s', so use a quote here.
func TestBuildQueryHistorySQLTimeRangeQuoting(t *testing.T) {
	sql := buildQueryHistorySql("all", "", "", "", "2026-01-01'T00:00:00Z", "2026-01-02T00:00:00Z", 50, false, QueryHistoryFilters{})
	if !strings.Contains(sql, "END_TIME_RANGE_START => '2026-01-01''T00:00:00Z'::TIMESTAMP_LTZ") {
		t.Errorf("single-quote in start timestamp must be doubled by QuoteStringLit:\n%s", sql)
	}
}

// TestGetQueryHistoryValidationGuards verifies the GetQueryHistory boundary
// guards (the primary gate) reject invalid filters before any client use, so a
// nil client is never dereferenced on these paths.
func TestGetQueryHistoryValidationGuards(t *testing.T) {
	tests := []struct {
		name       string
		filterType string
		sessionID  string
		userName   string
		warehouse  string
		wantErr    string
	}{
		{"invalid session id", "session", "abc", "", "", "invalid session id"},
		{"empty session id", "session", "", "", "", "invalid session id"},
		{"empty user", "user", "", "", "", "user name is required"},
		{"whitespace user", "user", "", "   ", "", "user name is required"},
		{"empty warehouse", "warehouse", "", "", "", "warehouse name is required"},
		{"whitespace warehouse", "warehouse", "", "", "  ", "warehouse name is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// nil client is safe: every guard returns before the client is used.
			_, err := GetQueryHistory(context.Background(), nil, tt.filterType, tt.sessionID, tt.userName, tt.warehouse, "", "", 100, false, QueryHistoryFilters{})
			if err == nil {
				t.Fatalf("expected an error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error %q does not contain %q", err.Error(), tt.wantErr)
			}
		})
	}
}

// TestBuildQueryHistorySQLFilters exercises the WHERE-clause filters: status and
// type IN-lists (case-insensitive, uppercased), the duration threshold, and the
// database/schema equality predicates. It also asserts the limit re-application:
// with a filter active the table function fetches the full window and the user's
// limit becomes an outer LIMIT.
func TestBuildQueryHistorySQLFilters(t *testing.T) {
	filters := QueryHistoryFilters{
		Statuses:      []string{"success", "FAIL"},
		QueryTypes:    []string{"select"},
		MinDurationMs: 5000,
		Database:      "my_db",
		Schema:        "public",
	}
	sql := buildQueryHistorySql("all", "", "", "", "", "", 100, false, filters)

	wantContains := []string{
		"WHERE ",
		"UPPER(EXECUTION_STATUS) IN ('SUCCESS', 'FAIL')",
		"UPPER(QUERY_TYPE) IN ('SELECT')",
		"TOTAL_ELAPSED_TIME >= 5000",
		"UPPER(DATABASE_NAME) = UPPER('my_db')",
		"UPPER(SCHEMA_NAME) = UPPER('public')",
		// The user's limit (100) is smaller than maxResultLimit, so the function
		// fetches the full window and the limit is re-applied afterwards.
		"RESULT_LIMIT => 10000",
		"LIMIT 100",
	}
	for _, want := range wantContains {
		if !strings.Contains(sql, want) {
			t.Errorf("expected %q in SQL:\n%s", want, sql)
		}
	}
	// The user's limit must be re-applied only as an outer LIMIT, never handed to
	// the table function (which would filter an already-truncated page).
	if strings.Contains(sql, "RESULT_LIMIT => 100)") {
		t.Errorf("user limit should not be passed to the table function when filters are active:\n%s", sql)
	}
}

// TestBuildQueryHistorySQLNoFilters confirms the unfiltered path is unchanged: no
// WHERE clause, no outer LIMIT, and the user's limit goes straight to the table
// function.
func TestBuildQueryHistorySQLNoFilters(t *testing.T) {
	sql := buildQueryHistorySql("all", "", "", "", "", "", 100, false, QueryHistoryFilters{})
	if strings.Contains(sql, "WHERE ") {
		t.Errorf("did not expect a WHERE clause without filters:\n%s", sql)
	}
	if strings.Contains(sql, "\nLIMIT ") {
		t.Errorf("did not expect an outer LIMIT without filters:\n%s", sql)
	}
	if !strings.Contains(sql, "RESULT_LIMIT => 100") {
		t.Errorf("expected the user limit passed to the table function:\n%s", sql)
	}
}

// TestBuildQueryHistorySQLFilterEscaping guards against injection through the
// string-valued filters: values are escaped by snowflake.QuoteStringLit and
// uppercased for the case-insensitive IN-lists.
func TestBuildQueryHistorySQLFilterEscaping(t *testing.T) {
	filters := QueryHistoryFilters{
		Statuses: []string{"a') OR ('1'='1"},
		Database: "db') OR ('1'='1",
	}
	sql := buildQueryHistorySql("all", "", "", "", "", "", 100, false, filters)
	if !strings.Contains(sql, "UPPER(EXECUTION_STATUS) IN ('A'') OR (''1''=''1')") {
		t.Errorf("status literal must be escaped and uppercased:\n%s", sql)
	}
	if !strings.Contains(sql, "UPPER(DATABASE_NAME) = UPPER('db'') OR (''1''=''1')") {
		t.Errorf("database literal must be escaped:\n%s", sql)
	}
}

// TestBuildQueryHistorySQLBlankFilterValues verifies that all-blank filter slices
// and whitespace-only strings add no predicate (so an accidental empty multi-select
// does not emit an empty IN-list or a spurious WHERE).
func TestBuildQueryHistorySQLBlankFilterValues(t *testing.T) {
	filters := QueryHistoryFilters{
		Statuses:      []string{"", "   "},
		QueryTypes:    []string{},
		MinDurationMs: 0,
		Database:      "   ",
		Schema:        "",
	}
	sql := buildQueryHistorySql("all", "", "", "", "", "", 100, false, filters)
	if strings.Contains(sql, "WHERE ") {
		t.Errorf("blank filter values must not produce a WHERE clause:\n%s", sql)
	}
}

func TestParseQueryHistory(t *testing.T) {
	res := &snowflake.QueryResult{
		Columns: []string{
			"QUERY_ID", "SESSION_ID", "QUERY_TEXT", "QUERY_TYPE", "USER_NAME", "WAREHOUSE_NAME",
			"DATABASE_NAME", "SCHEMA_NAME", "START_TIME", "END_TIME",
			"TOTAL_ELAPSED_TIME", "EXECUTION_STATUS", "ERROR_MESSAGE",
			"ROWS_PRODUCED", "BYTES_SCANNED",
		},
		Rows: [][]interface{}{
			{
				"q1", "9876543210", "SELECT 1", "SELECT", "ALICE", "WH",
				"DB", "PUBLIC", "2026-01-01", "2026-01-01",
				int64(1500), "SUCCESS", "",
				int64(1), int64(2048),
			},
		},
	}
	rows := ParseQueryHistory(res)
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	got := rows[0]
	if got.QueryID != "q1" || got.SessionID != "9876543210" || got.UserName != "ALICE" || got.Status != "SUCCESS" {
		t.Errorf("unexpected projection: %+v", got)
	}
	if got.ElapsedMs != 1500 || got.RowsProduced != 1 || got.BytesScanned != 2048 {
		t.Errorf("unexpected numeric projection: %+v", got)
	}
}

func TestParseQueryHistoryNil(t *testing.T) {
	if rows := ParseQueryHistory(nil); len(rows) != 0 {
		t.Errorf("expected empty slice for nil result, got %d", len(rows))
	}
}
