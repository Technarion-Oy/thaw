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

import (
	"thaw/internal/querylog"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// GetQueryLogEntries returns all entries in the session query log.
func (a *App) GetQueryLogEntries() []querylog.Entry {
	return a.queryLog.Entries()
}

// ClearQueryLog removes all entries from the session query log.
func (a *App) ClearQueryLog() {
	a.queryLog.Clear()
}

// IsQueryLogEnabled reports whether the query log is currently active.
func (a *App) IsQueryLogEnabled() bool {
	return a.queryLog.IsEnabled()
}

// SetQueryLogEnabled enables or disables the query log and emits a state event.
func (a *App) SetQueryLogEnabled(enabled bool) {
	a.queryLog.SetEnabled(enabled)
	wailsruntime.EventsEmit(a.ctx, "querylog:state", map[string]interface{}{
		"enabled": enabled,
		"filter":  a.queryLog.Filter(),
	})
}

// GetQueryLogFilter returns the current source filter ("all", "user", "internal").
func (a *App) GetQueryLogFilter() string {
	return a.queryLog.Filter()
}

// PickQueryLogExportFile opens a native save-file dialog with log/text filters
// and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickQueryLogExportFile(defaultName string) string {
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Export Query Log",
		DefaultFilename: defaultName,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "Log Files (*.log)", Pattern: "*.log"},
			{DisplayName: "Text Files (*.txt)", Pattern: "*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// SetQueryLogFilter sets the source filter and emits a state event.
func (a *App) SetQueryLogFilter(filter string) {
	a.queryLog.SetFilter(filter)
	wailsruntime.EventsEmit(a.ctx, "querylog:state", map[string]interface{}{
		"enabled": a.queryLog.IsEnabled(),
		"filter":  filter,
	})
}
