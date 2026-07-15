// SPDX-License-Identifier: GPL-3.0-or-later

package dbtproject

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

// ── BuildCreateDbtProjectSql ──────────────────────────────────────────────────

func TestBuildCreateDbtProjectSql_BasicRequired(t *testing.T) {
	cfg := CreateConfig{
		Name:           "MY_PROJECT",
		SourceLocation: "@mystage/dbt",
	}
	sql, err := BuildCreateDbtProjectSql("MYDB", "MYSCHEMA", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE DBT PROJECT`)
	assertContains(t, sql, `"MYDB"."MYSCHEMA".MY_PROJECT`)
	assertContains(t, sql, `FROM '@mystage/dbt'`)
	assertNotContains(t, sql, `OR REPLACE`)
	assertNotContains(t, sql, `IF NOT EXISTS`)
	assertNotContains(t, sql, `DBT_VERSION`)
	assertNotContains(t, sql, `DEFAULT_TARGET`)
	assertNotContains(t, sql, `COMMENT`)
	assertNotContains(t, sql, `EXTERNAL_ACCESS_INTEGRATIONS`)
}

func TestBuildCreateDbtProjectSql_OrReplace(t *testing.T) {
	cfg := CreateConfig{
		Name:           "MY_PROJECT",
		OrReplace:      true,
		SourceLocation: "@mystage/dbt",
	}
	sql, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE OR REPLACE DBT PROJECT`)
	assertNotContains(t, sql, `IF NOT EXISTS`)
}

func TestBuildCreateDbtProjectSql_IfNotExists(t *testing.T) {
	cfg := CreateConfig{
		Name:           "MY_PROJECT",
		IfNotExists:    true,
		SourceLocation: "@mystage/dbt",
	}
	sql, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE DBT PROJECT IF NOT EXISTS`)
}

func TestBuildCreateDbtProjectSql_OrReplace_IfNotExists_Ignored(t *testing.T) {
	cfg := CreateConfig{
		Name:           "MY_PROJECT",
		OrReplace:      true,
		IfNotExists:    true,
		SourceLocation: "@mystage/dbt",
	}
	sql, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `CREATE OR REPLACE DBT PROJECT`)
	assertNotContains(t, sql, `IF NOT EXISTS`)
}

func TestBuildCreateDbtProjectSql_AllOptions(t *testing.T) {
	cfg := CreateConfig{
		Name:                       "MY_PROJECT",
		SourceLocation:             "@mystage/dbt",
		DbtVersion:                 "1.8.0",
		DefaultTarget:              "prod",
		ExternalAccessIntegrations: []string{"EAI_1", "EAI_2"},
		Comment:                    "my dbt project",
	}
	sql, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `FROM '@mystage/dbt'`)
	assertContains(t, sql, `DBT_VERSION = '1.8.0'`)
	assertContains(t, sql, `DEFAULT_TARGET = 'prod'`)
	assertContains(t, sql, `EXTERNAL_ACCESS_INTEGRATIONS = ("EAI_1", "EAI_2")`)
	assertContains(t, sql, `COMMENT = 'my dbt project'`)
}

func TestBuildCreateDbtProjectSql_CaseSensitive(t *testing.T) {
	cfg := CreateConfig{
		Name:           "My_Project",
		CaseSensitive:  true,
		SourceLocation: "@mystage/dbt",
	}
	sql, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `"My_Project"`)
}

func TestBuildCreateDbtProjectSql_MissingSourceLocation(t *testing.T) {
	cfg := CreateConfig{
		Name: "MY_PROJECT",
	}
	_, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err == nil {
		t.Fatal("expected error for missing SourceLocation")
	}
}

func TestBuildCreateDbtProjectSql_EmptyName(t *testing.T) {
	cfg := CreateConfig{
		SourceLocation: "@mystage/dbt",
	}
	sql, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `project_name`)
}

func TestBuildCreateDbtProjectSql_EscapedLiterals(t *testing.T) {
	cfg := CreateConfig{
		Name:           "MY_PROJECT",
		SourceLocation: "@stage/it's",
		Comment:        "it's a test",
	}
	sql, err := BuildCreateDbtProjectSql("DB", "SC", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `FROM '@stage/it''s'`)
	assertContains(t, sql, `COMMENT = 'it''s a test'`)
}

// ── BuildAlterDbtProjectSetSql ────────────────────────────────────────────────

func TestBuildAlterDbtProjectSetSql_SetDbtVersion(t *testing.T) {
	cfg := AlterSetConfig{
		DbtVersion: "1.9.0",
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "", "1.8.0", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `ALTER DBT PROJECT`)
	assertContains(t, stmts[0], `DBT_VERSION = '1.9.0'`)
}

func TestBuildAlterDbtProjectSetSql_UnsetDbtVersion(t *testing.T) {
	cfg := AlterSetConfig{
		DbtVersion: "", // cleared
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "", "1.8.0", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 UNSET statement, got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `UNSET DBT_VERSION`)
}

func TestBuildAlterDbtProjectSetSql_UnsetComment(t *testing.T) {
	cfg := AlterSetConfig{
		Comment: "", // cleared
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "old comment", "", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 UNSET statement, got %d", len(stmts))
	}
	assertContains(t, stmts[0], `UNSET COMMENT`)
}

func TestBuildAlterDbtProjectSetSql_SetAndUnset(t *testing.T) {
	cfg := AlterSetConfig{
		DbtVersion:    "2.0.0",
		DefaultTarget: "", // cleared
		Comment:       "", // cleared
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "old comment", "1.8.0", "prod", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 2 {
		t.Fatalf("expected 2 statements (SET + UNSET), got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `SET`)
	assertContains(t, stmts[0], `DBT_VERSION = '2.0.0'`)
	assertContains(t, stmts[1], `UNSET`)
	assertContains(t, stmts[1], `DEFAULT_TARGET`)
	assertContains(t, stmts[1], `COMMENT`)
}

func TestBuildAlterDbtProjectSetSql_NoChanges(t *testing.T) {
	cfg := AlterSetConfig{
		DbtVersion:    "1.8.0",
		DefaultTarget: "prod",
		Comment:       "same comment",
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "same comment", "1.8.0", "prod", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 0 {
		t.Fatalf("expected 0 statements (no changes), got %d: %v", len(stmts), stmts)
	}
}

func TestBuildAlterDbtProjectSetSql_ChangeIntegrations(t *testing.T) {
	cfg := AlterSetConfig{
		ExternalAccessIntegrations: []string{"NEW_EAI"},
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "", "", "", []string{"OLD_EAI"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 statement, got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `EXTERNAL_ACCESS_INTEGRATIONS = ("NEW_EAI")`)
}

func TestBuildAlterDbtProjectSetSql_UnsetIntegrations(t *testing.T) {
	cfg := AlterSetConfig{
		ExternalAccessIntegrations: nil, // cleared
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "", "", "", []string{"OLD_EAI"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 1 {
		t.Fatalf("expected 1 UNSET statement, got %d: %v", len(stmts), stmts)
	}
	assertContains(t, stmts[0], `UNSET EXTERNAL_ACCESS_INTEGRATIONS`)
}

func TestBuildAlterDbtProjectSetSql_SameIntegrations_NoChanges(t *testing.T) {
	cfg := AlterSetConfig{
		ExternalAccessIntegrations: []string{"EAI_A", "EAI_B"},
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "", "", "", []string{"EAI_A", "EAI_B"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 0 {
		t.Fatalf("expected 0 statements (same integrations), got %d: %v", len(stmts), stmts)
	}
}

func TestBuildAlterDbtProjectSetSql_SameIntegrations_CaseInsensitive(t *testing.T) {
	cfg := AlterSetConfig{
		ExternalAccessIntegrations: []string{"eai_1", "Eai_2"},
	}
	stmts, err := BuildAlterDbtProjectSetSql("DB", "SC", "PROJ", cfg, "", "", "", []string{"EAI_1", "EAI_2"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(stmts) != 0 {
		t.Fatalf("expected 0 statements (same integrations, different casing), got %d: %v", len(stmts), stmts)
	}
}

// ── BuildExecuteDbtProjectSql ─────────────────────────────────────────────────

func TestBuildExecuteDbtProjectSql_Basic(t *testing.T) {
	cfg := ExecuteConfig{}
	sql, err := BuildExecuteDbtProjectSql("DB", "SC", "PROJ", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `EXECUTE DBT PROJECT "DB"."SC"."PROJ"`)
	assertNotContains(t, sql, `ARGS`)
	assertNotContains(t, sql, `DBT_VERSION`)
	assertNotContains(t, sql, `FROM WORKSPACE`)
}

func TestBuildExecuteDbtProjectSql_WithArgs(t *testing.T) {
	cfg := ExecuteConfig{
		Args:       "run --models my_model",
		DbtVersion: "1.8.0",
	}
	sql, err := BuildExecuteDbtProjectSql("DB", "SC", "PROJ", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `ARGS = 'run --models my_model'`)
	assertContains(t, sql, `DBT_VERSION = '1.8.0'`)
}

func TestBuildExecuteDbtProjectSql_FromWorkspace(t *testing.T) {
	cfg := ExecuteConfig{
		FromWorkspace: "MY_WS",
		ProjectRoot:   "/project",
		Args:          "run",
	}
	sql, err := BuildExecuteDbtProjectSql("DB", "SC", "PROJ", cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `EXECUTE DBT PROJECT`)
	assertContains(t, sql, `FROM WORKSPACE "MY_WS"`)
	assertContains(t, sql, `PROJECT_ROOT = '/project'`)
	assertContains(t, sql, `ARGS = 'run'`)
	assertNotContains(t, sql, `"DB"."SC"."PROJ"`)
}

// ── BuildAddVersionSql ────────────────────────────────────────────────────────

func TestBuildAddVersionSql_WithAlias(t *testing.T) {
	sql, err := BuildAddVersionSql("DB", "SC", "PROJ", "v1.0", "@stage/src")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `ALTER DBT PROJECT "DB"."SC"."PROJ" ADD VERSION "v1.0"`)
	assertContains(t, sql, `FROM '@stage/src'`)
}

func TestBuildAddVersionSql_WithoutAlias(t *testing.T) {
	sql, err := BuildAddVersionSql("DB", "SC", "PROJ", "", "@stage/src")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertContains(t, sql, `ALTER DBT PROJECT "DB"."SC"."PROJ" ADD VERSION`)
	assertNotContains(t, sql, `""`) // no empty alias
	assertContains(t, sql, `FROM '@stage/src'`)
}

func TestBuildAddVersionSql_MissingSourceLocation(t *testing.T) {
	_, err := BuildAddVersionSql("DB", "SC", "PROJ", "v1.0", "")
	if err == nil {
		t.Fatal("expected error for missing sourceLocation")
	}
}

// ── BuildDescribeSql ──────────────────────────────────────────────────────────

func TestBuildDescribeSql(t *testing.T) {
	sql := BuildDescribeSql("DB", "SC", "PROJ")
	assertContains(t, sql, `DESCRIBE DBT PROJECT "DB"."SC"."PROJ";`)
}
