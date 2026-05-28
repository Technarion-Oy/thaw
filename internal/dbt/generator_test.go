// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package dbt

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// readFile reads a generated file relative to projectDir.
func readFile(t *testing.T, projectDir, relPath string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(projectDir, relPath))
	if err != nil {
		t.Fatalf("readFile(%q): %v", relPath, err)
	}
	return string(data)
}

// assertContains fails the test if none of the needles are absent from s.
func assertContains(t *testing.T, s, needle, context string) {
	t.Helper()
	if !strings.Contains(s, needle) {
		t.Errorf("%s: want substring %q in:\n%s", context, needle, s)
	}
}

// assertNotContains fails if needle appears in s.
func assertNotContains(t *testing.T, s, needle, context string) {
	t.Helper()
	if strings.Contains(s, needle) {
		t.Errorf("%s: unexpected substring %q in:\n%s", context, needle, s)
	}
}

// assertFileExists fails if the file does not exist on disk.
func assertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected file to exist: %s", path)
	}
}

// assertFileAbsent fails if the file exists on disk.
func assertFileAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file NOT to exist: %s", path)
	}
}

// mustGenerate calls Generate and fails the test on error.
func mustGenerate(t *testing.T, req CreateRequest, session SessionInfo, objects []SchemaObjects) *CreateResult {
	t.Helper()
	result, err := Generate(req, session, objects)
	if err != nil {
		t.Fatalf("Generate() error: %v", err)
	}
	if result == nil {
		t.Fatal("Generate() returned nil result")
	}
	return result
}

// typicalSession returns a non-empty SessionInfo for use across tests.
func typicalSession() SessionInfo {
	return SessionInfo{
		Account:   "xy12345.us-east-1",
		User:      "ANALYST",
		Role:      "TRANSFORMER",
		Warehouse: "COMPUTE_WH",
		Database:  "ANALYTICS",
		Schema:    "PUBLIC",
	}
}

// ─── TestGenerate ─────────────────────────────────────────────────────────────

func TestGenerate(t *testing.T) {
	t.Run("single schema single table", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "my_project", OutputDir: dir}
		objects := []SchemaObjects{{DB: "MYDB", Schema: "PUBLIC", Tables: []string{"ORDERS"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		if result.ProjectDir != filepath.Join(dir, "my_project") {
			t.Errorf("ProjectDir = %q, want %q", result.ProjectDir, filepath.Join(dir, "my_project"))
		}

		// Static files must all exist.
		for _, f := range []string{
			"dbt_project.yml",
			"profiles.yml",
			filepath.Join("seeds", ".gitkeep"),
			filepath.Join("macros", ".gitkeep"),
			filepath.Join("models", "marts", ".gitkeep"),
			filepath.Join("models", "staging", "_sources.yml"),
		} {
			assertFileExists(t, filepath.Join(result.ProjectDir, f))
		}

		// Single stub: single-scope → stg_orders.sql
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_orders.sql"))
		assertFileAbsent(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_mydb_public_orders.sql"))

		if len(result.Warnings) != 0 {
			t.Errorf("unexpected warnings: %v", result.Warnings)
		}
	})

	t.Run("single schema tables and views both get stubs", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{
			DB:     "DB",
			Schema: "SCH",
			Tables: []string{"CUSTOMERS", "PRODUCTS"},
			Views:  []string{"V_SALES", "V_SUMMARY"},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)

		for _, stub := range []string{
			"stg_customers.sql",
			"stg_products.sql",
			"stg_v_sales.sql",
			"stg_v_summary.sql",
		} {
			assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", stub))
		}

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		for _, name := range []string{"CUSTOMERS", "PRODUCTS", "V_SALES", "V_SUMMARY"} {
			assertContains(t, sources, "- name: "+name, "_sources.yml table entry")
		}
	})

	t.Run("single db+schema uses simple stg_<table>.sql prefix", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{DB: "MYDB", Schema: "PUBLIC", Tables: []string{"FACT_SALES"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_fact_sales.sql"))
		// Multi-scope variant must NOT be present.
		assertFileAbsent(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_mydb_public_fact_sales.sql"))
	})

	t.Run("multiple db+schema pairs use db_schema_ prefix to avoid collisions", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "PROD", Schema: "RAW", Tables: []string{"EVENTS"}},
			{DB: "STAGING", Schema: "RAW", Tables: []string{"EVENTS"}}, // same table name, different db
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_prod_raw_events.sql"))
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_staging_raw_events.sql"))
		// Simple-prefix files must not exist.
		assertFileAbsent(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_events.sql"))
	})

	t.Run("empty schema emits warning and is not added to sources", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "DB", Schema: "EMPTY_SCH"},               // no tables or views
			{DB: "DB", Schema: "PUBLIC", Tables: []string{"ORDERS"}},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		if len(result.Warnings) != 1 {
			t.Fatalf("want 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}
		if !strings.Contains(result.Warnings[0], "EMPTY_SCH") {
			t.Errorf("warning should mention schema name, got: %q", result.Warnings[0])
		}

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		assertNotContains(t, sources, "empty_sch", "_sources.yml should not list empty schema")
		assertContains(t, sources, "db_public", "_sources.yml should list non-empty schema")
	})

	t.Run("multiple empty schemas produce multiple warnings", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "DB", Schema: "SCH_A"},
			{DB: "DB", Schema: "SCH_B"},
			{DB: "DB", Schema: "SCH_C"},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		if len(result.Warnings) != 3 {
			t.Errorf("want 3 warnings, got %d: %v", len(result.Warnings), result.Warnings)
		}
		for _, w := range result.Warnings {
			if !strings.Contains(w, "skipped") {
				t.Errorf("warning should mention 'skipped', got: %q", w)
			}
		}
	})

	t.Run("INFORMATION_SCHEMA adds source entry with tables:[] and no stubs", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "MYDB", Schema: "INFORMATION_SCHEMA", IsSystem: true},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		if len(result.Warnings) != 0 {
			t.Errorf("system schema should not produce warnings, got: %v", result.Warnings)
		}

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		assertContains(t, sources, "mydb_information_schema", "_sources.yml source name")
		assertContains(t, sources, "database: MYDB", "_sources.yml database")
		assertContains(t, sources, "schema: INFORMATION_SCHEMA", "_sources.yml schema")
		assertContains(t, sources, "tables: []", "_sources.yml empty tables list")

		// No staging stub files should be generated.
		stagingDir := filepath.Join(result.ProjectDir, "models", "staging")
		entries, _ := os.ReadDir(stagingDir)
		for _, e := range entries {
			if strings.HasSuffix(e.Name(), ".sql") {
				t.Errorf("unexpected stub file for system schema: %s", e.Name())
			}
		}
	})

	t.Run("INFORMATION_SCHEMA case-insensitive detection via IsSystem flag", func(t *testing.T) {
		// The dbt.go caller sets IsSystem based on case-insensitive comparison;
		// the generator trusts the flag regardless of the Schema string's case.
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "DB", Schema: "information_schema", IsSystem: true},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		assertContains(t, sources, "tables: []", "system schema must have empty tables list")
		if len(result.Warnings) != 0 {
			t.Errorf("unexpected warnings: %v", result.Warnings)
		}
	})

	t.Run("mixed: system + regular + empty schemas", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "DB", Schema: "INFORMATION_SCHEMA", IsSystem: true},
			{DB: "DB", Schema: "PUBLIC", Tables: []string{"ORDERS"}, Views: []string{"V_OPEN"}},
			{DB: "DB", Schema: "TEMP"},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		// One warning for the empty TEMP schema.
		if len(result.Warnings) != 1 {
			t.Fatalf("want 1 warning, got %d: %v", len(result.Warnings), result.Warnings)
		}

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		// System schema present as source entry.
		assertContains(t, sources, "db_information_schema", "system source entry")
		assertContains(t, sources, "tables: []", "system schema empty tables")
		// Regular schema present with its tables.
		assertContains(t, sources, "db_public", "regular source entry")
		assertContains(t, sources, "- name: ORDERS", "orders table entry")
		assertContains(t, sources, "- name: V_OPEN", "view entry")
		// Empty schema must be absent.
		assertNotContains(t, sources, "db_temp", "empty schema must not appear")

		// Stubs only for the regular schema — multi-scope since 3 objects passed.
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_db_public_orders.sql"))
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_db_public_v_open.sql"))
	})

	t.Run("profile name defaults to project name when empty", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "acme_dbt", OutputDir: dir, ProfileName: ""}
		objects := []SchemaObjects{{DB: "DB", Schema: "SCH", Tables: []string{"T"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		dbtProject := readFile(t, result.ProjectDir, "dbt_project.yml")
		assertContains(t, dbtProject, "profile: 'acme_dbt'", "dbt_project.yml profile reference")

		profiles := readFile(t, result.ProjectDir, "profiles.yml")
		assertContains(t, profiles, "acme_dbt:", "profiles.yml top-level key")
	})

	t.Run("profile name explicitly set overrides project name", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "my_project", OutputDir: dir, ProfileName: "snowflake_prod"}
		objects := []SchemaObjects{{DB: "DB", Schema: "SCH", Tables: []string{"T"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		dbtProject := readFile(t, result.ProjectDir, "dbt_project.yml")
		assertContains(t, dbtProject, "profile: 'snowflake_prod'", "dbt_project.yml profile reference")

		profiles := readFile(t, result.ProjectDir, "profiles.yml")
		assertContains(t, profiles, "snowflake_prod:", "profiles.yml top-level key")
		assertNotContains(t, profiles, "my_project:", "project name must not appear as profile key")
	})

	t.Run("profiles.yml contains all session values", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		session := SessionInfo{
			Account:   "abc12345.eu-central-1",
			User:      "DATA_ENGINEER",
			Role:      "ANALYST_ROLE",
			Warehouse: "TRANSFORM_WH",
			Database:  "DWH",
			Schema:    "GOLD",
		}
		objects := []SchemaObjects{{DB: "DB", Schema: "SCH", Tables: []string{"T"}}}

		result := mustGenerate(t, req, session, objects)
		profiles := readFile(t, result.ProjectDir, "profiles.yml")

		assertContains(t, profiles, "account: abc12345.eu-central-1", "account")
		assertContains(t, profiles, "user: DATA_ENGINEER", "user")
		assertContains(t, profiles, "role: ANALYST_ROLE", "role")
		assertContains(t, profiles, "warehouse: TRANSFORM_WH", "warehouse")
		assertContains(t, profiles, "database: DWH", "database")
		assertContains(t, profiles, "schema: GOLD", "schema")
	})

	t.Run("dbt_project.yml has correct structure", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "warehouse_dbt", OutputDir: dir, ProfileName: "wh_profile"}
		objects := []SchemaObjects{{DB: "DB", Schema: "SCH", Tables: []string{"T"}}}

		result := mustGenerate(t, req, typicalSession(), objects)
		yml := readFile(t, result.ProjectDir, "dbt_project.yml")

		assertContains(t, yml, "name: 'warehouse_dbt'", "project name")
		assertContains(t, yml, "profile: 'wh_profile'", "profile reference")
		assertContains(t, yml, "config-version: 2", "config version")
		assertContains(t, yml, `model-paths: ["models"]`, "model paths")
		assertContains(t, yml, `seed-paths: ["seeds"]`, "seed paths")
		assertContains(t, yml, `macro-paths: ["macros"]`, "macro paths")
		assertContains(t, yml, "+materialized: view", "staging materialization")
		assertContains(t, yml, "+materialized: table", "marts materialization")
		assertContains(t, yml, "  warehouse_dbt:", "models key indentation")
	})

	t.Run("gitkeep files created for seeds, macros, marts", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		// No objects at all — but static files still need creating.
		result := mustGenerate(t, req, typicalSession(), []SchemaObjects{})

		for _, f := range []string{
			filepath.Join("seeds", ".gitkeep"),
			filepath.Join("macros", ".gitkeep"),
			filepath.Join("models", "marts", ".gitkeep"),
		} {
			assertFileExists(t, filepath.Join(result.ProjectDir, f))
		}
	})

	t.Run("staging stub contains correct source() Jinja references", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{DB: "ANALYTICS", Schema: "RAW", Tables: []string{"PAGEVIEWS"}}}

		result := mustGenerate(t, req, typicalSession(), objects)
		stub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_pageviews.sql"))

		wantSource := "{{ source('analytics_raw', 'PAGEVIEWS') }}"
		// Both the select and the comment header should reference the source.
		if strings.Count(stub, "source('analytics_raw', 'PAGEVIEWS')") < 2 {
			t.Errorf("stub should reference source() at least twice (header + select), got:\n%s", stub)
		}
		assertContains(t, stub, wantSource, "stub source reference")
		assertContains(t, stub, "with source as (", "CTE structure")
		assertContains(t, stub, "renamed as (", "renamed CTE")
		assertContains(t, stub, "select * from renamed", "final select")
		assertContains(t, stub, "TODO", "stub TODO comment")
	})

	t.Run("source name in stub matches _sources.yml name entry", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{DB: "MYDB", Schema: "PUBLIC", Tables: []string{"DIM_CUSTOMER"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		stub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_dim_customer.sql"))

		// The source name in _sources.yml must match what the stub references.
		assertContains(t, sources, "name: mydb_public", "_sources.yml source name")
		assertContains(t, stub, "'mydb_public'", "stub source name")
	})

	t.Run("large number of tables all get stubs", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}

		tables := make([]string, 30)
		for i := range tables {
			tables[i] = fmt.Sprintf("TABLE_%02d", i)
		}
		objects := []SchemaObjects{{DB: "DB", Schema: "SCH", Tables: tables}}

		result := mustGenerate(t, req, typicalSession(), objects)

		for _, tbl := range tables {
			stub := filepath.Join(result.ProjectDir, "models", "staging",
				"stg_"+strings.ToLower(tbl)+".sql")
			assertFileExists(t, stub)
		}
		if len(result.Warnings) != 0 {
			t.Errorf("unexpected warnings: %v", result.Warnings)
		}
	})

	t.Run("three databases multi-scope prefixes are unique", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "PROD", Schema: "CORE", Tables: []string{"ORDERS"}},
			{DB: "STAGING", Schema: "CORE", Tables: []string{"ORDERS"}},
			{DB: "DEV", Schema: "CORE", Tables: []string{"ORDERS"}},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_prod_core_orders.sql"))
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_staging_core_orders.sql"))
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_dev_core_orders.sql"))
	})

	t.Run("filesCreated list matches actual files on disk", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{DB: "DB", Schema: "SCH", Tables: []string{"A", "B"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		for _, rel := range result.FilesCreated {
			assertFileExists(t, filepath.Join(result.ProjectDir, rel))
		}

		// Verify specific entries are present in the list.
		inList := func(suffix string) bool {
			for _, f := range result.FilesCreated {
				if strings.HasSuffix(filepath.ToSlash(f), suffix) {
					return true
				}
			}
			return false
		}
		for _, expected := range []string{
			"dbt_project.yml",
			"profiles.yml",
			"seeds/.gitkeep",
			"macros/.gitkeep",
			"models/marts/.gitkeep",
			"models/staging/_sources.yml",
			"models/staging/stg_a.sql",
			"models/staging/stg_b.sql",
		} {
			if !inList(expected) {
				t.Errorf("filesCreated missing: %s\ngot: %v", expected, result.FilesCreated)
			}
		}
	})

	t.Run("_sources.yml version header always present", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}

		result := mustGenerate(t, req, typicalSession(), []SchemaObjects{})

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		assertContains(t, sources, "version: 2", "_sources.yml version header")
		assertContains(t, sources, "sources:", "_sources.yml sources key")
	})

	t.Run("views-only schema works the same as tables-only", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{DB: "DB", Schema: "SCH", Views: []string{"V_REVENUE", "V_COSTS"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_v_revenue.sql"))
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_v_costs.sql"))
		if len(result.Warnings) != 0 {
			t.Errorf("unexpected warnings: %v", result.Warnings)
		}
	})

	t.Run("INFORMATION_SCHEMA alongside regular schemas multi-scope naming", func(t *testing.T) {
		// When INFORMATION_SCHEMA + one regular schema are both present,
		// len(objects)==2 so multi-scope prefixes apply to stub files.
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "DB", Schema: "INFORMATION_SCHEMA", IsSystem: true},
			{DB: "DB", Schema: "PUBLIC", Tables: []string{"USERS"}},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		// Stub uses multi-scope prefix.
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_db_public_users.sql"))
		assertFileAbsent(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_users.sql"))

		// _sources.yml has both entries.
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		assertContains(t, sources, "db_information_schema", "system source")
		assertContains(t, sources, "db_public", "regular source")
	})

	t.Run("project directory is nested under output dir", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "nested_project", OutputDir: dir}

		result := mustGenerate(t, req, typicalSession(), []SchemaObjects{})

		expected := filepath.Join(dir, "nested_project")
		if result.ProjectDir != expected {
			t.Errorf("ProjectDir = %q, want %q", result.ProjectDir, expected)
		}
		assertFileExists(t, filepath.Join(expected, "dbt_project.yml"))
	})

	t.Run("table and source names are lowercased in file paths and source names", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{DB: "UPPER_DB", Schema: "UPPER_SCH", Tables: []string{"UPPER_TABLE"}}}

		result := mustGenerate(t, req, typicalSession(), objects)

		// Source name must be fully lowercase.
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		assertContains(t, sources, "name: upper_db_upper_sch", "lowercase source name")
		// Database and schema in YAML kept as-is (user's original casing).
		assertContains(t, sources, "database: UPPER_DB", "database casing preserved")
		assertContains(t, sources, "schema: UPPER_SCH", "schema casing preserved")
		// Table name preserved in the tables entry.
		assertContains(t, sources, "- name: UPPER_TABLE", "table name preserved")
		// Stub filename lowercased.
		assertFileExists(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_upper_table.sql"))
	})

	t.Run("system schema description mentions manual entry", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{DB: "DB", Schema: "INFORMATION_SCHEMA", IsSystem: true}}

		result := mustGenerate(t, req, typicalSession(), objects)

		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))
		assertContains(t, sources, "manually", "system schema description should mention manual entry")
	})

	t.Run("source name collision with underscore-containing identifiers", func(t *testing.T) {
		// DB "A_B" + Schema "C" and DB "A" + Schema "B_C" must produce
		// distinct source names in _sources.yml.  The current underscore
		// separator is ambiguous for identifiers that themselves contain
		// underscores.
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{
			{DB: "A_B", Schema: "C", Tables: []string{"T1"}},
			{DB: "A", Schema: "B_C", Tables: []string{"T2"}},
		}

		result := mustGenerate(t, req, typicalSession(), objects)
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))

		count := strings.Count(sources, "- name: a_b_c")
		if count > 1 {
			t.Errorf("duplicate source name 'a_b_c' in _sources.yml (%d occurrences) — "+
				"A_B.C and A.B_C produce the same source name due to underscore separator ambiguity",
				count)
		}
	})
}

// ─── TestSourceName ───────────────────────────────────────────────────────────

func TestSourceName(t *testing.T) {
	tests := []struct {
		db, schema string
		want       string
	}{
		{"MYDB", "PUBLIC", "mydb_public"},
		{"mydb", "public", "mydb_public"},
		{"MyDb", "MySchema", "mydb_myschema"},
		{"DB", "INFORMATION_SCHEMA", "db_information_schema"},
		{"ANALYTICS", "GOLD", "analytics_gold"},
		{"A", "B", "a_b"},
		{"DB_WITH_UNDERSCORES", "SCH_WITH_UNDERSCORES", "db_with_underscores_sch_with_underscores"},
		// Numbers and mixed case.
		{"DB1", "SCH2", "db1_sch2"},
		{"UPPER", "lower", "upper_lower"},
	}

	for _, tt := range tests {
		t.Run(tt.db+"."+tt.schema, func(t *testing.T) {
			got := sourceName(tt.db, tt.schema)
			if got != tt.want {
				t.Errorf("sourceName(%q, %q) = %q, want %q", tt.db, tt.schema, got, tt.want)
			}
		})
	}

	t.Run("underscore separator must not cause collisions", func(t *testing.T) {
		// DB "A_B" + Schema "C" and DB "A" + Schema "B_C" are different
		// Snowflake scopes but both lower to "a_b" + "_" + "c" = "a_b_c".
		a := sourceName("A_B", "C")
		b := sourceName("A", "B_C")
		if a == b {
			t.Errorf("sourceName(%q, %q) = sourceName(%q, %q) = %q — "+
				"ambiguous underscore separator causes collision",
				"A_B", "C", "A", "B_C", a)
		}
	})
}

// ─── TestStagingModelPath ─────────────────────────────────────────────────────

func TestStagingModelPath(t *testing.T) {
	tests := []struct {
		name       string
		db, schema string
		table      string
		multiScope bool
		want       string
	}{
		{
			name:       "single-scope lowercase path",
			db:         "DB", schema: "SCH", table: "ORDERS",
			multiScope: false,
			want:       filepath.Join("models", "staging", "stg_orders.sql"),
		},
		{
			name:       "multi-scope includes db and schema prefix",
			db:         "DB", schema: "SCH", table: "ORDERS",
			multiScope: true,
			want:       filepath.Join("models", "staging", "stg_db_sch_orders.sql"),
		},
		{
			name:       "single-scope uppercase table lowercased",
			db:         "MYDB", schema: "PUBLIC", table: "FACT_SALES",
			multiScope: false,
			want:       filepath.Join("models", "staging", "stg_fact_sales.sql"),
		},
		{
			name:       "multi-scope all uppercase lowercased",
			db:         "MYDB", schema: "PUBLIC", table: "FACT_SALES",
			multiScope: true,
			want:       filepath.Join("models", "staging", "stg_mydb_public_fact_sales.sql"),
		},
		{
			name:       "single-scope mixed case",
			db:         "Db", schema: "Sch", table: "MyTable",
			multiScope: false,
			want:       filepath.Join("models", "staging", "stg_mytable.sql"),
		},
		{
			name:       "multi-scope mixed case",
			db:         "Db", schema: "Sch", table: "MyTable",
			multiScope: true,
			want:       filepath.Join("models", "staging", "stg_db_sch_mytable.sql"),
		},
		{
			name:       "multi-scope with underscores in identifiers",
			db:         "MY_DB", schema: "MY_SCH", table: "MY_TABLE",
			multiScope: true,
			want:       filepath.Join("models", "staging", "stg_my_db_my_sch_my_table.sql"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stagingModelPath(tt.db, tt.schema, tt.table, tt.multiScope)
			if got != tt.want {
				t.Errorf("stagingModelPath(%q, %q, %q, %v) = %q, want %q",
					tt.db, tt.schema, tt.table, tt.multiScope, got, tt.want)
			}
		})
	}

	t.Run("underscore separator must not cause multi-scope path collisions", func(t *testing.T) {
		a := stagingModelPath("A_B", "C", "T", true)
		b := stagingModelPath("A", "B_C", "T", true)
		if a == b {
			t.Errorf("stagingModelPath collision: (A_B, C, T) and (A, B_C, T) both produce %q", a)
		}
	})
}

// ─── TestStagingModelSQL ──────────────────────────────────────────────────────

func TestStagingModelSQL(t *testing.T) {
	tests := []struct {
		name           string
		source         string
		table          string
		sqlBody        string // non-empty → inline mode
		mustContain    []string
		mustNotContain []string
		mustCount      map[string]int // substring → minimum occurrence count
	}{
		{
			name:   "basic stub structure",
			source: "mydb_public", table: "ORDERS",
			mustContain: []string{
				"Generated by Thaw",
				"with source as (",
				"renamed as (",
				"select * from renamed",
				"TODO",
				"{{ source('mydb_public', 'ORDERS') }}",
			},
			mustCount: map[string]int{
				"source('mydb_public', 'ORDERS')": 2, // header comment + select
			},
		},
		{
			name:   "source references correct source name and table",
			source: "analytics_raw", table: "PAGEVIEWS",
			mustContain: []string{
				"{{ source('analytics_raw', 'PAGEVIEWS') }}",
				"dbt stub for {{ source('analytics_raw', 'PAGEVIEWS') }}",
			},
		},
		{
			name:   "different source and table",
			source: "staging_core", table: "DIM_CUSTOMER",
			mustContain: []string{
				"source('staging_core', 'DIM_CUSTOMER')",
				"select * from {{ source('staging_core', 'DIM_CUSTOMER') }}",
			},
		},
		{
			name:   "CTE pattern is correct",
			source: "src", table: "T",
			mustContain: []string{
				"with source as (",
				"select * from {{ source('src', 'T') }}",
				"),\nrenamed as (",
				"select\n        *",
				"from source",
				"select * from renamed",
			},
		},
		// ── inline view SQL body mode ─────────────────────────────────────────
		{
			name:    "non-empty body: header comment emitted, body embedded verbatim",
			source:  "prod_raw", table: "SALES_VIEW",
			sqlBody: "SELECT id, amount, region\nFROM RAW_DB.STAGING.ORDERS\nWHERE amount > 0",
			mustContain: []string{
				"Generated by Thaw",
				"view SQL inlined from Snowflake",
				"SELECT id, amount, region\nFROM RAW_DB.STAGING.ORDERS\nWHERE amount > 0",
			},
			mustNotContain: []string{
				"with source as (",
				"renamed as (",
				"{{ source('prod_raw', 'SALES_VIEW') }}",
			},
		},
		{
			name:    "non-empty body: already-rewritten Jinja refs emitted verbatim",
			source:  "db_sch", table: "MY_VIEW",
			sqlBody: "SELECT * FROM {{ source('db_raw', 'ORDERS') }}\nJOIN {{ ref('stg_customers') }} ON id = customer_id",
			mustContain: []string{
				"{{ source('db_raw', 'ORDERS') }}",
				"{{ ref('stg_customers') }}",
			},
			mustNotContain: []string{
				"with source as (",
			},
		},
		{
			name:    "non-empty body: complex multi-CTE SQL with comments preserved",
			source:  "analytics_mart", table: "REVENUE_VIEW",
			sqlBody: "-- Revenue by segment\nWITH base AS (\n  SELECT * FROM {{ source('raw', 'ORDERS') }}\n)\nSELECT segment, SUM(amount) AS revenue\nFROM base\nGROUP BY 1",
			mustContain: []string{
				"Generated by Thaw",
				"view SQL inlined from Snowflake",
				"-- Revenue by segment",
				"WITH base AS (",
				"{{ source('raw', 'ORDERS') }}",
				"GROUP BY 1",
			},
			mustNotContain: []string{
				"with source as (",
			},
		},
		{
			name:    "empty body string falls back to source() CTE stub",
			source:  "analytics_raw", table: "EVENTS",
			sqlBody: "",
			mustContain: []string{
				"{{ source('analytics_raw', 'EVENTS') }}",
				"with source as (",
				"renamed as (",
			},
		},
		{
			name:    "whitespace-only body falls back to source() stub",
			source:  "src", table: "T",
			sqlBody: "   \n  \t  ",
			mustContain: []string{
				"{{ source('src', 'T') }}",
				"with source as (",
			},
			mustNotContain: []string{
				"view SQL inlined from Snowflake",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stagingModelSQL(tt.source, tt.table, tt.sqlBody)
			for _, needle := range tt.mustContain {
				assertContains(t, got, needle, "stagingModelSQL")
			}
			for _, notWant := range tt.mustNotContain {
				assertNotContains(t, got, notWant, "stagingModelSQL")
			}
			for substr, minCount := range tt.mustCount {
				count := strings.Count(got, substr)
				if count < minCount {
					t.Errorf("stagingModelSQL: expected %q at least %d times, got %d\n%s",
						substr, minCount, count, got)
				}
			}
		})
	}
}

// ─── TestGenerate_InlineViewDefs ──────────────────────────────────────────────

func TestGenerate_InlineViewDefs(t *testing.T) {
	t.Run("view with ViewDef uses inlined body not source() stub", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir, InlineViewDefs: true}
		viewBody := "SELECT id, amount FROM RAW_DB.STAGING.ORDERS WHERE amount > 0"
		objects := []SchemaObjects{{
			DB:     "PROD",
			Schema: "MART",
			Views:  []string{"SALES_VIEW"},
			ViewDefs: map[string]string{
				"SALES_VIEW": viewBody,
			},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		stub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_sales_view.sql"))

		assertContains(t, stub, viewBody, "stub must contain inlined SQL body")
		assertContains(t, stub, "Generated by Thaw", "stub must have Thaw header comment")
		assertContains(t, stub, "view SQL inlined from Snowflake", "stub must identify source")
		// The passthrough stub's CTE structure must not appear — the inlined body
		// replaces the generic select * from {{ source(...) }} pattern entirely.
		assertNotContains(t, stub, "with source as (", "inlined view must not use CTE passthrough")
		assertNotContains(t, stub, "select * from {{ source(", "inlined view must not have passthrough select")
	})

	t.Run("view without ViewDef entry falls back to source() passthrough", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{
			DB:     "DB",
			Schema: "SCH",
			Views:  []string{"PLAIN_VIEW"},
			// ViewDefs is nil — no body available
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		stub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_plain_view.sql"))

		assertContains(t, stub, "{{ source('db_sch', 'PLAIN_VIEW') }}", "fallback must use source() ref")
		assertContains(t, stub, "with source as (", "fallback must use CTE stub pattern")
	})

	t.Run("tables always use source() stub even when ViewDefs map is present", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{
			DB:     "DB",
			Schema: "SCH",
			Tables: []string{"ORDERS"},
			// Tables don't have ViewDefs entries; map key mismatch → "" → source() stub
			ViewDefs: map[string]string{},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		stub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_orders.sql"))

		assertContains(t, stub, "{{ source('db_sch', 'ORDERS') }}", "table must use source() stub")
		assertContains(t, stub, "with source as (", "table must use CTE pattern")
	})

	t.Run("partial ViewDefs: some views inlined, rest use source() passthrough", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{
			DB:    "DB",
			Schema: "SCH",
			Views: []string{"VIEW_WITH_DEF", "VIEW_NO_DEF_A", "VIEW_NO_DEF_B"},
			ViewDefs: map[string]string{
				"VIEW_WITH_DEF": "SELECT a, b FROM DB.SCH.SOURCE_TABLE",
			},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)

		withDef := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_view_with_def.sql"))
		noDef1 := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_view_no_def_a.sql"))
		noDef2 := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_view_no_def_b.sql"))

		// View with body: inlined SQL
		assertContains(t, withDef, "SELECT a, b FROM DB.SCH.SOURCE_TABLE", "VIEW_WITH_DEF inlined body")
		assertNotContains(t, withDef, "with source as (", "VIEW_WITH_DEF must not use passthrough")

		// Views without body: source() passthrough
		assertContains(t, noDef1, "{{ source('db_sch', 'VIEW_NO_DEF_A') }}", "VIEW_NO_DEF_A source ref")
		assertContains(t, noDef1, "with source as (", "VIEW_NO_DEF_A CTE pattern")
		assertContains(t, noDef2, "{{ source('db_sch', 'VIEW_NO_DEF_B') }}", "VIEW_NO_DEF_B source ref")
	})

	t.Run("mixed tables and views: tables always source(), views with def inlined", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		rawSQL := "SELECT id, ts FROM INGEST.RAW.EVENTS WHERE ts > '2024-01-01'"
		objects := []SchemaObjects{{
			DB:     "DW",
			Schema: "MART",
			Tables: []string{"FACT_ORDERS", "DIM_CUSTOMER"},
			Views:  []string{"V_REVENUE", "V_COSTS"},
			ViewDefs: map[string]string{
				"V_REVENUE": rawSQL,
				// V_COSTS has no entry → passthrough
			},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)

		factStub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_fact_orders.sql"))
		dimStub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_dim_customer.sql"))
		revStub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_v_revenue.sql"))
		costStub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_v_costs.sql"))

		// Tables: source() passthrough regardless of ViewDefs
		assertContains(t, factStub, "{{ source('dw_mart', 'FACT_ORDERS') }}", "FACT_ORDERS stub")
		assertContains(t, factStub, "with source as (", "FACT_ORDERS CTE pattern")
		assertContains(t, dimStub, "{{ source('dw_mart', 'DIM_CUSTOMER') }}", "DIM_CUSTOMER stub")

		// V_REVENUE: inlined
		assertContains(t, revStub, rawSQL, "V_REVENUE inlined body")
		assertNotContains(t, revStub, "with source as (", "V_REVENUE must not use passthrough")

		// V_COSTS: no def → passthrough
		assertContains(t, costStub, "{{ source('dw_mart', 'V_COSTS') }}", "V_COSTS source ref")
		assertContains(t, costStub, "with source as (", "V_COSTS CTE pattern")
	})

	t.Run("multi-scope with ViewDefs: prefixed filename + inlined body", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir, InlineViewDefs: true}
		viewSQL := "SELECT user_id, event FROM TRACK.RAW.EVENTS"
		objects := []SchemaObjects{
			{
				DB:    "PROD",
				Schema: "MART",
				Views: []string{"SESSION_VIEW"},
				ViewDefs: map[string]string{
					"SESSION_VIEW": viewSQL,
				},
			},
			{
				DB:    "STAGING",
				Schema: "CORE",
				Tables: []string{"USERS"},
			},
		}

		result := mustGenerate(t, req, typicalSession(), objects)

		// Multi-scope: stubs prefixed with db_schema_
		viewStub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_prod_mart_session_view.sql"))
		assertContains(t, viewStub, viewSQL, "SESSION_VIEW inlined body in multi-scope file")

		tableStub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_staging_core_users.sql"))
		assertContains(t, tableStub, "{{ source('staging_core', 'USERS') }}", "USERS source ref")

		// Confirm unprefixed files do not exist
		assertFileAbsent(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_session_view.sql"))
		assertFileAbsent(t, filepath.Join(result.ProjectDir, "models", "staging", "stg_users.sql"))
	})

	t.Run("empty string ViewDef treated as missing — uses source() stub", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir}
		objects := []SchemaObjects{{
			DB:    "DB",
			Schema: "SCH",
			Views: []string{"EMPTY_DEF_VIEW"},
			ViewDefs: map[string]string{
				"EMPTY_DEF_VIEW": "", // explicitly empty
			},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		stub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_empty_def_view.sql"))

		assertContains(t, stub, "{{ source('db_sch', 'EMPTY_DEF_VIEW') }}", "empty def falls back to source()")
		assertContains(t, stub, "with source as (", "empty def uses CTE pattern")
		assertNotContains(t, stub, "view SQL inlined from Snowflake", "empty def must not show inline header")
	})

	t.Run("whitespace-only ViewDef treated as empty — uses source() stub", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{ProjectName: "proj", OutputDir: dir, InlineViewDefs: true}
		objects := []SchemaObjects{{
			DB:     "DB",
			Schema: "SCH",
			Views:  []string{"WS_VIEW"},
			ViewDefs: map[string]string{
				"WS_VIEW": "   \n  \t  ", // whitespace-only
			},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		stub := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "stg_ws_view.sql"))

		assertContains(t, stub, "{{ source('db_sch', 'WS_VIEW') }}", "whitespace body should fall back to source()")
		assertContains(t, stub, "with source as (", "whitespace body should use CTE pattern")
		assertNotContains(t, stub, "view SQL inlined from Snowflake", "whitespace body must not show inline header")
	})
}

// ─── TestGenerate_DatabaseVars ────────────────────────────────────────────────

func TestGenerate_DatabaseVars(t *testing.T) {
	t.Run("single DB: vars block in dbt_project.yml", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{{
			DB:     "MYDB",
			Schema: "PUBLIC",
			Tables: []string{"ORDERS"},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		proj := readFile(t, result.ProjectDir, "dbt_project.yml")

		assertContains(t, proj, "vars:", "vars block present")
		assertContains(t, proj, "  db_mydb: MYDB", "var entry for MYDB")
	})

	t.Run("single DB: _sources.yml uses var() not literal", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{{
			DB:     "MYDB",
			Schema: "PUBLIC",
			Tables: []string{"ORDERS"},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))

		assertContains(t, sources, "database: \"{{ var('db_mydb', 'MYDB') }}\"", "sources uses var ref")
		assertNotContains(t, sources, "database: MYDB", "sources must not emit literal database name")
	})

	t.Run("multiple DBs: all have var entries, sorted", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{
			{DB: "ZEBRA_DB", Schema: "SCH", Tables: []string{"T1"}},
			{DB: "ALPHA_DB", Schema: "SCH", Tables: []string{"T2"}},
			{DB: "MIDDLE_DB", Schema: "SCH", Tables: []string{"T3"}},
		}

		result := mustGenerate(t, req, typicalSession(), objects)
		proj := readFile(t, result.ProjectDir, "dbt_project.yml")

		assertContains(t, proj, "  db_alpha_db: ALPHA_DB", "alpha_db var present")
		assertContains(t, proj, "  db_middle_db: MIDDLE_DB", "middle_db var present")
		assertContains(t, proj, "  db_zebra_db: ZEBRA_DB", "zebra_db var present")

		// Verify alphabetical ordering: ALPHA before MIDDLE before ZEBRA.
		alphaPos := strings.Index(proj, "db_alpha_db")
		middlePos := strings.Index(proj, "db_middle_db")
		zebraPos := strings.Index(proj, "db_zebra_db")
		if !(alphaPos < middlePos && middlePos < zebraPos) {
			t.Errorf("vars not sorted alphabetically: alpha=%d middle=%d zebra=%d", alphaPos, middlePos, zebraPos)
		}
	})

	t.Run("multiple DBs: each schema uses correct var", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{
			{DB: "RAW", Schema: "INGEST", Tables: []string{"EVENTS"}},
			{DB: "ANALYTICS", Schema: "CORE", Tables: []string{"FACTS"}},
		}

		result := mustGenerate(t, req, typicalSession(), objects)
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))

		assertContains(t, sources, "database: \"{{ var('db_raw', 'RAW') }}\"", "RAW uses db_raw var")
		assertContains(t, sources, "database: \"{{ var('db_analytics', 'ANALYTICS') }}\"", "ANALYTICS uses db_analytics var")
		assertNotContains(t, sources, "database: RAW", "literal RAW must not appear")
		assertNotContains(t, sources, "database: ANALYTICS", "literal ANALYTICS must not appear")
	})

	t.Run("DatabaseVars=false: no vars block, literal database name", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: false,
		}
		objects := []SchemaObjects{{
			DB:     "MYDB",
			Schema: "PUBLIC",
			Tables: []string{"ORDERS"},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		proj := readFile(t, result.ProjectDir, "dbt_project.yml")
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))

		assertNotContains(t, proj, "vars:", "no vars block when disabled")
		assertContains(t, sources, "database: MYDB", "literal database name when disabled")
		assertNotContains(t, sources, "var(", "no var() when disabled")
	})

	t.Run("empty schemas excluded from vars", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{
			{DB: "GOODDB", Schema: "SCH", Tables: []string{"T1"}},
			// EMPTYDB has no objects → skipped from source entries and vars.
			{DB: "EMPTYDB", Schema: "SCH"},
		}

		result := mustGenerate(t, req, typicalSession(), objects)
		proj := readFile(t, result.ProjectDir, "dbt_project.yml")
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))

		assertContains(t, proj, "  db_gooddb: GOODDB", "GOODDB var present")
		assertNotContains(t, proj, "db_emptydb", "EMPTYDB must not appear in vars (no objects)")
		assertContains(t, sources, "database: \"{{ var('db_gooddb', 'GOODDB') }}\"", "GOODDB uses var")
		assertNotContains(t, sources, "EMPTYDB", "EMPTYDB must not appear in sources")

		if len(result.Warnings) != 1 {
			t.Errorf("expected 1 warning for empty schema, got %d: %v", len(result.Warnings), result.Warnings)
		}
	})

	t.Run("system schema included in vars", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{
			{DB: "MYDB", Schema: "PUBLIC", Tables: []string{"ORDERS"}},
			{DB: "MYDB", Schema: "INFORMATION_SCHEMA", IsSystem: true},
		}

		result := mustGenerate(t, req, typicalSession(), objects)
		proj := readFile(t, result.ProjectDir, "dbt_project.yml")
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))

		// Only one var entry for MYDB (deduped).
		count := strings.Count(proj, "db_mydb")
		if count != 1 {
			t.Errorf("expected exactly 1 db_mydb entry in vars, got %d", count)
		}
		// Both source entries use the var.
		assertContains(t, sources, "database: \"{{ var('db_mydb', 'MYDB') }}\"", "MYDB var in regular schema")
	})

	t.Run("two DBs same schema name: separate var entries", func(t *testing.T) {
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{
			{DB: "PROD", Schema: "PUBLIC", Tables: []string{"ORDERS"}},
			{DB: "DEV", Schema: "PUBLIC", Tables: []string{"ORDERS"}},
		}

		result := mustGenerate(t, req, typicalSession(), objects)
		proj := readFile(t, result.ProjectDir, "dbt_project.yml")

		assertContains(t, proj, "  db_prod: PROD", "PROD var")
		assertContains(t, proj, "  db_dev: DEV", "DEV var")
	})

	t.Run("var name uses original case of DB for default value", func(t *testing.T) {
		// Snowflake returns DB names in uppercase; verify the default value in
		// the var() call preserves the original case from the objects list.
		dir := t.TempDir()
		req := CreateRequest{
			ProjectName:  "proj",
			OutputDir:    dir,
			DatabaseVars: true,
		}
		objects := []SchemaObjects{{
			DB:     "MyMixedCaseDb",
			Schema: "SCH",
			Tables: []string{"T"},
		}}

		result := mustGenerate(t, req, typicalSession(), objects)
		proj := readFile(t, result.ProjectDir, "dbt_project.yml")
		sources := readFile(t, result.ProjectDir, filepath.Join("models", "staging", "_sources.yml"))

		// Var name is always lowercase; default preserves original DB string.
		assertContains(t, proj, "  db_mymixedcasedb: MyMixedCaseDb", "var preserves original DB case as default")
		assertContains(t, sources, "database: \"{{ var('db_mymixedcasedb', 'MyMixedCaseDb') }}\"", "sources var preserves case")
	})
}
