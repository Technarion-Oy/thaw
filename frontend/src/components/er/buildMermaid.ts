// Copyright (c) 2026 Technarion Oy. All rights reserved.

import type { snowflake } from "../../../wailsjs/go/models";

const COL_LIMIT = 30;

/** Replace non-alphanumeric chars with underscores; prefix _ if starts with a digit. */
function sanitiseId(s: string): string {
  const id = s.replace(/[^a-zA-Z0-9_]/g, "_");
  return /^[0-9]/.test(id) ? "_" + id : id;
}

/** Stable entity ID in the form SCHEMA__TABLE. */
function entityId(schema: string, table: string): string {
  return sanitiseId(schema) + "__" + sanitiseId(table);
}

/** Strip precision qualifiers like (38,0) and replace spaces so the type is a single token. */
function shortType(dt: string): string {
  return dt.replace(/\s*\([^)]*\)/g, "").replace(/\s+/g, "_") || "string";
}

/**
 * Generate a Mermaid erDiagram definition from ER diagram data.
 * Only tables whose schema is in `visibleSchemas` are included.
 * FK relationships where either end is hidden are also excluded.
 */
export function buildMermaid(
  data: snowflake.ERDiagramData,
  visibleSchemas: Set<string>,
): string {
  const lines: string[] = ["erDiagram"];

  const tables = data.tables.filter((t) => visibleSchemas.has(t.schema));

  for (const t of tables) {
    const id = entityId(t.schema, t.name);
    lines.push(`  ${id} {`);
    const cols = t.columns.slice(0, COL_LIMIT);
    for (const c of cols) {
      const type = shortType(c.dataType);
      const pk = c.isPK ? " PK" : "";
      lines.push(`    ${type} ${sanitiseId(c.name)}${pk}`);
    }
    if (t.columns.length > COL_LIMIT) {
      lines.push(`    string _and_${t.columns.length - COL_LIMIT}_more_columns`);
    }
    lines.push("  }");
  }

  // Include FK relationships only when both endpoints are visible
  const tableSet = new Set(tables.map((t) => t.schema + "\x00" + t.name));
  const fks = (data.fks ?? []).filter(
    (fk) =>
      tableSet.has(fk.fromSchema + "\x00" + fk.fromTable) &&
      tableSet.has(fk.toSchema + "\x00" + fk.toTable),
  );

  for (const fk of fks) {
    const fromId = entityId(fk.fromSchema, fk.fromTable);
    const toId = entityId(fk.toSchema, fk.toTable);
    lines.push(`  ${fromId} }o--|| ${toId} : "FK"`);
  }

  return lines.join("\n");
}
