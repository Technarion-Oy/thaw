// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import { useState, useEffect, useMemo, useCallback } from "react";
import { Button, Select, Tag, Collapse, Checkbox } from "antd";
import { CloseOutlined, CodeOutlined } from "@ant-design/icons";
import { BuildJoinSQL } from "../../../wailsjs/go/app/App";
import type { JoinQueryState, JoinEntry, JoinPath } from "./erTypes";

/** Canonical key for a table: "SCHEMA.TABLE" (trimmed, case-preserved). */
const tableKey = (schema: string, name: string) =>
  `${schema}.${name.trim()}`;

const JOIN_TYPES: JoinEntry["joinType"][] = ["INNER", "LEFT", "RIGHT", "FULL OUTER"];

const SQL_KEYWORDS = new Set([
  "SELECT", "FROM", "INNER", "LEFT", "RIGHT", "FULL", "OUTER", "JOIN",
  "ON", "AND", "OR", "AS", "WHERE", "ORDER", "BY", "GROUP", "HAVING",
]);

/** SQL keyword highlighting — basic tokenizer for the preview. */
function highlightSQL(sql: string): JSX.Element[] {

  return sql.split("\n").map((line, li) => {
    const tokens = line.split(/(\b\w+\b|[.,*()=])/g);
    return (
      <div key={li}>
        {tokens.map((tok, ti) => {
          if (SQL_KEYWORDS.has(tok.toUpperCase())) {
            return (
              <span key={ti} style={{ color: "var(--accent)", fontWeight: 600 }}>
                {tok}
              </span>
            );
          }
          return <span key={ti}>{tok}</span>;
        })}
      </div>
    );
  });
}

/** Format a join path as a readable chain for disambiguation.
 *  Guards against malformed paths where edges.length >= tables.length. */
export function formatPathLabel(path: JoinPath): string {
  if (path.tables.length === 0) return "";
  const parts: string[] = [
    `${path.tables[0].schema}.${path.tables[0].name}`,
  ];
  for (let i = 0; i < path.edges.length; i++) {
    const edge = path.edges[i];
    const nextTable = path.tables[i + 1];
    if (!nextTable) break;
    parts.push(`--(${edge.from.col} = ${edge.to.col})-->`);
    parts.push(`${nextTable.schema}.${nextTable.name}`);
  }
  return parts.join(" ");
}

// ── Disambiguation panel ─────────────────────────────────────────────────────

interface DisambiguationProps {
  paths: JoinPath[];
  onSelect: (index: number) => void;
  onCancel: () => void;
}

export function JoinPathDisambiguation({ paths, onSelect, onCancel }: DisambiguationProps) {
  return (
    <div
      style={{
        padding: 16,
        borderTop: "1px solid var(--border)",
        background: "var(--bg-elevated)",
      }}
    >
      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
        <span style={{ fontWeight: 600, fontSize: 13 }}>Multiple join paths found</span>
        <Button size="small" type="text" icon={<CloseOutlined />} onClick={onCancel} />
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
        {paths.map((path, i) => (
          <div
            key={i}
            role="button"
            tabIndex={0}
            className="er-join-path-option"
            onClick={() => onSelect(i)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onSelect(i);
              }
            }}
            style={{
              padding: "8px 12px",
              borderRadius: 6,
              border: "1px solid var(--border)",
              cursor: "pointer",
              fontFamily: "monospace",
              fontSize: 11,
              lineHeight: 1.6,
              wordBreak: "break-all",
            }}
          >
            {formatPathLabel(path)}
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Main join query panel ────────────────────────────────────────────────────

interface JoinQueryPanelProps {
  state: JoinQueryState;
  /** All columns for each table, keyed by "SCHEMA.TABLE". */
  tableColumns: Map<string, string[]>;
  onChange: (state: JoinQueryState) => void;
  onOpenInEditor: (sql: string) => void;
  onClose: () => void;
}

export default function JoinQueryPanel({
  state,
  tableColumns,
  onChange,
  onOpenInEditor,
  onClose,
}: JoinQueryPanelProps) {
  const [sql, setSql] = useState("");
  useEffect(() => {
    BuildJoinSQL(state as never).then(setSql);
  }, [state]);

  const updateJoinType = useCallback(
    (index: number, joinType: JoinEntry["joinType"]) => {
      const newJoins = [...state.joins];
      newJoins[index] = { ...newJoins[index], joinType };
      onChange({ ...state, joins: newJoins });
    },
    [state, onChange],
  );

  const updateSelectedColumns = useCallback(
    (tblKey: string, cols: string[]) => {
      const updated = { ...state.selectedColumns };
      if (cols.length === 0) {
        delete updated[tblKey];
      } else {
        updated[tblKey] = cols;
      }
      onChange({ ...state, selectedColumns: updated });
    },
    [state, onChange],
  );

  const baseKey = tableKey(state.baseTable.schema, state.baseTable.name);

  // Build collapse items for column selection
  const collapseItems = useMemo(() => [
    {
      key: baseKey,
      label: (
        <span style={{ fontFamily: "monospace", fontSize: 12 }}>
          {state.baseTable.schema}.{state.baseTable.name}
          <Tag color="blue" style={{ marginLeft: 8, fontSize: 10 }}>BASE</Tag>
        </span>
      ),
      children: (
        <ColumnPicker
          columns={tableColumns.get(baseKey) ?? []}
          selected={state.selectedColumns[baseKey] ?? []}
          onChange={(cols) => updateSelectedColumns(baseKey, cols)}
        />
      ),
    },
    ...state.joins.map((j) => {
      const jKey = tableKey(j.table.schema, j.table.name);
      return {
        key: jKey,
        label: (
          <span style={{ fontFamily: "monospace", fontSize: 12 }}>
            {j.table.schema}.{j.table.name}
            {j.isIntermediate && (
              <Tag style={{ marginLeft: 8, fontSize: 10, borderStyle: "dashed" }}>intermediate</Tag>
            )}
          </span>
        ),
        children: (
          <ColumnPicker
            columns={tableColumns.get(jKey) ?? []}
            selected={state.selectedColumns[jKey] ?? []}
            onChange={(cols) => updateSelectedColumns(jKey, cols)}
          />
        ),
      };
    }),
  ], [state, tableColumns, baseKey, updateSelectedColumns]);

  return (
    <div
      style={{
        borderTop: "1px solid var(--border)",
        background: "var(--bg-elevated)",
        display: "flex",
        flexDirection: "column",
        height: "100%",
        overflow: "hidden",
      }}
    >
      {/* Header */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "8px 16px",
          borderBottom: "1px solid var(--border)",
          flexShrink: 0,
        }}
      >
        <span style={{ fontWeight: 600, fontSize: 13 }}>Join Query Builder</span>
        <div style={{ display: "flex", gap: 4 }}>
          <Button
            size="small"
            icon={<CodeOutlined />}
            onClick={() => onOpenInEditor(sql)}
          >
            Open in Editor
          </Button>
          <Button size="small" type="text" icon={<CloseOutlined />} onClick={onClose} />
        </div>
      </div>

      {/* Body — split left/right */}
      <div style={{ display: "flex", flex: 1, overflow: "hidden" }}>
        {/* Left: join configuration */}
        <div
          style={{
            flex: 1,
            overflowY: "auto",
            padding: 12,
            borderRight: "1px solid var(--border)",
          }}
        >
          {/* Join rows */}
          <div style={{ marginBottom: 12 }}>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 8 }}>
              Join Configuration
            </div>

            {/* Base table */}
            <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}>
              <Tag color="blue" style={{ fontFamily: "monospace", fontSize: 11, margin: 0 }}>
                {state.baseTable.schema}.{state.baseTable.name}
              </Tag>
              <span style={{ fontSize: 11, color: "var(--text-muted)" }}>base table</span>
            </div>

            {/* Join entries */}
            {state.joins.map((j, i) => (
              <div
                key={i}
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 8,
                  marginBottom: 6,
                  flexWrap: "wrap",
                }}
              >
                <Select
                  size="small"
                  value={j.joinType}
                  onChange={(val) => updateJoinType(i, val)}
                  options={JOIN_TYPES.map((t) => ({ label: t, value: t }))}
                  style={{ width: 120 }}
                />
                <Tag
                  style={{
                    fontFamily: "monospace",
                    fontSize: 11,
                    margin: 0,
                    borderStyle: j.isIntermediate ? "dashed" : "solid",
                  }}
                >
                  {j.table.schema}.{j.table.name}
                </Tag>
                <span
                  style={{
                    fontSize: 10,
                    color: "var(--text-muted)",
                    fontFamily: "monospace",
                    flex: 1,
                    minWidth: 0,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                  title={j.onCondition}
                >
                  ON {j.onCondition}
                </span>
              </div>
            ))}
          </div>

          {/* Column selection */}
          <div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 8 }}>
              Column Selection (optional)
            </div>
            <Collapse
              size="small"
              items={collapseItems}
              bordered={false}
              style={{ background: "transparent" }}
            />
          </div>
        </div>

        {/* Right: SQL preview */}
        <div
          style={{
            flex: 1,
            overflowY: "auto",
            padding: 12,
            background: "var(--bg)",
          }}
        >
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 8 }}>
            SQL Preview
          </div>
          <pre
            style={{
              fontFamily: "monospace",
              fontSize: 12,
              lineHeight: 1.6,
              margin: 0,
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {highlightSQL(sql)}
          </pre>
        </div>
      </div>
    </div>
  );
}

// ── Column picker ────────────────────────────────────────────────────────────

function ColumnPicker({
  columns,
  selected,
  onChange,
}: {
  columns: string[];
  selected: string[];
  onChange: (cols: string[]) => void;
}) {
  const selectedSet = new Set(selected);
  const allSelected = columns.length > 0 && columns.every((c) => selectedSet.has(c));

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <Checkbox
        checked={selected.length === 0}
        onChange={() => onChange([])}
        style={{ fontSize: 11 }}
      >
        <span style={{ color: "var(--text-muted)" }}>All columns (*)</span>
      </Checkbox>
      {columns.length > 0 && (
        <Checkbox
          checked={allSelected}
          onChange={() => onChange(allSelected ? [] : [...columns])}
          style={{ fontSize: 11 }}
        >
          <span style={{ color: "var(--text-muted)" }}>Select all</span>
        </Checkbox>
      )}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 4, marginTop: 4 }}>
        {columns.map((col) => (
          <Tag
            key={col}
            style={{
              cursor: "pointer",
              fontFamily: "monospace",
              fontSize: 10,
              borderColor: selectedSet.has(col) ? "var(--accent)" : undefined,
              color: selectedSet.has(col) ? "var(--accent)" : undefined,
            }}
            onClick={() => {
              if (selectedSet.has(col)) {
                onChange(selected.filter((c) => c !== col));
              } else {
                onChange([...selected, col]);
              }
            }}
          >
            {col}
          </Tag>
        ))}
      </div>
    </div>
  );
}
