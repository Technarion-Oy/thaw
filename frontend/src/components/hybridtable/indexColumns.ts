// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration
//
// Shared helpers for the hybrid-table index editors (create modal + properties
// dialog): the column-type eligibility rules Snowflake enforces for
// hybrid-table indexes.

/**
 * Reduce a Snowflake data-type string to its bare base type, upper-cased:
 * "VARCHAR(256)" → "VARCHAR", "NUMBER(38,0)" → "NUMBER",
 * "TIMESTAMP_TZ(9)" → "TIMESTAMP_TZ", "VECTOR(FLOAT, 256)" → "VECTOR".
 * The synonym TIMESTAMPTZ is normalized to TIMESTAMP_TZ.
 */
export function baseType(dataType: string): string {
  let t = (dataType || "").trim().toUpperCase();
  const paren = t.indexOf("(");
  if (paren >= 0) t = t.slice(0, paren).trim();
  if (t === "TIMESTAMPTZ") return "TIMESTAMP_TZ";
  return t;
}

// Semi-structured and geospatial types are forbidden for both key and INCLUDE
// columns; VECTOR and TIMESTAMP_TZ are additionally forbidden for key columns.
const SEMI_STRUCTURED = new Set(["VARIANT", "OBJECT", "ARRAY"]);
const GEOSPATIAL = new Set(["GEOGRAPHY", "GEOMETRY"]);

/**
 * Whether a column of the given Snowflake type may be a hybrid-table index key
 * column. Forbidden: semi-structured (VARIANT/OBJECT/ARRAY), geospatial
 * (GEOGRAPHY/GEOMETRY), VECTOR, and TIMESTAMP_TZ. (Bare TIMESTAMP defaults to
 * TIMESTAMP_NTZ, which is supported, so it is allowed.)
 */
export function isIndexableType(dataType: string): boolean {
  const t = baseType(dataType);
  if (SEMI_STRUCTURED.has(t) || GEOSPATIAL.has(t)) return false;
  if (t === "VECTOR") return false;
  if (t === "TIMESTAMP_TZ") return false;
  return true;
}

/**
 * Whether a column of the given Snowflake type may be a hybrid-table index
 * INCLUDE column. Forbidden: semi-structured (VARIANT/OBJECT/ARRAY) and
 * geospatial (GEOGRAPHY/GEOMETRY) columns only.
 */
export function isIncludableType(dataType: string): boolean {
  const t = baseType(dataType);
  return !SEMI_STRUCTURED.has(t) && !GEOSPATIAL.has(t);
}
