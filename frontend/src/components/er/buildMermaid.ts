// Copyright (c) 2026 Technarion Oy. All rights reserved.

import type { DesignerTable } from "./erTypes";
import { ER_COL_LIMIT } from "./erTypes";

/** Replace non-alphanumeric chars with underscores; prefix _ if starts with a digit. */
export function sanitiseId(s: string): string {
  const id = s.replace(/[^a-zA-Z0-9_]/g, "_");
  return /^[0-9]/.test(id) ? "_" + id : id;
}

/** Stable entity ID in the form SCHEMA__TABLE. */
export function entityId(schema: string, table: string): string {
  return sanitiseId(schema) + "__" + sanitiseId(table);
}

/** Strip precision qualifiers like (38,0) and replace spaces so the type is a single token. */
export function shortType(dt: string): string {
  return dt.replace(/\s*\([^)]*\)/g, "").replace(/\s+/g, "_") || "string";
}

/**
 * Generate a Mermaid erDiagram definition from DesignerTable[].
 * If `visibleSchemas` is provided, only tables whose schema is in the set are included.
 */
export function buildMermaid(
  tables: DesignerTable[],
  visibleSchemas?: Set<string>,
): string {
  const lines: string[] = ["erDiagram"];
  const validTables = tables.filter(
    (t) => t.schema && t.name.trim() && (!visibleSchemas || visibleSchemas.has(t.schema)),
  );

  for (const t of validTables) {
    const allNamedCols = t.columns.filter((c) => c.name.trim());
    const namedCols = allNamedCols.slice(0, ER_COL_LIMIT);
    if (namedCols.length === 0) continue;
    const id = entityId(t.schema, t.name.trim());
    lines.push(`  ${id} {`);
    for (const c of namedCols) {
      const type = shortType(c.dataType) || "string";
      const pk = c.isPK ? " PK" : "";
      lines.push(`    ${type} ${sanitiseId(c.name)}${pk}`);
    }
    const overflow = allNamedCols.length - ER_COL_LIMIT;
    if (overflow > 0) {
      lines.push(`    string _and_${overflow}_more_columns`);
    }
    lines.push("  }");
  }

  // FK relationships — deduplicate by table pair
  const validKeys = new Set(
    validTables.map((t) => `${t.schema.toUpperCase()}\x00${t.name.trim().toUpperCase()}`),
  );
  const seen = new Set<string>();
  for (const t of validTables) {
    for (const c of t.columns) {
      if (!c.fkRef) continue;
      const parts = c.fkRef.split(".");
      if (parts.length !== 3) continue;
      const [refSchema, refTable] = parts;
      if (!refTable.trim()) continue;
      // Only include FK when both endpoints are visible
      if (!validKeys.has(`${refSchema.toUpperCase()}\x00${refTable.trim().toUpperCase()}`)) continue;
      const fromId = entityId(t.schema, t.name.trim());
      const toId = entityId(refSchema, refTable.trim());
      // Sort so bidirectional FKs between the same pair collapse into one line
      const pairKey = [fromId, toId].sort().join("__");
      if (seen.has(pairKey)) continue;
      seen.add(pairKey);
      lines.push(`  ${fromId} }o--|| ${toId} : "FK"`);
    }
  }

  return lines.join("\n");
}
