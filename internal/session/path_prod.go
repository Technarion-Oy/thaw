// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

//go:build !dev

package session

import (
	"os"
	"path/filepath"
	"runtime"
)

// StatePath returns the OS-specific session state file path for production builds.
//
//   - macOS:   ~/Library/Application Support/thaw/session.json
//   - Windows: %LOCALAPPDATA%\thaw\session.json
//   - Linux:   $XDG_DATA_HOME/thaw/session.json  (default: ~/.local/share/thaw/session.json)
func StatePath() string {
	var dir string
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		dir = filepath.Join(home, "Library", "Application Support", "thaw")
	case "windows":
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "thaw")
	default: // linux and other Unix-like systems
		if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
			dir = filepath.Join(xdg, "thaw")
		} else {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".local", "share", "thaw")
		}
	}
	_ = os.MkdirAll(dir, 0o755)
	return filepath.Join(dir, "session.json")
}
