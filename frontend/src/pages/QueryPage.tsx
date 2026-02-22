// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect } from "react";
import { Button, Space, Typography, Alert, Spin, Tag, Select, Tooltip, message } from "antd";
import { PlayCircleOutlined, DisconnectOutlined, SaveOutlined } from "@ant-design/icons";
import { ExecuteQuery, Disconnect, SaveFile, PickSaveFile } from "../../wailsjs/go/main/App";
import SqlEditor from "../components/editor/SqlEditor";
import TabBar from "../components/editor/TabBar";
import ResultGrid from "../components/results/ResultGrid";
import { useQueryStore } from "../store/queryStore";
import { useConnectionStore } from "../store/connectionStore";
import { useSessionStore } from "../store/sessionStore";

const { Text } = Typography;

export default function QueryPage() {
  const { sql, selectedSql, result, isRunning, error, setResult, setRunning, setError, markSaved } = useQueryStore();
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
    setRunning(true);
    try {
      const res = await ExecuteQuery(query);
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

  const handleSave = async () => {
    const { tabs, activeTabId, sql: currentSql } = useQueryStore.getState();
    const tab = tabs.find((t) => t.id === activeTabId);
    if (!tab) return;

    let savePath = tab.path;
    let saveTitle = tab.title;

    if (!savePath) {
      // No path yet — ask user where to save
      savePath = await PickSaveFile(tab.title === "SQL" ? "untitled.sql" : tab.title);
      if (!savePath) return; // cancelled
      saveTitle = savePath.split("/").pop() ?? savePath;
    }

    try {
      await SaveFile(savePath, currentSql);
      markSaved(activeTabId, savePath, saveTitle);
    } catch (e) {
      message.error(`Save failed: ${String(e)}`);
    }
  };

  // Listen for Cmd+Enter / Cmd+S from the editor
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
          <div style={{ position: "absolute", inset: 0, display: "flex", alignItems: "center", justifyContent: "center", zIndex: 10, background: "rgba(0,0,0,0.4)" }}>
            <Spin size="large" />
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

        {result && !error && <ResultGrid result={result} />}

        {!result && !error && !isRunning && (
          <div style={{ padding: 24, color: "#484f58", fontSize: 13 }}>
            Run a query to see results here.
          </div>
        )}
      </div>
    </div>
  );
}
