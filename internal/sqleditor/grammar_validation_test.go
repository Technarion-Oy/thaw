package sqleditor

import "testing"

func grammarMarkers(sql string) []DiagMarker {
	return ValidateGrammar(sql, GetStatementRanges(sql))
}

func TestValidateGrammar_AcceptsValid(t *testing.T) {
	valid := []string{
		`CREATE DATABASE my_db`,
		`CREATE OR REPLACE TABLE t (a INT, b STRING)`,
		`ALTER TABLE t RENAME TO t2`,
		`DROP TABLE IF EXISTS t CASCADE`,
		`SHOW TABLES IN SCHEMA db.s LIKE 'foo%'`,
		`SELECT * FROM t WHERE x = 1`,
		`GRANT ROLE r1 TO ROLE r2`,
		`USE WAREHOUSE wh`,
		// Multi-statement: all valid.
		"CREATE DATABASE a;\nDROP DATABASE b;",
	}
	for _, sql := range valid {
		if m := grammarMarkers(sql); len(m) != 0 {
			t.Errorf("expected no grammar markers for %q, got %d: %+v", sql, len(m), m)
		}
	}
}

func TestValidateGrammar_SkipsUnmodeled(t *testing.T) {
	// Leading keyword has no implemented grammar → no markers, ever.
	unmodeled := []string{
		`FOOBAR baz qux`,
		`-- just a comment`,
		``,
	}
	for _, sql := range unmodeled {
		if m := grammarMarkers(sql); len(m) != 0 {
			t.Errorf("expected unmodeled %q to be skipped, got markers: %+v", sql, m)
		}
	}
}

func TestValidateGrammar_FlagsMalformed(t *testing.T) {
	markers := grammarMarkers(`CREATE DATABASE`)
	if len(markers) != 1 {
		t.Fatalf("expected exactly 1 marker for `CREATE DATABASE`, got %d: %+v", len(markers), markers)
	}
	if markers[0].Severity != SeverityWarning {
		t.Errorf("expected Warning severity, got %d", markers[0].Severity)
	}
	if markers[0].Message == "" {
		t.Errorf("expected a non-empty 'expected …' message")
	}
}

// TestValidateGrammar_MultiLineFailureTokenSpan guards issue #703 (bug 2): when
// the failure token spans multiple lines (a `$$…$$` block, a multi-line string),
// the marker end must follow the token onto its last line instead of assuming
// the span stays on the start line.
func TestValidateGrammar_MultiLineFailureTokenSpan(t *testing.T) {
	sql := "SELECT 1;\nEXECUTE IMMEDIATE\n$$\nSELECT 1\n$$"
	markers := grammarMarkers(sql)
	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d: %+v", len(markers), markers)
	}
	m := markers[0]
	if m.StartLineNumber != 3 || m.StartColumn != 1 {
		t.Errorf("expected start at 3:1 (the `$$` block), got %d:%d", m.StartLineNumber, m.StartColumn)
	}
	// The `$$…$$` token ends on line 5; the span must reach it, not stop on line 3.
	if m.EndLineNumber != 5 || m.EndColumn != 3 {
		t.Errorf("expected end at 5:3 (end of the multi-line token), got %d:%d", m.EndLineNumber, m.EndColumn)
	}
}

// TestValidateGrammar_MarkerSkipsTrailingComment guards issue #703 (bug 3): an
// EOF-anchored marker must anchor just past the last significant token, not on a
// trailing line comment the grammar never consumes.
func TestValidateGrammar_MarkerSkipsTrailingComment(t *testing.T) {
	sql := "GRANT ROLE r TO -- oops"
	markers := grammarMarkers(sql)
	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d: %+v", len(markers), markers)
	}
	// `TO` ends at column 16; the marker must sit there, not at the comment (col 23+).
	if markers[0].EndColumn > 16 {
		t.Errorf("expected marker anchored just past `TO` (≤ col 16), got col %d — landed on the trailing comment",
			markers[0].EndColumn)
	}
}

func TestValidateGrammar_RebasesPositionToSecondStatement(t *testing.T) {
	// First statement valid; second (on line 3) is malformed. The marker must
	// land on line 3, not line 1.
	sql := "SELECT 1;\n\nGRANT ROLE r TO"
	markers := grammarMarkers(sql)
	if len(markers) != 1 {
		t.Fatalf("expected 1 marker, got %d: %+v", len(markers), markers)
	}
	if markers[0].StartLineNumber != 3 {
		t.Errorf("expected marker on line 3, got line %d (col %d)", markers[0].StartLineNumber, markers[0].StartColumn)
	}
}
