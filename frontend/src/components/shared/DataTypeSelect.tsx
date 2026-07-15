// SPDX-License-Identifier: GPL-3.0-or-later

import { Select, InputNumber } from "antd";

// Base types without parameters
const SIMPLE_TYPES = [
  "BOOLEAN",
  "DATE",
  "TIMESTAMP_NTZ",
  "TIMESTAMP_TZ",
  "TIMESTAMP_LTZ",
  "VARIANT",
  "OBJECT",
  "ARRAY",
  "FLOAT",
  "INTEGER",
  "GEOGRAPHY",
  "GEOMETRY",
];

// Types that accept a max length parameter: TYPE(n)
const LENGTH_TYPES = ["VARCHAR", "CHAR", "STRING", "TEXT", "BINARY"];

// Types that accept precision and scale: TYPE(p, s)
const PRECISION_TYPES = ["NUMBER", "DECIMAL", "NUMERIC"];

const ALL_BASE_TYPES = [...PRECISION_TYPES, ...LENGTH_TYPES, ...SIMPLE_TYPES];

const TYPE_OPTIONS = ALL_BASE_TYPES.map((t) => ({ value: t, label: t }));

/**
 * Parse a data type string like "VARCHAR(255)" or "NUMBER(10,2)" into
 * { base, length, precision, scale }.
 */
function parseDataType(dt: string): { base: string; length?: number; precision?: number; scale?: number } {
  const match = dt.match(/^([A-Z_]+)\((.+)\)$/i);
  if (!match) return { base: dt.toUpperCase() };
  const base = match[1].toUpperCase();
  const params = match[2];
  if (PRECISION_TYPES.includes(base)) {
    const parts = params.split(",").map((s) => s.trim());
    return {
      base,
      precision: parts[0] ? Number(parts[0]) : undefined,
      scale: parts[1] !== undefined ? Number(parts[1]) : undefined,
    };
  }
  if (LENGTH_TYPES.includes(base)) {
    return { base, length: Number(params) };
  }
  // Parameterised type whose params we don't model (e.g. TIMESTAMP_NTZ(9)):
  // strip the params so the base still matches a dropdown option.
  return { base };
}

/** Reconstruct the full type string from parts. */
function buildDataType(base: string, length?: number, precision?: number, scale?: number): string {
  if (PRECISION_TYPES.includes(base) && precision != null) {
    return scale != null ? `${base}(${precision},${scale})` : `${base}(${precision})`;
  }
  if (LENGTH_TYPES.includes(base) && length != null) {
    return `${base}(${length})`;
  }
  return base;
}

interface Props {
  value: string;
  onChange: (dataType: string) => void;
  size?: "small" | "middle" | "large";
  style?: React.CSSProperties;
}

export default function DataTypeSelect({ value, onChange, size = "small", style }: Props) {
  const parsed = parseDataType(value);
  const isLength = LENGTH_TYPES.includes(parsed.base);
  const isPrecision = PRECISION_TYPES.includes(parsed.base);

  const handleBaseChange = (newBase: string) => {
    if (PRECISION_TYPES.includes(newBase)) {
      onChange(buildDataType(newBase, undefined, 38, 0));
    } else if (LENGTH_TYPES.includes(newBase)) {
      onChange(newBase); // no length = Snowflake default (VARCHAR = 16777216)
    } else {
      onChange(newBase);
    }
  };

  return (
    <div style={style}>
      <Select
        showSearch
        size={size}
        value={parsed.base}
        onChange={handleBaseChange}
        options={TYPE_OPTIONS}
        style={{ width: "100%" }}
      />
      {isPrecision && (
        <div style={{ display: "flex", gap: 8, marginTop: 6, alignItems: "center" }}>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Precision</div>
            <InputNumber
              size={size}
              min={1}
              max={38}
              value={parsed.precision ?? 38}
              onChange={(v) => onChange(buildDataType(parsed.base, undefined, v ?? 38, parsed.scale ?? 0))}
              style={{ width: "100%" }}
            />
          </div>
          <div style={{ flex: 1 }}>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Scale</div>
            <InputNumber
              size={size}
              min={0}
              max={parsed.precision ?? 38}
              value={parsed.scale ?? 0}
              onChange={(v) => onChange(buildDataType(parsed.base, undefined, parsed.precision ?? 38, v ?? 0))}
              style={{ width: "100%" }}
            />
          </div>
        </div>
      )}
      {isLength && (
        <div style={{ marginTop: 6 }}>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Max length (empty = default)</div>
          <InputNumber
            size={size}
            min={1}
            max={16777216}
            value={parsed.length}
            placeholder="16777216"
            onChange={(v) => onChange(buildDataType(parsed.base, v ?? undefined))}
            style={{ width: "100%" }}
          />
        </div>
      )}
    </div>
  );
}
