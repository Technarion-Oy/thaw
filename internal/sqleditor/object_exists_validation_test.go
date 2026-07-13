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
		SQL:             sql,
		StmtRanges:      GetStatementRanges(sql),
		KnownDatabases:  []string{"MYDB"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
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

// TestValidateTablesExist_PredicateLikeNotATable locks in the #707 fix: a LIKE
// in a WHERE predicate must not be treated as a table-introducing keyword. The
// string literal is stripped before the fallback table scan runs, so a generic
// LIKE keyword would capture the following AND/OR/ANY (or the RHS ident) as a
// bogus table name. T1 is registered so the SELECT itself is otherwise clean.
func TestValidateTablesExist_PredicateLikeNotATable(t *testing.T) {
	clean := func(sql string) []DiagMarker {
		return ValidateTablesExist(ValidateTablesExistRequest{
			SQL:             sql,
			StmtRanges:      GetStatementRanges(sql),
			KnownDatabases:  []string{"MYDB"},
			KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
			ResolvedRefs:    []ResolvedRef{{DB: "MYDB", Schema: "PUBLIC", Name: "T1"}, {DB: "MYDB", Schema: "PUBLIC", Name: "T2"}},
			SessionDatabase: "MYDB",
			SessionSchema:   "PUBLIC",
		})
	}
	for _, sql := range []string{
		"SELECT * FROM t1 WHERE name LIKE '%foo%' AND id = 1;",
		"SELECT * FROM t1 WHERE name LIKE '%x%' OR city = 'y';",
		"SELECT * FROM t1 WHERE a LIKE b;",
		"SELECT * FROM t1 WHERE name LIKE ANY ('%x%', '%y%');",
	} {
		if m := clean(sql); len(m) != 0 {
			t.Errorf("expected no marker for predicate LIKE in %q, got %d: %+v", sql, len(m), m)
		}
	}
}

// TestValidateTablesExist_CreateTableLikeStillTracked ensures the #707 fix did
// not break CREATE TABLE … LIKE <source>: the source table is still existence-
// checked (flagged when missing, silent when known).
func TestValidateTablesExist_CreateTableLikeStillTracked(t *testing.T) {
	if m := tablesExistMarkers("CREATE TABLE a LIKE b;"); len(m) == 0 {
		t.Errorf("expected a missing-source marker for CREATE TABLE a LIKE b, got none")
	}
	created := "CREATE TABLE src (id INT);\nCREATE TABLE a LIKE src;"
	if m := tablesExistMarkers(created); len(m) != 0 {
		t.Errorf("expected no marker when LIKE source created in-script, got %d: %+v", len(m), m)
	}
}

// TestValidateTablesExist_RebasesColumnForMidLineStatement guards issue #703:
// a statement that begins mid-line (the second here) must have its marker
// columns rebased to document coordinates, so the marker lands on `ghost`
// (cols 25–30), not inside the first statement.
func TestValidateTablesExist_RebasesColumnForMidLineStatement(t *testing.T) {
	sql := "SELECT 1; SELECT * FROM ghost;"
	m := tablesExistMarkers(sql)
	if len(m) != 1 {
		t.Fatalf("expected 1 marker, got %d: %+v", len(m), m)
	}
	if m[0].StartColumn != 25 || m[0].EndColumn != 30 {
		t.Errorf("expected marker at cols 25–30 (over `ghost`), got %d–%d",
			m[0].StartColumn, m[0].EndColumn)
	}
}

// TestValidateTablesExist_NoContext_SchemaScopedObjects verifies that the
// "No database selected" diagnostic covers all schema-scoped object types, not
// just TABLE/VIEW — and stays silent for account-level objects (which are not
// schema-scoped) and for fully qualified names.
func TestValidateTablesExist_NoContext_SchemaScopedObjects(t *testing.T) {
	noCtx := func(sql string) []DiagMarker {
		return ValidateTablesExist(ValidateTablesExistRequest{
			SQL:        sql,
			StmtRanges: GetStatementRanges(sql),
			// no KnownDatabases/KnownSchemas → no session context
		})
	}

	flagged := []struct{ sql, objType string }{
		{`CREATE OR REPLACE SEQUENCE seq_01 START = 1 INCREMENT = 1 ORDER;`, "sequence"},
		{`CREATE STAGE my_stage;`, "stage"},
		{`CREATE STREAM s ON TABLE t;`, "stream"},
		{`CREATE TASK t1 SCHEDULE = '1 minute' AS SELECT 1;`, "task"},
		{`CREATE FILE FORMAT ff TYPE = CSV;`, "file format"},
		{`CREATE TABLE foo (id INT);`, "table"},
	}
	for _, c := range flagged {
		m := noCtx(c.sql)
		want := "No database selected. Cannot create " + c.objType
		if len(m) == 0 || !strings.Contains(m[0].Message, want) {
			t.Errorf("for %q: expected a marker containing %q, got %+v", c.sql, want, m)
		}
	}

	// Account-level objects are not schema-scoped, and a fully qualified name is
	// self-contained — neither should warn about a missing database/schema.
	silent := []string{
		`CREATE WAREHOUSE wh;`,
		`CREATE DATABASE db1;`,
		`CREATE ROLE r1;`,
		`CREATE SEQUENCE mydb.sch.seq_01;`,
		// INDEX names are table-relative, not db.schema-qualified, so an unqualified
		// index name must never trigger the warning (PR #561 review).
		`CREATE INDEX idx ON db.sch.tbl(c);`,
		`CREATE INDEX idx ON tbl(c);`,
	}
	for _, sql := range silent {
		if m := noCtx(sql); len(m) != 0 {
			t.Errorf("for %q: expected no marker, got %d: %+v", sql, len(m), m)
		}
	}
}

// Regression for #689: a populated catalog must NOT be mistaken for a selected
// database/schema. Once connected the object store always holds databases, but
// unless one is actually USE'd (SessionDatabase/SessionSchema), an unqualified
// schema-scoped CREATE still has nowhere to resolve and must warn.
func TestValidateTablesExist_CatalogPresentButNoSession(t *testing.T) {
	req := ValidateTablesExistRequest{
		KnownDatabases: []string{"DB1", "DB2"},
		KnownSchemas:   []SchemaEntry{{DB: "DB1", Name: "PUBLIC"}},
		// SessionDatabase/SessionSchema deliberately empty.
	}
	sql := "CREATE OR REPLACE TABLE spatial_test_data (id INT);"
	req.SQL = sql
	req.StmtRanges = GetStatementRanges(sql)
	m := ValidateTablesExist(req)
	if len(m) == 0 || !strings.Contains(m[0].Message, "No database selected") {
		t.Fatalf("expected 'No database selected' warning, got %+v", m)
	}

	// With a session database but no schema, it should flip to the schema warning.
	req.SessionDatabase = "DB1"
	m = ValidateTablesExist(req)
	if len(m) == 0 || !strings.Contains(m[0].Message, "No schema selected") {
		t.Fatalf("expected 'No schema selected' warning, got %+v", m)
	}

	// With both set, no warning.
	req.SessionSchema = "PUBLIC"
	if m = ValidateTablesExist(req); len(m) != 0 {
		t.Fatalf("expected no warning with full session context, got %+v", m)
	}
}
