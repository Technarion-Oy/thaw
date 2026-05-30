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
	"fmt"
	"os/exec"
	"strings"
	"thaw/internal/config"
)

// GetAIConfig returns the persisted AI provider settings.
func (a *App) GetAIConfig() config.AIConfig {
	cfg, err := config.Load()
	if err != nil {
		return config.AIConfig{}
	}
	return cfg.AI
}

// SaveAIConfig persists AI provider settings to disk.
func (a *App) SaveAIConfig(aiCfg config.AIConfig) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.AI = aiCfg
	return config.Save(cfg)
}

// GetSystemRAMGB returns the total physical RAM in gigabytes (rounded down).
// Returns 0 if the value cannot be determined (e.g. unsupported platform).
// Used by the frontend to suggest a sensible Ollama context-window size.
func (a *App) GetSystemRAMGB() int {
	// macOS / Linux: sysctl -n hw.memsize (macOS) or hw.physmem (some BSDs)
	for _, key := range []string{"hw.memsize", "hw.physmem"} {
		out, err := exec.Command("sysctl", "-n", key).Output()
		if err != nil {
			continue
		}
		var bytes uint64
		if _, err := fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &bytes); err != nil {
			continue
		}
		if bytes > 0 {
			return int(bytes / (1024 * 1024 * 1024))
		}
	}
	return 0
}

// GetEditorPrefs returns the persisted SQL editor formatting preferences.
// Returns sensible defaults when the config does not exist yet.
func (a *App) GetEditorPrefs() config.EditorPrefs {
	cfg, err := config.Load()
	if err != nil {
		return config.DefaultEditorPrefs()
	}
	prefs := cfg.Editor
	// Back-fill any zero fields with defaults so callers always get a fully populated struct.
	defaults := config.DefaultEditorPrefs()
	if prefs.KeywordCase == "" {
		prefs.KeywordCase = defaults.KeywordCase
	}
	if prefs.IdentifierCase == "" {
		prefs.IdentifierCase = defaults.IdentifierCase
	}
	if prefs.FunctionCase == "" {
		prefs.FunctionCase = defaults.FunctionCase
	}
	if prefs.IndentStyle == "" {
		prefs.IndentStyle = defaults.IndentStyle
	}
	if prefs.IndentSize == 0 {
		prefs.IndentSize = defaults.IndentSize
	}
	if prefs.CommaPosition == "" {
		prefs.CommaPosition = defaults.CommaPosition
	}
	if prefs.OperatorPosition == "" {
		prefs.OperatorPosition = defaults.OperatorPosition
	}
	return prefs
}

// SaveEditorPrefs persists SQL editor formatting preferences to disk.
func (a *App) SaveEditorPrefs(prefs config.EditorPrefs) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Editor = prefs
	return config.Save(cfg)
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	flags.Initialized = true

	// Preserve admin-locked values: restore the admin-controlled fields from
	// the effective flags so a user cannot bypass policy via the API.
	_, locked := config.LoadAdminConfig(flags)
	effective, _ := config.LoadAdminConfig(flags)
	if locked.ResultsetExport {
		flags.ResultsetExport = effective.ResultsetExport
	}
	if locked.ExportTableData {
		flags.ExportTableData = effective.ExportTableData
	}
	if locked.TableDataImport {
		flags.TableDataImport = effective.TableDataImport
	}
	if locked.DDLExport {
		flags.DDLExport = effective.DDLExport
	}
	if locked.UserRoleManagement {
		flags.UserRoleManagement = effective.UserRoleManagement
	}
	if locked.WarehouseManagement {
		flags.WarehouseManagement = effective.WarehouseManagement
	}
	if locked.WarehouseCreditUsage {
		flags.WarehouseCreditUsage = effective.WarehouseCreditUsage
	}
	if locked.QueryActivityHistory {
		flags.QueryActivityHistory = effective.QueryActivityHistory
	}
	if locked.IntegrationsManagement {
		flags.IntegrationsManagement = effective.IntegrationsManagement
	}
	if locked.BackupPoliciesAndSets {
		flags.BackupPoliciesAndSets = effective.BackupPoliciesAndSets
	}
	if locked.AIInlineCompletions {
		flags.AIInlineCompletions = effective.AIInlineCompletions
	}
	if locked.SchemaMigration {
		flags.SchemaMigration = effective.SchemaMigration
	}
	if locked.DbtScaffolding {
		flags.DbtScaffolding = effective.DbtScaffolding
	}
	if locked.ERDiagramDesigner {
		flags.ERDiagramDesigner = effective.ERDiagramDesigner
	}
	if locked.TaskGraphVisualizer {
		flags.TaskGraphVisualizer = effective.TaskGraphVisualizer
	}
	if locked.InsertMapping {
		flags.InsertMapping = effective.InsertMapping
	}
	if locked.CodeSnippets {
		flags.CodeSnippets = effective.CodeSnippets
	}
	if locked.SnowparkNotebooks {
		flags.SnowparkNotebooks = effective.SnowparkNotebooks
	}
	if locked.EmbeddedTerminal {
		flags.EmbeddedTerminal = effective.EmbeddedTerminal
	}
	if locked.GitIntegration {
		flags.GitIntegration = effective.GitIntegration
	}
	if locked.QueryProfile {
		flags.QueryProfile = effective.QueryProfile
	}
	if locked.ExplainSQL {
		flags.ExplainSQL = effective.ExplainSQL
	}

	cfg.FeatureFlags = flags
	if err := config.Save(cfg); err != nil {
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
	if a.client == nil {
		return
	}
	flags := a.GetFeatureFlags()
	excl := make(map[string]bool)
	if !flags.DbtProjectBrowser {
		excl["DBT PROJECT"] = true
	}
	a.client.SetExcludedExtendedKinds(excl)
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
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.NotebookPrefs = prefs
	return config.Save(cfg)
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
	if sc.MaxSessions < 1 {
		sc.MaxSessions = 1
	} else if sc.MaxSessions > 32 {
		sc.MaxSessions = 32
	}
	if sc.MaxOpenConnsPerSession < 1 {
		sc.MaxOpenConnsPerSession = 1
	} else if sc.MaxOpenConnsPerSession > 16 {
		sc.MaxOpenConnsPerSession = 16
	}
	if sc.MaxIdleConnsPerSession < 1 {
		sc.MaxIdleConnsPerSession = 1
	} else if sc.MaxIdleConnsPerSession > 16 {
		sc.MaxIdleConnsPerSession = 16
	}
	if sc.MaxIdleConnsPerSession > sc.MaxOpenConnsPerSession {
		sc.MaxIdleConnsPerSession = sc.MaxOpenConnsPerSession
	}
	if sc.InitMode != "lazy" && sc.InitMode != "eager" {
		sc.InitMode = "lazy"
	}
	if sc.IdleTimeoutMinutes < 0 {
		sc.IdleTimeoutMinutes = 0
	} else if sc.IdleTimeoutMinutes > 480 {
		sc.IdleTimeoutMinutes = 480
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.Session = sc
	if err := config.Save(cfg); err != nil {
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
