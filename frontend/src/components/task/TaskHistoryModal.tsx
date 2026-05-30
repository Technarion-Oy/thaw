// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { Modal, Table, Tag, Space, Typography, Spin, Tooltip, Button, Segmented } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  SyncOutlined,
  MinusCircleOutlined,
  HistoryOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { GetTaskRunHistory, ListObjects } from "../../../wailsjs/go/app/App";
import type { tasks, snowflake } from "../../../wailsjs/go/models";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  isRoot: boolean;
  onClose: () => void;
}

// ── Status tag rendering ────────────────────────────────────────────────────

function runStateTag(state: string) {
  if (!state) return <Text type="secondary" style={{ fontSize: 12 }}>—</Text>;
  switch (state.toUpperCase()) {
    case "SUCCEEDED": return <Tag icon={<CheckCircleOutlined />} color="success">SUCCEEDED</Tag>;
    case "FAILED":    return <Tag icon={<CloseCircleOutlined />} color="error">FAILED</Tag>;
    case "EXECUTING":
    case "RUNNING":   return <Tag icon={<SyncOutlined spin />} color="processing">EXECUTING</Tag>;
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

function formatDuration(startTs: string, endTs: string): string {
  if (!startTs || !endTs) return "—";
  const start = new Date(startTs);
  const end = new Date(endTs);
  if (isNaN(start.getTime()) || isNaN(end.getTime())) return "—";
  const diffMs = end.getTime() - start.getTime();
  if (diffMs < 0) return "—";
  const totalSecs = Math.floor(diffMs / 1000);
  if (totalSecs < 60) return `${totalSecs}s`;
  const mins = Math.floor(totalSecs / 60);
  const secs = totalSecs % 60;
  if (mins < 60) return `${mins}m ${secs}s`;
  const hrs = Math.floor(mins / 60);
  const remMins = mins % 60;
  return `${hrs}h ${remMins}m`;
}

// ── Task topological ordering ───────────────────────────────────────────────

// Build a topological ordering of task names from the predecessor graph.
// Returns a Map<UPPER_NAME, index> where the root task is 0, its direct
// children come next, etc.  Tasks not found in the graph get Infinity so
// they sort to the end.
function buildTopoOrder(taskObjects: snowflake.SnowflakeObject[]): Map<string, number> {
  const tasks = taskObjects.filter((o) => o.kind === "TASK");
  const nameSet = new Set(tasks.map((t) => t.name.toUpperCase()));

  // parentOf: child → first local predecessor
  const childrenOf = new Map<string, string[]>();
  const roots: string[] = [];

  for (const t of tasks) {
    const preds = parsePredecessors(t.predecessors ?? "");
    const localParent = preds
      .map((p) => extractName(p).toUpperCase())
      .find((n) => nameSet.has(n));
    if (localParent) {
      if (!childrenOf.has(localParent)) childrenOf.set(localParent, []);
      childrenOf.get(localParent)!.push(t.name.toUpperCase());
    } else if (!t.finalize) {
      // No predecessor and not a finalizer → root-level task
      roots.push(t.name.toUpperCase());
    }
  }

  // BFS from roots to assign depth-first order
  const order = new Map<string, number>();
  let idx = 0;
  const queue = [...roots];
  while (queue.length > 0) {
    const name = queue.shift()!;
    if (order.has(name)) continue;
    order.set(name, idx++);
    const kids = childrenOf.get(name) ?? [];
    for (const kid of kids) queue.push(kid);
  }
  // Finalizer tasks go at the end
  for (const t of tasks) {
    if (t.finalize && !order.has(t.name.toUpperCase())) {
      order.set(t.name.toUpperCase(), idx++);
    }
  }
  return order;
}

// ── DAG Run grouping ────────────────────────────────────────────────────────

interface DAGRun {
  runId: string;
  scheduledTime: string;
  tasks: tasks.TaskHistoryRow[];
  status: string; // aggregate: SUCCEEDED | FAILED | EXECUTING | MIXED
  taskCount: number;
}

function groupByRunId(
  rows: tasks.TaskHistoryRow[],
  topoOrder: Map<string, number>,
): DAGRun[] {
  const groups = new Map<string, tasks.TaskHistoryRow[]>();
  // Track first-seen scheduled time per run for display ordering
  const schedTimes = new Map<string, string>();

  for (const row of rows) {
    const key = row.runId || row.scheduledTime || "unknown";
    if (!groups.has(key)) {
      groups.set(key, []);
      schedTimes.set(key, row.scheduledTime);
    }
    groups.get(key)!.push(row);
  }

  return Array.from(groups.entries()).map(([runId, dagTasks]) => {
    // Sort tasks within the run by topological order
    dagTasks.sort((a, b) => {
      const aIdx = topoOrder.get(a.name.toUpperCase()) ?? Infinity;
      const bIdx = topoOrder.get(b.name.toUpperCase()) ?? Infinity;
      return aIdx - bIdx;
    });

    const states = dagTasks.map((t) => t.state.toUpperCase());
    let status: string;
    if (states.some((s) => s === "FAILED")) status = "FAILED";
    else if (states.some((s) => s === "EXECUTING" || s === "RUNNING")) status = "EXECUTING";
    else if (states.every((s) => s === "SUCCEEDED")) status = "SUCCEEDED";
    else status = "MIXED";

    return {
      runId,
      scheduledTime: schedTimes.get(runId) ?? "",
      tasks: dagTasks,
      status,
      taskCount: dagTasks.length,
    };
  });
}

// ── Error column renderer ────────────────────────────────────────────────

function renderError(msg: string) {
  if (!msg) return null;
  const short = msg.length > 60 ? msg.slice(0, 60) + "\u2026" : msg;
  return (
    <Tooltip
      title={<pre style={{ margin: 0, fontSize: 11, whiteSpace: "pre-wrap", maxWidth: 420 }}>{msg}</pre>}
      overlayStyle={{ maxWidth: 460 }}
    >
      <Text type="danger" style={{ fontSize: 12, cursor: "default" }}>{short}</Text>
    </Tooltip>
  );
}

// ── Column definitions (module-level — no state/props dependencies) ──────

const flatColumns: ColumnsType<tasks.TaskHistoryRow> = [
  {
    title: "Status",
    dataIndex: "state",
    key: "state",
    width: 130,
    render: (state: string) => runStateTag(state),
  },
  {
    title: "Scheduled",
    dataIndex: "scheduledTime",
    key: "scheduledTime",
    width: 190,
    render: (ts: string) => (
      <Text type="secondary" style={{ fontSize: 12 }}>{formatTime(ts)}</Text>
    ),
  },
  {
    title: "Start",
    dataIndex: "startTime",
    key: "startTime",
    width: 190,
    render: (ts: string) => (
      <Text type="secondary" style={{ fontSize: 12 }}>{formatTime(ts)}</Text>
    ),
  },
  {
    title: "Duration",
    key: "duration",
    width: 90,
    render: (_, record) => (
      <Text style={{ fontSize: 12 }}>{formatDuration(record.startTime, record.endTime)}</Text>
    ),
  },
  {
    title: "Error",
    dataIndex: "errorMessage",
    key: "errorMessage",
    render: (msg: string) => renderError(msg),
  },
];

const dagColumns: ColumnsType<DAGRun> = [
  {
    title: "Status",
    dataIndex: "status",
    key: "status",
    width: 130,
    render: (status: string) => runStateTag(status),
  },
  {
    title: "Scheduled",
    dataIndex: "scheduledTime",
    key: "scheduledTime",
    width: 190,
    render: (ts: string) => (
      <Text type="secondary" style={{ fontSize: 12 }}>{formatTime(ts)}</Text>
    ),
  },
  {
    title: "Tasks",
    dataIndex: "taskCount",
    key: "taskCount",
    width: 70,
    render: (count: number) => (
      <Text style={{ fontSize: 12 }}>{count}</Text>
    ),
  },
  {
    title: "",
    key: "spacer",
  },
];

const expandedColumns: ColumnsType<tasks.TaskHistoryRow> = [
  {
    title: "Status",
    dataIndex: "state",
    key: "state",
    width: 130,
    render: (state: string) => runStateTag(state),
  },
  {
    title: "Task",
    dataIndex: "name",
    key: "name",
    width: 180,
    render: (n: string) => (
      <Text style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}>
        {n}
      </Text>
    ),
  },
  {
    title: "Start",
    dataIndex: "startTime",
    key: "startTime",
    width: 190,
    render: (ts: string) => (
      <Text type="secondary" style={{ fontSize: 12 }}>{formatTime(ts)}</Text>
    ),
  },
  {
    title: "Duration",
    key: "duration",
    width: 90,
    render: (_, record) => (
      <Text style={{ fontSize: 12 }}>{formatDuration(record.startTime, record.endTime)}</Text>
    ),
  },
  {
    title: "Error",
    dataIndex: "errorMessage",
    key: "errorMessage",
    render: (msg: string) => renderError(msg),
  },
];

// ── Component ─────────────────────────────────────────────────────────────

type ScopeOption = "Last 24 Hours" | "Last 7 Days";

export default function TaskHistoryModal({ db, schema, name, isRoot, onClose }: Props) {
  const [rows, setRows] = useState<tasks.TaskHistoryRow[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [scope, setScope] = useState<ScopeOption>("Last 24 Hours");
  const [autoRefresh, setAutoRefresh] = useState(false);
  const [topoOrder, setTopoOrder] = useState<Map<string, number>>(new Map());
  const [expandedRunIds, setExpandedRunIds] = useState<string[]>([]);
  const intervalRef = useRef<ReturnType<typeof setInterval> | null>(null);
  const loadGenRef = useRef(0);

  const days = scope === "Last 7 Days" ? 7 : 1;

  // Fetch task hierarchy once to determine topological order
  useEffect(() => {
    if (!isRoot) return;
    ListObjects(db, schema)
      .then((objs) => setTopoOrder(buildTopoOrder(objs)))
      .catch(() => {}); // non-fatal — tasks will just be unsorted
  }, [db, schema, isRoot]);

  const load = useCallback(() => {
    const generation = ++loadGenRef.current;
    setError(null);
    GetTaskRunHistory(db, schema, name, isRoot, days)
      .then((result) => {
        if (loadGenRef.current === generation) setRows(result ?? []);
      })
      .catch((e) => {
        if (loadGenRef.current === generation) {
          setError(String(e));
          setRows([]);
        }
      });
  }, [db, schema, name, isRoot, days]);

  useEffect(() => {
    setRows(null);
    load();
  }, [load]);

  // Auto-refresh timer
  useEffect(() => {
    if (autoRefresh) {
      intervalRef.current = setInterval(load, 10_000);
    }
    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [autoRefresh, load]);

  // Summary counts
  const allRows = rows ?? [];
  const successCount = allRows.filter((r) => r.state?.toUpperCase() === "SUCCEEDED").length;
  const failedCount  = allRows.filter((r) => r.state?.toUpperCase() === "FAILED").length;
  const runningCount = allRows.filter((r) => {
    const s = r.state?.toUpperCase();
    return s === "EXECUTING" || s === "RUNNING";
  }).length;
  const skippedCount = allRows.filter((r) => {
    const s = r.state?.toUpperCase();
    return s === "SKIPPED" || s === "CANCELLED" || s === "CANCELED";
  }).length;

  // DAG runs for root tasks — grouped by RUN_ID, sorted by topological order
  const dagRuns = useMemo(
    () => isRoot ? groupByRunId(allRows, topoOrder) : [],
    [isRoot, allRows, topoOrder],
  );

  // Keep the latest (first) run expanded; prune stale IDs after refresh.
  useEffect(() => {
    if (dagRuns.length === 0) return;
    setExpandedRunIds((prev) => {
      const validIds = new Set(dagRuns.map((r) => r.runId));
      const kept = prev.filter((id) => validIds.has(id));
      // If nothing is expanded (first load or all stale), expand the latest run
      if (kept.length === 0) kept.push(dagRuns[0].runId);
      return kept;
    });
  }, [dagRuns]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <HistoryOutlined style={{ color: "var(--link)" }} />
          <span>Task Run History</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
          {isRoot && <Tag color="blue" style={{ fontSize: 10, lineHeight: "16px", padding: "0 4px" }}>ROOT</Tag>}
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button
            icon={<SyncOutlined spin={autoRefresh} />}
            onClick={() => setAutoRefresh((v) => !v)}
            type={autoRefresh ? "primary" : "default"}
            ghost={autoRefresh}
            size="small"
          >
            {autoRefresh ? "Auto-refresh ON" : "Auto-refresh"}
          </Button>
          <Button icon={<ReloadOutlined />} onClick={load} loading={rows === null && !error} size="small">
            Refresh
          </Button>
          <Button onClick={onClose}>Close</Button>
        </Space>
      }
      width={1050}
      styles={{ body: { padding: "12px 0 0" } }}
    >
      {/* Controls bar */}
      <div style={{ display: "flex", gap: 12, padding: "0 24px 12px", alignItems: "center", flexWrap: "wrap" }}>
        <Segmented
          size="small"
          options={["Last 24 Hours", "Last 7 Days"]}
          value={scope}
          onChange={(v) => setScope(v as ScopeOption)}
        />
      </div>

      {/* Summary chips */}
      {rows !== null && !error && (
        <div style={{ display: "flex", gap: 12, padding: "0 24px 12px", flexWrap: "wrap" }}>
          <Tag icon={<CheckCircleOutlined />} color="success">{successCount} succeeded</Tag>
          <Tag icon={<CloseCircleOutlined />} color="error">{failedCount} failed</Tag>
          {runningCount > 0 && <Tag icon={<SyncOutlined spin />} color="processing">{runningCount} executing</Tag>}
          {skippedCount > 0 && <Tag icon={<MinusCircleOutlined />} color="default">{skippedCount} skipped</Tag>}
          <Text type="secondary" style={{ fontSize: 12 }}>
            {isRoot ? `${dagRuns.length} DAG runs` : `${allRows.length} total executions`}
          </Text>
        </div>
      )}

      {/* Loading */}
      {rows === null && !error && (
        <div style={{ textAlign: "center", padding: "40px 0" }}>
          <Spin />
          <div style={{ marginTop: 12, fontSize: 12, color: "var(--text-muted)" }}>
            Loading task history…
          </div>
        </div>
      )}

      {/* Error */}
      {error && (
        <div style={{ padding: "16px 24px", color: "#f85149", fontFamily: "monospace", fontSize: 12 }}>
          {error}
        </div>
      )}

      {/* Root task: DAG run view with expandable rows */}
      {rows !== null && !error && isRoot && (
        <Table
          dataSource={dagRuns}
          columns={dagColumns}
          rowKey="runId"
          size="small"
          expandable={{
            expandedRowRender: (dagRun: DAGRun) => (
              <Table
                dataSource={dagRun.tasks}
                columns={expandedColumns}
                rowKey={(r) => `${r.name}-${r.startTime}-${r.runId}`}
                size="small"
                pagination={false}
                rowClassName={(r) =>
                  r.state?.toUpperCase() === "FAILED" ? "task-row-failed" : ""
                }
              />
            ),
            expandedRowKeys: expandedRunIds,
            onExpandedRowsChange: (keys) => setExpandedRunIds(keys as string[]),
          }}
          pagination={dagRuns.length > 20 ? { pageSize: 20, showSizeChanger: false } : false}
          style={{ fontSize: 12 }}
          locale={{ emptyText: "No task runs found in this period" }}
        />
      )}

      {/* Child / standalone task: flat list */}
      {rows !== null && !error && !isRoot && (
        <Table
          dataSource={allRows}
          columns={flatColumns}
          rowKey={(r) => `${r.scheduledTime}-${r.startTime}-${r.runId}`}
          size="small"
          pagination={allRows.length > 50 ? { pageSize: 50, showSizeChanger: false } : false}
          rowClassName={(r) =>
            r.state?.toUpperCase() === "FAILED" ? "task-row-failed" : ""
          }
          style={{ fontSize: 12 }}
          locale={{ emptyText: "No task runs found in this period" }}
        />
      )}
    </Modal>
  );
}
