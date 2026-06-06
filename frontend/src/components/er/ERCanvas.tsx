// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import { useEffect, useCallback, useRef, useMemo } from "react";
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  Panel,
  useNodesState,
  useEdgesState,
  type Node,
  type Edge,
  type OnConnect,
  type NodeChange,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Button } from "antd";
import { AimOutlined, ClearOutlined } from "@ant-design/icons";
import ERTableNode from "./ERTableNode";
import type { DesignerTable } from "./erTypes";
import { tablesToNodesAndEdges, applyERLayout } from "./erCanvasLayout";
import {
  loadERLayout,
  saveERLayout,
  clearERLayout,
  positionKey,
} from "./erLayoutStore";

// Module-level nodeTypes — XYFlow requires this to be stable across renders
const nodeTypes = { erTable: ERTableNode };

export interface ERCanvasProps {
  tables: DesignerTable[];
  mode: "edit" | "readonly";
  database: string;
  visibleSchemas?: Set<string>;
  selectedTableId?: string | null;
  onTableSelect?: (id: string | null) => void;
  onConnect?: (
    fromTableId: string,
    fromColId: string,
    toTableId: string,
    toColId: string,
  ) => void;
  onTableRename?: (tableId: string, newName: string) => void;
  onColumnRename?: (tableId: string, colId: string, newName: string) => void;
}

export default function ERCanvas({
  tables,
  mode,
  database,
  visibleSchemas,
  selectedTableId,
  onTableSelect,
  onConnect: onConnectProp,
  onTableRename,
  onColumnRename,
}: ERCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([] as Node[]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([] as Edge[]);
  const initialLayoutDone = useRef(false);
  const prevTableIds = useRef<string>("");

  // Callbacks for node interactions
  const handleHeaderDoubleClick = useCallback(
    (tableId: string) => {
      if (!onTableRename) return;
      const table = tables.find((t) => t.id === tableId);
      if (!table) return;
      const newName = prompt("Rename table:", table.name);
      if (newName !== null && newName.trim()) {
        onTableRename(tableId, newName.trim().toUpperCase());
      }
    },
    [tables, onTableRename],
  );

  const handleColumnDoubleClick = useCallback(
    (tableId: string, colId: string) => {
      if (!onColumnRename) return;
      const table = tables.find((t) => t.id === tableId);
      if (!table) return;
      const col = table.columns.find((c) => c.id === colId);
      if (!col) return;
      const newName = prompt("Rename column:", col.name);
      if (newName !== null && newName.trim()) {
        onColumnRename(tableId, colId, newName.trim());
      }
    },
    [tables, onColumnRename],
  );

  // Filter tables by visible schemas if provided
  const filteredTables = useMemo(() => {
    if (!visibleSchemas) return tables;
    return tables.filter((t) => visibleSchemas.has(t.schema));
  }, [tables, visibleSchemas]);

  // Rebuild nodes/edges when tables change
  useEffect(() => {
    const { nodes: newNodes, edges: newEdges } = tablesToNodesAndEdges(
      filteredTables,
      mode,
      selectedTableId,
      {
        onHeaderDoubleClick: handleHeaderDoubleClick,
        onColumnDoubleClick: handleColumnDoubleClick,
      },
    );

    // Determine if table set has changed (new/removed tables)
    const currentIds = filteredTables
      .map((t) => t.id)
      .sort()
      .join(",");
    const tableSetChanged = currentIds !== prevTableIds.current;
    prevTableIds.current = currentIds;

    if (!initialLayoutDone.current || tableSetChanged) {
      // Apply saved positions or dagre layout
      const saved = loadERLayout(database);
      let positioned = newNodes;

      if (saved) {
        let allFound = true;
        positioned = newNodes.map((n) => {
          const table = filteredTables.find((t) => t.id === n.id);
          if (!table) return n;
          const key = positionKey(table.schema, table.name);
          const pos = saved[key];
          if (pos) {
            return { ...n, position: pos };
          }
          allFound = false;
          return n;
        });

        // If some nodes don't have saved positions, apply dagre to all
        if (!allFound) {
          positioned = applyERLayout(positioned, newEdges);
        }
      } else {
        positioned = applyERLayout(newNodes, newEdges);
      }

      setNodes(positioned);
      initialLayoutDone.current = true;
    } else {
      // Preserve current positions, update data only
      setNodes((prev) => {
        const posMap = new Map(prev.map((n) => [n.id, n.position]));
        return newNodes.map((n) => ({
          ...n,
          position: posMap.get(n.id) ?? n.position,
        }));
      });
    }

    setEdges(newEdges);
  }, [filteredTables, mode, selectedTableId, database, handleHeaderDoubleClick, handleColumnDoubleClick, setNodes, setEdges]);

  // Track position changes and persist
  const handleNodesChange = useCallback(
    (changes: NodeChange[]) => {
      onNodesChange(changes);

      // Check if any position changes occurred
      const hasPositionChange = changes.some(
        (c) => c.type === "position" && c.position,
      );
      if (!hasPositionChange) return;

      // Debounced save — read current node positions after React state updates
      setTimeout(() => {
        setNodes((currentNodes) => {
          const positions: Record<string, { x: number; y: number }> = {};
          for (const n of currentNodes) {
            const table = tables.find((t) => t.id === n.id);
            if (table && table.schema && table.name.trim()) {
              positions[positionKey(table.schema, table.name)] = n.position;
            }
          }
          saveERLayout(database, positions);
          return currentNodes; // no-op update, just reading state
        });
      }, 0);
    },
    [onNodesChange, tables, database, setNodes],
  );

  // Handle new connections (FK creation via drag)
  const handleConnect: OnConnect = useCallback(
    (connection) => {
      if (!onConnectProp) return;
      const { source, target, sourceHandle, targetHandle } = connection;
      if (!source || !target || !sourceHandle || !targetHandle) return;

      // Prevent self-FK
      if (source === target) return;

      // Parse handle IDs: "col-source-{colId}" / "col-target-{colId}"
      const fromColId = sourceHandle.replace("col-source-", "");
      const toColId = targetHandle.replace("col-target-", "");

      onConnectProp(source, fromColId, target, toColId);
    },
    [onConnectProp],
  );

  const handleAutoLayout = useCallback(() => {
    setNodes((prev) => {
      const laid = applyERLayout(prev, edges);
      // Save new positions
      const positions: Record<string, { x: number; y: number }> = {};
      for (const n of laid) {
        const table = tables.find((t) => t.id === n.id);
        if (table && table.schema && table.name.trim()) {
          positions[positionKey(table.schema, table.name)] = n.position;
        }
      }
      saveERLayout(database, positions);
      return laid;
    });
  }, [edges, tables, database, setNodes]);

  const handleResetLayout = useCallback(() => {
    clearERLayout(database);
    initialLayoutDone.current = false;
    // Force re-layout
    const { nodes: newNodes, edges: newEdges } = tablesToNodesAndEdges(
      filteredTables,
      mode,
      selectedTableId,
      {
        onHeaderDoubleClick: handleHeaderDoubleClick,
        onColumnDoubleClick: handleColumnDoubleClick,
      },
    );
    const laid = applyERLayout(newNodes, newEdges);
    setNodes(laid);
    setEdges(newEdges);
    initialLayoutDone.current = true;
  }, [database, filteredTables, mode, selectedTableId, handleHeaderDoubleClick, handleColumnDoubleClick, setNodes, setEdges]);

  const handleNodeClick = useCallback(
    (_: React.MouseEvent, node: Node) => {
      onTableSelect?.(node.id);
    },
    [onTableSelect],
  );

  const handlePaneClick = useCallback(() => {
    onTableSelect?.(null);
  }, [onTableSelect]);

  if (filteredTables.length === 0) {
    return (
      <div
        style={{
          flex: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          color: "var(--text-muted)",
          fontSize: 13,
          background: "var(--bg)",
        }}
      >
        {tables.length === 0
          ? "Add tables and columns to see the diagram."
          : "No tables match the current filter."}
      </div>
    );
  }

  return (
    <div style={{ flex: 1, width: "100%", height: "100%" }}>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={handleNodesChange}
        onEdgesChange={onEdgesChange}
        onConnect={mode === "edit" ? handleConnect : undefined}
        onNodeClick={handleNodeClick}
        onPaneClick={handlePaneClick}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        nodesDraggable
        nodesConnectable={mode === "edit"}
        proOptions={{ hideAttribution: true }}
        style={{ background: "var(--bg)" }}
      >
        <Background color="var(--border)" gap={20} />
        <Controls showInteractive={false} />
        <MiniMap
          nodeColor="var(--bg-overlay)"
          maskColor="rgba(0, 0, 0, 0.6)"
          style={{ background: "var(--bg-elevated)" }}
        />
        <Panel position="top-right">
          <div style={{ display: "flex", gap: 4 }}>
            <Button
              size="small"
              icon={<AimOutlined />}
              onClick={handleAutoLayout}
            >
              Auto Layout
            </Button>
            <Button
              size="small"
              icon={<ClearOutlined />}
              onClick={handleResetLayout}
            >
              Reset Layout
            </Button>
          </div>
        </Panel>
      </ReactFlow>
    </div>
  );
}
