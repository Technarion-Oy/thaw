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
}

// ─── TestStagingModelSQL ──────────────────────────────────────────────────────

func TestStagingModelSQL(t *testing.T) {
	tests := []struct {
		name        string
		source      string
		table       string
		mustContain []string
		mustCount   map[string]int // substring → minimum occurrence count
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stagingModelSQL(tt.source, tt.table, "")
			for _, needle := range tt.mustContain {
				assertContains(t, got, needle, "stagingModelSQL")
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
