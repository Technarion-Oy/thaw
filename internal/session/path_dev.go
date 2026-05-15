// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

//go:build dev

package session

// StatePath returns the session state file path for local development builds.
// Stored in the project directory for easy inspection during development.
func StatePath() string {
	return "./thaw-session.json"
}
