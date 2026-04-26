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
//	  "ai":                       { "aiChat": false, "aiInlineCompletions": false },
//	  "advancedTools":            { "schemaMigration": false },
//	  "developerEnvironments":    { "snowparkNotebooks": false },
//	  "performanceDiagnostics":   { "explainSql": false }
//	}

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
	AIChat              ptrBool `json:"aiChat,omitempty"`
	AIInlineCompletions ptrBool `json:"aiInlineCompletions,omitempty"`
	AIImportSuggest     ptrBool `json:"aiImportSuggest,omitempty"`
}

// adminAdvancedTools is the "advancedTools" category.
type adminAdvancedTools struct {
	SchemaMigration     ptrBool `json:"schemaMigration,omitempty"`
	DbtScaffolding      ptrBool `json:"dbtScaffolding,omitempty"`
	ERDiagramDesigner   ptrBool `json:"erDiagramDesigner,omitempty"`
	TaskGraphVisualizer ptrBool `json:"taskGraphVisualizer,omitempty"`
	InsertMapping       ptrBool `json:"insertMapping,omitempty"`
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
}

// adminConfigJSON is the full schema for the admin features.json file.
type adminConfigJSON struct {
	DataExportImport         adminDataExportImport `json:"dataExportImport"`
	GovernanceAdministration adminGovernance       `json:"governanceAdministration"`
	AI                       adminAI               `json:"ai"`
	AdvancedTools            adminAdvancedTools    `json:"advancedTools"`
	DeveloperEnvironments    adminDevEnv           `json:"developerEnvironments"`
	PerformanceDiagnostics   adminPerfDiag         `json:"performanceDiagnostics"`
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
	apply(&effective.AIChat, &locked.AIChat, cfg.AI.AIChat)
	apply(&effective.AIInlineCompletions, &locked.AIInlineCompletions, cfg.AI.AIInlineCompletions)
	apply(&effective.AIImportSuggest, &locked.AIImportSuggest, cfg.AI.AIImportSuggest)

	// Advanced Tools & Data Engineering
	apply(&effective.SchemaMigration, &locked.SchemaMigration, cfg.AdvancedTools.SchemaMigration)
	apply(&effective.DbtScaffolding, &locked.DbtScaffolding, cfg.AdvancedTools.DbtScaffolding)
	apply(&effective.ERDiagramDesigner, &locked.ERDiagramDesigner, cfg.AdvancedTools.ERDiagramDesigner)
	apply(&effective.TaskGraphVisualizer, &locked.TaskGraphVisualizer, cfg.AdvancedTools.TaskGraphVisualizer)
	apply(&effective.InsertMapping, &locked.InsertMapping, cfg.AdvancedTools.InsertMapping)
	apply(&effective.CodeSnippets, &locked.CodeSnippets, cfg.AdvancedTools.CodeSnippets)

	// Developer Environments
	apply(&effective.SnowparkNotebooks, &locked.SnowparkNotebooks, cfg.DeveloperEnvironments.SnowparkNotebooks)
	apply(&effective.EmbeddedTerminal, &locked.EmbeddedTerminal, cfg.DeveloperEnvironments.EmbeddedTerminal)
	apply(&effective.GitIntegration, &locked.GitIntegration, cfg.DeveloperEnvironments.GitIntegration)

	// Performance & Diagnostics
	apply(&effective.QueryProfile, &locked.QueryProfile, cfg.PerformanceDiagnostics.QueryProfile)
	apply(&effective.ExplainSQL, &locked.ExplainSQL, cfg.PerformanceDiagnostics.ExplainSQL)

	return effective, locked
}
