// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect, useMemo } from "react";
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
import { GetTaskStatuses } from "../../../wailsjs/go/app/App";
import type { tasks } from "../../../wailsjs/go/models";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
}

// Extend the row type with an optional children array for Ant Design tree table.
interface TreeRow extends tasks.StatusRow {
  children?: TreeRow[];
}

// ── Tree builder ─────────────────────────────────────────────────────────────
// Builds a nested tree from a flat list.  A task is a child of the first
// predecessor that also lives in this schema.  Tasks whose predecessor is
// outside this schema (or who have no predecessors) are placed at the root.
function buildHierarchy(flat: tasks.StatusRow[]): TreeRow[] {
  const byName = new Map<string, tasks.StatusRow>();
  for (const r of flat) byName.set(r.name.toUpperCase(), r);

  // Map each task to its parent name within this schema (upper-cased).
  const parentOf = new Map<string, string>();
  const childrenOf = new Map<string, string[]>();

  for (const row of flat) {
    const preds = parsePredecessors(row.predecessors ?? "");
    const localParent = preds
      .map((p) => extractName(p).toUpperCase())
      .find((n) => byName.has(n));

    if (localParent) {
      parentOf.set(row.name.toUpperCase(), localParent);
      if (!childrenOf.has(localParent)) childrenOf.set(localParent, []);
      childrenOf.get(localParent)!.push(row.name);
    }
  }

  const inTree = new Set<string>();

  function buildSubTree(name: string): TreeRow {
    const row = byName.get(name.toUpperCase())!;
    inTree.add(name.toUpperCase());
    const kids = childrenOf.get(name.toUpperCase()) ?? [];
    const node: TreeRow = { ...row };
    if (kids.length > 0) node.children = kids.map(buildSubTree);
    return node;
  }

  const result: TreeRow[] = [];
  // Root tasks first (no local parent).
  for (const row of flat) {
    if (!parentOf.has(row.name.toUpperCase())) {
      result.push(buildSubTree(row.name));
    }
  }
  // Any task not yet placed (e.g., circular refs) — append as a root.
  for (const row of flat) {
    if (!inTree.has(row.name.toUpperCase())) {
      result.push({ ...row });
    }
  }
  return result;
}

// Flatten a tree to a plain list (used for search filtering).
function flattenTree(nodes: TreeRow[]): TreeRow[] {
  const out: TreeRow[] = [];
  for (const n of nodes) {
    out.push(n);
    if (n.children) out.push(...flattenTree(n.children));
  }
  return out;
}

// ── Tag renderers ─────────────────────────────────────────────────────────────
function taskStateTag(state: string) {
  switch (state.toUpperCase()) {
    case "STARTED":   return <Tag color="success">STARTED</Tag>;
    case "SUSPENDED": return <Tag color="default">SUSPENDED</Tag>;
    default:          return <Tag>{state || "—"}</Tag>;
  }
}

function runStateTag(state: string) {
  if (!state) return <Text type="secondary" style={{ fontSize: 12 }}>Never run</Text>;
  switch (state.toUpperCase()) {
    case "SUCCEEDED": return <Tag icon={<CheckCircleOutlined />} color="success">SUCCEEDED</Tag>;
    case "FAILED":    return <Tag icon={<CloseCircleOutlined />} color="error">FAILED</Tag>;
    case "RUNNING":   return <Tag icon={<SyncOutlined spin />} color="processing">RUNNING</Tag>;
    case "SKIPPED":   return <Tag icon={<MinusCircleOutlined />} color="gold">SKIPPED</Tag>;
    case "CANCELLED":
    case "CANCELED":  return <Tag icon={<MinusCircleOutlined />} color="default">CANCELLED</Tag>;
    default:          return <Tag>{state}</Tag>;
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

// ── Component ─────────────────────────────────────────────────────────────────
export default function TaskStatusesModal({ db, schema, onClose }: Props) {
  const [rows, setRows] = useState<tasks.StatusRow[] | null>(null);
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

  // Build the tree once; flatten only when searching.
  const treeData = useMemo(() => buildHierarchy(rows ?? []), [rows]);

  const displayData: TreeRow[] = useMemo(() => {
    if (!search) return treeData;
    // When searching, show a flat list filtered by name (no tree structure).
    return flattenTree(treeData).filter((r) =>
      r.name.toLowerCase().includes(search.toLowerCase())
    );
  }, [treeData, search]);

  const allFlat = useMemo(() => flattenTree(treeData), [treeData]);
  const successCount = allFlat.filter((r) => r.lastRunState?.toUpperCase() === "SUCCEEDED").length;
  const failedCount  = allFlat.filter((r) => r.lastRunState?.toUpperCase() === "FAILED").length;
  const runningCount = allFlat.filter((r) => r.lastRunState?.toUpperCase() === "RUNNING").length;
  const neverCount   = allFlat.filter((r) => !r.lastRunState).length;

  const columns = [
    {
      title: "Task",
      dataIndex: "name",
      key: "name",
      render: (name: string, record: TreeRow) => (
        <div>
          <Text style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}>
            {name}
          </Text>
          {/* TEMP DEBUG — remove once predecessor parsing is confirmed working */}
          <div style={{ fontSize: 10, color: "var(--text-muted)", fontFamily: "monospace", wordBreak: "break-all" }}>
            preds: {record.predecessors || "(empty)"}
          </div>
        </div>
      ),
    },
    {
      title: "State",
      dataIndex: "taskState",
      key: "taskState",
      width: 110,
      filters: [
        { text: "Started",   value: "STARTED" },
        { text: "Suspended", value: "SUSPENDED" },
      ],
      onFilter: (value: unknown, record: TreeRow) =>
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
      onFilter: (value: unknown, record: TreeRow) => {
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
      render: (ts: string) => (
        <Text type="secondary" style={{ fontSize: 12 }}>{formatTime(ts)}</Text>
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
          <Tooltip
            title={<pre style={{ margin: 0, fontSize: 11, whiteSpace: "pre-wrap", maxWidth: 420 }}>{msg}</pre>}
            overlayStyle={{ maxWidth: 460 }}
          >
            <Text type="danger" style={{ fontSize: 12, cursor: "default" }}>{short}</Text>
          </Tooltip>
        );
      },
    },
  ];

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

      {/* Table — tree when not searching, flat when searching */}
      {rows !== null && !error && (
        <Table
          dataSource={displayData}
          columns={columns as any}
          rowKey="name"
          size="small"
          expandable={
            !search
              ? { defaultExpandAllRows: true }
              : undefined
          }
          pagination={allFlat.length > 50 ? { pageSize: 50, showSizeChanger: false } : false}
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
