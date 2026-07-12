package sqleditor

import (
	"strings"
	"testing"
)

// kindImpliedMarkers runs ValidateTablesExist with a catalog of a procedure,
// task, masking policy, row access policy, and file format in MYDB.PUBLIC
// (fetched), plus a resolved table T so ALTER TABLE T … is not itself flagged.
func kindImpliedMarkers(sql string) []DiagMarker {
	return ValidateTablesExist(ValidateTablesExistRequest{
		SQL:             sql,
		StmtRanges:      GetStatementRanges(sql),
		KnownDatabases:  []string{"MYDB"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
		ResolvedRefs:    []ResolvedRef{{DB: "MYDB", Schema: "PUBLIC", Name: "T"}},
		KnownObjects: []ObjectRef{
			{DB: "MYDB", Schema: "PUBLIC", Name: "MYPROC", Kind: "PROCEDURE"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "MYTASK", Kind: "TASK"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "MP1", Kind: "MASKING POLICY"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "RAP1", Kind: "ROW ACCESS POLICY"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "FF1", Kind: "FILE FORMAT"},
			{DB: "MYDB", Schema: "PUBLIC", Name: "T", Kind: "TABLE"},
		},
		FetchedObjectSchemas: []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
	})
}

func TestValidateKindImplied_ExistingSilent(t *testing.T) {
	ok := []string{
		"CALL myproc();",
		"CALL myproc(1, 'x');",
		"CALL mydb.public.myproc();",
		"CALL SYSTEM$WAIT(1);",             // system function
		"CALL SNOWFLAKE.CORE.SOMETHING();", // built-in namespace
		"EXECUTE TASK mytask;",
		"EXECUTE IMMEDIATE 'SELECT 1';", // not EXECUTE TASK
		"ALTER TABLE t MODIFY COLUMN c SET MASKING POLICY mp1;",
		"ALTER TABLE t ADD ROW ACCESS POLICY rap1 ON (c);",
		"CREATE STAGE st FILE_FORMAT = (FORMAT_NAME = 'ff1');",
		"CREATE STAGE st2 FILE_FORMAT = (FORMAT_NAME => 'ff1');",
	}
	for _, sql := range ok {
		if m := kindImpliedMarkers(sql); len(m) != 0 {
			t.Errorf("expected no marker for %q, got %d: %+v", sql, len(m), m)
		}
	}
}

func TestValidateKindImplied_MissingFlagged(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"CALL nope();", "Procedure 'nope'"},
		{"EXECUTE TASK nope;", "Task 'nope'"},
		{"ALTER TABLE t MODIFY COLUMN c SET MASKING POLICY nope;", "Masking policy 'nope'"},
		{"ALTER TABLE t ADD ROW ACCESS POLICY nope ON (c);", "Row access policy 'nope'"},
		{"CREATE STAGE st FILE_FORMAT = (FORMAT_NAME = 'nope');", "File format 'nope'"},
		{"CREATE STAGE st FILE_FORMAT = (FORMAT_NAME => 'nope');", "File format 'nope'"},
	}
	for _, c := range cases {
		m := kindImpliedMarkers(c.sql)
		if len(m) == 0 {
			t.Errorf("expected a marker for %q, got none", c.sql)
			continue
		}
		found := false
		for _, mk := range m {
			if strings.Contains(mk.Message, c.want) {
				found = true
			}
		}
		if !found {
			t.Errorf("for %q: no marker contains %q, got %+v", c.sql, c.want, m)
		}
	}
}

// A kind we never see in the catalog stays silent (no procedures listed here).
func TestValidateKindImplied_UnseenKindSilent(t *testing.T) {
	m := ValidateTablesExist(ValidateTablesExistRequest{
		SQL:                  "CALL nope();",
		StmtRanges:           GetStatementRanges("CALL nope();"),
		SessionDatabase:      "MYDB",
		SessionSchema:        "PUBLIC",
		KnownObjects:         []ObjectRef{{DB: "MYDB", Schema: "PUBLIC", Name: "MYTASK", Kind: "TASK"}},
		FetchedObjectSchemas: []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
	})
	if len(m) != 0 {
		t.Errorf("unseen kind (procedure) should be silent, got %+v", m)
	}
}

func TestValidateKindImplied_CreatedInScriptSilent(t *testing.T) {
	// A task created earlier in the script is not flagged when executed.
	sql := "CREATE TASK newtask SCHEDULE = '1 minute' AS SELECT 1;\nEXECUTE TASK newtask;"
	if m := kindImpliedMarkers(sql); len(m) != 0 {
		t.Errorf("task created in-script should be silent, got %+v", m)
	}
}

func TestValidateKindImplied_QuickFix(t *testing.T) {
	m := kindImpliedMarkers("CALL nope();")
	if len(m) == 0 {
		t.Fatalf("expected a marker for CALL nope(), got none")
	}
	// No same-named proc elsewhere → no quick-fix suggestions; just ensure the
	// code payload machinery does not misfire.
	if m[0].Code != "" && !strings.Contains(m[0].Code, "qualify-procedure") {
		t.Errorf("unexpected code payload: %q", m[0].Code)
	}
}
