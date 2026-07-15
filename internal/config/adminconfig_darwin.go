// SPDX-License-Identifier: GPL-3.0-or-later

//go:build darwin

package config

import (
	"encoding/json"
	"os"
	"os/exec"
)

// macOS managed preference plist paths (highest priority first).
// IT admins push .mobileconfig profiles that write to the Managed Preferences
// path; the user-level path can also be populated via MDM user-channel profiles.
var macOSPlistPaths = []string{
	"/Library/Managed Preferences/com.thaw.app.plist",
	os.ExpandEnv("${HOME}/Library/Preferences/com.thaw.app.plist"),
}

// plistKeys maps plist Disable<Feature> keys to the corresponding
// adminConfigJSON pointer fields. A value of true in the plist means
// "disable this feature", so we store a pointer to false.
//
// Key naming convention: "Disable" + PascalCase feature name.
type plistFeatureKey struct {
	key      string
	apply    func(cfg *adminConfigJSON, disabled bool)
}

var plistFeatureKeys = []plistFeatureKey{
	{"DisableResultsetExport", func(c *adminConfigJSON, v bool) { c.DataExportImport.ResultsetExport = boolPtr(!v) }},
	{"DisableTableDataExport", func(c *adminConfigJSON, v bool) { c.DataExportImport.TableDataExport = boolPtr(!v) }},
	{"DisableTableDataImport", func(c *adminConfigJSON, v bool) { c.DataExportImport.TableDataImport = boolPtr(!v) }},
	{"DisableDDLExport", func(c *adminConfigJSON, v bool) { c.DataExportImport.DDLExport = boolPtr(!v) }},

	{"DisableUserRoleManagement", func(c *adminConfigJSON, v bool) { c.GovernanceAdministration.UserRoleManagement = boolPtr(!v) }},
	{"DisableWarehouseManagement", func(c *adminConfigJSON, v bool) { c.GovernanceAdministration.WarehouseManagement = boolPtr(!v) }},
	{"DisableWarehouseCreditUsage", func(c *adminConfigJSON, v bool) { c.GovernanceAdministration.WarehouseCreditUsage = boolPtr(!v) }},
	{"DisableQueryActivityHistory", func(c *adminConfigJSON, v bool) { c.GovernanceAdministration.QueryActivityHistory = boolPtr(!v) }},
	{"DisableIntegrationsManagement", func(c *adminConfigJSON, v bool) { c.GovernanceAdministration.IntegrationsManagement = boolPtr(!v) }},
	{"DisableBackupPoliciesAndSets", func(c *adminConfigJSON, v bool) { c.GovernanceAdministration.BackupPoliciesAndSets = boolPtr(!v) }},

	{"DisableAIInlineCompletions", func(c *adminConfigJSON, v bool) { c.AI.AIInlineCompletions = boolPtr(!v) }},

	{"DisableSchemaMigration", func(c *adminConfigJSON, v bool) { c.AdvancedTools.SchemaMigration = boolPtr(!v) }},
	{"DisableDbtScaffolding", func(c *adminConfigJSON, v bool) { c.AdvancedTools.DbtScaffolding = boolPtr(!v) }},
	{"DisableERDiagramDesigner", func(c *adminConfigJSON, v bool) { c.AdvancedTools.ERDiagramDesigner = boolPtr(!v) }},
	{"DisableTaskGraphVisualizer", func(c *adminConfigJSON, v bool) { c.AdvancedTools.TaskGraphVisualizer = boolPtr(!v) }},
	{"DisableInsertMapping", func(c *adminConfigJSON, v bool) { c.AdvancedTools.InsertMapping = boolPtr(!v) }},
	{"DisableInsertRow", func(c *adminConfigJSON, v bool) { c.AdvancedTools.InsertRow = boolPtr(!v) }},
	{"DisableCodeSnippets", func(c *adminConfigJSON, v bool) { c.AdvancedTools.CodeSnippets = boolPtr(!v) }},

	{"DisableSnowparkNotebooks", func(c *adminConfigJSON, v bool) { c.DeveloperEnvironments.SnowparkNotebooks = boolPtr(!v) }},
	{"DisableEmbeddedTerminal", func(c *adminConfigJSON, v bool) { c.DeveloperEnvironments.EmbeddedTerminal = boolPtr(!v) }},
	{"DisableGitIntegration", func(c *adminConfigJSON, v bool) { c.DeveloperEnvironments.GitIntegration = boolPtr(!v) }},

	{"DisableQueryProfile", func(c *adminConfigJSON, v bool) { c.PerformanceDiagnostics.QueryProfile = boolPtr(!v) }},
	{"DisableExplainSQL", func(c *adminConfigJSON, v bool) { c.PerformanceDiagnostics.ExplainSQL = boolPtr(!v) }},

	{"DisableSnowflakeCLIProfileManager", func(c *adminConfigJSON, v bool) { c.Connection.SnowflakeCLIProfileManager = boolPtr(!v) }},

	{"DisableColumnManagement", func(c *adminConfigJSON, v bool) { c.SchemaManagement.ColumnManagement = boolPtr(!v) }},

	{"DisableMCPServer", func(c *adminConfigJSON, v bool) { c.Integrations.MCPServer = boolPtr(!v) }},
}

func boolPtr(b bool) *bool { return &b }

// readPlist converts a plist file to JSON using plutil and returns a flat
// map of key → value. Returns nil if the file does not exist or conversion
// fails. Uses plutil (always present on macOS) to avoid CGo or binary plist
// parsing.
func readPlist(path string) map[string]interface{} {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	out, err := exec.Command("plutil", "-convert", "json", "-o", "-", path).Output()
	if err != nil {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(out, &m); err != nil {
		return nil
	}
	return m
}

// applyPlatformOverrides applies macOS managed preference plist values on top
// of the base JSON admin config. Plist values take the highest priority.
// Only the plist paths listed in macOSPlistPaths are checked; earlier entries
// take priority over later entries.
func applyPlatformOverrides(base adminConfigJSON) adminConfigJSON {
	cfg := base

	// Process plists in reverse priority order so higher-priority entries win.
	// We iterate in normal order and let later iterations overwrite earlier ones,
	// which means the last path in the slice (lowest priority) is applied first.
	// Reverse to apply highest priority last so it wins.
	for i := len(macOSPlistPaths) - 1; i >= 0; i-- {
		plist := readPlist(macOSPlistPaths[i])
		if plist == nil {
			continue
		}
		for _, kv := range plistFeatureKeys {
			if raw, ok := plist[kv.key]; ok {
				if disabled, ok := raw.(bool); ok {
					kv.apply(&cfg, disabled)
				}
			}
		}
	}

	return cfg
}
