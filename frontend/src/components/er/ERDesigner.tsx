// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: ER Designer

import { useState, useEffect, useRef, useCallback, useMemo, lazy, Suspense } from "react";
import { App as AntApp, Modal, Button, Input, Select, Checkbox, AutoComplete } from "antd";
import { PlusOutlined, DeleteOutlined, CopyOutlined, LinkOutlined, SwapOutlined, UndoOutlined, RedoOutlined } from "@ant-design/icons";
import { ExecuteQuery, ListUserSchemas, UpdateERDesignerState, ClearERDesignerState } from "../../../wailsjs/go/app/App";
import { ClipboardSetText, EventsOn } from "../../../wailsjs/runtime/runtime";
import type { snowflake } from "../../../wailsjs/go/models";
import { mcp } from "../../../wailsjs/go/models";
// Lazy — pulls in @xyflow/@dagrejs (the canvas renderer) only when shown.
const ERCanvas = lazy(() => import("./ERCanvas"));
import { initFromERData, normalizeDataType, mergeAITablesIntoDesigner, changedTableIdsFromMerge } from "./erCanvasLayout";
import type { AITableIn } from "./erCanvasLayout";
import { buildMermaid } from "./buildMermaid";
import DefaultFunctionPicker from "../shared/DefaultFunctionPicker";
import { columnConstraints } from "../shared/columnDdl";
import { createTableClause, tableOptionsClauses } from "../shared/tableDdl";
import CreateTableModal, { type TableConfig } from "../database/CreateTableModal";
import { type DesignerColumn, type DesignerTable, SF_DATA_TYPES, SF_TYPES, normalizeIdentifier, baselineTableKey } from "./erTypes";

interface Props {
  database: string;
  initialData?: snowflake.ERDiagramData;
  mergedData?: snowflake.ERDiagramData;
  onClose: () => void;
  onSuccess: () => void;
}

/** Set of canonical Snowflake type names for O(1) lookups in AutoComplete filter. */
const SF_TYPES_SET = new Set(SF_TYPES);

// ── Helpers ───────────────────────────────────────────────────────────────────

function setsEqual<T>(a: Set<T>, b: Set<T>): boolean {
  if (a.size !== b.size) return false;
  for (const x of a) if (!b.has(x)) return false;
  return true;
}

// ── FK dialog (extracted component) ────────────────────────────────────────────

interface FKDialogState {
  childTableId: string;
  parentTableId: string;
  childColId: string;
  parentColId: string;
}

function FKDialog({
  fkDialog,
  tables,
  onClose,
  onCommit,
  onUpdate,
}: {
  fkDialog: FKDialogState;
  tables: DesignerTable[];
  onClose: () => void;
  onCommit: () => void;
  onUpdate: (patch: FKDialogState) => void;
}) {
  const childTable = tables.find((t) => t.id === fkDialog.childTableId);
  const parentTable = tables.find((t) => t.id === fkDialog.parentTableId);
  const childCols = childTable?.columns.filter((c) => c.name.trim()) ?? [];
  const parentCols = parentTable?.columns.filter((c) => c.name.trim()) ?? [];
  const childLabel = childTable
    ? `${childTable.schema}.${childTable.name || "(unnamed)"}`
    : "(unknown)";
  const parentLabel = parentTable
    ? `${parentTable.schema}.${parentTable.name || "(unnamed)"}`
    : "(unknown)";

  return (
    <Modal
      open
      title={
        <span style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <LinkOutlined style={{ color: "var(--accent)" }} />
          Add FK Reference
        </span>
      }
      okText="Add Reference"
      okButtonProps={{ disabled: !fkDialog.childColId || !fkDialog.parentColId }}
      onCancel={onClose}
      onOk={onCommit}
      width={480}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 12, marginTop: 8 }}>
        {/* Direction header with swap button */}
        <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
          <span style={{ fontFamily: "monospace", fontSize: 12, flex: 1, textAlign: "center", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {childLabel}
          </span>
          <Button
            size="small"
            icon={<SwapOutlined />}
            onClick={() =>
              onUpdate({
                childTableId: fkDialog.parentTableId,
                parentTableId: fkDialog.childTableId,
                childColId: fkDialog.parentColId,
                parentColId: fkDialog.childColId,
              })
            }
            title="Swap direction"
          />
          <span style={{ fontFamily: "monospace", fontSize: 12, flex: 1, textAlign: "center", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {parentLabel}
          </span>
        </div>
        <div>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
            Child column (on {childLabel})
          </div>
          <Select
            style={{ width: "100%" }}
            placeholder="Select column"
            value={fkDialog.childColId || undefined}
            onChange={(v) => onUpdate({ ...fkDialog, childColId: v })}
            options={childCols.map((c) => ({ value: c.id, label: c.name }))}
            showSearch
            optionFilterProp="label"
          />
        </div>
        <div>
          <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 4 }}>
            Parent column (on {parentLabel})
          </div>
          <Select
            style={{ width: "100%" }}
            placeholder="Select column"
            value={fkDialog.parentColId || undefined}
            onChange={(v) => onUpdate({ ...fkDialog, parentColId: v })}
            options={parentCols.map((c) => ({ value: c.id, label: c.name }))}
            showSearch
            optionFilterProp="label"
          />
        </div>
      </div>
    </Modal>
  );
}

// ── SQL generation (diff-based) ───────────────────────────────────────────────

/**
 * Compare the designer tables against the baseline (initialData) and produce
 * only the SQL needed to migrate the schema:
 *   - Removed tables  → DROP TABLE
 *   - New tables      → CREATE TABLE
 *   - Existing tables → ALTER TABLE (add/drop/alter columns, PK, FK)
 *
 * When no baseline is provided (or it has no tables) every designer table
 * becomes a CREATE TABLE IF NOT EXISTS (original behaviour).
 */
function generateDiffSQL(
  tables: DesignerTable[],
  database: string,
  baseline?: snowflake.ERDiagramData
): string {
  // Quote a Snowflake identifier. Handles names that are already wrapped in double quotes
  // (from normalizeIdentifier) by stripping the outer quotes before re-quoting.
  const q = (s: string) => {
    const inner = (s.startsWith('"') && s.endsWith('"') && s.length >= 2)
      ? s.slice(1, -1)
      : s;
    return `"${inner.replace(/"/g, '""')}"`;
  };
  const tableRef = (schema: string, name: string) =>
    `${q(database)}.${q(schema)}.${q(name.trim())}`;
  // DEFAULT + NOT NULL in Snowflake-correct order (PK is emitted table-level).
  const colTail = (c: DesignerColumn) =>
    columnConstraints({ defaultValue: c.defaultValue, notNull: c.isPK || c.notNull });

  // Full CREATE TABLE for a brand-new table: inline columns, table-level PK/FK,
  // plus any table options captured from the Create Table modal (#615). Returns
  // null when the table has no named columns. Used by both the pure-create and
  // diff (new-table) paths.
  const createTableSQL = (t: DesignerTable): string | null => {
    const colLines: string[] = [];
    const pkCols: string[] = [];
    const fkLines: string[] = [];
    for (const c of t.columns) {
      if (!c.name.trim()) continue;
      colLines.push(`    ${q(c.name.trim())} ${c.dataType}${colTail(c)}`);
      if (c.isPK) pkCols.push(q(c.name.trim()));
      if (c.fkRef) {
        const parts = c.fkRef.split(".");
        if (parts.length === 3) {
          const [rs, rt, rc] = parts;
          fkLines.push(
            `    FOREIGN KEY (${q(c.name.trim())}) REFERENCES ${tableRef(rs, rt)}(${q(rc)})`
          );
        }
      }
    }
    if (colLines.length === 0) return null;
    const allLines = [...colLines];
    if (pkCols.length > 0) allLines.push(`    PRIMARY KEY (${pkCols.join(", ")})`);
    allLines.push(...fkLines);
    const suffix = tableOptionsClauses(t.options);
    const tail = suffix.length ? "\n" + suffix.join("\n") : "";
    return `${createTableClause(t.options)} ${tableRef(t.schema, t.name)} (\n${allLines.join(",\n")}\n)${tail};`;
  };

  const stmts: string[] = [];

  // ── Pure-create mode (no baseline tables) ────────────────────────────────────
  if (!baseline || baseline.tables.length === 0) {
    for (const t of tables) {
      if (!t.schema || !t.name.trim() || t.columns.length === 0) continue;
      const s = createTableSQL(t);
      if (s) stmts.push(s);
    }
    return stmts.join("\n\n");
  }

  // ── Diff mode ─────────────────────────────────────────────────────────────────

  // Baseline table lookup keyed by "SCHEMA.TABLE"
  const baselineMap = new Map<
    string,
    { schema: string; name: string; columns: snowflake.ERColumn[] }
  >();
  for (const t of baseline.tables) {
    baselineMap.set(baselineTableKey(t.schema, t.name), t);
  }

  // Baseline FK lookup keyed by "SCHEMA.TABLE.COL"
  const baseFKMap = new Map<string, snowflake.ERForeignKey>();
  for (const fk of baseline.fks ?? []) {
    baseFKMap.set(
      `${fk.fromSchema.toUpperCase()}.${fk.fromTable.toUpperCase()}.${fk.fromCol.toUpperCase()}`,
      fk
    );
  }

  // Current table keys present in the designer
  const currentSet = new Set<string>();
  for (const t of tables) {
    if (t.schema && t.name.trim()) {
      currentSet.add(baselineTableKey(t.schema, t.name));
    }
  }

  // 1. DROP tables removed from the designer
  for (const [key, bt] of baselineMap) {
    if (!currentSet.has(key)) {
      stmts.push(
        `-- WARNING: this will permanently drop the table and all its data\nDROP TABLE ${tableRef(bt.schema, bt.name)};`
      );
    }
  }

  // 2. Process each table currently in the designer
  for (const t of tables) {
    if (!t.schema || !t.name.trim()) continue;
    const key = baselineTableKey(t.schema, t.name);
    const bt = baselineMap.get(key);

    if (!bt) {
      // ── New table ─────────────────────────────────────────────────────────────
      const s = createTableSQL(t);
      if (s) stmts.push(s);
    } else {
      // ── Existing table: diff ──────────────────────────────────────────────────
      const ref = tableRef(t.schema, t.name);
      const alter = `ALTER TABLE ${ref}`;

      const baseColMap = new Map<string, snowflake.ERColumn>();
      for (const bc of bt.columns) baseColMap.set(bc.name.toUpperCase(), bc);

      const currentColSet = new Set<string>();
      for (const c of t.columns) if (c.name.trim()) currentColSet.add(c.name.trim().toUpperCase());

      const basePKs = new Set(bt.columns.filter((c) => c.isPK).map((c) => c.name.toUpperCase()));
      const currPKs = new Set(
        t.columns.filter((c) => c.isPK && c.name.trim()).map((c) => c.name.trim().toUpperCase())
      );
      const pkChanged = !setsEqual(basePKs, currPKs);

      // DROP PRIMARY KEY before dropping PK columns
      if (pkChanged && basePKs.size > 0) {
        stmts.push(`${alter} DROP PRIMARY KEY;`);
      }

      // DROP removed columns
      for (const [, bc] of baseColMap) {
        if (!currentColSet.has(bc.name.toUpperCase())) {
          stmts.push(
            `-- WARNING: dropping column "${bc.name}" will permanently delete its data\n${alter} DROP COLUMN ${q(bc.name)};`
          );
        }
      }

      // ADD new columns / ALTER changed columns
      for (const c of t.columns) {
        if (!c.name.trim()) continue;
        const ck = c.name.trim().toUpperCase();
        const bc = baseColMap.get(ck);
        if (!bc) {
          // New column on an existing table → ADD COLUMN. Only literal defaults
          // are valid here (Snowflake rejects function expressions), which is
          // why the ƒ picker is hidden for existing tables while the free-text
          // literal input stays. Existing columns' default changes are not
          // diffed — SET DEFAULT is heavily restricted anyway.
          stmts.push(`${alter} ADD COLUMN ${q(c.name.trim())} ${c.dataType}${colTail(c)};`);
        } else {
          // Existing column — check for type change.
          // normalizeDataType is applied to bc.dataType (raw from INFORMATION_SCHEMA)
          // so it matches the form used by initFromERData when populating the designer.
          // Alias types like INT, TEXT, DATETIME are valid Snowflake DDL types that
          // pass through normalizeDataType as-is, so an unmodified designer column
          // will always match its baseline.
          if (normalizeDataType(bc.dataType) !== c.dataType) {
            stmts.push(`${alter} ALTER COLUMN ${q(c.name.trim())} SET DATA TYPE ${c.dataType};`);
          }
          // Check nullability change
          const baseNN = bc.isPK || bc.nullable === "NO";
          const currNN = c.isPK || c.notNull;
          if (baseNN !== currNN) {
            stmts.push(
              currNN
                ? `${alter} ALTER COLUMN ${q(c.name.trim())} SET NOT NULL;`
                : `${alter} ALTER COLUMN ${q(c.name.trim())} DROP NOT NULL;`
            );
          }
        }
      }

      // ADD PRIMARY KEY (new or changed)
      if (pkChanged && currPKs.size > 0) {
        const pkList = t.columns
          .filter((c) => c.isPK && c.name.trim())
          .map((c) => q(c.name.trim()));
        stmts.push(`${alter} ADD PRIMARY KEY (${pkList.join(", ")});`);
      }

      // FK changes
      for (const c of t.columns) {
        if (!c.name.trim()) continue;
        const ck = c.name.trim().toUpperCase();
        const mapKey = `${t.schema.toUpperCase()}.${t.name.trim().toUpperCase()}.${ck}`;
        const baseFk = baseFKMap.get(mapKey);

        if (c.fkRef) {
          const parts = c.fkRef.split(".");
          if (parts.length !== 3) continue;
          const [rs, rt, rc] = parts;
          // Skip if FK already exists with the exact same target
          if (
            baseFk &&
            baseFk.toSchema.toUpperCase() === rs.toUpperCase() &&
            baseFk.toTable.toUpperCase() === rt.toUpperCase() &&
            baseFk.toCol.toUpperCase() === rc.toUpperCase()
          ) {
            continue;
          }
          stmts.push(
            `${alter} ADD FOREIGN KEY (${q(c.name.trim())}) REFERENCES ${tableRef(rs, rt)}(${q(rc)});`
          );
        } else if (baseFk) {
          // FK was removed — Snowflake requires the constraint name to drop it
          stmts.push(
            `-- NOTE: foreign key on "${c.name.trim()}" was removed in the designer.\n` +
              `-- Run the statement below to find the constraint name, then drop it manually:\n` +
              `-- SHOW IMPORTED KEYS IN TABLE ${ref};`
          );
        }
      }
    }
  }

  return stmts.join("\n\n");
}

// ── Undo/redo history ──────────────────────────────────────────────────────────

const HISTORY_LIMIT = 100;
/** Rapid edits sharing a coalesce key within this window collapse to one undo
 *  step (e.g. typing into a column name). */
const HISTORY_COALESCE_MS = 500;

interface HistoryControls {
  undo: () => void;
  redo: () => void;
  canUndo: boolean;
  canRedo: boolean;
}

/**
 * Bounded undo/redo stack over the designer's `tables` state. A drop-in
 * replacement for `useState<DesignerTable[]>` whose setter (`commit`) records
 * history. Node *positions* are intentionally not tracked here — they live in
 * localStorage (`erLayoutStore`), not in `tables`, so drag-to-reposition is not
 * undoable. History is ephemeral: it resets when the designer modal remounts.
 *
 * `commit(updater, coalesceKey?)` pushes a new undo boundary unless the previous
 * commit shared the same `coalesceKey` within HISTORY_COALESCE_MS, in which case
 * the edits merge into a single step. Structural edits (add/remove/FK/MCP merge)
 * omit the key so each is its own step.
 */
function useDesignerHistory(
  init: () => DesignerTable[],
): [DesignerTable[], (updater: (prev: DesignerTable[]) => DesignerTable[], coalesceKey?: string) => void, HistoryControls] {
  const [present, setPresent] = useState<DesignerTable[]>(init);
  // presentRef mirrors `present` synchronously so back-to-back commits in one
  // tick chain correctly (and so side-effect logic stays out of the updater).
  const presentRef = useRef(present);
  presentRef.current = present;
  const pastRef = useRef<DesignerTable[][]>([]);
  const futureRef = useRef<DesignerTable[][]>([]);
  const lastEditRef = useRef<{ key: string | null; time: number }>({ key: null, time: 0 });
  // No separate re-render trigger is needed: every commit/undo/redo calls
  // setPresent with a fresh array reference, which always re-renders and
  // recomputes canUndo/canRedo from the refs below.

  const commit = useCallback(
    (updater: (prev: DesignerTable[]) => DesignerTable[], coalesceKey?: string) => {
      const prev = presentRef.current;
      const next = updater(prev);
      if (next === prev) return;
      const now = Date.now();
      const key = coalesceKey ?? null;
      const coalesce =
        key !== null &&
        key === lastEditRef.current.key &&
        now - lastEditRef.current.time < HISTORY_COALESCE_MS &&
        pastRef.current.length > 0;
      if (!coalesce) {
        const nextPast = pastRef.current.concat([prev]);
        if (nextPast.length > HISTORY_LIMIT) nextPast.shift();
        pastRef.current = nextPast;
      }
      futureRef.current = [];
      lastEditRef.current = { key, time: now };
      presentRef.current = next;
      setPresent(next);
    },
    [],
  );

  const undo = useCallback(() => {
    if (pastRef.current.length === 0) return;
    const prevState = pastRef.current[pastRef.current.length - 1];
    pastRef.current = pastRef.current.slice(0, -1);
    futureRef.current = [presentRef.current, ...futureRef.current];
    lastEditRef.current = { key: null, time: 0 };
    presentRef.current = prevState;
    setPresent(prevState);
  }, []);

  const redo = useCallback(() => {
    if (futureRef.current.length === 0) return;
    const nextState = futureRef.current[0];
    futureRef.current = futureRef.current.slice(1);
    pastRef.current = pastRef.current.concat([presentRef.current]);
    lastEditRef.current = { key: null, time: 0 };
    presentRef.current = nextState;
    setPresent(nextState);
  }, []);

  return [
    present,
    commit,
    { undo, redo, canUndo: pastRef.current.length > 0, canRedo: futureRef.current.length > 0 },
  ];
}

// ── Component ─────────────────────────────────────────────────────────────────

export default function ERDesigner({ database, initialData, mergedData, onClose, onSuccess }: Props) {
  const { modal, message } = AntApp.useApp();
  const [leftWidth, setLeftWidth] = useState(490);
  const [resizing, setResizing] = useState(false);
  const resizeStart = useRef({ x: 0, width: 0 });

  const [schemas, setSchemas] = useState<string[]>([]);
  const [tables, commit, { undo, redo, canUndo, canRedo }] = useDesignerHistory(() => {
    if (mergedData) return initFromERData(mergedData);
    if (initialData) return initFromERData(initialData);
    return [];
  });

  // Highlight for the latest MCP change — only the most recent AI modification
  // is marked, and any manual edit / undo / redo clears it.
  const [changedTableIds, setChangedTableIds] = useState<Set<string> | null>(null);

  // Manual-edit wrapper: records an undo step and clears the AI-change highlight.
  // The MCP merge path calls `commit` directly (then sets its own highlight).
  const edit = useCallback(
    (updater: (prev: DesignerTable[]) => DesignerTable[], coalesceKey?: string) => {
      setChangedTableIds(null);
      commit(updater, coalesceKey);
    },
    [commit],
  );

  const doUndo = useCallback(() => { setChangedTableIds(null); undo(); }, [undo]);
  const doRedo = useCallback(() => { setChangedTableIds(null); redo(); }, [redo]);

  // Stable ref for tables — used in callbacks that shouldn't re-create on every tables change
  const tablesRef = useRef(tables);
  tablesRef.current = tables;

  // Baseline snapshot keyed by table (uppercased "SCHEMA.TABLE" → set of its
  // column names), matching generateDiffSQL's keying. Used to decide which
  // DEFAULT controls to show per column.
  const baselineCols = useMemo(() => {
    const m = new Map<string, Set<string>>();
    for (const bt of initialData?.tables ?? []) {
      m.set(baselineTableKey(bt.schema, bt.name), new Set(bt.columns.map((c) => c.name.trim().toUpperCase())));
    }
    return m;
  }, [initialData]);
  // A table absent from the baseline is brand-new → CREATE TABLE (function
  // DEFAULTs allowed). A table present diffs to ALTER; existing columns can't
  // change their DEFAULT, but a newly-added column emits ADD COLUMN … DEFAULT.
  const isExistingTable = (t: DesignerTable) => baselineCols.has(baselineTableKey(t.schema, t.name));
  const isNewColumn = (t: DesignerTable, c: DesignerColumn) => {
    const cols = baselineCols.get(baselineTableKey(t.schema, t.name));
    return !cols || !cols.has(c.name.trim().toUpperCase());
  };

  // Canvas selection (multi-select via Cmd/Ctrl+click)
  const [selectedTableIds, setSelectedTableIds] = useState<string[]>([]);

  // Create Table modal (new-table creation reuses the full modal — #615)
  const [createModalOpen, setCreateModalOpen] = useState(false);

  // SQL modal
  const [sqlModalOpen, setSqlModalOpen] = useState(false);
  const [runError, setRunError] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  // Refs for scrolling sidebar to selected table
  const tableCardRefs = useRef<Map<string, HTMLDivElement>>(new Map());

  // ── Left panel resize ────────────────────────────────────────────────────────

  const onResizeMouseDown = (e: React.MouseEvent) => {
    resizeStart.current = { x: e.clientX, width: leftWidth };
    setResizing(true);
    e.preventDefault();
  };

  useEffect(() => {
    if (!resizing) return;
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    const onMove = (e: MouseEvent) => {
      const w = resizeStart.current.width + (e.clientX - resizeStart.current.x);
      setLeftWidth(Math.max(260, Math.min(700, w)));
    };
    const onUp = () => setResizing(false);
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
    return () => {
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
    };
  }, [resizing]);

  // ── Fetch schemas on mount ──────────────────────────────────────────────────

  useEffect(() => {
    ListUserSchemas(database)
      .then((s) => setSchemas(s.filter((n) => n.toUpperCase() !== "INFORMATION_SCHEMA")))
      .catch(() => {});
  }, [database]);

  // ── Sync designer state to backend for MCP tools ──────────────────────────

  const pushState = useCallback((tbls: DesignerTable[]) => {
    const out = tbls.map((t) =>
      new mcp.ERDesignerTableOut({
        schema: t.schema,
        name: t.name,
        columns: t.columns.map((c) =>
          new mcp.ERDesignerColumnOut({
            name: c.name,
            dataType: c.dataType,
            isPK: c.isPK,
            notNull: c.notNull,
            fkRef: c.fkRef || undefined,
            defaultValue: c.defaultValue || undefined,
          }),
        ),
      }),
    );
    UpdateERDesignerState(database, out).catch(() => {});
  }, [database]);

  // Push initial state on mount, clear on unmount.
  useEffect(() => {
    pushState(tablesRef.current);
    return () => { ClearERDesignerState().catch(() => {}); };
  // eslint-disable-next-line react-hooks/exhaustive-deps -- mount/unmount only
  }, []);

  // Debounced push on subsequent tables changes (300ms).
  const prevTablesRef = useRef(tables);
  useEffect(() => {
    if (prevTablesRef.current === tables) return;
    prevTablesRef.current = tables;
    const timer = setTimeout(() => pushState(tables), 300);
    return () => clearTimeout(timer);
  }, [tables, pushState]);

  // ── Listen for MCP modify_er_designer events ─────────────────────────────

  useEffect(() => {
    const off = EventsOn("mcp:modify-er-designer", (payload: { tables: AITableIn[] }) => {
      // Merge inside commit's updater (against the hook's authoritative `prev`),
      // not against tablesRef — the ref only refreshes on render, so two events
      // arriving before a re-render would otherwise merge against stale data and
      // silently drop the first AI change.
      let merged: DesignerTable[] = tablesRef.current;
      commit((prev) => {
        merged = mergeAITablesIntoDesigner(prev, payload.tables);
        return merged; // discrete undo step — the user can undo an AI change
      });
      setChangedTableIds(changedTableIdsFromMerge(merged, payload.tables));
      message.info(`${payload.tables.length} table(s) updated by AI`);
    });
    return () => off();
  // eslint-disable-next-line react-hooks/exhaustive-deps -- commit/message stable, tablesRef is a stable ref
  }, []);

  // ── Undo/redo keyboard shortcuts (scoped to the designer modal) ────────────
  // Cmd/Ctrl+Z = undo, Cmd/Ctrl+Shift+Z or Ctrl+Y = redo. Skipped while focus is
  // in a text field so native input-level undo keeps working (and so Monaco's
  // editor undo, which lives outside this modal, is never intercepted here).
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!e.metaKey && !e.ctrlKey) return;
      const key = e.key.toLowerCase();
      const isUndo = key === "z" && !e.shiftKey;
      const isRedo = (key === "z" && e.shiftKey) || key === "y";
      if (!isUndo && !isRedo) return;
      // Yield to native text-editing undo only for fields that actually have it:
      // contentEditable, <textarea>, <select>, and text-like <input>s. A non-text
      // input (e.g. the schema-visibility checkbox, which keeps focus after a
      // click) has no native undo, so it must not swallow the shortcut.
      const t = e.target as HTMLElement | null;
      if (t) {
        const tag = t.tagName;
        const isTextInput =
          tag === "INPUT" &&
          /^(text|search|url|email|password|tel|number|)$/i.test((t as HTMLInputElement).type);
        if (t.isContentEditable || tag === "TEXTAREA" || tag === "SELECT" || isTextInput) return;
      }
      e.preventDefault();
      if (isRedo) doRedo();
      else doUndo();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [doUndo, doRedo]);

  // ── Schema filter for canvas ──────────────────────────────────────────────

  // schemas already has INFORMATION_SCHEMA filtered out on fetch
  const canvasSchemas = useMemo(() => {
    const fromTables = tables.map((t) => t.schema).filter(Boolean);
    return [...new Set([...fromTables, ...schemas])].sort();
  }, [tables, schemas]);

  const [visibleSchemas, setVisibleSchemas] = useState<Set<string> | null>(null);

  // Show all schemas by default (null = no filter)
  const effectiveVisibleSchemas = useMemo(
    () => visibleSchemas ?? new Set(canvasSchemas),
    [visibleSchemas, canvasSchemas],
  );

  const toggleSchema = (schema: string) => {
    setVisibleSchemas((prev) => {
      const current = prev ?? new Set(canvasSchemas);
      const next = new Set(current);
      if (next.has(schema)) {
        next.delete(schema);
      } else {
        next.add(schema);
      }
      return next;
    });
  };

  // ── Scroll sidebar to selected table ───────────────────────────────────────

  useEffect(() => {
    if (selectedTableIds.length === 0) return;
    const lastId = selectedTableIds[selectedTableIds.length - 1];
    const el = tableCardRefs.current.get(lastId);
    el?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [selectedTableIds]);

  // ── Table / column mutators ───────────────────────────────────────────────────

  // Default schema for a brand-new table — the modal has no schema picker, so the
  // table lands here and can be reassigned via the sidebar card's schema Select.
  const defaultSchema = schemas[0] ?? "";

  // "Add Table" opens the full Create Table modal; the returned definition is
  // mapped to a DesignerTable and placed on the canvas (#615). FKs are still
  // wired on the canvas; table-level options ride along in `options`.
  const handleDefine = (cfg: TableConfig) => {
    // A case-sensitive name must keep its case → wrap in quotes so
    // normalizeIdentifier preserves it (it uppercases only unquoted names).
    const name = normalizeIdentifier(cfg.caseSensitive ? `"${cfg.name.trim()}"` : cfg.name);
    const newTable: DesignerTable = {
      id: crypto.randomUUID(),
      schema: defaultSchema,
      name,
      // Column names are kept verbatim (not normalizeIdentifier'd) to match the
      // modal's direct ExecDDL path, which quotes them as-typed — so the same
      // modal input yields the same Snowflake identifier from either entry point
      // (#688 review). q() in createTableSQL quotes them the same way buildSql does.
      columns: cfg.columns.map((c) => ({
        id: crypto.randomUUID(),
        name: c.name.trim(),
        dataType: c.type,
        isPK: c.primaryKey,
        notNull: c.notNull || c.primaryKey,
        fkRef: "",
        defaultValue: c.defaultValue,
      })),
      options: {
        tableType: cfg.tableType,
        orReplace: cfg.orReplace,
        ifNotExists: cfg.ifNotExists,
        clusterBy: cfg.clusterBy,
        dataRetentionTimeInDays: cfg.dataRetentionTimeInDays,
        maxDataExtensionTimeInDays: cfg.maxDataExtensionTimeInDays,
        changeTracking: cfg.changeTracking,
        enableSchemaEvolution: cfg.enableSchemaEvolution,
        comment: cfg.comment,
      },
    };
    edit((prev) => [...prev, newTable]);
    setSelectedTableIds([newTable.id]);
  };

  const removeTable = useCallback((tableId: string) => {
    edit((prev) => prev.filter((t) => t.id !== tableId));
    setSelectedTableIds((prev) => prev.filter((id) => id !== tableId));
  }, [edit]);

  const confirmRemoveTable = useCallback(
    (tableId: string) => {
      const table = tablesRef.current.find((t) => t.id === tableId);
      const label = table
        ? `${table.schema ? table.schema + "." : ""}${table.name || "(unnamed)"}`
        : "this table";
      modal.confirm({
        title: "Delete table?",
        content: `"${label}" and all its columns will be removed from the designer. This does not drop the table in Snowflake.`,
        okText: "Delete",
        okButtonProps: { danger: true },
        onOk: () => removeTable(tableId),
      });
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps -- tablesRef is a stable ref, modal/removeTable are stable
    [modal, removeTable],
  );

  const updateTable = useCallback((tableId: string, patch: Partial<Pick<DesignerTable, "name" | "schema">>) => {
    // Coalesce name typing into one undo step; a schema pick is discrete.
    const coalesceKey = patch.name !== undefined ? `table:${tableId}:name` : undefined;
    edit((prev) => prev.map((t) => (t.id === tableId ? { ...t, ...patch } : t)), coalesceKey);
  }, [edit]);

  const addColumn = useCallback((tableId: string) => {
    edit((prev) =>
      prev.map((t) =>
        t.id === tableId
          ? { ...t, columns: [...t.columns, { id: crypto.randomUUID(), name: "", dataType: "VARCHAR", isPK: false, notNull: false, fkRef: "", defaultValue: "" }] }
          : t
      )
    );
  }, [edit]);

  const removeColumn = useCallback((tableId: string, colId: string) => {
    edit((prev) =>
      prev.map((t) => (t.id === tableId ? { ...t, columns: t.columns.filter((c) => c.id !== colId) } : t))
    );
  }, [edit]);

  const updateColumn = useCallback((tableId: string, colId: string, patch: Partial<DesignerColumn>) => {
    // Coalesce rapid text edits (name/type/default typing) into a single undo
    // step; discrete toggles (PK/NN/FK) each become their own step.
    const isText = patch.name !== undefined || patch.dataType !== undefined || patch.defaultValue !== undefined;
    const coalesceKey = isText ? `col:${tableId}:${colId}` : undefined;
    edit((prev) =>
      prev.map((t) =>
        t.id === tableId
          ? {
              ...t,
              columns: t.columns.map((c) => {
                if (c.id !== colId) return c;
                const updated = { ...c, ...patch };
                if (patch.isPK !== undefined && patch.isPK) updated.notNull = true;
                return updated;
              }),
            }
          : t
      ),
      coalesceKey,
    );
  }, [edit]);

  // FK options: "SCHEMA.TABLE.COLUMN" for every named column in every other table
  const fkOptions = (currentTableId: string): { value: string; label: string }[] => {
    const opts: { value: string; label: string }[] = [{ value: "", label: "—" }];
    for (const t of tables) {
      if (t.id === currentTableId || !t.name.trim() || !t.schema) continue;
      for (const c of t.columns) {
        if (!c.name.trim()) continue;
        const ref = `${t.schema}.${t.name.trim()}.${c.name.trim()}`;
        opts.push({ value: ref, label: ref });
      }
    }
    return opts;
  };

  // ── Canvas callbacks ────────────────────────────────────────────────────────

  const handleTableRename = useCallback(
    (tableId: string, newName: string) => updateTable(tableId, { name: newName }),
    [updateTable],
  );

  const handleColumnRename = useCallback(
    (tableId: string, colId: string, newName: string) => updateColumn(tableId, colId, { name: newName }),
    [updateColumn],
  );

  const handleFKConnect = useCallback(
    (fromTableId: string, fromColId: string, toTableId: string, toColId: string) => {
      const toTable = tablesRef.current.find((t) => t.id === toTableId);
      if (!toTable || !toTable.schema || !toTable.name.trim()) return;
      const toCol = toTable.columns.find((c) => c.id === toColId);
      if (!toCol || !toCol.name.trim()) return;

      const fkRef = `${toTable.schema}.${toTable.name.trim()}.${toCol.name.trim()}`;
      updateColumn(fromTableId, fromColId, { fkRef });
    },
    [updateColumn],
  );

  // ── Context menu handlers ──────────────────────────────────────────────────
  // tablesRef is a stable ref and setSelectedTableIds is a stable state setter;
  // `edit` (history-recording mutator) is stable, so it's the only real dep.

  const handleDuplicateTable = useCallback(
    (tableId: string) => {
      const source = tablesRef.current.find((t) => t.id === tableId);
      if (!source) return;
      const newTable: DesignerTable = {
        id: crypto.randomUUID(),
        schema: source.schema,
        name: source.name ? `${source.name}_COPY` : "",
        columns: source.columns.map((c) => ({
          ...c,
          id: crypto.randomUUID(),
          fkRef: "", // Clear FK refs — the copy is a fresh table
        })),
      };
      edit((prev) => [...prev, newTable]);
      setSelectedTableIds([newTable.id]);
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps -- tablesRef is a stable ref
    [edit],
  );

  const handleRemoveFKs = useCallback((tableId: string) => {
    edit((prev) =>
      prev.map((t) =>
        t.id === tableId
          ? { ...t, columns: t.columns.map((c) => ({ ...c, fkRef: "" })) }
          : t,
      ),
    );
  }, [edit]);

  // FK dialog state (two tables pre-populated from multi-select)
  // "child" = table that holds the FK column, "parent" = referenced table
  const [fkDialog, setFkDialog] = useState<FKDialogState | null>(null);

  const handleAddFK = useCallback(
    (tableIdA: string, tableIdB: string) => {
      const tableA = tablesRef.current.find((t) => t.id === tableIdA);
      const tableB = tablesRef.current.find((t) => t.id === tableIdB);

      let childColId = "";
      let parentColId = "";

      // Auto-detect the best FK column pair between the two tables.
      // Tries both directions (A=child/B=parent, then B=child/A=parent) and
      // picks the first match.
      // Priority per direction:
      //   1) child column whose name contains the parent table name
      //      matching a PK column on the parent (e.g. CUSTOMER_ID → CUSTOMER.ID)
      //   2) PK column on the parent that shares a name with a child column
      //   3) any common column name between the two tables
      let childTableId = tableIdA;
      let parentTableId = tableIdB;

      if (tableA && tableB) {
        const tryDetect = (
          child: DesignerTable,
          parent: DesignerTable,
        ): { childColId: string; parentColId: string } | null => {
          const namedChild = child.columns.filter((c) => c.name.trim());
          const namedParent = parent.columns.filter((c) => c.name.trim());
          const colMapChild = new Map(namedChild.map((c) => [c.name.trim().toUpperCase(), c]));
          const pName = parent.name.trim().toUpperCase();

          // Strategy 1: child col name contains parent table name + parent PK
          if (pName) {
            const parentPK = namedParent.find((c) => c.isPK);
            if (parentPK) {
              const candidate = namedChild.find(
                (c) => c.name.trim().toUpperCase().includes(pName),
              );
              if (candidate) return { childColId: candidate.id, parentColId: parentPK.id };
            }
          }

          // Strategy 2: parent PK column name exists in child
          const parentPK = namedParent.find((c) => c.isPK);
          if (parentPK) {
            const match = colMapChild.get(parentPK.name.trim().toUpperCase());
            if (match) return { childColId: match.id, parentColId: parentPK.id };
          }

          // Strategy 3: any common column name
          for (const colP of namedParent) {
            const match = colMapChild.get(colP.name.trim().toUpperCase());
            if (match) return { childColId: match.id, parentColId: colP.id };
          }

          return null;
        };

        // Try A=child, B=parent first
        const forward = tryDetect(tableA, tableB);
        if (forward) {
          childColId = forward.childColId;
          parentColId = forward.parentColId;
          childTableId = tableIdA;
          parentTableId = tableIdB;
        } else {
          // Try B=child, A=parent
          const reverse = tryDetect(tableB, tableA);
          if (reverse) {
            childColId = reverse.childColId;
            parentColId = reverse.parentColId;
            childTableId = tableIdB;
            parentTableId = tableIdA;
          }
        }
      }

      setFkDialog({
        childTableId,
        parentTableId,
        childColId,
        parentColId,
      });
    },
    // eslint-disable-next-line react-hooks/exhaustive-deps -- tablesRef is a stable ref
    [],
  );

  const commitFK = useCallback(() => {
    if (!fkDialog) return;
    const { childTableId, childColId, parentTableId, parentColId } = fkDialog;
    if (!childColId || !parentColId) return;

    const parentTable = tablesRef.current.find((t) => t.id === parentTableId);
    if (!parentTable || !parentTable.schema || !parentTable.name.trim()) return;
    const parentCol = parentTable.columns.find((c) => c.id === parentColId);
    if (!parentCol || !parentCol.name.trim()) return;

    const fkRef = `${parentTable.schema}.${parentTable.name.trim()}.${parentCol.name.trim()}`;
    updateColumn(childTableId, childColId, { fkRef });
    setFkDialog(null);
  }, [fkDialog, updateColumn]);

  // ── SQL & run ─────────────────────────────────────────────────────────────────

  const sql = useMemo(() => generateDiffSQL(tables, database, initialData), [tables, database, initialData]);
  const hasChanges = sql.trim().length > 0;

  const runSQL = async () => {
    setRunning(true);
    setRunError(null);
    try {
      await ExecuteQuery(sql);
      message.success("Changes applied successfully.");
      setSqlModalOpen(false);
      onSuccess();
    } catch (e) {
      setRunError(String(e));
    } finally {
      setRunning(false);
    }
  };

  const schemaOptions = schemas.map((s) => ({ value: s, label: s }));

  const handleClose = () => {
    if (!hasChanges) { onClose(); return; }
    modal.confirm({
      title: "Discard unsaved changes?",
      content: "You have unapplied schema changes. Close anyway?",
      okText: "Discard changes",
      okButtonProps: { danger: true },
      cancelText: "Keep editing",
      onOk: onClose,
    });
  };

  const copyMermaid = () => {
    ClipboardSetText(buildMermaid(tables, effectiveVisibleSchemas));
  };

  // ── Render ────────────────────────────────────────────────────────────────────

  return (
    <>
      <Modal
        open
        title={`Design Tables — ${database}`}
        onCancel={handleClose}
        footer={
          <div style={{ display: "flex", justifyContent: "flex-end", gap: 8 }}>
            <Button onClick={handleClose}>Cancel</Button>
            <Button
              type="primary"
              disabled={!hasChanges}
              onClick={() => { setRunError(null); setSqlModalOpen(true); }}
            >
              Review & Apply Changes
            </Button>
          </div>
        }
        width="90vw"
        styles={{ body: { padding: 0 } }}
      >
        <div style={{ display: "flex", height: "80vh" }}>
          {/* Left panel */}
          <div
            style={{
              width: leftWidth,
              flexShrink: 0,
              overflowY: "auto",
              padding: 12,
              display: "flex",
              flexDirection: "column",
              gap: 14,
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <Button size="small" icon={<PlusOutlined />} onClick={() => setCreateModalOpen(true)}>
                Add Table
              </Button>
              <div style={{ flex: 1 }} />
              <Button
                size="small"
                icon={<UndoOutlined />}
                onClick={doUndo}
                disabled={!canUndo}
                title="Undo (⌘Z / Ctrl+Z)"
              />
              <Button
                size="small"
                icon={<RedoOutlined />}
                onClick={doRedo}
                disabled={!canRedo}
                title="Redo (⇧⌘Z / Ctrl+Y)"
              />
            </div>

            {tables.length === 0 && (
              <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "12px 0" }}>
                No tables yet. Click "Add Table" to start.
              </div>
            )}

            {tables.filter((t) => !t.schema || effectiveVisibleSchemas.has(t.schema)).map((t) => (
              <div
                key={t.id}
                ref={(el) => {
                  if (el) tableCardRefs.current.set(t.id, el);
                  else tableCardRefs.current.delete(t.id);
                }}
                style={{
                  border: "1px solid var(--border)",
                  borderRadius: 6,
                  overflow: "hidden",
                  flexShrink: 0,
                  borderLeft: selectedTableIds.includes(t.id) ? "3px solid var(--accent)" : "1px solid var(--border)",
                  cursor: "pointer",
                }}
                onClick={() => setSelectedTableIds([t.id])}
              >
                {/* Table header — two rows: schema+delete, then table name */}
                <div
                  style={{
                    display: "flex",
                    flexDirection: "column",
                    gap: 4,
                    padding: "6px 8px",
                    background: "var(--bg-overlay)",
                    borderBottom: "1px solid var(--border)",
                  }}
                >
                  <div style={{ display: "flex", alignItems: "center", gap: 6 }}>
                    <Select
                      size="small"
                      placeholder="schema"
                      value={t.schema || undefined}
                      onChange={(v) => updateTable(t.id, { schema: v })}
                      options={schemaOptions}
                      style={{ flex: 1, fontFamily: "monospace", fontSize: 11 }}
                      showSearch
                    />
                    <Button
                      size="small"
                      type="text"
                      icon={<DeleteOutlined style={{ color: "#f85149" }} />}
                      onClick={(e) => { e.stopPropagation(); confirmRemoveTable(t.id); }}
                    />
                  </div>
                  <Input
                    size="small"
                    placeholder="TABLE_NAME"
                    value={t.name}
                    onChange={(e) => updateTable(t.id, { name: e.target.value })}
                    onBlur={(e) => updateTable(t.id, { name: normalizeIdentifier(e.target.value) })}
                    style={{ fontFamily: "monospace", fontSize: 13, fontWeight: 600 }}
                    onClick={(e) => e.stopPropagation()}
                  />
                </div>

                {/* Columns */}
                <div style={{ padding: "6px 8px", display: "flex", flexDirection: "column", gap: 4 }}>
                  {t.columns.map((c) => (
                    <div key={c.id} style={{ display: "flex", alignItems: "center", gap: 4 }} onClick={(e) => e.stopPropagation()}>
                      <Input
                        size="small"
                        placeholder="COLUMN_NAME"
                        value={c.name}
                        onChange={(e) => updateColumn(t.id, c.id, { name: e.target.value })}
                        onBlur={(e) => updateColumn(t.id, c.id, { name: normalizeIdentifier(e.target.value) })}
                        style={{ flex: 1, fontFamily: "monospace", fontSize: 11, minWidth: 80 }}
                      />
                      <AutoComplete
                        size="small"
                        value={c.dataType}
                        onChange={(v) => updateColumn(t.id, c.id, { dataType: v })}
                        onBlur={(e) => {
                          const val = (e.target as HTMLInputElement).value.trim().toUpperCase();
                          if (val) updateColumn(t.id, c.id, { dataType: val });
                        }}
                        style={{ width: 150, flexShrink: 0 }}
                        popupMatchSelectWidth={false}
                        options={SF_DATA_TYPES.map((dt) => ({
                          value: dt.name,
                          label: dt.paramHint ? `${dt.name} ${dt.paramHint}` : dt.name,
                        }))}
                        filterOption={(input, option) => {
                          // Show all options when input is an existing type (possibly with params)
                          const base = input.replace(/\s*\(.*$/, "").trim().toUpperCase();
                          if (SF_TYPES_SET.has(base)) return true;
                          return (option?.label as string ?? "").toUpperCase().includes(input.toUpperCase());
                        }}
                      />
                      <Button
                        size="small"
                        type={c.isPK ? "primary" : "default"}
                        title="Primary Key"
                        onClick={() => updateColumn(t.id, c.id, { isPK: !c.isPK })}
                        style={{ padding: "0 5px", fontSize: 10, flexShrink: 0 }}
                      >
                        PK
                      </Button>
                      <Button
                        size="small"
                        type={c.notNull ? "primary" : "default"}
                        title="Not Null"
                        onClick={() => updateColumn(t.id, c.id, { notNull: !c.notNull })}
                        style={{ padding: "0 5px", fontSize: 10, flexShrink: 0 }}
                      >
                        NN
                      </Button>
                      <Select
                        size="small"
                        value={c.fkRef || ""}
                        onChange={(v) => updateColumn(t.id, c.id, { fkRef: v })}
                        style={{ width: 150, flexShrink: 0 }}
                        options={fkOptions(t.id)}
                        showSearch
                      />
                      {/* DEFAULT editing is for *new* columns only — an existing
                          column's default can't be changed (SET DEFAULT is heavily
                          restricted). The ƒ picker is further limited to new tables:
                          Snowflake rejects function-expression defaults on ADD COLUMN,
                          so a new column on an existing table gets free-text (literal)
                          only. */}
                      {isNewColumn(t, c) && (
                        <>
                          <Input
                            size="small"
                            placeholder="DEFAULT"
                            value={c.defaultValue}
                            onChange={(e) => updateColumn(t.id, c.id, { defaultValue: e.target.value })}
                            style={{ width: 110, flexShrink: 0, fontFamily: "monospace", fontSize: 11 }}
                          />
                          {!isExistingTable(t) && (
                            <DefaultFunctionPicker onPick={(sql) => updateColumn(t.id, c.id, { defaultValue: sql })} />
                          )}
                        </>
                      )}
                      <Button
                        size="small"
                        type="text"
                        icon={<DeleteOutlined style={{ color: "#f85149", fontSize: 11 }} />}
                        onClick={() => removeColumn(t.id, c.id)}
                        style={{ flexShrink: 0 }}
                      />
                    </div>
                  ))}

                  <Button
                    size="small"
                    type="dashed"
                    icon={<PlusOutlined />}
                    onClick={(e) => { e.stopPropagation(); addColumn(t.id); }}
                    style={{ alignSelf: "flex-start", fontSize: 11, marginTop: 2 }}
                  >
                    Add Column
                  </Button>
                </div>
              </div>
            ))}
          </div>

          {/* Resize handle */}
          <div
            onMouseDown={onResizeMouseDown}
            style={{
              width: 5,
              flexShrink: 0,
              cursor: "col-resize",
              background: resizing ? "var(--accent)" : "transparent",
              borderLeft: "1px solid var(--border)",
              transition: resizing ? "none" : "background 0.15s",
            }}
            onMouseEnter={(e) => { if (!resizing) e.currentTarget.style.background = "color-mix(in srgb, var(--accent) 26%, transparent)"; }}
            onMouseLeave={(e) => { if (!resizing) e.currentTarget.style.background = "transparent"; }}
          />

          {/* Right panel — interactive canvas */}
          <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden", userSelect: resizing ? "none" : undefined }}>
            <div
              style={{
                display: "flex",
                alignItems: "center",
                gap: 8,
                padding: "6px 12px",
                borderBottom: "1px solid var(--border)",
                flexWrap: "wrap",
              }}
            >
              <div style={{ display: "flex", gap: 10, flexWrap: "wrap", flex: 1, alignItems: "center" }}>
                {canvasSchemas.map((s) => (
                  <Checkbox
                    key={s}
                    checked={effectiveVisibleSchemas.has(s)}
                    onChange={() => toggleSchema(s)}
                  >
                    <span style={{ fontSize: 11 }}>{s}</span>
                  </Checkbox>
                ))}
              </div>
              <Button size="small" icon={<CopyOutlined />} onClick={copyMermaid}>
                Copy Mermaid
              </Button>
            </div>

            <Suspense fallback={<div style={{ height: "100%" }} />}>
              <ERCanvas
                key={database}
                tables={tables}
                mode="edit"
                database={database}
                visibleSchemas={effectiveVisibleSchemas}
                selectedTableIds={selectedTableIds}
                onSelectionChange={setSelectedTableIds}
                onConnect={handleFKConnect}
                onTableRename={handleTableRename}
                onColumnRename={handleColumnRename}
                onColumnRemove={removeColumn}
                onDuplicateTable={handleDuplicateTable}
                onDeleteTable={confirmRemoveTable}
                onAddFK={handleAddFK}
                onRemoveFKs={handleRemoveFKs}
                changedNodeIds={changedTableIds ?? undefined}
              />
            </Suspense>
          </div>
        </div>
      </Modal>

      {/* Create Table modal — full new-table feature set incl. the ƒ DEFAULT picker (#615) */}
      {createModalOpen && (
        <CreateTableModal
          db={database}
          schema={defaultSchema}
          onClose={() => setCreateModalOpen(false)}
          onDefine={handleDefine}
        />
      )}

      {/* Add FK reference dialog (two tables pre-populated from multi-select) */}
      {fkDialog && (
        <FKDialog
          fkDialog={fkDialog}
          tables={tables}
          onClose={() => setFkDialog(null)}
          onCommit={commitFK}
          onUpdate={setFkDialog}
        />
      )}

      {/* SQL review modal */}
      <Modal
        open={sqlModalOpen}
        title="Schema Changes"
        onCancel={() => setSqlModalOpen(false)}
        footer={
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 8 }}>
            <div style={{ flex: 1 }}>
              {runError && (
                <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, wordBreak: "break-word" }}>
                  {runError}
                </div>
              )}
            </div>
            <div style={{ display: "flex", gap: 8 }}>
              <Button onClick={() => ClipboardSetText(sql)} icon={<CopyOutlined />}>Copy</Button>
              <Button type="primary" loading={running} onClick={runSQL} disabled={!hasChanges}>Apply</Button>
            </div>
          </div>
        }
        width={720}
      >
        <pre
          style={{
            margin: 0,
            padding: 16,
            background: "var(--bg)",
            color: "var(--text)",
            fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
            fontSize: 12,
            lineHeight: 1.6,
            overflowX: "auto",
            maxHeight: "55vh",
            overflowY: "auto",
            borderRadius: 4,
            border: "1px solid var(--border)",
          }}
        >
          {sql || "(no changes detected)"}
        </pre>
      </Modal>
    </>
  );
}
