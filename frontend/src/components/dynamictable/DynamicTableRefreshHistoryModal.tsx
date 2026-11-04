// SPDX-License-Identifier: GPL-3.0-or-later

// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useRef, useCallback } from "react";
import { Modal, Table, Tag, Space, Typography, Spin, Tooltip, Button, Segmented } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  MinusCircleOutlined,
  ClockCircleOutlined,
  HistoryOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { GetDynamicTableRefreshHistory } from "../../../wailsjs/go/app/App";
import type { dynamictable } from "../../../wailsjs/go/models";
import { formatTime, formatDuration } from "../../utils/formatTime";

const { Text } = Typography;

type Row = dynamictable.RefreshHistoryRow;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

const FAILED_STATES = new Set(["FAILED", "UPSTREAM_FAILED"]);

function stateTag(state: string) {
  switch (state?.toUpperCase()) {
    case "SUCCEEDED":       return <Tag icon={<CheckCircleOutlined />} color="success">SUCCEEDED</Tag>;
    case "FAILED":          return <Tag icon={<CloseCircleOutlined />} color="error">FAILED</Tag>;
    case "UPSTREAM_FAILED": return <Tag icon={<CloseCircleOutlined />} color="volcano">UPSTREAM_FAILED</Tag>;
    case "EXECUTING":       return <Tag icon={<SyncOutlined spin />} color="processing">EXECUTING</Tag>;
    case "SCHEDULED":       return <Tag icon={<ClockCircleOutlined />} color="blue">SCHEDULED</Tag>;
    case "SKIPPED":         return <Tag icon={<MinusCircleOutlined />} color="gold">SKIPPED</Tag>;
    case "CANCELLED":       return <Tag icon={<MinusCircleOutlined />} color="default">CANCELLED</Tag>;
    default:                return state ? <Tag>{state}</Tag> : <Text type="secondary">—</Text>;
  }
}

const small = (v: string) => <Text style={{ fontSize: 12 }}>{v || "—"}</Text>;
const time = (ts: string) => <Text type="secondary" style={{ fontSize: 12 }}>{formatTime(ts)}</Text>;

const columns: ColumnsType<Row> = [
  {
    title: "State",
    dataIndex: "state",
    key: "state",
    width: 160,
    render: (state: string, r) =>
      r.stateMessage && FAILED_STATES.has(state?.toUpperCase()) ? (
        <Tooltip
          title={<pre style={{ margin: 0, fontSize: 11, whiteSpace: "pre-wrap" }}>{r.stateCode ? `[${r.stateCode}] ` : ""}{r.stateMessage}</pre>}
          overlayStyle={{ maxWidth: 460 }}
        >
          <span>{stateTag(state)}</span>
        </Tooltip>
      ) : stateTag(state),
  },
  { title: "Action", dataIndex: "refreshAction", key: "refreshAction", width: 110, render: small },
  { title: "Trigger", dataIndex: "refreshTrigger", key: "refreshTrigger", width: 95, render: small },
  { title: "Data Timestamp", dataIndex: "dataTimestamp", key: "dataTimestamp", width: 180, render: time },
  { title: "Start", dataIndex: "refreshStartTime", key: "refreshStartTime", width: 180, render: time },
  { title: "End", dataIndex: "refreshEndTime", key: "refreshEndTime", width: 180, render: time },
  {
    title: "Duration",
    key: "duration",
    width: 85,
    render: (_, r) => small(formatDuration(r.refreshStartTime, r.refreshEndTime)),
  },
  { title: "Inserted", dataIndex: "numRowsInserted", key: "numRowsInserted", width: 85, align: "right", render: small },
  { title: "Deleted", dataIndex: "numRowsDeleted", key: "numRowsDeleted", width: 85, align: "right", render: small },
  { title: "Lag (s)", dataIndex: "targetLagSec", key: "targetLagSec", width: 75, align: "right", render: small },
  {
    title: "Query ID",
    dataIndex: "queryId",
    key: "queryId",
    render: (id: string) =>
      id ? (
        <Text copyable={{ text: id }} style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 11 }}>
          {id}
        </Text>
      ) : small(""),
  },
];

type ScopeOption = "Last 24 Hours" | "Last 7 Days";

/**
 * Lists recent refreshes of one dynamic table from
 * INFORMATION_SCHEMA.DYNAMIC_TABLE_REFRESH_HISTORY (which retains 7 days).
 */
export default function DynamicTableRefreshHistoryModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<Row[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [scope, setScope] = useState<ScopeOption>("Last 24 Hours");
  const loadGenRef = useRef(0);
  const days = scope === "Last 7 Days" ? 7 : 1;

  const load = useCallback(() => {
    const generation = ++loadGenRef.current;
    setRows(null);
    setError(null);
    GetDynamicTableRefreshHistory(db, schema, name, days)
      .then((result) => {
        if (loadGenRef.current === generation) setRows(result ?? []);
      })
      .catch((e) => {
        if (loadGenRef.current === generation) {
          setError(String(e));
          setRows([]);
        }
      });
  }, [db, schema, name, days]);

  useEffect(load, [load]);

  const allRows = rows ?? [];
  const count = (pred: (s: string) => boolean) => allRows.filter((r) => pred(r.state?.toUpperCase() ?? "")).length;
  const succeeded = count((s) => s === "SUCCEEDED");
  const failed = count((s) => FAILED_STATES.has(s));
  const running = count((s) => s === "EXECUTING" || s === "SCHEDULED");

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <HistoryOutlined style={{ color: "var(--link)" }} />
          <span>Dynamic Table Refresh History</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button icon={<ReloadOutlined />} onClick={load} loading={rows === null} size="small">
            Refresh
          </Button>
          <Button onClick={onClose}>Close</Button>
        </Space>
      }
      width={1300}
      styles={{ body: { padding: "12px 0 0" } }}
    >
      <div style={{ display: "flex", gap: 12, padding: "0 24px 12px", alignItems: "center", flexWrap: "wrap" }}>
        <Segmented
          size="small"
          options={["Last 24 Hours", "Last 7 Days"]}
          value={scope}
          onChange={(v) => setScope(v as ScopeOption)}
        />
        {rows !== null && !error && (
          <>
            <Tag icon={<CheckCircleOutlined />} color="success">{succeeded} succeeded</Tag>
            <Tag icon={<CloseCircleOutlined />} color="error">{failed} failed</Tag>
            {running > 0 && <Tag icon={<SyncOutlined spin />} color="processing">{running} pending</Tag>}
            <Text type="secondary" style={{ fontSize: 12 }}>{allRows.length} total refreshes</Text>
          </>
        )}
      </div>

      {rows === null && (
        <div style={{ textAlign: "center", padding: "40px 0" }}>
          <Spin />
          <div style={{ marginTop: 12, fontSize: 12, color: "var(--text-muted)" }}>Loading refresh history…</div>
        </div>
      )}

      {error && (
        <div style={{ padding: "16px 24px", color: "#f85149", fontFamily: "monospace", fontSize: 12, whiteSpace: "pre-wrap" }}>
          {error}
        </div>
      )}

      {rows !== null && !error && (
        <Table
          dataSource={allRows}
          columns={columns}
          // DATA_TIMESTAMP repeats on retries, so key on QUERY_ID + start time
          // (DATA_TIMESTAMP disambiguates not-yet-started rows with neither).
          rowKey={(r) => `${r.queryId}-${r.refreshStartTime}-${r.dataTimestamp}`}
          size="small"
          scroll={{ x: "max-content" }}
          pagination={allRows.length > 50 ? { pageSize: 50, showSizeChanger: false } : false}
          rowClassName={(r) => (FAILED_STATES.has(r.state?.toUpperCase()) ? "task-row-failed" : "")}
          style={{ fontSize: 12 }}
          locale={{ emptyText: "No refreshes found in this period" }}
        />
      )}
    </Modal>
  );
}
