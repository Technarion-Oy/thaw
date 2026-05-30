// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

package column

import (
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func assertContains(t *testing.T, sql, substr string) {
	t.Helper()
	if !strings.Contains(sql, substr) {
		t.Errorf("expected SQL to contain %q\nSQL:\n%s", substr, sql)
	}
}

func assertNotContains(t *testing.T, sql, substr string) {
	t.Helper()
	if strings.Contains(sql, substr) {
		t.Errorf("expected SQL NOT to contain %q\nSQL:\n%s", substr, sql)
	}
}

func assertEqual(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("SQL mismatch\n got: %s\nwant: %s", got, want)
	}
}

// ── BuildAddColumnSql ─────────────────────────────────────────────────────────

func TestBuildAddColumnSql_Basic(t *testing.T) {
	cfg := AddColumnConfig{Name: "MY_COL", DataType: "VARCHAR", ValueMode: "none", ConstraintKind: "none"}
	sql, err := BuildAddColumnSql("DB", "SC", "T", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEqual(t, sql, `ALTER TABLE "DB"."SC"."T" ADD COLUMN MY_COL VARCHAR;`)
}

func TestBuildAddColumnSql_CaseSensitiveAndIfNotExists(t *testing.T) {
	cfg := AddColumnConfig{Name: "myCol", CaseSensitive: true, IfNotExists: true, DataType: "NUMBER(10,2)", ValueMode: "none", ConstraintKind: "none"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `ADD COLUMN IF NOT EXISTS "myCol" NUMBER(10,2)`)
}

func TestBuildAddColumnSql_PlaceholderName(t *testing.T) {
	cfg := AddColumnConfig{Name: "", DataType: "VARCHAR", ValueMode: "none", ConstraintKind: "none"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `ADD COLUMN column_name VARCHAR`)
}

func TestBuildAddColumnSql_DefaultValue(t *testing.T) {
	cfg := AddColumnConfig{Name: "C", DataType: "NUMBER", ValueMode: "default", DefaultValue: "0", ConstraintKind: "none"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `DEFAULT 0`)
}

func TestBuildAddColumnSql_DefaultEmptyDropsClause(t *testing.T) {
	cfg := AddColumnConfig{Name: "C", DataType: "NUMBER", ValueMode: "default", DefaultValue: "  ", ConstraintKind: "none"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertNotContains(t, sql, "DEFAULT")
}

func TestBuildAddColumnSql_Autoincrement(t *testing.T) {
	cfg := AddColumnConfig{Name: "ID", DataType: "NUMBER", ValueMode: "autoincrement", IdentityStart: 1, IdentityStep: 2, IdentityOrder: "ORDER", ConstraintKind: "none"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `AUTOINCREMENT (1, 2) ORDER`)
}

func TestBuildAddColumnSql_NotNullBeforeNamedConstraint(t *testing.T) {
	cfg := AddColumnConfig{Name: "C", DataType: "VARCHAR", ValueMode: "none", NotNull: true, ConstraintKind: "unique", ConstraintName: "uq_c"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `NOT NULL CONSTRAINT "uq_c" UNIQUE`)
	// NOT NULL must come before CONSTRAINT.
	if strings.Index(sql, "NOT NULL") > strings.Index(sql, "CONSTRAINT") {
		t.Errorf("NOT NULL should precede CONSTRAINT\nSQL: %s", sql)
	}
}

func TestBuildAddColumnSql_PrimaryKey(t *testing.T) {
	cfg := AddColumnConfig{Name: "ID", DataType: "NUMBER", ValueMode: "none", ConstraintKind: "primary_key"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `PRIMARY KEY`)
}

func TestBuildAddColumnSql_ForeignKeyWithColumn(t *testing.T) {
	cfg := AddColumnConfig{
		Name: "FK", DataType: "NUMBER", ValueMode: "none", ConstraintKind: "foreign_key",
		FkDb: "RDB", FkSchema: "RSC", FkTableName: "PARENT", FkColumn: "ID",
	}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `REFERENCES "RDB"."RSC"."PARENT" ("ID")`)
}

func TestBuildAddColumnSql_ForeignKeyDefaultsToCurrentDbSchema(t *testing.T) {
	cfg := AddColumnConfig{
		Name: "FK", DataType: "NUMBER", ValueMode: "none", ConstraintKind: "foreign_key",
		FkTableName: "PARENT",
	}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `REFERENCES "DB"."SC"."PARENT"`)
	assertNotContains(t, sql, `()`)
}

func TestBuildAddColumnSql_ForeignKeyWithoutTableDropsClause(t *testing.T) {
	cfg := AddColumnConfig{Name: "FK", DataType: "NUMBER", ValueMode: "none", ConstraintKind: "foreign_key"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertNotContains(t, sql, "REFERENCES")
}

func TestBuildAddColumnSql_CollationEscaped(t *testing.T) {
	cfg := AddColumnConfig{Name: "C", DataType: "VARCHAR", ValueMode: "none", ConstraintKind: "none", Collation: "en-ci"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `COLLATE 'en-ci'`)
}

// COLLATE must sit adjacent to the data type, before DEFAULT/AUTOINCREMENT and
// before the inline constraints, per Snowflake's column-definition grammar.
func TestBuildAddColumnSql_CollationOrdering(t *testing.T) {
	cfg := AddColumnConfig{
		Name: "C", DataType: "VARCHAR", ValueMode: "default", DefaultValue: "'x'",
		NotNull: true, ConstraintKind: "unique", Collation: "en-ci",
	}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	// Data type immediately followed by COLLATE.
	assertContains(t, sql, `VARCHAR COLLATE 'en-ci' DEFAULT 'x' NOT NULL UNIQUE`)

	collateIdx := strings.Index(sql, "COLLATE")
	for _, after := range []string{"DEFAULT", "NOT NULL", "UNIQUE"} {
		if idx := strings.Index(sql, after); idx < collateIdx {
			t.Errorf("COLLATE must precede %q\nSQL: %s", after, sql)
		}
	}
	// COLLATE must come right after the data type, not after the constraints.
	if collateIdx > strings.Index(sql, "DEFAULT") {
		t.Errorf("COLLATE should be adjacent to the data type\nSQL: %s", sql)
	}
}

func TestBuildAddColumnSql_CommentEscaped(t *testing.T) {
	cfg := AddColumnConfig{Name: "C", DataType: "VARCHAR", ValueMode: "none", ConstraintKind: "none", Comment: "it's here"}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `COMMENT 'it''s here'`)
}

func TestBuildAddColumnSql_Computed(t *testing.T) {
	cfg := AddColumnConfig{
		Name: "FULL", ValueMode: "computed", ComputedExpr: "a + b",
		DataType: "VARCHAR", // should be ignored
		NotNull:  true, ConstraintKind: "unique", Collation: "en-ci",
		Comment: "derived",
	}
	sql, _ := BuildAddColumnSql("DB", "SC", "T", cfg)
	assertContains(t, sql, `AS (a + b)`)
	assertNotContains(t, sql, "VARCHAR")
	// Constraints / collation are invalid for virtual columns.
	assertNotContains(t, sql, "NOT NULL")
	assertNotContains(t, sql, "UNIQUE")
	assertNotContains(t, sql, "COLLATE")
	// Comment is still valid on computed columns.
	assertContains(t, sql, `COMMENT 'derived'`)
}

// ── ALTER COLUMN builders ─────────────────────────────────────────────────────

func TestBuildDropColumnSql(t *testing.T) {
	assertEqual(t, BuildDropColumnSql("DB", "SC", "T", "C"),
		`ALTER TABLE "DB"."SC"."T" DROP COLUMN "C";`)
}

func TestBuildRenameColumnSql(t *testing.T) {
	assertEqual(t, BuildRenameColumnSql("DB", "SC", "T", "OLD", "NEW", false),
		`ALTER TABLE "DB"."SC"."T" RENAME COLUMN "OLD" TO NEW;`)
	assertEqual(t, BuildRenameColumnSql("DB", "SC", "T", "OLD", "newName", true),
		`ALTER TABLE "DB"."SC"."T" RENAME COLUMN "OLD" TO "newName";`)
}

func TestBuildSetDropNotNullSql(t *testing.T) {
	assertEqual(t, BuildSetNotNullSql("DB", "SC", "T", "C"),
		`ALTER TABLE "DB"."SC"."T" ALTER COLUMN "C" SET NOT NULL;`)
	assertEqual(t, BuildDropNotNullSql("DB", "SC", "T", "C"),
		`ALTER TABLE "DB"."SC"."T" ALTER COLUMN "C" DROP NOT NULL;`)
}

func TestBuildSetColumnCommentSql(t *testing.T) {
	assertEqual(t, BuildSetColumnCommentSql("DB", "SC", "T", "C", "hi"),
		`ALTER TABLE "DB"."SC"."T" ALTER COLUMN "C" COMMENT 'hi';`)
	assertEqual(t, BuildSetColumnCommentSql("DB", "SC", "T", "C", "it's"),
		`ALTER TABLE "DB"."SC"."T" ALTER COLUMN "C" COMMENT 'it''s';`)
	// Empty comment → UNSET.
	assertEqual(t, BuildSetColumnCommentSql("DB", "SC", "T", "C", "  "),
		`ALTER TABLE "DB"."SC"."T" ALTER COLUMN "C" UNSET COMMENT;`)
}

func TestBuildChangeDataTypeSql(t *testing.T) {
	assertEqual(t, BuildChangeDataTypeSql("DB", "SC", "T", "C", "  VARCHAR(20) "),
		`ALTER TABLE "DB"."SC"."T" ALTER COLUMN "C" SET DATA TYPE VARCHAR(20);`)
}
