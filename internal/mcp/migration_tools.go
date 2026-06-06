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
	"fmt"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/dbt"
	"thaw/internal/filesystem"
	"thaw/internal/migration"
	"thaw/internal/snowflake"
)

// Tool input types for migration and dbt project tools.

type scanMigrationSourceInput struct {
	Dir string `json:"dir" jsonschema:"the directory containing .sql files to scan"`
}

type analyzeMigrationInput struct {
	Objects  []migration.MigrationObject `json:"objects" jsonschema:"the local migration objects to compare against Snowflake"`
	Database string                      `json:"database" jsonschema:"the target Snowflake database name"`
}

type generateMigrationScriptInput struct {
	Items    []migration.MigrationDiffItem `json:"items" jsonschema:"the diff items from analyze_migration"`
	Database string                        `json:"database" jsonschema:"the target database name"`
	Strategy string                        `json:"strategy" jsonschema:"table migration strategy: in_place, blue_green_swap, view_abstraction, or destructive_rebuild"`
}

type generateDbtProjectInput struct {
	Request dbt.CreateRequest      `json:"request" jsonschema:"the dbt project creation parameters"`
	Schemas map[string][]string    `json:"schemas" jsonschema:"map of database names to lists of schema names to include"`
}

// validMigrationStrategy is the set of accepted table migration strategies.
var validMigrationStrategy = map[migration.TableMigrationStrategy]bool{
	migration.StrategyInPlace:            true,
	migration.StrategyBlueGreenSwap:      true,
	migration.StrategyViewAbstraction:    true,
	migration.StrategyDestructiveRebuild: true,
}

// registerMigrationTools wires migration diff/script tools and the dbt project
// generator onto srv. scan_migration_source and generate_dbt_project are
// workspace-gated (only registered when workspaceRoot is non-empty).
// analyze_migration and generate_migration_script are always registered.
func registerMigrationTools(srv *mcpsdk.Server, client *snowflake.Client, workspaceRoot string) {

	// ── Workspace-gated tools ───────────────────────────────────────────

	if workspaceRoot != "" {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "scan_migration_source",
			Description: "Scan a local directory for .sql files and return the DDL objects found. The directory must be inside the configured workspace root.",
		}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in scanMigrationSourceInput) (*mcpsdk.CallToolResult, any, error) {
			if in.Dir == "" {
				return nil, nil, fmt.Errorf("dir is required")
			}
			if err := filesystem.ValidateInsideOrEqual(in.Dir, workspaceRoot); err != nil {
				return nil, nil, fmt.Errorf("access denied: %w", err)
			}
			svc := migration.NewService(func(string, any) {})
			objects, err := svc.ScanSource(in.Dir)
			if err != nil {
				return nil, nil, err
			}
			return jsonResult(objects), nil, nil
		})

		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "generate_dbt_project",
			Description: "Scaffold a dbt project pre-wired to the active Snowflake connection. Discovers tables/views per schema and writes the project files. The output directory must be inside the configured workspace root.",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in generateDbtProjectInput) (*mcpsdk.CallToolResult, any, error) {
			if in.Request.ProjectName == "" {
				return nil, nil, fmt.Errorf("request.projectName is required")
			}
			if in.Request.OutputDir == "" {
				return nil, nil, fmt.Errorf("request.outputDir is required")
			}
			if len(in.Schemas) == 0 {
				return nil, nil, fmt.Errorf("schemas is required")
			}
			if err := filesystem.ValidateInsideOrEqual(in.Request.OutputDir, workspaceRoot); err != nil {
				return nil, nil, fmt.Errorf("access denied: %w", err)
			}
			if client == nil {
				return nil, nil, fmt.Errorf("no active Snowflake connection")
			}
			result, err := dbt.CreateProject(ctx, client, in.Request, in.Schemas)
			if err != nil {
				return nil, nil, err
			}
			return jsonResult(result), nil, nil
		})
	}

	// ── Always-registered tools ─────────────────────────────────────────

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "analyze_migration",
		Description: "Compare local DDL objects against a live Snowflake database and return a diff (new, changed, unchanged, removed) for each object.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in analyzeMigrationInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if len(in.Objects) == 0 {
			return nil, nil, fmt.Errorf("objects is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no active Snowflake connection")
		}
		svc := migration.NewService(func(string, any) {})
		diffItems, err := svc.Analyze(client, in.Objects, in.Database)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(diffItems), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "generate_migration_script",
		Description: "Generate a human-readable SQL migration script from diff items. Pure function — no Snowflake connection needed.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in generateMigrationScriptInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database is required")
		}
		if len(in.Items) == 0 {
			return nil, nil, fmt.Errorf("items is required")
		}
		strategy := migration.TableMigrationStrategy(in.Strategy)
		if strategy == "" {
			strategy = migration.StrategyInPlace
		}
		if !validMigrationStrategy[strategy] {
			return nil, nil, fmt.Errorf("invalid strategy %q: must be one of in_place, blue_green_swap, view_abstraction, destructive_rebuild", in.Strategy)
		}
		script := migration.GenerateScript(in.Items, in.Database, strategy)
		return textResult(script), nil, nil
	})
}
