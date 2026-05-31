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

// buildServer constructs an MCP server and registers tools based on the
// execution mode. Schema-browsing and diagnostics tools are always registered.
// SQL execution tools (execute_snowflake_sql + context-switching) are only
// registered in readonly and explain_only modes.
func buildServer(client *snowflake.Client, mode string, cfg SessionConfig) *mcpsdk.Server {
	srv := mcpsdk.NewServer(&mcpsdk.Implementation{
		Name:    "thaw",
		Version: version.Version,
	}, nil)

	registerTools(srv, client)
	registerDiagTools(srv, client)

	if mode == ExecutionModeReadonly || mode == ExecutionModeExplainOnly {
		registerSQLTools(srv, client, mode, cfg)
	}
	return srv
}
