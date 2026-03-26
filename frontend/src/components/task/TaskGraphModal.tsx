// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useCallback } from "react";
import { Modal, Spin, Button, Space, Typography, Alert, Tag } from "antd";
import { ClockCircleOutlined, ReloadOutlined } from "@ant-design/icons";
import {
  ReactFlow,
  Background,
  Controls,
  useNodesState,
  useEdgesState,
  Position,
  MarkerType,
  type Node,
  type Edge,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import dagre from "@dagrejs/dagre";
import { GetTaskStatuses } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

const { Text } = Typography;

const NODE_W = 200;
const NODE_H = 64;

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

  // parent-of and children-of maps (UPPER-CASED keys)
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

  // Walk up from the focused task to find the root of this graph.
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

  const focusedUpper  = focusedName.toUpperCase();
  const rootTaskName  = byName.get(rootUpper)?.name ?? focusedName;

  const nodes: Node[] = [];
  const edges: Edge[] = [];

  included.forEach((upper) => {
    const t = byName.get(upper);
    if (!t) return;

    const isRoot    = upper === rootUpper;
    const isFocused = upper === focusedUpper && upper !== rootUpper;
    const started   = t.taskState?.toUpperCase() === "STARTED";

    nodes.push({
      id: t.name,
      position: { x: 0, y: 0 }, // overwritten by dagre
      sourcePosition: Position.Right,
      targetPosition: Position.Left,
      data: {
        label: (
          <div style={{ textAlign: "center", lineHeight: 1.3 }}>
            <div style={{
              fontFamily: "monospace", fontSize: 11,
              fontWeight: isRoot ? 700 : 400,
              color: "var(--text)",
              marginBottom: 5,
              wordBreak: "break-all",
            }}>
              {t.name}
            </div>
            <Tag
              color={started ? "success" : "default"}
              style={{ fontSize: 10, margin: 0, lineHeight: 1.6 }}
            >
              {t.taskState || "UNKNOWN"}
            </Tag>
          </div>
        ),
      },
      style: {
        background: isFocused
          ? "var(--accent-bg, #1c3a5e)"
          : "var(--bg-overlay, #252526)",
        border: `1.5px solid ${
          isRoot    ? "var(--link, #4d9ef7)"
          : isFocused ? "var(--link, #4d9ef7)"
          : "var(--border, #444)"
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

    // One edge per child
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

  return { nodes, edges, rootTaskName };
}

// ── Component ─────────────────────────────────────────────────────────────────

export interface TaskGraphModalProps {
  db:       string;
  schema:   string;
  taskName: string;
  onClose:  () => void;
}

export default function TaskGraphModal({ db, schema, taskName, onClose }: TaskGraphModalProps) {
  const [loading,   setLoading]   = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [rootName,  setRootName]  = useState(taskName);

  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);

  const load = useCallback(() => {
    setLoading(true);
    setLoadError(null);
    GetTaskStatuses(db, schema)
      .then((r) => {
        const { nodes: n, edges: e, rootTaskName } = buildGraph(r.rows ?? [], taskName);
        setRootName(rootTaskName);
        setNodes(applyLayout(n, e));
        setEdges(e);
      })
      .catch((err) => setLoadError(String(err)))
      .finally(() => setLoading(false));
  }, [db, schema, taskName]);

  useEffect(() => { load(); }, [load]);

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
        </ReactFlow>
      )}
    </Modal>
  );
}
