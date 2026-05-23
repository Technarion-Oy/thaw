package sqleditor

import (
	"strings"
	"testing"
)

func TestValidateSnowflakePatterns_TimeTravel(t *testing.T) {
	// ── Valid Time Travel queries ─────────────────────────────────────────
	validQueries := []string{
		"SELECT * FROM orders AT (TIMESTAMP => '2024-01-01 00:00:00'::TIMESTAMP_LTZ)",
		"SELECT * FROM orders AT (OFFSET => -3600)",
		"SELECT * FROM orders AT (OFFSET => -60*5)",
		"SELECT * FROM orders AT (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726')",
		"SELECT * FROM orders BEFORE (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726')",
		"SELECT * FROM orders BEFORE (TIMESTAMP => '2024-01-01 00:00:00'::TIMESTAMP_LTZ)",
		"SELECT * FROM orders BEFORE (OFFSET => -3600)",
		"SELECT * FROM orders AT (STREAM => my_stream)",
		// Fully qualified table with Time Travel
		"SELECT * FROM db.schema.orders AT (TIMESTAMP => '2024-01-01')",
		// Time Travel in CLONE context (already supported)
		"CREATE TABLE t CLONE s AT (TIMESTAMP => TO_TIMESTAMP_TZ('2023-01-01 00:00:00'))",
		"CREATE STREAM my_stream ON TABLE my_table AT (TIMESTAMP => TO_TIMESTAMP_TZ('2023-01-01 00:00:00'))",
		// Multiple Time Travel clauses in one query (JOIN)
		"SELECT a.id FROM t1 AT (OFFSET => -60) JOIN t2 BEFORE (STATEMENT => '8e5d0ca9-005e-44e6-b858-a8f5b37c5726') ON a.id = b.id",
		// DML with Time Travel
		"INSERT INTO t SELECT * FROM s AT (OFFSET => -3600)",
		// Case variation — lowercase keywords
		"SELECT * FROM orders at (timestamp => '2024-01-01')",
		"SELECT * FROM orders before (statement => 'abc-123')",
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 40)], func(t *testing.T) {
			ranges := GetStatementRanges(sql)
			markers := ValidateSnowflakePatterns(sql, ranges)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}

	// ── Invalid Time Travel queries ───────────────────────────────────────
	invalidCases := []struct {
		name     string
		sql      string
		wantMsgs []string
	}{
		{
			"Missing => operator",
			"SELECT * FROM orders AT (TIMESTAMP '2024-01-01')",
			[]string{"Missing '=>' operator"},
		},
		{
			"Multiple arguments",
			"SELECT * FROM orders AT (TIMESTAMP => '2024-01-01', OFFSET => -60)",
			[]string{"Multiple keyword arguments"},
		},
		{
			"STREAM in BEFORE clause",
			"SELECT * FROM orders BEFORE (STREAM => my_stream)",
			[]string{"STREAM => is not valid in a BEFORE clause"},
		},
		{
			"Missing parentheses",
			"SELECT * FROM orders AT TIMESTAMP '2024-01-01'",
			[]string{"requires parentheses"},
		},
		{
			"Unknown content in AT clause",
			"SELECT * FROM orders AT (123)",
			[]string{"Invalid AT clause. Expected one of"},
		},
		{
			"Unknown content in BEFORE clause",
			"SELECT * FROM orders BEFORE (123)",
			[]string{"Invalid BEFORE clause. Expected one of"},
		},
	}

	for _, tc := range invalidCases {
		t.Run(tc.name, func(t *testing.T) {
			ranges := GetStatementRanges(tc.sql)
			markers := ValidateSnowflakePatterns(tc.sql, ranges)
			warns := getWarnings(markers)
			if len(warns) == 0 {
				t.Errorf("Expected warnings for %q, got 0", tc.sql)
				return
			}
			for _, wantMsg := range tc.wantMsgs {
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, wantMsg) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", wantMsg, tc.sql, warns)
				}
			}
		})
	}
}

// ── Replication Group / Failover Group Tests ────────────────────────────────


