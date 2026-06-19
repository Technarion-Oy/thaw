// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import { SNOWFLAKE_DATA_TYPES, SNOWFLAKE_DATA_TYPE_NAMES } from "../../generated/snowflakeDataTypes";

export interface DesignerColumn {
  id: string;
  name: string;
  dataType: string;
  isPK: boolean;
  notNull: boolean;
  fkRef: string; // "SCHEMA.TABLE.COLUMN" or "" for none
}

export interface DesignerTable {
  id: string;
  schema: string;
  name: string;
  columns: DesignerColumn[];
}

/**
 * Full Snowflake data type catalogue.  Sourced from the generated artifact
 * (frontend/src/generated/snowflakeDataTypes.ts), whose single source of truth
 * is the Go registry in internal/snowflake/datatypes.go.  Each entry has a
 * canonical name and an optional parameter hint for the UI.
 */
export const SF_DATA_TYPES: { name: string; paramHint: string }[] =
  SNOWFLAKE_DATA_TYPES.map((dt) => ({ name: dt.name, paramHint: dt.paramHint }));

/** Flat list of canonical type names (for normalizeDataType lookups). */
export const SF_TYPES: string[] = [...SNOWFLAKE_DATA_TYPE_NAMES];

/**
 * Normalise a Snowflake identifier following Snowflake conventions:
 *   - Wrapped in double quotes → preserve inner case, keep quotes in stored name
 *   - Not quoted → uppercase the whole value
 *
 * This is applied on blur / commit, NOT on every keystroke, so the user
 * can freely type quotes and mixed case while editing.
 */
export function normalizeIdentifier(raw: string): string {
  const trimmed = raw.trim();
  if (trimmed.startsWith('"') && trimmed.endsWith('"') && trimmed.length >= 2) {
    // Quoted identifier — preserve case, keep quotes
    return trimmed;
  }
  return trimmed.toUpperCase();
}

// ── Join Query Builder types ─────────────────────────────────────────────────
// These mirror the Go types in internal/erdesigner/ and the Wails-generated
// models. Keeping hand-written interfaces avoids coupling to generation timing.

export interface FKPair {
  from: { schema: string; table: string; col: string };
  to: { schema: string; table: string; col: string };
}

export interface JoinEntry {
  table: { schema: string; name: string };
  joinType: "INNER" | "LEFT" | "RIGHT" | "FULL OUTER";
  onCondition: string;
  /** Structured FK column pairs used in this join — avoids reverse-parsing onCondition for highlighting. */
  fkPairs: FKPair[];
  isIntermediate: boolean;
}

export interface JoinQueryState {
  database: string;
  baseTable: { schema: string; name: string };
  joins: JoinEntry[];
  selectedColumns: Record<string, string[]>; // "SCHEMA.TABLE" → column names (empty = *)
}

export interface JoinPath {
  tables: { schema: string; name: string }[];
  edges: { from: { schema: string; table: string; col: string }; to: { schema: string; table: string; col: string } }[];
}

/** Canonical key for a table: "SCHEMA.TABLE" (both parts trimmed, case-preserved).
 *  Matches Go's `snowflake.TableKey` which trims both parts. */
export const tableKey = (schema: string, name: string) =>
  `${schema.trim()}.${name.trim()}`;

export const ER_NODE_WIDTH = 240;
export const ER_NODE_HEADER_HEIGHT = 32;
export const ER_NODE_ROW_HEIGHT = 24;
export const ER_NODE_PADDING = 8;
export const ER_COL_LIMIT = 30;
