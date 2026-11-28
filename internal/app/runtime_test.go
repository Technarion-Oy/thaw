// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"thaw/internal/session"
)

func TestToWailsFilters(t *testing.T) {
	// Passing nil/empty filters must stay nil so PickAnyFile keeps
	// allowsOtherFileTypes enabled on macOS.
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

func TestRuntimeHelpersSafeWithNilContext(t *testing.T) {
	a := &App{}

	// Void helpers must be silent no-ops without a Wails runtime context.
	a.emit("test:event", "payload")
	a.restoreWindowState(session.WindowState{X: 10, Y: 20, Maximized: true})
	a.setWindowTitle("Thaw")
	a.updateApplicationMenu()
	a.quit()
	a.logErrorf("msg: %s", "test")

	if got := a.windowState(); got != (session.WindowState{}) {
		t.Errorf("windowState() with nil ctx = %+v, want zero value", got)
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
	if _, err := a.confirmDialog(nil, confirmDialogOpts{title: "Confirm"}); !errors.Is(err, errRuntimeNotInitialized) {
		t.Errorf("confirmDialog err = %v, want %v", err, errRuntimeNotInitialized)
	}
}

// TestWailsRuntimeIsolatedToRuntimeFile guards the Wails runtime boundary in
// internal/app/: every call into github.com/wailsapp/wails/v2/pkg/runtime must
// go through the thin helpers in runtime.go so the Wails v3 cutover (#23) only
// touches one file in this package.
func TestWailsRuntimeIsolatedToRuntimeFile(t *testing.T) {
	const wailsRuntimeImport = "github.com/wailsapp/wails/v2/pkg/runtime"

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
		if name == "runtime.go" {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(".", name), nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", name, err)
		}
		for _, imp := range f.Imports {
			p, err := strconv.Unquote(imp.Path.Value)
			if err != nil {
				continue
			}
			if p == wailsRuntimeImport {
				t.Errorf("%s imports %q directly — use the helpers in internal/app/runtime.go instead", name, wailsRuntimeImport)
			}
		}
	}
	if scanned < 10 {
		t.Fatalf("expected to scan at least 10 Go files in internal/app, got %d", scanned)
	}
}
