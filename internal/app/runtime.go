// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/session"
)

// errRuntimeNotInitialized is returned by dialog helpers when invoked without a
// live Wails context (e.g. unit tests that construct App directly).
var errRuntimeNotInitialized = errors.New("wails runtime context not initialized")

// fileFilter describes one file-type filter in native open/save dialogs.
// Kept decoupled from wailsruntime.FileFilter so callers in internal/app/ do
// not import the Wails runtime package directly.
type fileFilter struct {
	displayName string
	pattern     string
}

// openDialogOpts configures open-file, multi-file, and directory dialogs.
type openDialogOpts struct {
	title                string
	defaultDirectory     string
	filters              []fileFilter
	canCreateDirectories bool
}

// saveDialogOpts configures native save-file dialogs.
type saveDialogOpts struct {
	title           string
	defaultFilename string
	filters         []fileFilter
}

// confirmDialogOpts configures native confirmation (question) message dialogs.
type confirmDialogOpts struct {
	title         string
	message       string
	buttons       []string
	defaultButton string
	cancelButton  string
}

func toWailsFilters(filters []fileFilter) []wailsruntime.FileFilter {
	if len(filters) == 0 {
		return nil
	}
	out := make([]wailsruntime.FileFilter, len(filters))
	for i, f := range filters {
		out[i] = wailsruntime.FileFilter{
			DisplayName: f.displayName,
			Pattern:     f.pattern,
		}
	}
	return out
}

// emit sends a Wails runtime event to the frontend. It is a no-op when App has
// no Wails context (e.g. in unit tests).
func (a *App) emit(eventName string, data ...any) {
	if a.ctx == nil {
		return
	}
	wailsruntime.EventsEmit(a.ctx, eventName, data...)
}

// openFileDialog opens a native single-file picker.
func (a *App) openFileDialog(opts openDialogOpts) (string, error) {
	if a.ctx == nil {
		return "", errRuntimeNotInitialized
	}
	return wailsruntime.OpenFileDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                opts.title,
		DefaultDirectory:     opts.defaultDirectory,
		Filters:              toWailsFilters(opts.filters),
		CanCreateDirectories: opts.canCreateDirectories,
	})
}

// openMultipleFilesDialog opens a native multi-file picker.
func (a *App) openMultipleFilesDialog(opts openDialogOpts) ([]string, error) {
	if a.ctx == nil {
		return nil, errRuntimeNotInitialized
	}
	return wailsruntime.OpenMultipleFilesDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                opts.title,
		DefaultDirectory:     opts.defaultDirectory,
		Filters:              toWailsFilters(opts.filters),
		CanCreateDirectories: opts.canCreateDirectories,
	})
}

// saveFileDialog opens a native save-file dialog.
func (a *App) saveFileDialog(opts saveDialogOpts) (string, error) {
	if a.ctx == nil {
		return "", errRuntimeNotInitialized
	}
	return wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           opts.title,
		DefaultFilename: opts.defaultFilename,
		Filters:         toWailsFilters(opts.filters),
	})
}

// openDirectoryDialog opens a native folder picker.
func (a *App) openDirectoryDialog(opts openDialogOpts) (string, error) {
	if a.ctx == nil {
		return "", errRuntimeNotInitialized
	}
	return wailsruntime.OpenDirectoryDialog(a.ctx, wailsruntime.OpenDialogOptions{
		Title:                opts.title,
		DefaultDirectory:     opts.defaultDirectory,
		CanCreateDirectories: opts.canCreateDirectories,
	})
}

// confirmDialog displays a native question dialog and returns the clicked button label.
func (a *App) confirmDialog(opts confirmDialogOpts) (string, error) {
	if a.ctx == nil {
		return "", errRuntimeNotInitialized
	}
	return wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         opts.title,
		Message:       opts.message,
		Buttons:       opts.buttons,
		DefaultButton: opts.defaultButton,
		CancelButton:  opts.cancelButton,
	})
}

// restoreWindowState restores the persisted window position and maximized state.
func (a *App) restoreWindowState(st session.WindowState) {
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowSetPosition(a.ctx, st.X, st.Y)
	if st.Maximized {
		wailsruntime.WindowMaximise(a.ctx)
	}
}

// windowState captures the current window position, size, and maximized state.
func (a *App) windowState() session.WindowState {
	if a.ctx == nil {
		return session.WindowState{}
	}
	w, h := wailsruntime.WindowGetSize(a.ctx)
	x, y := wailsruntime.WindowGetPosition(a.ctx)
	m := wailsruntime.WindowIsMaximised(a.ctx)
	return session.WindowState{X: x, Y: y, Width: w, Height: h, Maximized: m}
}

// setWindowTitle updates the native window title bar text.
func (a *App) setWindowTitle(title string) {
	if a.ctx == nil {
		return
	}
	wailsruntime.WindowSetTitle(a.ctx, title)
}

// updateApplicationMenu flushes native menu mutations to the OS.
func (a *App) updateApplicationMenu() {
	if a.ctx == nil {
		return
	}
	wailsruntime.MenuUpdateApplicationMenu(a.ctx)
}

// quit terminates the Wails application.
func (a *App) quit() {
	if a.ctx == nil {
		return
	}
	wailsruntime.Quit(a.ctx)
}

// logErrorf logs a formatted error message through the Wails runtime logger.
func (a *App) logErrorf(format string, args ...any) {
	if a.ctx == nil {
		return
	}
	wailsruntime.LogErrorf(a.ctx, format, args...)
}
