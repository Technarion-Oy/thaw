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

func TestValidateGrammar_SkipsUnmodelled(t *testing.T) {
	// Leading keyword has no implemented grammar → no markers, ever.
	unmodelled := []string{
		`FOOBAR baz qux`,
		`-- just a comment`,
		``,
	}
	for _, sql := range unmodelled {
		if m := grammarMarkers(sql); len(m) != 0 {
			t.Errorf("expected unmodelled %q to be skipped, got markers: %+v", sql, m)
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
