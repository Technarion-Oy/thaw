// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import React, { useState, useCallback } from "react";
import { Handle, Position, type NodeProps } from "@xyflow/react";
import { ER_NODE_WIDTH, ER_COL_LIMIT, type DesignerColumn, normalizeIdentifier } from "./erTypes";
import type { ERTableNodeData } from "./erCanvasLayout";

const SHORT_TYPES: Record<string, string> = {
  // Numeric — exact
  NUMBER: "NUM",
  DECIMAL: "DEC",
  NUMERIC: "NUM",
  INT: "INT",
  INTEGER: "INT",
  BIGINT: "BINT",
  SMALLINT: "SINT",
  TINYINT: "TINT",
  BYTEINT: "BYTE",
  // Numeric — approximate
  FLOAT: "FLT",
  FLOAT4: "FLT4",
  FLOAT8: "FLT8",
  DOUBLE: "DBL",
  "DOUBLE PRECISION": "DBL",
  REAL: "REAL",
  // String
  VARCHAR: "VC",
  CHAR: "CHR",
  CHARACTER: "CHR",
  STRING: "STR",
  TEXT: "TXT",
  // Binary
  BINARY: "BIN",
  VARBINARY: "VBIN",
  // Logical
  BOOLEAN: "BOOL",
  // Date & Time
  DATE: "DATE",
  DATETIME: "DTTM",
  TIME: "TIME",
  TIMESTAMP: "TS",
  TIMESTAMP_LTZ: "TSZ",
  TIMESTAMP_NTZ: "TS",
  TIMESTAMP_TZ: "TSTZ",
  // Semi-structured
  VARIANT: "VAR",
  OBJECT: "OBJ",
  ARRAY: "ARR",
  MAP: "MAP",
  // Geospatial
  GEOGRAPHY: "GEO",
  GEOMETRY: "GEOM",
  // Vector
  VECTOR: "VEC",
};

function abbreviateType(dt: string): string {
  // Extract base type name before any parenthesized parameters
  const base = dt.replace(/\s*\(.*$/, "").trim();
  return SHORT_TYPES[base] ?? base.slice(0, 4);
}

function ERTableNodeInner({ data, selected }: NodeProps) {
  const { table, mode, onTableRename, onColumnRename } =
    data as ERTableNodeData;

  const [editingHeader, setEditingHeader] = useState(false);
  const [headerValue, setHeaderValue] = useState(table.name);
  const [editingColId, setEditingColId] = useState<string | null>(null);
  const [colValue, setColValue] = useState("");

  const handleHeaderDoubleClick = useCallback(() => {
    if (mode !== "edit" || !onTableRename) return;
    setHeaderValue(table.name);
    setEditingHeader(true);
  }, [mode, onTableRename, table.name]);

  const commitHeader = useCallback(() => {
    setEditingHeader(false);
    const normalized = normalizeIdentifier(headerValue);
    if (normalized && normalized !== table.name) {
      onTableRename?.(table.id, normalized);
    }
  }, [headerValue, table.name, table.id, onTableRename]);

  const handleColDoubleClick = useCallback(
    (col: DesignerColumn) => {
      if (mode !== "edit" || !onColumnRename) return;
      setColValue(col.name);
      setEditingColId(col.id);
    },
    [mode, onColumnRename],
  );

  const commitCol = useCallback(() => {
    const normalized = normalizeIdentifier(colValue);
    if (editingColId && normalized) {
      onColumnRename?.(table.id, editingColId, normalized);
    }
    setEditingColId(null);
  }, [editingColId, colValue, table.id, onColumnRename]);

  const displayCols = table.columns.slice(0, ER_COL_LIMIT);
  const overflowCount = table.columns.length - ER_COL_LIMIT;

  return (
    <div
      style={{
        width: ER_NODE_WIDTH,
        background: selected ? "color-mix(in srgb, var(--accent) 8%, var(--bg-elevated))" : "var(--bg-elevated)",
        border: `1.5px solid ${selected ? "var(--accent)" : "var(--border)"}`,
        borderRadius: 6,
        overflow: "hidden",
        fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
        fontSize: 11,
      }}
    >
      {/* Header */}
      <div
        onDoubleClick={handleHeaderDoubleClick}
        style={{
          padding: "6px 10px",
          background: "var(--bg-overlay)",
          borderBottom: "1px solid var(--border)",
          fontWeight: 600,
          fontSize: 12,
          cursor: mode === "edit" ? "text" : "default",
          whiteSpace: "nowrap",
          overflow: "hidden",
          textOverflow: "ellipsis",
        }}
      >
        {editingHeader ? (
          <input
            autoFocus
            value={headerValue}
            onChange={(e) => setHeaderValue(e.target.value)}
            onBlur={commitHeader}
            onKeyDown={(e) => {
              if (e.key === "Enter") commitHeader();
              if (e.key === "Escape") setEditingHeader(false);
            }}
            style={{
              width: "100%",
              background: "transparent",
              border: "none",
              outline: "none",
              color: "inherit",
              fontFamily: "inherit",
              fontSize: "inherit",
              fontWeight: "inherit",
              padding: 0,
            }}
          />
        ) : (
          <span title={`${table.schema}.${table.name}`}>
            <span style={{ color: "var(--text-muted)", fontWeight: 400 }}>{table.schema}.</span>
            {table.name || "(unnamed)"}
          </span>
        )}
      </div>

      {/* Column rows */}
      <div style={{ padding: "2px 0" }}>
        {displayCols.map((col) => (
          <div
            key={col.id}
            onDoubleClick={() => handleColDoubleClick(col)}
            style={{
              display: "flex",
              alignItems: "center",
              gap: 4,
              padding: "2px 10px",
              height: 24,
              position: "relative",
              cursor: mode === "edit" ? "text" : "default",
            }}
          >
            {/* Target handle (left) */}
            <Handle
              type="target"
              position={Position.Left}
              id={`col-target-${col.id}`}
              style={{
                width: 8,
                height: 8,
                background: "var(--accent)",
                border: "1.5px solid var(--bg-elevated)",
                opacity: mode === "edit" ? 0.6 : 0,
                left: -4,
              }}
            />

            {/* Column content */}
            {editingColId === col.id ? (
              <input
                autoFocus
                value={colValue}
                onChange={(e) => setColValue(e.target.value)}
                onBlur={commitCol}
                onKeyDown={(e) => {
                  if (e.key === "Enter") commitCol();
                  if (e.key === "Escape") setEditingColId(null);
                }}
                style={{
                  flex: 1,
                  background: "transparent",
                  border: "none",
                  outline: "none",
                  color: "inherit",
                  fontFamily: "inherit",
                  fontSize: "inherit",
                  padding: 0,
                }}
              />
            ) : (
              <>
                <span
                  style={{
                    flex: 1,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {col.name || "(unnamed)"}
                </span>
                <span style={{ color: "var(--text-muted)", fontSize: 10, flexShrink: 0 }}>
                  {abbreviateType(col.dataType)}
                </span>
                {col.isPK && (
                  <span
                    style={{
                      background: "var(--accent)",
                      color: "#fff",
                      borderRadius: 3,
                      padding: "0 3px",
                      fontSize: 9,
                      fontWeight: 600,
                      flexShrink: 0,
                    }}
                  >
                    PK
                  </span>
                )}
                {col.notNull && !col.isPK && (
                  <span
                    style={{
                      background: "color-mix(in srgb, var(--text-muted) 30%, transparent)",
                      borderRadius: 3,
                      padding: "0 3px",
                      fontSize: 9,
                      flexShrink: 0,
                    }}
                  >
                    NN
                  </span>
                )}
                {col.fkRef && (
                  <span
                    style={{
                      color: "var(--accent)",
                      fontSize: 10,
                      flexShrink: 0,
                    }}
                    title={`FK → ${col.fkRef}`}
                  >
                    FK
                  </span>
                )}
              </>
            )}

            {/* Source handle (right) */}
            <Handle
              type="source"
              position={Position.Right}
              id={`col-source-${col.id}`}
              style={{
                width: 8,
                height: 8,
                background: "var(--accent)",
                border: "1.5px solid var(--bg-elevated)",
                opacity: mode === "edit" ? 0.6 : 0,
                right: -4,
              }}
            />
          </div>
        ))}

        {overflowCount > 0 && (
          <div
            style={{
              padding: "2px 10px",
              height: 24,
              color: "var(--text-muted)",
              fontSize: 10,
              display: "flex",
              alignItems: "center",
            }}
          >
            +{overflowCount} more column{overflowCount > 1 ? "s" : ""}
          </div>
        )}
      </div>
    </div>
  );
}

const ERTableNode = React.memo(ERTableNodeInner);
export default ERTableNode;
