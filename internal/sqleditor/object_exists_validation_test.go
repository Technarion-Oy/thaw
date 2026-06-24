package sqleditor

import (
	"strings"
	"testing"
)

// These tests lock in the "object exists OR is created in-script" behavior of
// ValidateTablesExist: a referenced object is accepted when it is present in the
// known catalog OR a CREATE earlier in the same script defines it; otherwise it
// is flagged. (See #556 — diagnostics check object-reference tokens for
// existence-or-creation.)
func tablesExistMarkers(sql string) []DiagMarker {
	return ValidateTablesExist(ValidateTablesExistRequest{
		SQL:            sql,
		StmtRanges:     GetStatementRanges(sql),
		KnownDatabases: []string{"MYDB"},
		KnownSchemas:   []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
	})
}

func TestValidateTablesExist_CreatedInScriptSuppressesWarning(t *testing.T) {
	created := []string{
		"CREATE TABLE t (id INT);\nSELECT * FROM t;",
		"CREATE TABLE mydb.public.t (id INT);\nSELECT * FROM mydb.public.t;",
		"CREATE OR REPLACE VIEW v AS SELECT 1;\nSELECT * FROM v;",
		"CREATE SCHEMA mydb.staging;\nCREATE TABLE mydb.staging.t (id INT);",
	}
	for _, sql := range created {
		if m := tablesExistMarkers(sql); len(m) != 0 {
			t.Errorf("expected no marker (object created in-script) for %q, got %d: %+v", sql, len(m), m)
		}
	}
}

func TestValidateTablesExist_MissingObjectFlagged(t *testing.T) {
	cases := []struct {
		sql  string
		want string // substring expected in the message
	}{
		{"SELECT * FROM nonexistent_tbl;", "does not exist"},
		{"SELECT * FROM mydb.nope.t;", "Schema 'nope'"},
	}
	for _, c := range cases {
		m := tablesExistMarkers(c.sql)
		if len(m) == 0 {
			t.Errorf("expected a missing-object marker for %q, got none", c.sql)
			continue
		}
		if !strings.Contains(m[0].Message, c.want) {
			t.Errorf("for %q: message %q does not contain %q", c.sql, m[0].Message, c.want)
		}
	}
}
