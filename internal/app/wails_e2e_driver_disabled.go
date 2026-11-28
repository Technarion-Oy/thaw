// SPDX-License-Identifier: GPL-3.0-or-later

//go:build !mcp || production

package app

// wailsE2EState is zero-sized outside non-production Wails E2E test builds so
// release and normal builds carry no extra state or runtime overhead.
type wailsE2EState struct{}

// recordWailsE2EMenuAction is a no-op outside non-production Wails E2E builds.
func (a *App) recordWailsE2EMenuAction(string, func()) {}

// registerWailsE2EMenuDriver is a no-op outside non-production Wails E2E builds.
func (a *App) registerWailsE2EMenuDriver(func(label string) (found, enabled bool)) {}
