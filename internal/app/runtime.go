// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"
	"fmt"
	goruntime "runtime"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"

	"thaw/internal/session"
)

// errRuntimeNotInitialized is returned by dialog helpers when invoked without a
// live Wails application (e.g. unit tests that construct App directly).
var errRuntimeNotInitialized = errors.New("wails runtime not initialized")

// fileFilter describes one file-type filter in native open/save dialogs.
// Kept decoupled from application.FileFilter so domain binding files in
// internal/app/ do not import Wails directly.
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

func toWailsFilters(filters []fileFilter) []application.FileFilter {
	if len(filters) == 0 {
		return nil
	}
	out := make([]application.FileFilter, len(filters))
	for i, f := range filters {
		out[i] = application.FileFilter{
			DisplayName: f.displayName,
			Pattern:     f.pattern,
		}
	}
	return out
}

func hasWildcardFilter(filters []fileFilter) bool {
	for _, f := range filters {
		for _, part := range strings.Split(f.pattern, ";") {
			p := strings.TrimSpace(part)
			if p == "*" || p == "*.*" {
				return true
			}
		}
	}
	return false
}

// toOpenFileFilters converts dialog filters for Wails v3 open-file dialogs and
// returns (filters, allowsOtherFileTypes).
//
// On macOS, Wails v3's OpenPanelDelegate checks [filename hasSuffix:allowedExt]
// whenever Filters is non-empty and treats "*.*" as literal suffix ".*", greying
// out arbitrary and extensionless files (e.g. ~/.ssh/id_ed25519). When filters
// are empty or include a catch-all "*.*"/"*" entry, omitting Filters on darwin
// lets OpenPanelDelegate enable every file while Windows/Linux keep the full
// filter dropdown.
func toOpenFileFilters(filters []fileFilter, goos string) ([]application.FileFilter, bool) {
	if len(filters) == 0 {
		return nil, true
	}
	if hasWildcardFilter(filters) {
		if goos == "darwin" {
			return nil, true
		}
		return toWailsFilters(filters), true
	}
	return toWailsFilters(filters), false
}

// isDialogCancelled reports whether err represents a user cancellation from a
// native file/directory dialog. On Windows, Wails v3 returns cfd.ErrorCancelled
// (British-spelled upstream error string) whereas macOS and Linux return
// ("", nil); normalizing here keeps all platforms consistent for callers.
func isDialogCancelled(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(strings.ToLower(err.Error()), "cancelled by user") //nolint:misspell // exact upstream cfd.ErrorCancelled message
}

// emit sends a Wails runtime event to the frontend. It is a no-op when App has
// no Wails application (e.g. in unit tests).
func (a *App) emit(eventName string, data ...any) {
	if a.wailsApp == nil {
		return
	}
	a.wailsApp.Event.Emit(eventName, data...)
}

// onEvent subscribes to a Wails custom event emitted from the frontend. It is a
// no-op when App has no Wails application.
func (a *App) onEvent(eventName string, callback func(data any)) {
	if a.wailsApp == nil {
		return
	}
	a.wailsApp.Event.On(eventName, func(ev *application.CustomEvent) {
		callback(ev.Data)
	})
}

// openFileDialog opens a native single-file picker.
func (a *App) openFileDialog(opts openDialogOpts) (string, error) {
	if a.wailsApp == nil {
		return "", errRuntimeNotInitialized
	}
	wailsFilters, allowsOther := toOpenFileFilters(opts.filters, goruntime.GOOS)
	dopts := &application.OpenFileDialogOptions{
		Title:                opts.title,
		Message:              opts.title,
		Directory:            opts.defaultDirectory,
		CanChooseFiles:       true,
		CanChooseDirectories: false,
		CanCreateDirectories: opts.canCreateDirectories,
		Filters:              wailsFilters,
		AllowsOtherFileTypes: allowsOther,
	}
	if a.mainWindow != nil {
		dopts.Window = a.mainWindow
	}
	path, err := a.wailsApp.Dialog.OpenFileWithOptions(dopts).PromptForSingleSelection()
	if isDialogCancelled(err) {
		return "", nil
	}
	return path, err
}

// openMultipleFilesDialog opens a native multi-file picker.
func (a *App) openMultipleFilesDialog(opts openDialogOpts) ([]string, error) {
	if a.wailsApp == nil {
		return nil, errRuntimeNotInitialized
	}
	wailsFilters, allowsOther := toOpenFileFilters(opts.filters, goruntime.GOOS)
	dopts := &application.OpenFileDialogOptions{
		Title:                   opts.title,
		Message:                 opts.title,
		Directory:               opts.defaultDirectory,
		CanChooseFiles:          true,
		CanChooseDirectories:    false,
		CanCreateDirectories:    opts.canCreateDirectories,
		AllowsMultipleSelection: true,
		Filters:                 wailsFilters,
		AllowsOtherFileTypes:    allowsOther,
	}
	if a.mainWindow != nil {
		dopts.Window = a.mainWindow
	}
	paths, err := a.wailsApp.Dialog.OpenFileWithOptions(dopts).PromptForMultipleSelection()
	if isDialogCancelled(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	if paths == nil {
		return []string{}, nil
	}
	return paths, nil
}

// saveFileDialog opens a native save-file dialog.
func (a *App) saveFileDialog(opts saveDialogOpts) (string, error) {
	if a.wailsApp == nil {
		return "", errRuntimeNotInitialized
	}
	dopts := &application.SaveFileDialogOptions{
		Title:                opts.title,
		Message:              opts.title,
		Filename:             opts.defaultFilename,
		CanCreateDirectories: true,
		Filters:              toWailsFilters(opts.filters),
	}
	if a.mainWindow != nil {
		dopts.Window = a.mainWindow
	}
	path, err := a.wailsApp.Dialog.SaveFileWithOptions(dopts).PromptForSingleSelection()
	if isDialogCancelled(err) {
		return "", nil
	}
	return path, err
}

// openDirectoryDialog opens a native folder picker.
func (a *App) openDirectoryDialog(opts openDialogOpts) (string, error) {
	if a.wailsApp == nil {
		return "", errRuntimeNotInitialized
	}
	dopts := &application.OpenFileDialogOptions{
		Title:                opts.title,
		Message:              opts.title,
		Directory:            opts.defaultDirectory,
		CanChooseFiles:       false,
		CanChooseDirectories: true,
		CanCreateDirectories: opts.canCreateDirectories,
	}
	if a.mainWindow != nil {
		dopts.Window = a.mainWindow
	}
	path, err := a.wailsApp.Dialog.OpenFileWithOptions(dopts).PromptForSingleSelection()
	if isDialogCancelled(err) {
		return "", nil
	}
	return path, err
}

// confirmDialog displays a native question dialog and returns the clicked button label.
func (a *App) confirmDialog(opts confirmDialogOpts) (string, error) {
	if a.wailsApp == nil {
		return "", errRuntimeNotInitialized
	}
	if len(opts.buttons) == 0 {
		return opts.cancelButton, nil
	}
	dlg := a.wailsApp.Dialog.Question().
		SetTitle(opts.title).
		SetMessage(opts.message)

	// Deliberately do NOT AttachToWindow: on macOS an attached alert runs
	// asynchronously via beginSheetModalForWindow and Show() returns before the
	// user clicks, whereas an unattached alert runs synchronously via runModal.
	// On Linux (GTK4), Show() dispatches asynchronously and returns immediately;
	// marking a cancel button ensures Escape and window-close also invoke the
	// button callback, so waiting on <-resultCh works on all platforms.
	resultCh := make(chan string, 1)
	var once sync.Once
	cancelAssigned := false
	var lastBtn *application.Button
	for _, label := range opts.buttons {
		lbl := label
		btn := dlg.AddButton(lbl).OnClick(func() {
			once.Do(func() {
				resultCh <- lbl
			})
		})
		if lbl == opts.defaultButton {
			btn.SetAsDefault()
		}
		if lbl == opts.cancelButton {
			btn.SetAsCancel()
			cancelAssigned = true
		}
		lastBtn = btn
	}
	if !cancelAssigned && lastBtn != nil {
		lastBtn.SetAsCancel()
	}

	dlg.Show()

	return <-resultCh, nil
}

// windowState captures the current window position, size, and maximized state.
func (a *App) windowState() session.WindowState {
	if a.mainWindow == nil {
		return session.WindowState{}
	}
	w, h := a.mainWindow.Size()
	x, y := a.mainWindow.Position()
	m := a.mainWindow.IsMaximised()
	return session.WindowState{X: x, Y: y, Width: w, Height: h, Maximized: m}
}

// setWindowTitle updates the native window title bar text.
func (a *App) setWindowTitle(title string) {
	if a.mainWindow == nil {
		return
	}
	a.mainWindow.SetTitle(title)
}

// quit terminates the Wails application.
func (a *App) quit() {
	if a.wailsApp == nil {
		return
	}
	a.wailsApp.Quit()
}

// logErrorf logs a formatted error message through the Wails runtime logger.
func (a *App) logErrorf(format string, args ...any) {
	if a.wailsApp == nil || a.wailsApp.Logger == nil {
		return
	}
	a.wailsApp.Logger.Error(fmt.Sprintf(format, args...))
}
