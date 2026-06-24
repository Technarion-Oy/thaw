package sqlgrammar

import (
	"strings"
	"testing"
)

// TestIdentifyStatement covers the effective-verb classifier, especially the
// WITH/CTE bypass that a literal-first-keyword check would get wrong.
func TestIdentifyStatement(t *testing.T) {
	cases := []struct {
		sql  string
		want StatementKind
	}{
		// Plain leaders.
		{"SELECT 1", StmtSelect},
		{"INSERT INTO t VALUES (1)", StmtInsert},
		{"UPDATE t SET a = 1", StmtUpdate},
		{"DELETE FROM t", StmtDelete},
		{"MERGE INTO t USING s ON t.id = s.id WHEN MATCHED THEN DELETE", StmtMerge},
		{"CREATE TABLE t (a INT)", StmtOther},
		{"", StmtOther},

		// WITH/CTE prefixes resolve to the statement they feed, not "WITH".
		{"WITH c AS (SELECT 1) SELECT * FROM c", StmtSelect},
		{"WITH c AS (SELECT 1) INSERT INTO t SELECT * FROM c", StmtInsert},
		{"WITH c AS (SELECT 1) DELETE FROM t WHERE id IN (SELECT id FROM c)", StmtDelete},
		{"WITH c (a, b) AS (SELECT 1, 2) UPDATE t SET x = 1", StmtUpdate},

		// A verb buried inside the CTE body must not leak out: the inner INSERT-ish
		// keyword sits at paren depth > 0, so the outer SELECT wins.
		{"WITH c AS (SELECT 'INSERT' AS k FROM t) SELECT * FROM c", StmtSelect},
		{"WITH c AS (SELECT * FROM (SELECT 1)) SELECT * FROM c", StmtSelect},
	}
	for _, tc := range cases {
		if got := New(tc.sql).IdentifyStatement(); got != tc.want {
			t.Errorf("IdentifyStatement(%q) = %v, want %v", tc.sql, got, tc.want)
		}
	}
}

// TestFailureMessageNamesToken verifies the diagnostic names both the offending
// token and the expectation, and reports end-of-statement when the parser runs
// off the end.
func TestFailureMessageNamesToken(t *testing.T) {
	// A malformed statement whose first token is recognized but whose body is
	// wrong, so a furthest-token failure is produced.
	v := New("CREATE TABLE")
	if v.ParseTopLevel() {
		t.Fatalf("expected CREATE TABLE (no body) to fail")
	}
	msg := v.Failure().Message()
	if !strings.HasPrefix(msg, "unexpected ") {
		t.Errorf("message should start with 'unexpected ', got %q", msg)
	}
	if !strings.Contains(msg, "end of statement") {
		t.Errorf("running off the end should say 'end of statement', got %q", msg)
	}

	// A statement that fails at a concrete token names it in quotes.
	v2 := New("SELECT 1 FROM")
	_ = v2.ParseTopLevel()
	if got := v2.Failure(); got.Found != "" {
		if !strings.Contains(got.Message(), "'"+got.Found+"'") {
			t.Errorf("message should quote the found token %q, got %q", got.Found, got.Message())
		}
	}
}
