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
import { Button, Dropdown, Space, Typography, Alert, Spin, Tag, Select, Tooltip, message } from "antd";
import { PlayCircleOutlined, StopOutlined, DisconnectOutlined, CopyOutlined, FileTextOutlined, FileExcelOutlined } from "@ant-design/icons";
import * as XLSX from "xlsx";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";
import { StartQuery, WaitForQueryResult, CancelQuery, Disconnect, SaveFile, PickSaveFile, PickSaveExportFile, SaveBinaryFile, PickOpenFile, ReadFile, GetAIConfig, GetSessionParameters, GetSessionVariables } from "../../wailsjs/go/main/App";
import type { main } from "../../wailsjs/go/models";
import SessionPropertiesModal from "../components/common/SessionPropertiesModal";
import SnippetsModal from "../components/snippets/SnippetsModal";
import ExportPathFormatModal from "../components/export/ExportPathFormatModal";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import SqlEditor from "../components/editor/SqlEditor";
import TabBar from "../components/editor/TabBar";
import { DiffEditor } from "@monaco-editor/react";
import { ensureMonacoSetup } from "../components/editor/monacoSetup";
import { useThemeStore } from "../store/themeStore";
import ResultGrid from "../components/results/ResultGrid";
import AiChat from "../components/chat/AiChat";
import TerminalPanel from "../components/terminal/TerminalPanel";
import { useQueryStore, type QueryResult, EXECUTE_IN_TAB_EVENT } from "../store/queryStore";
import { useConnectionStore } from "../store/connectionStore";
import { useSessionStore } from "../store/sessionStore";
import { usePanelLayoutStore } from "../store/panelLayoutStore";

const { Text } = Typography;

export default function QueryPage() {
  const { sql, selectedSql, isRunning, error, setResult, setRunning, setError, markSaved, openScratch, openFile, setSql } = useQueryStore();
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
  // Vertical (left/right) split state
  const vSplitResizing  = useRef(false);
  const vSplitStartX    = useRef(0);
  const vSplitStartW    = useRef(0);
  const [splitW, setSplitW] = useState(splitEditorWidth);
  const [runningQueryId, setRunningQueryId] = useState<string | null>(null);
  const [isCancelling, setIsCancelling] = useState(false);
  const [resultPane, setResultPane] = useState<"results" | "chat" | "terminal">("results");
  const [aiEnabled, setAiEnabled] = useState(false);
  const [terminalOpen, setTerminalOpen] = useState(false);

  // ── Result history (last 10 runs, most-recent-first) ──────────────────────
  interface HistoryEntry { queryID: string; sql: string; result: QueryResult; }
  const [resultHistory, setResultHistory] = useState<HistoryEntry[]>([]);
  const [historyIdx,    setHistoryIdx]    = useState<number | null>(null);

  const [snippetsOpen, setSnippetsOpen] = useState(false);
  const [exportPathFormatOpen, setExportPathFormatOpen] = useState(false);
  const [sessionPropsOpen, setSessionPropsOpen] = useState(false);
  const [sessionParams, setSessionParams] = useState<main.SessionParam[] | null>(null);
  const [sessionVars, setSessionVars] = useState<main.SessionVar[] | null>(null);
  const [sessionPropsError, setSessionPropsError] = useState<string | null>(null);
  // Ref so the async runQuery closure can detect user-initiated cancellation
  // without relying on stale React state.
  const cancelRequestedRef = useRef(false);
  const { params, disconnect } = useConnectionStore();
  const {
    role, warehouse, roles, warehouses,
    loadingContext, loadingRoles, loadingWarehouses,
    switchingRole, switchingWarehouse,
    error: sessionError,
    loadContext, loadRoles, loadWarehouses,
    switchRole, switchWarehouse, clearError,
  } = useSessionStore();

  // Sync local split state when the store value changes (e.g., after layout reset).
  useEffect(() => { setSplitPct(editorSplit); }, [editorSplit]);
  useEffect(() => { setSplitW(splitEditorWidth); }, [splitEditorWidth]);

  // Load current role/warehouse on mount
  useEffect(() => {
    loadContext();
  }, []);

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

  // Vertical drag handle mouse handlers (for split editor width).
  useEffect(() => {
    const onMove = (e: MouseEvent) => {
      if (!vSplitResizing.current) return;
      const parent = document.querySelector(".editor-area") as HTMLElement | null;
      if (!parent) return;
      const delta = e.clientX - vSplitStartX.current;
      const pct = vSplitStartW.current + delta / parent.clientWidth;
      setSplitW(Math.min(0.85, Math.max(0.15, pct)));
    };
    const onUp = () => {
      if (!vSplitResizing.current) return;
      vSplitResizing.current = false;
      document.body.style.cursor     = "";
      document.body.style.userSelect = "";
      setSplitW((w) => { setSplitEditorWidth(w); return w; });
    };
    document.addEventListener("mousemove", onMove);
    document.addEventListener("mouseup",   onUp);
    return () => {
      document.removeEventListener("mousemove", onMove);
      document.removeEventListener("mouseup",   onUp);
    };
  }, [setSplitEditorWidth]);

  const runQuery = async (overrideSql?: string) => {
    const query = overrideSql ?? (selectedSql.trim() || sql.trim());
    if (!query) return;
    cancelRequestedRef.current = false;
    setIsCancelling(false);
    setRunningQueryId(null);
    setResultPane("results");
    setRunning(true);
    try {
      // Phase 1: submit and get query ID.
      const qid = await StartQuery(query);
      // Force React to commit the query ID to the DOM synchronously, then wait
      // for a browser paint before fetching results. This guarantees the spinner
      // shows the query ID for at least one frame before the results arrive.
      flushSync(() => setRunningQueryId(qid));
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
      // Phase 2: block until results are ready.
      const res = await WaitForQueryResult();
      setResult(res);
      setResultHistory((prev) => [{ queryID: res.queryID ?? "", sql: query, result: res }, ...prev].slice(0, 10));
      setHistoryIdx(0);
    } catch (e) {
      // Suppress the error when the user explicitly cancelled — keep whatever
      // result was previously shown rather than replacing it with an error.
      if (!cancelRequestedRef.current) {
        setError(String(e));
        setHistoryIdx(null); // hide the grid; let the user re-select from history
      }
    } finally {
      setRunning(false);
      setIsCancelling(false);
    }
  };

  const handleCancel = async () => {
    if (!isRunning || isCancelling) return;
    cancelRequestedRef.current = true;
    setIsCancelling(true);
    try {
      await CancelQuery();
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

  const handleDisconnect = async () => {
    await Disconnect();
    disconnect();
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

  // Escape cancels the running query.
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === "Escape" && isRunning && !isCancelling) handleCancel();
    };
    window.addEventListener("keydown", handler);
    return () => window.removeEventListener("keydown", handler);
  }, [isRunning, isCancelling]);

  useEffect(() => {
    const handler = () => handleSave();
    window.addEventListener("save-file", handler);
    return () => window.removeEventListener("save-file", handler);
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
      setTerminalOpen(true);
      setResultPane("terminal");
    });
    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:code-snippets", () => setSnippetsOpen(true));
    return () => off();
  }, []);

  useEffect(() => {
    const off = EventsOn("menu:export-path-format", () => setExportPathFormatOpen(true));
    return () => off();
  }, []);


  const selectStyle = { fontSize: 12, minWidth: 130 };

  // The result currently shown in the grid — null when no result is selected
  // (e.g. right after a failed query; the user must pick from history explicitly).
  const displayedResult: QueryResult | null =
    historyIdx !== null ? (resultHistory[historyIdx]?.result ?? null) : null;

  const sqlSnippet = (s: string) => {
    const n = s.replace(/\s+/g, " ").trim();
    return n.length > 45 ? n.slice(0, 45) + "…" : n;
  };

  return (
    <div data-query-layout style={{ display: "flex", flexDirection: "column", height: "100%", background: "var(--bg)" }}>
      {/* Toolbar */}
      <div
        style={{
          padding: "6px 12px",
          borderBottom: "1px solid var(--border)",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          background: "var(--bg-raised)",
        }}
      >
        <Space>
          {isRunning ? (
            <Button
              danger
              icon={<StopOutlined />}
              loading={isCancelling}
              onClick={handleCancel}
              size="small"
            >
              {isCancelling ? "Cancelling…" : "Cancel"}
            </Button>
          ) : (
            <Button
              type="primary"
              icon={<PlayCircleOutlined />}
              onClick={() => runQuery()}
              size="small"
            >
              Run
            </Button>
          )}
          <Text type="secondary" style={{ fontSize: 11, whiteSpace: "nowrap" }}>
            {isRunning
              ? "Esc to cancel"
              : selectedSql.trim()
              ? "⌘↵ · running selection"
              : "⌘↵ to run"}
          </Text>
        </Space>

        <Space size={6}>
          {/* ── Role selector ─────────────────────────────────── */}
          <Tooltip title="Active role">
            <Select
              size="small"
              style={selectStyle}
              value={role || undefined}
              placeholder={loadingContext ? "…" : "Role"}
              loading={loadingRoles || switchingRole}
              showSearch
              optionFilterProp="label"
              onChange={switchRole}
              onDropdownVisibleChange={(open) => { if (open) loadRoles(); }}
              options={roles.map((r) => ({ value: r, label: r }))}
              dropdownStyle={{ minWidth: 200 }}
            />
          </Tooltip>

          {/* ── Warehouse selector ────────────────────────────── */}
          <Tooltip title="Active warehouse">
            <Select
              size="small"
              style={selectStyle}
              value={warehouse || undefined}
              placeholder={loadingContext ? "…" : "Warehouse"}
              loading={loadingWarehouses || switchingWarehouse}
              showSearch
              optionFilterProp="label"
              onChange={switchWarehouse}
              onDropdownVisibleChange={(open) => { if (open) loadWarehouses(); }}
              options={warehouses.map((w) => ({ value: w, label: w }))}
              dropdownStyle={{ minWidth: 200 }}
            />
          </Tooltip>

          {params && (
            <Dropdown
              trigger={["contextMenu"]}
              menu={{
                items: [
                  { key: "session-props", label: "Session Properties", onClick: openSessionProperties },
                ],
              }}
            >
              <Tag color="blue" style={{ fontSize: 11, margin: 0, cursor: "context-menu" }}>
                {params.account} · {params.user}
              </Tag>
            </Dropdown>
          )}
          <Button
            icon={<DisconnectOutlined />}
            size="small"
            danger
            onClick={handleDisconnect}
          >
            Disconnect
          </Button>
        </Space>
      </div>

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

      {/* SQL Editor — top portion (resizable) */}
      {!activeDiff && (
        <div
          className="editor-area"
          style={{ flex: `0 0 ${splitPct * 100}%`, borderBottom: "1px solid var(--border)", overflow: "hidden", display: "flex" }}
        >
          {/* Primary editor */}
          <div style={{ flex: splitTabId ? `0 0 ${splitW * 100}%` : "1 1 100%", overflow: "hidden" }}>
            <SqlEditor />
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
                vSplitStartW.current  = splitW;
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
      {!activeDiff && (
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
            splitStartPct.current = splitPct;
            document.body.style.cursor     = "row-resize";
            document.body.style.userSelect = "none";
            e.preventDefault();
            const parent = (e.currentTarget as HTMLElement).closest("[data-query-layout]") as HTMLElement | null;
            const onMove = (ev: MouseEvent) => {
              if (!parent) return;
              const delta = ev.clientY - splitStartY.current;
              const pct = splitStartPct.current + delta / parent.clientHeight;
              setSplitPct(Math.min(0.85, Math.max(0.15, pct)));
            };
            const onUp = (ev: MouseEvent) => {
              splitResizing.current = false;
              document.body.style.cursor     = "";
              document.body.style.userSelect = "";
              if (parent) {
                const delta = ev.clientY - splitStartY.current;
                const pct = splitStartPct.current + delta / parent.clientHeight;
                setEditorSplit(Math.min(0.85, Math.max(0.15, pct)));
              }
              window.removeEventListener("mousemove", onMove);
              window.removeEventListener("mouseup",   onUp);
            };
            window.addEventListener("mousemove", onMove);
            window.addEventListener("mouseup",   onUp);
          }}
        />
      )}

      {/* Results / AI Chat — bottom portion */}
      {!activeDiff &&
      <div style={{ flex: 1, overflow: "hidden", display: "flex", flexDirection: "column" }}>
        {/* Tab bar */}
        <div style={{ display: "flex", background: "var(--bg-raised)", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
          {(["results", ...(aiEnabled ? ["chat"] : []), ...(terminalOpen ? ["terminal"] : [])] as Array<"results" | "chat" | "terminal">).map((tab) => (
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
                {runningQueryId && (
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
                  </Space>
                )}
              </div>
            )}

            {displayedResult ? (
              <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
                {/* Error banner inside the results section when user is viewing history after a failure */}
                {error && (
                  <Alert
                    type="error"
                    message={error}
                    showIcon
                    closable
                    style={{ margin: "8px 12px 0", flexShrink: 0 }}
                  />
                )}
                <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "3px 12px", background: "var(--bg-raised)", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
                  {/* History selector */}
                  {resultHistory.length > 1 && (
                    <Select
                      size="small"
                      value={historyIdx}
                      onChange={(v) => setHistoryIdx(v)}
                      style={{ fontSize: 11, width: 220 }}
                      popupMatchSelectWidth={false}
                      options={resultHistory.map((e, i) => ({
                        value: i,
                        label: `#${i + 1}${i === 0 ? " · " : "  "}${sqlSnippet(e.sql)}`,
                      }))}
                    />
                  )}
                  {/* Query ID for displayed result */}
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
                    </Space>
                  )}
                  <div style={{ marginLeft: "auto", display: "flex", alignItems: "center", gap: 6 }}>
                    <Tooltip title="Export as CSV">
                      <Button
                        type="text"
                        size="small"
                        icon={<FileTextOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
                        style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                        onClick={exportCSV}
                      />
                    </Tooltip>
                    <Tooltip title="Export as Excel">
                      <Button
                        type="text"
                        size="small"
                        icon={<FileExcelOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
                        style={{ height: 18, padding: "0 4px", minWidth: 0 }}
                        onClick={exportExcel}
                      />
                    </Tooltip>
                    <Text style={{ fontSize: 11, color: "var(--text-faint)" }}>
                      {displayedResult.rows.length} row{displayedResult.rows.length !== 1 ? "s" : ""}
                    </Text>
                  </div>
                </div>
                <div style={{ flex: 1, overflow: "hidden" }}>
                  <ResultGrid result={displayedResult} />
                </div>
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
                      onChange={(v: number) => setHistoryIdx(v)}
                      style={{ fontSize: 11, width: 260 }}
                      popupMatchSelectWidth={false}
                      options={resultHistory.map((e, i) => ({
                        value: i,
                        label: `#${i + 1}  ${sqlSnippet(e.sql)}`,
                      }))}
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
    </div>
  );
}
