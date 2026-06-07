// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import type { JoinQueryState } from "./erTypes";
import { tableKey } from "./erTypes";

/**
 * Fully qualify a table name as DATABASE.SCHEMA.TABLE.
 */
function fqn(database: string, schema: string, table: string): string {
  return `${database}.${schema}.${table}`;
}

/**
 * Generate a formatted SELECT ... JOIN ... SQL string from a JoinQueryState.
 *
 * Assigns short aliases (t1, t2, ...) per table, uses fully-qualified
 * DATABASE.SCHEMA.TABLE names, and respects selectedColumns (defaults to *
 * when empty).
 */
export function buildJoinSQL(state: JoinQueryState): string {
  // Assign aliases: t1 for base, t2, t3, ... for joins
  const aliasMap = new Map<string, string>();

  aliasMap.set(
    tableKey(state.baseTable.schema, state.baseTable.name),
    "t1",
  );
  state.joins.forEach((j, i) => {
    aliasMap.set(tableKey(j.table.schema, j.table.name), `t${i + 2}`);
  });

  // Build SELECT columns
  const selectParts: string[] = [];

  const addColumnsForTable = (schema: string, name: string) => {
    const key = tableKey(schema, name);
    const alias = aliasMap.get(key)!;
    const cols = state.selectedColumns.get(key);
    if (!cols || cols.length === 0) {
      selectParts.push(`  ${alias}.*`);
    } else {
      for (const col of cols) {
        selectParts.push(`  ${alias}.${col}`);
      }
    }
  };

  addColumnsForTable(state.baseTable.schema, state.baseTable.name);
  for (const j of state.joins) {
    addColumnsForTable(j.table.schema, j.table.name);
  }

  // Build FROM clause
  const baseAlias = "t1";
  const fromLine = `FROM ${fqn(state.database, state.baseTable.schema, state.baseTable.name)} ${baseAlias}`;

  // Build JOIN clauses with aliased ON conditions
  const joinLines: string[] = [];
  for (const j of state.joins) {
    const key = tableKey(j.table.schema, j.table.name);
    const alias = aliasMap.get(key)!;
    const joinKw = j.joinType === "FULL OUTER" ? "FULL OUTER JOIN" : `${j.joinType} JOIN`;

    // Replace fully-qualified table references in ON condition with aliases
    let aliasedCondition = j.onCondition;
    for (const [tblKey, tblAlias] of aliasMap) {
      const [schema, table] = tblKey.split(".");
      // Replace SCHEMA.TABLE.COL with alias.COL (case-insensitive)
      const pattern = new RegExp(
        `${escapeRegex(schema)}\\.${escapeRegex(table)}\\.`,
        "gi",
      );
      aliasedCondition = aliasedCondition.replace(pattern, `${tblAlias}.`);
    }

    joinLines.push(`${joinKw} ${fqn(state.database, j.table.schema, j.table.name)} ${alias} ON ${aliasedCondition}`);
  }

  // Assemble — include a default LIMIT to prevent accidental full-table scans
  // on large Snowflake tables. The user can adjust or remove it in the editor.
  const lines = [
    "SELECT",
    selectParts.join(",\n"),
    fromLine,
    ...joinLines,
    "LIMIT 1000;",
  ];

  return lines.join("\n");
}

function escapeRegex(str: string): string {
  return str.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}
