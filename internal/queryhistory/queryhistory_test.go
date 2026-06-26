// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package queryhistory

import (
	"strings"
	"testing"

	"thaw/internal/snowflake"
)

func TestBuildQueryHistorySql(t *testing.T) {
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
			sql := BuildQueryHistorySql(tt.filterType, tt.sessionID, tt.userName, tt.warehouse, "", "", 100, true)
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

func TestBuildQueryHistorySqlSessionInjection(t *testing.T) {
	// A non-numeric / injection-laden session id must never be embedded.
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
		sql := BuildQueryHistorySql("session", sid, "", "", "", "", 100, false)
		if strings.Contains(sql, "SESSION_ID =>") {
			t.Errorf("session id %q must not be embedded as an argument:\n%s", sid, sql)
		}
	}

	// A clean numeric id is embedded as-is.
	sql := BuildQueryHistorySql("session", "1234567890", "", "", "", "", 100, false)
	if !strings.Contains(sql, "SESSION_ID => 1234567890") {
		t.Errorf("expected numeric SESSION_ID argument:\n%s", sql)
	}
}

func TestBuildQueryHistorySqlTimeRange(t *testing.T) {
	sql := BuildQueryHistorySql("all", "", "", "", "2026-01-01T00:00:00Z", "2026-01-02T00:00:00Z", 50, false)
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
