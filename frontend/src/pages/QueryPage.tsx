// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useRef, useState } from "react";
import { flushSync } from "react-dom";
import { Button, Dropdown, Space, Typography, Alert, Spin, Tag, Select, Tooltip, message, Modal, type MenuProps } from "antd";
import { CopyOutlined, FileTextOutlined, FileExcelOutlined, PushpinOutlined, PushpinFilled, CloseOutlined, LayoutOutlined, GlobalOutlined, BarChartOutlined } from "@ant-design/icons";
import * as XLSX from "xlsx";
import { ClipboardSetText, BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { StartQuery, WaitForQueryResult, CancelQuery, Disconnect, SaveFile, PickSaveFile, PickSaveExportFile, SaveBinaryFile, PickOpenFile, ReadFile, GetAIConfig, GetSessionParameters, GetSessionVariables, PickNotebookFile, ReadNotebook, NotebookUseContext, SaveNotebook, GetCurrentUser, GetCurrentRegion, GetSnowsightURL, CloseTabSession, GetSessionInitMode, InitTabSession } from "../../wailsjs/go/main/App";
import { GetSqlStatementRanges } from "../../wailsjs/go/sqleditor/Service";
import type { main } from "../../wailsjs/go/models";
import SessionPropertiesModal from "../components/common/SessionPropertiesModal";
import SnippetsModal from "../components/snippets/SnippetsModal";
import ExportPathFormatModal from "../components/export/ExportPathFormatModal";
import MigrationModal from "../components/migration/MigrationModal";
import DbtProjectModal from "../components/dbt/DbtProjectModal";
import FunctionCatalogModal from "../components/fnmeta/FunctionCatalogModal";
import KeyboardShortcutsModal from "../components/help/KeyboardShortcutsModal";
import AboutModal from "../components/help/AboutModal";
import { usePanelLayoutStore } from "../store/panelLayoutStore";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import SqlEditor from "../components/editor/SqlEditor";
import TabBar from "../components/editor/TabBar";
import { DiffEditor } from "@monaco-editor/react";
import { ensureMonacoSetup } from "../components/editor/monacoSetup";
import { useThemeStore } from "../store/themeStore";
import ResultGrid, { type ScrollSyncHandle } from "../components/results/ResultGrid";
import QueryProfileModal from "../components/results/QueryProfileModal";
import AiChat from "../components/chat/AiChat";
import TerminalPanel from "../components/terminal/TerminalPanel";
import NotebookTab from "../components/notebook/NotebookTab";
import { useQueryStore, type QueryResult, EXECUTE_IN_TAB_EVENT } from "../store/queryStore";
import { useConnectionStore } from "../store/connectionStore";
import { useSessionStore } from "../store/sessionStore";
import { useFeatureFlagsStore } from "../store/featureFlagsStore";
import { useNotebookToolbarStore } from "../store/notebookToolbarStore";
import Toolbar from "../components/toolbar/Toolbar";
import { notebookButtons, NotebookStatusIndicator } from "../components/toolbar/NotebookToolbarSlot";

const { Text } = Typography;

export default function QueryPage() {
  const { sql, selectedSql, isRunning, error, setResult, setError, markSaved, openScratch, openFile, setSql, openNotebook, openNotebookUnsaved, refreshFileTab, orphanFileTab } = useQueryStore();
  const activeTabId    = useQueryStore((s) => s.activeTabId);
  const isNotebookTab  = useQueryStore((s) => (s.tabs.find((t) => t.id === s.activeTabId)?.kind ?? "sql") === "notebook");
  const activeDiff     = useQueryStore((s) => s.tabs.find((t) => t.id === s.activeTabId)?.diff ?? null);
  const resolved       = useThemeStore((s) => s.resolved);
  const editorFont     = useThemeStore((s) => s.editorFont);
  const editorFontSize = useThemeStore((s) => s.editorFontSize);
  const editorSplit        = usePanelLayoutStore((s) => s.editorSplit);
  const setEditorSplit     = usePanelLayoutStore((s) => s.setEditorSplit);
  const splitEditorWidth   = usePanelLayoutStore((s) => s.splitEditorWidth);
  const setSplitEditorWidth = usePanelLayoutStore((s) => s.setSplitEditorWidth);
  const splitTabId  = useQueryStore((s) => s.splitTabId);
  const splitTab    = useQueryStore((s) => s.tabs.find((t) => t.id === s.splitTabId) ?? null);
  const setSplitTab = useQueryStore((s) => s.setSplitTab);
  const [splitPct, setSplitPct] = useState(editorSplit);
  const splitResizing  = useRef(false);
  const splitStartY    = useRef(0);
  const splitStartPct  = useRef(0);
  const splitLivePct   = useRef(editorSplit);
  const editorAreaRef  = useRef<HTMLDivElement>(null);
  // Vertical (left/right) split state
  const vSplitResizing    = useRef(false);
  const vSplitStartX      = useRef(0);
  const vSplitStartW      = useRef(0);
  const splitWLive        = useRef(splitEditorWidth);
  const primaryEditorRef  = useRef<HTMLDivElement>(null);
  const [splitW, setSplitW] = useState(splitEditorWidth);
  const [runningQueryId, setRunningQueryId] = useState<string | null>(null);
  const [isCancelling, setIsCancelling] = useState(false);
  const [profileQueryId,  setProfileQueryId]  = useState<string | null>(null);
  const [profileIsLive,   setProfileIsLive]   = useState(false);
  const [profileQuerySql, setProfileQuerySql] = useState<string>("");
  // Multi-statement progress: which statement is running and out of how many.
  const [stmtProgress, setStmtProgress] = useState<{ index: number; total: number; queryID?: string } | null>(null);
  // Zero-based index of the statement currently executing; drives editor highlight.
  const [activeStmtIdx, setActiveStmtIdx] = useState<number | null>(null);
  // True while the running query is a user-text selection (not the full buffer).
  const isSelectionRunRef = useRef(false);
  // Index of the first selected statement within the full-buffer statement list.
  // Used to map backend-reported indices (relative to selection) back to the
  // full-buffer indices that SqlEditor's decorator uses.
  const selectionBaseStmtIdxRef = useRef(0);
  const [resultPane, setResultPane] = useState<"results" | "chat" | "terminal">("results");
  const [aiEnabled, setAiEnabled] = useState(false);
  const [terminalOpen, setTerminalOpen] = useState(false);
  const featureFlags = useFeatureFlagsStore((s) => s.flags);

  // ── Result history — per tab (last 10 unpinned + all pinned, most-recent-first) ────
  interface HistoryEntry { id: string; queryID: string; sql: string; result: QueryResult; pinned: boolean; }
  const [tabHistories,  setTabHistories]  = useState<Map<string, HistoryEntry[]>>(() => new Map());
  const [tabHistoryIds, setTabHistoryIds] = useState<Map<string, string | null>>(() => new Map());
  const [tabCompareIds, setTabCompareIds] = useState<Map<string, string | null>>(() => new Map());

  // Derived values for the currently active tab.
  const resultHistory    = tabHistories.get(activeTabId)  ?? [];
  const historyId        = tabHistoryIds.get(activeTabId) ?? null;
  const compareHistoryId = tabCompareIds.get(activeTabId) ?? null;

  // Helpers that update any tab's history. Using the captured runTabId inside
  // runQuery ensures results from background queries land in the correct tab
  // even if the user has switched away before the query finishes.
  const updateTabHistory = (tabId: string, updater: (prev: HistoryEntry[]) => HistoryEntry[]) =>
    setTabHistories((prev) => {
      const m = new Map(prev);
      m.set(tabId, updater(m.get(tabId) ?? []));
      return m;
    });
  const updateTabHistoryId = (tabId: string, id: string | null) =>
    setTabHistoryIds((prev) => { const m = new Map(prev); m.set(tabId, id); return m; });
  const setCompareHistoryId = (id: string | null) =>
    setTabCompareIds((prev) => { const m = new Map(prev); m.set(activeTabId, id); return m; });

  const togglePin = (id: string) =>
    updateTabHistory(activeTabId, (prev) => prev.map((e) => (e.id === id ? { ...e, pinned: !e.pinned } : e)));

  const [snippetsOpen, setSnippetsOpen] = useState(false);
  const [exportPathFormatOpen, setExportPathFormatOpen] = useState(false);
  const [migrationOpen, setMigrationOpen] = useState(false);
  const [dbtCreateOpen, setDbtCreateOpen] = useState(false);
  const [fnCatalogOpen, setFnCatalogOpen] = useState(false);
  const [kbShortcutsOpen, setKbShortcutsOpen] = useState(false);
  const [aboutOpen, setAboutOpen] = useState(false);

  // Stack of recently-closed tabs for ⌘⇧T / Ctrl+Shift+T reopen.
  const closedTabsRef = useRef<Array<{ path: string | null; title: string; sql: string; kind?: string }>>([]);
  const [sessionPropsOpen, setSessionPropsOpen] = useState(false);
  const [closeConfirm, setCloseConfirm] = useState<{ tabId: string; title: string; isRunning: boolean; isDirty: boolean } | null>(null);
  const [currentUser, setCurrentUser] = useState<string | null>(null);
  const [currentRegion, setCurrentRegion] = useState<string | null>(null);
  const [snowsightUrl, setSnowsightUrl] = useState<string | null>(null);
  const [snowsightModalOpen, setSnowsightModalOpen] = useState(false);
  const [sessionParams, setSessionParams] = useState<main.SessionParam[] | null>(null);
  const [sessionVars, setSessionVars] = useState<main.SessionVar[] | null>(null);
  const [sessionPropsError, setSessionPropsError] = useState<string | null>(null);
  // Ref so the async runQuery closure can detect user-initiated cancellation
  // without relying on stale React state.
  const cancelRequestedRef = useRef(false);
  // Scroll-sync handles for the side-by-side grid split.
  const primarySyncRef = useRef<ScrollSyncHandle | null>(null);
  const compareSyncRef = useRef<ScrollSyncHandle | null>(null);
  const { disconnect, isConnected } = useConnectionStore();
  // Pending query stored when the user runs SQL while disconnected.
  const pendingQueryRef = useRef<string | null>(null);
  // Tracks previous isConnected value to detect connect transitions.
  const prevConnectedRef = useRef(isConnected);
  const {
    role, warehouse, database, schema,
    error: sessionError,
    loadContext, clearError,
  } = useSessionStore();

  // Sync local split state when the store value changes (e.g., after layout reset).
  useEffect(() => { setSplitPct(editorSplit); }, [editorSplit]);
  useEffect(() => { setSplitW(splitEditorWidth); }, [splitEditorWidth]);

  // Load current user/region/url on mount.
  useEffect(() => {
    GetCurrentUser().then(setCurrentUser).catch(() => {});
    GetCurrentRegion().then(setCurrentRegion).catch(() => {});
    GetSnowsightURL().then(setSnowsightUrl).catch(() => {});
  }, []);

  // When the active tab changes (including on mount), immediately reflect its
  // context in the toolbar: setActiveTab gives instant feedback from cache,
  // loadContext fetches fresh state from Go.
  useEffect(() => {
    const store = useSessionStore.getState();
    store.setActiveTab(activeTabId);
    store.loadContext(activeTabId);
  }, [activeTabId]);

  // Track tab additions/removals to manage sessions.
  const tabs = useQueryStore((s) => s.tabs);
  const prevTabIdsRef = useRef<Set<string>>(new Set(tabs.map((t) => t.id)));
  const sessionInitModeRef = useRef<string>("lazy");
  // Load init mode once on mount and when config changes via save.
  useEffect(() => {
    GetSessionInitMode().then((mode) => { sessionInitModeRef.current = mode; });
    const handler = () => {
      GetSessionInitMode().then((mode) => { sessionInitModeRef.current = mode; });
    };
    window.addEventListener("thaw:session-config-saved", handler);
    return () => window.removeEventListener("thaw:session-config-saved", handler);
  }, []);
  useEffect(() => {
    const currentIds = new Set(tabs.map((t) => t.id));
    // New tabs: init eagerly only when explicitly configured.
    if (sessionInitModeRef.current === "eager") {
      currentIds.forEach((id) => {
        if (!prevTabIdsRef.current.has(id)) {
          InitTabSession(id).catch(() => {});
        }
      });
    }
    // Removed tabs.
    prevTabIdsRef.current.forEach((id) => {
      if (!currentIds.has(id)) {
        CloseTabSession(id).catch(() => {});
      }
    });
    prevTabIdsRef.current = currentIds;
  }, [tabs]);

  // On mount, re-read file-backed tabs from disk so their content is fresh
  // after an app restart (they were persisted with cleared sql/savedSql).
  useEffect(() => {
    const { tabs } = useQueryStore.getState();
    tabs.forEach(async (tab) => {
      if (!tab.path) return;
      try {
        const content = tab.kind === "notebook"
          ? await ReadNotebook(tab.path)
          : await ReadFile(tab.path);
        refreshFileTab(tab.id, content);
      } catch {
        orphanFileTab(tab.id);
      }
    });
  }, []);

  // Keep the Snowpark kernel in sync with the tab's session context whenever
  // role, warehouse, database or schema changes, or when switching notebook tabs.
  const tabContexts = useSessionStore((s) => s.tabContexts);
  const activeTabCtx = tabContexts[activeTabId];
  useEffect(() => {
    if (!isNotebookTab) return;
    const ctx = useSessionStore.getState().tabContexts[activeTabId] ?? { role, warehouse, database, schema };
    NotebookUseContext(activeTabId, ctx.role, ctx.warehouse, ctx.database, ctx.schema).catch(() => {});
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTabCtx, activeTabId, isNotebookTab]);

  // Load AI config on mount
  useEffect(() => {
    GetAIConfig().then((c) => setAiEnabled(c.enabled));
  }, []);

  // Handle run-ai-sql events from the chat panel
  useEffect(() => {
    const handler = (e: CustomEvent<{ sql: string; run: boolean }>) => {
      useQueryStore.getState().setSql(e.detail.sql);
      if (e.detail.run) window.dispatchEvent(new Event("run-query"));
    };
    window.addEventListener("run-ai-sql", handler as EventListener);
    return () => window.removeEventListener("run-ai-sql", handler as EventListener);
  }, []);

  // Handle load-query events from QueryHistoryModal
  useEffect(() => {
    const handler = (e: Event) => {
      const { sql: querySql } = (e as CustomEvent<{ sql: string }>).detail;
      setSql(querySql);
    };
    window.addEventListener("load-query", handler);
    return () => window.removeEventListener("load-query", handler);
  }, [setSql]);

  // Open notebook from Snowpark menu
  useEffect(() => {
    const off = EventsOn("menu:snowpark-open-notebook", async () => {
      try {
        const path = await PickNotebookFile();
        if (!path) return;
        const content = await ReadNotebook(path);
        openNotebook(path, content);
      } catch (e) {
        message.error(String(e));
      }
    });
    return () => (off as () => void)();
  }, [openNotebook]);

  // New notebook from Snowpark menu — open a blank tab immediately, no file dialog
  useEffect(() => {
    const off = EventsOn("menu:snowpark-new-notebook", () => {
      const blank = JSON.stringify({
        nbformat: 4,
        nbformat_minor: 5,
        metadata: {
          kernelspec: { display_name: "Python 3", language: "python", name: "python3" },
          language_info: { name: "python", version: "3.12.0" },
        },
        cells: [{ cell_type: "code", execution_count: null, metadata: {}, outputs: [], source: [] }],
      }, null, 1);
      openNotebookUnsaved("Untitled Notebook", blank);
    });
    return () => (off as () => void)();
  }, [openNotebookUnsaved]);

  // Listen for per-statement progress events emitted by the Go backend while
  // executing a multi-statement script.  Update the spinner label and highlight
  // the active statement in the editor.
  useEffect(() => {
    const cleanupStart = EventsOn("query:statement-start", (data: { index: number; total: number }) => {
      setStmtProgress({ index: data.index, total: data.total });
      // Map the backend's selection-relative index to the full-buffer index so
      // the editor decorator highlights the correct statement in both full-buffer
      // and selection runs.
      setActiveStmtIdx(selectionBaseStmtIdxRef.current + data.index);
    });
    const cleanupQid = EventsOn("query:statement-qid", (data: { index: number; queryID: string }) => {
      // Update queryID unconditionally — for fast statements the qid can
      // arrive after the next statement-start has already advanced the index,
      // so a strict index guard would silently discard it.
      setStmtProgress(prev => prev ? { ...prev, queryID: data.queryID } : prev);
      // Also drive runningQueryId so the qid persists even if stmtProgress
      // is cleared before the next render.
      setRunningQueryId(data.queryID);
    });
    return () => { (cleanupStart as () => void)(); (cleanupQid as () => void)(); };
  }, []);

  // Vertical drag handle mouse handlers (for split editor width).
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!vSplitResizing.current) return;
      const parent = document.querySelector(".editor-area") as HTMLElement | null;
      if (!parent) return;
      const delta = e.clientX - vSplitStartX.current;
      const pct = vSplitStartW.current + delta / parent.clientWidth;
      const clamped = Math.min(0.85, Math.max(0.15, pct));
      splitWLive.current = clamped;
      if (primaryEditorRef.current) {
        primaryEditorRef.current.style.flex = `0 0 ${clamped * 100}%`;
      }
    };
    const onUp = () => {
      if (!vSplitResizing.current) return;
      vSplitResizing.current = false;
      document.body.style.cursor     = "";
      document.body.style.userSelect = "";
      const w = splitWLive.current;
      setSplitW(w);
      setSplitEditorWidth(w);
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup",   onUp);
    return () => {
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup",   onUp);
    };
  }, [setSplitEditorWidth]);

  const runQuery = async (overrideSql?: string) => {
    if (!isConnected) {
      const q = overrideSql ?? (selectedSql.trim() || sql.trim());
      if (q) pendingQueryRef.current = q;
      window.dispatchEvent(new Event("thaw:connect"));
      return;
    }
    const query = overrideSql ?? (selectedSql.trim() || sql.trim());
    if (!query) return;
    // Capture the tab that owns this query execution. The user may switch tabs
    // while the query runs; all state updates below use runTabId, not the
    // (potentially stale) activeTabId.
    const runTabId = activeTabId;
    cancelRequestedRef.current = false;
    isSelectionRunRef.current  = !overrideSql && selectedSql.trim() !== "";
    // For selection runs, find which full-buffer statement index the selection
    // starts at, so we can offset the backend's 0-based statement indices.
    if (isSelectionRunRef.current) {
      const offset = sql.indexOf(selectedSql);
      if (offset >= 0) {
        const linesBefore = sql.slice(0, offset).split("\n").length; // 1-based
        const fullRanges = await GetSqlStatementRanges(sql);
        const baseIdx = fullRanges.findIndex((r) => r.startLine >= linesBefore);
        selectionBaseStmtIdxRef.current = baseIdx >= 0 ? baseIdx : 0;
      } else {
        selectionBaseStmtIdxRef.current = 0;
      }
    } else {
      selectionBaseStmtIdxRef.current = 0;
    }
    setIsCancelling(false);
    setRunningQueryId(null);
    setStmtProgress(null);
    setActiveStmtIdx(null);
    setResultPane("results");
    useQueryStore.getState().setTabRunning(runTabId, true);
    try {
      // Phase 1: submit and get query ID.
      const qid = await StartQuery(runTabId, query);
      // For single-statement queries, commit the query ID synchronously so the
      // spinner shows it for at least one frame before results arrive.
      // For multi-statement (qid = ""), skip the overwrite — runningQueryId may
      // already hold a per-statement qid from a statement-qid event.
      if (qid) flushSync(() => setRunningQueryId(qid));
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      // Phase 2: block until results are ready.
      const res = await WaitForQueryResult(runTabId);
      setResult(res);
      const newId = crypto.randomUUID();
      updateTabHistory(runTabId, (prev) => {
        const withNew = [{ id: newId, queryID: res.queryID ?? "", sql: query, result: res, pinned: false }, ...prev];
        const pinned = withNew.filter((e) => e.pinned);
        const unpinned = withNew.filter((e) => !e.pinned).slice(0, 10);
        const kept = new Set([...unpinned.map((e) => e.id), ...pinned.map((e) => e.id)]);
        return withNew.filter((e) => kept.has(e.id));
      });
      updateTabHistoryId(runTabId, newId);
    } catch (e) {
      // Suppress the error when the user explicitly cancelled — keep whatever
      // result was previously shown rather than replacing it with an error.
      // Also suppress if the tab was closed while the query was running.
      if (!cancelRequestedRef.current) {
        const tabStillOpen = useQueryStore.getState().tabs.some((t) => t.id === runTabId);
        if (tabStillOpen) {
          setError(String(e));
          updateTabHistoryId(runTabId, null); // hide the grid; let the user re-select from history
        }
      }
    } finally {
      useQueryStore.getState().setTabRunning(runTabId, false);
      setIsCancelling(false);
      setStmtProgress(null);
      setActiveStmtIdx(null);
      loadContext(runTabId); // refresh database/schema (and role/warehouse) after any USE command
    }
  };

  const handleCancel = async () => {
    if (!isRunning || isCancelling) return;
    cancelRequestedRef.current = true;
    setIsCancelling(true);
    try {
      await CancelQuery(activeTabId);
    } catch (_) {
      // ignore — WaitForQueryResult will return an error regardless
    }
  };

  // Handle execute-in-tab events dispatched by executeInNewTab.  The store has
  // already opened the new tab and set its SQL; we receive the SQL in the event
  // detail to avoid stale closures and run through the proper
  // StartQuery/WaitForQueryResult path so resultHistory is populated.
  const runQueryRef = useRef(runQuery);
  useEffect(() => { runQueryRef.current = runQuery; });
  useEffect(() => {
    const handler = (e: Event) => {
      const { sql: querySql } = (e as CustomEvent<{ sql: string }>).detail;
      runQueryRef.current(querySql);
    };
    window.addEventListener(EXECUTE_IN_TAB_EVENT, handler);
    return () => window.removeEventListener(EXECUTE_IN_TAB_EVENT, handler);
  }, []);

  // When a connection is established after being disconnected, reload session
  // context and auto-run any pending query that was deferred.
  useEffect(() => {
    if (isConnected && !prevConnectedRef.current) {
      useSessionStore.getState().loadContext(activeTabId);
      GetCurrentUser().then(setCurrentUser).catch(() => {});
      GetCurrentRegion().then(setCurrentRegion).catch(() => {});
      GetSnowsightURL().then(setSnowsightUrl).catch(() => {});
      if (pendingQueryRef.current) {
        const q = pendingQueryRef.current;
        pendingQueryRef.current = null;
        setTimeout(() => runQueryRef.current(q), 300);
      }
    }
    prevConnectedRef.current = isConnected;
  }, [isConnected, activeTabId]); // eslint-disable-line react-hooks/exhaustive-deps

  const handleDisconnect = () => {
    const anyRunning = useQueryStore.getState().tabs.some((t) => t.isRunning);
    const doDisconnect = async () => { await Disconnect(); disconnect(); };
    if (anyRunning) {
      Modal.confirm({
        title: "Disconnect while query is running?",
        content: "A query is currently running. Disconnecting will cancel it and discard any pending results.",
        okText: "Disconnect",
        okButtonProps: { danger: true },
        cancelText: "Cancel",
        onOk: doDisconnect,
      });
    } else {
      void doDisconnect();
    }
  };

  const openSessionProperties = async () => {
    setSessionPropsOpen(true);
    setSessionParams(null);
    setSessionVars(null);
    setSessionPropsError(null);
    try {
      const [p, v] = await Promise.all([GetSessionParameters(), GetSessionVariables()]);
      setSessionParams(p);
      setSessionVars(v);
    } catch (e) {
      setSessionPropsError(String(e));
    }
  };

  const openSnowsight = () => setSnowsightModalOpen(true);

  const handleParamChange = (key: string, value: string) => {
    setSessionParams((prev) => prev ? prev.map((p) => p.key === key ? { ...p, value } : p) : prev);
  };

  const handleVarChange = (key: string, value: string) => {
    setSessionVars((prev) => prev ? prev.map((v) => v.key === key ? { ...v, value } : v) : prev);
  };

  const exportCSV = async () => {
    if (!displayedResult) return;
    const escape = (v: unknown) => {
      const s = v === null || v === undefined ? "" : String(v);
      return s.includes(",") || s.includes('"') || s.includes("\n")
        ? `"${s.replace(/"/g, '""')}"`
        : s;
    };
    const csv =
      displayedResult.columns.map(escape).join(",") +
      "\n" +
      displayedResult.rows.map((r) => r.map(escape).join(",")).join("\n");
    const path = await PickSaveExportFile("results.csv", "csv");
    if (!path) return;
    try {
      await SaveFile(path, csv);
      message.success("Exported to CSV");
    } catch (e) {
      message.error(String(e));
    }
  };

  const exportExcel = async () => {
    if (!displayedResult) return;
    const ws = XLSX.utils.aoa_to_sheet([displayedResult.columns, ...displayedResult.rows]);
    const wb = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(wb, ws, "Results");
    const b64 = XLSX.write(wb, { type: "base64", bookType: "xlsx" });
    const path = await PickSaveExportFile("results.xlsx", "excel");
    if (!path) return;
    try {
      await SaveBinaryFile(path, b64);
      message.success("Exported to Excel");
    } catch (e) {
      message.error(String(e));
    }
  };

  // Save to the tab's existing path, or open a Save As dialog if it has none.
  const handleSave = async () => {
    const { tabs, activeTabId, sql: currentSql } = useQueryStore.getState();
    const tab = tabs.find((t) => t.id === activeTabId);
    if (!tab) return;

    let savePath = tab.path;
    let saveTitle = tab.title;

    if (!savePath) {
      savePath = await PickSaveFile(tab.title === "SQL" ? "untitled.sql" : tab.title);
      if (!savePath) return;
      saveTitle = savePath.split("/").pop() ?? savePath;
    }

    try {
      await SaveFile(savePath, currentSql);
      markSaved(activeTabId, savePath, saveTitle);
    } catch (e) {
      message.error(`Save failed: ${String(e)}`);
    }
  };

  // Always open a Save As dialog, regardless of whether the tab has a path.
  const handleSaveAs = async () => {
    const { tabs, activeTabId, sql: currentSql } = useQueryStore.getState();
    const tab = tabs.find((t) => t.id === activeTabId);
    if (!tab) return;

    const defaultName = tab.path
      ? (tab.path.split("/").pop() ?? "untitled.sql")
      : (tab.title === "SQL" ? "untitled.sql" : tab.title);

    const savePath = await PickSaveFile(defaultName);
    if (!savePath) return;
    const saveTitle = savePath.split("/").pop() ?? savePath;

    try {
      await SaveFile(savePath, currentSql);
      markSaved(activeTabId, savePath, saveTitle);
    } catch (e) {
      message.error(`Save failed: ${String(e)}`);
    }
  };

  // Save a specific tab by ID. If the tab has no path yet, opens a Save As
  // dialog first. Returns true when the save succeeded, false on cancel/error.
  const saveTabById = async (tabId: string): Promise<boolean> => {
    const { tabs } = useQueryStore.getState();
    const tab = tabs.find((t) => t.id === tabId);
    if (!tab) return false;

    let savePath = tab.path;
    let saveTitle = tab.title;

    if (!savePath) {
      const defaultName = tab.title === "SQL" ? "untitled.sql" : tab.title;
      savePath = await PickSaveFile(defaultName);
      if (!savePath) return false; // user cancelled the dialog
      saveTitle = savePath.split("/").pop() ?? savePath;
    }

    try {
      if (tab.kind === "notebook") {
        await SaveNotebook(savePath, tab.sql);
      } else {
        await SaveFile(savePath, tab.sql);
      }
      markSaved(tabId, savePath, saveTitle);
      return true;
    } catch (e) {
      message.error(`Save failed: ${String(e)}`);
      return false;
    }
  };

  // Request to close a tab — confirms if the tab has a running query or
  // unsaved changes. Scratch tabs with no running query close without prompting.
  const requestClose = (tabId: string) => {
    const { tabs } = useQueryStore.getState();
    const tab = tabs.find((t) => t.id === tabId);
    if (!tab) return;
    const isDirty = tab.sql !== tab.savedSql;
    const isTabRunning = tab.isRunning ?? false;
    if (isTabRunning || isDirty) {
      setCloseConfirm({ tabId, title: tab.title, isRunning: isTabRunning, isDirty });
    } else {
      closedTabsRef.current.unshift({ path: tab.path, title: tab.title, sql: tab.sql, kind: tab.kind });
      if (closedTabsRef.current.length > 15) closedTabsRef.current.pop();
      useQueryStore.getState().closeTab(tabId);
    }
  };

  const handleOpen = async () => {
    const filePath = await PickOpenFile();
    if (!filePath) return;
    try {
      const content = await ReadFile(filePath);
      openFile(filePath, content);
    } catch (e) {
      message.error(`Open failed: ${String(e)}`);
    }
  };

  // Browser events — dispatched by Monaco keyboard bindings and the Save button.
  useEffect(() => {
    const handler = () => runQuery();
    window.addEventListener("run-query", handler);
    return () => window.removeEventListener("run-query", handler);
  });

  // ⌘⇧Enter / Ctrl+Shift+Enter — Run All Statements (ignores any selection).
  useEffect(() => {
    const handler = () => { const { sql } = useQueryStore.getState(); runQuery(sql); };
    window.addEventListener("run-all-query", handler);
    return () => window.removeEventListener("run-all-query", handler);
  });

  // thaw:focus-results — scroll results panel into view and switch tab to Results.
  useEffect(() => {
    const handler = () => setResultPane("results");
    window.addEventListener("thaw:focus-results", handler);
    return () => window.removeEventListener("thaw:focus-results", handler);
  }, []);

  // Escape cancels the running query.
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isRunning && !isCancelling) handleCancel();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isRunning, isCancelling]);

  // ── Global keyboard shortcuts ──────────────────────────────────────────────
  useEffect(() => {
    const isMac = /Mac|iPhone|iPad/.test(navigator.platform);

    const handler = (e: KeyboardEvent) => {
      const cmd  = isMac ? e.metaKey : e.ctrlKey;
      const ctrl = e.ctrlKey;

      // ── Tab shortcuts ────────────────────────────────────────────────────

      // ⌘W / Ctrl+W — Close current tab (with unsaved-changes guard)
      if (cmd && !e.shiftKey && !e.altKey && e.key === "w") {
        e.preventDefault();
        const { activeTabId } = useQueryStore.getState();
        requestClose(activeTabId);
        return;
      }

      // ⌘⇧T / Ctrl+Shift+T — Reopen last closed tab
      if (cmd && e.shiftKey && !e.altKey && e.key === "T") {
        e.preventDefault();
        const closed = closedTabsRef.current.shift();
        if (closed) {
          const { openScratch, openFile, setSql } = useQueryStore.getState();
          if (closed.path) {
            openFile(closed.path, closed.sql);
          } else {
            openScratch();
            if (closed.sql) setSql(closed.sql);
          }
        }
        return;
      }

      // Ctrl+Tab / Ctrl+Shift+Tab — Next / Prev tab (Ctrl on all platforms)
      if (ctrl && !e.metaKey && e.key === "Tab") {
        e.preventDefault();
        const { tabs, activeTabId, activateTab } = useQueryStore.getState();
        if (tabs.length < 2) return;
        const idx  = tabs.findIndex((t) => t.id === activeTabId);
        const next = e.shiftKey
          ? tabs[(idx - 1 + tabs.length) % tabs.length]
          : tabs[(idx + 1) % tabs.length];
        if (next) activateTab(next.id);
        return;
      }

      // ⌘, / Ctrl+, — Open Preferences (AI settings)
      if (cmd && !e.shiftKey && !e.altKey && e.key === ",") {
        e.preventDefault();
        window.dispatchEvent(new Event("thaw:configure-ai"));
        return;
      }

      // ── Query shortcuts ──────────────────────────────────────────────────

      // ⌘⇧Enter / Ctrl+Shift+Enter — Run All Statements
      if (cmd && e.shiftKey && !e.altKey && e.key === "Enter") {
        e.preventDefault();
        window.dispatchEvent(new Event("run-all-query"));
        return;
      }

      // ⌘E / Ctrl+E — Export current results as CSV
      if (cmd && !e.shiftKey && !e.altKey && e.key === "e") {
        e.preventDefault();
        window.dispatchEvent(new Event("thaw:export-csv"));
        return;
      }

      // ── UI shortcuts ─────────────────────────────────────────────────────

      // ⌘B / Ctrl+B — Toggle left sidebar
      if (cmd && !e.shiftKey && !e.altKey && e.key === "b") {
        e.preventDefault();
        usePanelLayoutStore.getState().toggleLeftHidden();
        return;
      }

      // ⌘\ / Ctrl+\ — Toggle split editor
      if (cmd && !e.shiftKey && !e.altKey && e.key === "\\") {
        e.preventDefault();
        const { tabs, activeTabId, splitTabId, setSplitTab } = useQueryStore.getState();
        if (splitTabId) {
          setSplitTab(null);
        } else {
          const others = tabs.filter((t) => t.id !== activeTabId && (!t.kind || t.kind === "sql"));
          if (others.length > 0) setSplitTab(others[others.length - 1].id);
        }
        return;
      }

      // ⌘L / Ctrl+L — Focus AI Chat
      if (cmd && !e.shiftKey && !e.altKey && e.key === "l") {
        e.preventDefault();
        setResultPane("chat");
        setTimeout(() => window.dispatchEvent(new Event("thaw:focus-ai-chat")), 30);
        return;
      }

      // ⌘⇧F / Ctrl+Shift+F — Focus Object Browser Search
      if (cmd && e.shiftKey && !e.altKey && e.key === "F") {
        e.preventDefault();
        window.dispatchEvent(new Event("thaw:focus-object-search"));
        return;
      }
    };

    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }); // no dep array — same pattern as run-query/save-file listeners

  useEffect(() => {
    const handler = () => handleSave();
    window.addEventListener("save-file", handler);
    return () => window.removeEventListener("save-file", handler);
  });

  useEffect(() => {
    const handler = (e: Event) => {
      const { tabId } = (e as CustomEvent<{ tabId: string }>).detail;
      requestClose(tabId);
    };
    window.addEventListener("thaw:request-close-tab", handler);
    return () => window.removeEventListener("thaw:request-close-tab", handler);
  });

  // Wails events — dispatched by the native application menu.
  useEffect(() => {
    const offNewTab  = EventsOn("menu:new-tab",  () => openScratch());
    const offOpen    = EventsOn("menu:open",     () => handleOpen());
    const offSave    = EventsOn("menu:save",     () => handleSave());
    const offSaveAs  = EventsOn("menu:save-as",  () => handleSaveAs());
    return () => { offNewTab(); offOpen(); offSave(); offSaveAs(); };
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:open-terminal", () => {
      if (!featureFlags.embeddedTerminal) return;
      setTerminalOpen(true);
      setResultPane("terminal");
    });
    return () => off();
  }, [featureFlags.embeddedTerminal]);

  useEffect(() => {
    const off = EventsOn("menu:code-snippets", () => { if (featureFlags.codeSnippets) setSnippetsOpen(true); });
    return () => off();
  }, [featureFlags.codeSnippets]);

  useEffect(() => {
    const off = EventsOn("menu:export-path-format", () => setExportPathFormatOpen(true));
    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:migration", () => { if (featureFlags.schemaMigration) setMigrationOpen(true); });
    return () => off();
  }, [featureFlags.schemaMigration]);

  useEffect(() => {
    const off = EventsOn("menu:dbt-create", () => { if (featureFlags.dbtScaffolding) setDbtCreateOpen(true); });
    return () => off();
  }, [featureFlags.dbtScaffolding]);

  useEffect(() => {
    const off = EventsOn("menu:function-catalog", () => setFnCatalogOpen(true));
    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:keyboard-shortcuts", () => setKbShortcutsOpen(true));
    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:about", () => setAboutOpen(true));
    return () => off();
  }, []);

  // ⌘E / Ctrl+E — Export current results as CSV (wired from keyboard handler).
  useEffect(() => {
    const handler = () => { if (featureFlags.resultsetExport) exportCSV(); };
    window.addEventListener("thaw:export-csv", handler);
    return () => window.removeEventListener("thaw:export-csv", handler);
  });



  // The result currently shown in the grid — null when no result is selected
  // (e.g. right after a failed query; the user must pick from history explicitly).
  const displayedResult: QueryResult | null =
    historyId !== null ? (resultHistory.find((e) => e.id === historyId)?.result ?? null) : null;

  const compareEntry = compareHistoryId !== null ? (resultHistory.find((e) => e.id === compareHistoryId) ?? null) : null;
  const compareResult: QueryResult | null = compareEntry?.result ?? null;

  // Pinned entries float to the top; within each group the original order is preserved.
  const sortedHistory = [
    ...resultHistory.filter((e) => e.pinned),
    ...resultHistory.filter((e) => !e.pinned),
  ];

  const sqlSnippet = (s: string) => {
    const n = s.replace(/\s+/g, " ").trim();
    return n.length > 45 ? n.slice(0, 45) + "…" : n;
  };

  // ── Notebook toolbar state (read from store, bridged by NotebookTab) ──────
  const nbKernelReady    = useNotebookToolbarStore((s) => s.kernelReady);
  const nbKernelStarting = useNotebookToolbarStore((s) => s.kernelStarting);
  const nbKernelError    = useNotebookToolbarStore((s) => s.kernelError);
  const nbOnRestartKernel = useNotebookToolbarStore((s) => s.onRestartKernel);
  const nbOnAddCell       = useNotebookToolbarStore((s) => s.onAddCell);
  const nbOnDeploy        = useNotebookToolbarStore((s) => s.onDeploy);
  const nbOnRunAll        = useNotebookToolbarStore((s) => s.onRunAll);

  const handleNewNotebook = () => {
    const blank = JSON.stringify({
      nbformat: 4,
      nbformat_minor: 5,
      metadata: {
        kernelspec: { display_name: "Python 3", language: "python", name: "python3" },
        language_info: { name: "python", version: "3.12.0" },
      },
      cells: [{ cell_type: "code", execution_count: null, metadata: {}, outputs: [], source: [] }],
    }, null, 1);
    openNotebookUnsaved("Untitled Notebook", blank);
  };

  const nbSlotProps = isNotebookTab && nbOnRestartKernel && nbOnAddCell && nbOnDeploy ? {
    kernelReady: nbKernelReady,
    kernelStarting: nbKernelStarting,
    kernelError: nbKernelError,
    onRestartKernel: nbOnRestartKernel,
    onAddCell: nbOnAddCell,
    onDeploy: nbOnDeploy,
  } : null;

  return (
    <div data-query-layout style={{ display: "flex", flexDirection: "column", height: "100%", background: "var(--bg)" }}>
      {/* Unified Toolbar */}
      <Toolbar
        isRunning={isRunning}
        isCancelling={isCancelling}
        selectedSql={selectedSql}
        currentUser={currentUser}
        currentRegion={currentRegion}
        onRun={isNotebookTab && nbOnRunAll ? nbOnRunAll : () => runQuery()}
        onCancel={handleCancel}
        onDisconnect={handleDisconnect}
        onOpenSessionProperties={openSessionProperties}
        onOpenSnowsight={openSnowsight}
        onNewSql={openScratch}
        onNewNotebook={handleNewNotebook}
        onSave={handleSave}
        contextButtons={nbSlotProps ? notebookButtons(nbSlotProps) : undefined}
        contextStatus={nbSlotProps ? <NotebookStatusIndicator {...nbSlotProps} /> : undefined}
      />

      {/* Session error banner (role/warehouse switch failures) */}
      {sessionError && (
        <Alert
          type="error"
          message={sessionError}
          showIcon
          closable
          onClose={clearError}
          style={{ margin: "8px 12px 0", fontSize: 12 }}
        />
      )}

      {/* Tab bar */}
      <TabBar />

      {/* Diff view — replaces editor + results when the active tab is a diff tab */}
      {activeDiff && (
        <div style={{ flex: 1, overflow: "hidden", display: "flex", flexDirection: "column" }}>
          <div
            style={{
              display: "grid",
              gridTemplateColumns: "1fr 1fr",
              background: "var(--bg-raised)",
              borderBottom: "1px solid var(--border)",
              flexShrink: 0,
            }}
          >
            <div
              style={{ padding: "4px 12px", fontSize: 12, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", borderRight: "1px solid var(--border)" }}
              title={activeDiff.leftLabel}
            >
              {activeDiff.leftLabel}
            </div>
            <div
              style={{ padding: "4px 12px", fontSize: 12, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
              title={activeDiff.rightLabel}
            >
              {activeDiff.rightLabel}
            </div>
          </div>
          <div style={{ flex: 1, minHeight: 0 }}>
            <DiffEditor
              height="100%"
              language="sql"
              theme={resolved === "dark" ? "thaw-dark" : "thaw-light"}
              original={activeDiff.leftText}
              modified={activeDiff.rightText}
              beforeMount={ensureMonacoSetup}
              options={{
                readOnly: true,
                renderSideBySide: true,
                minimap: { enabled: false },
                fontSize: editorFontSize,
                fontFamily: editorFont,
                scrollBeyondLastLine: false,
              }}
            />
          </div>
        </div>
      )}

      {/* Notebook view — replaces editor + results when the active tab is a notebook */}
      {!activeDiff && isNotebookTab && (
        <div style={{ flex: 1, overflow: "hidden" }}>
          <NotebookTab tabId={activeTabId} />
        </div>
      )}

      {/* SQL Editor — top portion (resizable) */}
      {!activeDiff && !isNotebookTab && (
        <div
          ref={editorAreaRef}
          className="editor-area"
          style={{ flex: `0 0 ${splitPct * 100}%`, borderBottom: "1px solid var(--border)", overflow: "hidden", display: "flex" }}
        >
          {/* Primary editor */}
          <div ref={primaryEditorRef} style={{ flex: splitTabId ? `0 0 ${splitW * 100}%` : "1 1 100%", overflow: "hidden" }}>
            <SqlEditor activeStmtIdx={activeStmtIdx} />
          </div>

          {/* Vertical drag handle + secondary editor */}
          {splitTabId && splitTab && <>
            <div
              style={{
                width: 4, cursor: "col-resize",
                background: "var(--border)", flexShrink: 0,
                position: "relative", zIndex: 1,
              }}
              onMouseDown={(e) => {
                vSplitResizing.current = true;
                vSplitStartX.current  = e.clientX;
                vSplitStartW.current  = splitWLive.current;
                document.body.style.cursor     = "col-resize";
                document.body.style.userSelect = "none";
                e.preventDefault();
              }}
            />
            <div style={{ flex: 1, display: "flex", flexDirection: "column", overflow: "hidden" }}>
              {/* Secondary tab header */}
              <div style={{
                height: 24, display: "flex", alignItems: "center",
                justifyContent: "space-between", padding: "0 8px",
                background: "var(--bg-raised)", borderBottom: "1px solid var(--border)",
                fontSize: 12, color: "var(--text-muted)", flexShrink: 0,
              }}>
                <span>{splitTab.title}</span>
                <button
                  onClick={() => setSplitTab(null)}
                  style={{
                    background: "none", border: "none", cursor: "pointer",
                    color: "var(--text-muted)", fontSize: 14, lineHeight: 1, padding: 0,
                  }}
                >×</button>
              </div>
              <div style={{ flex: 1, overflow: "hidden" }}>
                <SqlEditor tabId={splitTabId} />
              </div>
            </div>
          </>}
        </div>
      )}

      {/* Horizontal resize handle */}
      {!activeDiff && !isNotebookTab && (
        <div
          style={{
            height: 5,
            flexShrink: 0,
            cursor: "row-resize",
            background: "transparent",
            borderTop: "1px solid var(--border)",
            transition: "background 0.15s",
            zIndex: 10,
          }}
          onMouseEnter={(e) => { e.currentTarget.style.background = "color-mix(in srgb, var(--accent) 26%, transparent)"; }}
          onMouseLeave={(e) => { if (!splitResizing.current) e.currentTarget.style.background = "transparent"; }}
          onMouseDown={(e) => {
            splitResizing.current = true;
            splitStartY.current   = e.clientY;
            splitStartPct.current = splitLivePct.current;
            document.body.style.cursor     = "row-resize";
            document.body.style.userSelect = "none";
            e.preventDefault();
            const parent = (e.currentTarget as HTMLElement).closest("[data-query-layout]") as HTMLElement | null;
            const onMove = (ev: MouseEvent) => {
              if (!parent) return;
              const delta = ev.clientY - splitStartY.current;
              const pct = splitStartPct.current + delta / parent.clientHeight;
              const clamped = Math.min(0.85, Math.max(0.15, pct));
              splitLivePct.current = clamped;
              if (editorAreaRef.current) {
                editorAreaRef.current.style.flex = `0 0 ${clamped * 100}%`;
              }
            };
            const onUp = () => {
              splitResizing.current = false;
              document.body.style.cursor     = "";
              document.body.style.userSelect = "";
              const pct = splitLivePct.current;
              setSplitPct(pct);
              setEditorSplit(pct);
              window.removeEventListener("mousemove", onMove);
              window.removeEventListener("mouseup",   onUp);
            };
            window.addEventListener("mousemove", onMove);
            window.addEventListener("mouseup",   onUp);
          }}
        />
      )}

      {/* Results / AI Chat — bottom portion */}
      {!activeDiff && !isNotebookTab &&
      <div style={{ flex: 1, overflow: "hidden", display: "flex", flexDirection: "column" }}>
        {/* Tab bar */}
        <div style={{ display: "flex", background: "var(--bg-raised)", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
          {(["results", ...(aiEnabled && featureFlags.aiChat ? ["chat"] : []), ...(terminalOpen && featureFlags.embeddedTerminal ? ["terminal"] : [])] as Array<"results" | "chat" | "terminal">).map((tab) => (
            <button
              key={tab}
              onClick={() => setResultPane(tab)}
              style={{
                padding: "4px 14px",
                fontSize: 12,
                background: "none",
                border: "none",
                borderBottom: resultPane === tab ? "2px solid var(--accent)" : "2px solid transparent",
                color: resultPane === tab ? "var(--text)" : "var(--text-muted)",
                cursor: "pointer",
              }}
            >
              {tab === "results" ? "Results" : tab === "chat" ? "AI Chat" : "Terminal"}
            </button>
          ))}
        </div>

        <div style={{ flex: 1, overflow: "hidden", position: "relative", display: resultPane === "results" ? "block" : "none" }}>
            {isRunning && (
              <div style={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 12, zIndex: 10, background: "rgba(0,0,0,0.4)" }}>
                <Spin size="large" />
                {stmtProgress && stmtProgress.total > 1 ? (
                  <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 4 }}>
                    <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
                      statement {stmtProgress.index + 1} of {stmtProgress.total}
                    </Text>
                    {(stmtProgress.queryID || runningQueryId) && (
                      <Space size={4}>
                        <Text style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}>
                          {stmtProgress.queryID || runningQueryId}
                        </Text>
                        <Button
                          type="text"
                          size="small"
                          icon={<CopyOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                          style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                          onClick={async () => { await ClipboardSetText((stmtProgress.queryID || runningQueryId)!); message.success("Query ID copied"); }}
                        />
                        {featureFlags.queryProfile && (
                          <Tooltip title="Query Profile">
                            <Button
                              type="text"
                              size="small"
                              icon={<BarChartOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                              style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                              onClick={() => { setProfileQueryId((stmtProgress.queryID || runningQueryId)!); setProfileQuerySql(selectedSql.trim() || sql.trim()); setProfileIsLive(true); }}
                            />
                          </Tooltip>
                        )}
                      </Space>
                    )}
                  </div>
                ) : runningQueryId ? (
                  <Space size={4}>
                    <Text style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}>
                      {runningQueryId}
                    </Text>
                    <Button
                      type="text"
                      size="small"
                      icon={<CopyOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                      style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                      onClick={async () => { await ClipboardSetText(runningQueryId); message.success("Query ID copied"); }}
                    />
                    {featureFlags.queryProfile && (
                      <Tooltip title="Query Profile">
                        <Button
                          type="text"
                          size="small"
                          icon={<BarChartOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                          style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                          onClick={() => { setProfileQueryId(runningQueryId); setProfileQuerySql(selectedSql.trim() || sql.trim()); setProfileIsLive(true); }}
                        />
                      </Tooltip>
                    )}
                  </Space>
                ) : null}
              </div>
            )}

            {displayedResult ? (
              <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
                {/* Error banner spans full width above both panels */}
                {error && (
                  <Alert
                    type="error"
                    message={error}
                    showIcon
                    closable
                    style={{ margin: "8px 12px 0", flexShrink: 0 }}
                  />
                )}
                {/* Status bar — two rows when compare is active, one row otherwise */}
                <div style={{ display: "flex", flexDirection: "column", background: "var(--bg-raised)", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
                  {/* ── Row 1: primary controls ──────────────────────────── */}
                  <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "3px 12px" }}>
                    {/* History selector */}
                    {resultHistory.length > 1 && (
                      <Select
                        size="small"
                        value={historyId}
                        onChange={(v) => updateTabHistoryId(activeTabId, v)}
                        style={{ fontSize: 11, width: 220 }}
                        popupMatchSelectWidth={false}
                        options={sortedHistory.map((e) => {
                          const origIdx = resultHistory.indexOf(e);
                          return {
                            value: e.id,
                            label: `${e.pinned ? "📌 " : ""}#${origIdx + 1}${origIdx === 0 ? " · " : "  "}${sqlSnippet(e.sql)}`,
                          };
                        })}
                        optionRender={(option) => {
                          const entry = resultHistory.find((e) => e.id === option.value);
                          if (!entry) return option.label;
                          const ctxMenu: MenuProps = { items: [{
                            key: "side-by-side",
                            icon: <LayoutOutlined />,
                            label: "View side by side",
                            disabled: entry.id === historyId || entry.id === compareHistoryId,
                            onClick: ({ domEvent }) => { domEvent.stopPropagation(); setCompareHistoryId(entry.id); },
                          }]};
                          return (
                            <Dropdown trigger={["contextMenu"]} menu={ctxMenu}>
                              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 4 }}>
                                <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{option.label as string}</span>
                                <Button
                                  type="text"
                                  size="small"
                                  icon={entry.pinned
                                    ? <PushpinFilled style={{ fontSize: 11, color: "var(--color-primary, #1677ff)" }} />
                                    : <PushpinOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
                                  style={{ height: 16, padding: "0 2px", minWidth: 0, flexShrink: 0 }}
                                  onClick={(e) => { e.stopPropagation(); e.preventDefault(); togglePin(entry.id); }}
                                />
                              </div>
                            </Dropdown>
                          );
                        }}
                      />
                    )}
                    {displayedResult.queryID && (
                      <Space size={4}>
                        <Text style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}>
                          {displayedResult.queryID}
                        </Text>
                        <Button
                          type="text"
                          size="small"
                          icon={<CopyOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                          style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                          onClick={async () => { await ClipboardSetText(displayedResult.queryID!); message.success("Query ID copied"); }}
                        />
                        {featureFlags.queryProfile && (
                          <Tooltip title="Query Profile">
                            <Button
                              type="text"
                              size="small"
                              icon={<BarChartOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                              style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                              onClick={() => { setProfileQueryId(displayedResult.queryID!); setProfileQuerySql(resultHistory.find((e) => e.id === historyId)?.sql ?? ""); setProfileIsLive(false); }}
                            />
                          </Tooltip>
                        )}
                      </Space>
                    )}
                    <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 6 }}>
                      {featureFlags.resultsetExport && (
                        <Tooltip title="Export as CSV">
                          <Button
                            type="text"
                            size="small"
                            icon={<FileTextOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
                            style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                            onClick={exportCSV}
                          />
                        </Tooltip>
                      )}
                      {featureFlags.resultsetExport && (
                        <Tooltip title="Export as Excel">
                          <Button
                            type="text"
                            size="small"
                            icon={<FileExcelOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
                            style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                            onClick={exportExcel}
                          />
                        </Tooltip>
                      )}
                      <Text style={{ fontSize: 11, color: "var(--text-faint)" }}>
                        {displayedResult.rows.length} row{displayedResult.rows.length !== 1 ? "s" : ""}
                      </Text>
                      {displayedResult.truncated && (
                        <Tooltip title="Results capped at 50,000 rows. Add a LIMIT clause or refine your query to see all data.">
                          <Tag color="orange" style={{ fontSize: 10, lineHeight: "16px", padding: "0 5px", marginInlineEnd: 0 }}>truncated</Tag>
                        </Tooltip>
                      )}
                    </div>
                  </div>
                  {/* ── Row 2: compare info, right-aligned (only when active) */}
                  {compareResult && compareEntry && (
                    <div style={{ display: "flex", alignItems: "center", justifyContent: "flex-end", gap: 6, padding: "2px 8px 3px 12px", borderTop: "1px solid var(--border)" }}>
                      <Text style={{ fontSize: 11, color: "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                        {sqlSnippet(compareEntry.sql)}
                      </Text>
                      {compareEntry.queryID && (
                        <Space size={4} style={{ flexShrink: 0 }}>
                          <Text style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}>
                            {compareEntry.queryID}
                          </Text>
                          <Button
                            type="text"
                            size="small"
                            icon={<CopyOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                            style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                            onClick={async () => { await ClipboardSetText(compareEntry.queryID!); message.success("Query ID copied"); }}
                          />
                        </Space>
                      )}
                      <Text style={{ fontSize: 11, color: "var(--text-faint)", flexShrink: 0 }}>
                        {compareResult.rows.length} row{compareResult.rows.length !== 1 ? "s" : ""}
                      </Text>
                      <Tooltip title="Close side-by-side view">
                        <Button
                          type="text"
                          size="small"
                          icon={<CloseOutlined style={{ fontSize: 11 }} />}
                          style={{ height: 18, padding: "0 4px", minWidth: 0, flexShrink: 0 }}
                          onClick={() => setCompareHistoryId(null)}
                        />
                      </Tooltip>
                    </div>
                  )}
                </div>
                {/* Grids row — bare grids, headers at the same level */}
                <div style={{ display: "flex", flex: 1, overflow: "hidden" }}>
                  <div style={{ flex: 1, overflow: "hidden", ...(compareResult ? { borderRight: "1px solid var(--border)" } : {}) }}>
                    <ResultGrid
                      result={displayedResult}
                      syncScrollRef={compareResult ? primarySyncRef : undefined}
                      onVerticalScroll={compareResult ? (top) => compareSyncRef.current?.scrollTo(top) : undefined}
                    />
                  </div>
                  {compareResult && compareEntry && (
                    <div style={{ flex: 1, overflow: "hidden" }}>
                      <ResultGrid
                        result={compareResult}
                        syncScrollRef={compareSyncRef}
                        onVerticalScroll={(top) => primarySyncRef.current?.scrollTo(top)}
                      />
                    </div>
                  )}
                </div>{/* end grids row */}
              </div>
            ) : (
              <>
                {/* Error with no active result shown — offer the history picker */}
                {error && (
                  <Alert
                    type="error"
                    message={error}
                    showIcon
                    closable
                    style={{ margin: 12 }}
                  />
                )}
                {resultHistory.length > 0 && !isRunning && (
                  <div style={{ padding: "4px 12px 8px", display: "flex", alignItems: "center", gap: 8 }}>
                    <Text style={{ fontSize: 12, color: "var(--text-muted)", whiteSpace: "nowrap" }}>Previous results:</Text>
                    <Select
                      size="small"
                      placeholder="Select to view…"
                      value={undefined}
                      onChange={(v: string) => updateTabHistoryId(activeTabId, v)}
                      style={{ fontSize: 11, width: 260 }}
                      popupMatchSelectWidth={false}
                      options={sortedHistory.map((e) => {
                        const origIdx = resultHistory.indexOf(e);
                        return {
                          value: e.id,
                          label: `${e.pinned ? "📌 " : ""}#${origIdx + 1}  ${sqlSnippet(e.sql)}`,
                        };
                      })}
                      optionRender={(option) => {
                        const entry = resultHistory.find((e) => e.id === option.value);
                        if (!entry) return option.label;
                        return (
                          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 4 }}>
                            <span style={{ overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{option.label as string}</span>
                            <Button
                              type="text"
                              size="small"
                              icon={entry.pinned
                                ? <PushpinFilled style={{ fontSize: 11, color: "var(--color-primary, #1677ff)" }} />
                                : <PushpinOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
                              style={{ height: 16, padding: "0 2px", minWidth: 0, flexShrink: 0 }}
                              onClick={(e) => { e.stopPropagation(); e.preventDefault(); togglePin(entry.id); }}
                            />
                          </div>
                        );
                      }}
                    />
                  </div>
                )}
                {!error && !isRunning && resultHistory.length === 0 && (
                  <div style={{ padding: 24, color: "var(--text-faint)", fontSize: 13 }}>
                    Run a query to see results here.
                  </div>
                )}
              </>
            )}
          </div>

          <div style={{ flex: 1, overflow: "hidden", display: resultPane === "chat" ? "flex" : "none", flexDirection: "column" }}>
            <AiChat />
          </div>

          {terminalOpen && (
            <div style={{ flex: 1, overflow: "hidden", display: resultPane === "terminal" ? "flex" : "none", flexDirection: "column" }}>
              <TerminalPanel onClose={() => { setTerminalOpen(false); setResultPane("results"); }} />
            </div>
          )}
      </div>}

      {snippetsOpen && <SnippetsModal onClose={() => setSnippetsOpen(false)} />}
      {exportPathFormatOpen && <ExportPathFormatModal onClose={() => setExportPathFormatOpen(false)} />}
      {migrationOpen && <MigrationModal onClose={() => setMigrationOpen(false)} />}
      {dbtCreateOpen && <DbtProjectModal onClose={() => setDbtCreateOpen(false)} />}
      {fnCatalogOpen && <FunctionCatalogModal onClose={() => setFnCatalogOpen(false)} />}
      {kbShortcutsOpen && <KeyboardShortcutsModal onClose={() => setKbShortcutsOpen(false)} />}
      {aboutOpen && <AboutModal onClose={() => setAboutOpen(false)} />}

      {sessionPropsOpen && (
        <SessionPropertiesModal
          parameters={sessionParams}
          variables={sessionVars}
          error={sessionPropsError}
          onClose={() => setSessionPropsOpen(false)}
          onParamChange={handleParamChange}
          onVarChange={handleVarChange}
        />
      )}

      <Modal
        open={snowsightModalOpen}
        title="Open Snowsight"
        onCancel={() => setSnowsightModalOpen(false)}
        footer={[
          <Button key="copy" icon={<CopyOutlined />} onClick={() => {
            if (snowsightUrl) ClipboardSetText(snowsightUrl).then(() => message.success("Link copied"));
          }}>
            Copy Link
          </Button>,
          <Button key="open" type="primary" icon={<GlobalOutlined />} onClick={() => {
            if (snowsightUrl) { BrowserOpenURL(snowsightUrl); setSnowsightModalOpen(false); }
          }}>
            Open in Browser
          </Button>,
          <Button key="cancel" onClick={() => setSnowsightModalOpen(false)}>Cancel</Button>,
        ]}
      >
        {snowsightUrl
          ? <Typography.Text copyable={{ text: snowsightUrl }} style={{ wordBreak: "break-all" }}>{snowsightUrl}</Typography.Text>
          : <Spin size="small" />}
      </Modal>

      {profileQueryId && (
        <QueryProfileModal
          queryId={profileQueryId}
          sql={profileQuerySql}
          onClose={() => setProfileQueryId(null)}
          liveRefresh={profileIsLive && isRunning}
        />
      )}

      <Modal
        open={closeConfirm !== null}
        title={closeConfirm?.isRunning ? "Running query" : "Unsaved changes"}
        onCancel={() => setCloseConfirm(null)}
        footer={[
          <Button key="cancel" onClick={() => setCloseConfirm(null)}>
            Cancel
          </Button>,
          // "Discard / Stop & Discard" — always shown
          <Button
            key="discard"
            danger
            onClick={() => {
              if (!closeConfirm) return;
              if (closeConfirm.isRunning) {
                // Fire-and-forget cancel; CloseTabSession (lifecycle effect) finishes cleanup.
                CancelQuery(closeConfirm.tabId).catch(() => {});
              }
              const { tabs } = useQueryStore.getState();
              const tab = tabs.find((t) => t.id === closeConfirm.tabId);
              if (tab) {
                closedTabsRef.current.unshift({ path: tab.path, title: tab.title, sql: tab.sql, kind: tab.kind });
                if (closedTabsRef.current.length > 15) closedTabsRef.current.pop();
              }
              useQueryStore.getState().closeTab(closeConfirm.tabId);
              setCloseConfirm(null);
            }}
          >
            {closeConfirm?.isRunning ? (closeConfirm.isDirty ? "Stop & Discard" : "Stop & Close") : "Close without Saving"}
          </Button>,
          // "Save" — only shown when there are unsaved changes
          ...(closeConfirm?.isDirty ? [
            <Button
              key="save"
              type="primary"
              onClick={async () => {
                if (!closeConfirm) return;
                if (closeConfirm.isRunning) {
                  CancelQuery(closeConfirm.tabId).catch(() => {});
                }
                const saved = await saveTabById(closeConfirm.tabId);
                if (saved) {
                  const { tabs } = useQueryStore.getState();
                  const tab = tabs.find((t) => t.id === closeConfirm.tabId);
                  if (tab) {
                    closedTabsRef.current.unshift({ path: tab.path, title: tab.title, sql: tab.sql, kind: tab.kind });
                    if (closedTabsRef.current.length > 15) closedTabsRef.current.pop();
                  }
                  useQueryStore.getState().closeTab(closeConfirm.tabId);
                  setCloseConfirm(null);
                }
              }}
            >
              {closeConfirm?.isRunning ? "Stop & Save" : "Save"}
            </Button>,
          ] : []),
        ]}
      >
        <p>
          {closeConfirm?.isRunning && closeConfirm.isDirty
            ? <><strong>{closeConfirm.title}</strong> has a running query and unsaved changes. Stop the query and close?</>
            : closeConfirm?.isRunning
            ? <><strong>{closeConfirm.title}</strong> has a running query. Stop the query and close?</>
            : <><strong>{closeConfirm?.title}</strong> has unsaved changes. Do you want to save before closing?</>
          }
        </p>
      </Modal>
    </div>
  );
}
