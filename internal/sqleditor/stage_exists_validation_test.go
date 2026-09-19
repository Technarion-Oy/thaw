package sqleditor

import (
	"strings"
	"testing"
)

// stageMarkers runs ValidateTablesExist with a catalog containing one stage,
// MYDB.PUBLIC.MYSTAGE, and PUBLIC marked as fetched, so @stage existence checks
// actually fire.
func stageMarkers(sql string) []DiagMarker {
	return ValidateTablesExist(ValidateTablesExistRequest{
		SQL:             sql,
		StmtRanges:      GetStatementRanges(sql),
		KnownDatabases:  []string{"MYDB"},
		KnownSchemas:    []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
		KnownObjects: []ObjectRef{
			{DB: "MYDB", Schema: "PUBLIC", Name: "MYSTAGE", Kind: "STAGE"},
			{DB: "MYDB", Schema: "OTHER", Name: "OTHERSTAGE", Kind: "STAGE"},
		},
		FetchedObjectSchemas: []SchemaEntry{{DB: "MYDB", Name: "PUBLIC"}},
	})
}

func TestValidateStageRefs_ExistingStageSilent(t *testing.T) {
	ok := []string{
		"LIST @mystage;",
		"REMOVE @mystage/path/file.csv;",
		"COPY INTO t FROM @mystage;",
		"SELECT $1 FROM @mystage/f.csv;",
		"COPY INTO t FROM @mydb.public.mystage;",
		"SELECT $1 FROM @~/f.csv;",  // user stage always exists
		"COPY INTO t FROM @%t;",     // table stage — deferred, no flag
	}
	for _, sql := range ok {
		if m := stageMarkers(sql); len(m) != 0 {
			t.Errorf("expected no marker for %q, got %d: %+v", sql, len(m), m)
		}
	}
}

func TestValidateStageRefs_MissingStageFlagged(t *testing.T) {
	// NOTE: the `PUT file:///… @nope` repro is blocked by #700 (`//` in file://
	// is tokenized as a line comment, swallowing @nope). Covered once #700 lands.
	cases := []string{
		"LIST @nope;",
		"COPY INTO t FROM @nope/path/x.csv;",
		"SELECT $1 FROM @nope;",
	}
	for _, sql := range cases {
		m := stageMarkers(sql)
		if len(m) == 0 {
			t.Errorf("expected a missing-stage marker for %q, got none", sql)
			continue
		}
		if !strings.Contains(m[0].Message, "Stage 'nope' does not exist") {
			t.Errorf("for %q: unexpected message %q", sql, m[0].Message)
		}
	}
}

func TestValidateStageRefs_CreatedOrDroppedInScript(t *testing.T) {
	// Created earlier → silent.
	if m := stageMarkers("CREATE STAGE newstage;\nLIST @newstage;"); len(m) != 0 {
		t.Errorf("created-in-script stage should be silent, got %+v", m)
	}
	// Existing but dropped earlier → flagged.
	m := stageMarkers("DROP STAGE mystage;\nLIST @mystage;")
	if len(m) == 0 || !strings.Contains(m[0].Message, "Stage 'mystage'") {
		t.Errorf("dropped stage should be flagged, got %+v", m)
	}
}

func TestValidateStageRefs_UnfetchedSchemaSilent(t *testing.T) {
	// No FetchedObjectSchemas → we have no data for the schema, so stay silent
	// (the #709 shared-DB false-positive guard).
	m := ValidateTablesExist(ValidateTablesExistRequest{
		SQL:             "LIST @nope;",
		StmtRanges:      GetStatementRanges("LIST @nope;"),
		SessionDatabase: "MYDB",
		SessionSchema:   "PUBLIC",
		KnownObjects:    []ObjectRef{{DB: "MYDB", Schema: "PUBLIC", Name: "MYSTAGE", Kind: "STAGE"}},
		// FetchedObjectSchemas intentionally empty.
	})
	if len(m) != 0 {
		t.Errorf("unfetched schema should be silent, got %+v", m)
	}
}

func TestValidateStageRefs_QuickFixWhenInOtherSchema(t *testing.T) {
	// OTHERSTAGE lives in MYDB.OTHER; referenced unqualified it is missing from
	// the session schema but a qualify quick-fix should be offered.
	m := stageMarkers("LIST @otherstage;")
	if len(m) == 0 {
		t.Fatalf("expected a marker for @otherstage, got none")
	}
	if !strings.Contains(m[0].Code, "qualify-stage") || !strings.Contains(m[0].Code, "MYDB.OTHER.OTHERSTAGE") {
		t.Errorf("expected qualify-stage code payload, got %q", m[0].Code)
	}
}

func TestValidateStageRefs_AtInStringOrCommentIgnored(t *testing.T) {
	ok := []string{
		"SELECT 'has @nope inside a string';",
		"-- LIST @nope\nSELECT 1;",
		"SELECT $$ @nope in dollar quote $$;",
	}
	for _, sql := range ok {
		if m := stageMarkers(sql); len(m) != 0 {
			t.Errorf("@ inside string/comment should be ignored for %q, got %+v", sql, m)
		}
	}
}
