// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

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
	Provider     string `json:"provider"`               // "openai" | "google" | "ollama"
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
	Behavior             string                  `json:"behavior"`         // "override" | "extra"
	Credentials          []PipRegistryCredential `json:"credentials"`
	EnableProxy          bool                    `json:"enableProxy"`
	ProxyURL             string                  `json:"proxyURL"`
	ProxyUsername        string                  `json:"proxyUsername"`
	ProxyPassword        string                  `json:"proxyPassword"`
	ProxyBypassHosts     string                  `json:"proxyBypassHosts"` // comma-separated
	TrustedHosts         string                  `json:"trustedHosts"`    // comma-separated
	CustomCACertPath     string                  `json:"customCACertPath"`
}

// SessionConfig holds session pooling and lifecycle parameters.
type SessionConfig struct {
	MaxSessions            int    `json:"maxSessions"`
	MaxOpenConnsPerSession int    `json:"maxOpenConnsPerSession"`
	MaxIdleConnsPerSession int    `json:"maxIdleConnsPerSession"`
	InitMode               string `json:"initMode"`        // "lazy" | "eager"
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
// Current version: 9 (removed AIChat and AIImportSuggest).
const flagsVersion = 9

type FeatureFlags struct {
	Initialized bool `json:"initialized"`
	Version     int  `json:"version"`

	// Data Export & Import
	ResultsetExport bool `json:"resultsetExport"`
	ExportTableData bool `json:"exportTableData"` // Table Data Export
	TableDataImport bool `json:"tableDataImport"`
	DDLExport       bool `json:"ddlExport"`
	PutCommand      bool `json:"putCommand"` // PUT file:// … @stage uploads from the SQL editor
	GetCommand      bool `json:"getCommand"` // GET @stage file:// downloads from the SQL editor
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
	ERDiagramDesigner   bool `json:"erDiagramDesigner"`
	TaskGraphVisualizer bool `json:"taskGraphVisualizer"`
	InsertMapping       bool `json:"insertMapping"`
	CodeSnippets        bool `json:"codeSnippets"`

	// Developer Environments
	SnowparkNotebooks bool `json:"snowparkNotebooks"`
	EmbeddedTerminal  bool `json:"embeddedTerminal"`
	GitIntegration    bool `json:"gitIntegration"`

	// Performance & Diagnostics
	QueryProfile bool `json:"queryProfile"`
	ExplainSQL   bool `json:"explainSql"`

	// SQL Editor
	SqlDiagnostics     bool `json:"sqlDiagnostics"`
	SchemaAutocomplete bool `json:"schemaAutocomplete"`
	DdlHoverTooltips   bool `json:"ddlHoverTooltips"`

	// Data Engineering
	FileFormatBuilder bool `json:"fileFormatBuilder"` // Visual CREATE FILE FORMAT builder & previewer

	// Connection
	SnowflakeCLIProfileManager bool `json:"snowflakeCLIProfileManager"` // Manage Snowflake CLI profiles from the connection dialog

	// Results Grid
	MultiCellCopy bool `json:"multiCellCopy"` // Range selection and multi-cell copy in query results

	// Editor Productivity
	CrossTabSearch bool `json:"crossTabSearch"` // Search and replace across all open tabs
}

// DefaultFeatureFlags returns a FeatureFlags with every feature enabled.
func DefaultFeatureFlags() FeatureFlags {
	return FeatureFlags{
		Initialized:            true,
		Version:                flagsVersion,
		ResultsetExport:        true,
		ExportTableData:        true,
		TableDataImport:        true,
		DDLExport:              true,
		PutCommand:             true,
		GetCommand:             true,
		RemoveCommand:          true,
		UserRoleManagement:     true,
		WarehouseManagement:    true,
		WarehouseCreditUsage:   true,
		QueryActivityHistory:   true,
		IntegrationsManagement: true,
		BackupPoliciesAndSets:  true,
		AIInlineCompletions:    true,
		SchemaMigration:        true,
		DbtScaffolding:         true,
		ERDiagramDesigner:      true,
		TaskGraphVisualizer:    true,
		InsertMapping:          true,
		CodeSnippets:           true,
		SnowparkNotebooks:      true,
		EmbeddedTerminal:       true,
		GitIntegration:         true,
		QueryProfile:           true,
		ExplainSQL:             true,
		SqlDiagnostics:         true,
		SchemaAutocomplete:     true,
		DdlHoverTooltips:       true,
		FileFormatBuilder:      true,
		SnowflakeCLIProfileManager: true,
		MultiCellCopy:              true,
		CrossTabSearch:             true,
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
	f.Version = flagsVersion
	return f
}

// AppConfig is the on-disk configuration for Thaw.
type AppConfig struct {
	Connections            []Connection      `json:"connections"`
	Git                    GitConfig         `json:"git"`
	OAuth                  OAuthConfig       `json:"oauth"`
	AI                     AIConfig          `json:"ai"`
	Snowpark               SnowparkConfig    `json:"snowpark"`
	PipRegistry            PipRegistryConfig `json:"pipRegistry"`
	Editor                 EditorPrefs       `json:"editor"`
	NotebookPrefs          NotebookPrefs     `json:"notebookPrefs"`
	Session                SessionConfig     `json:"session"`
	SnowflakeCLIConfigPath string            `json:"snowflakeCliConfigPath"`
	FeatureFlags           FeatureFlags      `json:"featureFlags"`
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
	path, err := configPath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
