// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import { useState, useEffect, useLayoutEffect, useCallback, useRef, useMemo } from "react";
import {
  ReactFlow,
  ReactFlowProvider,
  Background,
  Controls,
  MiniMap,
  Panel,
  useNodesState,
  useEdgesState,
  useReactFlow,
  type Node,
  type Edge,
  type OnConnect,
  type NodeChange,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { Button, Menu } from "antd";
import {
  AimOutlined,
  ClearOutlined,
  CopyOutlined,
  DeleteOutlined,
  LinkOutlined,
  DisconnectOutlined,
} from "@ant-design/icons";
import ERTableNode from "./ERTableNode";
import type { DesignerTable } from "./erTypes";
import { tablesToNodesAndEdges, applyERLayout } from "./erCanvasLayout";
import {
  loadERLayout,
  saveERLayout,
  flushERLayout,
  positionKey,
} from "./erLayoutStore";

// Module-level nodeTypes — XYFlow requires this to be stable across renders
const nodeTypes = { erTable: ERTableNode };

/** Vertical gap (px) between saved-position nodes and dagre-positioned new nodes. */
const DAGRE_OFFSET_GAP = 120;

// ── Context menu (extracted for readability / testability) ──────────────────

interface CtxMenuState {
  x: number;
  y: number;
  tableId: string;
  tableName: string;
  hasFKs: boolean;
}

function ERContextMenu({
  ctxMenu,
  selectedNodeIds,
  onClose,
  onDuplicateTable,
  onDeleteTable,
  onAddFK,
  onRemoveFKs,
}: {
  ctxMenu: CtxMenuState;
  selectedNodeIds: string[];
  onClose: () => void;
  onDuplicateTable?: (tableId: string) => void;
  onDeleteTable?: (tableId: string) => void;
  onAddFK?: (tableIdA: string, tableIdB: string) => void;
  onRemoveFKs?: (tableId: string) => void;
}) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ top: ctxMenu.y, left: ctxMenu.x });

  // Measure the menu after first paint and clamp to viewport
  useLayoutEffect(() => {
    const el = menuRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    setPos({
      top: Math.min(ctxMenu.y, window.innerHeight - rect.height - 8),
      left: Math.min(ctxMenu.x, window.innerWidth - rect.width - 8),
    });
  }, [ctxMenu.x, ctxMenu.y]);

  // Dismiss on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  const twoSelected = selectedNodeIds.length === 2 && selectedNodeIds.includes(ctxMenu.tableId);

  return (
    <>
      {/* Transparent overlay to dismiss on click-away */}
      <div
        style={{ position: "fixed", inset: 0, zIndex: 998 }}
        onClick={onClose}
      />
      <div ref={menuRef} style={{ position: "fixed", top: pos.top, left: pos.left, zIndex: 999 }}>
        <Menu
          style={{
            minWidth: 200,
            borderRadius: 6,
            boxShadow: "0 4px 16px rgba(0,0,0,0.35)",
            border: "1px solid var(--border)",
          }}
          items={[
            {
              key: "label",
              label: (
                <span style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}>
                  {ctxMenu.tableName}
                </span>
              ),
              disabled: true,
            },
            { type: "divider" as const },
            {
              key: "duplicate",
              icon: <CopyOutlined />,
              label: "Duplicate Table",
              onClick: () => {
                onDuplicateTable?.(ctxMenu.tableId);
                onClose();
              },
            },
            {
              key: "delete",
              icon: <DeleteOutlined />,
              danger: true,
              label: "Delete Table",
              onClick: () => {
                onDeleteTable?.(ctxMenu.tableId);
                onClose();
              },
            },
            { type: "divider" as const },
            {
              key: "add-fk",
              icon: <LinkOutlined />,
              label: twoSelected
                ? "Add FK Reference..."
                : "Add FK Reference... (select 2 tables)",
              disabled: !twoSelected,
              onClick: () => {
                const [idA, idB] = selectedNodeIds;
                onAddFK?.(idA, idB);
                onClose();
              },
            },
            {
              key: "remove-fks",
              icon: <DisconnectOutlined />,
              label: "Remove FK References",
              disabled: !ctxMenu.hasFKs,
              onClick: () => {
                onRemoveFKs?.(ctxMenu.tableId);
                onClose();
              },
            },
          ]}
        />
      </div>
    </>
  );
}

export interface ERCanvasProps {
  tables: DesignerTable[];
  mode: "edit" | "readonly";
  database: string;
  visibleSchemas?: Set<string>;
  selectedTableIds?: string[];
  onSelectionChange?: (ids: string[]) => void;
  onConnect?: (
    fromTableId: string,
    fromColId: string,
    toTableId: string,
    toColId: string,
  ) => void;
  onTableRename?: (tableId: string, newName: string) => void;
  onColumnRename?: (tableId: string, colId: string, newName: string) => void;
  onColumnRemove?: (tableId: string, colId: string) => void;
  onDuplicateTable?: (tableId: string) => void;
  onDeleteTable?: (tableId: string) => void;
  onAddFK?: (tableIdA: string, tableIdB: string) => void;
  onRemoveFKs?: (tableId: string) => void;
}

function ERCanvasInner({
  tables,
  mode,
  database,
  visibleSchemas,
  selectedTableIds,
  onSelectionChange: onSelectionChangeProp,
  onConnect: onConnectProp,
  onTableRename,
  onColumnRename,
  onColumnRemove,
  onDuplicateTable,
  onDeleteTable,
  onAddFK,
  onRemoveFKs,
}: ERCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([] as Node[]);
  const [edges, setEdges] = useEdgesState<Edge>([] as Edge[]);
  const { getNodes, getEdges, fitView } = useReactFlow();
  const initialLayoutDone = useRef(false);
  const prevTableIds = useRef<string>("");

  // Stable refs — avoids re-running effects / recreating callbacks when
  // prop identity changes (e.g. parent re-renders)
  const callbackRefs = useRef({ onTableRename, onColumnRename, onColumnRemove });
  callbackRefs.current = { onTableRename, onColumnRename, onColumnRemove };
  const tablesRef = useRef(tables);
  tablesRef.current = tables;

  // Flush any pending debounced position save on unmount
  useEffect(() => {
    return () => flushERLayout(database);
  }, [database]);

  // Filter tables by visible schemas if provided
  const filteredTables = useMemo(() => {
    if (!visibleSchemas) return tables;
    return tables.filter((t) => visibleSchemas.has(t.schema));
  }, [tables, visibleSchemas]);

  // Memoized table lookup — reused by layout effect, context menu, etc.
  const filteredTableById = useMemo(
    () => new Map(filteredTables.map((t) => [t.id, t])),
    [filteredTables],
  );

  // Stable, sorted ID string for detecting table set changes (add/remove).
  // Memoized separately so the O(n log n) sort only runs when filteredTables changes,
  // not on every effect invocation.
  const filteredTableIdStr = useMemo(
    () => filteredTables.map((t) => t.id).sort().join(","),
    [filteredTables],
  );

  // Rebuild nodes/edges when tables change
  useEffect(() => {
    const { nodes: newNodes, edges: newEdges } = tablesToNodesAndEdges(
      filteredTables,
      mode,
      {
        onTableRename: callbackRefs.current.onTableRename,
        onColumnRename: callbackRefs.current.onColumnRename,
        onColumnRemove: callbackRefs.current.onColumnRemove,
      },
    );

    // Determine if table set has changed (new/removed tables)
    const tableSetChanged = filteredTableIdStr !== prevTableIds.current;
    prevTableIds.current = filteredTableIdStr;

    if (!initialLayoutDone.current || tableSetChanged) {
      // Apply saved positions or dagre layout
      const saved = loadERLayout(database);
      let positioned = newNodes;

      if (saved) {
        // Apply saved positions to nodes that have them, mark others for dagre
        const needsLayout = new Set<string>();
        positioned = newNodes.map((n) => {
          const table = filteredTableById.get(n.id);
          if (!table) return n;
          const key = positionKey(table.schema, table.name);
          const pos = saved[key];
          if (pos) {
            return { ...n, position: pos };
          }
          needsLayout.add(n.id);
          return n;
        });

        // Apply dagre only to nodes without saved positions
        if (needsLayout.size > 0) {
          const onlyNew = positioned.filter((n) => needsLayout.has(n.id));
          const laid = applyERLayout(onlyNew, newEdges);

          // Offset dagre output below saved-position nodes to avoid overlap
          const savedNodes = positioned.filter((n) => !needsLayout.has(n.id));
          if (savedNodes.length > 0) {
            let maxBottom = -Infinity;
            for (const n of savedNodes) {
              const bottom = n.position.y + (n.height ?? 200);
              if (bottom > maxBottom) maxBottom = bottom;
            }
            const dagreMinY = Math.min(...laid.map((n) => n.position.y));
            const offsetY = maxBottom + DAGRE_OFFSET_GAP - dagreMinY;
            for (const n of laid) {
              n.position = { x: n.position.x, y: n.position.y + offsetY };
            }
          }

          const laidMap = new Map(laid.map((n) => [n.id, n.position]));
          positioned = positioned.map((n) =>
            laidMap.has(n.id) ? { ...n, position: laidMap.get(n.id)! } : n,
          );
        }
      } else {
        positioned = applyERLayout(newNodes, newEdges);
      }

      setNodes(positioned);
      initialLayoutDone.current = true;

      // Ensure newly laid-out nodes are visible in the viewport.
      // requestAnimationFrame guarantees a paint cycle has occurred so
      // XYFlow has measured the new nodes before fitView runs.
      if (tableSetChanged) {
        requestAnimationFrame(() => fitView({ padding: 0.15 }));
      }
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
  }, [filteredTables, filteredTableById, filteredTableIdStr, mode, database, setNodes, setEdges, fitView]);

  // Sync parent selectedTableIds to XYFlow node.selected.
  // Compares against current node selection to avoid unnecessary updates
  // and prevent sync loops with handleSelectionChange.
  useEffect(() => {
    const desiredSet = new Set(selectedTableIds ?? []);
    const currentNodes = getNodes();
    const alreadyMatches = currentNodes.every(
      (n) => (n.selected ?? false) === desiredSet.has(n.id),
    );
    if (alreadyMatches) return;
    setNodes((prev) =>
      prev.map((n) => ({
        ...n,
        selected: desiredSet.has(n.id),
      })),
    );
  }, [selectedTableIds, setNodes, getNodes]);

  // Propagate XYFlow selection changes (Cmd/Ctrl+click multi-select) to parent
  const handleSelectionChange = useCallback(
    ({ nodes: selectedNodes }: { nodes: Node[] }) => {
      onSelectionChangeProp?.(selectedNodes.map((n) => n.id));
    },
    [onSelectionChangeProp],
  );

  // Track position changes and persist
  const handleNodesChange = useCallback(
    (changes: NodeChange[]) => {
      onNodesChange(changes);

      // Check if any position changes occurred (only completed drags, not in-progress)
      const hasPositionChange = changes.some(
        (c) => c.type === "position" && c.dragging === false,
      );
      if (!hasPositionChange) return;

      // Merge with existing saved positions so filtered-out schemas are preserved
      const currentNodes = getNodes();
      const tableById = new Map(tablesRef.current.map((t) => [t.id, t]));
      const positions = loadERLayout(database) ?? {};
      for (const n of currentNodes) {
        const table = tableById.get(n.id);
        if (table && table.schema && table.name.trim()) {
          positions[positionKey(table.schema, table.name)] = n.position;
        }
      }
      saveERLayout(database, positions);
    },
    [onNodesChange, database, getNodes],
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
      const srcPrefix = "col-source-";
      const tgtPrefix = "col-target-";
      if (!sourceHandle.startsWith(srcPrefix) || !targetHandle.startsWith(tgtPrefix)) return;
      const fromColId = sourceHandle.slice(srcPrefix.length);
      const toColId = targetHandle.slice(tgtPrefix.length);

      onConnectProp(source, fromColId, target, toColId);
    },
    [onConnectProp],
  );

  const handleAutoLayout = useCallback(() => {
    const currentEdges = getEdges();
    const currentNodes = getNodes();
    const tableById = new Map(tablesRef.current.map((t) => [t.id, t]));
    const laid = applyERLayout(currentNodes, currentEdges);
    setNodes(laid);
    // Merge new positions with saved positions for filtered-out schemas
    const positions = loadERLayout(database) ?? {};
    for (const n of laid) {
      const table = tableById.get(n.id);
      if (table && table.schema && table.name.trim()) {
        positions[positionKey(table.schema, table.name)] = n.position;
      }
    }
    saveERLayout(database, positions);
  }, [database, setNodes, getNodes, getEdges]);

  const handleResetLayout = useCallback(() => {
    initialLayoutDone.current = false;
    // Force re-layout
    const { nodes: newNodes, edges: newEdges } = tablesToNodesAndEdges(
      filteredTables,
      mode,
      {
        onTableRename: callbackRefs.current.onTableRename,
        onColumnRename: callbackRefs.current.onColumnRename,
        onColumnRemove: callbackRefs.current.onColumnRemove,
      },
    );
    const laid = applyERLayout(newNodes, newEdges);

    // Remove positions only for currently visible tables, preserve others
    const visibleKeys = new Set(
      filteredTables
        .filter((t) => t.schema && t.name.trim())
        .map((t) => positionKey(t.schema, t.name)),
    );
    const saved = loadERLayout(database) ?? {};
    const preserved: Record<string, { x: number; y: number }> = {};
    for (const [k, v] of Object.entries(saved)) {
      if (!visibleKeys.has(k)) preserved[k] = v;
    }
    // Add new dagre positions for visible tables
    for (const n of laid) {
      const table = filteredTableById.get(n.id);
      if (table && table.schema && table.name.trim()) {
        preserved[positionKey(table.schema, table.name)] = n.position;
      }
    }
    saveERLayout(database, preserved);

    setNodes(laid);
    setEdges(newEdges);
    initialLayoutDone.current = true;
  }, [database, filteredTables, filteredTableById, mode, setNodes, setEdges]);

  // Close the context menu without affecting selection — used by menu item
  // actions so the action's own selection update isn't overwritten.
  const closeContextMenu = useCallback(() => {
    setCtxMenu(null);
  }, []);

  // Pane click: close context menu AND clear selection (pane click doesn't
  // trigger XYFlow's onSelectionChange, so we propagate deselection explicitly).
  const handlePaneClick = useCallback(() => {
    setCtxMenu(null);
    onSelectionChangeProp?.([]);
  }, [onSelectionChangeProp]);

  // ── Context menu ─────────────────────────────────────────────────────────
  const [ctxMenu, setCtxMenu] = useState<CtxMenuState | null>(null);

  const handleNodeContextMenu = useCallback(
    (event: React.MouseEvent, node: Node) => {
      if (mode !== "edit") return;
      event.preventDefault();
      const table = filteredTableById.get(node.id);
      if (!table) return;
      const hasFKs = table.columns.some((c) => c.fkRef);
      setCtxMenu({
        x: event.clientX,
        y: event.clientY,
        tableId: table.id,
        tableName: table.schema ? `${table.schema}.${table.name || "(unnamed)"}` : table.name || "(unnamed)",
        hasFKs,
      });
    },
    [mode, filteredTableById],
  );

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
          background: "var(--bg-elevated)",
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
        onEdgesChange={undefined} /* edges are derived from fkRef — read-only */
        onConnect={mode === "edit" ? handleConnect : undefined}
        onNodeClick={() => setCtxMenu(null)}
        onPaneClick={handlePaneClick}
        onNodeContextMenu={mode === "edit" ? handleNodeContextMenu : undefined}
        onSelectionChange={handleSelectionChange}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        nodesDraggable={mode === "edit"}
        nodesConnectable={mode === "edit"}
        deleteKeyCode={null}
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

      {/* ── Node right-click context menu ──────────────────────────────── */}
      {ctxMenu && (
        <ERContextMenu
          ctxMenu={ctxMenu}
          selectedNodeIds={selectedTableIds ?? []}
          onClose={closeContextMenu}
          onDuplicateTable={onDuplicateTable}
          onDeleteTable={onDeleteTable}
          onAddFK={onAddFK}
          onRemoveFKs={onRemoveFKs}
        />
      )}
    </div>
  );
}

export default function ERCanvas(props: ERCanvasProps) {
  return (
    <ReactFlowProvider>
      <ERCanvasInner {...props} />
    </ReactFlowProvider>
  );
}
