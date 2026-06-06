// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package mcp

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/dbt"
	"thaw/internal/migration"
)

// TestMigrationToolsRegistered verifies that all 4 migration/dbt tools are
// registered in metadata, readonly, and explain_only modes when WorkspaceRoot
// is set.
func TestMigrationToolsRegistered(t *testing.T) {
	migrationTools := []string{
		"scan_migration_source",
		"analyze_migration",
		"generate_migration_script",
		"generate_dbt_project",
	}

	tmp := t.TempDir()

	for _, mode := range []string{ExecutionModeMetadata, ExecutionModeReadonly, ExecutionModeExplainOnly} {
		t.Run(mode, func(t *testing.T) {
			cfg := SessionConfig{WorkspaceRoot: tmp}
			srv := buildServer(nil, mode, cfg, nil, nil, nil)
			names := toolNames(t, srv)
			for _, tool := range migrationTools {
				if !hasToolName(names, tool) {
					t.Errorf("mode %q: expected tool %q to be registered, got tools: %v", mode, tool, names)
				}
			}
		})
	}
}

// TestMigrationWorkspaceToolsNotRegisteredWithoutRoot verifies that
// workspace-gated migration tools are NOT registered when WorkspaceRoot is
// empty, while always-registered tools are still present.
func TestMigrationWorkspaceToolsNotRegisteredWithoutRoot(t *testing.T) {
	workspaceGated := []string{
		"scan_migration_source",
		"generate_dbt_project",
	}
	alwaysRegistered := []string{
		"analyze_migration",
		"generate_migration_script",
	}

	srv := buildServer(nil, ExecutionModeMetadata, SessionConfig{}, nil, nil, nil)
	names := toolNames(t, srv)

	for _, tool := range workspaceGated {
		if hasToolName(names, tool) {
			t.Errorf("workspace-gated tool %q should NOT be registered when WorkspaceRoot is empty", tool)
		}
	}
	for _, tool := range alwaysRegistered {
		if !hasToolName(names, tool) {
			t.Errorf("always-registered tool %q should be registered even without WorkspaceRoot", tool)
		}
	}
}

// ── scan_migration_source tests ─────────────────────────────────────────────

// TestScanMigrationSourceMissingDir verifies that an empty dir string returns
// an error.
func TestScanMigrationSourceMissingDir(t *testing.T) {
	tmp := t.TempDir()
	cs := newWorkspaceTestSession(t, tmp)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "scan_migration_source",
		Arguments: scanMigrationSourceInput{Dir: ""},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty dir")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "dir is required") {
		t.Errorf("error should mention dir requirement, got: %s", text)
	}
}

// TestScanMigrationSourceOutsideWorkspace verifies that a dir outside the
// workspace root is rejected with an access-denied error.
func TestScanMigrationSourceOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	cs := newWorkspaceTestSession(t, workspace)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "scan_migration_source",
		Arguments: scanMigrationSourceInput{Dir: outside},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for dir outside workspace")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "access denied") {
		t.Errorf("error should mention access denied, got: %s", text)
	}
}

// TestScanMigrationSourceSuccess verifies that scan_migration_source returns
// objects from .sql files in a temp directory.
func TestScanMigrationSourceSuccess(t *testing.T) {
	tmp := t.TempDir()
	sqlContent := "CREATE TABLE my_table (id INT, name VARCHAR(100));\n"
	if err := os.WriteFile(filepath.Join(tmp, "schema.sql"), []byte(sqlContent), 0o644); err != nil {
		t.Fatal(err)
	}

	cs := newWorkspaceTestSession(t, tmp)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "scan_migration_source",
		Arguments: scanMigrationSourceInput{Dir: tmp},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(text, "my_table") && !strings.Contains(text, "MY_TABLE") {
		t.Errorf("expected result to contain table name, got: %s", text)
	}
}

// ── analyze_migration tests ─────────────────────────────────────────────────

// TestAnalyzeMigrationNilClient verifies that a nil client returns an error.
func TestAnalyzeMigrationNilClient(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "analyze_migration",
		Arguments: analyzeMigrationInput{
			Objects:  []migration.MigrationObject{{ObjectName: "T1", ObjectKind: "TABLE"}},
			Database: "MYDB",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "no active Snowflake connection") {
		t.Errorf("error should mention no connection, got: %s", text)
	}
}

// TestAnalyzeMigrationEmptyDatabase verifies that empty database returns an error.
func TestAnalyzeMigrationEmptyDatabase(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "analyze_migration",
		Arguments: analyzeMigrationInput{
			Objects:  []migration.MigrationObject{{ObjectName: "T1", ObjectKind: "TABLE"}},
			Database: "",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty database")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "database is required") {
		t.Errorf("error should mention database requirement, got: %s", text)
	}
}

// TestAnalyzeMigrationEmptyObjects verifies that empty objects returns an error.
func TestAnalyzeMigrationEmptyObjects(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "analyze_migration",
		Arguments: analyzeMigrationInput{
			Objects:  []migration.MigrationObject{},
			Database: "MYDB",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty objects")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "objects is required") {
		t.Errorf("error should mention objects requirement, got: %s", text)
	}
}

// ── generate_migration_script tests ─────────────────────────────────────────

// TestGenerateMigrationScriptEmptyDatabase verifies that empty database
// returns an error.
func TestGenerateMigrationScriptEmptyDatabase(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_migration_script",
		Arguments: generateMigrationScriptInput{
			Items: []migration.MigrationDiffItem{{
				Object: migration.MigrationObject{ObjectName: "T1", ObjectKind: "TABLE", DDL: "CREATE TABLE T1 (ID INT)"},
				Status: "new",
			}},
			Database: "",
			Strategy: "in_place",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty database")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "database is required") {
		t.Errorf("error should mention database requirement, got: %s", text)
	}
}

// TestGenerateMigrationScriptEmptyItems verifies that empty items returns an error.
func TestGenerateMigrationScriptEmptyItems(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_migration_script",
		Arguments: generateMigrationScriptInput{
			Items:    []migration.MigrationDiffItem{},
			Database: "MYDB",
			Strategy: "in_place",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty items")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "items is required") {
		t.Errorf("error should mention items requirement, got: %s", text)
	}
}

// TestGenerateMigrationScriptSuccess verifies that valid diff items produce a
// non-empty SQL migration script.
func TestGenerateMigrationScriptSuccess(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_migration_script",
		Arguments: generateMigrationScriptInput{
			Items: []migration.MigrationDiffItem{{
				Object: migration.MigrationObject{
					Database:   "MYDB",
					Schema:     "PUBLIC",
					ObjectName: "USERS",
					ObjectKind: "TABLE",
					DDL:        "CREATE TABLE USERS (ID INT, NAME VARCHAR(100))",
				},
				Status:   "new",
				LocalDDL: "CREATE TABLE USERS (ID INT, NAME VARCHAR(100))",
			}},
			Database: "MYDB",
			Strategy: "in_place",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(text, "CREATE TABLE") {
		t.Errorf("expected CREATE TABLE in script, got: %s", text)
	}
	if !strings.Contains(text, "Migration Script") {
		t.Errorf("expected script header, got: %s", text)
	}
}

// TestGenerateMigrationScriptInvalidStrategy verifies that an invalid strategy
// returns an error.
func TestGenerateMigrationScriptInvalidStrategy(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_migration_script",
		Arguments: generateMigrationScriptInput{
			Items: []migration.MigrationDiffItem{{
				Object: migration.MigrationObject{
					Database:   "MYDB",
					Schema:     "PUBLIC",
					ObjectName: "USERS",
					ObjectKind: "TABLE",
					DDL:        "CREATE TABLE USERS (ID INT)",
				},
				Status:   "new",
				LocalDDL: "CREATE TABLE USERS (ID INT)",
			}},
			Database: "MYDB",
			Strategy: "invalid_strategy",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for invalid strategy")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "invalid strategy") {
		t.Errorf("error should mention invalid strategy, got: %s", text)
	}
}

// TestGenerateMigrationScriptEmptyStrategyDefaultsToInPlace verifies that an
// empty strategy defaults to in_place and produces a valid script.
func TestGenerateMigrationScriptEmptyStrategyDefaultsToInPlace(t *testing.T) {
	cs := newTestSession(t)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_migration_script",
		Arguments: generateMigrationScriptInput{
			Items: []migration.MigrationDiffItem{{
				Object: migration.MigrationObject{
					Database:   "MYDB",
					Schema:     "PUBLIC",
					ObjectName: "USERS",
					ObjectKind: "TABLE",
					DDL:        "CREATE TABLE USERS (ID INT)",
				},
				Status:   "new",
				LocalDDL: "CREATE TABLE USERS (ID INT)",
			}},
			Database: "MYDB",
			Strategy: "",
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected error: %s", extractText(t, res))
	}

	text := extractText(t, res)
	if !strings.Contains(text, "Migration Script") {
		t.Errorf("expected script header, got: %s", text)
	}
}

// ── generate_dbt_project tests ──────────────────────────────────────────────

// TestGenerateDbtProjectNilClient verifies that a nil client returns an error.
func TestGenerateDbtProjectNilClient(t *testing.T) {
	tmp := t.TempDir()
	cs := newWorkspaceTestSession(t, tmp)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_dbt_project",
		Arguments: generateDbtProjectInput{
			Request: dbt.CreateRequest{
				ProjectName: "test_project",
				OutputDir:   tmp,
			},
			Schemas: map[string][]string{"MYDB": {"PUBLIC"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "no active Snowflake connection") {
		t.Errorf("error should mention no connection, got: %s", text)
	}
}

// TestGenerateDbtProjectEmptyProjectName verifies that empty project name
// returns an error.
func TestGenerateDbtProjectEmptyProjectName(t *testing.T) {
	tmp := t.TempDir()
	cs := newWorkspaceTestSession(t, tmp)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_dbt_project",
		Arguments: generateDbtProjectInput{
			Request: dbt.CreateRequest{
				ProjectName: "",
				OutputDir:   tmp,
			},
			Schemas: map[string][]string{"MYDB": {"PUBLIC"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty project name")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "projectName is required") {
		t.Errorf("error should mention projectName requirement, got: %s", text)
	}
}

// TestGenerateDbtProjectEmptyOutputDir verifies that empty output dir returns
// an error.
func TestGenerateDbtProjectEmptyOutputDir(t *testing.T) {
	tmp := t.TempDir()
	cs := newWorkspaceTestSession(t, tmp)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_dbt_project",
		Arguments: generateDbtProjectInput{
			Request: dbt.CreateRequest{
				ProjectName: "test_project",
				OutputDir:   "",
			},
			Schemas: map[string][]string{"MYDB": {"PUBLIC"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty output dir")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "outputDir is required") {
		t.Errorf("error should mention outputDir requirement, got: %s", text)
	}
}

// TestGenerateDbtProjectEmptySchemas verifies that empty schemas returns an error.
func TestGenerateDbtProjectEmptySchemas(t *testing.T) {
	tmp := t.TempDir()
	cs := newWorkspaceTestSession(t, tmp)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_dbt_project",
		Arguments: generateDbtProjectInput{
			Request: dbt.CreateRequest{
				ProjectName: "test_project",
				OutputDir:   tmp,
			},
			Schemas: map[string][]string{},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for empty schemas")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "schemas is required") {
		t.Errorf("error should mention schemas requirement, got: %s", text)
	}
}

// TestGenerateDbtProjectOutsideWorkspaceNilClient verifies that a nil client
// is rejected before path validation — preventing path-existence probing via
// differing ValidateInsideOrEqual errors when no connection is available.
func TestGenerateDbtProjectOutsideWorkspaceNilClient(t *testing.T) {
	workspace := t.TempDir()
	outside := t.TempDir()

	cs := newWorkspaceTestSession(t, workspace)
	ctx := context.Background()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{
		Name: "generate_dbt_project",
		Arguments: generateDbtProjectInput{
			Request: dbt.CreateRequest{
				ProjectName: "test_project",
				OutputDir:   outside,
			},
			Schemas: map[string][]string{"MYDB": {"PUBLIC"}},
		},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}
	if !res.IsError {
		t.Error("expected IsError=true for nil client")
	}
	text := extractText(t, res)
	if !strings.Contains(text, "no active Snowflake connection") {
		t.Errorf("nil-client check should fire before path validation, got: %s", text)
	}
}
