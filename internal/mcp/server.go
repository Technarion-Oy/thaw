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
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/snowflake"
	"thaw/internal/version"
)

// modeSpecificToolNames lists tools that are only registered in non-metadata
// modes. updateMode removes these before re-registering for the new mode.
// RemoveTools ignores names that aren't registered, so including use_role and
// use_warehouse (which may be absent when pinned) is harmless.
var modeSpecificToolNames = []string{
	"execute_snowflake_sql",
	"use_role",
	"use_warehouse",
	"use_database",
	"use_schema",
	"get_query_results_summary",
}

// buildServer constructs an MCP server and registers tools based on the
// execution mode. Schema-browsing and diagnostics tools are always registered.
// SQL execution tools (execute_snowflake_sql + context-switching) are only
// registered in readonly and explain_only modes. Editor context tools are
// registered when editorCtx is non-nil. Tab tools (open_sql_tab) are
// registered when emit is non-nil (i.e. running inside the app, not tests).
func buildServer(client *snowflake.Client, mode string, cfg SessionConfig, editorCtx *EditorContextStore, emit func(string, interface{})) *mcpsdk.Server {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "thaw",
		Version: version.Version,
	}, nil)

	registerTools(srv, client)
	registerSchemaTools(srv, client)
	registerAccountTools(srv, client)
	registerDiagTools(srv, client)
	registerProfileTools(srv, client)
	registerLineageTools(srv, client)
	if cfg.WorkspaceRoot != "" {
		registerWorkspaceTools(srv, cfg.WorkspaceRoot)
	}
	registerEditorTools(srv, client, mode, editorCtx)
	registerTabTools(srv, client, emit)

	if mode == ExecutionModeReadonly || mode == ExecutionModeExplainOnly {
		registerSQLTools(srv, client, mode, cfg)
	}
	return srv
}
