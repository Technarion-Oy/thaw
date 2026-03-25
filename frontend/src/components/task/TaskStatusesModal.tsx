// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Modal, Table, Tag, Space, Typography, Spin, Tooltip, Button, Input, Alert } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  MinusCircleOutlined,
  ClockCircleOutlined,
  SearchOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { GetTaskStatuses } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
}

function taskStateTag(state: string) {
  switch (state.toUpperCase()) {
    case "STARTED":
      return <Tag color="success">STARTED</Tag>;
    case "SUSPENDED":
      return <Tag color="default">SUSPENDED</Tag>;
    default:
      return <Tag>{state || "—"}</Tag>;
  }
}

function runStateTag(state: string) {
  if (!state) return <Text type="secondary" style={{ fontSize: 12 }}>Never run</Text>;
  switch (state.toUpperCase()) {
    case "SUCCEEDED":
      return <Tag icon={<CheckCircleOutlined />} color="success">SUCCEEDED</Tag>;
    case "FAILED":
      return <Tag icon={<CloseCircleOutlined />} color="error">FAILED</Tag>;
    case "RUNNING":
      return <Tag icon={<SyncOutlined spin />} color="processing">RUNNING</Tag>;
    case "SKIPPED":
      return <Tag icon={<MinusCircleOutlined />} color="gold">SKIPPED</Tag>;
    case "CANCELLED":
    case "CANCELED":
      return <Tag icon={<MinusCircleOutlined />} color="default">CANCELLED</Tag>;
    default:
      return <Tag>{state}</Tag>;
  }
}

function formatTime(ts: string): string {
  if (!ts) return "—";
  const d = new Date(ts);
  if (isNaN(d.getTime())) return ts;
  return d.toLocaleString(undefined, {
    year: "numeric", month: "short", day: "numeric",
    hour: "2-digit", minute: "2-digit", second: "2-digit",
  });
}

export default function TaskStatusesModal({ db, schema, onClose }: Props) {
  const [rows, setRows] = useState<main.TaskStatusRow[] | null>(null);
  const [historyError, setHistoryError] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  const load = () => {
    setRows(null);
    setError(null);
    setHistoryError("");
    GetTaskStatuses(db, schema)
      .then((result) => {
        setRows(result.rows ?? []);
        setHistoryError(result.historyError ?? "");
      })
      .catch((e) => setError(String(e)));
  };

  useEffect(() => { load(); }, [db, schema]);

  const filtered = (rows ?? []).filter((r) =>
    !search || r.name.toLowerCase().includes(search.toLowerCase())
  );

  const columns = [
    {
      title: "Task",
      dataIndex: "name",
      key: "name",
      sorter: (a: main.TaskStatusRow, b: main.TaskStatusRow) => a.name.localeCompare(b.name),
      render: (name: string) => (
        <Text
          style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}
        >
          {name}
        </Text>
      ),
    },
    {
      title: "State",
      dataIndex: "taskState",
      key: "taskState",
      width: 110,
      filters: [
        { text: "Started", value: "STARTED" },
        { text: "Suspended", value: "SUSPENDED" },
      ],
      onFilter: (value: unknown, record: main.TaskStatusRow) =>
        record.taskState.toUpperCase() === String(value),
      render: (state: string) => taskStateTag(state),
    },
    {
      title: "Last run",
      dataIndex: "lastRunState",
      key: "lastRunState",
      width: 150,
      filters: [
        { text: "Succeeded", value: "SUCCEEDED" },
        { text: "Failed",    value: "FAILED" },
        { text: "Running",   value: "RUNNING" },
        { text: "Skipped",   value: "SKIPPED" },
        { text: "Cancelled", value: "CANCELLED" },
        { text: "Never run", value: "" },
      ],
      onFilter: (value: unknown, record: main.TaskStatusRow) => {
        const v = String(value).toUpperCase();
        const s = (record.lastRunState ?? "").toUpperCase();
        if (v === "") return s === "";
        return s === v || (v === "CANCELLED" && s === "CANCELED");
      },
      render: (state: string) => runStateTag(state),
    },
    {
      title: "Completed",
      dataIndex: "lastRunTime",
      key: "lastRunTime",
      width: 190,
      sorter: (a: main.TaskStatusRow, b: main.TaskStatusRow) =>
        (a.lastRunTime ?? "").localeCompare(b.lastRunTime ?? ""),
      render: (ts: string) => (
        <Text type="secondary" style={{ fontSize: 12 }}>
          {formatTime(ts)}
        </Text>
      ),
    },
    {
      title: "Error",
      dataIndex: "errorMsg",
      key: "errorMsg",
      render: (msg: string) => {
        if (!msg) return null;
        const short = msg.length > 60 ? msg.slice(0, 60) + "…" : msg;
        return (
          <Tooltip title={<pre style={{ margin: 0, fontSize: 11, whiteSpace: "pre-wrap", maxWidth: 420 }}>{msg}</pre>} overlayStyle={{ maxWidth: 460 }}>
            <Text type="danger" style={{ fontSize: 12, cursor: "default" }}>{short}</Text>
          </Tooltip>
        );
      },
    },
  ];

  const successCount  = (rows ?? []).filter((r) => r.lastRunState?.toUpperCase() === "SUCCEEDED").length;
  const failedCount   = (rows ?? []).filter((r) => r.lastRunState?.toUpperCase() === "FAILED").length;
  const runningCount  = (rows ?? []).filter((r) => r.lastRunState?.toUpperCase() === "RUNNING").length;
  const neverCount    = (rows ?? []).filter((r) => !r.lastRunState).length;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ClockCircleOutlined style={{ color: "var(--link)" }} />
          <span>Task statuses</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button icon={<ReloadOutlined />} onClick={load} loading={rows === null && !error}>
            Refresh
          </Button>
          <Button onClick={onClose}>Close</Button>
        </Space>
      }
      width={900}
      styles={{ body: { padding: "12px 0 0" } }}
    >
      {/* Summary chips */}
      {rows !== null && !error && (
        <div style={{ display: "flex", gap: 12, padding: "0 24px 12px", flexWrap: "wrap" }}>
          <Tag icon={<CheckCircleOutlined />} color="success">{successCount} succeeded</Tag>
          <Tag icon={<CloseCircleOutlined />} color="error">{failedCount} failed</Tag>
          {runningCount > 0 && <Tag icon={<SyncOutlined spin />} color="processing">{runningCount} running</Tag>}
          <Tag color="default">{neverCount} never run</Tag>
        </div>
      )}

      {/* History query warning */}
      {historyError && (
        <div style={{ padding: "0 24px 12px" }}>
          <Alert
            type="warning"
            showIcon
            message="Run history unavailable"
            description={
              <span style={{ fontSize: 12, fontFamily: "monospace" }}>{historyError}</span>
            }
          />
        </div>
      )}

      {/* Search */}
      <div style={{ padding: "0 24px 10px" }}>
        <Input
          size="small"
          placeholder="Filter by name…"
          prefix={<SearchOutlined style={{ color: "var(--text-muted)", fontSize: 11 }} />}
          allowClear
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{ maxWidth: 260, fontSize: 12 }}
        />
      </div>

      {/* Loading */}
      {rows === null && !error && (
        <div style={{ textAlign: "center", padding: "40px 0" }}>
          <Spin />
          <div style={{ marginTop: 12, fontSize: 12, color: "var(--text-muted)" }}>
            Loading task statuses…
          </div>
        </div>
      )}

      {/* Error */}
      {error && (
        <div style={{ padding: "16px 24px", color: "#f85149", fontFamily: "monospace", fontSize: 12 }}>
          {error}
        </div>
      )}

      {/* Table */}
      {rows !== null && !error && (
        <Table
          dataSource={filtered}
          columns={columns as any}
          rowKey="name"
          size="small"
          pagination={filtered.length > 20 ? { pageSize: 20, showSizeChanger: false } : false}
          rowClassName={(r) =>
            r.lastRunState?.toUpperCase() === "FAILED" ? "task-row-failed" : ""
          }
          style={{ fontSize: 12 }}
          locale={{ emptyText: "No tasks found" }}
        />
      )}
    </Modal>
  );
}
