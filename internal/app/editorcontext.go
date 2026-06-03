// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import "thaw/internal/mcp"

// UpdateEditorContext sets the active editor tab and its SQL content.
// Called by the frontend when the user switches tabs.
func (a *App) UpdateEditorContext(tabID, sql string) {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.EditorContext().SetActiveTab(tabID, sql)
}

// UpdateEditorTabSQL updates the SQL content for a specific tab without
// changing the active tab. Called by the frontend on debounced text changes.
func (a *App) UpdateEditorTabSQL(tabID, sql string) {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.EditorContext().SetTabSQL(tabID, sql)
}

// UpdateQueryResult stores a condensed result summary for a tab. Called
// by the frontend after a query completes.
func (a *App) UpdateQueryResult(tabID string, columns []string, rowCount int, truncated bool, sampleRows [][]any, queryID string) {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.EditorContext().SetTabResult(tabID, &mcp.ResultSummary{
		TabID:      tabID,
		Columns:    columns,
		RowCount:   rowCount,
		Truncated:  truncated,
		SampleRows: sampleRows,
		QueryID:    queryID,
	})
}

// ClearQueryResult removes the result summary for a tab without
// affecting its SQL content. Called by the frontend when a new query
// starts executing, so that MCP clients don't see stale results from
// a previous execution.
func (a *App) ClearQueryResult(tabID string) {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.EditorContext().ClearTabResult(tabID)
}

// RemoveEditorTab removes all editor context state for a tab. Called by
// the frontend when a tab is closed.
func (a *App) RemoveEditorTab(tabID string) {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.EditorContext().RemoveTab(tabID)
}
