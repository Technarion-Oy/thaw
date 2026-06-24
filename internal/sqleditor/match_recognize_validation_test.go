package sqleditor

import (
	"testing"
)

// ── MATCH_RECOGNIZE Tests ────────────────────────────────────────────────────

func TestValidateBareColumnRefs_MatchRecognizeSuppression(t *testing.T) {
	// MATCH_RECOGNIZE queries should not produce false-positive column warnings
	// because pattern variables (A, B) are local aliases, not table references.
	validQueries := []string{
		// Pattern variables in MEASURES should not be flagged
		`SELECT * FROM DB.SCH.EMPLOYEES MATCH_RECOGNIZE (ORDER BY ID MEASURES FIRST(A.SALARY) AS start_sal PATTERN (A B+) DEFINE A AS SALARY < 50000, B AS SALARY > A.SALARY) AS mr`,
		// Result alias mr.start_sal should not be flagged
		`SELECT mr.start_sal FROM DB.SCH.EMPLOYEES MATCH_RECOGNIZE (ORDER BY ID MEASURES FIRST(A.SALARY) AS start_sal PATTERN (A B+) DEFINE A AS SALARY < 50000, B AS SALARY > A.SALARY) AS mr`,
	}

	req := ValidateBareColsRequest{
		ResolvedRefs: getTestRefs(),
		ColEntries:   getTestColCaches(),
	}

	for _, sql := range validQueries {
		t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
			req.SQL = sql
			req.StmtRanges = GetStatementRanges(sql)
			markers := ValidateBareColumnRefs(req)
			warnings := getWarnings(markers)
			if len(warnings) > 0 {
				t.Errorf("Expected 0 warnings for MATCH_RECOGNIZE query %q, got %d: %v", sql[:60], len(warnings), warnings)
			}
		})
	}
}
