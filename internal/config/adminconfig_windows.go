// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

//go:build windows

package config

import (
	"golang.org/x/sys/windows/registry"
)

// Windows registry paths for IT admin policy overrides.
// HKLM takes priority over HKCU.
const (
	regKeyLM = `SOFTWARE\Policies\Thaw\Features`
	regKeyCU = `SOFTWARE\Policies\Thaw\Features`
)

// registryFeatureEntry maps a registry DWORD value name to the
// adminConfigJSON field it controls.  A DWORD of 1 means "disable".
type registryFeatureEntry struct {
	name  string
	apply func(cfg *adminConfigJSON, disabled bool)
}

var registryFeatureEntries = []registryFeatureEntry{
	{"DisableResultsetExport", func(c *adminConfigJSON, v bool) { p := !v; c.DataExportImport.ResultsetExport = &p }},
	{"DisableTableDataExport", func(c *adminConfigJSON, v bool) { p := !v; c.DataExportImport.TableDataExport = &p }},
	{"DisableTableDataImport", func(c *adminConfigJSON, v bool) { p := !v; c.DataExportImport.TableDataImport = &p }},
	{"DisableDDLExport", func(c *adminConfigJSON, v bool) { p := !v; c.DataExportImport.DDLExport = &p }},

	{"DisableUserRoleManagement", func(c *adminConfigJSON, v bool) { p := !v; c.GovernanceAdministration.UserRoleManagement = &p }},
	{"DisableWarehouseManagement", func(c *adminConfigJSON, v bool) { p := !v; c.GovernanceAdministration.WarehouseManagement = &p }},
	{"DisableWarehouseCreditUsage", func(c *adminConfigJSON, v bool) { p := !v; c.GovernanceAdministration.WarehouseCreditUsage = &p }},
	{"DisableQueryActivityHistory", func(c *adminConfigJSON, v bool) { p := !v; c.GovernanceAdministration.QueryActivityHistory = &p }},
	{"DisableIntegrationsManagement", func(c *adminConfigJSON, v bool) { p := !v; c.GovernanceAdministration.IntegrationsManagement = &p }},
	{"DisableBackupPoliciesAndSets", func(c *adminConfigJSON, v bool) { p := !v; c.GovernanceAdministration.BackupPoliciesAndSets = &p }},

	{"DisableAIInlineCompletions", func(c *adminConfigJSON, v bool) { p := !v; c.AI.AIInlineCompletions = &p }},

	{"DisableSchemaMigration", func(c *adminConfigJSON, v bool) { p := !v; c.AdvancedTools.SchemaMigration = &p }},
	{"DisableDbtScaffolding", func(c *adminConfigJSON, v bool) { p := !v; c.AdvancedTools.DbtScaffolding = &p }},
	{"DisableERDiagramDesigner", func(c *adminConfigJSON, v bool) { p := !v; c.AdvancedTools.ERDiagramDesigner = &p }},
	{"DisableTaskGraphVisualizer", func(c *adminConfigJSON, v bool) { p := !v; c.AdvancedTools.TaskGraphVisualizer = &p }},
	{"DisableInsertMapping", func(c *adminConfigJSON, v bool) { p := !v; c.AdvancedTools.InsertMapping = &p }},
	{"DisableCodeSnippets", func(c *adminConfigJSON, v bool) { p := !v; c.AdvancedTools.CodeSnippets = &p }},

	{"DisableSnowparkNotebooks", func(c *adminConfigJSON, v bool) { p := !v; c.DeveloperEnvironments.SnowparkNotebooks = &p }},
	{"DisableEmbeddedTerminal", func(c *adminConfigJSON, v bool) { p := !v; c.DeveloperEnvironments.EmbeddedTerminal = &p }},
	{"DisableGitIntegration", func(c *adminConfigJSON, v bool) { p := !v; c.DeveloperEnvironments.GitIntegration = &p }},

	{"DisableQueryProfile", func(c *adminConfigJSON, v bool) { p := !v; c.PerformanceDiagnostics.QueryProfile = &p }},
	{"DisableExplainSQL", func(c *adminConfigJSON, v bool) { p := !v; c.PerformanceDiagnostics.ExplainSQL = &p }},

	{"DisableSnowflakeCLIProfileManager", func(c *adminConfigJSON, v bool) { p := !v; c.Connection.SnowflakeCLIProfileManager = &p }},
}

// readRegistryKey opens a registry key and applies feature policy values.
// A DWORD value of 1 means "disabled".  Missing values are skipped.
func readRegistryKey(cfg *adminConfigJSON, hive registry.Key, path string) {
	key, err := registry.OpenKey(hive, path, registry.QUERY_VALUE)
	if err != nil {
		return
	}
	defer key.Close()

	for _, entry := range registryFeatureEntries {
		val, _, err := key.GetIntegerValue(entry.name)
		if err != nil {
			continue
		}
		entry.apply(cfg, val == 1)
	}
}

// applyPlatformOverrides reads Windows Group Policy registry values on top of
// the base JSON admin config.  HKLM takes priority over HKCU (applied last).
func applyPlatformOverrides(base adminConfigJSON) adminConfigJSON {
	cfg := base
	// HKCU applied first (lower priority), HKLM applied second (wins).
	readRegistryKey(&cfg, registry.CURRENT_USER, regKeyCU)
	readRegistryKey(&cfg, registry.LOCAL_MACHINE, regKeyLM)
	return cfg
}
