// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package ddl

import (
	"context"
	"os"
	"path/filepath"
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

func runExport(t *testing.T, dir string, opts ExportOptions) ExportResult {
	t.Helper()
	opts.OutputDir = dir
	fetch := func(context.Context, string) (string, error) { return testDDL, nil }
	results := ExportDatabases(context.Background(), []string{"MYDB"}, fetch, opts, nil)
	if len(results[0].Errors) > 0 {
		t.Fatalf("export errors: %v", results[0].Errors)
	}
	return results[0]
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
	mustExist(t, dir, "MYDB/_database.sql", true)             // anchor always kept
	mustExist(t, dir, "MYDB/schemas/PUBLIC.sql", true)        // anchor always kept
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
