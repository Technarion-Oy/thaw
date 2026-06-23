package sqleditor

import (
	"testing"
)

// ── PIVOT / UNPIVOT Tests ────────────────────────────────────────────────────

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
