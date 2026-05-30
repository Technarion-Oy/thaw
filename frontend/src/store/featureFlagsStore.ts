// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { create } from "zustand";
import { GetAdminLockedFlags, GetFeatureFlags } from "../../wailsjs/go/app/App";
import type { config } from "../../wailsjs/go/models";

// Optimistic defaults: every feature enabled until the backend responds.
const allEnabled: config.FeatureFlags = {
  initialized: true,
  version: 1,
  resultsetExport: true,
  exportTableData: true,
  tableDataImport: true,
  ddlExport: true,
  putCommand: true,
  getCommand: true,
  removeCommand: true,
  userRoleManagement: true,
  warehouseManagement: true,
  warehouseCreditUsage: true,
  queryActivityHistory: true,
  integrationsManagement: true,
  backupPoliciesAndSets: true,
  aiInlineCompletions: true,
  schemaMigration: true,
  dbtScaffolding: true,
  dbtProjectBrowser: true,
  erDiagramDesigner: true,
  taskGraphVisualizer: true,
  insertMapping: true,
  codeSnippets: true,
  snowparkNotebooks: true,
  embeddedTerminal: true,
  gitIntegration: true,
  queryProfile: true,
  explainSql: true,
  sqlDiagnostics: true,
  schemaAutocomplete: true,
  ddlHoverTooltips: true,
  fileFormatBuilder: true,
  snowflakeCLIProfileManager: true,
  multiCellCopy: true,
  crossTabSearch: true,
  fileWatcher: true,
  columnManagement: true,
  mcpServer: true,
};

// allLocked default: nothing is admin-locked.
const nothingLocked: config.FeatureFlags = {
  initialized: false,
  version: 0,
  resultsetExport: false,
  exportTableData: false,
  tableDataImport: false,
  ddlExport: false,
  putCommand: false,
  getCommand: false,
  removeCommand: false,
  userRoleManagement: false,
  warehouseManagement: false,
  warehouseCreditUsage: false,
  queryActivityHistory: false,
  integrationsManagement: false,
  backupPoliciesAndSets: false,
  aiInlineCompletions: false,
  schemaMigration: false,
  dbtScaffolding: false,
  dbtProjectBrowser: false,
  erDiagramDesigner: false,
  taskGraphVisualizer: false,
  insertMapping: false,
  codeSnippets: false,
  snowparkNotebooks: false,
  embeddedTerminal: false,
  gitIntegration: false,
  queryProfile: false,
  explainSql: false,
  sqlDiagnostics: false,
  schemaAutocomplete: false,
  ddlHoverTooltips: false,
  fileFormatBuilder: false,
  snowflakeCLIProfileManager: false,
  multiCellCopy: false,
  crossTabSearch: false,
  fileWatcher: false,
  columnManagement: false,
  mcpServer: false,
};

interface FeatureFlagsState {
  flags: config.FeatureFlags;
  /** Which flags are controlled by IT admin and cannot be changed by the user. */
  locked: config.FeatureFlags;
  /** Reload flags from the backend (call after SaveFeatureFlags). */
  load: () => Promise<void>;
}

export const useFeatureFlagsStore = create<FeatureFlagsState>((set) => ({
  flags: allEnabled,
  locked: nothingLocked,
  load: async () => {
    const [flags, locked] = await Promise.all([GetFeatureFlags(), GetAdminLockedFlags()]);
    set({ flags, locked });
  },
}));
