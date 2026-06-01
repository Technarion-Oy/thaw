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
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
)

// maxMCPResultRows is the maximum number of rows returned from the MCP
// execute_snowflake_sql tool. This is much lower than the 50k cap in
// QuerySingle because MCP responses are serialized as JSON text content and
// sent over SSE — large payloads can exhaust memory or overwhelm the client.
const maxMCPResultRows = 1000

// maxMCPQueryLimit is the LIMIT injected into SELECT/WITH queries in readonly
// mode to prevent full-table scans. The value is intentionally conservative —
// MCP tool results are serialized as JSON text and large payloads overwhelm
// clients. The client-side maxMCPResultRows cap is still applied as a
// defense-in-depth backstop.
const maxMCPQueryLimit = 100

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

// injectLimit wraps the query with a LIMIT clause to prevent full-table scans.
// It strips a trailing semicolon before wrapping and produces:
//
//	SELECT * FROM (<query>) AS _mcp_limit LIMIT <limit>
//
// Snowflake's optimizer flattens trivial subqueries, so this does not add
// meaningful overhead.
//
// Note: ORDER BY in the inner query is not guaranteed to be preserved by the
// outer SELECT — standard SQL does not require subquery ordering to propagate.
// Snowflake often preserves it in practice, but clients should not rely on
// result ordering.
func injectLimit(sql string, limit int) string {
	trimmed := strings.TrimSpace(sql)
	trimmed = strings.TrimRight(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	return fmt.Sprintf("SELECT * FROM (%s) AS _mcp_limit LIMIT %d", trimmed, limit)
}

// executeSQLPipeline implements the SQL execution pipeline for the MCP
// execute_snowflake_sql tool. It delegates validation to [CheckGate]
// (empty check, single-statement, USE rejection, EXPLAIN USING TABULAR),
// then adds mode-specific behavior: explain_only returns the gate verdict;
// readonly injects a LIMIT wrapper and executes. If [CheckGate] returns an
// error (e.g. EXPLAIN doesn't support the statement type), the pipeline
// treats it as a rejection rather than a Go-level error.
func executeSQLPipeline(ctx context.Context, runner queryRunner, sql string, mode string) (*mcpsdk.CallToolResult, error) {
	// Run the EXPLAIN precompilation gate (empty check, single-statement,
	// USE rejection, EXPLAIN USING TABULAR). If EXPLAIN errors (e.g. on
	// SHOW, DESCRIBE, LIST, DDL), the statement is rejected as "not
	// supported" — metadata needs are served by the dedicated schema-browsing
	// tools, not raw SQL passthrough.
	verdict, err := CheckGate(ctx, runner, sql)
	if err != nil {
		return jsonResult(GateVerdict{
			Reason: fmt.Sprintf("statement not supported: %s", err),
		}), nil
	}
	if !verdict.Allowed {
		return jsonResult(verdict), nil
	}

	// explain_only mode — return the verdict without executing.
	if mode == ExecutionModeExplainOnly {
		return jsonResult(verdict), nil
	}

	// readonly mode — inject LIMIT and execute.
	if mode != ExecutionModeReadonly {
		return jsonResult(GateVerdict{
			Reason: fmt.Sprintf("unsupported execution mode: %s", mode),
		}), nil
	}
	limited := injectLimit(verdict.Statement, maxMCPQueryLimit)
	result, err := runner.QuerySingle(ctx, limited)
	if err != nil {
		return nil, err
	}
	if len(result.Rows) > maxMCPResultRows {
		result.Rows = result.Rows[:maxMCPResultRows]
		result.Truncated = true
	}
	return jsonResult(result), nil
}

// registerSQLTools wires the SQL execution and context-switching tools onto
// srv. These tools are only registered in readonly and explain_only modes.
// The execute_snowflake_sql tool is the single chokepoint — every SQL
// statement passes through the SQL execution pipeline before execution.
func registerSQLTools(srv *mcpsdk.Server, client *snowflake.Client, mode string, cfg SessionConfig) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "execute_snowflake_sql",
		Description: "Execute a single read-only SQL statement against the Snowflake session. " +
			"Every statement passes through EXPLAIN USING TABULAR before execution — only " +
			"statements whose query plan contains exclusively read-only operations are allowed. " +
			"DDL, DML, and statements that EXPLAIN does not support (SHOW, DESCRIBE, LIST, etc.) " +
			"are rejected; use the dedicated schema-browsing tools for metadata. " +
			"In readonly mode, queries are automatically limited to 100 rows; " +
			"result ordering is not guaranteed (ORDER BY may not be preserved). " +
			"Multi-statement SQL and USE statements are rejected.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in executeSQLInput) (*mcpsdk.CallToolResult, any, error) {
		result, err := executeSQLPipeline(ctx, client, in.SQL, mode)
		if err != nil {
			return nil, nil, err
		}
		return result, nil, nil
	})

	// Context-switching tools — trusted, not gated. Omitted when the
	// corresponding session config pins the value.

	if !cfg.PinnedRole {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name:        "use_role",
			Description: "Switch the active Snowflake role for this session.",
		}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in useRoleInput) (*mcpsdk.CallToolResult, any, error) {
			if in.Role == "" {
				return nil, nil, fmt.Errorf("role name is required")
			}
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
			if in.Warehouse == "" {
				return nil, nil, fmt.Errorf("warehouse name is required")
			}
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
		if in.Database == "" {
			return nil, nil, fmt.Errorf("database name is required")
		}
		if err := client.UseDatabase(ctx, in.Database); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Switched to database %s", in.Database)), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name:        "use_schema",
		Description: "Switch the active Snowflake schema for this session.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in useSchemaInput) (*mcpsdk.CallToolResult, any, error) {
		if in.Schema == "" {
			return nil, nil, fmt.Errorf("schema name is required")
		}
		if err := client.UseSchema(ctx, in.Schema); err != nil {
			return nil, nil, err
		}
		return textResult(fmt.Sprintf("Switched to schema %s", in.Schema)), nil, nil
	})
}
