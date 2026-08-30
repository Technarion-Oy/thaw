// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { quoteIdent } from "../shared/ObjectNameCaseControl";

export type RefreshMode = "AUTO" | "FULL" | "INCREMENTAL";

// Draft state for the Properties modal's "Add materialization" form.
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

export function isMaterializationValid(m: NewMaterialization): boolean {
  return m.name.trim() !== "" && m.warehouse !== "" && m.dimensions.length > 0 && m.metrics.length > 0;
}

// Builds the clause that follows `ALTER SEMANTIC VIEW <fqn>` to add a
// materialization, per
// https://docs.snowflake.com/en/sql-reference/sql/alter-semantic-view:
//
//   ADD MATERIALIZATION <name> WAREHOUSE = <warehouse>
//     [ REFRESH_MODE = { AUTO | FULL | INCREMENTAL } ]
//     [ IMMUTABLE WHERE ( <condition> ) ]
//   AS DIMENSIONS <dim> [, ...] METRICS <metric> [, ...] [ WHERE ( <condition> ) ]
//
// REFRESH_MODE is only emitted when it differs from Snowflake's own AUTO
// default. IMMUTABLE WHERE / WHERE are raw SQL boolean expressions (not
// free text), so they're parenthesized as typed rather than quoted as string
// literals — same treatment as the other WHERE filters in this modal's
// sibling create form.
export function buildAddMaterializationClause(m: NewMaterialization): string {
  let clause = `ADD MATERIALIZATION ${quoteIdent(m.name.trim())} WAREHOUSE = ${quoteIdent(m.warehouse)}`;
  if (m.refreshMode !== "AUTO") clause += ` REFRESH_MODE = ${m.refreshMode}`;
  if (m.immutableWhere.trim() !== "") clause += ` IMMUTABLE WHERE (${m.immutableWhere.trim()})`;
  clause += ` AS\n  DIMENSIONS ${m.dimensions.map(quoteIdent).join(", ")}`;
  clause += `\n  METRICS ${m.metrics.map(quoteIdent).join(", ")}`;
  if (m.where.trim() !== "") clause += `\n  WHERE (${m.where.trim()})`;
  return clause;
}
