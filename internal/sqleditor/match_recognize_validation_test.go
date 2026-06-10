package sqleditor

import (
	"strings"
	"testing"
)

// ── MATCH_RECOGNIZE Tests ────────────────────────────────────────────────────

func TestValidateSnowflakePatterns_MatchRecognize(t *testing.T) {
	t.Run("valid MATCH_RECOGNIZE queries", func(t *testing.T) {
		validQueries := []string{
			// Basic MATCH_RECOGNIZE with all clauses
			`SELECT * FROM stock_prices
MATCH_RECOGNIZE (
  PARTITION BY symbol
  ORDER BY trade_date
  MEASURES
    FIRST(A.price) AS start_price,
    LAST(B.price)  AS end_price
  ONE ROW PER MATCH
  PATTERN (A B+)
  DEFINE
    A AS price < AVG(price) OVER (ROWS BETWEEN 5 PRECEDING AND CURRENT ROW),
    B AS price > A.price
) AS mr`,
			// Minimal: only mandatory PATTERN
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) DEFINE A AS col > 0, B AS col < 0)`,
			// ALL ROWS PER MATCH
			`SELECT * FROM t MATCH_RECOGNIZE (ALL ROWS PER MATCH PATTERN (X+) DEFINE X AS val > 10)`,
			// AFTER MATCH SKIP TO NEXT ROW
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO NEXT ROW DEFINE A AS x > 0, B AS x < 0)`,
			// AFTER MATCH SKIP PAST LAST ROW
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP PAST LAST ROW DEFINE A AS x > 0, B AS x < 0)`,
			// AFTER MATCH SKIP TO FIRST <var>
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO FIRST A DEFINE A AS x > 0, B AS x < 0)`,
			// AFTER MATCH SKIP TO LAST <var>
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO LAST B DEFINE A AS x > 0, B AS x < 0)`,
			// Mixed-case keywords
			`SELECT * FROM t match_recognize (pattern (a b+) define a AS col > 0, b AS col < 0)`,
			// With alias on result
			`SELECT mr.start_price FROM prices MATCH_RECOGNIZE (ORDER BY ts MEASURES FIRST(A.price) AS start_price PATTERN (A B+) DEFINE A AS price < 100, B AS price > A.price) AS mr`,
			// No space before opening paren
			`SELECT * FROM t MATCH_RECOGNIZE(PATTERN (X) DEFINE X AS val > 0)`,
			// With fully qualified table
			`SELECT * FROM db.schema.events MATCH_RECOGNIZE (ORDER BY ts PATTERN (A B) DEFINE A AS type = 'login', B AS type = 'purchase')`,
			// Complex PATTERN with quantifiers
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B* C+? D{2,5}) DEFINE A AS x=1, B AS x=2, C AS x=3, D AS x=4)`,
			// Keywords inside string literals must not trigger false positives
			`SELECT * FROM t MATCH_RECOGNIZE (
    PATTERN (A B)
    DEFINE A AS col = 'PATTERN', B AS col = 'DEFINE'
)`,
			// AFTER MATCH SKIP TO FIRST with quoted identifier
			`SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP TO FIRST "myVar" DEFINE A AS x > 0, B AS x < 0)`,
			// Multiple MATCH_RECOGNIZE in a single statement (subquery)
			`SELECT * FROM (SELECT * FROM t1 MATCH_RECOGNIZE (PATTERN (A B) DEFINE A AS x > 0, B AS x < 0)) a JOIN (SELECT * FROM t2 MATCH_RECOGNIZE (PATTERN (X Y+) DEFINE X AS v = 1, Y AS v = 2)) b ON a.id = b.id`,
		}
		for _, sql := range validQueries {
			t.Run(sql[:min(len(sql), 60)], func(t *testing.T) {
				stmtRanges := GetStatementRanges(sql)
				markers := ValidateSnowflakePatterns(sql, stmtRanges)
				warns := getWarnings(markers)
				if len(warns) > 0 {
					t.Errorf("Expected no warnings for MATCH_RECOGNIZE query, got: %v", warns)
				}
			})
		}
	})

	t.Run("invalid MATCH_RECOGNIZE queries", func(t *testing.T) {
		cases := []struct {
			sql     string
			wantMsg string
		}{
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (DEFINE A AS col > 0)`,
				wantMsg: "MATCH_RECOGNIZE requires a PATTERN clause",
			},
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN () DEFINE A AS col > 0)`,
				wantMsg: "MATCH_RECOGNIZE PATTERN must contain at least one pattern variable",
			},
			{
				sql: `SELECT * FROM t MATCH_RECOGNIZE (
					ONE ROW PER MATCH
					ALL ROWS PER MATCH
					PATTERN (A B)
					DEFINE A AS x > 0, B AS x < 0
				)`,
				wantMsg: "ONE ROW PER MATCH and ALL ROWS PER MATCH are mutually exclusive",
			},
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) AFTER MATCH SKIP NOWHERE DEFINE A AS x > 0, B AS x < 0)`,
				wantMsg: "Invalid AFTER MATCH SKIP target",
			},
			// Multi-line: invalid AFTER MATCH SKIP must be caught even when
			// DEFINE is on a separate line.
			{
				sql: `SELECT * FROM t MATCH_RECOGNIZE (
  PATTERN (A B)
  AFTER MATCH SKIP NOWHERE
  DEFINE A AS x > 0, B AS x < 0
)`,
				wantMsg: "Invalid AFTER MATCH SKIP target",
			},
			// Missing DEFINE clause.
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B))`,
				wantMsg: "MATCH_RECOGNIZE requires a DEFINE clause",
			},
			// DEFINE keyword present but without any variable bindings.
			{
				sql:     `SELECT * FROM t MATCH_RECOGNIZE (PATTERN (A B) DEFINE)`,
				wantMsg: "MATCH_RECOGNIZE requires a DEFINE clause",
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
					t.Errorf("Expected warning containing %q, got: %v", tc.wantMsg, warns)
				}
			})
		}
	})
}

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
