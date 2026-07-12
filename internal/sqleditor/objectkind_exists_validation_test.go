package sqleditor

import (
	"strings"
	"testing"
)

// objKindMarkers runs ValidateTablesExist with a catalog of assorted
// schema-scoped objects in MYDB.PUBLIC, with PUBLIC marked as fetched.
func objKindMarkers(sql string) []DiagMarker {
	return ValidateTablesExist(ValidateTablesExistRequest{
		SQL:             sql,
		StmtRanges:      GetStatementRanges(sql),
		KnownDatabases:  []string{"MYDB"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
		KnownObjects: []ObjectRef{
			{DB: "MYDB", Schema: "PUBLIC", Name: "S1", Kind: "STREAM"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "T1", Kind: "TASK"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "FF1", Kind: "FILE FORMAT"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "P1", Kind: "PIPE"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "ET1", Kind: "EVENT TABLE"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "RAP1", Kind: "ROW ACCESS POLICY"},
			{DB: "MYDB", Schema: "OTHER", Name: "S2", Kind: "STREAM"},
			// A plain table — used for the kind-mismatch test.
			{DB: "MYDB", Schema: "PUBLIC", Name: "TBL1", Kind: "TABLE"},
		},
		FetchedObjectSchemas: []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
	})
}

func TestValidateObjectKind_ExistingSilent(t *testing.T) {
	ok := []string{
		"ALTER TASK t1 RESUME;",
		"DROP STREAM s1;",
		"DESCRIBE PIPE p1;",
		"DESC FILE FORMAT ff1;",
		"COMMENT ON STREAM s1 IS 'note';",
		"DROP FILE FORMAT ff1;",
		"ALTER EVENT TABLE et1 SET COMMENT = 'x';", // longest-first: EVENT TABLE, not TABLE
		"DROP ROW ACCESS POLICY rap1;",             // multi-word kind
		"ALTER STREAM mydb.public.s1 SET COMMENT = 'x';",
	}
	for _, sql := range ok {
		if m := objKindMarkers(sql); len(m) != 0 {
			t.Errorf("expected no marker for %q, got %d: %+v", sql, len(m), m)
		}
	}
}

func TestValidateObjectKind_MissingFlagged(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"ALTER TASK nope RESUME;", "Task 'nope'"},
		{"DROP STREAM nope;", "Stream 'nope'"},
		{"DESCRIBE PIPE nope;", "Pipe 'nope'"},
		{"DROP FILE FORMAT nope;", "File format 'nope'"},
		{"DROP ROW ACCESS POLICY nope;", "Row access policy 'nope'"},
		{"COMMENT ON TASK nope IS 'x';", "Task 'nope'"},
	}
	for _, c := range cases {
		m := objKindMarkers(c.sql)
		if len(m) == 0 {
			t.Errorf("expected a marker for %q, got none", c.sql)
			continue
		}
		if !strings.Contains(m[0].Message, c.want) {
			t.Errorf("for %q: message %q lacks %q", c.sql, m[0].Message, c.want)
		}
	}
}

// A name that exists as a TABLE, used with the wrong kind, must flag.
func TestValidateObjectKind_KindMismatchFlagged(t *testing.T) {
	m := objKindMarkers("DROP STREAM tbl1;")
	if len(m) == 0 || !strings.Contains(m[0].Message, "Stream 'tbl1'") {
		t.Errorf("a TABLE name used as DROP STREAM must flag, got %+v", m)
	}
}

func TestValidateObjectKind_IfExistsSilent(t *testing.T) {
	for _, sql := range []string{"DROP STREAM IF EXISTS nope;", "ALTER TASK IF EXISTS nope RESUME;"} {
		if m := objKindMarkers(sql); len(m) != 0 {
			t.Errorf("IF EXISTS should suppress the marker for %q, got %+v", sql, m)
		}
	}
}

// Account-/database-scoped kinds are not schema-scoped, so the sweep must not
// touch them (deferred to phase 3).
func TestValidateObjectKind_AccountScopedSkipped(t *testing.T) {
	for _, sql := range []string{
		"DROP WAREHOUSE nope;",
		"ALTER ROLE nope SET COMMENT = 'x';",
		"DROP DATABASE nope;", // handled by the DB path, not this sweep
	} {
		// These must not produce a "<kind> does not exist" object-kind marker.
		for _, m := range objKindMarkers(sql) {
			if strings.Contains(m.Message, "Warehouse") || strings.Contains(m.Message, "Role") {
				t.Errorf("account-scoped %q should be skipped, got %+v", sql, m)
			}
		}
	}
}

func TestValidateObjectKind_CreatedOrDroppedInScript(t *testing.T) {
	if m := objKindMarkers("CREATE STREAM news ON TABLE t;\nDROP STREAM news;"); len(m) != 0 {
		t.Errorf("stream created earlier should be silent, got %+v", m)
	}
	m := objKindMarkers("DROP STREAM s1;\nALTER STREAM s1 SET COMMENT = 'x';")
	if len(m) == 0 || !strings.Contains(m[0].Message, "Stream 's1'") {
		t.Errorf("stream dropped earlier should be flagged on later use, got %+v", m)
	}
}

// A kind we never see in the catalog (no SHOW command / SHOW failed) stays
// silent — sequences are not listed by ListObjects.
func TestValidateObjectKind_UnseenKindSilent(t *testing.T) {
	if m := objKindMarkers("ALTER SEQUENCE nope SET INCREMENT = 2;"); len(m) != 0 {
		t.Errorf("unseen kind (sequence) should be silent, got %+v", m)
	}
}

func TestValidateObjectKind_UnfetchedSchemaSilent(t *testing.T) {
	m := ValidateTablesExist(ValidateTablesExistRequest{
		SQL:             "DROP STREAM nope;",
		StmtRanges:      GetStatementRanges("DROP STREAM nope;"),
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
		KnownObjects:    []ObjectRef{{DB: "MYDB", Schema: "PUBLIC", Name: "S1", Kind: "STREAM"}},
		// FetchedObjectSchemas empty → no data → silent.
	})
	if len(m) != 0 {
		t.Errorf("unfetched schema should be silent, got %+v", m)
	}
}

func TestValidateObjectKind_QuickFixWhenInOtherSchema(t *testing.T) {
	// S2 lives in MYDB.OTHER; referenced unqualified it is missing here but a
	// qualify quick-fix should be offered.
	m := objKindMarkers("DROP STREAM s2;")
	if len(m) == 0 {
		t.Fatalf("expected a marker for s2, got none")
	}
	if !strings.Contains(m[0].Code, "qualify-stream") || !strings.Contains(m[0].Code, "MYDB.OTHER.S2") {
		t.Errorf("expected qualify-stream code payload, got %q", m[0].Code)
	}
}
