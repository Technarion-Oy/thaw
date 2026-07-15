// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !dev

package logger

import (
	"os"
	"path/filepath"
	"runtime"
)

const devMode = false

// logFilePath returns the OS-specific log file path for production builds.
//
//   - macOS:   ~/Library/Logs/thaw/thaw.log
//   - Windows: %APPDATA%\thaw\logs\thaw.log
//   - Linux:   $XDG_STATE_HOME/thaw/thaw.log  (default: ~/.local/state/thaw/thaw.log)
func logFilePath() string {
	var dir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Logs", "thaw")
	case "windows":
		dir = filepath.Join(os.Getenv("APPDATA"), "thaw", "logs")
	default: // linux and other Unix-like systems
		if xdg := os.Getenv("XDG_STATE_HOME"); xdg != "" {
			dir = filepath.Join(xdg, "thaw")
		} else {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".local", "state", "thaw")
		}
	}
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "thaw.log")
}
