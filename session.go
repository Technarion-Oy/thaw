// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package main

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

// loadWindowState reads the persisted window state from disk.
// Returns (state, true) on success, or (zero, false) if the file is missing,
// unreadable, or contains invalid JSON.
func loadWindowState() (WindowState, bool) {
	data, err := os.ReadFile(sessionStatePath())
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

// saveWindowState marshals the given WindowState to JSON and writes it to the
// session state file, creating the file if it does not exist.
func saveWindowState(s WindowState) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(sessionStatePath(), data, 0o644)
}
