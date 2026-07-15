// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !darwin && !windows

package config

// applyPlatformOverrides is a no-op on Linux and other platforms.
// Admin configuration is read solely from /etc/thaw/features.json.
func applyPlatformOverrides(base adminConfigJSON) adminConfigJSON {
	return base
}
