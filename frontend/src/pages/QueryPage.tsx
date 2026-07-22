// SPDX-License-Identifier: GPL-3.0-or-later

import { useEffect, useRef, useState, useCallback, lazy, Suspense } from "react";
import { flushSync } from "react-dom";
import { useShallow } from "zustand/react/shallow";
import { Button, Dropdown, Space, Typography, Alert, Spin, Tag, Select, Tooltip, message, Modal, type MenuProps } from "antd";
import { CopyOutlined, FileTextOutlined, FileExcelOutlined, PushpinOutlined, PushpinFilled, CloseOutlined, LayoutOutlined, GlobalOutlined, BarChartOutlined, SearchOutlined, CloudUploadOutlined } from "@ant-design/icons";
import { ClipboardSetText, BrowserOpenURL } from "../../wailsjs/runtime/runtime";
import { StartQuery, WaitForQueryResult, CancelQuery, Disconnect, SaveFile, PickSaveFile, PickSaveExportFile, SaveBinaryFile, PickOpenFile, PickAnyFile, ReadFile, GetSessionParameters, GetSessionVariables, GetAccountParameters, PickNotebookFile, ReadNotebook, NotebookUseContext, SaveNotebook, GetCurrentUser, GetCurrentRegion, GetSnowsightURL, CloseTabSession, GetSessionInitMode, InitTabSession, SetQueryLogEnabled, GetFeatureFlags, SaveFeatureFlags, StartFileWatcher, StopFileWatcher } from "../../wailsjs/go/app/App";
import { GetSqlStatementRanges } from "../../wailsjs/go/sqleditor/Service";
import type { snowflake } from "../../wailsjs/go/models";
import { usePanelLayoutStore } from "../store/panelLayoutStore";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import SqlEditor, { type DiagMarker, pendingMcpMarkers, clearMetadataCaches } from "../components/editor/SqlEditor";
import TabBar from "../components/editor/TabBar";
import { DiffEditor } from "@monaco-editor/react";
import { ensureMonacoSetup } from "../components/editor/monacoSetup";
import { patchMonacoClipboard } from "../utils/monacoClipboard";
import { useThemeStore } from "../store/themeStore";
import ResultGrid, { type ResultGridHandle } from "../components/results/ResultGrid";
import GridSearch from "../components/results/GridSearch";
import StatusBar from "../components/results/StatusBar";
import CellDetailPanel from "../components/results/CellDetailPanel";
import QueryLogPane from "../components/results/QueryLogPane";

// Catalog-mutating statement leaders. Only a run that creates/alters/drops objects
// changes the columns/object lists the editor caches, so only these trigger a
// metadata-cache clear after a run. Matched at the buffer start or after a `;`
// statement boundary, case-insensitive. RENAME/SWAP/SET arrive via ALTER.
const DDL_LEADER_RE = /(?:^|;)\s*(?:CREATE|ALTER|DROP|TRUNCATE|UNDROP)\b/i;

// Whether an error from ReadFile/ReadNotebook means the file is actually gone
// (vs. a transient IPC/permission/binary-file error). Both backends map
// os.ErrNotExist to a locale-independent "file not found" marker before the error
// crosses the Wails bridge (the raw OS message is localized, so it can't be
// matched on non-English Windows), so matching that single marker is exact —
// raw OS-string forms are deliberately not matched (they'd risk a spurious
// orphan on an unrelated IPC error like a Wails "cannot find module").
function isFileNotFound(e: unknown): boolean {
  // The marker is lowercase ASCII, formatted as `<marker>: <path>`; no case-fold needed.
  return String(e).includes("file not found");
}

// ranContainedDDL reports whether the executed SQL has a DDL statement. Comments
// and string/dollar-quoted literals are blanked first so a `;` or DDL verb inside
// one (e.g. INSERT … VALUES ('ran; CREATE TABLE x')) can't trigger a spurious
// clear — a real statement boundary only exists in code, not inside a literal.
function ranContainedDDL(sql: string): boolean {
  const code = sql
    .replace(/\$\$[\s\S]*?\$\$/g, "$$$$") // dollar-quoted blocks
    .replace(/'(?:[^']|'')*'/g, "''")     // single-quoted string literals
    .replace(/"(?:[^"]|"")*"/g, '""')     // quoted identifiers
    .replace(/--[^\n]*/g, "")             // line comments
    .replace(/\/\*[\s\S]*?\*\//g, "");    // block comments
  return DDL_LEADER_RE.test(code);
}

// ── Lazy-loaded panels & modals ───────────────────────────────────────────────
// None of these are on the initial render path — they mount only when the user
// opens a notebook, the terminal, or a specific dialog.  React.lazy keeps their
// code (and heavy deps: xterm for the terminal, the notebook kernel UI, the
// migration / dbt / function-catalog modules) out of the eager boot bundle, so
// they download on first use instead of at cold start.
const SessionPropertiesModal = lazy(() => import("../components/common/SessionPropertiesModal"));
const AccountParametersModal = lazy(() => import("../components/common/AccountParametersModal"));
const SnippetsModal          = lazy(() => import("../components/snippets/SnippetsModal"));
const ExportPathFormatModal  = lazy(() => import("../components/export/ExportPathFormatModal"));
const ExportOptionsModal     = lazy(() => import("../components/export/ExportOptionsModal"));
const ExcelExportModal       = lazy(() => import("../components/export/ExcelExportModal"));
const MigrationModal         = lazy(() => import("../components/migration/MigrationModal"));
const DbtProjectModal        = lazy(() => import("../components/dbt/DbtProjectModal"));
const FunctionCatalogModal   = lazy(() => import("../components/fnmeta/FunctionCatalogModal"));
const TagManagementModal     = lazy(() => import("../components/tag/TagManagementModal"));
const KeyboardShortcutsModal = lazy(() => import("../components/help/KeyboardShortcutsModal"));
const AboutModal             = lazy(() => import("../components/help/AboutModal"));
const CrossTabSearch         = lazy(() => import("../components/editor/CrossTabSearch"));
const QueryProfileModal      = lazy(() => import("../components/results/QueryProfileModal"));
const TerminalPanel          = lazy(() => import("../components/terminal/TerminalPanel"));
const NotebookTab            = lazy(() => import("../components/notebook/NotebookTab"));
import { useQueryStore, type QueryResult, type Tab, EXECUTE_IN_TAB_EVENT } from "../store/queryStore";
import { openFileInTab } from "../utils/openFileInTab";
import { deriveSheetName } from "../utils/excelSheetName";
import type { ExcelExportEntry } from "../components/export/ExcelExportModal";
import { useConnectionStore } from "../store/connectionStore";
import { useSessionStore } from "../store/sessionStore";
import { useGitStore } from "../store/gitStore";
import { useFeatureFlagsStore } from "../store/featureFlagsStore";
import { useTagManagementStore } from "../store/tagManagementStore";
import { useNotebookToolbarStore } from "../store/notebookToolbarStore";
import { useGridStore } from "../store/gridStore";
import Toolbar from "../components/toolbar/Toolbar";
import { NotebookToolbarSlot } from "../components/notebook/NotebookToolbarSlot";
import { useEditorContextSync } from "../hooks/useEditorContextSync";

const { Text } = Typography;

// Default save-dialog filename for a tab: untitled.sql for scratch tabs still
// carrying their auto-generated "SQL (n)" title, otherwise the tab's own title.
const saveDefaultName = (tab: Tab) =>
  tab.isDefaultTitle ? "untitled.sql" : tab.title;

export default function QueryPage() {
  // Subscribe to specific fields via useShallow — NOT the bare `useQueryStore()`,
  // which subscribed to the whole store and re-rendered the entire page (results
  // grid included) on every keystroke. `sql` is deliberately excluded: it changes
  // on every keystroke and is only needed inside callbacks (which read it fresh
  // via getState), so leaving it out is what stops the per-keystroke re-render
  // storm that made fast typing lag by seconds. (#762)
  const { selectedSql, isRunning, error, setResult, setError, markSaved, openScratch, openNotebook, openNotebookUnsaved, refreshFileTab, orphanFileTab } =
    useQueryStore(
      useShallow((s) => ({
        selectedSql: s.selectedSql,
        isRunning: s.isRunning,
        error: s.error,
        setResult: s.setResult,
        setError: s.setError,
        markSaved: s.markSaved,
        openScratch: s.openScratch,
        openNotebook: s.openNotebook,
        openNotebookUnsaved: s.openNotebookUnsaved,
        refreshFileTab: s.refreshFileTab,
        orphanFileTab: s.orphanFileTab,
      })),
    );
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
  const [resultPane, setResultPane] = useState<"results" | "terminal" | "querylog">("results");
  const [terminalOpen, setTerminalOpen] = useState(false);
  const featureFlags = useFeatureFlagsStore((s) => s.flags);
  const tagMgmtOpen = useTagManagementStore((s) => s.open);
  const openTagMgmt = useTagManagementStore((s) => s.openView);
  const closeTagMgmt = useTagManagementStore((s) => s.closeView);

  // Sync editor state to the MCP EditorContextStore so external AI clients
  // can read the active SQL and query results.
  useEditorContextSync();

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
  const [excelExportOpen, setExcelExportOpen] = useState(false);
  const [exportDdlOpen, setExportDdlOpen] = useState(false);
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
  const [sessionParams, setSessionParams] = useState<snowflake.SessionParam[] | null>(null);
  const [sessionVars, setSessionVars] = useState<snowflake.SessionVar[] | null>(null);
  const [sessionPropsError, setSessionPropsError] = useState<string | null>(null);
  const [accountParamsOpen, setAccountParamsOpen] = useState(false);
  const [accountParams, setAccountParams] = useState<snowflake.SessionParam[] | null>(null);
  const [accountParamsError, setAccountParamsError] = useState<string | null>(null);
  // Ref so the async runQuery closure can detect user-initiated cancellation
  // without relying on stale React state.
  const cancelRequestedRef = useRef(false);
  // Grid handle for search scroll-to-row.
  const primaryGridRef = useRef<ResultGridHandle | null>(null);
  // Grid search bar visibility.
  const [gridSearchOpen, setGridSearchOpen] = useState(false);
  // Cross-tab search/replace panel visibility.
  const [crossTabSearchOpen, setCrossTabSearchOpen] = useState(false);
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
    setGridSearchOpen(false);
    useGridStore.getState().resetNavigation();
  }, [activeTabId]);

  // Track tab additions/removals to manage sessions. Subscribe to the tab-id
  // *set* (a signature string), NOT the `tabs` array identity — a per-keystroke
  // SQL edit rebuilds `tabs`, so subscribing to the array re-rendered the whole
  // page on every character. The signature only changes when a tab is added or
  // removed. Callbacks that need live tab data read it via getState(). (#762)
  const tabIdsSig = useQueryStore((s) => s.tabs.map((t) => t.id).join(" "));
  const prevTabIdsRef = useRef<Set<string>>(new Set(useQueryStore.getState().tabs.map((t) => t.id)));
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
    const currentIds = new Set(useQueryStore.getState().tabs.map((t) => t.id));
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
  }, [tabIdsSig]);

  // Re-read a single file-backed tab from disk: refresh content (clean tabs
  // only — see refreshFileTab) or orphan it if the file is gone.
  //
  // A per-tab read sequence guards against out-of-order completion: two change
  // events can launch concurrent reads of the same tab, and a slow earlier read
  // landing after a newer one would otherwise revert the tab to stale content
  // (observable on network filesystems). Only the latest-issued read applies.
  const readSeqRef = useRef<Map<string, number>>(new Map());
  const rereadTab = useCallback(async (tab: Tab) => {
    if (!tab.path) return;
    const seq = (readSeqRef.current.get(tab.id) ?? 0) + 1;
    readSeqRef.current.set(tab.id, seq);
    try {
      const content = tab.kind === "notebook"
        ? await ReadNotebook(tab.path)
        : await ReadFile(tab.path);
      if (readSeqRef.current.get(tab.id) !== seq) return; // superseded by a newer read
      refreshFileTab(tab.id, content);
    } catch (e) {
      if (readSeqRef.current.get(tab.id) !== seq) return; // superseded by a newer read
      // Only drop the file association when the file is truly gone — a transient
      // IPC/permission error must not permanently orphan a still-valid tab. Surface
      // anything else (binary file, EACCES, …) so a blank-reopen isn't silent.
      if (isFileNotFound(e)) orphanFileTab(tab.id);
      else console.warn(`Could not re-read ${tab.path}:`, e);
    }
  }, [refreshFileTab, orphanFileTab]);

  // On mount, re-read file-backed tabs from disk so their content is fresh
  // after an app restart (they were persisted with cleared sql/savedSql).
  useEffect(() => {
    useQueryStore.getState().tabs.forEach(rereadTab);
  }, [rereadTab]);

  // Reflect external edits: when the file watcher reports any change, re-read
  // every open file-backed tab. We deliberately don't match the event's
  // directory against tab paths — native open-dialogs return canonical
  // (symlink-resolved) paths while the watcher reports the pre-resolution root,
  // so the two namespaces can differ (e.g. /tmp vs /private/tmp on macOS).
  //
  // The watcher debounces per *directory*, so a tool touching K directories at
  // once fires K separate events; debouncing here collapses that burst into a
  // single fan-out so the cost stays N (one ReadFile per tab), not K×N.
  // ponytail: refreshFileTab no-ops when content is unchanged, so a re-read of an
  // untouched tab is just one cheap ReadFile. Revisit only if N grows large.
  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    const off = EventsOn("fs:changed", () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        useQueryStore.getState().tabs.forEach(rereadTab);
      }, 250);
    });
    return () => { if (timer) clearTimeout(timer); off(); };
  }, [rereadTab]);

  // Own the file watcher lifecycle here rather than in FileBrowser: QueryPage is
  // always mounted, whereas the left sidebar (and FileBrowser) is unmounted when
  // hidden via ⌘B — which would otherwise stop the watcher and silently freeze
  // tab refresh. Gated on the same `fileWatcher` flag the FileBrowser tree uses.
  const exportDir = useGitStore((s) => s.exportDir);
  // Bumped when File Watching preferences are saved so the watcher restarts and
  // picks up new exclude globs / caps / FD-limit setting without an app restart.
  const [fileWatchConfigVer, setFileWatchConfigVer] = useState(0);
  useEffect(() => {
    const onSaved = () => setFileWatchConfigVer((v) => v + 1);
    window.addEventListener("thaw:filewatch-config-saved", onSaved);
    return () => window.removeEventListener("thaw:filewatch-config-saved", onSaved);
  }, []);
  useEffect(() => {
    if (!exportDir || !featureFlags.fileWatcher) return;
    StartFileWatcher(exportDir).catch((e) => console.warn("File watcher failed to start:", e));
    return () => { StopFileWatcher().catch(() => {}); };
  }, [exportDir, featureFlags.fileWatcher, fileWatchConfigVer]);

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
    // `sql` is no longer a reactive subscription (#762) — read the live active-tab
    // text from the store at call time so a run always uses the current buffer.
    const sql = useQueryStore.getState().sql;
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
      // Only DDL changes the catalog, so only DDL needs the column/object metadata
      // dropped. Gating on a leading DDL verb keeps the common edit→run→edit SELECT
      // loop's warm cache instead of forcing a cold re-fetch after every run.
      if (ranContainedDDL(query)) clearMetadataCaches();
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
    const doDisconnect = async () => {
      await Disconnect(); // backend tears down all MCP sessions (StopAll)
      disconnect();
      // Disconnect stops every MCP session server-side; refresh the store so
      // the toolbar indicator and sessions modal don't show stale "Running".
      window.dispatchEvent(new Event("thaw:mcp-changed"));
    };
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

  const openAccountParameters = async () => {
    setAccountParamsOpen(true);
    setAccountParams(null);
    setAccountParamsError(null);
    try {
      setAccountParams(await GetAccountParameters());
    } catch (e) {
      setAccountParamsError(String(e));
    }
  };

  const openSnowsight = () => setSnowsightModalOpen(true);

  const handleParamChange = (key: string, value: string) => {
    setSessionParams((prev) => prev ? prev.map((p) => p.key === key ? { ...p, value } : p) : prev);
  };

  const handleVarChange = (key: string, value: string) => {
    setSessionVars((prev) => prev ? prev.map((v) => v.key === key ? { ...v, value } : v) : prev);
  };

  const handleAccountParamChange = (key: string, value: string) => {
    setAccountParams((prev) => prev ? prev.map((p) => p.key === key ? { ...p, value } : p) : prev);
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

  // Excel export button. With more than one resultset in history, open the
  // multi-select modal (one sheet per chosen resultset); otherwise export the
  // single displayed result directly.
  const exportExcel = async () => {
    if (resultHistory.length > 1) {
      setExcelExportOpen(true);
      return;
    }
    if (!displayedResult) return;
    const entry = resultHistory.find((e) => e.id === historyId);
    await writeExcelWorkbook([{ sql: entry?.sql ?? "", index: 1, result: displayedResult }]);
  };

  // Build a single .xlsx workbook with one sheet per resultset and save it via a
  // native dialog. Sheet names are derived from each query (31-char capped,
  // invalid chars stripped, de-duplicated).
  const writeExcelWorkbook = async (sheets: Array<{ sql: string; index: number; result: QueryResult }>) => {
    if (sheets.length === 0) return;
    // Load the ~17 MB xlsx library on demand — Excel export is rarely the first
    // thing a user does, so it must not sit in the eager boot bundle.
    const XLSX = await import("xlsx");
    const wb = XLSX.utils.book_new();
    const used = new Set<string>();
    for (const s of sheets) {
      const ws = XLSX.utils.aoa_to_sheet([s.result.columns, ...s.result.rows]);
      XLSX.utils.book_append_sheet(wb, ws, deriveSheetName(s.sql, s.index, used));
    }
    const b64 = XLSX.write(wb, { type: "base64", bookType: "xlsx" });
    const path = await PickSaveExportFile("results.xlsx", "excel");
    if (!path) return;
    try {
      await SaveBinaryFile(path, b64);
      message.success(sheets.length > 1 ? `Exported ${sheets.length} resultsets to Excel` : "Exported to Excel");
    } catch (e) {
      message.error(String(e));
    }
  };

  // Export the resultsets selected in the modal, in the given order (one sheet
  // each). `ids` are HistoryEntry ids; entries are resolved against the current
  // history and numbered by their dropdown position.
  const exportExcelSheets = async (ids: string[]) => {
    setExcelExportOpen(false);
    const sheets = ids
      .map((id) => {
        const entry = resultHistory.find((e) => e.id === id);
        return entry ? { sql: entry.sql, index: resultHistory.indexOf(entry) + 1, result: entry.result } : null;
      })
      .filter((s): s is { sql: string; index: number; result: QueryResult } => s !== null);
    await writeExcelWorkbook(sheets);
  };

  // Save to the tab's existing path, or open a Save As dialog if it has none.
  const handleSave = async () => {
    const { tabs, activeTabId, sql: currentSql } = useQueryStore.getState();
    const tab = tabs.find((t) => t.id === activeTabId);
    if (!tab) return;

    let savePath = tab.path;
    let saveTitle = tab.title;

    if (!savePath) {
      savePath = await PickSaveFile(saveDefaultName(tab));
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
      : saveDefaultName(tab);

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
      const defaultName = saveDefaultName(tab);
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
    // Clean up any pending MCP markers for this tab (prevents a small leak
    // if a tab is closed before its editor ever mounts and consumes them).
    pendingMcpMarkers.delete(tabId);
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

  const openPicked = async (filePath: string) => {
    if (!filePath) return;
    const err = await openFileInTab(filePath);
    if (err) message.error(`Open failed: ${err}`);
  };

  const handleOpen    = async () => openPicked(await PickOpenFile());
  const handleOpenAny = async () => openPicked(await PickAnyFile());

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

  // thaw:toggle-cross-tab-search — toggle the cross-tab search panel (fired
  // from the editor context menu action registered in SqlEditor).
  useEffect(() => {
    const handler = () => {
      setCrossTabSearchOpen((prev) => !prev);
    };
    window.addEventListener("thaw:toggle-cross-tab-search", handler);
    return () => window.removeEventListener("thaw:toggle-cross-tab-search", handler);
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
    const isMac = /Macintosh/i.test(navigator.userAgent);

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

      // ⌘⇧E / Ctrl+Shift+E — Toggle the Active Files dropdown
      if (cmd && e.shiftKey && !e.altKey && e.key === "E") {
        e.preventDefault();
        window.dispatchEvent(new Event("thaw:open-active-files"));
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
          const others = tabs.filter((t) => t.id !== activeTabId && (!t.kind || t.kind === "sql" || t.kind === "plaintext" || t.kind === "markdown"));
          if (others.length > 0) setSplitTab(others[others.length - 1].id);
        }
        return;
      }

      // ⌘G / Ctrl+G — Open Grid Search, or Find Next when already open
      // (skip when Monaco editor has focus or results pane isn't active)
      if (cmd && !e.shiftKey && !e.altKey && e.key === "g") {
        const monacoEl = document.querySelector(".monaco-editor");
        if (monacoEl?.contains(document.activeElement)) return;
        if (resultPane !== "results") return;
        e.preventDefault();
        if (gridSearchOpen) {
          useGridStore.getState().nextMatch();
        } else {
          setGridSearchOpen(true);
        }
        return;
      }

      // ⌘⇧F / Ctrl+Shift+F — Focus Object Browser Search
      if (cmd && e.shiftKey && !e.altKey && e.key === "F") {
        e.preventDefault();
        window.dispatchEvent(new Event("thaw:focus-object-search"));
        return;
      }

      // ⌘⇧H / Ctrl+Shift+H — Toggle cross-tab search/replace
      // Skip if Monaco already handled this keybinding via defaultPrevented.
      if (cmd && e.shiftKey && !e.altKey && e.key === "H" && !e.defaultPrevented) {
        e.preventDefault();
        setCrossTabSearchOpen((prev) => !prev);
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
    const offOpenAny = EventsOn("menu:open-any", () => handleOpenAny());
    const offSave    = EventsOn("menu:save",     () => handleSave());
    const offSaveAs  = EventsOn("menu:save-as",  () => handleSaveAs());
    return () => { offNewTab(); offOpen(); offOpenAny(); offSave(); offSaveAs(); };
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:open-terminal", () => {
      if (!featureFlags.embeddedTerminal) return;
      setTerminalOpen(true);
      setResultPane("terminal");
    });
    return () => off();
  }, [featureFlags.embeddedTerminal]);

  // Query Log — toggle and filter via Tools → Query Log menu.
  // Enabling from the Tools menu also turns on the queryLog feature flag (unless
  // an admin has locked it off) so the "Query Log" result-pane tab appears — the
  // pane is otherwise hidden behind that flag and the action would look like a
  // no-op. Uses the store's getState() so the handler never goes stale.
  useEffect(() => {
    const off = EventsOn("menu:query-log-toggle", async (enabled: boolean) => {
      const store = useFeatureFlagsStore.getState();
      if (enabled && !store.flags.queryLog) {
        if (store.locked.queryLog) {
          message.warning("Query Log is disabled by your IT administrator.");
          return;
        }
        try {
          const current = await GetFeatureFlags();
          await SaveFeatureFlags({ ...current, queryLog: true } as any);
          await store.load();
        } catch (err) {
          message.error(String(err));
          return;
        }
      }
      if (!useFeatureFlagsStore.getState().flags.queryLog) return;
      if (enabled) setResultPane("querylog");
      SetQueryLogEnabled(enabled);
    });
    return () => off();
  }, []);

  // Reset to Results pane if queryLog feature flag is disabled while viewing the log.
  useEffect(() => {
    if (!featureFlags.queryLog) {
      setResultPane((prev) => prev === "querylog" ? "results" : prev);
    }
  }, [featureFlags.queryLog]);

  useEffect(() => {
    const off = EventsOn("menu:code-snippets", () => { if (featureFlags.codeSnippets) setSnippetsOpen(true); });
    return () => off();
  }, [featureFlags.codeSnippets]);

  useEffect(() => {
    const off = EventsOn("menu:export-path-format", () => setExportPathFormatOpen(true));
    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:export-ddl", () => setExportDdlOpen(true));
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
    const off = EventsOn("menu:tag-management", () => openTagMgmt());
    return () => off();
  }, [openTagMgmt]);

  useEffect(() => {
    const off = EventsOn("menu:keyboard-shortcuts", () => setKbShortcutsOpen(true));
    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:about", () => setAboutOpen(true));
    return () => off();
  }, []);

  // MCP open_sql_tab — opens a new tab with AI-generated SQL and pre-seeded diagnostics.
  useEffect(() => {
    const off = EventsOn("mcp:open-sql-tab", (payload: {
      title: string; sql: string; markers: DiagMarker[];
    }) => {
      const tabId = useQueryStore.getState().openMcpTab(payload.title, payload.sql);
      if (payload.markers?.length > 0) {
        pendingMcpMarkers.set(tabId, payload.markers);
      }
    });
    return () => off();
  }, []);

  // MCP open_notebook_tab — opens a new notebook tab with AI-generated cells.
  useEffect(() => {
    const off = EventsOn("mcp:open-notebook-tab", (payload: {
      title: string; content: string;
    }) => {
      useQueryStore.getState().openMcpNotebookTab(payload.title, payload.content);
    });
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

  // Resultsets selectable in the Excel export modal, most-recent-first (matching
  // the history dropdown order). The 1-based index is the resultset number shown
  // in the dropdown and drives the "Result N" sheet-name fallback.
  const excelExportEntries: ExcelExportEntry[] = sortedHistory.map((e) => ({
    id: e.id,
    index: resultHistory.indexOf(e) + 1,
    sql: e.sql,
    rowCount: e.result.rows.length,
    pinned: e.pinned,
  }));

  // ── Notebook toolbar state (read from store, bridged by NotebookTab) ──────
  const nbKernelReady         = useNotebookToolbarStore((s) => s.kernelReady);
  const nbKernelStarting      = useNotebookToolbarStore((s) => s.kernelStarting);
  const nbKernelError         = useNotebookToolbarStore((s) => s.kernelError);
  const nbKernelPythonVersion = useNotebookToolbarStore((s) => s.kernelPythonVersion);
  const nbOnRestartKernel     = useNotebookToolbarStore((s) => s.onRestartKernel);
  const nbOnDeploy            = useNotebookToolbarStore((s) => s.onDeploy);
  const nbOnRunAll            = useNotebookToolbarStore((s) => s.onRunAll);

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

  const nbSlotProps = isNotebookTab && nbOnRestartKernel ? {
    kernelReady: nbKernelReady,
    kernelStarting: nbKernelStarting,
    kernelError: nbKernelError,
    kernelName: nbKernelPythonVersion ? `Python ${nbKernelPythonVersion}` : undefined,
    onRestartKernel: nbOnRestartKernel,
  } : null;

  const deployButton = isNotebookTab && nbOnDeploy ? (
    <Tooltip title="Deploy notebook to Snowflake">
      <Button
        className="thaw-tb-vstack-primary"
        aria-label="Deploy notebook"
        icon={<CloudUploadOutlined />}
        onClick={nbOnDeploy}
      >
        Deploy
      </Button>
    </Tooltip>
  ) : undefined;

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
        onOpenAccountParameters={openAccountParameters}
        onOpenSnowsight={openSnowsight}
        onNewSql={openScratch}
        onNewNotebook={handleNewNotebook}
        onSave={handleSave}
        contextSlot={nbSlotProps ? <NotebookToolbarSlot {...nbSlotProps} /> : undefined}
        primaryAction={deployButton}
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

      {/* Cross-tab search/replace panel */}
      {crossTabSearchOpen && (
        <Suspense fallback={null}>
          <CrossTabSearch onClose={() => setCrossTabSearchOpen(false)} />
        </Suspense>
      )}

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
              onMount={(editor) => patchMonacoClipboard(editor)}
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
          <Suspense fallback={null}>
            <NotebookTab tabId={activeTabId} />
          </Suspense>
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

      {/* Results — bottom portion */}
      {!activeDiff && !isNotebookTab &&
      <div style={{ flex: 1, overflow: "hidden", display: "flex", flexDirection: "column" }}>
        {/* Tab bar */}
        <div style={{ display: "flex", background: "var(--bg-raised)", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
          {(["results", ...(terminalOpen && featureFlags.embeddedTerminal ? ["terminal"] : []), ...(featureFlags.queryLog ? ["querylog"] : [])] as Array<"results" | "terminal" | "querylog">).map((tab) => (
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
              {tab === "results" ? "Results" : tab === "terminal" ? "Terminal" : "Query Log"}
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
                              onClick={() => { setProfileQueryId((stmtProgress.queryID || runningQueryId)!); setProfileIsLive(true); }}
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
                          onClick={() => { setProfileQueryId(runningQueryId); setProfileIsLive(true); }}
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
                            disabled: entry.id === historyId || compareHistoryId !== null,
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
                              onClick={() => { setProfileQueryId(displayedResult.queryID!); setProfileIsLive(false); }}
                            />
                          </Tooltip>
                        )}
                      </Space>
                    )}
                    <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 6 }}>
                      <Tooltip title="Search Results (⌘G)">
                        <Button
                          type="text"
                          size="small"
                          icon={<SearchOutlined style={{ fontSize: 11, color: gridSearchOpen ? "var(--accent)" : "var(--text-muted)" }} />}
                          style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                          onClick={() => setGridSearchOpen((prev) => !prev)}
                        />
                      </Tooltip>
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
                        <Tooltip title={resultHistory.length > 1 ? "Export resultsets as Excel…" : "Export as Excel"}>
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
                {/* Grid search bar */}
                {gridSearchOpen && displayedResult && (
                  <GridSearch
                    columnCount={displayedResult.columns.length}
                    onScrollToRow={(row) => primaryGridRef.current?.scrollToRow(row)}
                    onClose={() => setGridSearchOpen(false)}
                  />
                )}
                {/* Grids row — bare grids, headers at the same level */}
                <div style={{ display: "flex", flex: 1, overflow: "hidden" }}>
                  <div style={{ flex: 1, overflow: "hidden", ...(compareResult ? { borderRight: "1px solid var(--border)" } : {}) }}>
                    <ResultGrid
                      result={displayedResult}
                      gridRef={primaryGridRef}
                    />
                  </div>
                  {compareResult && compareEntry && (
                    <div style={{ flex: 1, overflow: "hidden" }}>
                      <ResultGrid
                        result={compareResult}
                      />
                    </div>
                  )}
                  {/* Cell detail side panel (hidden in compare mode — gridStore is a singleton) */}
                  {featureFlags.cellDetailPanel && featureFlags.multiCellCopy && !compareResult && (
                    <CellDetailPanel
                      columns={displayedResult.columns}
                      onVisibleCellChange={(row, col) => primaryGridRef.current?.scrollToCell(row, col)}
                    />
                  )}
                </div>{/* end grids row */}
                {/* Selection aggregations status bar (hidden in compare mode — gridStore is a singleton) */}
                {featureFlags.multiCellCopy && !compareResult && <StatusBar />}
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

          {terminalOpen && (
            <div style={{ flex: 1, overflow: "hidden", display: resultPane === "terminal" ? "flex" : "none", flexDirection: "column" }}>
              <Suspense fallback={null}>
                <TerminalPanel onClose={() => { setTerminalOpen(false); setResultPane("results"); }} />
              </Suspense>
            </div>
          )}

          {featureFlags.queryLog && (
            <div style={{ flex: 1, overflow: "hidden", display: resultPane === "querylog" ? "flex" : "none", flexDirection: "column" }}>
              <QueryLogPane />
            </div>
          )}
      </div>}

      <Suspense fallback={null}>
        {snippetsOpen && <SnippetsModal onClose={() => setSnippetsOpen(false)} />}
        {exportPathFormatOpen && <ExportPathFormatModal onClose={() => setExportPathFormatOpen(false)} />}
        {exportDdlOpen && <ExportOptionsModal onClose={() => setExportDdlOpen(false)} />}
        {excelExportOpen && (
          <ExcelExportModal
            open
            entries={excelExportEntries}
            onCancel={() => setExcelExportOpen(false)}
            onExport={exportExcelSheets}
          />
        )}
        {migrationOpen && <MigrationModal onClose={() => setMigrationOpen(false)} />}
        {dbtCreateOpen && <DbtProjectModal onClose={() => setDbtCreateOpen(false)} />}
        {fnCatalogOpen && <FunctionCatalogModal onClose={() => setFnCatalogOpen(false)} />}
        {tagMgmtOpen && <TagManagementModal onClose={closeTagMgmt} />}
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

        {accountParamsOpen && (
          <AccountParametersModal
            parameters={accountParams}
            error={accountParamsError}
            onClose={() => setAccountParamsOpen(false)}
            onParamChange={handleAccountParamChange}
          />
        )}
      </Suspense>

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
        <Suspense fallback={null}>
          <QueryProfileModal
            queryId={profileQueryId}
            onClose={() => setProfileQueryId(null)}
            liveRefresh={profileIsLive && isRunning}
          />
        </Suspense>
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
