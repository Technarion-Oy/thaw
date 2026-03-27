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
import { Modal, Spin, Button, Space, Typography, Alert, Tag, message, Tooltip, Menu } from "antd";
import {
  CheckCircleOutlined, CloseCircleOutlined, SyncOutlined,
  MinusCircleOutlined, ClockCircleOutlined, ReloadOutlined,
  CaretRightOutlined, RedoOutlined,
  PauseCircleOutlined, PlayCircleOutlined,
  PlusOutlined, FlagOutlined, DeleteOutlined,
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
import { GetTaskStatuses, ExecuteTask, AlterTask, DropTaskTree, ExecDDL, SuspendTaskGraph, EnableTaskDependents, GetObjectDDL } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";
import CreateTaskModal from "./CreateTaskModal";

const { Text } = Typography;

const NODE_W = 200;
const NODE_H = 96;
const POLL_MS = 3_000;

// Module-level DDL cache — same 60 s TTL pattern used by SqlEditor hover.
const taskDDLCache = new Map<string, { ddl: string; ts: number }>();
const DDL_TTL_MS = 60_000;

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
// Snowflake does not create a TASK_HISTORY row for tasks that were skipped
// because a predecessor failed. This fixed-point walk infers SKIPPED so
// transitive chains also propagate (task_d → task_c → task_a=FAILED → all SKIPPED).
//
// ACTIVE_STATES: tasks actively running in the current graph run are never
// inferred as skipped. SUCCEEDED is intentionally excluded — it is always from
// a previous run when a predecessor is currently FAILED (Snowflake cannot
// schedule a task whose predecessor just failed in the same run).
//
// Timestamp guard: if this task's lastRunTime is strictly newer than the
// predecessor's failure time, the SUCCEEDED belongs to a more-recent graph run
// (e.g. the predecessor was fixed between runs) — don't override it.

const ACTIVE_STATES = new Set(["EXECUTING", "RUNNING", "SCHEDULED"]);

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
          // If our last completion is strictly newer than the predecessor's
          // failure we ran in a more-recent graph run — leave as-is.
          if (t.lastRunTime && pred.lastRunTime) {
            const tMs = new Date(t.lastRunTime).getTime();
            const pMs = new Date(pred.lastRunTime).getTime();
            if (!isNaN(tMs) && !isNaN(pMs) && tMs > pMs) continue;
          }
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

function buildLabel(t: main.TaskStatusRow, isRoot: boolean, overrideRunState?: string, isFinalizer?: boolean) {
  const started      = t.taskState?.toUpperCase() === "STARTED";
  const effectiveState = (overrideRunState ?? t.lastRunState ?? "").toUpperCase();
  // Show timestamp for terminal states; suppress for Waiting/never-run/executing.
  // Also suppress for SKIPPED: the lastRunTime comes from a previous succeeded
  // run (Snowflake creates no TASK_HISTORY row for skipped tasks), so showing
  // it would be misleading.
  const showTime     = !!t.lastRunTime &&
    effectiveState !== "WAITING" &&
    effectiveState !== "EXECUTING" &&
    effectiveState !== "RUNNING" &&
    effectiveState !== "SCHEDULED" &&
    effectiveState !== "SKIPPED" &&
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
        {isFinalizer && (
          <Tag
            icon={<FlagOutlined />}
            color="purple"
            style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}
          >
            Finalizer
          </Tag>
        )}
        {!isFinalizer && (
          <Tag
            color={started ? "success" : "default"}
            style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}
          >
            {t.taskState || "UNKNOWN"}
          </Tag>
        )}
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

function applyLayout(
  nodes: Node[],
  edges: Edge[],
  extraEdges?: Array<{ source: string; target: string }>,
): Node[] {
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({ rankdir: "LR", nodesep: 40, ranksep: 80 });
  nodes.forEach((n) => g.setNode(n.id, { width: NODE_W, height: NODE_H }));
  edges.forEach((e) => g.setEdge(e.source, e.target));
  extraEdges?.forEach((e) => g.setEdge(e.source, e.target));
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

  // Also include finalizer tasks that reference the root task.
  const finalizerUpperNames = new Set<string>();
  tasks.forEach((t) => {
    if (!t.finalize) return;
    const finUpper = extractName(t.finalize).toUpperCase();
    if (finUpper === rootUpper) {
      included.add(t.name.toUpperCase());
      finalizerUpperNames.add(t.name.toUpperCase());
    }
  });

  const focusedUpper = focusedName.toUpperCase();
  const rootTaskName = byName.get(rootUpper)?.name ?? focusedName;

  // Leaf nodes: included non-finalizer nodes that have no included children.
  // Used as layout-hint sources so Dagre pushes the finalizer to the far right.
  const leafUpperNames = new Set<string>();
  included.forEach((upper) => {
    if (finalizerUpperNames.has(upper)) return;
    const hasIncludedChild = (childrenOf.get(upper) ?? []).some((c) =>
      included.has(c.toUpperCase()),
    );
    if (!hasIncludedChild) leafUpperNames.add(upper);
  });

  const nodes: Node[] = [];
  const edges: Edge[] = [];
  const layoutOnlyEdges: Array<{ source: string; target: string }> = [];

  included.forEach((upper) => {
    const t = byName.get(upper);
    if (!t) return;

    const isRoot      = upper === rootUpper;
    const isFinalizer = finalizerUpperNames.has(upper);
    const isFocused   = upper === focusedUpper && upper !== rootUpper && !isFinalizer;

    nodes.push({
      id: t.name,
      position: { x: 0, y: 0 },
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        label: buildLabel(t, isRoot, skippedNodes.has(upper) ? "SKIPPED" : undefined, isFinalizer),
        isFinalizer,
      },
      style: {
        background: isFocused
          ? "var(--accent-bg, #1c3a5e)"
          : "var(--bg-overlay, #252526)",
        border: `1.5px solid ${
          isFinalizer          ? "#9254de"
          : isRoot || isFocused ? "var(--link, #4d9ef7)"
          : "var(--border, #444)"
        }`,
        borderStyle: isFinalizer ? "dashed" : "solid",
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

    if (!isFinalizer) {
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
    } else {
      // Dashed purple edge from root → finalizer.
      edges.push({
        id: `${rootTaskName}⟶finalizer:${t.name}`,
        source: rootTaskName,
        target: t.name,
        type: "smoothstep",
        label: "finalizes",
        labelStyle: { fontSize: 10, fill: "#9254de" },
        labelBgStyle: { fill: "var(--bg-overlay, #252526)", fillOpacity: 0.85 },
        markerEnd: { type: MarkerType.ArrowClosed, width: 14, height: 14, color: "#9254de" },
        style: { stroke: "#9254de", strokeWidth: 1.5, strokeDasharray: "5 3" },
      });
      // Invisible layout edges from every leaf → finalizer so Dagre places it
      // at the rightmost rank, after all leaf tasks.
      leafUpperNames.forEach((leafUpper) => {
        const leaf = byName.get(leafUpper);
        if (leaf) layoutOnlyEdges.push({ source: leaf.name, target: t.name });
      });
      // Always include a layout edge from root for single-task graphs.
      layoutOnlyEdges.push({ source: rootTaskName, target: t.name });
    }
  });

  return { nodes, edges, layoutOnlyEdges, rootTaskName, childrenOf };
}

// ── Component ─────────────────────────────────────────────────────────────────

export interface TaskGraphModalProps {
  db:       string;
  schema:   string;
  taskName: string;
  onClose:  () => void;
}

export default function TaskGraphModal({ db, schema, taskName, onClose }: TaskGraphModalProps) {
  const [loading,      setLoading]      = useState(true);
  const [loadError,    setLoadError]    = useState<string | null>(null);
  const [rootName,     setRootName]     = useState(taskName);
  const [executing,    setExecuting]    = useState(false);
  const [retrying,     setRetrying]     = useState(false);
  const [lastPollAt,   setLastPollAt]   = useState<Date | null>(null);
  const [taskRows,     setTaskRows]     = useState<main.TaskStatusRow[]>([]);
  const [togglingTask, setTogglingTask] = useState<string | null>(null);
  const [togglingAll,  setTogglingAll]  = useState(false);

  // DDL hover tooltip: fixed-position overlay shown while hovering a node.
  const [ddlTooltip, setDdlTooltip] = useState<{
    x: number; y: number; nodeId: string; ddl: string | null; // null = loading
  } | null>(null);
  const ddlHoverNode = useRef<string | null>(null); // tracks current hover to discard stale fetches

  // Right-click context menu state: viewport-relative position + target task info.
  const [ctxMenu, setCtxMenu] = useState<{
    x: number; y: number; name: string; taskState: string; isFinalizer: boolean;
  } | null>(null);

  // Create Task dialog opened from the graph (child or finalizer mode).
  const [createTaskDialog, setCreateTaskDialog] = useState<{
    mode: "child" | "finalizer"; taskName: string;
  } | null>(null);

  // Delete-all confirmation dialog.
  const [deleteAllConfirm, setDeleteAllConfirm] = useState(false);
  const [deletingTree, setDeletingTree] = useState(false);

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
        const { nodes: n, edges: e, layoutOnlyEdges, rootTaskName } = buildGraph(r.rows ?? [], taskName);
        rootNameRef.current  = rootTaskName;
        rootUpperRef.current = rootTaskName.toUpperCase();
        setRootName(rootTaskName);
        setNodes(applyLayout(n, e, layoutOnlyEdges));
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
              const isRoot        = n.id.toUpperCase() === rootUpper;
              const overrideState = skipped.has(n.id.toUpperCase()) ? "SKIPPED" : undefined;
              const isFinalizer   = !!(n.data as { isFinalizer?: boolean }).isFinalizer;
              return { ...n, data: { ...n.data, label: buildLabel(t, isRoot, overrideState, isFinalizer) } };
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

  // ── Suspend / Resume all tasks in the graph ───────────────────────────────
  const suspendResumeAll = useCallback(async () => {
    const rootRow = taskRowsRef.current.find(
      (t) => t.name.toUpperCase() === rootUpperRef.current,
    );
    const isStarted = rootRow?.taskState?.toUpperCase() === "STARTED";
    setTogglingAll(true);
    try {
      if (isStarted) {
        await SuspendTaskGraph(db, schema, rootNameRef.current);
        message.success(`Graph suspended: ${rootNameRef.current}`);
      } else {
        await EnableTaskDependents(db, schema, rootNameRef.current);
        message.success(`Graph resumed: ${rootNameRef.current}`);
      }
      load();
    } catch (err) {
      message.error(String(err));
    } finally {
      setTogglingAll(false);
    }
  }, [db, schema, load]);

  // ── Right-click context menu ──────────────────────────────────────────────
  const onNodeCtxMenu = useCallback((event: React.MouseEvent, node: Node) => {
    event.preventDefault();
    const t = taskRowsRef.current.find((r) => r.name.toUpperCase() === node.id.toUpperCase());
    if (!t) return;
    const isFinalizer = !!(node.data as { isFinalizer?: boolean }).isFinalizer;
    setCtxMenu({ x: event.clientX, y: event.clientY, name: t.name, taskState: t.taskState ?? "", isFinalizer });
  }, []);

  const onNodeMouseEnter = useCallback((event: React.MouseEvent, node: Node) => {
    const id = node.id;
    ddlHoverNode.current = id;
    const cached = taskDDLCache.get(id.toUpperCase());
    if (cached && Date.now() - cached.ts < DDL_TTL_MS) {
      setDdlTooltip({ x: event.clientX, y: event.clientY, nodeId: id, ddl: cached.ddl });
      return;
    }
    setDdlTooltip({ x: event.clientX, y: event.clientY, nodeId: id, ddl: null });
    GetObjectDDL(db, schema, "task", id, "")
      .then((ddl) => {
        taskDDLCache.set(id.toUpperCase(), { ddl, ts: Date.now() });
        if (ddlHoverNode.current === id) {
          setDdlTooltip((prev) => prev?.nodeId === id ? { ...prev, ddl } : prev);
        }
      })
      .catch(() => {
        if (ddlHoverNode.current === id) {
          setDdlTooltip((prev) => prev?.nodeId === id ? { ...prev, ddl: "" } : prev);
        }
      });
  }, [db, schema]);

  const onNodeMouseLeave = useCallback(() => {
    ddlHoverNode.current = null;
    setDdlTooltip(null);
  }, []);

  const toggleTask = useCallback(async (name: string, action: "SUSPEND" | "RESUME") => {
    setCtxMenu(null);
    setTogglingTask(name);
    try {
      await AlterTask(db, schema, name, action);
      message.success(`Task ${action === "SUSPEND" ? "suspended" : "resumed"}: ${name}`);
      // Optimistically update node label and taskRowsRef so the state badge
      // changes immediately without waiting for the next poll.
      const newState = action === "SUSPEND" ? "SUSPENDED" : "STARTED";
      const updateRow = (r: main.TaskStatusRow) =>
        r.name.toUpperCase() === name.toUpperCase() ? { ...r, taskState: newState } : r;
      taskRowsRef.current = taskRowsRef.current.map(updateRow);
      setTaskRows((prev) => prev.map(updateRow));
      setNodes((prev) =>
        prev.map((n) => {
          if (n.id.toUpperCase() !== name.toUpperCase()) return n;
          const t = taskRowsRef.current.find((r) => r.name.toUpperCase() === name.toUpperCase());
          if (!t) return n;
          const isRoot = n.id.toUpperCase() === rootUpperRef.current;
          return { ...n, data: { ...n.data, label: buildLabel(t, isRoot, undefined) } };
        })
      );
    } catch (err) {
      message.error(String(err));
    } finally {
      setTogglingTask(null);
    }
  }, [db, schema]);

  // ── Finalizer presence check ──────────────────────────────────────────────
  // True when the current graph already has a finalizer task, so the context
  // menu item "Add Finalizer Task…" on the root node should be disabled.
  const rootHasFinalizer = taskRows.some(
    (t) => t.finalize && extractName(t.finalize).toUpperCase() === rootUpperRef.current,
  );

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
  const canRetry     = graphRunFailed && within14Days;
  const retryTooltip = !graphRunFailed
    ? "Last graph run did not fail or get cancelled"
    : !within14Days
    ? "Last failed run was more than 14 days ago"
    : `Retry last failed run of ${rootName}`;

  // ── Root task state (for Suspend/Resume All button label) ────────────────
  const rootIsStarted = taskRows.find(
    (t) => t.name.toUpperCase() === rootUpperRef.current,
  )?.taskState?.toUpperCase() === "STARTED";

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
          onNodeContextMenu={onNodeCtxMenu}
          onNodeMouseEnter={onNodeMouseEnter}
          onNodeMouseLeave={onNodeMouseLeave}
          onPaneClick={() => setCtxMenu(null)}
          onNodeClick={() => setCtxMenu(null)}
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
                <Tooltip title={rootIsStarted ? "Suspend all tasks in graph" : "Resume all tasks in graph"}>
                  <Button
                    icon={rootIsStarted ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
                    loading={togglingAll}
                    size="small"
                    onClick={suspendResumeAll}
                  >
                    {rootIsStarted ? "Suspend All" : "Resume All"}
                  </Button>
                </Tooltip>
                <Tooltip title="Delete all tasks in this graph">
                  <Button
                    danger
                    icon={<DeleteOutlined />}
                    size="small"
                    onClick={() => setDeleteAllConfirm(true)}
                  >
                    Delete All
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

      {/* ── Create child / finalizer task dialog ────────────────────────── */}
      {createTaskDialog && (
        <CreateTaskModal
          db={db}
          schema={schema}
          mode={createTaskDialog.mode}
          predecessorTask={createTaskDialog.mode === "child" ? createTaskDialog.taskName : undefined}
          finalizerForTask={createTaskDialog.mode === "finalizer" ? createTaskDialog.taskName : undefined}
          onClose={() => setCreateTaskDialog(null)}
          onSuccess={load}
        />
      )}

      {/* ── Delete-all confirmation modal ─────────────────────────────────── */}
      {deleteAllConfirm && (() => {
        const escId = (s: string) => s.replace(/"/g, '""');
        const allNames = nodes.map((n) => n.id);
        return (
          <Modal
            open
            title={
              <Space>
                <DeleteOutlined style={{ color: "#ff4d4f" }} />
                <span>Delete All Tasks in Graph</span>
              </Space>
            }
            okText={`Delete ${allNames.length} task${allNames.length !== 1 ? "s" : ""}`}
            okButtonProps={{ danger: true, loading: deletingTree }}
            cancelButtonProps={{ disabled: deletingTree }}
            onCancel={() => !deletingTree && setDeleteAllConfirm(false)}
            onOk={async () => {
              setDeletingTree(true);
              try {
                // Drop finalizer nodes first (they have no children).
                const finalizerIds = nodes
                  .filter((n) => (n.data as { isFinalizer?: boolean }).isFinalizer)
                  .map((n) => n.id);
                for (const name of finalizerIds) {
                  try { await AlterTask(db, schema, name, "SUSPEND"); } catch { /* ignore */ }
                  await ExecDDL(
                    `DROP TASK IF EXISTS "${escId(db)}"."${escId(schema)}"."${escId(name)}"`,
                  );
                }
                // Drop root + all descendants in leaf-first order.
                await DropTaskTree(db, schema, rootNameRef.current);
                message.success(`Deleted ${allNames.length} task${allNames.length !== 1 ? "s" : ""}`);
                setDeleteAllConfirm(false);
                onClose();
              } catch (err) {
                message.error(String(err));
              } finally {
                setDeletingTree(false);
              }
            }}
          >
            <Text>
              The following tasks in <Text code>{db}.{schema}</Text> will be permanently dropped:
            </Text>
            <div style={{
              maxHeight: 180, overflowY: "auto", margin: "10px 0",
              border: "1px solid var(--border, #444)", borderRadius: 6, padding: "6px 10px",
            }}>
              {allNames.map((name) => (
                <div key={name} style={{ fontFamily: "monospace", fontSize: 12, padding: "2px 0" }}>
                  {name}
                </div>
              ))}
            </div>
            <Alert type="warning" showIcon message="This action cannot be undone." />
          </Modal>
        );
      })()}

      {/* ── DDL hover tooltip ────────────────────────────────────────────── */}
      {ddlTooltip && (
        <div
          style={{
            position: "fixed",
            left: ddlTooltip.x + 16,
            top: ddlTooltip.y + 16,
            zIndex: 9999,
            maxWidth: 520,
            maxHeight: 340,
            overflow: "auto",
            background: "var(--bg-overlay, #1e1e1e)",
            border: "1px solid var(--border, #555)",
            borderRadius: 6,
            padding: "8px 12px",
            boxShadow: "0 4px 20px rgba(0,0,0,0.5)",
            pointerEvents: "none",
          }}
        >
          {ddlTooltip.ddl === null ? (
            <Space size={6} style={{ color: "var(--text-faint)", fontSize: 12 }}>
              <Spin size="small" />
              <span>Loading DDL…</span>
            </Space>
          ) : ddlTooltip.ddl === "" ? (
            <Text type="secondary" style={{ fontSize: 12 }}>DDL unavailable</Text>
          ) : (
            <pre style={{
              margin: 0,
              fontSize: 11,
              fontFamily: "monospace",
              whiteSpace: "pre",
              color: "var(--text)",
              lineHeight: 1.5,
            }}>
              {ddlTooltip.ddl}
            </pre>
          )}
        </div>
      )}

      {/* ── Node right-click context menu ────────────────────────────────── */}

      {ctxMenu && (() => {
        const isStarted = ctxMenu.taskState.toUpperCase() === "STARTED";
        return (
          <>
            {/* Transparent overlay to dismiss the menu on click-away */}
            <div
              style={{ position: "fixed", inset: 0, zIndex: 998 }}
              onClick={() => setCtxMenu(null)}
            />
            <div style={{ position: "fixed", top: ctxMenu.y, left: ctxMenu.x, zIndex: 999 }}>
              <Menu
                style={{
                  minWidth: 180,
                  borderRadius: 6,
                  boxShadow: "0 4px 16px rgba(0,0,0,0.35)",
                  border: "1px solid var(--border, #444)",
                }}
                items={[
                  {
                    key: "label",
                    label: (
                      <span style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-faint)" }}>
                        {ctxMenu.name}
                      </span>
                    ),
                    disabled: true,
                  },
                  { type: "divider" as const },
                  isStarted
                    ? {
                        key: "suspend",
                        icon: <PauseCircleOutlined />,
                        label: togglingTask === ctxMenu.name ? "Suspending…" : "Suspend",
                        disabled: togglingTask === ctxMenu.name,
                        onClick: () => toggleTask(ctxMenu.name, "SUSPEND"),
                      }
                    : {
                        key: "resume",
                        icon: <PlayCircleOutlined />,
                        label: togglingTask === ctxMenu.name ? "Resuming…" : "Resume",
                        disabled: togglingTask === ctxMenu.name,
                        onClick: () => toggleTask(ctxMenu.name, "RESUME"),
                      },
                  { type: "divider" as const },
                  {
                    key: "add-child",
                    icon: <PlusOutlined />,
                    label: ctxMenu.isFinalizer
                      ? "Add Child Task… (not for finalizers)"
                      : "Add Child Task…",
                    disabled: ctxMenu.isFinalizer,
                    onClick: () => {
                      if (ctxMenu.isFinalizer) return;
                      setCreateTaskDialog({ mode: "child", taskName: ctxMenu.name });
                      setCtxMenu(null);
                    },
                  },
                  {
                    key: "add-finalizer",
                    icon: <FlagOutlined />,
                    label: ctxMenu.isFinalizer
                      ? "Add Finalizer Task… (not for finalizers)"
                      : ctxMenu.name.toUpperCase() !== rootUpperRef.current
                      ? "Add Finalizer Task… (root only)"
                      : rootHasFinalizer
                      ? "Add Finalizer Task… (already has one)"
                      : "Add Finalizer Task…",
                    disabled: ctxMenu.isFinalizer || ctxMenu.name.toUpperCase() !== rootUpperRef.current || rootHasFinalizer,
                    onClick: () => {
                      if (ctxMenu.isFinalizer || ctxMenu.name.toUpperCase() !== rootUpperRef.current || rootHasFinalizer) return;
                      setCreateTaskDialog({ mode: "finalizer", taskName: ctxMenu.name });
                      setCtxMenu(null);
                    },
                  },
                ]}
              />
            </div>
          </>
        );
      })()}
    </Modal>
  );
}
