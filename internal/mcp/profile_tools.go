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

	"thaw/internal/queryprofile"
	"thaw/internal/snowflake"
)

// Tool input type for the query profiling tools.

type explainInput struct {
	SQL string `json:"sql" jsonschema:"the SQL statement to explain"`
}

// registerProfileTools wires the query profiling and EXPLAIN diagnostics tools
// onto srv. Both tools are read-only metadata operations registered in every
// execution mode (metadata, readonly, explain_only).
func registerProfileTools(srv *mcpsdk.Server, client *snowflake.Client) {
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "explain_query",
		Description: "Run EXPLAIN USING TABULAR on a SQL statement and return the full plan tree " +
			"(partitions, bytes, operations) plus performance diagnostics (full scans, cartesian joins, row explosion).",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in explainInput) (*mcpsdk.CallToolResult, any, error) {
		sql := strings.TrimSpace(in.SQL)
		if sql == "" {
			return nil, nil, fmt.Errorf("sql is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		result, err := queryprofile.RunExplain(ctx, client, sql)
		if err != nil {
			return nil, nil, err
		}
		// Ensure non-nil so JSON serializes as [] not null.
		if result.Diagnostics == nil {
			result.Diagnostics = []queryprofile.ExplainMarker{}
		}
		return jsonResult(result), nil, nil
	})

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_explain_diagnostics",
		Description: "Run EXPLAIN on a SQL statement and return only the performance diagnostics " +
			"(full partition scans, cartesian joins, row explosion warnings). " +
			"Lighter than explain_query when you only need the warnings, not the full plan tree.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in explainInput) (*mcpsdk.CallToolResult, any, error) {
		sql := strings.TrimSpace(in.SQL)
		if sql == "" {
			return nil, nil, fmt.Errorf("sql is required")
		}
		if client == nil {
			return nil, nil, fmt.Errorf("no Snowflake connection available")
		}
		markers, err := queryprofile.GetExplainDiagnostics(ctx, client, sql)
		if err != nil {
			return nil, nil, err
		}
		// Ensure non-nil so JSON serializes as [] not null.
		if markers == nil {
			markers = []queryprofile.ExplainMarker{}
		}
		return jsonResult(markers), nil, nil
	})
}
