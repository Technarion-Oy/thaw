// SPDX-License-Identifier: GPL-3.0-or-later
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
  BuildOutlined,
  BorderOuterOutlined,
} from "@ant-design/icons";
import ERTableNode from "./ERTableNode";
import type { DesignerTable } from "./erTypes";
import { tablesToNodesAndEdges, applyERLayout, nodeHeight } from "./erCanvasLayout";
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

// ── MiniMap visibility preference ───────────────────────────────────────────
// The MiniMap renders a second scaled copy of every node in the DOM, which is a
// meaningful memory cost on wide schemas (see issue #821). It defaults on to
// preserve existing behaviour, but the toggle lets memory-conscious users on
// Windows/WebView2 drop that second copy. Persisted globally (a display
// preference, not per-database).
const MINIMAP_PREF_KEY = "thaw-er-minimap";

function loadMinimapPref(): boolean {
  try {
    // Absent → default on.
    return localStorage.getItem(MINIMAP_PREF_KEY) !== "0";
  } catch {
    return true;
  }
}

function saveMinimapPref(show: boolean): void {
  try {
    localStorage.setItem(MINIMAP_PREF_KEY, show ? "1" : "0");
  } catch {
    // localStorage full or unavailable — silently ignore
  }
}

// ── Context menu shell (shared positioning / dismiss logic) ─────────────────

interface CtxMenuState {
  x: number;
  y: number;
  tableId: string;
  tableName: string;
  hasFKs: boolean;
}

/**
 * Shared wrapper for context menus — handles viewport clamping, Escape key
 * dismissal, wheel-to-close, and a transparent click-away overlay.
 */
function ContextMenuShell({
  x,
  y,
  onClose,
  canvasRef,
  children,
}: {
  x: number;
  y: number;
  onClose: () => void;
  canvasRef: React.RefObject<HTMLDivElement | null>;
  children: React.ReactNode;
}) {
  const menuRef = useRef<HTMLDivElement>(null);
  const [pos, setPos] = useState({ top: y, left: x });
  const [visible, setVisible] = useState(false);

  // Measure the menu after first paint and clamp to viewport.
  // Starts hidden to prevent a flash at the unclamped position.
  useLayoutEffect(() => {
    const el = menuRef.current;
    if (!el) return;
    const rect = el.getBoundingClientRect();
    setPos({
      top: Math.min(y, window.innerHeight - rect.height - 8),
      left: Math.min(x, window.innerWidth - rect.width - 8),
    });
    setVisible(true);
  }, [x, y]);

  // Dismiss on Escape key or scroll on the canvas (prevents the menu from
  // floating over a panned canvas). Scoped to the canvas container so that
  // scrolling the sidebar or other areas doesn't dismiss the menu.
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === "Escape") onClose();
    };
    const handleWheel = () => onClose();
    const canvas = canvasRef.current;
    window.addEventListener("keydown", handleKeyDown);
    canvas?.addEventListener("wheel", handleWheel, { passive: true });
    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      canvas?.removeEventListener("wheel", handleWheel);
    };
  }, [onClose, canvasRef]);

  return (
    <>
      {/* Transparent overlay to dismiss on click-away */}
      <div
        style={{ position: "fixed", inset: 0, zIndex: 998 }}
        onClick={onClose}
      />
      <div ref={menuRef} style={{ position: "fixed", top: pos.top, left: pos.left, zIndex: 999, visibility: visible ? "visible" : "hidden" }}>
        {children}
      </div>
    </>
  );
}

const menuStyle = {
  minWidth: 200,
  borderRadius: 6,
  boxShadow: "0 4px 16px rgba(0,0,0,0.35)",
  border: "1px solid var(--border)",
};

// ── Edit context menu ────────────────────────────────────────────────────────

function ERContextMenu({
  ctxMenu,
  selectedNodeIds,
  onClose,
  onDuplicateTable,
  onDeleteTable,
  onAddFK,
  onRemoveFKs,
  canvasRef,
}: {
  ctxMenu: CtxMenuState;
  selectedNodeIds: string[];
  onClose: () => void;
  onDuplicateTable?: (tableId: string) => void;
  onDeleteTable?: (tableId: string) => void;
  onAddFK?: (tableIdA: string, tableIdB: string) => void;
  onRemoveFKs?: (tableId: string) => void;
  canvasRef: React.RefObject<HTMLDivElement | null>;
}) {
  const twoSelected = selectedNodeIds.length === 2 && selectedNodeIds.includes(ctxMenu.tableId);

  return (
    <ContextMenuShell x={ctxMenu.x} y={ctxMenu.y} onClose={onClose} canvasRef={canvasRef}>
      <Menu
        style={menuStyle}
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
    </ContextMenuShell>
  );
}

// ── Readonly context menu (Build Query only) ────────────────────────────────

function ERReadonlyContextMenu({
  ctxMenu,
  selectedCount,
  onClose,
  onBuildQuery,
  canvasRef,
}: {
  ctxMenu: CtxMenuState;
  selectedCount: number;
  onClose: () => void;
  onBuildQuery: () => void;
  canvasRef: React.RefObject<HTMLDivElement | null>;
}) {
  const canBuild = selectedCount >= 2;

  return (
    <ContextMenuShell x={ctxMenu.x} y={ctxMenu.y} onClose={onClose} canvasRef={canvasRef}>
      <Menu
        style={menuStyle}
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
            key: "build-query",
            icon: <BuildOutlined />,
            label: canBuild
              ? "Build Query"
              : "Build Query (select 2+ tables)",
            disabled: !canBuild,
            onClick: () => {
              onBuildQuery();
              onClose();
            },
          },
        ]}
      />
    </ContextMenuShell>
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
  onBuildQuery?: (tableIds: string[]) => void;
  highlightedEdgeIds?: Set<string>;
  highlightedNodeIds?: Set<string>;
  /** Table node ids changed by the most recent MCP/AI modification — rendered
   *  with the `er-ai-changed` class so the latest AI change stands out. */
  changedNodeIds?: Set<string>;
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
  onBuildQuery,
  highlightedEdgeIds,
  highlightedNodeIds,
  changedNodeIds,
}: ERCanvasProps) {
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([] as Node[]);
  const [edges, setEdges] = useEdgesState<Edge>([] as Edge[]);
  const { getNodes, getEdges, fitView } = useReactFlow();
  const initialLayoutDone = useRef(false);
  const prevTableIds = useRef<string>("");
  const canvasRef = useRef<HTMLDivElement>(null);

  // Stable refs — avoids re-running effects / recreating callbacks when
  // prop identity changes (e.g. parent re-renders)
  const callbackRefs = useRef({ onTableRename, onColumnRename, onColumnRemove });
  callbackRefs.current = { onTableRename, onColumnRename, onColumnRemove };
  const tablesRef = useRef(tables);
  tablesRef.current = tables;
  // Track the last selection propagated to/from the parent to avoid
  // unnecessary re-renders and sync loops.
  const lastSelectionRef = useRef<string[]>([]);

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
          if (laid.length > 0 && savedNodes.length > 0) {
            let maxBottom = -Infinity;
            for (const n of savedNodes) {
              const table = filteredTableById.get(n.id);
              const h = n.height ?? (table ? nodeHeight(table.columns.length) : 200);
              const bottom = n.position.y + h;
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

  // Apply edge highlighting via setEdges only when the highlighted set
  // changes — avoids a full .map() on every drag/position update (XYFlow
  // recalculates edge routing on node moves, triggering edges updates).
  const prevHighlightEdgeRef = useRef<Set<string> | undefined>(undefined);
  useEffect(() => {
    if (prevHighlightEdgeRef.current === highlightedEdgeIds) return;
    prevHighlightEdgeRef.current = highlightedEdgeIds;
    setEdges((es) =>
      es.map((e) => {
        if (highlightedEdgeIds?.has(e.id)) {
          return {
            ...e,
            animated: false,
            style: { ...e.style, strokeWidth: 3, stroke: "var(--accent)" },
          };
        }
        return { ...e, animated: true, style: { ...e.style, strokeWidth: 1.5 } };
      }),
    );
  }, [highlightedEdgeIds, setEdges]);

  // Apply node classNames (join-path intermediate + latest-AI-change) via
  // setNodes only when either set changes, avoiding a full .map() on every
  // drag/position update. The two mechanisms are independent — `highlightedNodeIds`
  // (readonly join builder) and `changedNodeIds` (edit-mode MCP highlight) — but
  // are combined here so a node can carry both classes without one clobbering
  // the other.
  const prevNodeClassRef = useRef<{ h?: Set<string>; c?: Set<string> }>({});
  useEffect(() => {
    if (prevNodeClassRef.current.h === highlightedNodeIds && prevNodeClassRef.current.c === changedNodeIds) return;
    prevNodeClassRef.current = { h: highlightedNodeIds, c: changedNodeIds };
    setNodes((ns) =>
      ns.map((n) => {
        const classes: string[] = [];
        if (highlightedNodeIds?.has(n.id)) classes.push("er-intermediate");
        if (changedNodeIds?.has(n.id)) classes.push("er-ai-changed");
        const className = classes.length ? classes.join(" ") : undefined;
        return n.className === className ? n : { ...n, className };
      }),
    );
  }, [highlightedNodeIds, changedNodeIds, setNodes]);

  // Sync parent selectedTableIds to XYFlow node.selected.
  // Uses lastSelectionRef to detect whether this update was already propagated
  // (by handleSelectionChange) and skip the redundant setNodes call.
  useEffect(() => {
    const incoming = [...(selectedTableIds ?? [])].sort();
    const last = lastSelectionRef.current;
    // Shallow equality — skip if selection hasn't actually changed.
    // Both sides are sorted so order differences don't cause spurious updates.
    if (
      incoming.length === last.length &&
      incoming.every((id, i) => id === last[i])
    ) {
      return;
    }
    lastSelectionRef.current = incoming;
    const desiredSet = new Set(incoming);
    setNodes((prev) =>
      prev.map((n) => ({
        ...n,
        selected: desiredSet.has(n.id),
      })),
    );
  }, [selectedTableIds, setNodes]);

  // Propagate XYFlow selection changes (Cmd/Ctrl+click multi-select) to parent.
  // Debounced via requestAnimationFrame so at most one update fires per paint
  // frame during box-select drags. Shallow-compares against the last known
  // selection to skip no-op updates.
  const selectionRafRef = useRef(0);
  useEffect(() => {
    return () => cancelAnimationFrame(selectionRafRef.current);
  }, []);
  const handleSelectionChange = useCallback(
    ({ nodes: selectedNodes }: { nodes: Node[] }) => {
      cancelAnimationFrame(selectionRafRef.current);
      selectionRafRef.current = requestAnimationFrame(() => {
        const ids = selectedNodes.map((n) => n.id).sort();
        const last = lastSelectionRef.current;
        if (
          ids.length === last.length &&
          ids.every((id, i) => id === last[i])
        ) {
          return;
        }
        lastSelectionRef.current = ids;
        onSelectionChangeProp?.(ids);
      });
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

      // Merge with existing saved positions so filtered-out schemas are preserved.
      // loadERLayout is called on each drop; the pendingData fast path in
      // erLayoutStore avoids hitting localStorage between debounce flushes.
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
    // fitView via rAF: React commits the setNodes synchronously in most
    // cases, and the rAF fires after the next paint — giving XYFlow time
    // to measure the new node positions before fitting the viewport.
    requestAnimationFrame(() => fitView({ padding: 0.15 }));
    // Merge new positions with saved positions for filtered-out schemas
    const positions = loadERLayout(database) ?? {};
    for (const n of laid) {
      const table = tableById.get(n.id);
      if (table && table.schema && table.name.trim()) {
        positions[positionKey(table.schema, table.name)] = n.position;
      }
    }
    saveERLayout(database, positions);
  }, [database, setNodes, getNodes, getEdges, fitView]);

  const handleResetLayout = useCallback(() => {
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
    requestAnimationFrame(() => fitView({ padding: 0.15 }));
    initialLayoutDone.current = true;
  }, [database, filteredTables, filteredTableById, mode, setNodes, setEdges, fitView]);

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

  // ── MiniMap visibility ─────────────────────────────────────────────────────
  const [showMinimap, setShowMinimap] = useState<boolean>(loadMinimapPref);
  const toggleMinimap = useCallback(() => {
    setShowMinimap((prev) => {
      const next = !prev;
      saveMinimapPref(next);
      return next;
    });
  }, []);

  // ── Context menu ─────────────────────────────────────────────────────────
  const [ctxMenu, setCtxMenu] = useState<CtxMenuState | null>(null);

  // Note: selectedTableIds may lag by one rAF frame due to debounced
  // handleSelectionChange. If a user Ctrl+clicks and immediately right-clicks,
  // the FK "Add" option might show "select 2 tables" until the next frame.
  // In practice the natural click delay makes this a non-issue.
  const handleNodeContextMenu = useCallback(
    (event: React.MouseEvent, node: Node) => {
      if (mode !== "edit" && mode !== "readonly") return;
      if (mode === "readonly" && !onBuildQuery) return;
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
    [mode, filteredTableById, onBuildQuery],
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
    <div ref={canvasRef} style={{ flex: 1, width: "100%", height: "100%" }}>
      {/* onEdgesChange intentionally omitted — edges are derived from fkRef (read-only) */}
      <ReactFlow
        nodes={nodes}
        edges={edges}
        onNodesChange={handleNodesChange}
        onConnect={mode === "edit" ? handleConnect : undefined}
        onNodeClick={closeContextMenu}
        onPaneClick={handlePaneClick}
        onNodeContextMenu={mode === "edit" || (mode === "readonly" && onBuildQuery) ? handleNodeContextMenu : undefined}
        onSelectionChange={handleSelectionChange}
        nodeTypes={nodeTypes}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        nodesDraggable={mode === "edit"}
        nodesConnectable={mode === "edit"}
        deleteKeyCode={null}
        proOptions={{ hideAttribution: true }}
        // Cull off-screen nodes/edges from the DOM. On wide schemas the ER
        // designer otherwise holds every table node in the DOM regardless of
        // viewport, inflating WebView2 memory (issue #821).
        onlyRenderVisibleElements
        style={{ background: "var(--bg)" }}
      >
        <Background color="var(--border)" gap={20} />
        <Controls showInteractive={false} />
        {showMinimap && (
          <MiniMap
            nodeColor="var(--bg-overlay)"
            maskColor="rgba(0, 0, 0, 0.6)"
            style={{ background: "var(--bg-elevated)" }}
          />
        )}
        <Panel position="top-right">
          <div style={{ display: "flex", gap: 4 }}>
            <Button
              size="small"
              type={showMinimap ? "primary" : "default"}
              icon={<BorderOuterOutlined />}
              onClick={toggleMinimap}
              title={showMinimap ? "Hide minimap (saves memory)" : "Show minimap"}
            >
              Minimap
            </Button>
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
      {ctxMenu && mode === "edit" && (
        <ERContextMenu
          ctxMenu={ctxMenu}
          selectedNodeIds={selectedTableIds ?? []}
          onClose={closeContextMenu}
          canvasRef={canvasRef}
          onDuplicateTable={onDuplicateTable}
          onDeleteTable={onDeleteTable}
          onAddFK={onAddFK}
          onRemoveFKs={onRemoveFKs}
        />
      )}
      {ctxMenu && mode === "readonly" && onBuildQuery && (
        <ERReadonlyContextMenu
          ctxMenu={ctxMenu}
          selectedCount={(selectedTableIds ?? []).length}
          onClose={closeContextMenu}
          onBuildQuery={() => onBuildQuery(selectedTableIds ?? [])}
          canvasRef={canvasRef}
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
