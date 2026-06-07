// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

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
 * Full Snowflake data type catalogue, mirroring internal/snowflake/datatypes.go.
 * Each entry has a canonical name and an optional parameter hint for the UI.
 */
export const SF_DATA_TYPES: { name: string; paramHint: string }[] = [
  // Numeric — exact
  { name: "NUMBER", paramHint: "(precision, scale)" },
  { name: "DECIMAL", paramHint: "(precision, scale)" },
  { name: "NUMERIC", paramHint: "(precision, scale)" },
  { name: "INT", paramHint: "" },
  { name: "INTEGER", paramHint: "" },
  { name: "BIGINT", paramHint: "" },
  { name: "SMALLINT", paramHint: "" },
  { name: "TINYINT", paramHint: "" },
  { name: "BYTEINT", paramHint: "" },
  // Numeric — approximate
  { name: "FLOAT", paramHint: "" },
  { name: "FLOAT4", paramHint: "" },
  { name: "FLOAT8", paramHint: "" },
  { name: "DOUBLE", paramHint: "" },
  { name: "DOUBLE PRECISION", paramHint: "" },
  { name: "REAL", paramHint: "" },
  // String
  { name: "VARCHAR", paramHint: "(length)" },
  { name: "CHAR", paramHint: "(length)" },
  { name: "CHARACTER", paramHint: "(length)" },
  { name: "STRING", paramHint: "" },
  { name: "TEXT", paramHint: "" },
  // Binary
  { name: "BINARY", paramHint: "(length)" },
  { name: "VARBINARY", paramHint: "(length)" },
  // Logical
  { name: "BOOLEAN", paramHint: "" },
  // Date & Time
  { name: "DATE", paramHint: "" },
  { name: "DATETIME", paramHint: "" },
  { name: "TIME", paramHint: "(scale)" },
  { name: "TIMESTAMP", paramHint: "(scale)" },
  { name: "TIMESTAMP_LTZ", paramHint: "(scale)" },
  { name: "TIMESTAMP_NTZ", paramHint: "(scale)" },
  { name: "TIMESTAMP_TZ", paramHint: "(scale)" },
  // Semi-structured
  { name: "VARIANT", paramHint: "" },
  { name: "OBJECT", paramHint: "(name type, ...)" },
  { name: "ARRAY", paramHint: "(element_type)" },
  // Structured
  { name: "MAP", paramHint: "(key_type, value_type)" },
  // Geospatial
  { name: "GEOGRAPHY", paramHint: "" },
  { name: "GEOMETRY", paramHint: "" },
  // Vector
  { name: "VECTOR", paramHint: "(element_type, dimension)" },
];

/** Flat list of canonical type names (for normalizeDataType lookups). */
export const SF_TYPES = SF_DATA_TYPES.map((dt) => dt.name);

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

export interface JoinEntry {
  table: { schema: string; name: string };
  joinType: "INNER" | "LEFT" | "RIGHT" | "FULL OUTER";
  onCondition: string;
  isIntermediate: boolean;
}

export interface JoinQueryState {
  database: string;
  baseTable: { schema: string; name: string };
  joins: JoinEntry[];
  selectedColumns: Map<string, string[]>; // "SCHEMA.TABLE" → column names (empty = *)
}

export interface JoinPath {
  tables: { schema: string; name: string }[];
  edges: { from: { schema: string; table: string; col: string }; to: { schema: string; table: string; col: string } }[];
}

export const ER_NODE_WIDTH = 240;
export const ER_NODE_HEADER_HEIGHT = 32;
export const ER_NODE_ROW_HEIGHT = 24;
export const ER_NODE_PADDING = 8;
export const ER_COL_LIMIT = 30;
