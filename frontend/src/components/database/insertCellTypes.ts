// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

// Classification of a Snowflake column data type into an input "family" for the
// Insert Row form, together with parsed parameters (length / precision / scale /
// vector dimension) used to configure the widget and pre-validate the value.
//
// Rendering into SQL is done in Go (internal/table/insert.go) — this module is
// UX-only: it picks the widget and reports validation hints. The backend stays
// the single injection-safe source of truth and lets Snowflake report any type
// error a widget could not catch.

export type TypeFamily =
  | "numeric"
  | "text"
  | "boolean"
  | "date"
  | "time"
  | "timestamp" // TIMESTAMP / TIMESTAMP_NTZ / DATETIME (no zone)
  | "timestamptz" // TIMESTAMP_TZ / TIMESTAMP_LTZ (zone-aware)
  | "json" // VARIANT / OBJECT / ARRAY / MAP
  | "binary"
  | "geo"
  | "uuid"
  | "vector"
  | "other"; // FILE / user-defined / anything unmodelled

export interface ParsedColumnType {
  base: string; // leading type word, uppercased, e.g. "NUMBER"
  family: TypeFamily;
  length?: number; // VARCHAR(n) / CHAR(n) / BINARY(n)
  precision?: number; // NUMBER(p, s)
  scale?: number;
  dimension?: number; // VECTOR(elem, dim)
  elementType?: string; // VECTOR element type ("FLOAT" | "INT")
}

const NUMERIC = new Set([
  "NUMBER", "DECIMAL", "NUMERIC",
  "INT", "INTEGER", "BIGINT", "SMALLINT", "TINYINT", "BYTEINT",
  "FLOAT", "FLOAT4", "FLOAT8", "DOUBLE", "REAL",
]);
const TEXT = new Set(["VARCHAR", "CHAR", "CHARACTER", "STRING", "TEXT", "NCHAR", "NVARCHAR", "NVARCHAR2"]);
const BOOLEAN = new Set(["BOOLEAN", "BOOL"]);
const TIMESTAMP = new Set(["TIMESTAMP", "TIMESTAMP_NTZ", "DATETIME"]);
const TIMESTAMPTZ = new Set(["TIMESTAMP_TZ", "TIMESTAMP_LTZ"]);
const JSON_TYPES = new Set(["VARIANT", "OBJECT", "ARRAY", "MAP"]);
const BINARY = new Set(["BINARY", "VARBINARY"]);
const GEO = new Set(["GEOGRAPHY", "GEOMETRY"]);

// LENGTH_TYPES carry a single max-length parameter, PRECISION_TYPES a
// precision/scale pair — used only to surface the widget's numeric bounds/hints.
const LENGTH_TYPES = new Set(["VARCHAR", "CHAR", "CHARACTER", "STRING", "TEXT", "NCHAR", "NVARCHAR", "NVARCHAR2", "BINARY", "VARBINARY"]);
const PRECISION_TYPES = new Set(["NUMBER", "DECIMAL", "NUMERIC"]);

/** baseType extracts the leading, uppercased type word (mirrors Go's baseType). */
export function baseType(dataType: string): string {
  const s = (dataType ?? "").trim();
  const m = s.match(/^[^\s(]+/);
  return (m ? m[0] : s).toUpperCase();
}

function familyOf(base: string): TypeFamily {
  if (NUMERIC.has(base)) return "numeric";
  if (TEXT.has(base)) return "text";
  if (BOOLEAN.has(base)) return "boolean";
  if (base === "DATE") return "date";
  if (base === "TIME") return "time";
  if (TIMESTAMP.has(base)) return "timestamp";
  if (TIMESTAMPTZ.has(base)) return "timestamptz";
  if (JSON_TYPES.has(base)) return "json";
  if (BINARY.has(base)) return "binary";
  if (GEO.has(base)) return "geo";
  if (base === "UUID") return "uuid";
  if (base === "VECTOR") return "vector";
  return "other";
}

/**
 * parseColumnType classifies a Snowflake data-type string and extracts the
 * parameters relevant to widget selection and validation. Unparseable or
 * unparameterised types return just { base, family }.
 */
export function parseColumnType(dataType: string): ParsedColumnType {
  const base = baseType(dataType);
  const family = familyOf(base);
  const out: ParsedColumnType = { base, family };

  const paren = (dataType ?? "").match(/\(([^)]*)\)/);
  if (!paren) return out;
  const params = paren[1].split(",").map((s) => s.trim());

  if (base === "VECTOR") {
    // VECTOR(elem, dim)
    out.elementType = params[0]?.toUpperCase();
    const dim = Number(params[1]);
    if (Number.isFinite(dim)) out.dimension = dim;
    return out;
  }
  if (PRECISION_TYPES.has(base)) {
    const p = Number(params[0]);
    if (Number.isFinite(p)) out.precision = p;
    if (params[1] !== undefined) {
      const s = Number(params[1]);
      if (Number.isFinite(s)) out.scale = s;
    }
    return out;
  }
  if (LENGTH_TYPES.has(base)) {
    const n = Number(params[0]);
    if (Number.isFinite(n)) out.length = n;
    return out;
  }
  return out;
}

// ── Validation helpers ──────────────────────────────────────────────────────
// Each returns a human-readable error string when the value is invalid, or null
// when it passes (or when there is nothing to check). Empty values are always
// allowed here — the backend renders an empty value as NULL (or '') — so the
// form never blocks on a blank cell.

/** validateNumeric checks a numeric literal and, where declared, its scale. */
export function validateNumeric(value: string, t: ParsedColumnType): string | null {
  const v = value.trim();
  if (v === "") return null;
  if (!/^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$/.test(v)) return "Not a valid number";
  // Scale check: digits after the decimal point must not exceed the declared scale.
  if (t.scale != null && t.scale >= 0 && !/[eE]/.test(v)) {
    const dot = v.indexOf(".");
    const frac = dot === -1 ? 0 : v.length - dot - 1;
    if (frac > t.scale) return `Max ${t.scale} decimal place${t.scale === 1 ? "" : "s"}`;
  }
  return null;
}

/** validateJson checks that the value parses as JSON. */
export function validateJson(value: string): string | null {
  const v = value.trim();
  if (v === "") return null;
  try {
    JSON.parse(v);
    return null;
  } catch {
    return "Invalid JSON";
  }
}

/** validateUuid checks the canonical 8-4-4-4-12 hex form. */
export function validateUuid(value: string): string | null {
  const v = value.trim();
  if (v === "") return null;
  if (!/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/.test(v)) {
    return "Not a valid UUID";
  }
  return null;
}

/** validateHex checks an even-length string of hex digits (BINARY input). */
export function validateHex(value: string): string | null {
  const v = value.trim();
  if (v === "") return null;
  if (!/^[0-9a-fA-F]*$/.test(v)) return "Hex digits only (0-9, A-F)";
  if (v.length % 2 !== 0) return "Even number of hex digits";
  return null;
}

/**
 * validateVector checks a "[n, n, …]" list of numbers and, where the column
 * declares one, the exact element count against the vector dimension.
 */
export function validateVector(value: string, t: ParsedColumnType): string | null {
  const v = value.trim();
  if (v === "") return null;
  if (!v.startsWith("[") || !v.endsWith("]")) return "Expected [n, n, …]";
  const inner = v.slice(1, -1).trim();
  const parts = inner === "" ? [] : inner.split(",").map((s) => s.trim());
  for (const p of parts) {
    if (!/^[+-]?(\d+(\.\d*)?|\.\d+)([eE][+-]?\d+)?$/.test(p)) return "All elements must be numbers";
  }
  if (t.dimension != null && parts.length !== t.dimension) {
    return `Expected ${t.dimension} element${t.dimension === 1 ? "" : "s"}, got ${parts.length}`;
  }
  return null;
}

// ── Insert helpers (templates & formatting) ─────────────────────────────────
// Quick-insert snippets and a JSON pretty-printer for the fiddly complex types
// (semi-structured JSON and geospatial WKT/GeoJSON), surfaced as a small toolbar
// in the Value-mode widget. Snippets replace the cell value verbatim; they are
// plain strings the Go builder renders the same as hand-typed input.

export interface CellSnippet {
  label: string;
  value: string;
}

const OBJECT_SNIPPETS: CellSnippet[] = [
  { label: "Empty object  {}", value: "{}" },
  { label: "Sample object", value: '{\n  "id": 1,\n  "name": "example"\n}' },
];

const ARRAY_SNIPPETS: CellSnippet[] = [
  { label: "Empty array  []", value: "[]" },
  { label: "Number array", value: "[1, 2, 3]" },
  { label: "String array", value: '["a", "b", "c"]' },
];

// VARIANT can hold any JSON value, so it offers the widest set of scaffolds.
const VARIANT_SNIPPETS: CellSnippet[] = [
  { label: "Object  {}", value: '{\n  "key": "value"\n}' },
  { label: "Array  []", value: "[1, 2, 3]" },
  { label: "String", value: '"text"' },
  { label: "Number", value: "42" },
  { label: "Boolean", value: "true" },
  { label: "Null", value: "null" },
];

const GEO_SNIPPETS: CellSnippet[] = [
  { label: "Point (WKT)", value: "POINT(-122.35 37.55)" },
  { label: "LineString (WKT)", value: "LINESTRING(-122.35 37.55, -122.40 37.60)" },
  {
    label: "Polygon (WKT)",
    value: "POLYGON((-122.35 37.55, -122.40 37.55, -122.40 37.60, -122.35 37.60, -122.35 37.55))",
  },
  { label: "MultiPoint (WKT)", value: "MULTIPOINT((-122.35 37.55), (-122.40 37.60))" },
  { label: "Point (GeoJSON)", value: '{"type": "Point", "coordinates": [-122.35, 37.55]}' },
];

/**
 * snippetsFor returns the quick-insert templates for a column's type, or an
 * empty list for types that have none. `VARIANT`/`OBJECT`/`ARRAY` get JSON
 * scaffolds; geospatial columns get WKT/GeoJSON templates.
 */
export function snippetsFor(t: ParsedColumnType): CellSnippet[] {
  if (t.family === "geo") return GEO_SNIPPETS;
  if (t.family === "json") {
    switch (t.base) {
      case "OBJECT":
        return OBJECT_SNIPPETS;
      case "ARRAY":
        return ARRAY_SNIPPETS;
      default: // VARIANT / MAP
        return VARIANT_SNIPPETS;
    }
  }
  return [];
}

/**
 * formatJson pretty-prints a JSON value with two-space indentation, returning
 * null when the value is empty or does not parse (so the caller can leave it
 * untouched). Used by the "Format" action on semi-structured cells.
 */
export function formatJson(value: string): string | null {
  const v = value.trim();
  if (v === "") return null;
  try {
    return JSON.stringify(JSON.parse(v), null, 2);
  } catch {
    return null;
  }
}

/** validateValue dispatches to the family-specific validator. */
export function validateValue(value: string, t: ParsedColumnType): string | null {
  switch (t.family) {
    case "numeric":
      return validateNumeric(value, t);
    case "json":
      return validateJson(value);
    case "uuid":
      return validateUuid(value);
    case "binary":
      return validateHex(value);
    case "vector":
      return validateVector(value, t);
    default:
      return null;
  }
}
