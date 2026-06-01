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
func injectLimit(sql string, limit int) string {
	trimmed := strings.TrimSpace(sql)
	trimmed = strings.TrimRight(trimmed, ";")
	trimmed = strings.TrimSpace(trimmed)
	return fmt.Sprintf("SELECT * FROM (%s) AS _mcp_limit LIMIT %d", trimmed, limit)
}

// executeSQLPipeline implements the SQL execution pipeline for the MCP
// execute_snowflake_sql tool. It is extracted from the tool handler for
// testability.
//
// Principle: no raw SQL reaches Snowflake without passing through EXPLAIN
// USING TABULAR first. The EXPLAIN gate is the authoritative safety layer —
// Snowflake's own query planner determines whether the statement is read-only,
// not fragile keyword heuristics.
//
// Pipeline steps:
//  1. Empty/whitespace check
//  2. Single-statement check (SplitStatements)
//  3. USE statement rejection (best-effort early check; EXPLAIN is the backstop)
//  4. EXPLAIN USING TABULAR gate (checkExplainPlan) — if EXPLAIN itself errors,
//     the statement is not supported and is rejected
//  5. If explain_only mode: return the verdict without executing
//  6. If readonly mode: inject LIMIT, execute, apply row cap
func executeSQLPipeline(ctx context.Context, runner queryRunner, sql string, mode string) (*mcpsdk.CallToolResult, error) {
	// Step 1: Empty/whitespace check.
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return jsonResult(GateVerdict{Reason: "empty SQL"}), nil
	}

	// Step 2: Single-statement check.
	stmts := snowflake.SplitStatements(trimmed)
	if len(stmts) != 1 {
		return jsonResult(GateVerdict{
			Reason: fmt.Sprintf("multi-statement SQL not allowed (%d statements)", len(stmts)),
		}), nil
	}
	stmt := stmts[0]

	// Step 3: USE statement → reject (best-effort traceability; EXPLAIN
	// would also catch this, but the error message is clearer here).
	if isUSEStatement(stmt) {
		return jsonResult(GateVerdict{
			Reason: "USE statements are not allowed; use the dedicated context-switching tools",
		}), nil
	}

	// Step 4: EXPLAIN USING TABULAR gate. If EXPLAIN errors (e.g. on SHOW,
	// DESCRIBE, LIST, or any unsupported statement type), the statement is
	// rejected — no raw SQL bypasses the gate. Metadata needs are served by
	// the dedicated schema-browsing tools (list_databases, describe_table, etc.).
	verdict, err := checkExplainPlan(ctx, runner, stmt)
	if err != nil {
		return jsonResult(GateVerdict{
			Reason: fmt.Sprintf("statement not supported: %s", err),
		}), nil
	}
	if !verdict.Allowed {
		return jsonResult(verdict), nil
	}

	// Step 5: explain_only mode — return the verdict without executing.
	if mode == ExecutionModeExplainOnly {
		return jsonResult(verdict), nil
	}

	// Step 6: readonly mode — inject LIMIT and execute.
	limited := injectLimit(stmt, maxMCPQueryLimit)
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
			"In readonly mode, queries are automatically limited to 100 rows. " +
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
