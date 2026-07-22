// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !windows

package filesystem

import (
	"os/exec"
	"path/filepath"
	"runtime"
)

// revealInFileManager opens the native file manager and selects abs.
//
// macOS uses `open -R` to reveal-and-select the file. Linux (and any other
// non-Windows platform) falls back to opening the containing directory with
// `xdg-open`, since most Linux file managers don't support selecting a specific
// file from the command line.
func revealInFileManager(abs string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", "-R", abs).Start() // #nosec G204 — abs is validated inside allowedRoot by the caller
	}
	// linux and others
	return exec.Command("xdg-open", filepath.Dir(abs)).Start() // #nosec G204 — abs is validated inside allowedRoot by the caller
}
