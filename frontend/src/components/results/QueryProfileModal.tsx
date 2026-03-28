// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useCallback, useRef } from "react";
import { Modal, Table, Tag, Button, Spin, Alert, Tooltip, Space, Typography } from "antd";
import { ReloadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { ExecuteQuery } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

interface Props {
  queryId: string;
  onClose: () => void;
  /** When true, auto-refreshes every 3 s (used while a query is still running). */
  liveRefresh?: boolean;
}

type Row = Record<string, unknown>;

const OPERATOR_TYPE_COLORS: Record<string, string> = {
  RESULT: "blue",
  TABLESCAN: "green",
  INMEMTABLESCAN: "green",
  FILTER: "orange",
  AGGREGATE: "purple",
  SORT: "cyan",
  JOIN: "magenta",
  MERGEJOIN: "magenta",
  HASHJOIN: "magenta",
  NESTEDLOOP: "magenta",
  WITHREFERENCE: "geekblue",
  EXTERNALFUNCTION: "volcano",
  UNION: "gold",
  LIMIT: "lime",
};

const JSON_COLS = new Set(["OPERATOR_STATISTICS", "EXECUTION_TIME_BREAKDOWN", "OPERATOR_ATTRIBUTES"]);
const HIDDEN_COLS = new Set(["QUERY_ID"]);

function operatorTypeColor(type: string): string {
  const key = (type ?? "").toUpperCase().replace(/[\s_]/g, "");
  return OPERATOR_TYPE_COLORS[key] ?? "default";
}

function prettyJson(value: unknown): string {
  if (value == null) return "";
  if (typeof value === "string") {
    try { return JSON.stringify(JSON.parse(value), null, 2); }
    catch { return value; }
  }
  return JSON.stringify(value, null, 2);
}

function renderCell(col: string, value: unknown): React.ReactNode {
  if (value == null || value === "") return <span style={{ color: "var(--text-faint)" }}>—</span>;

  if (col === "OPERATOR_TYPE") {
    const s = String(value);
    return (
      <Tag
        color={operatorTypeColor(s)}
        style={{ fontFamily: "monospace", fontSize: 10, margin: 0, lineHeight: "18px" }}
      >
        {s}
      </Tag>
    );
  }

  if (JSON_COLS.has(col)) {
    const text = prettyJson(value);
    if (!text) return <span style={{ color: "var(--text-faint)" }}>—</span>;
    return (
      <pre
        style={{
          fontFamily: "monospace",
          fontSize: 10,
          margin: 0,
          maxHeight: 130,
          overflow: "auto",
          whiteSpace: "pre-wrap",
          wordBreak: "break-word",
          background: "var(--bg-subtle, rgba(0,0,0,0.03))",
          padding: "2px 5px",
          borderRadius: 3,
          lineHeight: 1.45,
        }}
      >
        {text}
      </pre>
    );
  }

  if (col === "PARENT_OPERATORS") {
    let arr: unknown = value;
    if (typeof value === "string") {
      try { arr = JSON.parse(value); } catch { /* leave */ }
    }
    if (Array.isArray(arr)) return <span>{arr.join(", ") || "—"}</span>;
    return <span>{String(value)}</span>;
  }

  const mono = col === "STEP_ID" || col === "OPERATOR_ID";
  return (
    <span style={{ fontFamily: mono ? "monospace" : undefined, fontSize: mono ? 11 : undefined }}>
      {String(value)}
    </span>
  );
}

export default function QueryProfileModal({ queryId, onClose, liveRefresh }: Props) {
  const [loading,       setLoading]       = useState(false);
  const [columns,       setColumns]       = useState<string[]>([]);
  const [rows,          setRows]          = useState<Row[]>([]);
  const [error,         setError]         = useState<string | null>(null);
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const fetchStats = useCallback(async () => {
    setLoading(true);
    try {
      const result = await ExecuteQuery(
        `SELECT * FROM TABLE(GET_QUERY_OPERATOR_STATS('${queryId}'))`
      );
      const cols = result.columns ?? [];
      setColumns(cols);
      setRows(
        (result.rows ?? []).map((row) =>
          Object.fromEntries(cols.map((col, i) => [col, (row as unknown[])[i]]))
        )
      );
      setError(null);
      setLastRefreshed(new Date());
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [queryId]);

  useEffect(() => {
    fetchStats();
    if (liveRefresh) {
      timerRef.current = setInterval(fetchStats, 3000);
    }
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, [fetchStats, liveRefresh]);

  const tableCols: ColumnsType<Row> = columns
    .filter((c) => !HIDDEN_COLS.has(c))
    .map((col) => {
      const colDef: ColumnsType<Row>[number] = {
        key: col,
        title: (
          <span style={{ fontSize: 11 }}>
            {col.toLowerCase().replace(/_/g, "\u00A0")}
          </span>
        ),
        dataIndex: col,
        render: (v: unknown) => renderCell(col, v),
      };
      if      (col === "STEP_ID" || col === "OPERATOR_ID") colDef.width = 65;
      else if (col === "PARENT_OPERATORS")                 colDef.width = 110;
      else if (col === "OPERATOR_TYPE")                    colDef.width = 165;
      else if (col === "OPERATOR_STATISTICS")              colDef.width = 290;
      else if (col === "EXECUTION_TIME_BREAKDOWN")         colDef.width = 260;
      else if (col === "OPERATOR_ATTRIBUTES")              colDef.width = 290;
      return colDef;
    });

  const shortQid =
    queryId.length > 36 ? `${queryId.slice(0, 16)}…${queryId.slice(-16)}` : queryId;

  return (
    <Modal
      open
      title={
        <Space size={8}>
          <span>Query Profile</span>
          <Text
            style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}
            title={queryId}
          >
            {shortQid}
          </Text>
          {liveRefresh && (
            <Tag
              color="green"
              style={{ fontSize: 10, padding: "0 5px", lineHeight: "18px", marginInlineEnd: 0 }}
            >
              ● Live
            </Tag>
          )}
        </Space>
      }
      onCancel={onClose}
      width="min(1200px, 95vw)"
      footer={null}
      styles={{ body: { paddingTop: 12 } }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
        {lastRefreshed && (
          <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
            Updated {lastRefreshed.toLocaleTimeString()}
          </Text>
        )}
        <Tooltip title="Refresh">
          <Button
            size="small"
            type="text"
            icon={<ReloadOutlined style={{ fontSize: 11 }} />}
            loading={loading && rows.length > 0}
            onClick={fetchStats}
            style={{ height: 22, padding: "0 4px" }}
          />
        </Tooltip>
      </div>

      {error && (
        <Alert
          type="error"
          message={error}
          style={{ marginBottom: 8 }}
          closable
          onClose={() => setError(null)}
        />
      )}

      {loading && rows.length === 0 ? (
        <div style={{ textAlign: "center", padding: 40 }}>
          <Spin />
        </div>
      ) : !error && rows.length === 0 ? (
        <Alert
          type="info"
          message="No profiling data available yet"
          description="Operator statistics are populated once the query has started executing operators. If the query just started, wait a moment and click Refresh."
          style={{ marginBottom: 8 }}
        />
      ) : (
        <Table<Row>
          dataSource={rows}
          columns={tableCols}
          rowKey={(_, idx) => String(idx)}
          size="small"
          pagination={false}
          scroll={{ x: "max-content", y: 500 }}
        />
      )}
    </Modal>
  );
}
