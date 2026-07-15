// SPDX-License-Identifier: GPL-3.0-or-later

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

// SetQueryLogEnabled enables or disables the query log, emits a state event,
// and keeps the native menu checkbox in sync.
func (a *App) SetQueryLogEnabled(enabled bool) {
	a.queryLog.SetEnabled(enabled)
	if a.setQueryLogMenuCheck != nil {
		a.setQueryLogMenuCheck(enabled)
	}
	wailsruntime.EventsEmit(a.ctx, "querylog:state", map[string]interface{}{
		"enabled": enabled,
	})
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

