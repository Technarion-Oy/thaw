// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"
	"go/parser"
	"go/token"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"thaw/internal/session"
)

func TestToWailsFilters(t *testing.T) {
	if got := toWailsFilters(nil); got != nil {
		t.Fatalf("toWailsFilters(nil) = %v, want nil", got)
	}
	if got := toWailsFilters([]fileFilter{}); got != nil {
		t.Fatalf("toWailsFilters(empty) = %v, want nil", got)
	}

	in := []fileFilter{
		{displayName: "SQL Files (*.sql)", pattern: "*.sql"},
		{displayName: "All Files (*.*)", pattern: "*.*"},
	}
	got := toWailsFilters(in)
	if len(got) != len(in) {
		t.Fatalf("len(toWailsFilters) = %d, want %d", len(got), len(in))
	}
	for i := range in {
		if got[i].DisplayName != in[i].displayName || got[i].Pattern != in[i].pattern {
			t.Errorf("filter[%d] = %+v, want %+v", i, got[i], in[i])
		}
	}
}

func TestToOpenFileFilters(t *testing.T) {
	// Empty filters: nil + AllowsOtherFileTypes=true on all OSes.
	for _, goos := range []string{"darwin", "windows", "linux"} {
		got, allowsOther := toOpenFileFilters(nil, goos)
		if got != nil || !allowsOther {
			t.Errorf("empty/%s = (%v, %v), want (nil, true)", goos, got, allowsOther)
		}
	}

	// Filters containing a catch-all *.* entry: darwin must omit Filters so
	// OpenPanelDelegate allows arbitrary/extensionless files, while Windows/Linux
	// preserve the dropdown filters with AllowsOtherFileTypes=true.
	withAll := []fileFilter{
		{displayName: "Private Keys (*.p8;*.pem;*.key)", pattern: "*.p8;*.pem;*.key"},
		{displayName: "All Files (*.*)", pattern: "*.*"},
	}
	gotDarwin, allowsDarwin := toOpenFileFilters(withAll, "darwin")
	if gotDarwin != nil || !allowsDarwin {
		t.Errorf("withAll/darwin = (%v, %v), want (nil, true)", gotDarwin, allowsDarwin)
	}
	for _, goos := range []string{"windows", "linux"} {
		got, allows := toOpenFileFilters(withAll, goos)
		if len(got) != 2 || !allows {
			t.Errorf("withAll/%s = (%v, %v), want (2 filters, true)", goos, got, allows)
		}
	}

	// Strict extension-only filters without *.* keep Filters and AllowsOtherFileTypes=false.
	strict := []fileFilter{
		{displayName: "Jupyter Notebooks (*.ipynb)", pattern: "*.ipynb"},
	}
	for _, goos := range []string{"darwin", "windows", "linux"} {
		got, allows := toOpenFileFilters(strict, goos)
		if len(got) != 1 || allows {
			t.Errorf("strict/%s = (%v, %v), want (1 filter, false)", goos, got, allows)
		}
	}
}

func TestIsDialogCancelled(t *testing.T) {
	if isDialogCancelled(nil) {
		t.Error("isDialogCancelled(nil) = true, want false")
	}
	if !isDialogCancelled(errors.New("cancelled by user")) {
		t.Error("isDialogCancelled('cancelled by user') = false, want true")
	}
	if !isDialogCancelled(errors.New("dialog: Cancelled By User")) {
		t.Error("isDialogCancelled(mixed case) = false, want true")
	}
	if isDialogCancelled(errors.New("permission denied")) {
		t.Error("isDialogCancelled('permission denied') = true, want false")
	}
}

func TestLimitAssetConcurrencySkipsWailsPaths(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})

	mw := limitAssetConcurrency(1)
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/assets/slow.js" {
			close(started)
			<-release
		}
		w.WriteHeader(http.StatusOK)
	}))

	// Occupy the sole asset concurrency slot.
	assetDone := make(chan struct{})
	go func() {
		defer close(assetDone)
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/assets/slow.js", nil))
	}()
	<-started

	// A /wails/runtime IPC call must bypass the semaphore and finish immediately.
	ipcDone := make(chan int, 1)
	go func() {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/wails/runtime", nil))
		ipcDone <- rec.Code
	}()

	select {
	case code := <-ipcDone:
		if code != http.StatusOK {
			t.Fatalf("/wails/runtime status = %d, want 200", code)
		}
	case <-time.After(2 * time.Second):
		close(release)
		t.Fatal("/wails/runtime blocked behind saturated asset semaphore")
	}

	close(release)
	<-assetDone
}

func TestRuntimeHelpersSafeWithNilApp(t *testing.T) {
	a := &App{}

	// Void helpers must be silent no-ops without a Wails application.
	a.emit("test:event", "payload")
	a.onEvent("test:event", func(any) {})
	a.setWindowTitle("Thaw")
	a.quit()
	a.logErrorf("msg: %s", "test")

	if got := a.windowState(); got != (session.WindowState{}) {
		t.Errorf("windowState() with nil mainWindow = %+v, want zero value", got)
	}

	// Dialog helpers must return errRuntimeNotInitialized rather than panicking.
	if _, err := a.openFileDialog(openDialogOpts{title: "Open"}); !errors.Is(err, errRuntimeNotInitialized) {
		t.Errorf("openFileDialog err = %v, want %v", err, errRuntimeNotInitialized)
	}
	if _, err := a.openMultipleFilesDialog(openDialogOpts{title: "Open"}); !errors.Is(err, errRuntimeNotInitialized) {
		t.Errorf("openMultipleFilesDialog err = %v, want %v", err, errRuntimeNotInitialized)
	}
	if _, err := a.saveFileDialog(saveDialogOpts{title: "Save"}); !errors.Is(err, errRuntimeNotInitialized) {
		t.Errorf("saveFileDialog err = %v, want %v", err, errRuntimeNotInitialized)
	}
	if _, err := a.openDirectoryDialog(openDialogOpts{title: "Dir"}); !errors.Is(err, errRuntimeNotInitialized) {
		t.Errorf("openDirectoryDialog err = %v, want %v", err, errRuntimeNotInitialized)
	}
	if _, err := a.confirmDialog(confirmDialogOpts{title: "Confirm"}); !errors.Is(err, errRuntimeNotInitialized) {
		t.Errorf("confirmDialog err = %v, want %v", err, errRuntimeNotInitialized)
	}
}

// TestWailsRuntimeIsolatedToShellFiles guards the Wails runtime boundary in
// internal/app/: domain IPC binding files must never import Wails directly —
// only the desktop shell files (app.go, run.go, runtime.go, menu.go) may import
// github.com/wailsapp/wails/v3/... and no file may import Wails v2.
func TestWailsRuntimeIsolatedToShellFiles(t *testing.T) {
	allowedWailsFiles := map[string]bool{
		"app.go":     true,
		"run.go":     true,
		"runtime.go": true,
		"menu.go":    true,
	}

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}

	fset := token.NewFileSet()
	scanned := 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		scanned++
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", name, err)
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if strings.HasPrefix(p, "github.com/wailsapp/wails/v2") {
				t.Errorf("%s still imports legacy Wails v2 package %q", name, p)
			}
			if strings.HasPrefix(p, "github.com/wailsapp/wails/") && !allowedWailsFiles[name] {
				t.Errorf("%s imports %q directly — use the helpers in internal/app/runtime.go instead", name, p)
			}
		}
	}
	if scanned < 10 {
		t.Fatalf("expected to scan at least 10 Go files in internal/app, got %d", scanned)
	}
}
