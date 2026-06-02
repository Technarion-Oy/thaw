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
	"log"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"thaw/internal/queryhistory"
	"thaw/internal/snowflake"
)

// Tool input types for the editor context tools.

type resultSummaryInput struct {
	TabID string `json:"tabId,omitempty" jsonschema:"optional tab ID; empty = active tab"`
}

type queryHistoryInput struct {
	Limit int `json:"limit,omitempty" jsonschema:"max entries to return (default 20, max 50)"`
}

// registerEditorTools wires the editor context tools onto srv. If
// editorCtx is nil, no tools are registered (graceful degradation when
// the store is not available, e.g. in tests).
func registerEditorTools(srv *mcpsdk.Server, client *snowflake.Client, mode string, editorCtx *EditorContextStore) {
	if editorCtx == nil {
		return
	}

	// get_current_editor_sql — available in all modes.
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_current_editor_sql",
		Description: "Return the SQL text currently visible in the user's active editor tab. " +
			"Use this to understand what query the user is working on.",
	}, func(_ context.Context, _ *mcpsdk.CallToolRequest, _ emptyInput) (*mcpsdk.CallToolResult, any, error) {
		sql, ok := editorCtx.ActiveEditorSQL()
		if !ok || sql == "" {
			return textResult("No SQL is currently in the active editor tab."), nil, nil
		}
		return textResult(sql), nil, nil
	})

	// get_query_results_summary — suppressed in metadata mode because it
	// exposes actual data rows.
	if mode == ExecutionModeReadonly || mode == ExecutionModeExplainOnly {
		mcpsdk.AddTool(srv, &mcpsdk.Tool{
			Name: "get_query_results_summary",
			Description: "Return a summary of the latest query result in the editor, including " +
				"column names, row count, and a sample of up to 5 rows. " +
				"Optionally specify a tab ID; defaults to the active tab.",
		}, func(_ context.Context, _ *mcpsdk.CallToolRequest, in resultSummaryInput) (*mcpsdk.CallToolResult, any, error) {
			summary := editorCtx.QueryResultSummary(in.TabID)
			if summary == nil {
				return textResult("No query results available for the requested tab."), nil, nil
			}
			return jsonResult(summary), nil, nil
		})
	}

	// get_query_history — available in all modes. Uses the MCP session's
	// own Snowflake client to query INFORMATION_SCHEMA.QUERY_HISTORY.
	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "get_query_history",
		Description: "Return the user's recent Snowflake query history (up to 50 entries), " +
			"ordered by start time descending. Includes query ID, SQL text, status, " +
			"elapsed time, rows produced, and error messages.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in queryHistoryInput) (*mcpsdk.CallToolResult, any, error) {
		if client == nil {
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{&mcpsdk.TextContent{Text: "no Snowflake connection available"}},
				IsError: true,
			}, nil, nil
		}

		limit := in.Limit
		if limit <= 0 {
			limit = 20
		}
		if limit > 50 {
			limit = 50
		}

		rows, err := queryhistory.GetQueryHistory(
			ctx, client,
			"user", // filter by current user
			"",     // sessionID (unused for user filter)
			"",     // userName — empty uses current user
			"",     // warehouseName
			"",     // endTimeStart
			"",     // endTimeEnd
			limit,
			false, // exclude client-generated statements
		)
		if err != nil {
			// Log the full error for debugging but return a generic
			// message to MCP clients to avoid leaking Snowflake
			// connection details, role names, or warehouse names.
			log.Printf("get_query_history: %v", err)
			return &mcpsdk.CallToolResult{
				Content: []mcpsdk.Content{&mcpsdk.TextContent{
					Text: "failed to fetch query history",
				}},
				IsError: true,
			}, nil, nil
		}

		// Project to the condensed QueryHistoryEntry type.
		entries := make([]QueryHistoryEntry, len(rows))
		for i, r := range rows {
			entries[i] = QueryHistoryEntry{
				QueryID:      r.QueryID,
				QueryText:    r.QueryText,
				Status:       r.Status,
				ErrorMessage: r.ErrorMessage,
				ElapsedMs:    r.ElapsedMs,
				RowsProduced: r.RowsProduced,
				StartTime:    r.StartTime,
				DatabaseName: r.DatabaseName,
				SchemaName:   r.SchemaName,
			}
		}
		return jsonResult(entries), nil, nil
	})
}
