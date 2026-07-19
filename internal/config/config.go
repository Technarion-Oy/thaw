// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"

	"thaw/internal/filesystem"
	"thaw/internal/logger"
)

// fileMu serializes config read-modify-write within this process, so concurrent
// Wails IPC calls (Wails dispatches bound methods concurrently) can't interleave a
// Load→Save round trip and lose an update. It does NOT guard against a *second Thaw
// process* racing on the same file — see Update.
var fileMu sync.Mutex

// Connection is a saved Snowflake connection profile.
type Connection struct {
	Name      string `json:"name"`
	Account   string `json:"account"`
	User      string `json:"user"`
	Role      string `json:"role"`
	Warehouse string `json:"warehouse"`
	Database  string `json:"database"`
	Schema    string `json:"schema"`
}

// GitConfig holds the persisted git / export settings.
// Token is intentionally excluded — it must not be written to disk.
type GitConfig struct {
	ExportDir          string `json:"exportDir"`
	RemoteURL          string `json:"remoteURL"`
	Branch             string `json:"branch"`
	AuthorName         string `json:"authorName"`
	AuthorEmail        string `json:"authorEmail"`
	ExportPathTemplate string `json:"exportPathTemplate"`
	// RecentDirs is the most-recently-opened working directories, newest first,
	// for quick project switching. Capped in the frontend when updated.
	RecentDirs []string `json:"recentDirs,omitempty"`
}

// OAuthConfig holds the OAuth client IDs and secrets for Git providers.
type OAuthConfig struct {
	GithubClientID     string `json:"githubClientId"`
	GithubClientSecret string `json:"githubClientSecret"`
	GitlabClientID     string `json:"gitlabClientId"`
	GitlabClientSecret string `json:"gitlabClientSecret"`
}

// AIConfig holds AI provider settings.
// APIKey is never persisted to config.json — it is scrubbed on save and kept in
// the OS secure store (see internal/secrets and secretsync.go).
type AIConfig struct {
	Provider     string `json:"provider"` // "openai" | "google" | "ollama"
	APIKey       string `json:"apiKey"`
	Model        string `json:"model"`
	Enabled      bool   `json:"enabled"`
	OllamaPort   int    `json:"ollamaPort,omitempty"`   // 0 means default (11434)
	OllamaNumCtx int    `json:"ollamaNumCtx,omitempty"` // 0 means let Ollama decide (usually 4096)
}

// SnowparkConfig holds Snowpark environment settings.
type SnowparkConfig struct {
	Backend    string `json:"backend"`    // "conda" | "venv" | "" (empty = default to conda)
	VenvPath   string `json:"venvPath"`   // custom venv path; empty = use computed default
	PythonPath string `json:"pythonPath"` // explicit python binary for venv creation; empty = auto-detect
}

// PipRegistryCredential holds Basic Auth credentials for a single pip registry URL.
// The password is never persisted to config.json — it is scrubbed on save and
// kept in the OS secure store (see internal/secrets), then hydrated and embedded
// into the registry URL at pip-call time. No .netrc file writes are performed.
type PipRegistryCredential struct {
	Registry string `json:"registry"` // URL the credentials apply to
	Username string `json:"username"`
	Password string `json:"password"` // scrubbed from config.json; stored in the OS secure store, embedded in URL at pip-call time
}

// PipRegistryConfig holds corporate/private pip registry settings.
// When PrimaryURL is non-empty the configured flags are appended automatically
// to every pip install invocation.
type PipRegistryConfig struct {
	PrimaryURL           string                  `json:"primaryURL"`
	AdditionalRegistries []string                `json:"additionalRegistries"`
	Behavior             string                  `json:"behavior"` // "override" | "extra"
	Credentials          []PipRegistryCredential `json:"credentials"`
	EnableProxy          bool                    `json:"enableProxy"`
	ProxyURL             string                  `json:"proxyURL"`
	ProxyUsername        string                  `json:"proxyUsername"`
	ProxyPassword        string                  `json:"proxyPassword"`
	ProxyBypassHosts     string                  `json:"proxyBypassHosts"` // comma-separated
	TrustedHosts         string                  `json:"trustedHosts"`     // comma-separated
	CustomCACertPath     string                  `json:"customCACertPath"`
}

// SessionConfig holds session pooling and lifecycle parameters.
type SessionConfig struct {
	MaxSessions            int    `json:"maxSessions"`
	MaxOpenConnsPerSession int    `json:"maxOpenConnsPerSession"`
	MaxIdleConnsPerSession int    `json:"maxIdleConnsPerSession"`
	InitMode               string `json:"initMode"` // "lazy" | "eager"
	IdleTimeoutMinutes     int    `json:"idleTimeoutMinutes"`
}

// DefaultSessionConfig returns CPU-based defaults for session management.
func DefaultSessionConfig() SessionConfig {
	cpus := runtime.NumCPU()
	return SessionConfig{
		MaxSessions:            min(16, max(4, cpus)),
		MaxOpenConnsPerSession: min(8, max(2, cpus)),
		MaxIdleConnsPerSession: min(4, max(1, cpus/2)),
		InitMode:               "lazy",
		IdleTimeoutMinutes:     0,
	}
}

// EditorPrefs holds SQL formatting preferences for the Monaco editor.
type EditorPrefs struct {
	// KeywordCase controls casing for SQL reserved words (SELECT, FROM, …).
	// Valid values: "UPPER" | "lower" | "Title" | "Preserve"
	KeywordCase string `json:"keywordCase"`
	// IdentifierCase controls casing for unquoted table/column names.
	// Double-quoted identifiers are never modified.
	// Valid values: "Preserve" | "UPPER" | "lower"
	IdentifierCase string `json:"identifierCase"`
	// FunctionCase controls casing for built-in and user-defined function calls.
	// Valid values: "UPPER" | "lower"
	FunctionCase string `json:"functionCase"`
	// IndentStyle is the character used for indentation.
	// Valid values: "spaces" | "tabs"
	IndentStyle string `json:"indentStyle"`
	// IndentSize is the number of spaces per indent level (ignored when IndentStyle is "tabs").
	// Valid values: 2 | 4
	IndentSize int `json:"indentSize"`
	// CommaPosition controls where commas appear in multi-value lists.
	// Valid values: "trailing" | "leading"
	CommaPosition string `json:"commaPosition"`
	// OperatorPosition controls whether AND/OR operators appear before or after the line break.
	// Valid values: "before" | "after"
	OperatorPosition string `json:"operatorPosition"`
}

// DefaultEditorPrefs returns sensible defaults for SQL editing in Snowflake.
func DefaultEditorPrefs() EditorPrefs {
	return EditorPrefs{
		KeywordCase:      "UPPER",
		IdentifierCase:   "Preserve",
		FunctionCase:     "UPPER",
		IndentStyle:      "spaces",
		IndentSize:       2,
		CommaPosition:    "trailing",
		OperatorPosition: "before",
	}
}

// EditorPrefsWithDefaults returns a copy of prefs with any zero-value fields
// filled from DefaultEditorPrefs. Use this instead of manually back-filling
// defaults in multiple call sites.
func EditorPrefsWithDefaults(prefs EditorPrefs) EditorPrefs {
	defaults := DefaultEditorPrefs()
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

// NotebookPrefs holds user preferences for the notebook editor.
type NotebookPrefs struct {
	// SyntaxMode controls how Python diagnostics are produced.
	// "off"    — diagnostics disabled entirely.
	// "static" — ast.parse + pyflakes on the cell text only; no kernel state used.
	// "kernel" — ast.parse + pyflakes with live kernel namespace stubs so that
	//            variables from previously-run cells are not flagged as undefined.
	SyntaxMode string `json:"syntaxMode"`
}

// DefaultNotebookPrefs returns the out-of-the-box notebook preference values.
func DefaultNotebookPrefs() NotebookPrefs {
	return NotebookPrefs{SyntaxMode: "kernel"}
}

// LogPrefs holds user-configurable file-logging preferences. They control the
// verbosity and content of thaw.log, the persistent on-disk log used for
// post-mortem debugging and support diagnostics. See internal/logger.
type LogPrefs struct {
	// LogLevel is the runtime minimum severity written to the log file.
	// Valid values: "debug" | "info" | "warn" | "error". An empty value means
	// "use the build default" (debug for dev builds, info for production).
	LogLevel string `json:"logLevel"`
	// IncludeQuerySQL writes the full SQL text of executed statements to the log
	// file. Off by default: SQL can carry sensitive data (credentials in COPY
	// INTO, PII in WHERE clauses, secrets in CREATE SECRET).
	IncludeQuerySQL bool `json:"includeQuerySQL"`
	// IncludeInternalQueries also logs internal/background queries (object
	// listing, DDL fetching, session setup) when IncludeQuerySQL is on. Without
	// it, only user-initiated queries are written to the log file.
	IncludeInternalQueries bool `json:"includeInternalQueries"`
}

// DefaultLogPrefs returns the out-of-the-box logging preferences: info level,
// no SQL text written to disk.
func DefaultLogPrefs() LogPrefs {
	return LogPrefs{
		LogLevel:               "info",
		IncludeQuerySQL:        false,
		IncludeInternalQueries: false,
	}
}

// ValidLogLevel reports whether name is one of the accepted log levels.
func ValidLogLevel(name string) bool {
	switch name {
	case "debug", "info", "warn", "error":
		return true
	}
	return false
}

// ValidateLogPrefs normalizes a LogPrefs so it is safe to persist, apply, and
// display: a non-empty but unrecognized LogLevel is reset to the default, and
// IncludeInternalQueries is cleared when IncludeQuerySQL is off (it has no
// effect on its own). An empty LogLevel is the legitimate "use the build
// default" sentinel and is left untouched, so this is safe to run on the
// effective-prefs read/apply path — not just the write path.
func ValidateLogPrefs(p LogPrefs) LogPrefs {
	if p.LogLevel != "" && !ValidLogLevel(p.LogLevel) {
		p.LogLevel = DefaultLogPrefs().LogLevel
	}
	if !p.IncludeQuerySQL {
		p.IncludeInternalQueries = false
	}
	return p
}

// LogPrefsLocked marks which LogPrefs fields are controlled by IT-admin policy
// and are therefore not user-editable. true = locked.
type LogPrefsLocked struct {
	LogLevel               bool `json:"logLevel"`
	IncludeQuerySQL        bool `json:"includeQuerySQL"`
	IncludeInternalQueries bool `json:"includeInternalQueries"`
}

// RestoreAdminLockedLogPrefs returns a copy of user with every admin-locked
// field overwritten by the corresponding effective (admin-enforced) value, so
// a client cannot bypass IT policy by submitting values that differ from the
// enforced configuration.
func RestoreAdminLockedLogPrefs(user, effective LogPrefs, locked LogPrefsLocked) LogPrefs {
	if locked.LogLevel {
		user.LogLevel = effective.LogLevel
	}
	if locked.IncludeQuerySQL {
		user.IncludeQuerySQL = effective.IncludeQuerySQL
	}
	if locked.IncludeInternalQueries {
		user.IncludeInternalQueries = effective.IncludeInternalQueries
	}
	return user
}

// FileWatchConfig holds user-tunable file-watcher resource controls. They are
// escape hatches for very large or atypical workspaces (network drives, huge
// dependency trees) where the recursive FS watcher — which auto-refreshes the
// file browser and open editor tabs — is heavier than desired. See
// internal/filesystem/watcher.go and internal/app/filesystem.go.
type FileWatchConfig struct {
	// ExcludeGlobs lists glob patterns matched against individual path components
	// (e.g. "node_modules") and against the whole tree-relative path for
	// multi-segment patterns (e.g. "build/generated"). A change whose path
	// matches any pattern is dropped before it reaches the frontend, so heavy
	// build/dependency directories don't churn the file browser and open tabs.
	// A nil slice means "unconfigured" and resolves to DefaultWatchExcludeGlobs;
	// an explicit empty slice disables exclusion entirely.
	//
	// Note: hidden entries (any path component starting with a dot) are already
	// dropped by the watcher's hidden-directory filter before exclusion runs, so
	// a dot-prefixed pattern like ".venv" or ".git/objects" is redundant — the
	// defaults deliberately list only non-hidden directories.
	ExcludeGlobs []string `json:"excludeGlobs"`
	// MaxWatchedDirs caps the number of distinct directories the watcher will
	// emit change events for. Once the cap is reached, changes in
	// not-yet-seen directories are ignored (already-tracked directories keep
	// working). 0 = unlimited. Bounds the debounce-timer map and the downstream
	// re-list churn on pathological trees.
	MaxWatchedDirs int `json:"maxWatchedDirs"`
	// RaiseFDLimit opts in to raising the process file-descriptor soft limit to
	// the hard limit when a watcher starts (a macOS/kqueue and Linux/inotify
	// mitigation; the recursive backend needs far fewer FDs, but network drives
	// and other tooling can still push against the limit). No-op on Windows.
	RaiseFDLimit bool `json:"raiseFDLimit"`
}

// DefaultWatchExcludeGlobs returns the out-of-the-box watch-exclusion patterns:
// heavy dependency/build directories that rarely need live file-browser refresh.
//
// Only non-hidden directories are listed. Dot-prefixed paths such as ".venv" or
// ".git/objects" are intentionally omitted: the watcher's hidden-directory
// filter already drops every event under them before exclusion runs, so listing
// them here would be dead weight (an unreachable pattern).
func DefaultWatchExcludeGlobs() []string {
	return []string{
		"node_modules",
		"venv",
		"__pycache__",
		"dist",
		"build",
		"target",
		"*.dist-info",
	}
}

// DefaultFileWatchConfig returns the out-of-the-box file-watcher controls:
// sensible exclusion globs, no directory cap, FD-limit raising off.
func DefaultFileWatchConfig() FileWatchConfig {
	return FileWatchConfig{
		ExcludeGlobs:   DefaultWatchExcludeGlobs(),
		MaxWatchedDirs: 0,
		RaiseFDLimit:   false,
	}
}

// FileWatchConfigWithDefaults returns a copy of fw with unconfigured fields
// filled from the defaults. A nil ExcludeGlobs (key absent from config.json) is
// treated as unconfigured and replaced with DefaultWatchExcludeGlobs; a non-nil
// but empty slice is an explicit "exclude nothing" and is left untouched.
func FileWatchConfigWithDefaults(fw FileWatchConfig) FileWatchConfig {
	if fw.ExcludeGlobs == nil {
		fw.ExcludeGlobs = DefaultWatchExcludeGlobs()
	}
	return ValidateFileWatchConfig(fw)
}

// ValidateFileWatchConfig normalizes a FileWatchConfig so it is safe to persist
// and apply: trims blank glob patterns and clamps MaxWatchedDirs to be
// non-negative. It preserves the nil-vs-empty distinction on ExcludeGlobs so the
// "unconfigured" sentinel survives a round trip (use FileWatchConfigWithDefaults
// on the read path to resolve it).
func ValidateFileWatchConfig(fw FileWatchConfig) FileWatchConfig {
	if fw.MaxWatchedDirs < 0 {
		fw.MaxWatchedDirs = 0
	}
	if fw.ExcludeGlobs != nil {
		cleaned := make([]string, 0, len(fw.ExcludeGlobs))
		for _, g := range fw.ExcludeGlobs {
			if g = strings.TrimSpace(g); g != "" {
				cleaned = append(cleaned, g)
			}
		}
		fw.ExcludeGlobs = cleaned
	}
	return fw
}

// CollapseDefaultExcludeGlobs resets ExcludeGlobs to nil — the "unconfigured,
// track evolving defaults" sentinel — when it exactly equals the current default
// list. Use it on the persist path only: because the read path resolves nil to a
// concrete default list before the UI ever sees it, a user who opens the modal
// and saves without changing the globs would otherwise pin today's defaults and
// stop picking up defaults added in future releases. Collapsing an unchanged
// list back to nil keeps such an install on the auto-updating track. A
// deliberate edit (added, removed, or reordered pattern) no longer equals the
// default list and is persisted verbatim. Must run AFTER ValidateFileWatchConfig
// so the comparison sees the same trimmed form as the defaults.
func CollapseDefaultExcludeGlobs(fw FileWatchConfig) FileWatchConfig {
	if slices.Equal(fw.ExcludeGlobs, DefaultWatchExcludeGlobs()) {
		fw.ExcludeGlobs = nil
	}
	return fw
}

// FeatureFlags holds toggles for optional or experimental features.
//
// Adding a new flag:
//  1. Add a bool field here with a json tag.
//  2. Set it to true in DefaultFeatureFlags so it is on by default.
//  3. Bump flagsVersion and add migration logic in MigrateFlags.
//  4. Add a Switch row in FeatureFlagsModal.tsx.
//  5. Read it from featureFlagsStore in whichever component needs gating.
//
// Initialized is a sentinel: when false the config file predates feature flags
// and GetFeatureFlags returns DefaultFeatureFlags instead of the zero struct.
//
// Version tracks the schema revision so new flags introduced after an initial
// save can be filled with their defaults rather than the zero value (false).
// Current version: 18 (removed the PutCommand, GetCommand, RemoveCommand,
// CrossTabSearch, and FileFormatBuilder toggles — those features are now always
// on; the fields were deleted entirely because they were never admin-lockable.
// See issue #567).
const flagsVersion = 18

type FeatureFlags struct {
	Initialized bool `json:"initialized"`
	Version     int  `json:"version"`

	// Data Export & Import
	ResultsetExport bool `json:"resultsetExport"`
	ExportTableData bool `json:"exportTableData"` // Table Data Export
	TableDataImport bool `json:"tableDataImport"`
	DDLExport       bool `json:"ddlExport"`

	// Governance & Administration
	UserRoleManagement     bool `json:"userRoleManagement"`
	WarehouseManagement    bool `json:"warehouseManagement"`
	WarehouseCreditUsage   bool `json:"warehouseCreditUsage"`
	QueryActivityHistory   bool `json:"queryActivityHistory"`
	IntegrationsManagement bool `json:"integrationsManagement"`
	BackupPoliciesAndSets  bool `json:"backupPoliciesAndSets"`

	// AI & Assistance
	AIInlineCompletions bool `json:"aiInlineCompletions"`

	// Advanced Tools & Data Engineering
	SchemaMigration     bool `json:"schemaMigration"`
	DbtScaffolding      bool `json:"dbtScaffolding"`
	DbtProjectBrowser   bool `json:"dbtProjectBrowser"`
	ERDiagramDesigner   bool `json:"erDiagramDesigner"`
	TaskGraphVisualizer bool `json:"taskGraphVisualizer"`
	InsertMapping       bool `json:"insertMapping"`
	InsertRow           bool `json:"insertRow"` // Per-column grid form to INSERT one or more rows into a table
	CodeSnippets        bool `json:"codeSnippets"`

	// Developer Environments
	SnowparkNotebooks bool `json:"snowparkNotebooks"`
	EmbeddedTerminal  bool `json:"embeddedTerminal"`
	GitIntegration    bool `json:"gitIntegration"`

	// Performance & Diagnostics
	QueryProfile bool `json:"queryProfile"`
	ExplainSQL   bool `json:"explainSql"`
	QueryLog     bool `json:"queryLog"` // Session-scoped log of all SQL queries for debugging

	// SQL Editor
	SqlDiagnostics     bool `json:"sqlDiagnostics"`
	SchemaAutocomplete bool `json:"schemaAutocomplete"`
	DdlHoverTooltips   bool `json:"ddlHoverTooltips"`

	// Connection
	SnowflakeCLIProfileManager bool `json:"snowflakeCLIProfileManager"` // Manage Snowflake CLI profiles from the connection dialog

	// Results Grid
	MultiCellCopy   bool `json:"multiCellCopy"`   // Range selection and multi-cell copy in query results
	CellDetailPanel bool `json:"cellDetailPanel"` // Side panel showing the full content of the selected cell
	ColumnReorder   bool `json:"columnReorder"`   // Drag result-grid column headers to reorder them (view-only)

	// File Browser
	FileWatcher bool `json:"fileWatcher"` // Auto-refresh file browser on external changes

	// Schema Management
	ColumnManagement bool `json:"columnManagement"` // Add/alter/drop columns from the sidebar tree

	// Integrations
	MCPServer bool `json:"mcpServer"` // Model Context Protocol server for external AI clients
}

// DefaultFeatureFlags returns a FeatureFlags with every feature enabled.
func DefaultFeatureFlags() FeatureFlags {
	return FeatureFlags{
		Initialized:                true,
		Version:                    flagsVersion,
		ResultsetExport:            true,
		ExportTableData:            true,
		TableDataImport:            true,
		DDLExport:                  true,
		UserRoleManagement:         true,
		WarehouseManagement:        true,
		WarehouseCreditUsage:       true,
		QueryActivityHistory:       true,
		IntegrationsManagement:     true,
		BackupPoliciesAndSets:      true,
		AIInlineCompletions:        true,
		SchemaMigration:            true,
		DbtScaffolding:             true,
		DbtProjectBrowser:          true,
		ERDiagramDesigner:          true,
		TaskGraphVisualizer:        true,
		InsertMapping:              true,
		InsertRow:                  true,
		CodeSnippets:               true,
		SnowparkNotebooks:          true,
		EmbeddedTerminal:           true,
		GitIntegration:             true,
		QueryProfile:               true,
		ExplainSQL:                 true,
		QueryLog:                   false,
		SqlDiagnostics:             true,
		SchemaAutocomplete:         true,
		DdlHoverTooltips:           true,
		SnowflakeCLIProfileManager: true,
		MultiCellCopy:              true,
		CellDetailPanel:            true,
		ColumnReorder:              true,
		FileWatcher:                true,
		ColumnManagement:           true,
		MCPServer:                  false,
	}
}

// MigrateFlags fills in zero-value fields for flags that were added after an
// existing config was saved. When Version < flagsVersion, flags that are false
// (the zero value) but have a default of true are set to true.
// This is safe because false = "not set" for any flag not present in the
// config file at the time it was written; explicitly user-disabled flags will
// be re-enabled only if the user upgrades — they can turn them off again.
func MigrateFlags(f FeatureFlags) FeatureFlags {
	if f.Version >= flagsVersion {
		return f
	}
	// Version 0 → 1: 21 new flags added; all default to true.
	defaults := DefaultFeatureFlags()
	setIfZero := func(field *bool, def bool) {
		if !*field {
			*field = def
		}
	}
	setIfZero(&f.ResultsetExport, defaults.ResultsetExport)
	setIfZero(&f.TableDataImport, defaults.TableDataImport)
	setIfZero(&f.DDLExport, defaults.DDLExport)
	setIfZero(&f.UserRoleManagement, defaults.UserRoleManagement)
	setIfZero(&f.WarehouseManagement, defaults.WarehouseManagement)
	setIfZero(&f.WarehouseCreditUsage, defaults.WarehouseCreditUsage)
	setIfZero(&f.QueryActivityHistory, defaults.QueryActivityHistory)
	setIfZero(&f.IntegrationsManagement, defaults.IntegrationsManagement)
	setIfZero(&f.BackupPoliciesAndSets, defaults.BackupPoliciesAndSets)
	setIfZero(&f.AIInlineCompletions, defaults.AIInlineCompletions)
	setIfZero(&f.SchemaMigration, defaults.SchemaMigration)
	setIfZero(&f.DbtScaffolding, defaults.DbtScaffolding)
	setIfZero(&f.ERDiagramDesigner, defaults.ERDiagramDesigner)
	setIfZero(&f.TaskGraphVisualizer, defaults.TaskGraphVisualizer)
	setIfZero(&f.InsertMapping, defaults.InsertMapping)
	setIfZero(&f.CodeSnippets, defaults.CodeSnippets)
	setIfZero(&f.SnowparkNotebooks, defaults.SnowparkNotebooks)
	setIfZero(&f.EmbeddedTerminal, defaults.EmbeddedTerminal)
	setIfZero(&f.GitIntegration, defaults.GitIntegration)
	setIfZero(&f.QueryProfile, defaults.QueryProfile)
	setIfZero(&f.ExplainSQL, defaults.ExplainSQL)
	// Version 1 → 2: SQL editor flags added; all default to true.
	setIfZero(&f.SqlDiagnostics, defaults.SqlDiagnostics)
	setIfZero(&f.SchemaAutocomplete, defaults.SchemaAutocomplete)
	setIfZero(&f.DdlHoverTooltips, defaults.DdlHoverTooltips)
	// Version 2 → 3: PutCommand/GetCommand added (v3 → 4 added FileFormatBuilder,
	// v4 → 5 added RemoveCommand, v7 → 8 added CrossTabSearch). All five toggles
	// were removed in v18 — those features are now always on — so their
	// backfills are gone. See the v17 → 18 note below.
	// Version 5 → 6: SnowflakeCLIProfileManager added; defaults to true.
	setIfZero(&f.SnowflakeCLIProfileManager, defaults.SnowflakeCLIProfileManager)
	// Version 6 → 7: MultiCellCopy added; defaults to true.
	setIfZero(&f.MultiCellCopy, defaults.MultiCellCopy)
	// Version 9 → 10: FileWatcher added; defaults to true.
	setIfZero(&f.FileWatcher, defaults.FileWatcher)
	// Version 10 → 11: DbtProjectBrowser added; defaults to true.
	setIfZero(&f.DbtProjectBrowser, defaults.DbtProjectBrowser)
	// Version 11 → 12: ColumnManagement added; defaults to true.
	setIfZero(&f.ColumnManagement, defaults.ColumnManagement)
	// Version 12 → 13: MCPServer added; defaults to false (opt-in).
	// setIfZero is a no-op here because the default is false (the zero
	// value), but kept for consistency with the migration pattern.
	setIfZero(&f.MCPServer, defaults.MCPServer)
	// Version 13 → 14: QueryLog added; defaults to false (opt-in).
	setIfZero(&f.QueryLog, defaults.QueryLog)
	// Version 14 → 15: CellDetailPanel added; defaults to true.
	setIfZero(&f.CellDetailPanel, defaults.CellDetailPanel)
	// Version 15 → 16: ColumnReorder added; defaults to true.
	setIfZero(&f.ColumnReorder, defaults.ColumnReorder)
	// Version 16 → 17: InsertRow added; defaults to true.
	setIfZero(&f.InsertRow, defaults.InsertRow)
	// Version 17 → 18: the PutCommand, GetCommand, RemoveCommand, CrossTabSearch,
	// and FileFormatBuilder fields were deleted — those features are now always
	// on and were never admin-lockable. Unknown JSON keys in an older config are
	// simply ignored on unmarshal, so no field-level migration is needed here.
	f.Version = flagsVersion
	return f
}

// ForceAlwaysOnFlags forces every flag whose user-facing toggle was removed in
// issue #567 — but whose field is kept so IT-admin policy can still force it
// off — back to true. It runs on the read path (see loadUserFeatureFlags),
// after MigrateFlags and before admin overrides are merged, so:
//   - a user who had disabled one of these before the toggle was removed gets
//     the feature back (the stored false is overridden), and
//   - an admin who force-disables it via features.json still wins, because
//     LoadAdminConfig applies its overrides on top of the returned value.
//
// These are the "basic functionality" features the modal no longer exposes:
// basic data workflows, results-grid UX, user-initiated read-only diagnostics,
// data-entry helpers, schema management, and the file watcher (now owned by
// Preferences, see #488/#803).
func ForceAlwaysOnFlags(f FeatureFlags) FeatureFlags {
	f.ResultsetExport = true
	f.ExportTableData = true
	f.TableDataImport = true
	f.DDLExport = true
	f.MultiCellCopy = true
	f.CellDetailPanel = true
	f.ColumnReorder = true
	f.QueryProfile = true
	f.ExplainSQL = true
	f.InsertMapping = true
	f.InsertRow = true
	f.ColumnManagement = true
	f.FileWatcher = true
	return f
}

// UpdateCheckState caches the result of the last background update check so the
// startup check can stay well under GitHub's unauthenticated rate limit (60
// req/hour): a fresh network check runs only when LastCheckUnix is older than
// the check interval, otherwise the cached release info is reused to decide
// whether to notify. The full release notes and page URL are cached so the
// notification can be shown from the cache without another network round-trip.
type UpdateCheckState struct {
	LastCheckUnix  int64  `json:"lastCheckUnix"`
	LatestVersion  string `json:"latestVersion"`
	ReleaseNotes   string `json:"releaseNotes"`
	ReleasePageURL string `json:"releasePageURL"`
}

// MCPSessionCredential holds the persisted port (config.json) and auth token
// for an MCP session so that restarting Thaw and re-launching the same session
// label reuses the same URL, keeping external AI client configs valid. The token
// is never persisted to config.json — it is scrubbed on save and kept in the OS
// secure store (see internal/secrets and secretsync.go); only the port persists here.
type MCPSessionCredential struct {
	Port  int    `json:"port"`
	Token string `json:"token"` // scrubbed from config.json; stored in the OS secure store
}

// AppConfig is the on-disk configuration for Thaw.
type AppConfig struct {
	Connections            []Connection                    `json:"connections"`
	Git                    GitConfig                       `json:"git"`
	OAuth                  OAuthConfig                     `json:"oauth"`
	AI                     AIConfig                        `json:"ai"`
	Snowpark               SnowparkConfig                  `json:"snowpark"`
	PipRegistry            PipRegistryConfig               `json:"pipRegistry"`
	Editor                 EditorPrefs                     `json:"editor"`
	NotebookPrefs          NotebookPrefs                   `json:"notebookPrefs"`
	Session                SessionConfig                   `json:"session"`
	SnowflakeCLIConfigPath string                          `json:"snowflakeCliConfigPath"`
	FeatureFlags           FeatureFlags                    `json:"featureFlags"`
	LogPrefs               LogPrefs                        `json:"logPrefs"`
	FileWatch              FileWatchConfig                 `json:"fileWatch"`
	MCPCredentials         map[string]MCPSessionCredential `json:"mcpCredentials,omitempty"`
	UpdateCheck            UpdateCheckState                `json:"updateCheck"`
	// LicenseAccepted records whether the user has accepted the in-app license
	// agreement shown on first launch. Defaults to false, so a fresh install (no
	// config file) and any existing install written before this field was added
	// (the key is simply absent) are both prompted to accept on next launch. No
	// explicit migration is needed: the JSON zero value already means "not yet
	// accepted". Set to true by App.AcceptLicense.
	LicenseAccepted bool `json:"licenseAccepted"`
}

// configPath returns the absolute path to the application configuration file,
// typically $HOME/.config/thaw/config.json on Linux/macOS.
func configPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "thaw", "config.json"), nil
}

// Load reads the config file, returning an empty config if it doesn't exist yet.
//
// Thaw-owned secrets are never returned in the config's secret fields — those
// live in the OS secure store (see internal/secrets and secretsync.go). If an
// older config.json still holds plaintext secrets, they are migrated into the
// store here and the scrubbed file is written back.
func Load() (*AppConfig, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	cfg, err := load()
	if err != nil {
		return nil, err
	}
	if hasPlaintextSecret(cfg) {
		// Legacy plaintext config.json: persist the secrets to the store and
		// scrub them from disk. buildDiskConfig keeps any secret it couldn't
		// store on disk (never lost) so the next load retries — but that also
		// means consumers, which hydrate from the store, see it as absent until
		// migration succeeds. Log both the write failure and any secret that
		// couldn't be stored so this degraded state is diagnosable from thaw.log.
		diskCfg := buildDiskConfig(cfg)
		if err := writeConfigFile(&diskCfg); err != nil {
			logger.L.Warn("config: failed to write scrubbed config during secret migration", "err", err)
		} else if hasPlaintextSecret(&diskCfg) {
			logger.L.Warn("config: some Thaw secrets could not be moved to the OS secure store; " +
				"they remain in config.json (0600) and will be retried on the next load — " +
				"dependent features may be unavailable until then")
		}
	}
	// Never hand back a secret the store doesn't own; the store is authoritative
	// and consumers hydrate from it at their IPC seam.
	blankSecrets(cfg)
	return cfg, nil
}

// Update runs fn against the current on-disk config and writes the result back as
// a single locked read-modify-write, so a concurrent config write in this process
// can't clobber fn's change (or vice versa). Use this instead of a bare
// Load→mutate→Save whenever the new value depends on the old one.
//
// ponytail: process-scoped lock only. A second Thaw process writing concurrently
// can still last-writer-win (a dropped update); the atomic temp+rename in save
// keeps the file from ever being torn/corrupt, which is the dangerous part. Add an
// flock here if cross-process lost updates ever matter.
func Update(fn func(*AppConfig) error) error {
	fileMu.Lock()
	defer fileMu.Unlock()
	cfg, err := load()
	if err != nil {
		return err
	}
	if err := fn(cfg); err != nil {
		return err
	}
	// save persists any secret set on cfg to the store (without overwriting an
	// existing store value) and scrubs the disk copy — see buildDiskConfig.
	return save(cfg)
}

// load reads and parses the config; caller must hold fileMu.
func load() (*AppConfig, error) {
	path, err := configPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		cfg := &AppConfig{}
		cfg.OAuth.GithubClientID = "Ov23liqwbGA6HHQ1za1a"
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}

	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Populate default OAuth client ID if missing
	if cfg.OAuth.GithubClientID == "" {
		cfg.OAuth.GithubClientID = "Ov23liqwbGA6HHQ1za1a"
	}

	return &cfg, nil
}

// Save writes the config to disk, creating directories as needed.
func Save(cfg *AppConfig) error {
	fileMu.Lock()
	defer fileMu.Unlock()
	return save(cfg)
}

// save marshals and atomically writes the config; caller must hold fileMu.
//
// buildDiskConfig persists cfg's secrets to the OS store (never overwriting a
// value already there) and returns a copy with those secrets scrubbed, so
// Thaw-owned secrets never reach config.json. See secretsync.go.
func save(cfg *AppConfig) error {
	diskCfg := buildDiskConfig(cfg)
	return writeConfigFile(&diskCfg)
}

// writeConfigFile marshals and atomically writes an already-scrubbed config;
// caller must hold fileMu. Split from save so Load's migration path can inspect
// the scrubbed result (to detect secrets that couldn't reach the store) without
// running buildDiskConfig twice.
func writeConfigFile(diskCfg *AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(diskCfg, "", "  ")
	if err != nil {
		return err
	}
	// Atomic temp+rename so a second Thaw process never reads a half-written file.
	return filesystem.WriteFileAtomic(path, data, 0o600)
}
