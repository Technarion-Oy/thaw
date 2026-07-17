// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"thaw/internal/filesystem"
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
// APIKey is stored in ~/.config/thaw/config.json (mode 0600).
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
// The password is stored in config.json (mode 0600) and embedded into the URL
// at pip-call time — no keychain or .netrc file writes are performed.
type PipRegistryCredential struct {
	Registry string `json:"registry"` // URL the credentials apply to
	Username string `json:"username"`
	Password string `json:"password"` // stored in config.json (0600); embedded in URL at pip-call time
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
// Current version: 17 (added InsertRow).
const flagsVersion = 17

type FeatureFlags struct {
	Initialized bool `json:"initialized"`
	Version     int  `json:"version"`

	// Data Export & Import
	ResultsetExport bool `json:"resultsetExport"`
	ExportTableData bool `json:"exportTableData"` // Table Data Export
	TableDataImport bool `json:"tableDataImport"`
	DDLExport       bool `json:"ddlExport"`
	PutCommand      bool `json:"putCommand"`    // PUT file:// … @stage uploads from the SQL editor
	GetCommand      bool `json:"getCommand"`    // GET @stage file:// downloads from the SQL editor
	RemoveCommand   bool `json:"removeCommand"` // REMOVE @stage/file deletes

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

	// Data Engineering
	FileFormatBuilder bool `json:"fileFormatBuilder"` // Visual CREATE FILE FORMAT builder & previewer

	// Connection
	SnowflakeCLIProfileManager bool `json:"snowflakeCLIProfileManager"` // Manage Snowflake CLI profiles from the connection dialog

	// Results Grid
	MultiCellCopy   bool `json:"multiCellCopy"`   // Range selection and multi-cell copy in query results
	CellDetailPanel bool `json:"cellDetailPanel"` // Side panel showing the full content of the selected cell
	ColumnReorder   bool `json:"columnReorder"`   // Drag result-grid column headers to reorder them (view-only)

	// Editor Productivity
	CrossTabSearch bool `json:"crossTabSearch"` // Search and replace across all open tabs

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
		PutCommand:                 true,
		GetCommand:                 true,
		RemoveCommand:              true,
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
		FileFormatBuilder:          true,
		SnowflakeCLIProfileManager: true,
		MultiCellCopy:              true,
		CellDetailPanel:            true,
		ColumnReorder:              true,
		CrossTabSearch:             true,
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
	// Version 2 → 3: file-transfer flags added; both default to true.
	setIfZero(&f.PutCommand, defaults.PutCommand)
	setIfZero(&f.GetCommand, defaults.GetCommand)
	// Version 3 → 4: FileFormatBuilder added; defaults to true.
	setIfZero(&f.FileFormatBuilder, defaults.FileFormatBuilder)
	// Version 4 → 5: RemoveCommand added; defaults to true.
	setIfZero(&f.RemoveCommand, defaults.RemoveCommand)
	// Version 5 → 6: SnowflakeCLIProfileManager added; defaults to true.
	setIfZero(&f.SnowflakeCLIProfileManager, defaults.SnowflakeCLIProfileManager)
	// Version 6 → 7: MultiCellCopy added; defaults to true.
	setIfZero(&f.MultiCellCopy, defaults.MultiCellCopy)
	// Version 7 → 8: CrossTabSearch added; defaults to true.
	setIfZero(&f.CrossTabSearch, defaults.CrossTabSearch)
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
	f.Version = flagsVersion
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

// MCPSessionCredential holds the persisted port and auth token for an MCP
// session so that restarting Thaw and re-launching the same session label
// reuses the same URL, keeping external AI client configs valid.
type MCPSessionCredential struct {
	Port  int    `json:"port"`
	Token string `json:"token"`
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
	MCPCredentials         map[string]MCPSessionCredential `json:"mcpCredentials,omitempty"`
	UpdateCheck            UpdateCheckState                `json:"updateCheck"`
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
func Load() (*AppConfig, error) {
	fileMu.Lock()
	defer fileMu.Unlock()
	return load()
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
func save(cfg *AppConfig) error {
	path, err := configPath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Atomic temp+rename so a second Thaw process never reads a half-written file.
	return filesystem.WriteFileAtomic(path, data, 0o600)
}
