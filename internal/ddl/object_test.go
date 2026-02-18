package ddl

import (
	"path/filepath"
	"sync"
	"testing"
)

// ─── Parse ────────────────────────────────────────────────────────────────────

func TestParse_Kinds(t *testing.T) {
	tests := []struct {
		name      string
		sql       string
		wantKind  Kind
		wantDB    string
		wantSch   string
		wantName  string
		wantArgSig string
	}{
		// ── DATABASE ────────────────────────────────────────────────────────
		{
			name:     "database quoted",
			sql:      `create or replace database "MY_DB"`,
			wantKind: KindDatabase, wantName: "MY_DB",
		},
		{
			name:     "database unquoted",
			sql:      `CREATE OR REPLACE DATABASE MY_DB`,
			wantKind: KindDatabase, wantName: "MY_DB",
		},

		// ── SCHEMA ──────────────────────────────────────────────────────────
		{
			name:     "schema two-part",
			sql:      `create or replace schema "MY_DB"."PUBLIC"`,
			wantKind: KindSchema, wantSch: "MY_DB", wantName: "PUBLIC",
		},
		{
			name:     "schema one-part unquoted",
			sql:      `create schema PUBLIC`,
			wantKind: KindSchema, wantName: "PUBLIC",
		},

		// ── TABLE ───────────────────────────────────────────────────────────
		{
			name:     "table three-part fully-qualified",
			sql:      `CREATE OR REPLACE TABLE "DB"."SCH"."TBL" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "TBL",
		},
		{
			name:     "table two-part",
			sql:      `CREATE TABLE "PUBLIC"."MY_TABLE" (id INT)`,
			wantKind: KindTable, wantSch: "PUBLIC", wantName: "MY_TABLE",
		},
		{
			name:     "table one-part unquoted",
			sql:      `CREATE TABLE orders (id INT)`,
			wantKind: KindTable, wantName: "orders",
		},
		{
			name:     "transient table modifier",
			sql:      `CREATE OR REPLACE TRANSIENT TABLE t (id INT)`,
			wantKind: KindTable, wantName: "t",
		},
		{
			name:     "temporary table modifier",
			sql:      `CREATE OR REPLACE TEMPORARY TABLE t (id INT)`,
			wantKind: KindTable, wantName: "t",
		},
		{
			name:     "external table modifier",
			sql:      `CREATE OR REPLACE EXTERNAL TABLE "DB"."SCH"."EXT" LOCATION=@s`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "EXT",
		},
		{
			name:     "dynamic table modifier",
			sql:      `CREATE OR REPLACE DYNAMIC TABLE "DB"."SCH"."DYN" TARGET_LAG='1 minute' AS SELECT 1`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "DYN",
		},

		// ── VIEW ────────────────────────────────────────────────────────────
		{
			name:     "view three-part",
			sql:      `CREATE OR REPLACE VIEW "DB"."SCH"."VW" AS SELECT 1`,
			wantKind: KindView, wantDB: "DB", wantSch: "SCH", wantName: "VW",
		},
		{
			name:     "secure view modifier",
			sql:      `CREATE OR REPLACE SECURE VIEW v AS SELECT 1`,
			wantKind: KindView, wantName: "v",
		},
		{
			name:     "secure recursive view modifiers",
			sql:      `CREATE OR REPLACE SECURE RECURSIVE VIEW v AS SELECT 1`,
			wantKind: KindView, wantName: "v",
		},
		{
			name:     "materialized view modifier",
			sql:      `CREATE OR REPLACE MATERIALIZED VIEW mv AS SELECT 1`,
			wantKind: KindView, wantName: "mv",
		},

		// ── FUNCTION ────────────────────────────────────────────────────────
		{
			name:      "function three-part with args",
			sql:       `CREATE OR REPLACE FUNCTION "DB"."SCH"."F"(X FLOAT) RETURNS FLOAT AS $$ return X; $$`,
			wantKind:  KindFunction, wantDB: "DB", wantSch: "SCH", wantName: "F",
			wantArgSig: "FLOAT",
		},
		{
			name:      "function no arguments",
			sql:       `CREATE FUNCTION f() RETURNS INT AS $$ return 1; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "noargs",
		},
		{
			name:      "function two args",
			sql:       `CREATE FUNCTION f(A FLOAT, B VARCHAR) RETURNS FLOAT AS $$ return 0; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "FLOAT_VARCHAR",
		},
		{
			name:      "function arg with size qualifier stripped",
			sql:       `CREATE FUNCTION f(X VARCHAR(256)) RETURNS VARCHAR AS $$ return X; $$`,
			wantKind:  KindFunction, wantName: "f",
			wantArgSig: "VARCHAR",
		},
		{
			name:      "function secure modifier",
			sql:       `CREATE OR REPLACE SECURE FUNCTION "DB"."SCH"."F"(X NUMBER) RETURNS NUMBER AS $$ return X; $$`,
			wantKind:  KindFunction, wantDB: "DB", wantSch: "SCH", wantName: "F",
			wantArgSig: "NUMBER",
		},

		// ── PROCEDURE ───────────────────────────────────────────────────────
		{
			name:      "procedure three-part",
			sql:       `CREATE OR REPLACE PROCEDURE "DB"."SCH"."P"(N NUMBER) RETURNS STRING AS $$ return ''; $$`,
			wantKind:  KindProcedure, wantDB: "DB", wantSch: "SCH", wantName: "P",
			wantArgSig: "NUMBER",
		},
		{
			name:      "procedure no args",
			sql:       `CREATE PROCEDURE do_thing() RETURNS VARCHAR AS $$ return ''; $$`,
			wantKind:  KindProcedure, wantName: "do_thing",
			wantArgSig: "noargs",
		},

		// ── SEQUENCE ────────────────────────────────────────────────────────
		{
			name:     "sequence three-part",
			sql:      `CREATE OR REPLACE SEQUENCE "DB"."SCH"."SEQ" START 1 INCREMENT 1`,
			wantKind: KindSequence, wantDB: "DB", wantSch: "SCH", wantName: "SEQ",
		},

		// ── STAGE ───────────────────────────────────────────────────────────
		{
			name:     "stage three-part",
			sql:      `CREATE OR REPLACE STAGE "DB"."SCH"."STG" URL='s3://bucket/'`,
			wantKind: KindStage, wantDB: "DB", wantSch: "SCH", wantName: "STG",
		},

		// ── STREAM ──────────────────────────────────────────────────────────
		{
			name:     "stream three-part",
			sql:      `CREATE OR REPLACE STREAM "DB"."SCH"."STR" ON TABLE t`,
			wantKind: KindStream, wantDB: "DB", wantSch: "SCH", wantName: "STR",
		},

		// ── TASK ────────────────────────────────────────────────────────────
		{
			name:     "task three-part",
			sql:      `CREATE OR REPLACE TASK "DB"."SCH"."TSK" WAREHOUSE=wh AS SELECT 1`,
			wantKind: KindTask, wantDB: "DB", wantSch: "SCH", wantName: "TSK",
		},

		// ── FILE FORMAT ─────────────────────────────────────────────────────
		{
			name:     "file format three-part",
			sql:      `CREATE OR REPLACE FILE FORMAT "DB"."SCH"."FF" TYPE='CSV'`,
			wantKind: KindFileFormat, wantDB: "DB", wantSch: "SCH", wantName: "FF",
		},

		// ── PIPE ────────────────────────────────────────────────────────────
		{
			name:     "pipe three-part",
			sql:      `CREATE OR REPLACE PIPE "DB"."SCH"."PP" AS COPY INTO t FROM @s`,
			wantKind: KindPipe, wantDB: "DB", wantSch: "SCH", wantName: "PP",
		},

		// ── IF NOT EXISTS ────────────────────────────────────────────────────
		{
			name:     "IF NOT EXISTS is skipped",
			sql:      `CREATE TABLE IF NOT EXISTS "DB"."SCH"."T" (id INT)`,
			wantKind: KindTable, wantDB: "DB", wantSch: "SCH", wantName: "T",
		},

		// ── case insensitivity ───────────────────────────────────────────────
		{
			name:     "lowercase create table",
			sql:      `create or replace table "PUBLIC"."my_table" (id int)`,
			wantKind: KindTable, wantSch: "PUBLIC", wantName: "my_table",
		},

		// ── quoted identifier with embedded double-quote ─────────────────────
		{
			name:     `double-quote escape in table name`,
			sql:      `CREATE TABLE "MY""TABLE" (id INT)`,
			wantKind: KindTable, wantName: `MY"TABLE`,
		},

		// ── non-CREATE / unknown statements ──────────────────────────────────
		{
			name:     "SELECT is unknown",
			sql:      "SELECT 1",
			wantKind: KindUnknown,
		},
		{
			name:     "DROP TABLE is unknown",
			sql:      "DROP TABLE t",
			wantKind: KindUnknown,
		},
		{
			name:     "ALTER TABLE is unknown",
			sql:      "ALTER TABLE t ADD COLUMN x INT",
			wantKind: KindUnknown,
		},
		{
			name:     "USE DATABASE is unknown",
			sql:      "USE DATABASE MY_DB",
			wantKind: KindUnknown,
		},
		{
			name:     "empty string is unknown",
			sql:      "",
			wantKind: KindUnknown,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.sql)

			if got.Kind != tt.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tt.wantKind)
			}
			if got.Database != tt.wantDB {
				t.Errorf("Database = %q, want %q", got.Database, tt.wantDB)
			}
			if got.Schema != tt.wantSch {
				t.Errorf("Schema = %q, want %q", got.Schema, tt.wantSch)
			}
			if got.Name != tt.wantName {
				t.Errorf("Name = %q, want %q", got.Name, tt.wantName)
			}
			if got.ArgSig != tt.wantArgSig {
				t.Errorf("ArgSig = %q, want %q", got.ArgSig, tt.wantArgSig)
			}
			if got.SQL != tt.sql {
				t.Errorf("SQL not preserved:\ngot:  %q\nwant: %q", got.SQL, tt.sql)
			}
		})
	}
}

// ─── parseArgSig ─────────────────────────────────────────────────────────────

func TestParseArgSig(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"()", "noargs"},
		{"(  )", "noargs"},
		{"(X FLOAT)", "FLOAT"},
		{"(X FLOAT, Y VARCHAR)", "FLOAT_VARCHAR"},
		{"(X FLOAT, Y VARCHAR, Z NUMBER)", "FLOAT_VARCHAR_NUMBER"},
		// Size qualifiers stripped.
		{"(X VARCHAR(256))", "VARCHAR"},
		{"(X NUMBER(38,0))", "NUMBER"},
		// Positional params (type only, no name).
		{"(FLOAT)", "FLOAT"},
		// No opening paren → empty.
		{"FLOAT", ""},
		{"", ""},
		// Unclosed paren → empty.
		{"(FLOAT", ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := parseArgSig(tt.input); got != tt.want {
				t.Errorf("parseArgSig(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── sanitize ────────────────────────────────────────────────────────────────

func TestSanitize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"MY_TABLE", "MY_TABLE"},
		{"my-table", "my-table"},
		{"MY TABLE", "MY_TABLE"},    // space → underscore
		{"MY.TABLE", "MY_TABLE"},    // dot → underscore
		{"MY\"TABLE", "MY_TABLE"},   // quote → underscore
		{"schema;name", "schema_name"},
		{"", ""},
		{"abc123", "abc123"},
		{"MY__TABLE", "MY__TABLE"},  // double underscore preserved
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := sanitize(tt.input); got != tt.want {
				t.Errorf("sanitize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// ─── FilePath ────────────────────────────────────────────────────────────────

func TestFilePath(t *testing.T) {
	fp := func(parts ...string) string { return filepath.Join(parts...) }

	tests := []struct {
		name string
		obj  Object
		want string
	}{
		// DATABASE always maps to the root sentinel file.
		{
			name: "database",
			obj:  Object{Kind: KindDatabase, Name: "MY_DB"},
			want: "_database.sql",
		},
		// SCHEMA goes into the schemas/ directory.
		{
			name: "schema",
			obj:  Object{Kind: KindSchema, Name: "PUBLIC"},
			want: fp("schemas", "PUBLIC.sql"),
		},
		{
			name: "schema with special chars sanitized",
			obj:  Object{Kind: KindSchema, Name: "MY SCHEMA"},
			want: fp("schemas", "MY_SCHEMA.sql"),
		},
		// Regular objects: <schema>/<dir>/<name>.sql
		{
			name: "table",
			obj:  Object{Kind: KindTable, Schema: "PUBLIC", Name: "MY_TABLE"},
			want: fp("PUBLIC", "tables", "MY_TABLE.sql"),
		},
		{
			name: "view",
			obj:  Object{Kind: KindView, Schema: "PUBLIC", Name: "MY_VIEW"},
			want: fp("PUBLIC", "views", "MY_VIEW.sql"),
		},
		{
			name: "sequence",
			obj:  Object{Kind: KindSequence, Schema: "PUBLIC", Name: "MY_SEQ"},
			want: fp("PUBLIC", "sequences", "MY_SEQ.sql"),
		},
		{
			name: "stage",
			obj:  Object{Kind: KindStage, Schema: "PUBLIC", Name: "MY_STG"},
			want: fp("PUBLIC", "stages", "MY_STG.sql"),
		},
		{
			name: "stream",
			obj:  Object{Kind: KindStream, Schema: "PUBLIC", Name: "MY_STR"},
			want: fp("PUBLIC", "streams", "MY_STR.sql"),
		},
		{
			name: "task",
			obj:  Object{Kind: KindTask, Schema: "PUBLIC", Name: "MY_TSK"},
			want: fp("PUBLIC", "tasks", "MY_TSK.sql"),
		},
		{
			name: "file format",
			obj:  Object{Kind: KindFileFormat, Schema: "PUBLIC", Name: "MY_FF"},
			want: fp("PUBLIC", "file_formats", "MY_FF.sql"),
		},
		{
			name: "pipe",
			obj:  Object{Kind: KindPipe, Schema: "PUBLIC", Name: "MY_PP"},
			want: fp("PUBLIC", "pipes", "MY_PP.sql"),
		},
		// Functions/procedures include the arg signature.
		{
			name: "function with args",
			obj:  Object{Kind: KindFunction, Schema: "PUBLIC", Name: "F", ArgSig: "FLOAT"},
			want: fp("PUBLIC", "functions", "F__FLOAT.sql"),
		},
		{
			name: "function no args",
			obj:  Object{Kind: KindFunction, Schema: "PUBLIC", Name: "F", ArgSig: "noargs"},
			want: fp("PUBLIC", "functions", "F__noargs.sql"),
		},
		{
			name: "function empty argsig (no parens found)",
			obj:  Object{Kind: KindFunction, Schema: "PUBLIC", Name: "F", ArgSig: ""},
			want: fp("PUBLIC", "functions", "F.sql"),
		},
		{
			name: "procedure with args",
			obj:  Object{Kind: KindProcedure, Schema: "SCH", Name: "P", ArgSig: "NUMBER_VARCHAR"},
			want: fp("SCH", "procedures", "P__NUMBER_VARCHAR.sql"),
		},
		// Empty schema falls back to _root.
		{
			name: "table without schema",
			obj:  Object{Kind: KindTable, Schema: "", Name: "T"},
			want: fp("_root", "tables", "T.sql"),
		},
		{
			name: "function without schema",
			obj:  Object{Kind: KindFunction, Schema: "", Name: "F", ArgSig: "noargs"},
			want: fp("_root", "functions", "F__noargs.sql"),
		},
		// Name sanitization.
		{
			name: "table name with space",
			obj:  Object{Kind: KindTable, Schema: "PUBLIC", Name: "MY TABLE"},
			want: fp("PUBLIC", "tables", "MY_TABLE.sql"),
		},
		{
			name: "schema name with dot",
			obj:  Object{Kind: KindTable, Schema: "MY.SCHEMA", Name: "T"},
			want: fp("MY_SCHEMA", "tables", "T.sql"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.obj.FilePath(); got != tt.want {
				t.Errorf("FilePath() = %q, want %q", got, tt.want)
			}
		})
	}
}

// ─── nameTracker ─────────────────────────────────────────────────────────────

func TestNameTracker_FirstCallReturnsOriginal(t *testing.T) {
	nt := newNameTracker()
	got := nt.resolve("foo.sql")
	if got != "foo.sql" {
		t.Errorf("resolve() = %q, want %q", got, "foo.sql")
	}
}

func TestNameTracker_SecondCallGetsSuffix(t *testing.T) {
	nt := newNameTracker()
	nt.resolve("foo.sql")
	got := nt.resolve("foo.sql")
	if got != "foo_2.sql" {
		t.Errorf("resolve() second = %q, want %q", got, "foo_2.sql")
	}
}

func TestNameTracker_ThirdCallGetsNextSuffix(t *testing.T) {
	nt := newNameTracker()
	nt.resolve("foo.sql")
	nt.resolve("foo.sql")
	got := nt.resolve("foo.sql")
	if got != "foo_3.sql" {
		t.Errorf("resolve() third = %q, want %q", got, "foo_3.sql")
	}
}

func TestNameTracker_CollisionWithGeneratedSuffix(t *testing.T) {
	nt := newNameTracker()
	// Register foo.sql and foo_2.sql independently before any collision.
	nt.resolve("foo.sql")
	nt.resolve("foo_2.sql") // this is legitimately named foo_2

	// Now a collision on foo.sql must skip foo_2 (already taken) and use foo_3.
	got := nt.resolve("foo.sql")
	if got != "foo_3.sql" {
		t.Errorf("resolve() after pre-registered suffix = %q, want %q", got, "foo_3.sql")
	}
}

func TestNameTracker_DifferentPathsDontInterfere(t *testing.T) {
	nt := newNameTracker()
	got1 := nt.resolve("a.sql")
	got2 := nt.resolve("b.sql")
	if got1 != "a.sql" {
		t.Errorf("first path = %q, want %q", got1, "a.sql")
	}
	if got2 != "b.sql" {
		t.Errorf("second path = %q, want %q", got2, "b.sql")
	}
}

func TestNameTracker_PathWithSubdirectory(t *testing.T) {
	nt := newNameTracker()
	path := filepath.Join("PUBLIC", "tables", "T.sql")
	got := nt.resolve(path)
	if got != path {
		t.Errorf("resolve() = %q, want %q", got, path)
	}
	// Collision keeps the directory prefix.
	want := filepath.Join("PUBLIC", "tables", "T_2.sql")
	got = nt.resolve(path)
	if got != want {
		t.Errorf("resolve() collision = %q, want %q", got, want)
	}
}

func TestNameTracker_AllResultsUnique(t *testing.T) {
	// Resolve the same path many times and verify no duplicates.
	nt := newNameTracker()
	seen := make(map[string]bool)
	const n = 20
	for i := 0; i < n; i++ {
		p := nt.resolve("obj.sql")
		if seen[p] {
			t.Fatalf("duplicate path returned at iteration %d: %q", i, p)
		}
		seen[p] = true
	}
}

func TestNameTracker_ConcurrentSafety(t *testing.T) {
	// Many goroutines resolving the same path must all receive unique results.
	nt := newNameTracker()
	const goroutines = 50

	results := make([]string, goroutines)
	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			results[idx] = nt.resolve("concurrent.sql")
		}(i)
	}
	wg.Wait()

	seen := make(map[string]bool, goroutines)
	for _, r := range results {
		if seen[r] {
			t.Errorf("duplicate path in concurrent results: %q", r)
		}
		seen[r] = true
	}
}

// ─── Parse + Split integration ────────────────────────────────────────────────

// TestParseAfterSplit verifies that the full pipeline — splitting a realistic
// DDL blob produced by GET_DDL and then parsing each statement — yields the
// correct metadata for every object type.
func TestParseAfterSplit(t *testing.T) {
	ddl := `create or replace database "ACME";

create or replace schema "ACME"."SALES";

create or replace TABLE "ACME"."SALES"."ORDERS" (
	"ORDER_ID" NUMBER(38,0) NOT NULL,
	"CUSTOMER" VARCHAR(256)
);

create or replace view "ACME"."SALES"."RECENT_ORDERS"(
	"ORDER_ID",
	"CUSTOMER"
) as
select "ORDER_ID", "CUSTOMER" from "ACME"."SALES"."ORDERS" where 1=1;

create or replace function "ACME"."SALES"."DISCOUNT"(PRICE FLOAT, PCT FLOAT)
returns float
language javascript
as $$
    return PRICE * (1 - PCT / 100);
$$;

create or replace procedure "ACME"."SALES"."REFRESH"()
returns varchar
language sql
as $$
BEGIN
    RETURN 'ok';
END
$$;

create or replace sequence "ACME"."SALES"."ORDER_SEQ" start 1 increment 1;

create or replace stage "ACME"."SALES"."LOAD_STAGE" url='s3://acme/load/';

create or replace stream "ACME"."SALES"."ORDERS_STREAM" on table "ACME"."SALES"."ORDERS";

create or replace task "ACME"."SALES"."NIGHTLY_REFRESH"
    warehouse = COMPUTE_WH
    schedule = 'USING CRON 0 2 * * * UTC'
AS
CALL "ACME"."SALES"."REFRESH"();

create or replace file format "ACME"."SALES"."CSV_FORMAT"
    type = 'CSV'
    field_delimiter = ','
    skip_header = 1;

create or replace pipe "ACME"."SALES"."ORDERS_PIPE"
    as copy into "ACME"."SALES"."ORDERS" from @"ACME"."SALES"."LOAD_STAGE";`

	stmts := Split(ddl)

	type wantRow struct {
		kind   Kind
		db     string
		schema string
		name   string
	}

	want := []wantRow{
		{KindDatabase, "", "", "ACME"},
		{KindSchema, "", "ACME", "SALES"},
		{KindTable, "ACME", "SALES", "ORDERS"},
		{KindView, "ACME", "SALES", "RECENT_ORDERS"},
		{KindFunction, "ACME", "SALES", "DISCOUNT"},
		{KindProcedure, "ACME", "SALES", "REFRESH"},
		{KindSequence, "ACME", "SALES", "ORDER_SEQ"},
		{KindStage, "ACME", "SALES", "LOAD_STAGE"},
		{KindStream, "ACME", "SALES", "ORDERS_STREAM"},
		{KindTask, "ACME", "SALES", "NIGHTLY_REFRESH"},
		{KindFileFormat, "ACME", "SALES", "CSV_FORMAT"},
		{KindPipe, "ACME", "SALES", "ORDERS_PIPE"},
	}

	if len(stmts) != len(want) {
		t.Fatalf("Split produced %d statements, want %d\nstmts: %#v", len(stmts), len(want), stmts)
	}

	for i, stmt := range stmts {
		obj := Parse(stmt)
		w := want[i]

		if obj.Kind != w.kind {
			t.Errorf("[%d] Kind = %q, want %q (sql: %q)", i, obj.Kind, w.kind, stmt[:min(60, len(stmt))])
		}
		if obj.Database != w.db {
			t.Errorf("[%d] Database = %q, want %q", i, obj.Database, w.db)
		}
		if obj.Schema != w.schema {
			t.Errorf("[%d] Schema = %q, want %q", i, obj.Schema, w.schema)
		}
		if obj.Name != w.name {
			t.Errorf("[%d] Name = %q, want %q", i, obj.Name, w.name)
		}
	}

	// Spot-check the function overload argument signature.
	funcObj := Parse(stmts[4])
	if funcObj.ArgSig != "FLOAT_FLOAT" {
		t.Errorf("DISCOUNT ArgSig = %q, want %q", funcObj.ArgSig, "FLOAT_FLOAT")
	}

	// Spot-check the procedure no-args signature.
	procObj := Parse(stmts[5])
	if procObj.ArgSig != "noargs" {
		t.Errorf("REFRESH ArgSig = %q, want %q", procObj.ArgSig, "noargs")
	}
}
