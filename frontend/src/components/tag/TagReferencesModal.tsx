// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback, useMemo } from "react";
import { Modal, Button, Space, Typography, Alert, Table, Empty, Spin } from "antd";
import { TagsOutlined, ReloadOutlined } from "@ant-design/icons";
import { GetObjectTagReferences, GetColumnTagReferences } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "18px 0 8px",
};

// Object kinds that carry columns — for these we also load the per-column tags
// via TAG_REFERENCES_ALL_COLUMNS. The strings match the object-browser `kind`.
const COLUMN_BEARING_KINDS = new Set([
  "TABLE", "VIEW", "MATERIALIZED VIEW", "DYNAMIC TABLE", "EXTERNAL TABLE",
  "ICEBERG TABLE", "HYBRID TABLE", "EVENT TABLE",
]);

// Renders a QueryResult as an Ant Design table keyed by column index, matching
// the lightweight tabular rendering used elsewhere for tag references.
function ResultTable({ result }: { result: snowflake.QueryResult }) {
  if (!result.rows || result.rows.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No tags found" />;
  }
  return (
    <Table
      size="small"
      rowKey={(_r, i) => String(i)}
      pagination={result.rows.length > 12 ? { pageSize: 12, size: "small" } : false}
      columns={(result.columns ?? []).map((c, ci) => ({
        title: c,
        dataIndex: ci,
        key: String(ci),
        ellipsis: true,
        render: (v: unknown) => (v === null || v === undefined ? "" : String(v)),
      }))}
      dataSource={result.rows.map((row) => {
        const obj: Record<number, unknown> = {};
        row.forEach((cell, ci) => { obj[ci] = cell; });
        return obj;
      })}
    />
  );
}

interface Props {
  db: string;
  schema: string;
  name: string;
  kind: string;
  // Comma-separated parameter type list for procedures / functions (resolves the
  // overload); empty for every other object kind.
  args?: string;
  // When set, the modal shows the tags on this single column of the object,
  // filtered from TAG_REFERENCES_ALL_COLUMNS.
  column?: string;
  onClose: () => void;
}

// Keeps only the rows of an all-columns result that belong to the given column.
// The comparison is case-insensitive so a node key whose case differs from the
// stored column name (e.g. a quoted mixed-case column) still matches.
function filterToColumn(result: snowflake.QueryResult, column: string): snowflake.QueryResult {
  const idx = (result.columns ?? []).findIndex((c) => c.toUpperCase() === "COLUMN_NAME");
  if (idx < 0) return result;
  const want = column.toLowerCase();
  const rows = (result.rows ?? []).filter((r) => String(r[idx] ?? "").toLowerCase() === want);
  return { ...result, rows } as snowflake.QueryResult;
}

export default function TagReferencesModal({ db, schema, name, kind, args, column, onClose }: Props) {
  const columnMode = !!column;
  const hasColumns = !columnMode && COLUMN_BEARING_KINDS.has(kind.toUpperCase());

  const [objectRefs, setObjectRefs] = useState<snowflake.QueryResult | null>(null);
  const [columnRefs, setColumnRefs] = useState<snowflake.QueryResult | null>(null);
  const [objectErr, setObjectErr] = useState<string | null>(null);
  const [columnErr, setColumnErr] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    setObjectErr(null);
    setColumnErr(null);
    setObjectRefs(null);
    setColumnRefs(null);
    const tasks: Promise<void>[] = [];
    // Object-level tags (skipped in single-column mode).
    if (!columnMode) {
      tasks.push(
        GetObjectTagReferences(kind, db, schema, name, args ?? "")
          .then((r) => setObjectRefs(r))
          .catch((e) => setObjectErr(String(e))),
      );
    }
    // Column-level tags — all columns for a table/view, or just the one in
    // single-column mode.
    if (columnMode || hasColumns) {
      tasks.push(
        GetColumnTagReferences(kind, db, schema, name)
          .then((r) => setColumnRefs(column ? filterToColumn(r, column) : r))
          .catch((e) => setColumnErr(String(e))),
      );
    }
    await Promise.all(tasks);
    setLoading(false);
  }, [kind, db, schema, name, args, column, columnMode, hasColumns]);

  useEffect(() => { load(); }, [load]);

  const objectRef = useMemo(() => {
    const k = kind.toUpperCase();
    // Container kinds carry fewer name parts than a regular object.
    if (k === "DATABASE") return `"${db}"`;
    if (k === "SCHEMA") return `"${db}"."${schema}"`;
    const base = `"${db}"."${schema}"."${name}"`;
    return column ? `${base}."${column}"` : base;
  }, [db, schema, name, kind, column]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <TagsOutlined style={{ color: "var(--link)" }} />
          <span>Tag References</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {columnMode ? "COLUMN" : kind} · {objectRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>Refresh</Button>
          <Button onClick={onClose}>Close</Button>
        </Space>
      }
      width={780}
      styles={{ body: { maxHeight: "74vh", overflowY: "auto", paddingTop: 8 } }}
    >
      <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
        {columnMode
          ? <>Tags applied to this column, from the no-latency <code>INFORMATION_SCHEMA.TAG_REFERENCES_ALL_COLUMNS</code> table function — changes appear immediately.</>
          : <>Tags applied to this object, from the no-latency <code>INFORMATION_SCHEMA.TAG_REFERENCES</code>{hasColumns && <> / <code>TAG_REFERENCES_ALL_COLUMNS</code></>} table function — changes appear immediately.</>}
      </Text>

      {!columnMode && (
        <>
          <div style={SECTION_HEAD}>Object tags</div>
          {objectErr && (
            <Alert type="warning" message="Could not load object tags" description={objectErr} showIcon style={{ marginBottom: 8 }} />
          )}
          {!objectRefs && !objectErr ? (
            <div style={{ textAlign: "center", padding: 20 }}><Spin /></div>
          ) : objectRefs ? (
            <ResultTable result={objectRefs} />
          ) : null}
        </>
      )}

      {(columnMode || hasColumns) && (
        <>
          <div style={SECTION_HEAD}>{columnMode ? "Column tags" : "Column tags (all columns)"}</div>
          {columnErr && (
            <Alert type="warning" message="Could not load column tags" description={columnErr} showIcon style={{ marginBottom: 8 }} />
          )}
          {!columnRefs && !columnErr ? (
            <div style={{ textAlign: "center", padding: 20 }}><Spin /></div>
          ) : columnRefs ? (
            <ResultTable result={columnRefs} />
          ) : null}
        </>
      )}
    </Modal>
  );
}
