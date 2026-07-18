// SPDX-License-Identifier: GPL-3.0-or-later

package app

import (
	"thaw/internal/config"
	"thaw/internal/logger"
	"thaw/internal/secrets"
	"thaw/internal/sysinfo"
)

// GetAIConfig returns the persisted AI provider settings. The API key is read
// from the OS secure store (not config.json) and populated onto the returned
// struct so the Settings UI can show it.
func (a *App) GetAIConfig() config.AIConfig {
	cfg, err := config.Load()
	if err != nil {
		return config.AIConfig{}
	}
	if key, err := secrets.Get(secrets.KeyAIAPIKey); err == nil {
		cfg.AI.APIKey = key
	}
	return cfg.AI
}

// SaveAIConfig persists AI provider settings. The API key is written to the OS
// secure store; the config.json copy is scrubbed on save. The key is NOT blanked
// up front — passing it through to config.Update lets buildDiskConfig scrub it
// only once it is confirmed safely stored, so a failed store write (locked
// keychain, dbus unavailable, …) keeps the key on disk rather than losing it.
func (a *App) SaveAIConfig(aiCfg config.AIConfig) error {
	if err := storeOrDelete(secrets.KeyAIAPIKey, aiCfg.APIKey); err != nil {
		logger.L.Warn("secrets: failed to store AI API key", "err", err)
	}
	return config.Update(func(cfg *config.AppConfig) error {
		cfg.AI = aiCfg
		return nil
	})
}

// GetSystemRAMGB returns the total physical RAM in gigabytes (rounded down).
// Returns 0 if the value cannot be determined (e.g. unsupported platform).
// Used by the frontend to suggest a sensible Ollama context-window size.
func (a *App) GetSystemRAMGB() int {
	return sysinfo.MemoryGB()
}

// GetEditorPrefs returns the persisted SQL editor formatting preferences.
// Returns sensible defaults when the config does not exist yet.
func (a *App) GetEditorPrefs() config.EditorPrefs {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultEditorPrefs()
	}
	return config.EditorPrefsWithDefaults(cfg.Editor)
}

// SaveEditorPrefs persists SQL editor formatting preferences to disk.
func (a *App) SaveEditorPrefs(prefs config.EditorPrefs) error {
	return config.Update(func(cfg *config.AppConfig) error {
		cfg.Editor = prefs
		return nil
	})
}

// loadUserFeatureFlags returns the raw user-persisted flags (or defaults when
// the config predates feature flags). Runs MigrateFlags so that new flags
// added after an existing config was written are filled with their defaults.
func loadUserFeatureFlags() config.FeatureFlags {
	cfg, err := config.Load()
	if err != nil || !cfg.FeatureFlags.Initialized {
		return config.DefaultFeatureFlags()
	}
	return config.MigrateFlags(cfg.FeatureFlags)
}

// GetFeatureFlags returns the effective feature flag settings with IT admin
// overrides applied on top of the user's saved preferences.
func (a *App) GetFeatureFlags() config.FeatureFlags {
	user := loadUserFeatureFlags()
	effective, _ := config.LoadAdminConfig(user)
	return effective
}

// GetAdminLockedFlags returns a FeatureFlags mask where true means the field
// is controlled by an IT admin configuration and cannot be changed by the user.
func (a *App) GetAdminLockedFlags() config.FeatureFlags {
	user := loadUserFeatureFlags()
	_, locked := config.LoadAdminConfig(user)
	return locked
}

// SaveFeatureFlags persists feature flag settings to disk.
// Admin-locked fields in flags are silently ignored — the admin value is
// preserved so a rogue client cannot bypass IT policy.
// Initialized is always set to true so subsequent loads use the saved values.
func (a *App) SaveFeatureFlags(flags config.FeatureFlags) error {
	flags.Initialized = true

	// Preserve admin-locked values: restore the admin-controlled fields from
	// the effective flags so a user cannot bypass policy via the API.
	effective, locked := config.LoadAdminConfig(flags)
	flags = config.RestoreAdminLockedFields(flags, effective, locked)

	if err := config.Update(func(cfg *config.AppConfig) error {
		cfg.FeatureFlags = flags
		return nil
	}); err != nil {
		return err
	}
	a.applyFeatureFlagExclusions()
	return nil
}

// applyFeatureFlagExclusions updates the Snowflake client's excluded extended
// object kinds based on the current feature flags. Called after connecting and
// after saving feature flags so disabled features don't incur unnecessary
// SHOW queries during schema expansion.
func (a *App) applyFeatureFlagExclusions() {
	client := a.currentClient()
	if client == nil {
		return
	}
	flags := a.GetFeatureFlags()
	excl := make(map[string]bool)
	if !flags.DbtProjectBrowser {
		excl["DBT PROJECT"] = true
	}
	client.SetExcludedExtendedKinds(excl)
}

// GetNotebookPrefs returns the persisted notebook editor preferences.
// Returns sensible defaults when the config does not exist yet.
func (a *App) GetNotebookPrefs() config.NotebookPrefs {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultNotebookPrefs()
	}
	prefs := cfg.NotebookPrefs
	if prefs.SyntaxMode == "" {
		prefs.SyntaxMode = config.DefaultNotebookPrefs().SyntaxMode
	}
	return prefs
}

// SaveNotebookPrefs persists notebook editor preferences to disk.
func (a *App) SaveNotebookPrefs(prefs config.NotebookPrefs) error {
	return config.Update(func(cfg *config.AppConfig) error {
		cfg.NotebookPrefs = prefs
		return nil
	})
}

// GetSessionConfig returns the persisted session management configuration.
// Zero-value fields are backfilled with CPU-based defaults.
func (a *App) GetSessionConfig() config.SessionConfig {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultSessionConfig()
	}
	sc := cfg.Session
	defaults := config.DefaultSessionConfig()
	if sc.MaxSessions == 0 {
		sc.MaxSessions = defaults.MaxSessions
	}
	if sc.MaxOpenConnsPerSession == 0 {
		sc.MaxOpenConnsPerSession = defaults.MaxOpenConnsPerSession
	}
	if sc.MaxIdleConnsPerSession == 0 {
		sc.MaxIdleConnsPerSession = defaults.MaxIdleConnsPerSession
	}
	if sc.InitMode == "" {
		sc.InitMode = defaults.InitMode
	}
	return sc
}

// SaveSessionConfig persists session management settings and applies them at runtime.
func (a *App) SaveSessionConfig(sc config.SessionConfig) error {
	// Validate and clamp values to valid ranges.
	sc = config.ValidateSessionConfig(sc)

	if err := config.Update(func(cfg *config.AppConfig) error {
		cfg.Session = sc
		return nil
	}); err != nil {
		return err
	}
	a.applySessionConfig(sc)
	return nil
}

// GetDefaultSessionConfig returns the CPU-based default values (for the Reset button).
func (a *App) GetDefaultSessionConfig() config.SessionConfig {
	return config.DefaultSessionConfig()
}

// GetSessionInitMode returns the current session initialization mode ("lazy" or "eager").
func (a *App) GetSessionInitMode() string {
	a.sessionConfigMu.RLock()
	mode := a.sessionInitMode
	a.sessionConfigMu.RUnlock()
	if mode == "" {
		return "lazy"
	}
	return mode
}
