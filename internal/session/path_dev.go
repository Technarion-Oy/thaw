// SPDX-License-Identifier: GPL-3.0-or-later

//go:build dev

package session

// StatePath returns the session state file path for local development builds.
// Stored in the project directory for easy inspection during development.
func StatePath() string {
	return "./thaw-session.json"
}
