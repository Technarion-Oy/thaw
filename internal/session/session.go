// SPDX-License-Identifier: GPL-3.0-or-later

package session

import (
	"encoding/json"
	"os"
)

// WindowState records the window geometry at shutdown so it can be restored on
// the next launch.
type WindowState struct {
	X         int  `json:"x"`
	Y         int  `json:"y"`
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximised"` //nolint:misspell // JSON tag kept for backward-compat with existing session files
}

// LoadWindowState reads the persisted window state from disk.
// Returns (state, true) on success, or (zero, false) if the file is missing,
// unreadable, or contains invalid JSON.
func LoadWindowState() (WindowState, bool) {
	data, err := os.ReadFile(StatePath())
	if err != nil {
		return WindowState{}, false
	}
	var s WindowState
	if err := json.Unmarshal(data, &s); err != nil {
		return WindowState{}, false
	}
	// Sanity-check: reject implausible dimensions (e.g. zeroed-out file).
	if s.Width < 100 || s.Height < 100 {
		return WindowState{}, false
	}
	return s, true
}

// SaveWindowState marshals the given WindowState to JSON and writes it to the
// session state file, creating the file if it does not exist.
func SaveWindowState(s WindowState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(StatePath(), data, 0o644)
}
