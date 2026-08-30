// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import type { snowflake } from "../../../wailsjs/go/models";

export type RefreshMode = "AUTO" | "FULL" | "INCREMENTAL";

// Draft state for the Properties modal's "Add materialization" form — the
// same shape as the Go backend's semanticview.MaterializationConfig (see
// BuildAddSemanticViewMaterializationSql), which is what actually builds the
// ALTER SEMANTIC VIEW ... ADD MATERIALIZATION SQL; this module only shapes
// the form's picker options and its own submit-button gating. dimensions/
// metrics hold unquoted `table.name` references (or a bare name when the
// SHOW result has no table_name) — see qualifiedOptionsFromResult.
export interface NewMaterialization {
  name: string;
  warehouse: string;
  refreshMode: RefreshMode;
  immutableWhere: string;
  dimensions: string[];
  metrics: string[];
  where: string;
}

export const NEW_MATERIALIZATION: NewMaterialization = {
  name: "", warehouse: "", refreshMode: "AUTO", immutableWhere: "", dimensions: [], metrics: [], where: "",
};

// Mirrors the requirement BuildAddSemanticViewMaterializationSql itself
// enforces server-side — this copy only gates the modal's Add button so it's
// disabled before a doomed round-trip, the same relationship
// AddDbtProjectVersionModal.tsx's local canSubmit has with BuildAddVersionSql.
export function isMaterializationValid(m: NewMaterialization): boolean {
  return m.name.trim() !== "" && m.warehouse !== "" && m.dimensions.length > 0 && m.metrics.length > 0;
}

// Turns a SHOW SEMANTIC DIMENSIONS/METRICS result into "table.name" options
// for the Add Materialization form's multi-selects. Snowflake's own ADD
// MATERIALIZATION example (docs.snowflake.com/en/sql-reference/sql/alter-semantic-view)
// qualifies every dimension/metric by its logical table — `DIMENSIONS
// customers.customer_name` — because a bare name is ambiguous the moment a
// view joins more than one logical table, which is the normal case. The
// reference is left unquoted here; BuildAddSemanticViewMaterializationSql
// (Go) does the quoting when it assembles the final statement, splitting on
// the last dot the same way internal/semanticview/sql.go's qualifiedIdent
// does elsewhere in this package. Column lookup is case-insensitive, matching
// every other SHOW/DESCRIBE reader in this codebase
// (internal/snowflake/result.go's ColIdx).
export function qualifiedOptionsFromResult(res: snowflake.QueryResult | null): string[] {
  if (!res) return [];
  const cols = res.columns.map((c) => c.toLowerCase());
  const nameI = cols.indexOf("name");
  const tableI = cols.indexOf("table_name");
  if (nameI < 0) return [];
  return res.rows.map((r) => {
    const nm = String(r[nameI]);
    const tbl = tableI >= 0 && r[tableI] != null ? String(r[tableI]) : "";
    return tbl ? `${tbl}.${nm}` : nm;
  });
}
