// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useCallback, useEffect, useRef, useState } from "react";
import Editor, { type BeforeMount, type OnMount } from "@monaco-editor/react";
import { ensureMonacoSetup } from "../editor/monacoSetup";
import { Button, Space, Spin, Tooltip, Typography, Select, Tag, message } from "antd";
import {
  PlayCircleOutlined,
  PlusOutlined,
  DeleteOutlined,
  CaretUpOutlined,
  CaretDownOutlined,
  SaveOutlined,
  ReloadOutlined,
  CopyOutlined,
  CloudUploadOutlined,
} from "@ant-design/icons";
import {
  StartNotebookSession,
  RunNotebookCell,
  RunNotebookSql,
  StopNotebookSession,
  SaveNotebook,
  GetTableColumns,
} from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import { ClipboardGetText, ClipboardSetText } from "../../../wailsjs/runtime/runtime";

// Install a document-level capture-phase Cmd+C handler that routes clipboard
// writes through the Wails native API.  WKWebView blocks all web-content
// clipboard writes, so we must do this explicitly for every focusable surface.
function installCopyHandler(containerEl: HTMLElement): () => void {
  const handler = (e: KeyboardEvent) => {
    if (!(e.metaKey || e.ctrlKey) || e.key !== "c") return;
    const sel = window.getSelection();
    if (!sel || sel.toString() === "") return;
    const anchor = sel.anchorNode;
    if (!anchor || !containerEl.contains(anchor)) return;
    // Let the textarea's own keydown handler manage its copy.
    let node: Node | null = anchor;
    while (node && node !== containerEl) {
      if (node instanceof HTMLTextAreaElement) return;
      node = node.parentNode;
    }
    e.preventDefault();
    e.stopPropagation();
    ClipboardSetText(sel.toString());
  };
  document.addEventListener("keydown", handler, true);
  return () => document.removeEventListener("keydown", handler, true);
}
import { useQueryStore } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";
import DeployNotebookModal from "./DeployNotebookModal";

const { Text } = Typography;

// ─── notebook JSON types (Jupyter nbformat v4) ────────────────────────────────

interface RawOutput {
  output_type: string;
  name?: string;
  text?: string | string[];
  data?: Record<string, string | string[]>;
  ename?: string;
  evalue?: string;
  traceback?: string[];
}

interface RawCell {
  id?: string;
  cell_type: "code" | "markdown" | "raw";
  source: string | string[];
  outputs?: RawOutput[];
  execution_count?: number | null;
  metadata?: Record<string, unknown>;
}

interface RawNotebook {
  nbformat?: number;
  nbformat_minor?: number;
  metadata?: Record<string, unknown>;
  cells: RawCell[];
}

// ─── internal cell state ──────────────────────────────────────────────────────

interface CellOutput {
  type: "stdout" | "stderr" | "error" | "text";
  text: string;
}

type SqlResult = main.NotebookSqlResult;

interface Cell {
  id: string;
  kind: "code" | "markdown" | "sql";
  source: string;
  outputs: CellOutput[];
  images: string[];   // base64-encoded PNG plots from matplotlib
  sqlResult: SqlResult | null;
  executionCount: number | null;
  running: boolean;
}

// ─── helpers ──────────────────────────────────────────────────────────────────

function toStr(v: string | string[] | undefined): string {
  if (!v) return "";
  return Array.isArray(v) ? v.join("") : v;
}

function parseNotebook(json: string): { cells: Cell[]; raw: RawNotebook } {
  let raw: RawNotebook = { cells: [] };
  try { raw = JSON.parse(json); } catch { /* use empty */ }

  const cells: Cell[] = (raw.cells ?? []).map((c, i) => {
    const isSql = c.cell_type === "raw"
      ? c.metadata?.["thaw_cell_type"] === "sql"
      : c.metadata?.["thaw_cell_type"] === "sql";
    const kind: Cell["kind"] = isSql ? "sql"
      : c.cell_type === "markdown" ? "markdown"
      : "code";
    return {
      id: c.id ?? String(i),
      kind,
      source: toStr(c.source),
      outputs: mapOutputs(c.outputs ?? []),
      images: [],
      sqlResult: null,
      executionCount: c.execution_count ?? null,
      running: false,
    };
  });
  return { cells, raw };
}

function mapOutputs(raw: RawOutput[]): CellOutput[] {
  return raw.flatMap((o): CellOutput[] => {
    if (o.output_type === "stream") {
      return [{ type: o.name === "stderr" ? "stderr" : "stdout", text: toStr(o.text) }];
    }
    if (o.output_type === "error") {
      return [{ type: "error", text: [o.ename, o.evalue, ...(o.traceback ?? [])].filter(Boolean).join("\n") }];
    }
    if (o.output_type === "execute_result" || o.output_type === "display_data") {
      const txt = toStr(o.data?.["text/plain"]);
      return txt ? [{ type: "text", text: txt }] : [];
    }
    return [];
  });
}

function serializeNotebook(raw: RawNotebook, cells: Cell[]): string {
  const outCells: RawCell[] = cells.map((c) => ({
    id: c.id,
    cell_type: c.kind === "sql" ? "raw" : c.kind,
    source: c.source,
    metadata: c.kind === "sql" ? { thaw_cell_type: "sql" } : {},
    outputs: c.kind === "code" ? serializeOutputs(c.outputs) : undefined,
    execution_count: c.kind === "code" ? c.executionCount : undefined,
  }));
  const nb: RawNotebook = {
    nbformat: raw.nbformat ?? 4,
    nbformat_minor: raw.nbformat_minor ?? 5,
    metadata: raw.metadata ?? { kernelspec: { display_name: "Python 3", language: "python", name: "python3" }, language_info: { name: "python" } },
    cells: outCells,
  };
  return JSON.stringify(nb, null, 1);
}

function serializeOutputs(outputs: CellOutput[]): RawOutput[] {
  return outputs.map((o): RawOutput => {
    if (o.type === "error") {
      const lines = o.text.split("\n");
      return { output_type: "error", ename: lines[0] ?? "", evalue: lines[1] ?? "", traceback: lines };
    }
    return { output_type: "stream", name: o.type === "stderr" ? "stderr" : "stdout", text: o.text };
  });
}

// ─── component ────────────────────────────────────────────────────────────────

interface Props {
  tabId: string;
}

export default function NotebookTab({ tabId }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const isDark   = resolved === "dark";

  const tab       = useQueryStore((s) => s.tabs.find((t) => t.id === tabId));
  const setSql    = useQueryStore((s) => s.setSql);
  const markSaved = useQueryStore((s) => s.markSaved);

  const containerRef = useRef<HTMLDivElement>(null);

  const [cells, setCells] = useState<Cell[]>([]);
  const [rawNb, setRawNb] = useState<RawNotebook>({ cells: [] });
  const [kernelReady, setKernelReady] = useState(false);
  const [kernelStarting, setKernelStarting] = useState(false);
  const [kernelError, setKernelError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [deployOpen, setDeployOpen] = useState(false);
  // Snapshot of serialized notebook content captured when the Deploy modal is opened.
  // Used for unsaved notebooks that have no on-disk file path.
  const [deployContent, setDeployContent] = useState("");

  // Track current cells in a ref to avoid stale closures in the serializer.
  const cellsRef = useRef(cells);
  useEffect(() => { cellsRef.current = cells; }, [cells]);

  // ── copy handler for output areas ────────────────────────────────────────
  useEffect(() => {
    if (!containerRef.current) return;
    return installCopyHandler(containerRef.current);
  }, []);

  // ── parse notebook JSON when the tab content changes ──────────────────────
  useEffect(() => {
    if (!tab?.sql) return;
    const { cells: parsed, raw } = parseNotebook(tab.sql);
    setCells(parsed);
    setRawNb(raw);
  }, [tab?.path]); // only re-parse when the file changes, not on every edit

  // ── start kernel on mount, stop on unmount ────────────────────────────────
  useEffect(() => {
    setKernelStarting(true);
    setKernelError(null);
    StartNotebookSession(tabId)
      .then(() => { setKernelReady(true); setKernelStarting(false); })
      .catch((e) => { setKernelError(String(e)); setKernelStarting(false); });

    return () => { StopNotebookSession(tabId).catch(() => {}); };
  }, [tabId]);

  // ── helpers ───────────────────────────────────────────────────────────────

  const patchCell = useCallback((id: string, patch: Partial<Cell>) => {
    setCells((prev) => prev.map((c) => (c.id === id ? { ...c, ...patch } : c)));
  }, []);

  const syncToStore = useCallback((updated: Cell[]) => {
    const json = serializeNotebook(rawNb, updated);
    setSql(json);
  }, [rawNb, setSql]);

  const runCell = useCallback(async (cell: Cell, code?: string) => {
    if (cell.running) return;

    const codeToRun = code ?? cell.source;

    if (cell.kind === "sql") {
      patchCell(cell.id, { running: true, sqlResult: null, outputs: [], images: [] });
      try {
        const result = await RunNotebookSql(codeToRun);
        setCells((prev) => {
          const updated = prev.map((c) =>
            c.id === cell.id
              ? { ...c, running: false, sqlResult: result, executionCount: (c.executionCount ?? 0) + 1 }
              : c,
          );
          syncToStore(updated);
          return updated;
        });
      } catch (e) {
        patchCell(cell.id, {
          running: false,
          outputs: [{ type: "error", text: String(e) }],
        });
      }
      return;
    }

    if (!kernelReady) return;
    patchCell(cell.id, { running: true, outputs: [], images: [], sqlResult: null });
    try {
      const out = await RunNotebookCell(tabId, codeToRun);
      const outputs: CellOutput[] = [];
      if (out.stdout) outputs.push({ type: "stdout", text: out.stdout });
      if (out.stderr) outputs.push({ type: "stderr", text: out.stderr });
      if (out.error)  outputs.push({ type: "error",  text: out.error  });
      const images = out.images ?? [];
      setCells((prev) => {
        const updated = prev.map((c) =>
          c.id === cell.id
            ? { ...c, running: false, outputs, images, executionCount: (c.executionCount ?? 0) + 1 }
            : c,
        );
        syncToStore(updated);
        return updated;
      });
    } catch (e) {
      patchCell(cell.id, {
        running: false,
        outputs: [{ type: "error", text: String(e) }],
      });
    }
  }, [kernelReady, patchCell, syncToStore, tabId]);

  const runAll = useCallback(async () => {
    for (const cell of cellsRef.current) {
      if (cell.kind === "code") await runCell(cell);
    }
  }, [runCell]);

  const restartKernel = useCallback(async () => {
    await StopNotebookSession(tabId).catch(() => {});
    setKernelReady(false);
    setKernelStarting(true);
    setKernelError(null);
    StartNotebookSession(tabId)
      .then(() => { setKernelReady(true); setKernelStarting(false); })
      .catch((e) => { setKernelError(String(e)); setKernelStarting(false); });
  }, [tabId]);

  const saveNotebook = useCallback(async () => {
    if (!tab?.path) { message.warning("No file path — use File > Save As to save."); return; }
    const json = serializeNotebook(rawNb, cellsRef.current);
    setSaving(true);
    try {
      await SaveNotebook(tab.path, json);
      markSaved(tabId, tab.path, tab.title);
      message.success("Notebook saved");
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  }, [tab, rawNb, markSaved, tabId]);

  const addCell = useCallback((afterId?: string) => {
    const newCell: Cell = {
      id: crypto.randomUUID(),
      kind: "code",
      source: "",
      outputs: [],
      images: [],
      sqlResult: null,
      executionCount: null,
      running: false,
    };
    setCells((prev) => {
      let updated: Cell[];
      if (!afterId) {
        updated = [...prev, newCell];
      } else {
        const idx = prev.findIndex((c) => c.id === afterId);
        updated = [...prev.slice(0, idx + 1), newCell, ...prev.slice(idx + 1)];
      }
      syncToStore(updated);
      return updated;
    });
  }, [syncToStore]);

  const deleteCell = useCallback((id: string) => {
    setCells((prev) => {
      const updated = prev.filter((c) => c.id !== id);
      syncToStore(updated);
      return updated;
    });
  }, [syncToStore]);

  const moveCell = useCallback((id: string, dir: -1 | 1) => {
    setCells((prev) => {
      const idx = prev.findIndex((c) => c.id === id);
      const to  = idx + dir;
      if (to < 0 || to >= prev.length) return prev;
      const next = [...prev];
      [next[idx], next[to]] = [next[to], next[idx]];
      syncToStore(next);
      return next;
    });
  }, [syncToStore]);

  const updateSource = useCallback((id: string, source: string) => {
    setCells((prev) => {
      const updated = prev.map((c) => (c.id === id ? { ...c, source } : c));
      syncToStore(updated);
      return updated;
    });
  }, [syncToStore]);

  const setCellKind = useCallback((id: string, kind: Cell["kind"]) => {
    setCells((prev) => {
      const updated = prev.map((c) => (c.id === id ? { ...c, kind, outputs: [], images: [] } : c));
      syncToStore(updated);
      return updated;
    });
  }, [syncToStore]);

  // ── styles ────────────────────────────────────────────────────────────────

  const bg      = isDark ? "#1e1e1e" : "#ffffff";
  const bgRaised = isDark ? "#252526" : "#f5f5f5";
  const border  = isDark ? "#3c3c3c" : "#d9d9d9";
  const textMuted = isDark ? "#8b949e" : "#6e7781";

  // ── render ────────────────────────────────────────────────────────────────

  return (
    <div ref={containerRef} style={{ display: "flex", flexDirection: "column", height: "100%", background: bg, overflow: "hidden" }}>
      {/* Toolbar */}
      <div style={{
        padding: "4px 12px",
        borderBottom: `1px solid ${border}`,
        display: "flex",
        alignItems: "center",
        gap: 8,
        background: bgRaised,
        flexShrink: 0,
      }}>
        <Space size={4}>
          <Tooltip title="Run all cells">
            <Button
              icon={<PlayCircleOutlined />}
              size="small"
              type="primary"
              onClick={runAll}
              disabled={!kernelReady}
            >
              Run All
            </Button>
          </Tooltip>
          <Tooltip title="Restart kernel">
            <Button icon={<ReloadOutlined />} size="small" onClick={restartKernel} />
          </Tooltip>
          <Tooltip title="Save notebook">
            <Button icon={<SaveOutlined />} size="small" loading={saving} onClick={saveNotebook} />
          </Tooltip>
          <Button icon={<PlusOutlined />} size="small" onClick={() => addCell()}>
            Add Cell
          </Button>
        </Space>

        <div style={{ flex: 1 }} />

        <Tooltip title="Deploy this notebook to Snowflake">
          <Button
            icon={<CloudUploadOutlined />}
            size="small"
            onClick={() => {
              setDeployContent(serializeNotebook(rawNb, cellsRef.current));
              setDeployOpen(true);
            }}
          >
            Deploy
          </Button>
        </Tooltip>

        {/* Kernel status */}
        {kernelStarting && <Spin size="small" />}
        {kernelStarting && <Text style={{ fontSize: 11, color: textMuted }}>Starting kernel…</Text>}
        {kernelReady && !kernelStarting && (
          <Tag color="success" style={{ fontSize: 10 }}>Kernel ready</Tag>
        )}
        {kernelError && (
          <Tag color="error" style={{ fontSize: 10 }} title={kernelError}>Kernel error</Tag>
        )}
      </div>

      {/* Cell list */}
      <div style={{ flex: 1, overflowY: "auto", padding: "12px 0" }}>
        {cells.length === 0 && (
          <div style={{ textAlign: "center", padding: 32 }}>
            <Text type="secondary" style={{ fontSize: 13 }}>
              Empty notebook.{" "}
            </Text>
            <Button type="link" size="small" onClick={() => addCell()}>
              Add a cell
            </Button>
          </div>
        )}

        {cells.map((cell, idx) => (
          <CellView
            key={cell.id}
            cell={cell}
            isFirst={idx === 0}
            isLast={idx === cells.length - 1}
            kernelReady={kernelReady}
            isDark={isDark}
            border={border}
            bgRaised={bgRaised}
            textMuted={textMuted}
            onRun={(code) => runCell(cell, code)}
            onDelete={() => deleteCell(cell.id)}
            onMoveUp={() => moveCell(cell.id, -1)}
            onMoveDown={() => moveCell(cell.id, 1)}
            onSourceChange={(s) => updateSource(cell.id, s)}
            onKindChange={(k) => setCellKind(cell.id, k)}
            onAddAfter={() => addCell(cell.id)}
          />
        ))}
      </div>

      <DeployNotebookModal
        open={deployOpen}
        filePath={tab?.path ?? ""}
        content={deployContent}
        defaultName={tab?.title ?? "notebook"}
        onClose={() => setDeployOpen(false)}
        onDeployed={() => setDeployOpen(false)}
      />
    </div>
  );
}

// ─── SqlResultTable ───────────────────────────────────────────────────────────

function SqlResultTable({ result, isDark, border }: {
  result: SqlResult;
  isDark: boolean;
  border: string;
}) {
  const MAX_ROWS = 1000;
  const rows = result.rows.slice(0, MAX_ROWS);
  const bgHead = isDark ? "#2d2d2d" : "#f0f0f0";
  const bgRow  = isDark ? "#1e1e1e" : "#ffffff";
  const bgAlt  = isDark ? "#252525" : "#fafafa";
  const text   = isDark ? "#d4d4d4" : "#1f2328";

  return (
    <div style={{ borderTop: `1px solid ${border}`, overflow: "auto", maxHeight: 360 }}>
      <table style={{
        borderCollapse: "collapse",
        fontSize: 12,
        fontFamily: "monospace",
        width: "max-content",
        minWidth: "100%",
      }}>
        <thead>
          <tr>
            {result.columns.map((col) => (
              <th key={col} style={{
                padding: "4px 10px",
                textAlign: "left",
                background: bgHead,
                color: text,
                borderBottom: `1px solid ${border}`,
                whiteSpace: "nowrap",
                position: "sticky",
                top: 0,
                fontWeight: 600,
              }}>
                {col}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, ri) => (
            <tr key={ri} style={{ background: ri % 2 === 0 ? bgRow : bgAlt }}>
              {row.map((cell, ci) => (
                <td key={ci} style={{
                  padding: "3px 10px",
                  color: text,
                  borderBottom: `1px solid ${border}`,
                  whiteSpace: "nowrap",
                  maxWidth: 400,
                  overflow: "hidden",
                  textOverflow: "ellipsis",
                }}>
                  {cell === null || cell === undefined ? (
                    <span style={{ opacity: 0.4 }}>NULL</span>
                  ) : String(cell)}
                </td>
              ))}
            </tr>
          ))}
        </tbody>
      </table>
      <div style={{ padding: "4px 10px", fontSize: 11, color: isDark ? "#6e7781" : "#6e7781" }}>
        {result.rows.length > MAX_ROWS
          ? `Showing first ${MAX_ROWS} of ${result.rows.length} rows`
          : `${result.rows.length} row${result.rows.length !== 1 ? "s" : ""}`}
        {result.queryID ? `  ·  ${result.queryID}` : ""}
      </div>
    </div>
  );
}

// ─── CellView ─────────────────────────────────────────────────────────────────

interface CellViewProps {
  cell: Cell;
  isFirst: boolean;
  isLast: boolean;
  kernelReady: boolean;
  isDark: boolean;
  border: string;
  bgRaised: string;
  textMuted: string;
  onRun: (code?: string) => void;
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onSourceChange: (s: string) => void;
  onKindChange: (k: Cell["kind"]) => void;
  onAddAfter: () => void;
}

function CellView({
  cell, isFirst, isLast, kernelReady, isDark, border, bgRaised, textMuted,
  onRun, onDelete, onMoveUp, onMoveDown, onSourceChange, onKindChange, onAddAfter,
}: CellViewProps) {
  const [focused, setFocused] = useState(false);
  const accentColor = isDark ? "#40c8fc" : "#0969da";

  // ── Monaco editor setup ───────────────────────────────────────────────────
  const editorRef = useRef<any>(null);
  const onRunRef  = useRef(onRun);
  useEffect(() => { onRunRef.current = onRun; }, [onRun]);

  const [editorHeight, setEditorHeight] = useState(60);

  const handleBeforeMount: BeforeMount = (monaco) => {
    ensureMonacoSetup(monaco);
  };

  const handleMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;

    // Auto-size height to content.
    const updateHeight = () =>
      setEditorHeight(Math.max(60, editor.getContentHeight()));
    editor.onDidContentSizeChange(updateHeight);
    updateHeight();

    // Focus border.
    editor.onDidFocusEditorWidget(() => setFocused(true));
    editor.onDidBlurEditorWidget(() => setFocused(false));

    // Shift+Enter: run selection if text is selected, otherwise run full cell.
    editor.addCommand(
      monaco.KeyMod.Shift | monaco.KeyCode.Enter,
      () => {
        if (cell.kind === "markdown") return;
        const sel = editor.getSelection();
        const model = editor.getModel();
        if (sel && !sel.isEmpty() && model) {
          onRunRef.current(model.getValueInRange(sel));
        } else {
          onRunRef.current();
        }
      },
    );

    // WKWebView clipboard fix — same pattern as SqlEditor.
    const doCopy = async () => {
      const sel = editor.getSelection();
      const model = editor.getModel();
      if (!sel || !model) return;
      const text = model.getValueInRange(sel);
      if (text) await ClipboardSetText(text);
    };
    const doPaste = async () => {
      const text = await ClipboardGetText();
      if (!text) return;
      const sel = editor.getSelection();
      if (!sel) return;
      editor.executeEdits("clipboard-paste", [{ range: sel, text, forceMoveMarkers: true }]);
      editor.pushUndoStop();
    };
    const doCut = async () => {
      const sel = editor.getSelection();
      const model = editor.getModel();
      if (!sel || !model) return;
      const text = model.getValueInRange(sel);
      if (!text) return;
      await ClipboardSetText(text);
      editor.executeEdits("clipboard-cut", [{ range: sel, text: "", forceMoveMarkers: true }]);
      editor.pushUndoStop();
    };
    const editorDom = editor.getDomNode();
    editorDom?.addEventListener("keydown", (e: KeyboardEvent) => {
      if (!(e.metaKey || e.ctrlKey)) return;
      switch (e.key.toLowerCase()) {
        case "c": e.preventDefault(); e.stopPropagation(); doCopy(); break;
        case "v": e.preventDefault(); e.stopPropagation(); doPaste(); break;
        case "x": e.preventDefault(); e.stopPropagation(); doCut(); break;
      }
    }, true /* capture */);

    // Drag-and-drop from sidebar (same payload as SqlEditor).
    editorDom?.addEventListener("dragover", (e: DragEvent) => {
      if (e.dataTransfer?.types.includes("thaw/table")) {
        e.preventDefault();
        e.dataTransfer.dropEffect = "copy";
      }
    });

    editorDom?.addEventListener("drop", async (e: DragEvent) => {
      const raw = e.dataTransfer?.getData("thaw/table");
      if (!raw) return;
      e.preventDefault();
      let info: { db: string; schema: string; name: string };
      try { info = JSON.parse(raw); } catch { return; }

      const target = editor.getTargetAtClientPoint(e.clientX, e.clientY);
      const pos = target?.position ?? editor.getPosition() ?? { lineNumber: 1, column: 1 };

      const esc = (s: string) => s.replace(/"/g, '""');
      const fqn = `"${esc(info.db)}"."${esc(info.schema)}"."${esc(info.name)}"`;

      let text: string;
      if (cell.kind === "sql") {
        try {
          const columns = await GetTableColumns(info.db, info.schema, info.name);
          const colList = columns.map((c) => `    "${esc(c)}"`).join(",\n");
          text = `SELECT\n${colList}\nFROM ${fqn};`;
        } catch {
          text = `SELECT *\nFROM ${fqn};`;
        }
      } else {
        // Code cell: generate a Snowpark DataFrame snippet.
        try {
          const columns = await GetTableColumns(info.db, info.schema, info.name);
          text = `df = session.table('${fqn}')\n# columns: ${columns.join(", ")}\ndf.show()`;
        } catch {
          text = `df = session.table('${fqn}')\ndf.show()`;
        }
      }

      const range = {
        startLineNumber: pos.lineNumber,
        endLineNumber:   pos.lineNumber,
        startColumn:     pos.column,
        endColumn:       pos.column,
      };
      editor.executeEdits("drag-drop", [{ range, text, forceMoveMarkers: true }]);
      editor.pushUndoStop();
      editor.focus();
    });
  };

  return (
    <div style={{ margin: "0 20px 8px", position: "relative" }}>
      <div style={{
        border: `1px solid ${focused ? accentColor : border}`,
        borderRadius: 6,
        overflow: "hidden",
        transition: "border-color 0.15s",
      }}>
        {/* Cell header */}
        <div style={{
          display: "flex",
          alignItems: "center",
          gap: 4,
          padding: "2px 6px",
          background: bgRaised,
          borderBottom: `1px solid ${border}`,
        }}>
          {/* Execution count */}
          <Text style={{ fontSize: 10, color: textMuted, fontFamily: "monospace", minWidth: 28 }}>
            {cell.kind === "code"
              ? (cell.executionCount !== null ? `[${cell.executionCount}]` : "[ ]")
              : "md"}
          </Text>

          {/* Cell type selector */}
          <Select
            size="small"
            value={cell.kind}
            onChange={onKindChange}
            style={{ width: 88, fontSize: 11 }}
            options={[
              { value: "code",     label: "Code" },
              { value: "markdown", label: "Markdown" },
              { value: "sql",      label: "SQL" },
            ]}
          />

          <div style={{ flex: 1 }} />

          {/* Actions */}
          <Tooltip title="Run cell (Shift+Enter) · Run selection (select text, then Shift+Enter)">
            <Button
              type="text"
              size="small"
              icon={cell.running ? <Spin size="small" /> : <PlayCircleOutlined />}
              disabled={cell.kind === "markdown" || (cell.kind === "code" && !kernelReady)}
              onClick={() => onRun()}
              style={{ color: accentColor }}
            />
          </Tooltip>
          <Tooltip title="Move up">
            <Button type="text" size="small" icon={<CaretUpOutlined />}
              disabled={isFirst} onClick={onMoveUp} />
          </Tooltip>
          <Tooltip title="Move down">
            <Button type="text" size="small" icon={<CaretDownOutlined />}
              disabled={isLast} onClick={onMoveDown} />
          </Tooltip>
          <Tooltip title="Add cell below">
            <Button type="text" size="small" icon={<PlusOutlined />} onClick={onAddAfter} />
          </Tooltip>
          <Tooltip title="Delete cell">
            <Button type="text" size="small" danger icon={<DeleteOutlined />} onClick={onDelete} />
          </Tooltip>
        </div>

        {/* Source editor — Monaco */}
        <div style={{ height: editorHeight }}>
          <Editor
            key={`${cell.id}:${cell.kind}`}
            defaultValue={cell.source}
            language={cell.kind === "sql" ? "sql" : cell.kind === "code" ? "python" : "markdown"}
            theme={isDark ? "thaw-dark" : "thaw-light"}
            beforeMount={handleBeforeMount}
            onMount={handleMount}
            onChange={(v) => onSourceChange(v ?? "")}
            options={{
              fontSize: 13,
              fontFamily: "\"JetBrains Mono\", \"Fira Code\", \"Cascadia Code\", monospace",
              minimap: { enabled: false },
              lineNumbers: "off",
              scrollBeyondLastLine: false,
              wordWrap: "on",
              scrollbar: {
                vertical: "hidden",
                horizontal: "hidden",
                alwaysConsumeMouseWheel: false,
              },
              overviewRulerLanes: 0,
              overviewRulerBorder: false,
              renderLineHighlight: "none",
              padding: { top: 8, bottom: 8 },
              contextmenu: false,
              automaticLayout: true,
              fixedOverflowWidgets: true,
            }}
          />
        </div>

        {/* Code cell outputs */}
        {cell.kind === "code" && (cell.outputs.length > 0 || cell.images.length > 0) && (
          <div style={{ borderTop: `1px solid ${border}`, background: isDark ? "#1a1a1a" : "#fafafa" }}>
            {cell.outputs.map((out, i) => (
              <div key={i} style={{ position: "relative" }}>
                <pre style={{
                  margin: 0,
                  padding: "6px 36px 6px 10px",
                  fontFamily: "monospace",
                  fontSize: 12,
                  lineHeight: "1.5",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                  background: "transparent",
                  color: out.type === "error"  ? "#ff4d4f"
                       : out.type === "stderr" ? "#e6a817"
                       : isDark ? "#d4d4d4" : "#1f2328",
                }}>
                  {out.text}
                </pre>
                <Tooltip title="Copy output">
                  <Button
                    type="text"
                    size="small"
                    icon={<CopyOutlined />}
                    onClick={() => ClipboardSetText(out.text)}
                    style={{ position: "absolute", top: 4, right: 4, opacity: 0.45, fontSize: 11 }}
                  />
                </Tooltip>
              </div>
            ))}
            {cell.images.map((b64, i) => (
              <div key={`img-${i}`} style={{ padding: "8px 10px" }}>
                <img
                  src={`data:image/png;base64,${b64}`}
                  alt={`plot ${i + 1}`}
                  style={{ maxWidth: "100%", display: "block", borderRadius: 4 }}
                />
              </div>
            ))}
          </div>
        )}

        {/* SQL cell error output */}
        {cell.kind === "sql" && cell.outputs.length > 0 && (
          <div style={{ borderTop: `1px solid ${border}` }}>
            {cell.outputs.map((out, i) => (
              <pre key={i} style={{
                margin: 0,
                padding: "6px 10px",
                fontFamily: "monospace",
                fontSize: 12,
                whiteSpace: "pre-wrap",
                wordBreak: "break-word",
                background: isDark ? "#1a1a1a" : "#fafafa",
                color: "#ff4d4f",
              }}>
                {out.text}
              </pre>
            ))}
          </div>
        )}

        {/* SQL result table */}
        {cell.kind === "sql" && cell.sqlResult && cell.sqlResult.columns.length > 0 && (
          <SqlResultTable result={cell.sqlResult} isDark={isDark} border={border} />
        )}
        {cell.kind === "sql" && cell.sqlResult && cell.sqlResult.columns.length === 0 && (
          <div style={{
            borderTop: `1px solid ${border}`,
            padding: "6px 10px",
            fontSize: 12,
            color: textMuted,
            fontFamily: "monospace",
          }}>
            OK — {cell.sqlResult.rowCount} row{cell.sqlResult.rowCount !== 1 ? "s" : ""} affected
            {cell.sqlResult.queryID ? `  ·  ${cell.sqlResult.queryID}` : ""}
          </div>
        )}
      </div>
    </div>
  );
}
