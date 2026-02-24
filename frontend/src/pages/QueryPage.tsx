// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import { flushSync } from "react-dom";
import { Button, Space, Typography, Alert, Spin, Tag, Select, Tooltip, message } from "antd";
import { PlayCircleOutlined, DisconnectOutlined, SaveOutlined, CopyOutlined } from "@ant-design/icons";
import { ClipboardSetText } from "../../wailsjs/runtime/runtime";
import { StartQuery, WaitForQueryResult, Disconnect, SaveFile, PickSaveFile } from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import SqlEditor from "../components/editor/SqlEditor";
import TabBar from "../components/editor/TabBar";
import ResultGrid from "../components/results/ResultGrid";
import { useQueryStore } from "../store/queryStore";
import { useConnectionStore } from "../store/connectionStore";
import { useSessionStore } from "../store/sessionStore";

const { Text } = Typography;

export default function QueryPage() {
  const { sql, selectedSql, result, isRunning, error, setResult, setRunning, setError, markSaved, openScratch } = useQueryStore();
  const [runningQueryId, setRunningQueryId] = useState<string | null>(null);
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

  const runQuery = async () => {
    const query = selectedSql.trim() || sql.trim();
    if (!query) return;
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
      setError(String(e));
    } finally {
      setRunning(false);
    }
  };

  const handleDisconnect = async () => {
    await Disconnect();
    disconnect();
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

  // Browser events — dispatched by Monaco keyboard bindings and the Save button.
  useEffect(() => {
    const handler = () => runQuery();
    window.addEventListener("run-query", handler);
    return () => window.removeEventListener("run-query", handler);
  });

  useEffect(() => {
    const handler = () => handleSave();
    window.addEventListener("save-file", handler);
    return () => window.removeEventListener("save-file", handler);
  });

  // Wails events — dispatched by the native application menu.
  useEffect(() => {
    const offNewTab  = EventsOn("menu:new-tab",  () => openScratch());
    const offSave    = EventsOn("menu:save",     () => handleSave());
    const offSaveAs  = EventsOn("menu:save-as",  () => handleSaveAs());
    return () => { offNewTab(); offSave(); offSaveAs(); };
  }, []);


  const selectStyle = { fontSize: 12, minWidth: 130 };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", background: "#0d1117" }}>
      {/* Toolbar */}
      <div
        style={{
          padding: "6px 12px",
          borderBottom: "1px solid #30363d",
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          background: "#161b22",
        }}
      >
        <Space>
          <Button
            type="primary"
            icon={<PlayCircleOutlined />}
            loading={isRunning}
            onClick={runQuery}
            size="small"
          >
            Run
          </Button>
          <Button
            icon={<SaveOutlined />}
            onClick={handleSave}
            size="small"
          >
            Save
          </Button>
          <Button
            icon={<SaveOutlined />}
            onClick={handleSaveAs}
            size="small"
          >
            Save As…
          </Button>
          <Text type="secondary" style={{ fontSize: 11 }}>
            {selectedSql.trim() ? "⌘↵ · running selection" : "⌘↵ to run"}
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
      <div style={{ flex: "0 0 40%", borderBottom: "1px solid #30363d" }}>
        <SqlEditor />
      </div>

      {/* Results — bottom half */}
      <div style={{ flex: 1, overflow: "hidden", position: "relative" }}>
        {isRunning && (
          <div style={{ position: "absolute", inset: 0, display: "flex", flexDirection: "column", alignItems: "center", justifyContent: "center", gap: 12, zIndex: 10, background: "rgba(0,0,0,0.4)" }}>
            <Spin size="large" />
            {runningQueryId && (
              <Space size={4}>
                <Text style={{ fontFamily: "monospace", fontSize: 11, color: "#8b949e" }}>
                  {runningQueryId}
                </Text>
                <Button
                  type="text"
                  size="small"
                  icon={<CopyOutlined style={{ fontSize: 10, color: "#8b949e" }} />}
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
            <div style={{ display: "flex", alignItems: "center", gap: 12, padding: "3px 12px", background: "#161b22", borderBottom: "1px solid #30363d", flexShrink: 0 }}>
              {result.queryID && (
                <Space size={4}>
                  <Text style={{ fontFamily: "monospace", fontSize: 11, color: "#8b949e" }}>
                    {result.queryID}
                  </Text>
                  <Button
                    type="text"
                    size="small"
                    icon={<CopyOutlined style={{ fontSize: 10, color: "#8b949e" }} />}
                    style={{ height: 16, padding: "0 2px", minWidth: 0 }}
                    onClick={async () => { await ClipboardSetText(result.queryID!); message.success("Query ID copied"); }}
                  />
                </Space>
              )}
              <Text style={{ fontSize: 11, color: "#484f58" }}>
                {result.rows.length} row{result.rows.length !== 1 ? "s" : ""}
              </Text>
            </div>
            <div style={{ flex: 1, overflow: "hidden" }}>
              <ResultGrid result={result} />
            </div>
          </div>
        )}

        {!result && !error && !isRunning && (
          <div style={{ padding: 24, color: "#484f58", fontSize: 13 }}>
            Run a query to see results here.
          </div>
        )}
      </div>
    </div>
  );
}
