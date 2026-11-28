// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !mcp || production

package app

// recordWailsE2EMenuAction is a no-op outside non-production Wails E2E builds.
func (a *App) recordWailsE2EMenuAction(string, func()) {}

// registerWailsE2EMenuDriver is a no-op outside non-production Wails E2E builds.
func (a *App) registerWailsE2EMenuDriver(func(label string) (found, enabled bool)) {}
