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
import Editor, { type BeforeMount, type OnMount, type Monaco } from "@monaco-editor/react";
import { ensureMonacoSetup } from "../editor/monacoSetup";
import { setActiveSnippetEditor } from "../editor/SqlEditor";
import { getPythonSnippets, PYTHON_SNIPPET_CATEGORIES } from "../editor/snowflakeSnippets";
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { MenuRegistry, MenuId } from "monaco-editor/esm/vs/platform/actions/common/actions.js";
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { CommandsRegistry } from "monaco-editor/esm/vs/platform/commands/common/commands.js";
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { ContextKeyExpr } from "monaco-editor/esm/vs/platform/contextkey/common/contextkey.js";
import * as monacoLib from "monaco-editor";
import { Button, Modal, Space, Spin, Tooltip, Typography, Select, Tag, message } from "antd";
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
  RunNotebookCellSql,
  StopNotebookSession,
  SaveNotebook,
  GetTableColumns,
  GetNotebookCompletions,
  GetNotebookHover,
  CheckPythonSyntax,
} from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";
import { ClipboardGetText, ClipboardSetText, EventsOn } from "../../../wailsjs/runtime/runtime";
import { useNotebookPrefsStore } from "../../store/notebookPrefsStore";

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
// ─── Python snippet context menu (registered once at module load) ─────────────
let _pythonSnippetMenuRegistered = false;
(() => {
  if (_pythonSnippetMenuRegistered) return;
  _pythonSnippetMenuRegistered = true;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let pySubMenuId: any;
  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    pySubMenuId = new (MenuId as any)("thaw.python.snippets.submenu");
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    pySubMenuId = (MenuId as any)._instances?.get("thaw.python.snippets.submenu");
  }
  if (!pySubMenuId) return;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (MenuRegistry as any).appendMenuItem((MenuId as any).EditorContext, {
    submenu: pySubMenuId,
    title: "Python Snippets",
    group: "9_snippets",
    order: 1,
    when: ContextKeyExpr.equals("editorLangId", "python"),
  });

  const snippetItems = getPythonSnippets(monacoLib);
  const snippetMap   = new Map(snippetItems.map((s) => [String(s.label), s]));

  PYTHON_SNIPPET_CATEGORIES.forEach((cat, gi) => {
    cat.labels.forEach((lbl, li) => {
      const s = snippetMap.get(lbl);
      if (!s) return;
      const cmdId = `thaw.python.snippet.${lbl}`;

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (CommandsRegistry as any).registerCommand(cmdId, () => {
        // _activeSnippetEditor is set by the cell editor's onContextMenu handler.
        // We access it via setActiveSnippetEditor's closure in SqlEditor.tsx; since
        // we can't import the private var we keep a local shadow updated below.
        if (!_lastPythonEditor) return;
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const ctrl = (_lastPythonEditor as any).getContribution("snippetController2");
        if (ctrl) ctrl.insert(s.insertText as string);
        _lastPythonEditor.focus();
      });

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (MenuRegistry as any).appendMenuItem(pySubMenuId, {
        command: { id: cmdId, title: cat.titles?.[lbl] ?? lbl },
        group: `${gi + 1}`,
        order: li,
      });
    });
  });
})();

/** Tracks the most-recently right-clicked Python cell editor for snippet insertion. */
let _lastPythonEditor: monacoLib.editor.ICodeEditor | null = null;

import { useQueryStore } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";
import { useSessionStore } from "../../store/sessionStore";
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

  const tab         = useQueryStore((s) => s.tabs.find((t) => t.id === tabId));
  const setSql      = useQueryStore((s) => s.setSql);
  const markSaved   = useQueryStore((s) => s.markSaved);
  const loadContext = useSessionStore((s) => s.loadContext);
  const syntaxMode  = useNotebookPrefsStore((s) => s.prefs.syntaxMode);

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

  // Command-mode cell selection.
  const [selectedCellId, setSelectedCellId] = useState<string | null>(null);
  const selectedCellIdRef = useRef<string | null>(null);
  useEffect(() => { selectedCellIdRef.current = selectedCellId; }, [selectedCellId]);
  // D+D detection: store timestamp of last "d" keypress.
  const lastDPressRef = useRef<number>(0);

  // Track current cells in a ref to avoid stale closures in the serializer.
  const cellsRef = useRef(cells);
  // Tracks all Monaco models so the global syntax checker can apply markers to them
  const cellModelsRef = useRef<Map<string, monacoLib.editor.ITextModel>>(new Map());
  
  // ── Global cross-cell Python syntax check ─────────────────────────────────
  useEffect(() => {
    if (!kernelReady) return;

    // When diagnostics are disabled, clear any stale markers and bail out.
    if (syntaxMode === "off") {
      for (const model of cellModelsRef.current.values()) {
        monacoLib.editor.setModelMarkers(model, "python-syntax", []);
      }
      return;
    }

    const timer = setTimeout(async () => {
      let fullCode = "";
      let currentLineCount = 0;

      // 1. Build the full notebook source string
      for (const c of cells) {
        if (c.kind !== "code") continue;
        fullCode += c.source + "\n";
        currentLineCount += c.source.split("\n").length;
      }

      if (!fullCode.trim()) {
         for (const model of cellModelsRef.current.values()) {
           monacoLib.editor.setModelMarkers(model, "python-syntax", []);
         }
         return;
      }

      try {
        const errors = await CheckPythonSyntax(tabId, fullCode, syntaxMode);
        if (!errors) return;

        const markersByCell = new Map<string, monacoLib.editor.IMarkerData[]>();

        // 2. Map the global line numbers back to their specific cells
        for (const err of errors) {
          let targetCellId = "";
          let offset = 0;
          let lineAccum = 0;

          for (const c of cells) {
            if (c.kind !== "code") continue;
            const cellLines = c.source.split("\n").length;
            if (err.line > lineAccum && err.line <= lineAccum + cellLines) {
              targetCellId = c.id;
              offset = lineAccum;
              break;
            }
            lineAccum += cellLines;
          }

          if (targetCellId) {
            if (!markersByCell.has(targetCellId)) markersByCell.set(targetCellId, []);
            markersByCell.get(targetCellId)!.push({
              severity: err.severity === "error" ? monacoLib.MarkerSeverity.Error : monacoLib.MarkerSeverity.Warning,
              message: err.msg,
              startLineNumber: err.line - offset,
              startColumn: err.col + 1,
              endLineNumber: err.line - offset,
              endColumn: err.endCol != null ? err.endCol + 1 : err.col + 2,
            });
          }
        }

        // 3. Paint the markers onto the individual cell editors
        for (const [id, model] of cellModelsRef.current.entries()) {
           const markers = markersByCell.get(id) || [];
           monacoLib.editor.setModelMarkers(model, "python-syntax", markers);
        }
      } catch (e) {
        // ignore
      }
    }, 750);

    return () => clearTimeout(timer);
  }, [cells, kernelReady, tabId, syntaxMode]);
  
  useEffect(() => { cellsRef.current = cells; }, [cells]);

  // ── copy handler for output areas ────────────────────────────────────────
  useEffect(() => {
    if (!containerRef.current) return;
    return installCopyHandler(containerRef.current);
  }, []);

  // ── parse notebook JSON when the tab content changes ──────────────────────
  // parsedRef tracks whether we have performed the initial parse for the
  // current path.  We must not also depend on tab?.sql directly (that changes
  // on every cell edit via the serializer below and would cause infinite loops).
  // Instead we reset the flag whenever the path changes, then parse on the
  // first non-empty sql — which may arrive asynchronously after a disk refresh
  // on app restart (partialize clears notebook sql to keep storage small).
  const parsedRef = useRef(false);
  useEffect(() => { parsedRef.current = false; }, [tab?.path]);
  useEffect(() => {
    if (!tab?.sql || parsedRef.current) return;
    parsedRef.current = true;
    const { cells: parsed, raw } = parseNotebook(tab.sql);
    setCells(parsed);
    setRawNb(raw);
  }, [tab?.path, tab?.sql]); // tab?.sql triggers the initial parse after async disk refresh

  // ── start kernel on mount, stop on unmount ────────────────────────────────
  useEffect(() => {
    setKernelStarting(true);
    setKernelError(null);
    StartNotebookSession(tabId)
      .then(() => { setKernelReady(true); setKernelStarting(false); })
      .catch((e) => { setKernelError(String(e)); setKernelStarting(false); });

    return () => { StopNotebookSession(tabId).catch(() => {}); };
  }, [tabId]);

  // ── sync session context changes from Python cells back to the toolbar ────
  // When a Python cell runs session.sql("USE DATABASE X"), the Go backend syncs
  // the change to the main connection and emits this event so the toolbar reflects it.
  useEffect(() => {
    const off = EventsOn("notebook:session:context:changed", () => {
      loadContext();
    });
    return off;
  }, [loadContext]);

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
        const result = await RunNotebookCellSql(tabId, codeToRun);
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
      } finally {
        loadContext(); // refresh toolbar after USE commands
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
    setSelectedCellId(newCell.id);
  }, [syncToStore]);

  const addCellAbove = useCallback((beforeId: string) => {
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
      const idx = prev.findIndex((c) => c.id === beforeId);
      const updated = idx >= 0
        ? [...prev.slice(0, idx), newCell, ...prev.slice(idx)]
        : [newCell, ...prev];
      syncToStore(updated);
      return updated;
    });
    setSelectedCellId(newCell.id);
  }, [syncToStore]);

  const deleteCell = useCallback((id: string) => {
    setCells((prev) => {
      const updated = prev.filter((c) => c.id !== id);
      syncToStore(updated);
      return updated;
    });
  }, [syncToStore]);

  const confirmDeleteCell = useCallback((id: string) => {
    Modal.confirm({
      title: "Delete cell?",
      content: "This action cannot be undone.",
      okText: "Delete",
      okButtonProps: { danger: true },
      cancelText: "Cancel",
      onOk: () => deleteCell(id),
    });
  }, [deleteCell]);

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

  // ── Notebook command-mode keyboard shortcuts ───────────────────────────────
  // Only fire when no Monaco editor (or other input) is focused.
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const el = document.activeElement;
      if (el?.classList.contains("inputarea")) return; // Monaco focused
      if (el instanceof HTMLInputElement || el instanceof HTMLTextAreaElement) return;

      const selId = selectedCellIdRef.current ?? cellsRef.current[0]?.id ?? null;
      if (!selId) return;

      switch (e.key) {
        case "b":
          e.preventDefault();
          addCell(selId);
          return;
        case "a":
          e.preventDefault();
          addCellAbove(selId);
          return;
        case "d":
        case "D": {
          const now = Date.now();
          if (now - lastDPressRef.current < 500) {
            e.preventDefault();
            lastDPressRef.current = 0;
            confirmDeleteCell(selId);
          } else {
            lastDPressRef.current = now;
          }
          return;
        }
        case "y":
          e.preventDefault();
          setCellKind(selId, "code");
          return;
        case "m":
          e.preventDefault();
          setCellKind(selId, "markdown");
          return;
        case "s":
          e.preventDefault();
          setCellKind(selId, "sql");
          return;
      }
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [addCell, addCellAbove, confirmDeleteCell, setCellKind]);

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
            tabId={tabId}
            cell={cell}
            isFirst={idx === 0}
            isLast={idx === cells.length - 1}
            kernelReady={kernelReady}
            isDark={isDark}
            border={border}
            bgRaised={bgRaised}
            textMuted={textMuted}
            isSelected={selectedCellId === cell.id}
            onRun={(code) => runCell(cell, code)}
            onDelete={() => confirmDeleteCell(cell.id)}
            onMoveUp={() => moveCell(cell.id, -1)}
            onMoveDown={() => moveCell(cell.id, 1)}
            onSourceChange={(s) => updateSource(cell.id, s)}
            onKindChange={(k) => setCellKind(cell.id, k)}
            onAddAfter={() => addCell(cell.id)}
            onSelect={() => setSelectedCellId(cell.id)}
            onModelReady={(model) => cellModelsRef.current.set(cell.id, model)}
            onModelDispose={() => cellModelsRef.current.delete(cell.id)}
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

// ─── Python intellisense (jedi via kernel) ────────────────────────────────────

// Maps Monaco model URI → notebook tabId so the global providers know which
// kernel to query for each individual cell editor.
const cellModelTabIds = new Map<string, string>();

// Registered once — Monaco language providers are global per language.
let pythonProvidersRegistered = false;

function jediKindToMonaco(monaco: Monaco, jediType: string): number {
  const K = monaco.languages.CompletionItemKind;
  switch (jediType) {
    case "function":  return K.Function;
    case "class":     return K.Class;
    case "module":    return K.Module;
    case "keyword":   return K.Keyword;
    case "property":  return K.Property;
    case "path":      return K.File;
    case "instance":  return K.Variable;
    case "statement": return K.Variable;
    case "param":     return K.Variable;
    default:          return K.Text;
  }
}

function ensurePythonProviders(monaco: Monaco) {
  if (pythonProvidersRegistered) return;
  pythonProvidersRegistered = true;

  // Completion provider — triggered by "." and Ctrl+Space.
  monaco.languages.registerCompletionItemProvider("python", {
    triggerCharacters: ["."],
    provideCompletionItems: async (model: any, position: any) => {
      const tabId = cellModelTabIds.get(model.uri.toString());
      if (!tabId) return { suggestions: [] };
      const code = model.getValue();
      const line = position.lineNumber;           // jedi: 1-indexed
      const col  = position.column - 1;           // jedi: 0-indexed
      try {
        const items = await GetNotebookCompletions(tabId, code, line, col);
        if (!items || items.length === 0) return { suggestions: [] };
        const word = model.getWordUntilPosition(position);
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber:   position.lineNumber,
          startColumn:     word.startColumn,
          endColumn:       position.column,
        };
        return {
          suggestions: items.map((item) => ({
            label:         { label: item.label, description: item.detail },
            kind:          jediKindToMonaco(monaco, item.type),
            documentation: item.documentation
              ? { value: item.documentation, isTrusted: false }
              : undefined,
            insertText: item.label,
            range,
          })),
        };
      } catch {
        return { suggestions: [] };
      }
    },
  });

  // Hover provider — fires when the cursor rests on a token.
  monaco.languages.registerHoverProvider("python", {
    provideHover: async (model: any, position: any) => {
      const tabId = cellModelTabIds.get(model.uri.toString());
      if (!tabId) return null;
      const code = model.getValue();
      const line = position.lineNumber;
      const col  = position.column - 1;
      try {
        const hover = await GetNotebookHover(tabId, code, line, col);
        if (!hover) return null;
        return { contents: [{ value: "```python\n" + hover + "\n```" }] };
      } catch {
        return null;
      }
    },
  });
}

// ─── CellView ─────────────────────────────────────────────────────────────────

interface CellViewProps {
  tabId: string;
  cell: Cell;
  isFirst: boolean;
  isLast: boolean;
  kernelReady: boolean;
  isDark: boolean;
  border: string;
  bgRaised: string;
  textMuted: string;
  isSelected: boolean;
  onRun: (code?: string) => void;
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onSourceChange: (s: string) => void;
  onKindChange: (k: Cell["kind"]) => void;
  onAddAfter: () => void;
  onSelect: () => void;
  onModelReady?: (model: monacoLib.editor.ITextModel) => void;
  onModelDispose?: () => void;
}

function CellView({
  tabId, cell, isFirst, isLast, kernelReady, isDark, border, bgRaised, textMuted,
  isSelected, onRun, onDelete, onMoveUp, onMoveDown, onSourceChange, onKindChange, onAddAfter, onSelect, onModelReady, onModelDispose,
}: CellViewProps) {
  const [focused, setFocused] = useState(false);
  const accentColor = isDark ? "#40c8fc" : "#0969da";

  // ── Monaco editor setup ───────────────────────────────────────────────────
  const editorRef       = useRef<any>(null);
  const onRunRef        = useRef(onRun);
  const onSelectRef     = useRef(onSelect);
  useEffect(() => { onRunRef.current = onRun; }, [onRun]);
  useEffect(() => { onSelectRef.current = onSelect; }, [onSelect]);

  const [editorHeight, setEditorHeight] = useState(60);

  const handleBeforeMount: BeforeMount = (monaco) => {
    ensureMonacoSetup(monaco);
    ensurePythonProviders(monaco);
  };

  const handleMount: OnMount = (editor, monaco) => {
    editorRef.current = editor;

    // Register model → tabId mapping so the global jedi providers can route
    // requests to the correct kernel.  Clean up on editor disposal.
    if (cell.kind === "code") {
      const model = editor.getModel();
      if (model) {
        const uri = model.uri.toString();
        cellModelTabIds.set(uri, tabId);
        onModelReady?.(model);

        editor.onDidDispose(() => {
          cellModelTabIds.delete(uri);
          onModelDispose?.();
        });
      }
    }

    // Auto-size height to content.
    const updateHeight = () =>
      setEditorHeight(Math.max(60, editor.getContentHeight()));
    editor.onDidContentSizeChange(updateHeight);
    updateHeight();

    // Focus border + cell selection.
    editor.onDidFocusEditorWidget(() => { setFocused(true); onSelectRef.current(); });
    editor.onDidBlurEditorWidget(() => setFocused(false));

    // Track which cell editor was last right-clicked so snippet commands insert
    // into the correct editor.  setActiveSnippetEditor keeps the shared SQL
    // snippet machinery in sync; _lastPythonEditor drives Python snippet commands.
    editor.onContextMenu(() => {
      setActiveSnippetEditor(editor);
      if (cell.kind === "code") _lastPythonEditor = editor;
    });
    editor.onDidDispose(() => {
      if (_lastPythonEditor === editor) _lastPythonEditor = null;
    });

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

    // Explicitly bind editing shortcuts so WKWebView doesn't intercept them before Monaco.
    const trigger = (id: string) => editor.trigger("keyboard", id, null);
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Slash,                        () => trigger("editor.action.commentLine"));
    editor.addCommand(monaco.KeyMod.Shift   | monaco.KeyMod.Alt | monaco.KeyCode.KeyF,     () => trigger("editor.action.formatDocument"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyF,                         () => trigger("actions.find"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyD,                         () => trigger("editor.action.addSelectionToNextFindMatch"));
    // ⌘⌥↑ / Ctrl+Alt+↑ — add cursor above; ⌘⌥↓ / Ctrl+Alt+↓ — add cursor below.
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyMod.Alt | monaco.KeyCode.UpArrow,   () => trigger("editor.action.insertCursorAbove"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyMod.Alt | monaco.KeyCode.DownArrow, () => trigger("editor.action.insertCursorBelow"));

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
      // Intercept in capture phase so Monaco's suggest widget cannot grab these first.
      if (e.altKey) {
        if (e.key === "ArrowUp")   { e.preventDefault(); e.stopPropagation(); trigger("editor.action.insertCursorAbove"); return; }
        if (e.key === "ArrowDown") { e.preventDefault(); e.stopPropagation(); trigger("editor.action.insertCursorBelow"); return; }
      }
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
    <div style={{ margin: "0 20px 8px", position: "relative" }} onClick={onSelect}>
      <div style={{
        border: `1px solid ${(focused || isSelected) ? accentColor : border}`,
        borderLeft: isSelected && !focused ? `3px solid ${accentColor}` : undefined,
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