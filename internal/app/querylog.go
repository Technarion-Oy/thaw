// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/querylog"
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
	a.emit("querylog:state", map[string]interface{}{
		"enabled": enabled,
	})
}

// PickQueryLogExportFile opens a native save-file dialog with log/text filters
// and returns the chosen path, or an empty string if the user cancels.
func (a *App) PickQueryLogExportFile(defaultName string) string {
	path, err := a.saveFileDialog(saveDialogOpts{
		title:           "Export Query Log",
		defaultFilename: defaultName,
		filters: []fileFilter{
			{displayName: "Log Files (*.log)", pattern: "*.log"},
			{displayName: "Text Files (*.txt)", pattern: "*.txt"},
			{displayName: "All Files (*.*)", pattern: "*.*"},
		},
	})
	if err != nil {
		return ""
	}
	return path
}

// ExportQueryLog writes the assembled query-log text to path with credential
// literals masked (redactSQLSecrets). The live in-memory log and its UI panel
// stay unredacted for readability, but an exported file is the same
// bug-report-artifact class as thaw.log, so secrets are scrubbed on the way out.
// The redactor matches KEYWORD=<literal> anywhere in the text, so it works on
// the already-formatted export blob regardless of the surrounding layout. See
// issue #804 F1.
func (a *App) ExportQueryLog(path, content string) error {
	return a.SaveFile(path, redactSQLSecrets(content))
}
