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

	"thaw/internal/config"
	"thaw/internal/logger"
	"thaw/internal/snowflake"
	"thaw/internal/sqleditor"
)

// openSqlTabInput is the input schema for the open_sql_tab tool.
type openSqlTabInput struct {
	Title string `json:"title,omitempty" jsonschema:"tab title (default: AI Query)"`
	SQL   string `json:"sql" jsonschema:"the SQL text to place in the new tab"`
}

// OpenSqlTabPayload is the Wails event payload for "mcp:open-sql-tab".
// The frontend listens for this event and opens a new editor tab.
type OpenSqlTabPayload struct {
	Title   string                 `json:"title"`
	SQL     string                 `json:"sql"`
	Markers []sqleditor.DiagMarker `json:"markers"`
}

// registerTabTools wires the tab-delivery tools onto srv. If emit is nil
// (e.g. in tests without a Wails runtime), no tools are registered.
func registerTabTools(srv *mcpsdk.Server, client *snowflake.Client, emit func(string, interface{})) {
	if emit == nil {
		return
	}

	mcpsdk.AddTool(srv, &mcpsdk.Tool{
		Name: "open_sql_tab",
		Description: "Open a new editor tab in Thaw with the provided SQL. " +
			"Keyword, identifier, and function casing is applied according to the user's " +
			"editor preferences, and the SQL is validated with inline diagnostics. " +
			"The user must manually run the query (human-in-the-loop). " +
			"Returns a success message or a count of diagnostic errors.",
	}, func(ctx context.Context, _ *mcpsdk.CallToolRequest, in openSqlTabInput) (*mcpsdk.CallToolResult, any, error) {
		if in.SQL == "" {
			return nil, nil, fmt.Errorf("sql is required")
		}

		title := in.Title
		if title == "" {
			title = "AI Query"
		}
		if len(title) > 100 {
			title = title[:100]
		}

		// Load editor preferences for formatting.
		prefs := loadEditorPrefs()

		// Format before validation so marker positions match the displayed text.
		formatted := sqleditor.ApplyCasing(in.SQL, prefs.KeywordCase, prefs.IdentifierCase, prefs.FunctionCase)

		// Run the full diagnostics pipeline.
		markers := validateSQL(ctx, client, formatted)
		if markers == nil {
			markers = []sqleditor.DiagMarker{}
		}

		// Emit the event to the frontend. Recover from panics so that a
		// torn-down Wails context during shutdown doesn't kill the MCP
		// server goroutine.
		func() {
			defer func() {
				if r := recover(); r != nil {
					logger.L.Error("mcp open_sql_tab: emit panicked", "err", r)
				}
			}()
			emit("mcp:open-sql-tab", OpenSqlTabPayload{
				Title:   title,
				SQL:     formatted,
				Markers: markers,
			})
		}()

		errorCount := 0
		for _, m := range markers {
			if m.Severity == sqleditor.SeverityError {
				errorCount++
			}
		}

		if errorCount > 0 {
			return textResult(fmt.Sprintf("Tab opened with %d diagnostic error(s). The user will see inline markers.", errorCount)), nil, nil
		}
		return textResult("Tab opened successfully with no diagnostic errors."), nil, nil
	})
}

// loadEditorPrefs reads the user's editor preferences from the config file
// and back-fills defaults for any missing fields.
func loadEditorPrefs() config.EditorPrefs {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultEditorPrefs()
	}
	return config.EditorPrefsWithDefaults(cfg.Editor)
}
