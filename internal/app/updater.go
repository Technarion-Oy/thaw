// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"time"

	"thaw/internal/config"
	"thaw/internal/logger"
	"thaw/internal/updater"
	"thaw/internal/version"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// updateCheckInterval throttles the background startup check so repeated app
// launches stay well under GitHub's 60 req/hour unauthenticated limit. The
// on-demand "Check for Updates…" menu action bypasses this and always hits the
// network.
const updateCheckInterval = 24 * time.Hour

// startupUpdateCheckDelay lets the window settle before the background check
// fires, keeping startup responsive.
const startupUpdateCheckDelay = 5 * time.Second

// CheckForUpdate performs an on-demand update check (Help → Check for Updates…).
// It always hits the network so the user gets a fresh answer, persists the
// result to the config cache, and returns it. A network/proxy failure is
// returned as an error for the frontend to surface as "couldn't check".
func (a *App) CheckForUpdate() (updater.CheckResult, error) {
	res, err := updater.Check(a.ctx, version.Version)
	if err != nil {
		logger.L.Warn("on-demand update check failed", "err", err)
		return updater.CheckResult{CurrentVersion: version.Version}, err
	}
	a.persistUpdateCheck(res)
	return res, nil
}

// startUpdateChecker runs the non-blocking background update check a few seconds
// after startup. It is a no-op for "dev" builds (unversioned local/dev binaries
// must never be nagged to update). When a throttled cache entry is still fresh
// it reuses the cached release info; otherwise it hits the network. Either way,
// if a newer version is available it emits "update:available" to the frontend.
// All failures are silent (logged only) — the check must never disrupt the UI.
func (a *App) startUpdateChecker() {
	if version.Version == "dev" {
		logger.L.Debug("update check skipped for dev build")
		return
	}
	go func() {
		select {
		case <-a.ctx.Done():
			return
		case <-time.After(startupUpdateCheckDelay):
		}

		// Reuse a still-fresh cached result to avoid a needless network call.
		if cfg, err := config.Load(); err == nil {
			last := time.Unix(cfg.UpdateCheck.LastCheckUnix, 0)
			if cfg.UpdateCheck.LastCheckUnix > 0 && time.Since(last) < updateCheckInterval {
				res := updater.CheckResult{
					Available:      updater.IsNewer(cfg.UpdateCheck.LatestVersion, version.Version),
					CurrentVersion: version.Version,
					LatestVersion:  cfg.UpdateCheck.LatestVersion,
					ReleaseNotes:   cfg.UpdateCheck.ReleaseNotes,
					ReleasePageURL: cfg.UpdateCheck.ReleasePageURL,
				}
				if res.Available {
					a.emitUpdateAvailable(res)
				}
				return
			}
		}

		res, err := updater.Check(a.ctx, version.Version)
		if err != nil {
			logger.L.Warn("background update check failed", "err", err)
			return
		}
		a.persistUpdateCheck(res)
		if res.Available {
			a.emitUpdateAvailable(res)
		}
	}()
}

// persistUpdateCheck stores the latest check result in the config cache so the
// next background check can be throttled and shown without a network call.
func (a *App) persistUpdateCheck(res updater.CheckResult) {
	if err := config.Update(func(c *config.AppConfig) error {
		c.UpdateCheck = config.UpdateCheckState{
			LastCheckUnix:  time.Now().Unix(),
			LatestVersion:  res.LatestVersion,
			ReleaseNotes:   res.ReleaseNotes,
			ReleasePageURL: res.ReleasePageURL,
		}
		return nil
	}); err != nil {
		logger.L.Warn("failed to persist update check state", "err", err)
	}
}

// emitUpdateAvailable logs and notifies the frontend that a newer version is
// available. The frontend (UpdateNotification) shows a dismissible banner and,
// on request, a modal with the release notes and a "Download update" button.
func (a *App) emitUpdateAvailable(res updater.CheckResult) {
	logger.L.Info("update available", "current", res.CurrentVersion, "latest", res.LatestVersion)
	wailsruntime.EventsEmit(a.ctx, "update:available", res)
}
