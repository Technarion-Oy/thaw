// SPDX-License-Identifier: GPL-3.0-or-later

package app

import "thaw/internal/mcp"

// UpdateOpenFormState pushes the open create dialog's field values into the
// MCP manager's cache (get_open_form / fill_open_form). Called by the
// frontend's useMcpFormSync hook on mount and on debounced changes.
func (a *App) UpdateOpenFormState(state mcp.OpenFormState) {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.OpenForm().Set(&state)
}

// ClearOpenFormState marks no form as open. Called on dialog unmount.
func (a *App) ClearOpenFormState() {
	if a.mcpManager == nil {
		return
	}
	a.mcpManager.OpenForm().Clear()
}
