// SPDX-License-Identifier: GPL-3.0-or-later

package ddl

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const testDDL = `
create or replace database MYDB;
create or replace schema MYDB.PUBLIC;
create or replace schema MYDB.STAGING;
create or replace TABLE MYDB.PUBLIC.T1 (ID NUMBER);
create or replace TABLE MYDB.STAGING.T2 (ID NUMBER);
create or replace view MYDB.PUBLIC.V1 as select * from T1;
`

// overloadDDL has two FOO overloads that collide under the size-stripped
// argtypes strategy (both VARCHAR) plus a third with a distinct type.
const overloadDDL = `
create or replace database MYDB;
create or replace schema MYDB.PUBLIC;
create or replace function MYDB.PUBLIC.FOO(X VARCHAR(16)) returns float as $$ 1 $$;
create or replace function MYDB.PUBLIC.FOO(X VARCHAR(256)) returns float as $$ 2 $$;
create or replace function MYDB.PUBLIC.FOO(X FLOAT) returns float as $$ 3 $$;
`

func runExportDDL(t *testing.T, dir, ddlText string, opts ExportOptions) ExportResult {
	t.Helper()
	opts.OutputDir = dir
	fetch := func(context.Context, string) (string, error) { return ddlText, nil }
	results := ExportDatabases(context.Background(), []string{"MYDB"}, fetch, opts, nil)
	if len(results[0].Errors) > 0 {
		t.Fatalf("export errors: %v", results[0].Errors)
	}
	return results[0]
}

func runExport(t *testing.T, dir string, opts ExportOptions) ExportResult {
	t.Helper()
	return runExportDDL(t, dir, testDDL, opts)
}

func mustExist(t *testing.T, dir string, rel string, want bool) {
	t.Helper()
	_, err := os.Stat(filepath.Join(dir, rel))
	if exists := err == nil; exists != want {
		t.Errorf("%s: exists=%v, want %v", rel, exists, want)
	}
}

func TestExportObjectTypeFilter(t *testing.T) {
	dir := t.TempDir()
	res := runExport(t, dir, ExportOptions{ObjectTypes: []Kind{KindTable}})
	if res.Files != 5 { // _database + 2 schemas + 2 tables
		t.Errorf("Files = %d, want 5", res.Files)
	}
	mustExist(t, dir, "MYDB/_database.sql", true)      // anchor always kept
	mustExist(t, dir, "MYDB/schemas/PUBLIC.sql", true) // anchor always kept
	mustExist(t, dir, "MYDB/PUBLIC/tables/T1.sql", true)
	mustExist(t, dir, "MYDB/PUBLIC/views/V1.sql", false)
}

func TestExportSchemaFilter(t *testing.T) {
	dir := t.TempDir()
	runExport(t, dir, ExportOptions{Schemas: []string{"staging"}}) // case-insensitive
	mustExist(t, dir, "MYDB/STAGING/tables/T2.sql", true)
	mustExist(t, dir, "MYDB/PUBLIC/tables/T1.sql", false)
	mustExist(t, dir, "MYDB/PUBLIC/views/V1.sql", false)
}

func TestExportQualifiedSchemaFilter(t *testing.T) {
	dir := t.TempDir()
	fetch := func(context.Context, string) (string, error) { return testDDL, nil }
	ExportDatabases(context.Background(), []string{"DB1", "DB2"}, fetch,
		ExportOptions{OutputDir: dir, Schemas: []string{"db1.public"}}, nil) // case-insensitive
	mustExist(t, dir, "DB1/PUBLIC/tables/T1.sql", true)
	mustExist(t, dir, "DB1/STAGING/tables/T2.sql", false)
	mustExist(t, dir, "DB2/PUBLIC/tables/T1.sql", false) // same schema name, other database
}

// ─── overload naming ─────────────────────────────────────────────────────────

func TestExportOverloadNamingDefault(t *testing.T) {
	dir := t.TempDir()
	runExportDDL(t, dir, overloadDDL, ExportOptions{}) // zero value = argtypes
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__FLOAT.sql", true)
	// The two VARCHAR overloads collide and are numbered.
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__VARCHAR.sql", true)
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__VARCHAR_2.sql", true)
}

func TestExportOverloadNamingSignature(t *testing.T) {
	dir := t.TempDir()
	runExportDDL(t, dir, overloadDDL, ExportOptions{OverloadNaming: OverloadNamingSignature})
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__VARCHAR_16.sql", true)
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__VARCHAR_256.sql", true)
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__FLOAT.sql", true)
	// No numbered fallback is needed any more.
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__VARCHAR_2.sql", false)
}

func TestExportOverloadNamingGrouped(t *testing.T) {
	dir := t.TempDir()
	res := runExportDDL(t, dir, overloadDDL, ExportOptions{OverloadNaming: OverloadNamingGrouped})
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO.sql", true)
	mustExist(t, dir, "MYDB/PUBLIC/functions/FOO__VARCHAR.sql", false)
	if res.Files != 3 { // _database + PUBLIC schema + the single grouped FOO.sql
		t.Errorf("Files = %d, want 3", res.Files)
	}

	body, err := os.ReadFile(filepath.Join(dir, "MYDB/PUBLIC/functions/FOO.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"VARCHAR(16)", "VARCHAR(256)", "FLOAT)"} {
		if !strings.Contains(string(body), want) {
			t.Errorf("grouped file is missing %s:\n%s", want, body)
		}
	}
	if n := strings.Count(string(body), ";\n"); n != 3 {
		t.Errorf("grouped file has %d terminated statements, want 3:\n%s", n, body)
	}
}

// Re-exporting the same DDL with the statements reordered must reproduce the
// same file names, so a git working directory shows no spurious changes.
func TestExportOverloadNamingStableAcrossStatementOrder(t *testing.T) {
	reordered := `
create or replace database MYDB;
create or replace schema MYDB.PUBLIC;
create or replace function MYDB.PUBLIC.FOO(X FLOAT) returns float as $$ 3 $$;
create or replace function MYDB.PUBLIC.FOO(X VARCHAR(256)) returns float as $$ 2 $$;
create or replace function MYDB.PUBLIC.FOO(X VARCHAR(16)) returns float as $$ 1 $$;
`
	read := func(ddlText string) map[string]string {
		dir := t.TempDir()
		runExportDDL(t, dir, ddlText, ExportOptions{})
		files := map[string]string{}
		root := filepath.Join(dir, "MYDB/PUBLIC/functions")
		entries, err := os.ReadDir(root)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			b, err := os.ReadFile(filepath.Join(root, e.Name()))
			if err != nil {
				t.Fatal(err)
			}
			files[e.Name()] = string(b)
		}
		return files
	}

	if got, want := read(reordered), read(overloadDDL); !reflect.DeepEqual(got, want) {
		t.Errorf("statement order changed the exported files:\ngot  %v\nwant %v", got, want)
	}
}

func TestExportSkipExisting(t *testing.T) {
	dir := t.TempDir()
	pre := filepath.Join(dir, "MYDB/PUBLIC/tables/T1.sql")
	if err := os.MkdirAll(filepath.Dir(pre), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pre, []byte("-- keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res := runExport(t, dir, ExportOptions{SkipExisting: true})
	if res.Skipped != 1 {
		t.Errorf("Skipped = %d, want 1", res.Skipped)
	}
	got, err := os.ReadFile(pre)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "-- keep me\n" {
		t.Errorf("existing file was overwritten: %q", got)
	}

	// Without SkipExisting the file is overwritten (historical behavior).
	res = runExport(t, dir, ExportOptions{})
	if res.Skipped != 0 {
		t.Errorf("Skipped = %d, want 0", res.Skipped)
	}
	got, _ = os.ReadFile(pre)
	if string(got) == "-- keep me\n" {
		t.Error("file was not overwritten with SkipExisting=false")
	}
}

// bodyDDL has a procedure whose body arrives in the single-quoted form GET_DDL
// returns, a view whose definition contains a string literal, and a function
// that is already dollar-quoted.
const bodyDDL = `
create or replace database MYDB;
create or replace schema MYDB.PUBLIC;
create or replace procedure MYDB.PUBLIC.P1() returns varchar language sql as 'begin
  let x := ''hello'';
  return x;
end';
create or replace function MYDB.PUBLIC.F1() returns float as $$ 1 $$;
create or replace view MYDB.PUBLIC.V1 as select ''a'' as C;
`

func readExported(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestExportDollarQuoteBodies(t *testing.T) {
	dir := t.TempDir()
	runExportDDL(t, dir, bodyDDL, ExportOptions{DollarQuoteBodies: true})

	proc := readExported(t, dir, "MYDB/PUBLIC/procedures/P1__noargs.sql")
	if !strings.Contains(proc, "as $$begin\n  let x := 'hello';\n  return x;\nend$$;") {
		t.Errorf("procedure body not dollar-quoted:\n%s", proc)
	}
	// Already dollar-quoted and non-overloadable objects pass through untouched.
	if fn := readExported(t, dir, "MYDB/PUBLIC/functions/F1__noargs.sql"); !strings.Contains(fn, "as $$ 1 $$") {
		t.Errorf("function body altered:\n%s", fn)
	}
	if v := readExported(t, dir, "MYDB/PUBLIC/views/V1.sql"); !strings.Contains(v, "select ''a'' as C") {
		t.Errorf("view definition altered:\n%s", v)
	}
}

func TestExportDollarQuoteBodiesOff(t *testing.T) {
	dir := t.TempDir()
	runExportDDL(t, dir, bodyDDL, ExportOptions{})

	proc := readExported(t, dir, "MYDB/PUBLIC/procedures/P1__noargs.sql")
	if !strings.Contains(proc, "as 'begin\n  let x := ''hello'';") {
		t.Errorf("procedure body rewritten with the option off:\n%s", proc)
	}
}
