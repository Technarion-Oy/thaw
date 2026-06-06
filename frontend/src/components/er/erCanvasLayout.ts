// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import dagre from "@dagrejs/dagre";
import type { Node, Edge } from "@xyflow/react";
import type { snowflake } from "../../../wailsjs/go/models";
import {
  type DesignerTable,
  SF_TYPES,
  ER_NODE_WIDTH,
  ER_NODE_HEADER_HEIGHT,
  ER_NODE_ROW_HEIGHT,
  ER_NODE_PADDING,
  ER_COL_LIMIT,
} from "./erTypes";

/** Calculate the pixel height of a table node based on its column count. */
function nodeHeight(colCount: number): number {
  const rows = Math.min(colCount, ER_COL_LIMIT) + (colCount > ER_COL_LIMIT ? 1 : 0);
  return ER_NODE_HEADER_HEIGHT + rows * ER_NODE_ROW_HEIGHT + ER_NODE_PADDING;
}

export interface ERTableNodeData {
  table: DesignerTable;
  selected: boolean;
  mode: "edit" | "readonly";
  onTableRename?: (tableId: string, newName: string) => void;
  onColumnRename?: (tableId: string, colId: string, newName: string) => void;
  [key: string]: unknown;
}

/**
 * Convert DesignerTable[] to XYFlow Node[] + Edge[].
 * Edges are derived from column fkRef fields.
 */
export function tablesToNodesAndEdges(
  tables: DesignerTable[],
  mode: "edit" | "readonly",
  callbacks?: {
    onTableRename?: (tableId: string, newName: string) => void;
    onColumnRename?: (tableId: string, colId: string, newName: string) => void;
  },
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = tables.map((t) => ({
    id: t.id,
    type: "erTable",
    position: { x: 0, y: 0 },
    data: {
      table: t,
      selected: false,
      mode,
      onTableRename: callbacks?.onTableRename,
      onColumnRename: callbacks?.onColumnRename,
    } satisfies ERTableNodeData,
    width: ER_NODE_WIDTH,
    height: nodeHeight(t.columns.length),
  }));

  // Build a lookup: "SCHEMA.TABLE" (uppercase) → tableId
  const tableKey = (schema: string, name: string) =>
    `${schema.toUpperCase()}.${name.trim().toUpperCase()}`;
  const keyToId = new Map<string, string>();
  for (const t of tables) {
    if (t.schema && t.name.trim()) {
      keyToId.set(tableKey(t.schema, t.name), t.id);
    }
  }

  const edges: Edge[] = [];
  for (const t of tables) {
    for (const c of t.columns) {
      if (!c.fkRef) continue;
      const parts = c.fkRef.split(".");
      if (parts.length !== 3) continue;
      const [refSchema, refTable, refCol] = parts;
      const targetTableId = keyToId.get(tableKey(refSchema, refTable));
      if (!targetTableId) continue;

      // Find the target column id
      const targetTable = tables.find((tt) => tt.id === targetTableId);
      if (!targetTable) continue;
      const targetCol = targetTable.columns.find(
        (tc) => tc.name.trim().toUpperCase() === refCol.trim().toUpperCase(),
      );
      if (!targetCol) continue;

      edges.push({
        id: `fk-${t.id}-${c.id}-${targetTableId}-${targetCol.id}`,
        source: t.id,
        target: targetTableId,
        sourceHandle: `col-source-${c.id}`,
        targetHandle: `col-target-${targetCol.id}`,
        type: "smoothstep",
        animated: true,
        style: { stroke: "var(--accent)", strokeWidth: 1.5 },
        label: "FK",
        labelStyle: { fontSize: 10, fill: "var(--text-muted)" },
        labelBgStyle: { fill: "var(--bg-overlay)", fillOpacity: 0.8 },
      });
    }
  }

  return { nodes, edges };
}

/**
 * Apply dagre auto-layout to ER nodes.
 * Uses each node's actual height (dynamic based on column count).
 */
export function applyERLayout(nodes: Node[], edges: Edge[]): Node[] {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "TB", nodesep: 60, ranksep: 120 });

  for (const n of nodes) {
    const table = (n.data as ERTableNodeData).table;
    const h = nodeHeight(table.columns.length);
    g.setNode(n.id, { width: ER_NODE_WIDTH, height: h });
  }

  for (const e of edges) {
    g.setEdge(e.source, e.target);
  }

  dagre.layout(g);

  return nodes.map((n) => {
    const pos = g.node(n.id);
    return {
      ...n,
      position: {
        x: pos.x - ER_NODE_WIDTH / 2,
        y: pos.y - (pos.height ?? 0) / 2,
      },
    };
  });
}

/** Map Snowflake INFORMATION_SCHEMA type strings to the canonical SF_TYPES list. */
export function normalizeDataType(dt: string): string {
  const base = dt.replace(/\s*\([^)]*\)/g, "").trim().toUpperCase();
  if (SF_TYPES.includes(base)) return base;
  const aliases: Record<string, string> = {
    TEXT: "VARCHAR", STRING: "VARCHAR", CHAR: "VARCHAR", CHARACTER: "VARCHAR",
    NCHAR: "VARCHAR", NVARCHAR: "VARCHAR", NVARCHAR2: "VARCHAR",
    INT: "NUMBER", INTEGER: "NUMBER", BIGINT: "NUMBER", SMALLINT: "NUMBER",
    TINYINT: "NUMBER", BYTEINT: "NUMBER", DECIMAL: "NUMBER", NUMERIC: "NUMBER",
    DOUBLE: "FLOAT", REAL: "FLOAT", FLOAT4: "FLOAT", FLOAT8: "FLOAT",
    BOOL: "BOOLEAN",
    DATETIME: "TIMESTAMP_NTZ", TIMESTAMP: "TIMESTAMP_NTZ",
    TIMESTAMP_TZ: "TIMESTAMP_LTZ",
  };
  return aliases[base] ?? "VARCHAR";
}

/** Convert snowflake.ERDiagramData to DesignerTable[] with FK wiring. */
export function initFromERData(data: snowflake.ERDiagramData): DesignerTable[] {
  const tables: DesignerTable[] = data.tables.map((t) => ({
    id: crypto.randomUUID(),
    schema: t.schema,
    name: t.name,
    columns: t.columns.map((c) => ({
      id: crypto.randomUUID(),
      name: c.name,
      dataType: normalizeDataType(c.dataType),
      isPK: c.isPK,
      notNull: c.isPK || c.nullable === "NO",
      fkRef: "",
    })),
  }));

  // Wire up FK references
  for (const fk of data.fks ?? []) {
    const tbl = tables.find((t) => t.schema === fk.fromSchema && t.name === fk.fromTable);
    if (!tbl) continue;
    const col = tbl.columns.find((c) => c.name === fk.fromCol);
    if (!col) continue;
    col.fkRef = `${fk.toSchema}.${fk.toTable}.${fk.toCol}`;
  }

  return tables;
}
