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

// UpdateERDesignerState pushes the current ER designer state into the MCP
// manager's cache. Called by the frontend on mount, on debounced table
// changes, and before unmount.
func (a *App) UpdateERDesignerState(database string, tables []mcp.ERDesignerTableOut) {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.ERDesignerState().Set(&mcp.ERDesignerState{
		Database: database,
		Tables:   tables,
	})
}

// ClearERDesignerState removes the ER designer state from the MCP manager's
// cache, indicating the designer is closed. Called by the frontend on
// unmount.
func (a *App) ClearERDesignerState() {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.ERDesignerState().Clear()
}
