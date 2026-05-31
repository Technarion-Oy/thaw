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

	"thaw/internal/snowflake"
)

// Tool input types for SQL execution tools.

type executeSQLInput struct {
	SQL string `json:"sql" jsonschema:"the SQL statement to execute (single statement only)"`
}

type useRoleInput struct {
	Role string `json:"role" jsonschema:"the role name to switch to"`
}

type useWarehouseInput struct {
	Warehouse string `json:"warehouse" jsonschema:"the warehouse name to switch to"`
}

type useDatabaseInput struct {
	Database string `json:"database" jsonschema:"the database name to switch to"`
}

type useSchemaInput struct {
	Schema string `json:"schema" jsonschema:"the schema name to switch to"`
}

// registerSQLTools wires the SQL execution and context-switching tools onto
// srv. These tools are only registered in readonly and explain_only modes.
// The execute_snowflake_sql tool is the single chokepoint — every SQL
// statement passes through the EXPLAIN precompilation gate before execution.
func registerSQLTools(srv *mcpsdk.Server, client *snowflake.Client, mode string, cfg SessionConfig) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "execute_snowflake_sql",
		Description: "Execute a single read-only SQL statement against the Snowflake session. The statement is validated through the EXPLAIN precompilation gate before execution — only read-only operations (SELECT, etc.) are allowed. Multi-statement SQL and USE statements are rejected.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in executeSQLInput) (*mcpsdk.CallToolResult, any, error) {
		verdict, err := CheckGate(ctx, client, in.SQL)
		if err != nil {
			return nil, nil, fmt.Errorf("EXPLAIN gate error: %w", err)
		}
		if !verdict.Allowed {
			return jsonResult(verdict), nil, nil
		}

		// In explain_only mode, return the gate verdict (plan metadata)
		// without executing the statement.
		if mode == ExecutionModeExplainOnly {
			return jsonResult(verdict), nil, nil
		}

		// readonly mode — execute the statement.
		result, err := client.QuerySingle(ctx, in.SQL)
		if err != nil {
			return nil, nil, err
		}
		return jsonResult(result), nil, nil
	})

	// Context-switching tools — trusted, not gated. Omitted when the
	// corresponding session config pins the value.

	if !cfg.PinnedRole {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "use_role",
			Description: "Switch the active Snowflake role for this session.",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in useRoleInput) (*mcpsdk.CallToolResult, any, error) {
			if err := client.UseRole(ctx, in.Role); err != nil {
				return nil, nil, err
			}
			return textResult(fmt.Sprintf("Switched to role %s", in.Role)), nil, nil
		})
	}

	if !cfg.PinnedWarehouse {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "use_warehouse",
			Description: "Switch the active Snowflake warehouse for this session.",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in useWarehouseInput) (*mcpsdk.CallToolResult, any, error) {
			if err := client.UseWarehouse(ctx, in.Warehouse); err != nil {
				return nil, nil, err
			}
			return textResult(fmt.Sprintf("Switched to warehouse %s", in.Warehouse)), nil, nil
		})
	}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "use_database",
		Description: "Switch the active Snowflake database for this session.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in useDatabaseInput) (*mcpsdk.CallToolResult, any, error) {
		if err := client.UseDatabase(ctx, in.Database); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Switched to database %s", in.Database)), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "use_schema",
		Description: "Switch the active Snowflake schema for this session.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in useSchemaInput) (*mcpsdk.CallToolResult, any, error) {
		if err := client.UseSchema(ctx, in.Schema); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Switched to schema %s", in.Schema)), nil, nil
	})
}
