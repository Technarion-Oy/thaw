package sqleditor

import (
	"strings"
	"testing"
)

// ── PIVOT / UNPIVOT Tests ────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_Pivot(t *testing.T) {
	t.Run("valid PIVOT queries", func(t *testing.T) {
		validQueries := []string{
			// Basic PIVOT with SUM
			"SELECT * FROM monthly_sales PIVOT (SUM(amount) FOR month IN ('Jan', 'Feb', 'Mar')) AS p",
			// PIVOT with AVG
			"SELECT * FROM sales PIVOT (AVG(revenue) FOR quarter IN ('Q1', 'Q2', 'Q3', 'Q4'))",
			// PIVOT with COUNT
			"SELECT * FROM events PIVOT (COUNT(event_id) FOR status IN ('active', 'inactive'))",
			// PIVOT with MAX
			"SELECT * FROM readings PIVOT (MAX(value) FOR sensor IN ('temp', 'humidity')) AS pvt",
			// PIVOT with MIN
			"SELECT * FROM readings PIVOT (MIN(value) FOR sensor IN ('temp', 'humidity'))",
			// PIVOT with ANY_VALUE
			"SELECT * FROM data PIVOT (ANY_VALUE(val) FOR key IN ('a', 'b'))",
			// PIVOT with LISTAGG
			"SELECT * FROM data PIVOT (LISTAGG(name) FOR category IN ('x', 'y'))",
			// PIVOT with MEDIAN
			"SELECT * FROM data PIVOT (MEDIAN(score) FOR subject IN ('math', 'science'))",
			// PIVOT with STDDEV
			"SELECT * FROM data PIVOT (STDDEV(measurement) FOR sensor IN ('s1', 's2'))",
			// PIVOT with VARIANCE
			"SELECT * FROM data PIVOT (VARIANCE(amount) FOR region IN ('east', 'west'))",
			// PIVOT with numeric values in IN list
			"SELECT * FROM data PIVOT (SUM(val) FOR code IN (1, 2, 3))",
			// PIVOT with fully qualified table
			"SELECT * FROM db.schema.monthly_sales PIVOT (SUM(amount) FOR month IN ('Jan', 'Feb'))",
			// PIVOT with alias on the source table
			"SELECT * FROM sales_data s PIVOT (SUM(s.amount) FOR s.month IN ('Jan', 'Feb'))",
			// Mixed-case keywords
			"SELECT * FROM t pivot (sum(amount) for month in ('Jan', 'Feb'))",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid PIVOT queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "SELECT * FROM sales PIVOT (INVALID_FUNC(amount) FOR month IN ('Jan'))",
				wantMsg: "not a valid aggregate function",
			},
			{
				sql:     "SELECT * FROM sales PIVOT (SUM(amount) FOR month IN ())",
				wantMsg: "PIVOT IN list must not be empty",
			},
			{
				sql:     "SELECT * FROM sales PIVOT (SUM(amount) IN ('Jan'))",
				wantMsg: "PIVOT requires FOR <column> IN",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

func TestValidateSnowflakePatterns_Unpivot(t *testing.T) {
	t.Run("valid UNPIVOT queries", func(t *testing.T) {
		validQueries := []string{
			// Basic UNPIVOT
			"SELECT * FROM wide_table UNPIVOT (value FOR metric IN (col_a, col_b, col_c))",
			// UNPIVOT with alias
			"SELECT * FROM wide_table UNPIVOT (val FOR col_name IN (q1, q2, q3, q4)) AS u",
			// UNPIVOT with fully qualified table
			"SELECT * FROM db.schema.wide_table UNPIVOT (value FOR metric IN (col_a, col_b))",
			// UNPIVOT with quoted identifiers
			`SELECT * FROM wide_table UNPIVOT ("value" FOR "metric" IN ("COL_A", "COL_B"))`,
			// UNPIVOT INCLUDE NULLS
			"SELECT * FROM wide_table UNPIVOT INCLUDE NULLS (value FOR metric IN (col_a, col_b))",
			// UNPIVOT EXCLUDE NULLS
			"SELECT * FROM wide_table UNPIVOT EXCLUDE NULLS (value FOR metric IN (col_a, col_b))",
			// No space before opening paren
			"SELECT * FROM wide_table UNPIVOT(value FOR metric IN (col_a, col_b))",
			// Mixed-case keywords
			"SELECT * FROM t unpivot (value FOR metric IN (col_a, col_b))",
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for %q, got: %v", sql, warns)
				}
			})
		}
	})

	t.Run("invalid UNPIVOT queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     "SELECT * FROM wide_table UNPIVOT (value FOR metric IN ())",
				wantMsg: "UNPIVOT IN list must not be empty",
			},
			{
				sql:     "SELECT * FROM wide_table UNPIVOT (value IN (col_a, col_b))",
				wantMsg: "UNPIVOT requires FOR <name_column> IN",
			},
			{
				sql:     "SELECT * FROM wide_table UNPIVOT INCLUDE NULLS (value FOR metric IN ())",
				wantMsg: "UNPIVOT IN list must not be empty",
			},
		}
		for _, tc := range cases {
			t.Run(tc.sql[:min(len(tc.sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(tc.sql)
				markers := ValidateSnowflakePatterns(tc.sql, stmtRanges)
				warns := getWarnings(markers)
				found := false
				for _, w := range warns {
					if strings.Contains(w.Message, tc.wantMsg) {
						found = true
					}
				}
				if !found {
					t.Errorf("Expected warning containing %q for %q, got: %v", tc.wantMsg, tc.sql, warns)
				}
			})
		}
	})
}

func TestValidateBareColumnRefs_PivotSuppression(t *testing.T) {
	// PIVOT/UNPIVOT queries should not produce false-positive column warnings
	// because the generated columns are dynamic / virtual.
	validQueries := []string{
		// PIVOT — the columns 'Alice','Bob' are generated dynamically; should not flag
		`SELECT * FROM DB.SCH.EMPLOYEES PIVOT (COUNT(ID) FOR FIRST_NAME IN ('Alice', 'Bob'))`,
		// PIVOT with alias — p.Alice, p.Bob should not be flagged
		`SELECT p.ID FROM DB.SCH.EMPLOYEES PIVOT (COUNT(ID) FOR FIRST_NAME IN ('Alice', 'Bob')) AS p`,
		// UNPIVOT — "value" and "metric" are generated columns
		`SELECT * FROM DB.SCH.EMPLOYEES UNPIVOT (value FOR metric IN (FIRST_NAME, LAST_NAME))`,
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 50)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateBareColumnRefs(req)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for PIVOT/UNPIVOT query %q, got %d: %v", sql, len(warnings), warnings)
			}
		})
	}
}

