// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package app

import (
	"os/exec"
	"runtime"
	"time"

	"thaw/internal/config"
	"thaw/internal/logger"
	"thaw/internal/querylog"
)

// GetLogPrefs returns the effective file-logging preferences (persisted user
// values with IT-admin policy applied), backfilled with defaults for display.
func (a *App) GetLogPrefs() config.LogPrefs {
	return config.LogPrefsWithDefaults(a.loadEffectiveLogPrefs())
}

// GetLogPrefsLocked returns a mask where true means the field is controlled by
// an IT-admin logging policy and cannot be changed by the user.
func (a *App) GetLogPrefsLocked() config.LogPrefsLocked {
	user := config.DefaultLogPrefs()
	if cfg, err := config.Load(); err == nil {
		user = cfg.LogPrefs
	}
	_, locked := config.LoadAdminLogPrefs(user)
	return locked
}

// UpdateLogPrefs validates and persists file-logging preferences, then applies
// them at runtime (log level + SQL-logging switches). Admin-locked fields in
// prefs are ignored: the enforced admin value is preserved so a client cannot
// bypass IT policy via the API.
func (a *App) UpdateLogPrefs(prefs config.LogPrefs) error {
	prefs = config.ValidateLogPrefs(prefs)

	// Preserve admin-locked values before persisting.
	effective, locked := config.LoadAdminLogPrefs(prefs)
	prefs = config.RestoreAdminLockedLogPrefs(prefs, effective, locked)

	if err := config.Update(func(cfg *config.AppConfig) error {
		cfg.LogPrefs = prefs
		return nil
	}); err != nil {
		return err
	}

	// Apply the effective prefs directly rather than re-reading from disk:
	// prefs already has admin-locked fields restored, so LoadAdminLogPrefs is
	// idempotent here and effective == prefs. Normalize so runtime state
	// upholds the same invariant GetLogPrefs presents.
	a.applyLogPrefs(config.ValidateLogPrefs(effective))
	return nil
}

// RevealLogFile opens the OS file manager at the log file's location so the
// user can inspect or share thaw.log for support diagnostics.
func (a *App) RevealLogFile() error {
	path := logger.Path
	switch runtime.GOOS {
	case "darwin":
		// -R reveals (selects) the file in Finder.
		return exec.Command("open", "-R", path).Start()
	case "windows":
		return exec.Command("explorer", "/select,", path).Start()
	default: // linux and others: open the containing directory.
		return exec.Command("xdg-open", logger.Dir).Start()
	}
}

// loadEffectiveLogPrefs reads the persisted LogPrefs and applies IT-admin
// logging policy on top. Returns admin-only defaults when config can't be read.
// The result is normalized so the "IncludeInternalQueries implies
// IncludeQuerySQL" invariant holds even after admin overrides — e.g. a policy
// that force-disables IncludeQuerySQL while leaving IncludeInternalQueries
// unlocked must not surface a checked-but-inert internal-queries switch.
func (a *App) loadEffectiveLogPrefs() config.LogPrefs {
	user := config.LogPrefs{}
	if cfg, err := config.Load(); err == nil {
		user = cfg.LogPrefs
	}
	effective, _ := config.LoadAdminLogPrefs(user)
	return config.ValidateLogPrefs(effective)
}

// applyLogPrefs applies the given preferences at runtime: it sets the logger's
// minimum level (a no-op for an empty LogLevel, keeping the build default) and
// caches the SQL-logging switches for the OnQuery hook to consult.
func (a *App) applyLogPrefs(prefs config.LogPrefs) {
	logger.SetLevel(prefs.LogLevel)
	a.logPrefsMu.Lock()
	a.logPrefs = prefs
	a.logPrefsMu.Unlock()
}

// currentLogPrefs returns a snapshot of the cached effective logging prefs.
func (a *App) currentLogPrefs() config.LogPrefs {
	a.logPrefsMu.RLock()
	defer a.logPrefsMu.RUnlock()
	return a.logPrefs
}

// maybeFileLogQuery writes an executed statement's SQL text to the application
// log file when LogPrefs.IncludeQuerySQL is enabled. Internal/background
// queries are only written when IncludeInternalQueries is also on. This is the
// file-based consumer of the same OnQuery choke point that feeds the in-memory
// query log, so there is a single source of truth for "what Thaw executed".
func (a *App) maybeFileLogQuery(src querylog.Source, sql, qid string, err error, dur time.Duration) {
	prefs := a.currentLogPrefs()
	if !prefs.IncludeQuerySQL {
		return
	}
	if src == querylog.SourceInternal && !prefs.IncludeInternalQueries {
		return
	}
	attrs := []any{
		"source", string(src),
		"duration_ms", dur.Milliseconds(),
	}
	if qid != "" {
		attrs = append(attrs, "query_id", qid)
	}
	attrs = append(attrs, "sql", sql)
	if err != nil {
		attrs = append(attrs, "err", err.Error())
		logger.L.Error("query executed", attrs...)
		return
	}
	logger.L.Info("query executed", attrs...)
}
