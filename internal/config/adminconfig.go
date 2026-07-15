// SPDX-License-Identifier: GPL-3.0-or-later

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// ─── Admin config JSON schema ──────────────────────────────────────────────────
//
// IT admins deploy a features.json to a platform-specific system directory.
// Any key set to false disables the feature for all users on the machine and
// prevents the user from re-enabling it via the UI.
//
// macOS:   /Library/Application Support/Thaw/features.json
// Windows: %PROGRAMDATA%\Thaw\features.json
// Linux:   /etc/thaw/features.json
//
// Example:
//
//	{
//	  "dataExportImport":         { "ddlExport": false },
//	  "governanceAdministration": { "userRoleManagement": false },
//	  "ai":                       { "aiInlineCompletions": false },
//	  "advancedTools":            { "schemaMigration": false },
//	  "developerEnvironments":    { "snowparkNotebooks": false },
//	  "performanceDiagnostics":   { "explainSql": false },
//	  "connection":               { "snowflakeCLIProfileManager": false },
//	  "fileBrowser":              { "fileWatcher": false },
//	  "schemaManagement":         { "columnManagement": false },
//	  "integrations":             { "mcpServer": false },
//	  "logging":                  { "logLevel": "info", "includeQuerySQL": false, "includeInternalQueries": false }
//	}
//
// The "logging" category is special: unlike the boolean feature categories it
// enforces LogPrefs values (a string log level plus two switches). A present
// key both sets the value and locks the corresponding field in the UI. This is
// how IT can force-disable SQL logging (privacy) or force-enable it (audit).
//
// Audit use case: "includeInternalQueries" depends on "includeQuerySQL".
// Forcing "includeInternalQueries": true automatically implies (and locks)
// "includeQuerySQL": true, so the audit policy takes effect with a single key.
// An explicit "includeQuerySQL": false alongside it is honored as-is (a
// contradictory config the admin opted into) — internal logging then normalizes
// off, since it has no effect without SQL logging.

// ptrbool is a small helper so JSON null / absent ≠ false.
type ptrBool = *bool

// adminDataExportImport is the "dataExportImport" category in features.json.
type adminDataExportImport struct {
	ResultsetExport ptrBool `json:"resultsetExport,omitempty"`
	TableDataExport ptrBool `json:"tableDataExport,omitempty"` // corresponds to ExportTableData
	TableDataImport ptrBool `json:"tableDataImport,omitempty"`
	DDLExport       ptrBool `json:"ddlExport,omitempty"`
}

// adminGovernance is the "governanceAdministration" category.
type adminGovernance struct {
	UserRoleManagement     ptrBool `json:"userRoleManagement,omitempty"`
	WarehouseManagement    ptrBool `json:"warehouseManagement,omitempty"`
	WarehouseCreditUsage   ptrBool `json:"warehouseCreditUsage,omitempty"`
	QueryActivityHistory   ptrBool `json:"queryActivityHistory,omitempty"`
	IntegrationsManagement ptrBool `json:"integrationsManagement,omitempty"`
	BackupPoliciesAndSets  ptrBool `json:"backupPoliciesAndSets,omitempty"`
}

// adminAI is the "ai" category.
type adminAI struct {
	AIInlineCompletions ptrBool `json:"aiInlineCompletions,omitempty"`
}

// adminAdvancedTools is the "advancedTools" category.
type adminAdvancedTools struct {
	SchemaMigration     ptrBool `json:"schemaMigration,omitempty"`
	DbtScaffolding      ptrBool `json:"dbtScaffolding,omitempty"`
	DbtProjectBrowser   ptrBool `json:"dbtProjectBrowser,omitempty"`
	ERDiagramDesigner   ptrBool `json:"erDiagramDesigner,omitempty"`
	TaskGraphVisualizer ptrBool `json:"taskGraphVisualizer,omitempty"`
	InsertMapping       ptrBool `json:"insertMapping,omitempty"`
	InsertRow           ptrBool `json:"insertRow,omitempty"`
	CodeSnippets        ptrBool `json:"codeSnippets,omitempty"`
}

// adminDevEnv is the "developerEnvironments" category.
type adminDevEnv struct {
	SnowparkNotebooks ptrBool `json:"snowparkNotebooks,omitempty"`
	EmbeddedTerminal  ptrBool `json:"embeddedTerminal,omitempty"`
	GitIntegration    ptrBool `json:"gitIntegration,omitempty"`
}

// adminPerfDiag is the "performanceDiagnostics" category.
type adminPerfDiag struct {
	QueryProfile ptrBool `json:"queryProfile,omitempty"`
	ExplainSQL   ptrBool `json:"explainSql,omitempty"`
	QueryLog     ptrBool `json:"queryLog,omitempty"`
}

// adminConnection is the "connection" category.
type adminConnection struct {
	SnowflakeCLIProfileManager ptrBool `json:"snowflakeCLIProfileManager,omitempty"`
}

// adminResultsGrid is the "resultsGrid" category.
type adminResultsGrid struct {
	MultiCellCopy   ptrBool `json:"multiCellCopy,omitempty"`
	CellDetailPanel ptrBool `json:"cellDetailPanel,omitempty"`
	ColumnReorder   ptrBool `json:"columnReorder,omitempty"`
}

// adminFileBrowser is the "fileBrowser" category.
type adminFileBrowser struct {
	FileWatcher ptrBool `json:"fileWatcher,omitempty"`
}

// adminSchemaManagement is the "schemaManagement" category.
type adminSchemaManagement struct {
	ColumnManagement ptrBool `json:"columnManagement,omitempty"`
}

// adminIntegrations is the "integrations" category.
type adminIntegrations struct {
	MCPServer ptrBool `json:"mcpServer,omitempty"`
}

// adminLogging is the "logging" category. Unlike the boolean feature
// categories it enforces LogPrefs values. A non-nil field both sets the value
// and locks the field in the UI.
type adminLogging struct {
	LogLevel               *string `json:"logLevel,omitempty"`
	IncludeQuerySQL        ptrBool `json:"includeQuerySQL,omitempty"`
	IncludeInternalQueries ptrBool `json:"includeInternalQueries,omitempty"`
}

// adminConfigJSON is the full schema for the admin features.json file.
type adminConfigJSON struct {
	DataExportImport         adminDataExportImport `json:"dataExportImport"`
	GovernanceAdministration adminGovernance       `json:"governanceAdministration"`
	AI                       adminAI               `json:"ai"`
	AdvancedTools            adminAdvancedTools    `json:"advancedTools"`
	DeveloperEnvironments    adminDevEnv           `json:"developerEnvironments"`
	PerformanceDiagnostics   adminPerfDiag         `json:"performanceDiagnostics"`
	Connection               adminConnection       `json:"connection"`
	ResultsGrid              adminResultsGrid      `json:"resultsGrid"`
	FileBrowser              adminFileBrowser      `json:"fileBrowser"`
	SchemaManagement         adminSchemaManagement `json:"schemaManagement"`
	Integrations             adminIntegrations     `json:"integrations"`
	Logging                  adminLogging          `json:"logging"`
}

// ─── System config file path ───────────────────────────────────────────────────

// systemFeaturesPath returns the platform-appropriate path for the admin
// features.json file. Returns an empty string on unsupported platforms.
func systemFeaturesPath() string {
	switch runtime.GOOS {
	case "darwin":
		return "/Library/Application Support/Thaw/features.json"
	case "windows":
		programData := os.Getenv("PROGRAMDATA")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return filepath.Join(programData, "Thaw", "features.json")
	default: // linux and others
		return "/etc/thaw/features.json"
	}
}

// loadAdminJSON reads the system-level features.json, if present.
// Returns a zero adminConfigJSON (all nils) when the file does not exist.
func loadAdminJSON() adminConfigJSON {
	path := systemFeaturesPath()
	if path == "" {
		return adminConfigJSON{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return adminConfigJSON{}
	}
	var cfg adminConfigJSON
	_ = json.Unmarshal(data, &cfg)
	return cfg
}

// ─── Platform override hook ────────────────────────────────────────────────────
// loadPlatformAdminOverrides is implemented per-platform (darwin, windows, other)
// and applies MDM / registry values on top of the base admin JSON config.
// The function signature is the same on all platforms but the implementation
// differs. Platform files must not import adminConfigJSON fields by pointer
// aliases — they receive a copy and return a modified copy.

// applyPlatformOverrides is defined in adminconfig_{darwin,windows,other}.go.
// It takes the base JSON config and enriches it with MDM / registry values.
// Callers should use LoadAdminConfig() rather than this function directly.

// ─── Public API ────────────────────────────────────────────────────────────────

// LoadAdminConfig reads all system-level feature override sources and returns:
//   - effective: the FeatureFlags with admin overrides applied on top of defaults
//   - locked:    a FeatureFlags where true means the field is admin-controlled
//     (user may not change it via the UI)
func LoadAdminConfig(user FeatureFlags) (effective FeatureFlags, locked FeatureFlags) {
	base := loadAdminJSON()
	cfg := applyPlatformOverrides(base) // platform hook (darwin/windows/other)
	return mergeAdminOverrides(user, cfg)
}

// ─── Merge logic ──────────────────────────────────────────────────────────────

// mergeAdminOverrides applies cfg on top of user, returning the effective flags
// and a locked mask (true = this flag is controlled by admin).
func mergeAdminOverrides(user FeatureFlags, cfg adminConfigJSON) (effective FeatureFlags, locked FeatureFlags) {
	effective = user

	apply := func(target *bool, lockedField *bool, override ptrBool) {
		if override != nil {
			*target = *override
			*lockedField = true
		}
	}

	// Data Export & Import
	apply(&effective.ResultsetExport, &locked.ResultsetExport, cfg.DataExportImport.ResultsetExport)
	apply(&effective.ExportTableData, &locked.ExportTableData, cfg.DataExportImport.TableDataExport)
	apply(&effective.TableDataImport, &locked.TableDataImport, cfg.DataExportImport.TableDataImport)
	apply(&effective.DDLExport, &locked.DDLExport, cfg.DataExportImport.DDLExport)

	// Governance & Administration
	apply(&effective.UserRoleManagement, &locked.UserRoleManagement, cfg.GovernanceAdministration.UserRoleManagement)
	apply(&effective.WarehouseManagement, &locked.WarehouseManagement, cfg.GovernanceAdministration.WarehouseManagement)
	apply(&effective.WarehouseCreditUsage, &locked.WarehouseCreditUsage, cfg.GovernanceAdministration.WarehouseCreditUsage)
	apply(&effective.QueryActivityHistory, &locked.QueryActivityHistory, cfg.GovernanceAdministration.QueryActivityHistory)
	apply(&effective.IntegrationsManagement, &locked.IntegrationsManagement, cfg.GovernanceAdministration.IntegrationsManagement)
	apply(&effective.BackupPoliciesAndSets, &locked.BackupPoliciesAndSets, cfg.GovernanceAdministration.BackupPoliciesAndSets)

	// AI & Assistance
	apply(&effective.AIInlineCompletions, &locked.AIInlineCompletions, cfg.AI.AIInlineCompletions)

	// Advanced Tools & Data Engineering
	apply(&effective.SchemaMigration, &locked.SchemaMigration, cfg.AdvancedTools.SchemaMigration)
	apply(&effective.DbtScaffolding, &locked.DbtScaffolding, cfg.AdvancedTools.DbtScaffolding)
	apply(&effective.DbtProjectBrowser, &locked.DbtProjectBrowser, cfg.AdvancedTools.DbtProjectBrowser)
	apply(&effective.ERDiagramDesigner, &locked.ERDiagramDesigner, cfg.AdvancedTools.ERDiagramDesigner)
	apply(&effective.TaskGraphVisualizer, &locked.TaskGraphVisualizer, cfg.AdvancedTools.TaskGraphVisualizer)
	apply(&effective.InsertMapping, &locked.InsertMapping, cfg.AdvancedTools.InsertMapping)
	apply(&effective.InsertRow, &locked.InsertRow, cfg.AdvancedTools.InsertRow)
	apply(&effective.CodeSnippets, &locked.CodeSnippets, cfg.AdvancedTools.CodeSnippets)

	// Developer Environments
	apply(&effective.SnowparkNotebooks, &locked.SnowparkNotebooks, cfg.DeveloperEnvironments.SnowparkNotebooks)
	apply(&effective.EmbeddedTerminal, &locked.EmbeddedTerminal, cfg.DeveloperEnvironments.EmbeddedTerminal)
	apply(&effective.GitIntegration, &locked.GitIntegration, cfg.DeveloperEnvironments.GitIntegration)

	// Performance & Diagnostics
	apply(&effective.QueryProfile, &locked.QueryProfile, cfg.PerformanceDiagnostics.QueryProfile)
	apply(&effective.ExplainSQL, &locked.ExplainSQL, cfg.PerformanceDiagnostics.ExplainSQL)
	apply(&effective.QueryLog, &locked.QueryLog, cfg.PerformanceDiagnostics.QueryLog)

	// Connection
	apply(&effective.SnowflakeCLIProfileManager, &locked.SnowflakeCLIProfileManager, cfg.Connection.SnowflakeCLIProfileManager)

	// Results Grid
	apply(&effective.MultiCellCopy, &locked.MultiCellCopy, cfg.ResultsGrid.MultiCellCopy)
	apply(&effective.CellDetailPanel, &locked.CellDetailPanel, cfg.ResultsGrid.CellDetailPanel)
	apply(&effective.ColumnReorder, &locked.ColumnReorder, cfg.ResultsGrid.ColumnReorder)

	// File Browser
	apply(&effective.FileWatcher, &locked.FileWatcher, cfg.FileBrowser.FileWatcher)

	// Schema Management
	apply(&effective.ColumnManagement, &locked.ColumnManagement, cfg.SchemaManagement.ColumnManagement)

	// Integrations
	apply(&effective.MCPServer, &locked.MCPServer, cfg.Integrations.MCPServer)

	return effective, locked
}

// LoadAdminLogPrefs applies any system-level logging policy from features.json
// on top of the user's LogPrefs and returns:
//   - effective: the LogPrefs with admin overrides applied
//   - locked:    a LogPrefsLocked where true means the field is admin-controlled
//     (the user may not change it via the UI)
//
// Logging policy is read from the JSON file only (there is no MDM/registry hook
// for it, unlike the boolean feature flags).
func LoadAdminLogPrefs(user LogPrefs) (effective LogPrefs, locked LogPrefsLocked) {
	return mergeAdminLogPrefs(user, loadAdminJSON().Logging)
}

// mergeAdminLogPrefs applies the "logging" admin category on top of the user's
// LogPrefs, returning the effective values and a locked mask. Factored out from
// LoadAdminLogPrefs (which supplies the on-disk JSON) so the merge logic is unit
// testable without a features.json on disk — mirroring mergeAdminOverrides.
func mergeAdminLogPrefs(user LogPrefs, cfg adminLogging) (effective LogPrefs, locked LogPrefsLocked) {
	effective = user
	if cfg.LogLevel != nil && ValidLogLevel(*cfg.LogLevel) {
		effective.LogLevel = *cfg.LogLevel
		locked.LogLevel = true
	}
	if cfg.IncludeQuerySQL != nil {
		effective.IncludeQuerySQL = *cfg.IncludeQuerySQL
		locked.IncludeQuerySQL = true
	}
	if cfg.IncludeInternalQueries != nil {
		effective.IncludeInternalQueries = *cfg.IncludeInternalQueries
		locked.IncludeInternalQueries = true
		// Internal-query logging has no effect without SQL logging. When an
		// admin forces it on but doesn't explicitly set includeQuerySQL, imply
		// and lock includeQuerySQL=true so the audit policy actually takes
		// effect instead of silently normalizing back to off. An explicit
		// includeQuerySQL (even false) is honored as-is — that contradictory
		// combination is the admin's own choice.
		if *cfg.IncludeInternalQueries && cfg.IncludeQuerySQL == nil {
			effective.IncludeQuerySQL = true
			locked.IncludeQuerySQL = true
		}
	}
	return effective, locked
}
