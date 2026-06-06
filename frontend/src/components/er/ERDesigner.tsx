// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import { Modal, Button, Input, Select, Checkbox, AutoComplete, message as antMessage } from "antd";
import { PlusOutlined, DeleteOutlined, CopyOutlined } from "@ant-design/icons";
import { ExecuteQuery, ListSchemas } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import ERCanvas from "./ERCanvas";
import { initFromERData, normalizeDataType } from "./erCanvasLayout";
import { type DesignerColumn, type DesignerTable, SF_DATA_TYPES, normalizeIdentifier } from "./erTypes";

interface Props {
  database: string;
  initialData?: snowflake.ERDiagramData;
  onClose: () => void;
  onSuccess: () => void;
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function setsEqual<T>(a: Set<T>, b: Set<T>): boolean {
  if (a.size !== b.size) return false;
  for (const x of a) if (!b.has(x)) return false;
  return true;
}

// ── Mermaid export (for "Copy Mermaid" button) ────────────────────────────────

function sanitiseId(s: string): string {
  const id = s.replace(/[^a-zA-Z0-9_]/g, "_");
  return /^[0-9]/.test(id) ? "_" + id : id;
}

function entityId(schema: string, table: string): string {
  return sanitiseId(schema) + "__" + sanitiseId(table);
}

function shortType(dt: string): string {
  return dt.replace(/\s*\([^)]*\)/g, "").replace(/\s+/g, "_") || "string";
}

function buildDesignerMermaid(tables: DesignerTable[]): string {
  const lines: string[] = ["erDiagram"];
  const validTables = tables.filter((t) => t.schema && t.name.trim());

  for (const t of validTables) {
    const namedCols = t.columns.filter((c) => c.name.trim());
    if (namedCols.length === 0) continue;
    const id = entityId(t.schema, t.name.trim());
    lines.push(`  ${id} {`);
    for (const c of namedCols) {
      const type = shortType(c.dataType) || "string";
      const pk = c.isPK ? " PK" : "";
      lines.push(`    ${type} ${sanitiseId(c.name)}${pk}`);
    }
    lines.push("  }");
  }

  const seen = new Set<string>();
  for (const t of validTables) {
    for (const c of t.columns) {
      if (!c.fkRef) continue;
      const parts = c.fkRef.split(".");
      if (parts.length !== 3) continue;
      const [refSchema, refTable] = parts;
      if (!refTable.trim()) continue;
      const fromId = entityId(t.schema, t.name.trim());
      const toId = entityId(refSchema, refTable.trim());
      const pairKey = `${fromId}__${toId}`;
      if (seen.has(pairKey)) continue;
      seen.add(pairKey);
      lines.push(`  ${fromId} }o--|| ${toId} : "FK"`);
    }
  }

  return lines.join("\n");
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
  const q = (s: string) => `"${s.replace(/"/g, '""')}"`;
  const tableRef = (schema: string, name: string) =>
    `${q(database)}.${q(schema)}.${q(name.trim())}`;

  const stmts: string[] = [];

  // ── Pure-create mode (no baseline tables) ────────────────────────────────────
  if (!baseline || baseline.tables.length === 0) {
    for (const t of tables) {
      if (!t.schema || !t.name.trim() || t.columns.length === 0) continue;
      const colLines: string[] = [];
      const pkCols: string[] = [];
      const fkLines: string[] = [];
      for (const c of t.columns) {
        if (!c.name.trim()) continue;
        const nn = c.isPK || c.notNull ? " NOT NULL" : "";
        colLines.push(`    ${q(c.name.trim())} ${c.dataType}${nn}`);
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
      if (colLines.length === 0) continue;
      const allLines = [...colLines];
      if (pkCols.length > 0) allLines.push(`    PRIMARY KEY (${pkCols.join(", ")})`);
      allLines.push(...fkLines);
      stmts.push(
        `CREATE TABLE IF NOT EXISTS ${tableRef(t.schema, t.name)} (\n${allLines.join(",\n")}\n);`
      );
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
    baselineMap.set(`${t.schema.toUpperCase()}.${t.name.toUpperCase()}`, t);
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
      currentSet.add(`${t.schema.toUpperCase()}.${t.name.trim().toUpperCase()}`);
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
    const key = `${t.schema.toUpperCase()}.${t.name.trim().toUpperCase()}`;
    const bt = baselineMap.get(key);

    if (!bt) {
      // ── New table ─────────────────────────────────────────────────────────────
      const colLines: string[] = [];
      const pkCols: string[] = [];
      const fkLines: string[] = [];
      for (const c of t.columns) {
        if (!c.name.trim()) continue;
        const nn = c.isPK || c.notNull ? " NOT NULL" : "";
        colLines.push(`    ${q(c.name.trim())} ${c.dataType}${nn}`);
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
      if (colLines.length === 0) continue;
      const allLines = [...colLines];
      if (pkCols.length > 0) allLines.push(`    PRIMARY KEY (${pkCols.join(", ")})`);
      allLines.push(...fkLines);
      stmts.push(
        `CREATE TABLE IF NOT EXISTS ${tableRef(t.schema, t.name)} (\n${allLines.join(",\n")}\n);`
      );
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
          // New column
          const nn = c.isPK || c.notNull ? " NOT NULL" : "";
          stmts.push(`${alter} ADD COLUMN ${q(c.name.trim())} ${c.dataType}${nn};`);
        } else {
          // Existing column — check for type change
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

// ── Component ─────────────────────────────────────────────────────────────────

export default function ERDesigner({ database, initialData, onClose, onSuccess }: Props) {
  const [leftWidth, setLeftWidth] = useState(490);
  const [resizing, setResizing] = useState(false);
  const resizeStart = useRef({ x: 0, width: 0 });

  const [schemas, setSchemas] = useState<string[]>([]);
  const [tables, setTables] = useState<DesignerTable[]>(() =>
    initialData ? initFromERData(initialData) : []
  );

  // Canvas selection
  const [selectedTableId, setSelectedTableId] = useState<string | null>(null);

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
    ListSchemas(database)
      .then((s) => setSchemas(s.filter((n) => n.toUpperCase() !== "INFORMATION_SCHEMA")))
      .catch(() => {});
  }, [database]);

  // ── Schema filter for canvas ──────────────────────────────────────────────

  const canvasSchemas = useMemo(() => {
    const fromTables = tables.map((t) => t.schema).filter(Boolean);
    return [...new Set([...fromTables, ...schemas])]
      .filter((s) => s.toUpperCase() !== "INFORMATION_SCHEMA")
      .sort();
  }, [tables, schemas]);

  const [visibleSchemas, setVisibleSchemas] = useState<Set<string> | null>(null);

  // Show all schemas by default (null = no filter)
  const effectiveVisibleSchemas = visibleSchemas ?? new Set(canvasSchemas);

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
    if (!selectedTableId) return;
    const el = tableCardRefs.current.get(selectedTableId);
    el?.scrollIntoView({ behavior: "smooth", block: "nearest" });
  }, [selectedTableId]);

  // ── Table / column mutators ───────────────────────────────────────────────────

  const addTable = () => {
    setTables((prev) => [
      ...prev,
      { id: crypto.randomUUID(), schema: schemas[0] ?? "", name: "", columns: [] },
    ]);
  };

  const removeTable = (tableId: string) => {
    setTables((prev) => prev.filter((t) => t.id !== tableId));
    if (selectedTableId === tableId) setSelectedTableId(null);
  };

  const updateTable = useCallback((tableId: string, patch: Partial<Pick<DesignerTable, "name" | "schema">>) => {
    setTables((prev) => prev.map((t) => (t.id === tableId ? { ...t, ...patch } : t)));
  }, []);

  const addColumn = (tableId: string) => {
    setTables((prev) =>
      prev.map((t) =>
        t.id === tableId
          ? { ...t, columns: [...t.columns, { id: crypto.randomUUID(), name: "", dataType: "VARCHAR", isPK: false, notNull: false, fkRef: "" }] }
          : t
      )
    );
  };

  const removeColumn = (tableId: string, colId: string) => {
    setTables((prev) =>
      prev.map((t) => (t.id === tableId ? { ...t, columns: t.columns.filter((c) => c.id !== colId) } : t))
    );
  };

  const updateColumn = useCallback((tableId: string, colId: string, patch: Partial<DesignerColumn>) => {
    setTables((prev) =>
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
      )
    );
  }, []);

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
      const toTable = tables.find((t) => t.id === toTableId);
      if (!toTable || !toTable.schema || !toTable.name.trim()) return;
      const toCol = toTable.columns.find((c) => c.id === toColId);
      if (!toCol || !toCol.name.trim()) return;

      const fkRef = `${toTable.schema}.${toTable.name.trim()}.${toCol.name.trim()}`;
      updateColumn(fromTableId, fromColId, { fkRef });
    },
    [tables, updateColumn],
  );

  // ── SQL & run ─────────────────────────────────────────────────────────────────

  const sql = generateDiffSQL(tables, database, initialData);
  const hasChanges = sql.trim().length > 0;

  const runSQL = async () => {
    setRunning(true);
    setRunError(null);
    try {
      await ExecuteQuery(sql);
      antMessage.success("Changes applied successfully.");
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
    Modal.confirm({
      title: "Discard unsaved changes?",
      content: "You have unapplied schema changes. Close anyway?",
      okText: "Discard changes",
      okButtonProps: { danger: true },
      cancelText: "Keep editing",
      onOk: onClose,
    });
  };

  const copyMermaid = () => {
    navigator.clipboard.writeText(buildDesignerMermaid(tables));
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
            <Button size="small" icon={<PlusOutlined />} onClick={addTable} style={{ alignSelf: "flex-start" }}>
              Add Table
            </Button>

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
                  borderLeft: selectedTableId === t.id ? "3px solid var(--accent)" : "1px solid var(--border)",
                  cursor: "pointer",
                }}
                onClick={() => setSelectedTableId(t.id)}
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
                      onClick={(e) => { e.stopPropagation(); removeTable(t.id); }}
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
                          if (SF_DATA_TYPES.some((dt) => dt.name === base)) return true;
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

            <ERCanvas
              tables={tables}
              mode="edit"
              database={database}
              visibleSchemas={effectiveVisibleSchemas}
              selectedTableId={selectedTableId}
              onTableSelect={setSelectedTableId}
              onConnect={handleFKConnect}
              onTableRename={handleTableRename}
              onColumnRename={handleColumnRename}
            />
          </div>
        </div>
      </Modal>

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
              <Button onClick={() => navigator.clipboard.writeText(sql)} icon={<CopyOutlined />}>Copy</Button>
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
