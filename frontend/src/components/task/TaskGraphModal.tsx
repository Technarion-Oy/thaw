// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef, useCallback } from "react";
import { Modal, Spin, Button, Space, Typography, Alert, Tag, message, Tooltip } from "antd";
import {
  CheckCircleOutlined, CloseCircleOutlined, SyncOutlined,
  MinusCircleOutlined, ClockCircleOutlined, ReloadOutlined,
  CaretRightOutlined, RedoOutlined,
} from "@ant-design/icons";
import {
  ReactFlow,
  Background,
  Controls,
  Panel,
  useNodesState,
  useEdgesState,
  Position,
  MarkerType,
  type Node,
  type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import dagre from "@dagrejs/dagre";
import { GetTaskStatuses, ExecuteTask } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

const { Text } = Typography;

const NODE_W = 200;
const NODE_H = 96;
const POLL_MS = 3_000;

// ── Status tags ───────────────────────────────────────────────────────────────

function runStateTag(state: string) {
  const s = (state ?? "").toUpperCase();
  switch (s) {
    case "SUCCEEDED": return <Tag icon={<CheckCircleOutlined />}  color="success"    style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}>Succeeded</Tag>;
    case "FAILED":
    case "FAILED_AND_AUTO_SUSPENDED":
                      return <Tag icon={<CloseCircleOutlined />}  color="error"      style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}>Failed</Tag>;
    case "RUNNING":
    case "EXECUTING": return <Tag icon={<SyncOutlined spin />}    color="processing" style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}>Running</Tag>;
    case "SCHEDULED": return <Tag icon={<ClockCircleOutlined />}  color="processing" style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}>Scheduled</Tag>;
    case "SKIPPED":   return <Tag icon={<MinusCircleOutlined />}  color="gold"       style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}>Skipped</Tag>;
    case "CANCELLED":
    case "CANCELED":  return <Tag icon={<MinusCircleOutlined />}  color="default"    style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}>Cancelled</Tag>;
    case "WAITING":   return <Tag color="default" style={{ fontSize: 10, margin: 0, lineHeight: 1.6, color: "var(--text-faint)", fontStyle: "italic" }}>Waiting…</Tag>;
    default:          return <Tag color="default" style={{ fontSize: 10, margin: 0, lineHeight: 1.6, color: "var(--text-faint)", fontStyle: "italic" }}>Never run</Tag>;
  }
}

// ── Infer SKIPPED state from predecessor failures ─────────────────────────────
// Snowflake may not create a TASK_HISTORY row for tasks skipped because a
// predecessor failed. This does a fixed-point walk so transitive skips also
// propagate (e.g. task_d depends on task_c which was skipped).
// A task is NOT inferred as skipped if it is currently executing/scheduled/
// has already succeeded in this round.

const ACTIVE_STATES = new Set(["EXECUTING", "RUNNING", "SCHEDULED", "SUCCEEDED"]);

function computeSkippedNodes(byName: Map<string, main.TaskStatusRow>): Set<string> {
  const skipped = new Set<string>();
  let changed = true;
  while (changed) {
    changed = false;
    for (const t of byName.values()) {
      const upper = t.name.toUpperCase();
      if (skipped.has(upper)) continue;
      if (ACTIVE_STATES.has((t.lastRunState ?? "").toUpperCase())) continue;
      for (const p of parsePredecessors(t.predecessors ?? "")) {
        const pu = extractName(p).toUpperCase();
        const pred = byName.get(pu);
        if (!pred) continue;
        const ps = (pred.lastRunState ?? "").toUpperCase();
        if (ps === "FAILED" || ps === "FAILED_AND_AUTO_SUSPENDED" || skipped.has(pu)) {
          skipped.add(upper);
          changed = true;
          break;
        }
      }
    }
  }
  return skipped;
}

// ── Timestamp formatting ──────────────────────────────────────────────────────
// lastRunTime arrives as RFC3339 (e.g. "2024-01-15T10:30:00Z") after the Go
// toString fix. Shows HH:MM:SS for today, "Jan 15 HH:MM" for other days.

function formatRunTime(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  const now = new Date();
  const isToday = d.toDateString() === now.toDateString();
  return isToday
    ? d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : d.toLocaleDateString([], { month: "short", day: "numeric" }) + " " +
      d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
}

// ── Node label (module-level so it can be called during polling) ──────────────
// overrideRunState replaces lastRunState for display purposes (e.g. "WAITING",
// inferred "SKIPPED"). Pass undefined to use the real value.

function buildLabel(t: main.TaskStatusRow, isRoot: boolean, overrideRunState?: string) {
  const started      = t.taskState?.toUpperCase() === "STARTED";
  const effectiveState = (overrideRunState ?? t.lastRunState ?? "").toUpperCase();
  // Show timestamp for terminal states; suppress for Waiting/never-run/executing.
  const showTime     = !!t.lastRunTime &&
    effectiveState !== "WAITING" &&
    effectiveState !== "EXECUTING" &&
    effectiveState !== "RUNNING" &&
    effectiveState !== "SCHEDULED" &&
    effectiveState !== "";
  const timeLabel    = showTime ? formatRunTime(t.lastRunTime!) : null;

  return (
    <div style={{ textAlign: "center", lineHeight: 1.3 }}>
      <div style={{
        fontFamily: "monospace", fontSize: 11,
        fontWeight: isRoot ? 700 : 400,
        color: "var(--text)",
        marginBottom: 4,
        wordBreak: "break-all",
      }}>
        {t.name}
      </div>
      <div style={{ display: "flex", gap: 4, justifyContent: "center", flexWrap: "wrap" }}>
        <Tag
          color={started ? "success" : "default"}
          style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}
        >
          {t.taskState || "UNKNOWN"}
        </Tag>
        {runStateTag(overrideRunState ?? t.lastRunState ?? "")}
      </div>
      {timeLabel && (
        <div style={{ fontSize: 10, color: "var(--text-faint)", marginTop: 3 }}>
          {timeLabel}
        </div>
      )}
    </div>
  );
}

// ── Dagre layout ──────────────────────────────────────────────────────────────

function applyLayout(nodes: Node[], edges: Edge[]): Node[] {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "LR", nodesep: 40, ranksep: 80 });
  nodes.forEach((n) => g.setNode(n.id, { width: NODE_W, height: NODE_H }));
  edges.forEach((e) => g.setEdge(e.source, e.target));
  dagre.layout(g);
  return nodes.map((n) => {
    const { x, y } = g.node(n.id);
    return { ...n, position: { x: x - NODE_W / 2, y: y - NODE_H / 2 } };
  });
}

// ── Graph builder ─────────────────────────────────────────────────────────────
// Finds the root of the connected component containing `focusedName`, then
// collects all descendants via BFS to render the full task graph.

function buildGraph(tasks: main.TaskStatusRow[], focusedName: string) {
  const byName = new Map<string, main.TaskStatusRow>();
  tasks.forEach((t) => byName.set(t.name.toUpperCase(), t));

  const skippedNodes = computeSkippedNodes(byName);

  const childrenOf = new Map<string, string[]>();
  const parentOf   = new Map<string, string>();

  tasks.forEach((t) => {
    for (const p of parsePredecessors(t.predecessors ?? "")) {
      const pu = extractName(p).toUpperCase();
      if (!byName.has(pu)) continue;
      if (!childrenOf.has(pu)) childrenOf.set(pu, []);
      childrenOf.get(pu)!.push(t.name);
      parentOf.set(t.name.toUpperCase(), pu);
    }
  });

  // Walk up from the focused task to find the root.
  let rootUpper = focusedName.toUpperCase();
  while (parentOf.has(rootUpper)) rootUpper = parentOf.get(rootUpper)!;

  // BFS from root to collect all tasks in the graph.
  const included = new Set<string>();
  const queue = [rootUpper];
  while (queue.length > 0) {
    const cur = queue.shift()!;
    if (included.has(cur)) continue;
    included.add(cur);
    for (const child of childrenOf.get(cur) ?? []) {
      queue.push(child.toUpperCase());
    }
  }

  const focusedUpper = focusedName.toUpperCase();
  const rootTaskName = byName.get(rootUpper)?.name ?? focusedName;

  const nodes: Node[] = [];
  const edges: Edge[] = [];

  included.forEach((upper) => {
    const t = byName.get(upper);
    if (!t) return;

    const isRoot    = upper === rootUpper;
    const isFocused = upper === focusedUpper && upper !== rootUpper;

    nodes.push({
      id: t.name,
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: { label: buildLabel(t, isRoot, skippedNodes.has(upper) ? "SKIPPED" : undefined) },
      style: {
        background: isFocused
          ? "var(--accent-bg, #1c3a5e)"
          : "var(--bg-overlay, #252526)",
        border: `1.5px solid ${
          isRoot || isFocused ? "var(--link, #4d9ef7)" : "var(--border, #444)"
        }`,
        borderRadius: 8,
        width: NODE_W,
        height: NODE_H,
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        padding: "0 12px",
        boxSizing: "border-box",
      },
    });

    for (const child of childrenOf.get(upper) ?? []) {
      if (!included.has(child.toUpperCase())) continue;
      edges.push({
        id: `${t.name}→${child}`,
        source: t.name,
        target: child,
        type: "smoothstep",
        markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14, color: "#888" },
        style: { stroke: "#888", strokeWidth: 1.5 },
      });
    }
  });

  return { nodes, edges, rootTaskName, childrenOf };
}

// ── Component ─────────────────────────────────────────────────────────────────

export interface TaskGraphModalProps {
  db:       string;
  schema:   string;
  taskName: string;
  onClose:  () => void;
}

export default function TaskGraphModal({ db, schema, taskName, onClose }: TaskGraphModalProps) {
  const [loading,    setLoading]    = useState(true);
  const [loadError,  setLoadError]  = useState<string | null>(null);
  const [rootName,   setRootName]   = useState(taskName);
  const [executing,  setExecuting]  = useState(false);
  const [retrying,   setRetrying]   = useState(false);
  const [lastPollAt, setLastPollAt] = useState<Date | null>(null);
  const [taskRows,   setTaskRows]   = useState<main.TaskStatusRow[]>([]);

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  // Stable refs so the polling closure doesn't go stale.
  const rootNameRef  = useRef(taskName);
  const rootUpperRef = useRef<string>("");
  const taskRowsRef  = useRef<main.TaskStatusRow[]>([]);

  const load = useCallback(() => {
    setLoading(true);
    setLoadError(null);
    GetTaskStatuses(db, schema)
      .then((r) => {
        const { nodes: n, edges: e, rootTaskName } = buildGraph(r.rows ?? [], taskName);
        rootNameRef.current  = rootTaskName;
        rootUpperRef.current = rootTaskName.toUpperCase();
        setRootName(rootTaskName);
        setNodes(applyLayout(n, e));
        setEdges(e);
        taskRowsRef.current = r.rows ?? [];
        setTaskRows(r.rows ?? []);
        setLastPollAt(new Date());
      })
      .catch((err) => setLoadError(String(err)))
      .finally(() => setLoading(false));
  }, [db, schema, taskName]);

  useEffect(() => { load(); }, [load]);

  // ── Live polling ─────────────────────────────────────────────────────────
  useEffect(() => {
    if (loading || loadError) return;

    const id = setInterval(() => {
      GetTaskStatuses(db, schema)
        .then((r) => {
          const rows = r.rows ?? [];
          const byName = new Map<string, main.TaskStatusRow>();
          rows.forEach((t) => byName.set(t.name.toUpperCase(), t));

          const rootUpper  = rootUpperRef.current;
          const skipped    = computeSkippedNodes(byName);

          setNodes((prev) =>
            prev.map((n) => {
              const t = byName.get(n.id.toUpperCase());
              if (!t) return n;
              const isRoot       = n.id.toUpperCase() === rootUpper;
              const overrideState = skipped.has(n.id.toUpperCase()) ? "SKIPPED" : undefined;
              return { ...n, data: { ...n.data, label: buildLabel(t, isRoot, overrideState) } };
            })
          );
          taskRowsRef.current = rows;
          setTaskRows(rows);
          setLastPollAt(new Date());
        })
        .catch(() => { /* silently ignore poll errors */ });
    }, POLL_MS);

    return () => clearInterval(id);
  }, [loading, loadError, db, schema]);

  // ── Execute root task ─────────────────────────────────────────────────────
  const runGraph = useCallback(() => {
    setExecuting(true);
    ExecuteTask(db, schema, rootNameRef.current, "", false)
      .then(() => {
        message.success(`Task graph started: ${rootNameRef.current}`);
        // Optimistically mark all child nodes as "Waiting" so stale states
        // don't linger. The polling loop will replace these with real states.
        const rootUpper = rootUpperRef.current;
        const rows      = taskRowsRef.current;
        setNodes((prev) =>
          prev.map((n) => {
            if (n.id.toUpperCase() === rootUpper) return n;
            const t = rows.find((r) => r.name.toUpperCase() === n.id.toUpperCase());
            if (!t) return n;
            return { ...n, data: { ...n.data, label: buildLabel(t, false, "WAITING") } };
          })
        );
      })
      .catch((err) => {
        message.error(String(err));
      })
      .finally(() => setExecuting(false));
  }, [db, schema]);

  // ── Retry last failed graph run (root task only) ─────────────────────────
  const retryFailed = useCallback(async () => {
    setRetrying(true);
    try {
      await ExecuteTask(db, schema, rootNameRef.current, "", true);
      message.success(`Retrying last failed run of ${rootNameRef.current}`);
    } catch (err) {
      message.error(String(err));
    } finally {
      setRetrying(false);
    }
  }, [db, schema]);

  // ── Retry eligibility (mirrors Snowflake's RETRY LAST conditions) ────────
  // A graph run is considered failed if ANY task in it failed — the root task
  // itself may have succeeded while a child task failed.
  const graphNames      = new Set(nodes.map((n) => n.id.toUpperCase()));
  const failedGraphRows = taskRows.filter((t) => {
    const s = (t.lastRunState ?? "").toUpperCase();
    return graphNames.has(t.name.toUpperCase()) &&
      (s === "FAILED" || s === "FAILED_AND_AUTO_SUSPENDED" ||
       s === "CANCELED" || s === "CANCELLED");
  });
  const graphRunFailed  = failedGraphRows.length > 0;
  // 14-day window: measure from the most recently failed/cancelled task.
  const mostRecentFailMs = failedGraphRows
    .map((t) => new Date(t.lastRunTime ?? "").getTime())
    .filter((ms) => !isNaN(ms))
    .reduce((max, ms) => Math.max(max, ms), 0);
  const within14Days    = mostRecentFailMs > 0 &&
    Date.now() - mostRecentFailMs < 14 * 24 * 60 * 60 * 1000;
  const canRetry        = graphRunFailed && within14Days;
  const retryTooltip    = !graphRunFailed
    ? "Last graph run did not fail or get cancelled"
    : !within14Days
    ? "Last failed run was more than 14 days ago"
    : `Retry last failed run of ${rootName}`;

  // ── Formatted last-updated ────────────────────────────────────────────────
  const lastUpdatedLabel = lastPollAt
    ? lastPollAt.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit", second: "2-digit" })
    : null;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ClockCircleOutlined style={{ color: "var(--link)" }} />
          <span>Task Graph</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{rootName}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button icon={<ReloadOutlined />} onClick={load} loading={loading}>
            Refresh
          </Button>
          <Button onClick={onClose}>Close</Button>
        </Space>
      }
      width={920}
      styles={{ body: { padding: 0, height: 540, overflow: "hidden" } }}
    >
      {loading && (
        <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%" }}>
          <Spin tip="Loading task graph…" />
        </div>
      )}
      {loadError && (
        <Alert type="error" message={loadError} showIcon style={{ margin: 16 }} />
      )}
      {!loading && !loadError && (
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          fitView
          fitViewOptions={{ padding: 0.18 }}
          nodesDraggable
          nodesConnectable={false}
          elementsSelectable={false}
          proOptions={{ hideAttribution: true }}
          style={{ background: "var(--bg)" }}
        >
          <Background color="var(--border)" gap={20} />
          <Controls showInteractive={false} />

          {/* ── Top-right toolbar ──────────────────────────────────────── */}
          <Panel position="top-right">
            <Space direction="vertical" size={6} style={{ alignItems: "flex-end" }}>
              <Space size={6}>
                <Tooltip title={`Execute root task: ${rootName}`}>
                  <Button
                    type="primary"
                    icon={<CaretRightOutlined />}
                    loading={executing}
                    onClick={runGraph}
                    size="small"
                  >
                    Run Graph
                  </Button>
                </Tooltip>
                <Tooltip title={retryTooltip}>
                  <Button
                    danger
                    icon={<RedoOutlined />}
                    loading={retrying}
                    disabled={!canRetry}
                    onClick={retryFailed}
                    size="small"
                  >
                    Retry Failed
                  </Button>
                </Tooltip>
              </Space>
              {lastUpdatedLabel && (
                <Text
                  type="secondary"
                  style={{ fontSize: 10, display: "flex", alignItems: "center", gap: 4 }}
                >
                  <span
                    style={{
                      display: "inline-block",
                      width: 6,
                      height: 6,
                      borderRadius: "50%",
                      background: "#52c41a",
                      animation: "pulse 2s ease-in-out infinite",
                    }}
                  />
                  Live · {lastUpdatedLabel}
                </Text>
              )}
            </Space>
          </Panel>
        </ReactFlow>
      )}
    </Modal>
  );
}
