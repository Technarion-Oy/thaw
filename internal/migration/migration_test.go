// SPDX-License-Identifier: GPL-3.0-or-later

package migration

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// ─── normalizeDDL ─────────────────────────────────────────────────────────────

func TestNormalizeDDL(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "block_comment_stripped",
			input: "CREATE /* a comment */ TABLE foo (id NUMBER)",
			want:  "CREATE TABLE FOO (ID NUMBER)",
		},
		{
			name:  "line_comment_stripped",
			input: "CREATE TABLE foo -- trailing comment\n(id NUMBER)",
			want:  "CREATE TABLE FOO (ID NUMBER)",
		},
		{
			name:  "mixed_comments",
			input: "/* block */ CREATE TABLE -- line\nfoo (id NUMBER)",
			want:  "CREATE TABLE FOO (ID NUMBER)",
		},
		{
			name:  "whitespace_collapsed",
			input: "CREATE   TABLE\t\tfoo   (  id   NUMBER  )",
			want:  "CREATE TABLE FOO ( ID NUMBER )",
		},
		{
			name:  "trailing_semicolon_removed",
			input: "CREATE TABLE foo (id NUMBER);",
			want:  "CREATE TABLE FOO (ID NUMBER)",
		},
		{
			name:  "trailing_semicolons_and_spaces",
			input: "SELECT 1 ; ; ",
			want:  "SELECT 1 ;",
		},
		{
			name:  "uppercased",
			input: "create table my_table (col varchar)",
			want:  "CREATE TABLE MY_TABLE (COL VARCHAR)",
		},
		{
			name:  "leading_trailing_whitespace",
			input: "   SELECT 1   ",
			want:  "SELECT 1",
		},
		{
			name:  "empty_string",
			input: "",
			want:  "",
		},
		{
			name:  "only_comment",
			input: "-- just a comment",
			want:  "",
		},
		{
			// The old non-greedy /\*.*?\*/ regex stopped at the first "*/",
			// leaking "OUTER */"; the tokenizer strips nested comments whole.
			name:  "nested_block_comment",
			input: "CREATE /* outer /* inner */ outer */ TABLE foo (id NUMBER)",
			want:  "CREATE TABLE FOO (ID NUMBER)",
		},
		{
			// A comment-like sequence inside a string literal must be preserved;
			// the old --[^\n]* regex would have eaten it.
			name:  "comment_marker_inside_string",
			input: "SELECT '-- not a comment'",
			want:  "SELECT '-- NOT A COMMENT'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeDDL(tc.input)
			if got != tc.want {
				t.Errorf("normalizeDDL(%q)\n  got  %q\n  want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ─── migrQuote ────────────────────────────────────────────────────────────────

func TestMigrQuote(t *testing.T) {
	cases := []struct {
		input string
		want  string
	}{
		{"FOO", `"FOO"`},
		{"my_table", `"my_table"`},
		{`has"quote`, `"has""quote"`},
		{`a""b`, `"a""""b"`},
		{"", `""`},
		{"My Table With Spaces", `"My Table With Spaces"`},
	}
	for _, tc := range cases {
		got := migrQuote(tc.input)
		if got != tc.want {
			t.Errorf("migrQuote(%q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

// ─── parseLocalTableColumns ───────────────────────────────────────────────────

func TestParseLocalTableColumns(t *testing.T) {
	cases := []struct {
		name string
		ddl  string
		want []colDef
	}{
		{
			name: "simple_table",
			ddl:  `CREATE TABLE t (id NUMBER, name VARCHAR(255))`,
			want: []colDef{{Name: "ID", TypeExpr: "NUMBER"}, {Name: "NAME", TypeExpr: "VARCHAR(255)"}},
		},
		{
			name: "complex_numeric_type",
			ddl:  `CREATE TABLE t (amount NUMBER(38,0), price FLOAT)`,
			want: []colDef{{Name: "AMOUNT", TypeExpr: "NUMBER(38,0)"}, {Name: "PRICE", TypeExpr: "FLOAT"}},
		},
		{
			name: "quoted_column_name",
			ddl:  `CREATE TABLE t ("MyCol" VARCHAR, "Count" NUMBER)`,
			want: []colDef{{Name: "MYCOL", TypeExpr: "VARCHAR"}, {Name: "COUNT", TypeExpr: "NUMBER"}},
		},
		{
			name: "column_with_default_and_not_null",
			ddl:  `CREATE TABLE t (id NUMBER NOT NULL DEFAULT 0, active BOOLEAN NOT NULL DEFAULT TRUE)`,
			want: []colDef{{Name: "ID", TypeExpr: "NUMBER"}, {Name: "ACTIVE", TypeExpr: "BOOLEAN"}},
		},
		{
			name: "constraint_clauses_skipped",
			ddl:  `CREATE TABLE t (id NUMBER, CONSTRAINT pk PRIMARY KEY (id), UNIQUE (id))`,
			want: []colDef{{Name: "ID", TypeExpr: "NUMBER"}},
		},
		{
			name: "primary_key_inline_skipped",
			ddl:  `CREATE TABLE t (id NUMBER, name VARCHAR, PRIMARY KEY (id))`,
			want: []colDef{{Name: "ID", TypeExpr: "NUMBER"}, {Name: "NAME", TypeExpr: "VARCHAR"}},
		},
		{
			name: "cluster_by_skipped",
			ddl:  `CREATE TABLE t (id NUMBER, ts TIMESTAMP_NTZ, CLUSTER BY (ts))`,
			want: []colDef{{Name: "ID", TypeExpr: "NUMBER"}, {Name: "TS", TypeExpr: "TIMESTAMP_NTZ"}},
		},
		{
			name: "variant_and_array_types",
			ddl:  `CREATE TABLE t (id NUMBER, tags ARRAY, meta VARIANT, doc OBJECT)`,
			want: []colDef{
				{Name: "ID", TypeExpr: "NUMBER"},
				{Name: "TAGS", TypeExpr: "ARRAY"},
				{Name: "META", TypeExpr: "VARIANT"},
				{Name: "DOC", TypeExpr: "OBJECT"},
			},
		},
		{
			name: "transient_table",
			ddl:  `CREATE OR REPLACE TRANSIENT TABLE mydb.myschema.events (id NUMBER, payload VARIANT)`,
			want: []colDef{{Name: "ID", TypeExpr: "NUMBER"}, {Name: "PAYLOAD", TypeExpr: "VARIANT"}},
		},
		{
			name: "no_parens_returns_nil",
			ddl:  `CREATE TABLE t`,
			want: nil,
		},
		{
			name: "empty_column_list",
			ddl:  `CREATE TABLE t ()`,
			want: nil,
		},
		{
			name: "dollar_sign_in_column_name",
			ddl:  `CREATE TABLE t (col$1 VARCHAR, _private NUMBER)`,
			want: []colDef{{Name: "COL$1", TypeExpr: "VARCHAR"}, {Name: "_PRIVATE", TypeExpr: "NUMBER"}},
		},
		{
			name: "timestamp_with_precision",
			ddl:  `CREATE TABLE t (created_at TIMESTAMP_NTZ(9), updated_at TIMESTAMP_LTZ(6))`,
			want: []colDef{
				{Name: "CREATED_AT", TypeExpr: "TIMESTAMP_NTZ(9)"},
				{Name: "UPDATED_AT", TypeExpr: "TIMESTAMP_LTZ(6)"},
			},
		},
		{
			name: "check_constraint_skipped",
			ddl:  `CREATE TABLE t (age NUMBER, CHECK (age >= 0))`,
			want: []colDef{{Name: "AGE", TypeExpr: "NUMBER"}},
		},
		{
			name: "foreign_key_skipped",
			ddl:  `CREATE TABLE t (id NUMBER, ref_id NUMBER, FOREIGN KEY (ref_id) REFERENCES other(id))`,
			want: []colDef{{Name: "ID", TypeExpr: "NUMBER"}, {Name: "REF_ID", TypeExpr: "NUMBER"}},
		},
		{
			// Bug fix: a ")" inside a string default used to prematurely close
			// the column-list scan (byte-level depth counting), dropping every
			// later column. The token scan treats the string as one token.
			name: "paren_inside_string_default",
			ddl:  `CREATE TABLE t (a VARCHAR DEFAULT ')', b NUMBER)`,
			want: []colDef{{Name: "A", TypeExpr: "VARCHAR"}, {Name: "B", TypeExpr: "NUMBER"}},
		},
		{
			// Bug fix: a comma inside a string default no longer splits a column.
			name: "comma_inside_string_default",
			ddl:  `CREATE TABLE t (label VARCHAR DEFAULT 'a,b', id NUMBER)`,
			want: []colDef{{Name: "LABEL", TypeExpr: "VARCHAR"}, {Name: "ID", TypeExpr: "NUMBER"}},
		},
		{
			// Bug fix: a quoted column name containing a space is captured whole
			// (the old regex stopped at the first space, yielding just "MY").
			name: "quoted_name_with_space",
			ddl:  `CREATE TABLE t ("My Col" VARCHAR, id NUMBER)`,
			want: []colDef{{Name: "MY COL", TypeExpr: "VARCHAR"}, {Name: "ID", TypeExpr: "NUMBER"}},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := parseLocalTableColumns(tc.ddl)
			if len(got) != len(tc.want) {
				t.Fatalf("parseLocalTableColumns: got %d cols %v, want %d cols %v", len(got), got, len(tc.want), tc.want)
			}
			for i, g := range got {
				w := tc.want[i]
				if g.Name != w.Name || g.TypeExpr != w.TypeExpr {
					t.Errorf("col[%d]: got {%q, %q}, want {%q, %q}", i, g.Name, g.TypeExpr, w.Name, w.TypeExpr)
				}
			}
		})
	}
}

// ─── commonColumnNames ────────────────────────────────────────────────────────

func TestCommonColumnNames(t *testing.T) {
	mkCols := func(names ...string) []colDef {
		cols := make([]colDef, len(names))
		for i, n := range names {
			cols[i] = colDef{Name: n}
		}
		return cols
	}

	cases := []struct {
		name string
		a, b []colDef
		want []string
	}{
		{
			name: "intersection",
			a:    mkCols("A", "B", "C"),
			b:    mkCols("B", "C", "D"),
			want: []string{"B", "C"},
		},
		{
			name: "a_empty",
			a:    nil,
			b:    mkCols("A", "B"),
			want: nil,
		},
		{
			name: "b_empty",
			a:    mkCols("A", "B"),
			b:    nil,
			want: nil,
		},
		{
			name: "no_overlap",
			a:    mkCols("A", "B"),
			b:    mkCols("C", "D"),
			want: nil,
		},
		{
			name: "full_overlap",
			a:    mkCols("A", "B", "C"),
			b:    mkCols("C", "B", "A"),
			want: []string{"A", "B", "C"},
		},
		{
			name: "order_preserved_from_a",
			a:    mkCols("Z", "A", "M"),
			b:    mkCols("A", "M", "Z"),
			want: []string{"Z", "A", "M"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := commonColumnNames(tc.a, tc.b)
			if len(got) != len(tc.want) {
				t.Fatalf("commonColumnNames: got %v, want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("commonColumnNames[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// ─── replaceDDLTableName ─────────────────────────────────────────────────────

func TestReplaceDDLTableName(t *testing.T) {
	cases := []struct {
		name    string
		ddl     string
		db      string
		schema  string
		newName string
		want    string
	}{
		{
			name:    "simple_unqualified",
			ddl:     `CREATE TABLE old_name (id NUMBER)`,
			db:      "DB1",
			schema:  "SCH1",
			newName: "new_name",
			want:    `CREATE TABLE "DB1"."SCH1"."new_name" (id NUMBER)`,
		},
		{
			name:    "three_part_qualified",
			ddl:     `CREATE TABLE mydb.myschema.old_name (id NUMBER)`,
			db:      "DB2",
			schema:  "SCH2",
			newName: "new_name",
			want:    `CREATE TABLE "DB2"."SCH2"."new_name" (id NUMBER)`,
		},
		{
			name:    "quoted_identifier",
			ddl:     `CREATE TABLE "MY_DB"."MY_SCH"."OLD_NAME" (id NUMBER)`,
			db:      "DB3",
			schema:  "SCH3",
			newName: "new_name",
			want:    `CREATE TABLE "DB3"."SCH3"."new_name" (id NUMBER)`,
		},
		{
			name:    "create_or_replace_transient",
			ddl:     `CREATE OR REPLACE TRANSIENT TABLE old_name (id NUMBER, name VARCHAR)`,
			db:      "DB4",
			schema:  "SCH4",
			newName: "tmp_table",
			want:    `CREATE OR REPLACE TRANSIENT TABLE "DB4"."SCH4"."tmp_table" (id NUMBER, name VARCHAR)`,
		},
		{
			name:    "create_or_replace",
			ddl:     `CREATE OR REPLACE TABLE old_name (col TEXT)`,
			db:      "MYDB",
			schema:  "PUBLIC",
			newName: "new_table",
			want:    `CREATE OR REPLACE TABLE "MYDB"."PUBLIC"."new_table" (col TEXT)`,
		},
		{
			name:    "no_parens_unchanged",
			ddl:     `CREATE TABLE no_parens`,
			db:      "DB",
			schema:  "SCH",
			newName: "x",
			want:    `CREATE TABLE no_parens`,
		},
		{
			name:    "newname_with_special_chars",
			ddl:     `CREATE TABLE t (id NUMBER)`,
			db:      `my"db`,
			schema:  "PUBLIC",
			newName: `table"name`,
			want:    `CREATE TABLE "my""db"."PUBLIC"."table""name" (id NUMBER)`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := replaceDDLTableName(tc.ddl, tc.db, tc.schema, tc.newName)
			if got != tc.want {
				t.Errorf("replaceDDLTableName:\n  got  %q\n  want %q", got, tc.want)
			}
		})
	}
}

// ─── isDependencyError ────────────────────────────────────────────────────────

func TestIsDependencyError(t *testing.T) {
	cases := []struct {
		msg  string
		want bool
	}{
		{"Object 'FOO.BAR.BAZ' does not exist or not authorized.", true},
		{"Table 'MY_TABLE' does not exist or not authorized.", true},
		{"View 'MY_VIEW' does not exist or not authorized.", true},
		{"002003 (42S02): SQL compilation error: Object 'X' does not exist or not authorized.", true},
		{"not authorized to access the table.", true},
		{"Does Not Exist (case insensitive check)", true},
		{"SQL compilation error: syntax error.", false},
		{"division by zero", false},
		{"", false},
	}
	for _, tc := range cases {
		got := isDependencyError(tc.msg)
		if got != tc.want {
			t.Errorf("isDependencyError(%q) = %v, want %v", tc.msg, got, tc.want)
		}
	}
}

// ─── executionPriority ────────────────────────────────────────────────────────

func TestExecutionPriority(t *testing.T) {
	expected := map[string]int{
		"DATABASE":          0,
		"SCHEMA":            1,
		"SEQUENCE":          2,
		"TABLE":             3,
		"FILE FORMAT":       4,
		"STAGE":             5,
		"VIEW":              6,
		"MATERIALIZED VIEW": 7,
		"FUNCTION":          8,
		"PROCEDURE":         9,
		"STREAM":            10,
		"TASK":              11,
		"PIPE":              12,
		"UNKNOWN_KIND":      99,
	}
	for kind, want := range expected {
		got := executionPriority(kind)
		if got != want {
			t.Errorf("executionPriority(%q) = %d, want %d", kind, got, want)
		}
	}

	if executionPriority("table") != executionPriority("TABLE") {
		t.Error("executionPriority should be case-insensitive")
	}
	if executionPriority("view") != executionPriority("VIEW") {
		t.Error("executionPriority should be case-insensitive")
	}

	if !(executionPriority("DATABASE") < executionPriority("SCHEMA") &&
		executionPriority("SCHEMA") < executionPriority("TABLE") &&
		executionPriority("TABLE") < executionPriority("VIEW") &&
		executionPriority("VIEW") < executionPriority("FUNCTION") &&
		executionPriority("FUNCTION") < executionPriority("PROCEDURE") &&
		executionPriority("PROCEDURE") < executionPriority("STREAM") &&
		executionPriority("STREAM") < executionPriority("TASK") &&
		executionPriority("TASK") < executionPriority("PIPE")) {
		t.Error("executionPriority ordering is wrong")
	}
}

// ─── remoteKey ────────────────────────────────────────────────────────────────

func TestRemoteKey(t *testing.T) {
	cases := []struct {
		db, schema, kind, name, argSig string
		want                           string
	}{
		{
			db:     "mydb",
			schema: "PUBLIC",
			kind:   "table",
			name:   "orders",
			argSig: "",
			want:   "MYDB\x00PUBLIC\x00TABLE\x00ORDERS\x00",
		},
		{
			db:     "DB",
			schema: "SCH",
			kind:   "FUNCTION",
			name:   "MY_FUNC",
			argSig: "(NUMBER)",
			want:   "DB\x00SCH\x00FUNCTION\x00MY_FUNC\x00(NUMBER)",
		},
		{
			db:     "Db",
			schema: "sCh",
			kind:   "View",
			name:   "Vw",
			argSig: "",
			want:   "DB\x00SCH\x00VIEW\x00VW\x00",
		},
	}
	for _, tc := range cases {
		got := remoteKey(tc.db, tc.schema, tc.kind, tc.name, tc.argSig)
		if got != tc.want {
			t.Errorf("remoteKey(%q,%q,%q,%q,%q)\n  got  %q\n  want %q",
				tc.db, tc.schema, tc.kind, tc.name, tc.argSig, got, tc.want)
		}
	}
}

// ─── buildMigrationScript ────────────────────────────────────────────────────

func TestBuildMigrationScript(t *testing.T) {
	newTableItem := MigrationDiffItem{
		Object: MigrationObject{
			ObjectKind: "TABLE",
			ObjectName: "ORDERS",
			Database:   "MYDB",
			Schema:     "PUBLIC",
			DDL:        `CREATE TABLE "MYDB"."PUBLIC"."ORDERS" (id NUMBER, amount NUMBER(10,2))`,
		},
		Status:   "new",
		LocalDDL: `CREATE TABLE "MYDB"."PUBLIC"."ORDERS" (id NUMBER, amount NUMBER(10,2))`,
	}

	changedTableLocal := `CREATE TABLE "MYDB"."PUBLIC"."ORDERS" (id NUMBER, status VARCHAR(50))`
	changedTableRemote := `CREATE TABLE "MYDB"."PUBLIC"."ORDERS" (id NUMBER, amount NUMBER(10,2))`
	changedTableItem := MigrationDiffItem{
		Object: MigrationObject{
			ObjectKind: "TABLE",
			ObjectName: "ORDERS",
			Database:   "MYDB",
			Schema:     "PUBLIC",
			DDL:        changedTableLocal,
		},
		Status:    "changed",
		LocalDDL:  changedTableLocal,
		RemoteDDL: changedTableRemote,
	}

	unchangedItem := MigrationDiffItem{
		Object: MigrationObject{
			ObjectKind: "TABLE",
			ObjectName: "IGNORED",
			Database:   "MYDB",
			Schema:     "PUBLIC",
		},
		Status: "unchanged",
	}
	removedItem := MigrationDiffItem{
		Object: MigrationObject{
			ObjectKind: "TABLE",
			ObjectName: "DROPPED",
			Database:   "MYDB",
			Schema:     "PUBLIC",
		},
		Status: "removed",
	}

	t.Run("new_table_emits_ddl_directly", func(t *testing.T) {
		script := buildMigrationScript([]MigrationDiffItem{newTableItem}, "MYDB", StrategyInPlace)
		if !strings.Contains(script, `CREATE TABLE "MYDB"."PUBLIC"."ORDERS"`) {
			t.Errorf("expected DDL in script, got:\n%s", script)
		}
	})

	t.Run("unchanged_and_removed_excluded", func(t *testing.T) {
		script := buildMigrationScript([]MigrationDiffItem{unchangedItem, removedItem}, "MYDB", StrategyInPlace)
		if strings.Contains(script, "IGNORED") || strings.Contains(script, "DROPPED") {
			t.Errorf("unchanged/removed items should not appear in script:\n%s", script)
		}
	})

	t.Run("in_place_emits_alter_statements", func(t *testing.T) {
		script := buildMigrationScript([]MigrationDiffItem{changedTableItem}, "MYDB", StrategyInPlace)
		if !strings.Contains(script, "ADD COLUMN") && !strings.Contains(script, "DROP COLUMN") {
			t.Errorf("expected ALTER TABLE statements in in_place script:\n%s", script)
		}
		if !strings.Contains(script, `"STATUS"`) {
			t.Errorf("expected STATUS column in ALTER statement:\n%s", script)
		}
		if !strings.Contains(script, `"AMOUNT"`) {
			t.Errorf("expected AMOUNT drop in ALTER statement:\n%s", script)
		}
	})

	t.Run("blue_green_swap_emits_swap_pattern", func(t *testing.T) {
		script := buildMigrationScript([]MigrationDiffItem{changedTableItem}, "MYDB", StrategyBlueGreenSwap)
		if !strings.Contains(script, "SWAP WITH") {
			t.Errorf("expected SWAP WITH in blue_green script:\n%s", script)
		}
		if !strings.Contains(script, "__migration_tmp") {
			t.Errorf("expected temp table name in blue_green script:\n%s", script)
		}
		if !strings.Contains(script, "DROP TABLE IF EXISTS") {
			t.Errorf("expected DROP TABLE IF EXISTS in blue_green script:\n%s", script)
		}
	})

	t.Run("view_abstraction_emits_rename_and_view", func(t *testing.T) {
		script := buildMigrationScript([]MigrationDiffItem{changedTableItem}, "MYDB", StrategyViewAbstraction)
		if !strings.Contains(script, "RENAME TO") {
			t.Errorf("expected RENAME TO in view_abstraction script:\n%s", script)
		}
		if !strings.Contains(script, "ORDERS_v1") {
			t.Errorf("expected archive table _v1 in view_abstraction script:\n%s", script)
		}
		if !strings.Contains(script, "CREATE OR REPLACE VIEW") {
			t.Errorf("expected compat view in view_abstraction script:\n%s", script)
		}
		if !strings.Contains(script, "ORDERS_compat") {
			t.Errorf("expected _compat view in view_abstraction script:\n%s", script)
		}
	})

	t.Run("destructive_rebuild_emits_drop_create", func(t *testing.T) {
		script := buildMigrationScript([]MigrationDiffItem{changedTableItem}, "MYDB", StrategyDestructiveRebuild)
		if !strings.Contains(script, "DROP TABLE IF EXISTS") {
			t.Errorf("expected DROP TABLE IF EXISTS in destructive script:\n%s", script)
		}
		if !strings.Contains(script, `CREATE TABLE "MYDB"."PUBLIC"."ORDERS"`) {
			t.Errorf("expected CREATE TABLE in destructive script:\n%s", script)
		}
	})

	t.Run("sorted_by_execution_priority", func(t *testing.T) {
		viewItem := MigrationDiffItem{
			Object: MigrationObject{
				ObjectKind: "VIEW",
				ObjectName: "MY_VIEW",
				Database:   "MYDB",
				Schema:     "PUBLIC",
				DDL:        `CREATE OR REPLACE VIEW "MYDB"."PUBLIC"."MY_VIEW" AS SELECT 1`,
			},
			Status:   "new",
			LocalDDL: `CREATE OR REPLACE VIEW "MYDB"."PUBLIC"."MY_VIEW" AS SELECT 1`,
		}
		items := []MigrationDiffItem{viewItem, newTableItem}
		script := buildMigrationScript(items, "MYDB", StrategyInPlace)
		tablePos := strings.Index(script, "ORDERS")
		viewPos := strings.Index(script, "MY_VIEW")
		if tablePos < 0 || viewPos < 0 {
			t.Fatalf("expected both ORDERS and MY_VIEW in script:\n%s", script)
		}
		if tablePos > viewPos {
			t.Errorf("TABLE should appear before VIEW (lower priority), but got tablePos=%d, viewPos=%d\nScript:\n%s",
				tablePos, viewPos, script)
		}
	})

	t.Run("use_database_emitted", func(t *testing.T) {
		script := buildMigrationScript([]MigrationDiffItem{newTableItem}, "MYDB", StrategyInPlace)
		if !strings.Contains(script, `USE DATABASE "MYDB"`) {
			t.Errorf("expected USE DATABASE in script:\n%s", script)
		}
	})
}

// ─── ScanSource ──────────────────────────────────────────────────────────────

func sortMigObjs(objs []MigrationObject) {
	sort.Slice(objs, func(i, j int) bool {
		ki := objs[i].Database + objs[i].Schema + objs[i].ObjectKind + objs[i].ObjectName
		kj := objs[j].Database + objs[j].Schema + objs[j].ObjectKind + objs[j].ObjectName
		return ki < kj
	})
}

func TestScanSource(t *testing.T) {
	svc := NewService(func(string, interface{}) {}) // no-op emitter

	t.Run("empty_dir_returns_empty_slice", func(t *testing.T) {
		dir := t.TempDir()
		objs, err := svc.ScanSource(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(objs) != 0 {
			t.Errorf("expected 0 objects, got %d: %v", len(objs), objs)
		}
	})

	t.Run("non_sql_files_ignored", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# README"), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("key: val"), 0644); err != nil {
			t.Fatal(err)
		}
		objs, err := svc.ScanSource(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(objs) != 0 {
			t.Errorf("expected 0 objects, got %d", len(objs))
		}
	})

	t.Run("table_and_view_discovered", func(t *testing.T) {
		dir := t.TempDir()
		sql := `USE DATABASE MYDB;
USE SCHEMA MYSCHEMA;
CREATE TABLE orders (id NUMBER, amount NUMBER(10,2));
CREATE OR REPLACE VIEW order_summary AS SELECT id FROM orders;`
		if err := os.WriteFile(filepath.Join(dir, "schema.sql"), []byte(sql), 0644); err != nil {
			t.Fatal(err)
		}
		objs, err := svc.ScanSource(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(objs) != 2 {
			t.Fatalf("expected 2 objects, got %d: %v", len(objs), objs)
		}
		sortMigObjs(objs)
		table := objs[0]
		if strings.ToUpper(table.ObjectKind) != "TABLE" || strings.ToUpper(table.ObjectName) != "ORDERS" {
			t.Errorf("expected ORDERS TABLE, got %+v", table)
		}
		if table.Database != "MYDB" || table.Schema != "MYSCHEMA" {
			t.Errorf("USE context not applied: got db=%q schema=%q", table.Database, table.Schema)
		}

		view := objs[1]
		if strings.ToUpper(view.ObjectKind) != "VIEW" || strings.ToUpper(view.ObjectName) != "ORDER_SUMMARY" {
			t.Errorf("expected ORDER_SUMMARY VIEW, got %+v", view)
		}
		if !view.IsReplace {
			t.Errorf("IsReplace should be true for CREATE OR REPLACE VIEW")
		}
	})

	t.Run("deduplication_last_definition_wins", func(t *testing.T) {
		dir := t.TempDir()
		sqlA := `USE DATABASE DB;
USE SCHEMA SCH;
CREATE TABLE orders (id NUMBER);`
		sqlB := `USE DATABASE DB;
USE SCHEMA SCH;
CREATE TABLE orders (id NUMBER, name VARCHAR(100));`
		if err := os.WriteFile(filepath.Join(dir, "a_orders.sql"), []byte(sqlA), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "b_orders.sql"), []byte(sqlB), 0644); err != nil {
			t.Fatal(err)
		}
		objs, err := svc.ScanSource(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(objs) != 1 {
			t.Fatalf("expected 1 object after dedup, got %d: %v", len(objs), objs)
		}
		if !strings.Contains(objs[0].DDL, "name") && !strings.Contains(strings.ToUpper(objs[0].DDL), "NAME") {
			t.Errorf("last definition should win; expected DDL with 'name' column, got: %q", objs[0].DDL)
		}
	})

	t.Run("use_context_resets_on_new_database", func(t *testing.T) {
		dir := t.TempDir()
		sql := `USE DATABASE DB1;
USE SCHEMA SCH1;
CREATE TABLE table1 (id NUMBER);
USE DATABASE DB2;
CREATE TABLE table2 (id NUMBER);`
		if err := os.WriteFile(filepath.Join(dir, "multi.sql"), []byte(sql), 0644); err != nil {
			t.Fatal(err)
		}
		objs, err := svc.ScanSource(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		sortMigObjs(objs)
		var t1, t2 MigrationObject
		for _, o := range objs {
			if strings.ToUpper(o.ObjectName) == "TABLE1" {
				t1 = o
			}
			if strings.ToUpper(o.ObjectName) == "TABLE2" {
				t2 = o
			}
		}
		if t1.Database != "DB1" || t1.Schema != "SCH1" {
			t.Errorf("table1 got db=%q schema=%q, want DB1 SCH1", t1.Database, t1.Schema)
		}
		if t2.Database != "DB2" || t2.Schema != "" {
			t.Errorf("table2 got db=%q schema=%q, want DB2 (empty schema)", t2.Database, t2.Schema)
		}
	})

	t.Run("multiple_files_combined", func(t *testing.T) {
		dir := t.TempDir()
		sqlTables := `USE DATABASE PROD;
USE SCHEMA PUBLIC;
CREATE TABLE customers (id NUMBER, email VARCHAR);
CREATE TABLE products (id NUMBER, price NUMBER(10,2));`
		sqlViews := `USE DATABASE PROD;
USE SCHEMA PUBLIC;
CREATE OR REPLACE VIEW active_customers AS SELECT id FROM customers;`
		if err := os.WriteFile(filepath.Join(dir, "tables.sql"), []byte(sqlTables), 0644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "views.sql"), []byte(sqlViews), 0644); err != nil {
			t.Fatal(err)
		}
		objs, err := svc.ScanSource(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(objs) != 3 {
			t.Fatalf("expected 3 objects, got %d: %v", len(objs), objs)
		}
		for _, o := range objs {
			if o.Database != "PROD" || o.Schema != "PUBLIC" {
				t.Errorf("object %q has wrong context: db=%q schema=%q", o.ObjectName, o.Database, o.Schema)
			}
		}
	})
}
