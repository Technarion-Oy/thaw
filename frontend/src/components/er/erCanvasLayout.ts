// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import dagre from "@dagrejs/dagre";
import { MarkerType, type Node, type Edge } from "@xyflow/react";
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

/** Fallback accent color when CSS variable is unavailable (SSR, tests, or empty value). */
const ACCENT_FALLBACK = "#58a6ff";

/** Resolve the --accent CSS variable to a hex value for SVG markers.
 *  CSS variables don't work inside SVG marker definitions, so we need the
 *  computed value. Called per `tablesToNodesAndEdges` invocation (not a hot
 *  path) so the color stays correct after theme changes. */
function resolveAccentHex(): string {
  if (typeof document === "undefined") return ACCENT_FALLBACK;
  return getComputedStyle(document.documentElement).getPropertyValue("--accent").trim() || ACCENT_FALLBACK;
}

/** Calculate the pixel height of a table node based on its column count. */
function nodeHeight(colCount: number): number {
  const rows = Math.min(colCount, ER_COL_LIMIT) + (colCount > ER_COL_LIMIT ? 1 : 0);
  return ER_NODE_HEADER_HEIGHT + rows * ER_NODE_ROW_HEIGHT + ER_NODE_PADDING;
}

export interface ERTableNodeData {
  table: DesignerTable;
  mode: "edit" | "readonly";
  onTableRename?: (tableId: string, newName: string) => void;
  onColumnRename?: (tableId: string, colId: string, newName: string) => void;
  onColumnRemove?: (tableId: string, colId: string) => void;
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
    onColumnRemove?: (tableId: string, colId: string) => void;
  },
): { nodes: Node[]; edges: Edge[] } {
  const nodes: Node[] = tables.map((t) => ({
    id: t.id,
    type: "erTable",
    position: { x: 0, y: 0 },
    data: {
      table: t,
      mode,
      onTableRename: callbacks?.onTableRename,
      onColumnRename: callbacks?.onColumnRename,
      onColumnRemove: callbacks?.onColumnRemove,
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

  const accentHex = resolveAccentHex();

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
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: accentHex,
          width: 16,
          height: 16,
        },
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

/**
 * Normalise a Snowflake INFORMATION_SCHEMA data type string to a canonical
 * form.  Preserves any parenthesised parameters (e.g. "VARCHAR(50)" stays
 * "VARCHAR(50)", "NUMBER(10,2)" stays "NUMBER(10,2)").  Unknown aliases
 * (TEXT, STRING, NVARCHAR, …) are mapped to their canonical base type.
 */
export function normalizeDataType(dt: string): string {
  const paramsMatch = dt.match(/(\([^)]*\))\s*$/);
  const params = paramsMatch ? paramsMatch[1] : "";
  const base = dt.replace(/\s*\([^)]*\)/g, "").trim().toUpperCase();

  // SF_TYPES includes all canonical Snowflake types plus multi-word forms
  // like "DOUBLE PRECISION", so they're returned as-is (e.g. INT stays INT).
  if (SF_TYPES.includes(base)) return base + params;

  // Only aliases NOT already in SF_TYPES need to be mapped here.
  // Types like INT, TEXT, DATETIME, DECIMAL, etc. are valid Snowflake types
  // included in SF_TYPES, so they pass through as-is above.
  const aliases: Record<string, string> = {
    NCHAR: "VARCHAR", NVARCHAR: "VARCHAR", NVARCHAR2: "VARCHAR",
    BOOL: "BOOLEAN",
  };
  return (aliases[base] ?? "VARCHAR") + params;
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
