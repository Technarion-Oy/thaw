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
import { Button, Space, Typography, Alert, Spin, Tag, Select, Tooltip, message } from "antd";
import { PlayCircleOutlined, StopOutlined, DisconnectOutlined, CopyOutlined, FileTextOutlined, FileExcelOutlined } from "@ant-design/icons";
import * as XLSX from "xlsx";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";
import { StartQuery, WaitForQueryResult, CancelQuery, Disconnect, SaveFile, PickSaveFile, PickSaveExportFile, SaveBinaryFile, PickOpenFile, ReadFile, GetAIConfig } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import SqlEditor from "../components/editor/SqlEditor";
import TabBar from "../components/editor/TabBar";
import ResultGrid from "../components/results/ResultGrid";
import AiChat from "../components/chat/AiChat";
import { useQueryStore } from "../store/queryStore";
import { useConnectionStore } from "../store/connectionStore";
import { useSessionStore } from "../store/sessionStore";

const { Text } = Typography;

export default function QueryPage() {
  const { sql, selectedSql, result, isRunning, error, setResult, setRunning, setError, markSaved, openScratch, openFile } = useQueryStore();
  const [runningQueryId, setRunningQueryId] = useState<string | null>(null);
  const [isCancelling, setIsCancelling] = useState(false);
  const [resultPane, setResultPane] = useState<"results" | "chat">("results");
  const [aiEnabled, setAiEnabled] = useState(false);
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

  const runQuery = async () => {
    const query = selectedSql.trim() || sql.trim();
    if (!query) return;
    cancelRequestedRef.current = false;
    setIsCancelling(false);
    setRunningQueryId(null);
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
    } catch (e) {
      // Suppress the error when the user explicitly cancelled — keep whatever
      // result was previously shown rather than replacing it with an error.
      if (!cancelRequestedRef.current) {
        setError(String(e));
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

  const handleDisconnect = async () => {
    await Disconnect();
    disconnect();
  };

  const exportCSV = async () => {
    if (!result) return;
    const escape = (v: unknown) => {
      const s = v === null || v === undefined ? "" : String(v);
      return s.includes(",") || s.includes('"') || s.includes("\n")
        ? `"${s.replace(/"/g, '""')}"`
        : s;
    };
    const csv =
      result.columns.map(escape).join(",") +
      "\n" +
      result.rows.map((r) => r.map(escape).join(",")).join("\n");
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
    if (!result) return;
    const ws = XLSX.utils.aoa_to_sheet([result.columns, ...result.rows]);
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


  const selectStyle = { fontSize: 12, minWidth: 130 };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", background: "var(--bg)" }}>
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
              onClick={runQuery}
              size="small"
            >
              Run
            </Button>
          )}
          <Text type="secondary" style={{ fontSize: 11 }}>
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
            <Tag color="blue" style={{ fontSize: 11, margin: 0 }}>
              {params.account} · {params.user}
            </Tag>
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

      {/* SQL Editor — top half */}
      <div style={{ flex: "0 0 40%", borderBottom: "1px solid var(--border)" }}>
        <SqlEditor />
      </div>

      {/* Results / AI Chat — bottom half */}
      <div style={{ flex: 1, overflow: "hidden", display: "flex", flexDirection: "column" }}>
        {/* Tab bar */}
        <div style={{ display: "flex", background: "var(--bg-raised)", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
          {(["results", ...(aiEnabled ? ["chat"] : [])] as Array<"results" | "chat">).map((tab) => (
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
              {tab === "results" ? "Results" : "AI Chat"}
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

            {error && (
              <Alert
                type="error"
                message={error}
                showIcon
                closable
                style={{ margin: 12 }}
              />
            )}

            {result && !error && (
              <div style={{ display: "flex", flexDirection: "column", height: "100%" }}>
                <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "3px 12px", background: "var(--bg-raised)", borderBottom: "1px solid var(--border)", flexShrink: 0 }}>
                  {result.queryID && (
                    <Space size={4}>
                      <Text style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}>
                        {result.queryID}
                      </Text>
                      <Button
                        type="text"
                        size="small"
                        icon={<CopyOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
                        style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                        onClick={async () => { await ClipboardSetText(result.queryID!); message.success("Query ID copied"); }}
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
                      {result.rows.length} row{result.rows.length !== 1 ? "s" : ""}
                    </Text>
                  </div>
                </div>
                <div style={{ flex: 1, overflow: "hidden" }}>
                  <ResultGrid result={result} />
                </div>
              </div>
            )}

            {!result && !error && !isRunning && (
              <div style={{ padding: 24, color: "var(--text-faint)", fontSize: 13 }}>
                Run a query to see results here.
              </div>
            )}
          </div>

          <div style={{ flex: 1, overflow: "hidden", display: resultPane === "chat" ? "flex" : "none", flexDirection: "column" }}>
            <AiChat />
          </div>
      </div>
    </div>
  );
}
