// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties without an
// license agreement with Technarion Oy.

//go:build !darwin && !windows

package config

// applyPlatformOverrides is a no-op on Linux and other platforms.
// Admin configuration is read solely from /etc/thaw/features.json.
func applyPlatformOverrides(base adminConfigJSON) adminConfigJSON {
	return base
}
