// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"

	"thaw/internal/config"
)

// The license gate is a UI convention, not a backend-enforced authorization
// boundary: other IPC methods do not check LicenseAccepted, so a user with
// devtools access could invoke bound methods on window.go.app.App directly while
// the modal is up. This is a deliberate call for a GPL acknowledgement gate —
// declining still quits, and refusing to persist acceptance means the gate
// reappears on the next normal launch. Adding a LicenseAccepted guard to ~150
// methods is out of proportion to what the gate is for.

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
