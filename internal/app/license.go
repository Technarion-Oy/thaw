// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/config"
)

// GetLicenseText returns the full license text (embedded LICENSE, GPL-3.0)
// displayed in the first-launch license agreement gate.
func (a *App) GetLicenseText() string {
	return a.licenseText
}

// IsLicenseAccepted reports whether the user has already accepted the in-app
// license agreement. When false the frontend shows the blocking license gate on
// launch. Any config load error is treated as "not accepted" so a failure fails
// safe toward re-prompting rather than silently skipping the gate.
func (a *App) IsLicenseAccepted() bool {
	cfg, err := config.Load()
	if err != nil {
		return false
	}
	return cfg.LicenseAccepted
}

// AcceptLicense records that the user accepted the license agreement and
// persists the choice so subsequent launches skip the gate.
func (a *App) AcceptLicense() error {
	return config.Update(func(cfg *config.AppConfig) error {
		cfg.LicenseAccepted = true
		return nil
	})
}

// DeclineLicense quits the application. Called when the user declines the
// license agreement; the choice is intentionally not persisted, so relaunching
// prompts again.
func (a *App) DeclineLicense() {
	wailsruntime.Quit(a.ctx)
}
