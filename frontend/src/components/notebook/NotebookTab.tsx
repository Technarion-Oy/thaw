// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Snowpark & Developer Workflows

import { useCallback, useEffect, useRef, useState, useMemo, Fragment } from "react";
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
// Slim editor API only (no language services) — see monacoSetup.ts for why.
import * as monacoLib from "monaco-editor/esm/vs/editor/editor.api.js";
import { Button, Dropdown, Modal, Space, Spin, Tooltip, Typography, Select, message, Tag } from "antd";
import type { MenuProps } from "antd";
import {
  PlayCircleOutlined,
  BugOutlined,
  DownOutlined,
  PlusOutlined,
  DeleteOutlined,
  CaretUpOutlined,
  CaretDownOutlined,
  CopyOutlined,
} from "@ant-design/icons";
import {
  StartNotebookSession,
  RunNotebookCell,
  RunNotebookCellSql,
  StopNotebookSession,
  GetTableColumns,
  GetNotebookCompletions,
  GetNotebookHover,
  CheckPythonSyntax,
  DebugNotebookCell,
  StopDapProxy,
  SaveNotebookBreakpoints,
  LoadNotebookBreakpoints,
  GetKernelPythonVersion,
} from "../../../wailsjs/go/app/App";
import { DapClient, type CellBreakpoints, type DebugVariable } from "./debugClient";
import type { snowpark } from "../../../wailsjs/go/models";
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

import { useQueryStore, type QueryResult } from "../../store/queryStore";
import { useThemeStore } from "../../store/themeStore";
import { useSessionStore } from "../../store/sessionStore";
import { useNotebookToolbarStore } from "../../store/notebookToolbarStore";
import DeployNotebookModal from "./DeployNotebookModal";
import ResultGrid from "../results/ResultGrid";

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

type SqlResult = snowpark.NotebookSqlResult;

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
  const loadContext = useSessionStore((s) => s.loadContext);
  const syntaxMode  = useNotebookPrefsStore((s) => s.prefs.syntaxMode);

  const containerRef = useRef<HTMLDivElement>(null);

  const [cells, setCells] = useState<Cell[]>([]);
  const [rawNb, setRawNb] = useState<RawNotebook>({ cells: [] });
  const [kernelReady, setKernelReady] = useState(false);
  const [kernelStarting, setKernelStarting] = useState(false);
  const [kernelError, setKernelError] = useState<string | null>(null);
  const [deployOpen, setDeployOpen] = useState(false);
  // Snapshot of serialized notebook content captured when the Deploy modal is opened.
  // Used for unsaved notebooks that have no on-disk file path.
  const [deployContent, setDeployContent] = useState("");

  // Sync any-cell-running into the queryStore tab so the disconnect guard works.
  const anyCellRunning = cells.some((c) => c.running);
  useEffect(() => {
    useQueryStore.getState().setTabRunning(tabId, anyCellRunning);
  }, [tabId, anyCellRunning]);

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

  // ── Debug state ──────────────────────────────────────────────────────────
  // breakpoints: cellId → set of 1-indexed line numbers that have a breakpoint
  const [breakpoints, setBreakpoints] = useState<Map<string, Set<number>>>(new Map());
  // Ref so toggleBreakpoint can read the current notebook path without being
  // a dependency (avoids stale-closure issues on hot reload).
  const notebookPathRef = useRef(tab?.path ?? "");
  useEffect(() => { notebookPathRef.current = tab?.path ?? ""; }, [tab?.path]);
  // Active debug session state (null = not debugging)
  const [debugState, setDebugState] = useState<{
    /** ID of the cell whose editor should show the highlight and debug panel.
     *  Normally the cell being debugged; switches to another cell when the
     *  debugger steps into a function defined there. */
    cellId: string;
    stopped: boolean;
    variables: DebugVariable[];
    currentLine?: number;
    /** True when paused inside a function from a different cell (not the debug cell itself). */
    inOtherCell?: boolean;
  } | null>(null);
  const dapClientRef = useRef<DapClient | null>(null);
  // Temp dir used by the active debug session (e.g. /tmp). Set when a debug
  // session is running; cleared in finally. Used by toggleBreakpoint to push
  // live breakpoint updates to debugpy via the open DAP connection.
  const debugTempDirRef = useRef<string>("");
  // Ref so debugCell can trigger a kernel restart without being a dep of useCallback.
  const restartKernelRef = useRef<() => void>(() => {});
  // Whether the sticky debug variables panel is collapsed.
  const [debugVarsCollapsed, setDebugVarsCollapsed] = useState(false);

  // Load persisted breakpoints whenever the notebook file changes.
  useEffect(() => {
    if (!tab?.path) { setBreakpoints(new Map()); return; }
    LoadNotebookBreakpoints(tab.path)
      .then((bps) => {
        const map = new Map<string, Set<number>>();
        for (const [cellId, lines] of Object.entries(bps ?? {})) {
          if (lines.length > 0) map.set(cellId, new Set(lines));
        }
        setBreakpoints(map);
      })
      .catch(() => {}); // no file yet → keep empty map
  }, [tab?.path]);

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

  // ── shared kernel start helper ────────────────────────────────────────────
  const startKernel = useCallback(() => {
    setKernelStarting(true);
    setKernelError(null);
    StartNotebookSession(tabId)
      .then(() => {
        setKernelReady(true);
        setKernelStarting(false);
        GetKernelPythonVersion(tabId).then((v) => {
          if (v) useNotebookToolbarStore.getState().setKernelPythonVersion(v);
        }).catch(() => {});
      })
      .catch((e) => { setKernelError(String(e)); setKernelStarting(false); });
  }, [tabId]);

  // ── start kernel on mount, stop on unmount ────────────────────────────────
  useEffect(() => {
    startKernel();
    return () => { StopNotebookSession(tabId).catch(() => {}); };
  }, [tabId, startKernel]);

  // ── sync session context changes from Python cells back to the toolbar ────
  // When a Python cell runs session.sql("USE DATABASE X"), the Go backend syncs
  // the change to the tab's isolated session and emits this event so the toolbar
  // reflects it.
  useEffect(() => {
    const off = EventsOn("notebook:session:context:changed", () => {
      loadContext(tabId);
    });
    return off;
  }, [loadContext, tabId]);

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
        loadContext(tabId); // refresh toolbar after USE commands
      }
      return;
    }

    if (!kernelReady) return;
    patchCell(cell.id, { running: true, outputs: [], images: [], sqlResult: null });
    try {
      const out = await RunNotebookCell(tabId, cell.id, codeToRun);
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

  // Toggle a breakpoint line for a specific cell and persist to disk.
  // Also updates the live DAP session (if any) so debugpy immediately reflects
  // the change — this allows removing a breakpoint on the currently-paused line.
  const toggleBreakpoint = useCallback((cellId: string, line: number) => {
    setBreakpoints((prev) => {
      const next = new Map(prev);
      const set = new Set(next.get(cellId) ?? []);
      if (set.has(line)) set.delete(line);
      else set.add(line);
      if (set.size === 0) next.delete(cellId); else next.set(cellId, set);
      // Persist inside the updater so we have the final value synchronously.
      // Best-effort fire-and-forget; errors are silently ignored.
      if (notebookPathRef.current) {
        const toSave: Record<string, number[]> = {};
        for (const [cId, ls] of next.entries()) {
          if (ls.size > 0) toSave[cId] = Array.from(ls).sort((a, b) => a - b);
        }
        SaveNotebookBreakpoints(notebookPathRef.current, toSave).catch(() => {});
      }
      // Push the updated breakpoint set to the live debugpy session so changes
      // are reflected immediately without requiring a restart.
      const dap = dapClientRef.current;
      const tempDir = debugTempDirRef.current;
      if (dap && tempDir) {
        const filepath = `${tempDir}/thaw_cell_${cellId}.py`;
        dap.updateBreakpoints({ filepath, lines: set });
      }
      return next;
    });
  }, []);

  // Debug a code cell using debugpy via the DAP proxy.
  const debugCell = useCallback(async (cell: Cell) => {
    if (!kernelReady || cell.running || cell.kind !== "code") return;

    patchCell(cell.id, { running: true, outputs: [], images: [], sqlResult: null });
    setDebugState({ cellId: cell.id, stopped: false, variables: [] });

    let debugFilepath = "";
    let offProxyReady:  (() => void) | null = null;
    let offDebugReady:  (() => void) | null = null;
    let offDapMessage:  (() => void) | null = null;
    let offDebugOutput: (() => void) | null = null;

    try {
      // 1. Listen for the DAP proxy-ready event before calling DebugNotebookCell
      //    so we don't miss it if Go starts the proxy before the listeners register.
      let resolveProxy!: () => void;
      let rejectProxy!: (err: Error) => void;
      const proxyReadyPromise = new Promise<void>((res, rej) => { resolveProxy = res; rejectProxy = rej; });

      // If Go proxy fails to attach, it will pass the error string here
      offProxyReady = EventsOn("dap:proxy-ready", (errMsg?: string) => {
        if (errMsg) rejectProxy(new Error(errMsg));
        else resolveProxy();
      }) as () => void;

      offDebugReady = EventsOn("dap:debug-ready", (data: { filepath?: string }) => {
        debugFilepath = data?.filepath ?? "";
      }) as () => void;

      // Stream real-time stdout from the debug run; Go emits one line per event.
      // The accumulated text is shown in the cell's output area while paused.
      let debugStdout = "";
      offDebugOutput = EventsOn("notebook:debug:output", (line: string) => {
        debugStdout += line + "\n";
        setCells((prev) => prev.map((c) =>
          c.id === cell.id
            ? { ...c, outputs: [{ type: "stdout" as const, text: debugStdout }] }
            : c,
        ));
      }) as () => void;

      // 2. Start the long-running IPC call (resolves only when the session ends)
      const debugPromise = DebugNotebookCell(tabId, cell.id, cell.source);

      // 3. Wait for the proxy to be up, then start the DAP handshake
      await proxyReadyPromise;
      offProxyReady();  offProxyReady = null;
      offDebugReady();  offDebugReady = null;

      // 4. Build breakpoint list for ALL cells that have breakpoints.
      //    debugpy needs to know about every file it might stop in, not just
      //    the cell being debugged.  Other cells' temp files follow the same
      //    naming convention as the debug cell: thaw_cell_<cellId>.py in the
      //    same temp directory.
      const tempDir = debugFilepath.replace(/[/\\][^/\\]+$/, ""); // dirname
      debugTempDirRef.current = tempDir;
      const allBreakpoints: CellBreakpoints[] = [];
      for (const [bpCellId, lines] of breakpoints) {
        if (lines.size === 0) continue;
        const filepath = bpCellId === cell.id
          ? debugFilepath
          : `${tempDir}/thaw_cell_${bpCellId}.py`;
        allBreakpoints.push({ filepath, lines });
      }

      // Create the DAP client and wire it to events
      const dap = new DapClient(allBreakpoints);
      dapClientRef.current = dap;

      // Number of source lines in this cell; used to detect the trailing 'pass'
      // sentinel that was appended to the debug file by Go.
      const cellLineCount = cell.source.split("\n").length;

      dap.onStopped = (variables, currentLine, currentFile) => {
        // Only auto-continue for the trailing sentinel line when we are still
        // inside the debug cell's own file.  If we stepped into a function
        // defined in another cell, currentFile will differ and we must NOT
        // auto-continue (the line number comparison would be meaningless there).
        const inDebugFile = !currentFile || currentFile === debugFilepath;
        if (inDebugFile && currentLine != null && currentLine > cellLineCount) {
          dap.continue();
          return;
        }

        // When paused inside a function from another cell, extract that cell's
        // ID from the temp-file path (/tmp/thaw_cell_<cellId>.py) and move the
        // line highlight + debug controls into that cell's editor so the user
        // can see exactly which line they are on.
        let pausedCellId = cell.id;
        const inOtherCell = !inDebugFile;
        if (inOtherCell && currentFile) {
          const m = currentFile.match(/thaw_cell_([^/\\]+)\.py$/);
          if (m) pausedCellId = m[1];
        }

        setDebugState({ cellId: pausedCellId, stopped: true, variables, currentLine, inOtherCell });
      };
      dap.onContinued = () => {
        setDebugState({ cellId: cell.id, stopped: false, variables: [], currentLine: undefined });
      };

      dap.start();
      offDapMessage = () => dap.stop();

      // 5. Run the DAP handshake (initialize → attach → setBreakpoints → configurationDone)
      await dap.initialize();

      // 6. Await the execution result
      const out = await debugPromise;

      // Build final outputs: use accumulated real-time stdout plus any
      // stderr / error from the result JSON (images come separately).
      const finalOutputs: CellOutput[] = [];
      if (debugStdout)  finalOutputs.push({ type: "stdout", text: debugStdout });
      if (out.stderr)   finalOutputs.push({ type: "stderr", text: out.stderr  });
      if (out.error) {
        const isServerErr = /not available|failed to connect to debugpy/i.test(out.error);
        finalOutputs.push({
          type: "error",
          text: isServerErr
            ? out.error + "\n\nKernel is restarting — try again in a moment."
            : out.error,
        });
        if (isServerErr) restartKernelRef.current();
      }

      // Dismiss the debug panel BEFORE showing outputs so the panel never
      // visually overlaps the output area.
      setDebugState(null);

      setCells((prev) => {
        const updated = prev.map((c) =>
          c.id === cell.id
            ? { ...c, running: false, outputs: finalOutputs, images: out.images ?? [], executionCount: (c.executionCount ?? 0) + 1 }
            : c,
        );
        syncToStore(updated);
        return updated;
      });
    } catch (e) {
      const msg = String(e);
      const isServerErr = /not available|failed to connect to debugpy/i.test(msg);
      patchCell(cell.id, {
        running: false,
        outputs: [{
          type: "error",
          text: isServerErr
            ? msg + "\n\nKernel is restarting — try again in a moment."
            : msg,
        }],
      });
      if (isServerErr) restartKernelRef.current();
    } finally {
      offProxyReady?.();
      offDebugReady?.();
      offDapMessage?.();
      offDebugOutput?.();
      dapClientRef.current?.stop();
      dapClientRef.current = null;
      debugTempDirRef.current = "";
      setDebugState(null); // no-op if already cleared in try, safety net for catch path
      // Close the TCP proxy so Python's wait_for_client() unblocks and Go releases s.mu.
      // This is a no-op if Go already called StopDapProxy() on a clean exit.
      await StopDapProxy().catch(() => {});
    }
  }, [kernelReady, breakpoints, tabId, patchCell, syncToStore, setCells]);

  const restartKernel = useCallback(async () => {
    await StopNotebookSession(tabId).catch(() => {});
    setKernelReady(false);
    startKernel();
  }, [tabId, startKernel]);
  useEffect(() => { restartKernelRef.current = restartKernel; }, [restartKernel]);

  const addCellOfKind = useCallback(
    (afterId: string | undefined, kind: "code" | "sql" | "markdown") => {
      const newCell: Cell = {
        id: crypto.randomUUID(),
        kind,
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
    },
    [syncToStore],
  );

  const addCell = useCallback((afterId?: string) => {
    addCellOfKind(afterId, "code");
  }, [addCellOfKind]);

  // ── Sync kernel state & callbacks to the unified toolbar store ────────────
  // Merged into a single effect to avoid transient states when switching tabs
  // (clear + partial re-set in separate effects could briefly null out callbacks).
  useEffect(() => {
    const store = useNotebookToolbarStore.getState();
    store.setKernelState({ kernelReady, kernelStarting, kernelError });
    store.setCallbacks({
      onRestartKernel: restartKernel,
      onDeploy: () => {
        setDeployContent(serializeNotebook(rawNb, cellsRef.current));
        setDeployOpen(true);
      },
      onRunAll: () => {
        const currentCells = cellsRef.current;
        (async () => {
          for (const cell of currentCells) {
            if (cell.kind === "markdown") continue;
            await runCell(cell);
          }
        })();
      },
    });
    return () => { useNotebookToolbarStore.getState().clear(); };
  }, [restartKernel, addCell, rawNb, runCell, kernelReady, kernelStarting, kernelError]);

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

  // ── render ────────────────────────────────────────────────────────────────

  return (
    <div ref={containerRef} style={{ display: "flex", flexDirection: "column", height: "100%", background: "var(--bg)", overflow: "hidden" }}>

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
          <Fragment key={cell.id}>
            <CellView
              tabId={tabId}
              cell={cell}
              isFirst={idx === 0}
              isLast={idx === cells.length - 1}
              kernelReady={kernelReady}
              isDark={isDark}
              isSelected={selectedCellId === cell.id}
              onRun={(code) => runCell(cell, code)}
              onDebug={() => debugCell(cell)}
              onDelete={() => confirmDeleteCell(cell.id)}
              onMoveUp={() => moveCell(cell.id, -1)}
              onMoveDown={() => moveCell(cell.id, 1)}
              onSourceChange={(s) => updateSource(cell.id, s)}
              onKindChange={(k) => setCellKind(cell.id, k)}
              onAddAfter={() => addCell(cell.id)}
              onSelect={() => setSelectedCellId(cell.id)}
              onModelReady={(model) => cellModelsRef.current.set(cell.id, model)}
              onModelDispose={() => cellModelsRef.current.delete(cell.id)}
              breakpoints={breakpoints.get(cell.id) ?? new Set()}
              onBreakpointToggle={(line) => toggleBreakpoint(cell.id, line)}
              debugCurrentLine={debugState?.cellId === cell.id && debugState.stopped ? debugState.currentLine : undefined}
            />
            {/* Hover-reveal add bar between every pair of cells (NOT after the
                last one — that gets the permanent bar below). */}
            {idx < cells.length - 1 && (
              <AddCellBar onAdd={(kind) => addCellOfKind(cell.id, kind)} />
            )}
          </Fragment>
        ))}
        {cells.length > 0 && (
          <AddCellBar
            permanent
            onAdd={(kind) => addCellOfKind(cells[cells.length - 1].id, kind)}
          />
        )}
      </div>

      {/* Sticky debug bar — always visible at the bottom while a debug session is paused */}
      {debugState?.stopped && (
        <div style={{
          flexShrink: 0,
          borderTop: "1px solid var(--border)",
          background: isDark ? "#0d2137" : "#eff6ff",
          padding: "8px 12px 10px",
        }}>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: debugVarsCollapsed ? 0 : 8 }}>
            <button
              onClick={() => setDebugVarsCollapsed((c) => !c)}
              title={debugVarsCollapsed ? "Expand variables" : "Collapse variables"}
              style={{
                background: "none", border: "none", cursor: "pointer", padding: "0 4px 0 0",
                display: "flex", alignItems: "center", gap: 4,
                fontSize: 11, fontWeight: 600, color: isDark ? "#7dd3fc" : "#1d4ed8",
              }}
            >
              <span style={{ fontSize: 9, display: "inline-block", transform: debugVarsCollapsed ? "rotate(-90deg)" : "rotate(0deg)", transition: "transform 0.15s" }}>▼</span>
              ⏸ {debugState.inOtherCell ? "Paused inside function" : `Paused${debugState.currentLine != null ? ` at line ${debugState.currentLine}` : " at breakpoint"}`}
            </button>
            <Space.Compact size="small">
              <Button type="primary" onClick={() => dapClientRef.current?.continue()} title="Continue">▶</Button>
              <Button onClick={() => dapClientRef.current?.stepOver()} title="Step Over">⤵</Button>
              <Button onClick={() => dapClientRef.current?.stepInto()} title="Step Into">⬇</Button>
              <Button danger onClick={() => dapClientRef.current?.disconnect()} title="Stop Debugging">⏹</Button>
            </Space.Compact>
          </div>
          {!debugVarsCollapsed && (debugState.variables.length === 0 ? (
            <span style={{ fontSize: 11, color: "var(--text-muted)" }}>No local variables</span>
          ) : (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 4, maxHeight: 120, overflowY: "auto" }}>
              {debugState.variables.map((v) => (
                <Tooltip key={v.name} title="Click to copy value">
                  <div
                    onClick={() => { ClipboardSetText(`${v.name} = ${v.value}`); message.success("Copied!", 1); }}
                    style={{
                      background: "var(--bg-overlay)",
                      border: "1px solid var(--border)",
                      borderRadius: 4,
                      padding: "2px 8px",
                      fontFamily: "monospace",
                      fontSize: 11,
                      maxWidth: 320,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                      cursor: "pointer",
                    }}
                  >
                    <span style={{ color: isDark ? "#93c5fd" : "#1e40af" }}>{v.name}</span>
                    <span style={{ color: "var(--text-muted)" }}> = </span>
                    <span style={{ color: "var(--text)" }}>{v.value}</span>
                    {v.type && (
                      <span style={{ color: "var(--text-muted)", fontSize: 10 }}> ({v.type})</span>
                    )}
                  </div>
                </Tooltip>
              ))}
            </div>
          ))}
        </div>
      )}

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

// ─── AddCellBar ───────────────────────────────────────────────────────────────

function AddCellBar({
  onAdd,
  permanent = false,
}: {
  onAdd: (kind: "code" | "sql" | "markdown") => void;
  permanent?: boolean;
}) {
  return (
    <div className={"thaw-nb-add-row" + (permanent ? " permanent" : "")}>
      <div className="thaw-nb-add-line" />
      <div className="thaw-nb-add-buttons">
        <button
          className="thaw-nb-add-btn"
          data-kind="code"
          onClick={(e) => { e.stopPropagation(); onAdd("code"); }}
        >
          <span className="thaw-nb-add-dot" />+ Code
        </button>
        <button
          className="thaw-nb-add-btn"
          data-kind="sql"
          onClick={(e) => { e.stopPropagation(); onAdd("sql"); }}
        >
          <span className="thaw-nb-add-dot" />+ SQL
        </button>
        <button
          className="thaw-nb-add-btn"
          data-kind="markdown"
          onClick={(e) => { e.stopPropagation(); onAdd("markdown"); }}
        >
          <span className="thaw-nb-add-dot" />+ Markdown
        </button>
      </div>
    </div>
  );
}

// ─── CellView ─────────────────────────────────────────────────────────────────

interface CellViewProps {
  tabId: string;
  cell: Cell;
  isFirst: boolean;
  isLast: boolean;
  kernelReady: boolean;
  isDark: boolean;
  isSelected: boolean;
  onRun: (code?: string) => void;
  onDebug?: () => void;
  onDelete: () => void;
  onMoveUp: () => void;
  onMoveDown: () => void;
  onSourceChange: (s: string) => void;
  onKindChange: (k: Cell["kind"]) => void;
  onAddAfter: () => void;
  onSelect: () => void;
  onModelReady?: (model: monacoLib.editor.ITextModel) => void;
  onModelDispose?: () => void;
  /** Line numbers (1-indexed) that have active breakpoints for this cell. */
  breakpoints?: Set<number>;
  /** Called when the user clicks the gutter margin to toggle a breakpoint. */
  onBreakpointToggle?: (line: number) => void;
  /** 1-indexed line number where the debugger is currently paused (from DAP stackTrace). */
  debugCurrentLine?: number;
}

function CellView({
  tabId, cell, isFirst, isLast, kernelReady, isDark,
  isSelected, onRun, onDebug, onDelete, onMoveUp, onMoveDown, onSourceChange, onKindChange,
  onAddAfter, onSelect, onModelReady, onModelDispose,
  breakpoints = new Set(), onBreakpointToggle,
  debugCurrentLine,
}: CellViewProps) {
  const [focused, setFocused] = useState(false);
  
  // Accent colour now comes from the per-kind CSS variable so SQL cells get
  // teal, markdown cells get violet, and code cells stay on brand blue.
  // ConfigProvider's `colorPrimary` is no longer hard-coded here.

  const { editorFontSize, editorFont } = useThemeStore();

  // Memoize by queryID (stable string) to avoid rebuilding the object when
  // unrelated cells re-render (setCells rebuilds all cell references).
  const sqlResult = cell.sqlResult;
  const queryResult: QueryResult | null = useMemo(() => {
    if (!sqlResult) return null;
    return {
      columns: sqlResult.columns,
      rows: sqlResult.rows,
      rowsAffected: sqlResult.rowCount,
      queryID: sqlResult.queryID,
      truncated: sqlResult.truncated,
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sqlResult?.queryID]);

  // Decorations collection for breakpoint glyphs in the Monaco gutter.
  const decorationsRef = useRef<monacoLib.editor.IEditorDecorationsCollection | null>(null);
  // Decorations collection for the current debug line highlight.
  const debugLineDecRef = useRef<monacoLib.editor.IEditorDecorationsCollection | null>(null);
  // ── Monaco editor setup ───────────────────────────────────────────────────
  const editorRef       = useRef<any>(null);
  const onRunRef        = useRef(onRun);
  const onSelectRef     = useRef(onSelect);
  useEffect(() => { onRunRef.current = onRun; }, [onRun]);
  useEffect(() => { onSelectRef.current = onSelect; }, [onSelect]);

  // Sync breakpoint decorations into the Monaco editor whenever the set changes.
  useEffect(() => {
    if (!decorationsRef.current) return;
    decorationsRef.current.set(
      Array.from(breakpoints).map((line) => ({
        range: new monacoLib.Range(line, 1, line, 1),
        options: {
          glyphMarginClassName: "thaw-debug-breakpoint",
          glyphMarginHoverMessage: { value: `Breakpoint at line ${line}` },
        },
      })),
    );
  }, [breakpoints]);

  // Sync current debug line highlight whenever the paused line changes.
  useEffect(() => {
    if (!debugLineDecRef.current) return;
    if (debugCurrentLine != null) {
      debugLineDecRef.current.set([{
        range: new monacoLib.Range(debugCurrentLine, 1, debugCurrentLine, 1),
        options: {
          isWholeLine: true,
          className: "thaw-debug-current-line",
          glyphMarginClassName: "thaw-debug-current-line-arrow",
          zIndex: 10,
        },
      }]);
    } else {
      debugLineDecRef.current.set([]);
    }
  }, [debugCurrentLine]);

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

      // Set up breakpoint glyph decorations collection.
      decorationsRef.current = editor.createDecorationsCollection([]);
      // Set up current debug line highlight decorations collection.
      debugLineDecRef.current = editor.createDecorationsCollection([]);
      // Apply any breakpoints that were set before the editor mounted.
      if (breakpoints.size > 0) {
        decorationsRef.current.set(
          Array.from(breakpoints).map((line) => ({
            range: new monacoLib.Range(line, 1, line, 1),
            options: {
              glyphMarginClassName: "thaw-debug-breakpoint",
              glyphMarginHoverMessage: { value: `Breakpoint at line ${line}` },
            },
          })),
        );
      }

      // Toggle breakpoints on glyph-margin clicks.
      editor.onMouseDown((e) => {
        // MouseTargetType.GUTTER_GLYPH_MARGIN === 2
        if (e.target.type === 2 && onBreakpointToggle) {
          const line = e.target.position?.lineNumber;
          if (line != null) onBreakpointToggle(line);
        }
      });
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
    <div onClick={onSelect}>
      <div
        className="thaw-nb-cell"
        data-kind={cell.kind}
        data-selected={focused || isSelected}
      >
        {/* Left gutter — execution count + kind tag */}
        <div className="thaw-nb-cell-gutter">
          <span className="thaw-nb-count">
            {cell.running ? "[*]"
             : cell.executionCount != null ? `[${cell.executionCount}]`
             : ""}
          </span>
          <span className="thaw-nb-kind-tag">
            {cell.kind === "code" ? "PY" : cell.kind === "sql" ? "SQL" : "MD"}
          </span>
        </div>

        {/* Body — toolbar / editor / outputs */}
        <div className="thaw-nb-cell-body">
          {/* Cell header */}
          <div style={{
            display: "flex",
            alignItems: "center",
            gap: 4,
            padding: "2px 6px",
            background: "var(--bg-raised)",
            borderBottom: "1px solid var(--border)",
          }}>
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

            {/* Toolbar — arranged in three logical groups separated by hairlines */}
            <div className="thaw-nb-btn-group">
              <Tooltip title="Run cell (Shift+Enter) · Run selection (select text, then Shift+Enter)">
                <Button
                  type="text"
                  className="thaw-nb-icon-btn run"
                  icon={cell.running ? <Spin size="small" /> : <PlayCircleOutlined />}
                  disabled={cell.kind === "markdown" || (cell.kind === "code" && !kernelReady)}
                  onClick={() => onRun()}
                />
              </Tooltip>
              {cell.kind === "code" && onDebug && (
                <Dropdown
                  trigger={["click"]}
                  disabled={!kernelReady || cell.running}
                  menu={{
                    items: [
                      {
                        key: "debug",
                        label: "Debug cell",
                        icon: <BugOutlined />,
                        onClick: onDebug,
                      },
                    ] satisfies MenuProps["items"],
                  }}
                >
                  <Button
                    type="text"
                    className="thaw-nb-icon-btn"
                    icon={<DownOutlined style={{ fontSize: 8 }} />}
                    disabled={!kernelReady || cell.running}
                  />
                </Dropdown>
              )}
            </div>

            <div className="thaw-nb-btn-sep" />

            <div className="thaw-nb-btn-group">
              <Tooltip title="Move up">
                <Button type="text" className="thaw-nb-icon-btn nav" icon={<CaretUpOutlined />} disabled={isFirst} onClick={onMoveUp} />
              </Tooltip>
              <Tooltip title="Move down">
                <Button type="text" className="thaw-nb-icon-btn nav" icon={<CaretDownOutlined />} disabled={isLast} onClick={onMoveDown} />
              </Tooltip>
              <Tooltip title="Add cell below">
                <Button type="text" className="thaw-nb-icon-btn" icon={<PlusOutlined />} onClick={onAddAfter} />
              </Tooltip>
            </div>

            <div className="thaw-nb-btn-sep" />

            <Tooltip title="Delete cell">
              <Button type="text" className="thaw-nb-icon-btn danger" icon={<DeleteOutlined />} onClick={onDelete} />
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
                fontSize: editorFontSize,
                fontFamily: editorFont || "\"JetBrains Mono\", \"Fira Code\", \"Cascadia Code\", monospace",
                minimap: { enabled: false },
                lineNumbers: cell.kind === "code" ? "on" : "off",
                lineNumbersMinChars: 2,
                glyphMargin: cell.kind === "code",
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
            <div className="thaw-nb-output-area">
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
                    color: out.type === "error"  ? "var(--cell-output-error)"
                         : out.type === "stderr" ? "var(--cell-output-stderr)"
                         : "var(--cell-output-stdout)",
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
            <div style={{ borderTop: "1px solid var(--border)" }}>
              {cell.outputs.map((out, i) => (
                <pre key={i} style={{
                  margin: 0,
                  padding: "6px 10px",
                  fontFamily: "monospace",
                  fontSize: 12,
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                  background: "var(--cell-output-bg)",
                  color: "var(--cell-output-error)",
                }}>
                  {out.text}
                </pre>
              ))}
            </div>
          )}

          {/* SQL result table */}
          {cell.kind === "sql" && queryResult && queryResult.columns.length > 0 && (
            <>
              <div style={{ borderTop: "1px solid var(--border)", height: 360, overflow: "hidden" }}>
                <ResultGrid result={queryResult} standalone />
              </div>
              <div style={{
                padding: "4px 10px",
                fontSize: 11,
                color: "var(--text-muted)",
                background: "var(--bg-raised)",
                borderTop: "1px solid var(--border)",
                display: "flex",
                alignItems: "center",
                gap: 8,
              }}>
                <span>
                  {queryResult.truncated
                    ? `${queryResult.rowsAffected.toLocaleString()}+ rows`
                    : `${queryResult.rowsAffected} row${queryResult.rowsAffected !== 1 ? "s" : ""}`}
                </span>
                {queryResult.truncated && (
                  <Tag color="orange" style={{ fontSize: 10, lineHeight: "16px", padding: "0 5px", border: "none" }}>truncated</Tag>
                )}
                {queryResult.queryID && (
                  <span style={{ opacity: 0.7 }}>  ·  {queryResult.queryID}</span>
                )}
              </div>
            </>
          )}
          {cell.kind === "sql" && cell.sqlResult && cell.sqlResult.columns.length === 0 && (
            <div style={{
              borderTop: "1px solid var(--border)",
              padding: "6px 10px",
              fontSize: 12,
              color: "var(--text-muted)",
              fontFamily: "monospace",
            }}>
              OK — {cell.sqlResult.rowCount} row{cell.sqlResult.rowCount !== 1 ? "s" : ""} affected
              {cell.sqlResult.queryID ? `  ·  ${cell.sqlResult.queryID}` : ""}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
