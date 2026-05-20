// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { useState, useEffect } from "react";
import { Button, Progress, Typography, Space, Tag, Collapse, Alert, Tooltip, Checkbox } from "antd";
import {
  CloudUploadOutlined,
  DatabaseOutlined,
  CheckCircleOutlined,
  WarningOutlined,
  FolderOpenOutlined,
  CaretDownOutlined,
  CaretRightOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import {
  ExportAllDatabasesDDL,
  CancelExport,
  ListExportableDatabases,
  RevealInFinder,
  GetPlatformOS,
} from "../../../wailsjs/go/main/App";
import { useGitStore } from "../../store/gitStore";
import { useConnectionStore } from "../../store/connectionStore";
import type { ddl } from "../../../wailsjs/go/models";

type ExportResult = ddl.ExportResult;

interface ProgressEvent {
  done: number;
  total: number;
  result: ExportResult;
}

const { Text } = Typography;

// Module-level cache for platform OS (compile-time constant, fetched once).
let _platformOS: string | null = null;
const getPlatform = (): Promise<string> =>
  _platformOS
    ? Promise.resolve(_platformOS)
    : GetPlatformOS().then((os) => { _platformOS = os; return os; }).catch(() => "darwin");

function revealLabelFor(os: string): string {
  return os === "windows" ? "Show in Explorer" : os === "darwin" ? "Show in Finder" : "Show in File Manager";
}

export default function ExportPanel() {
  const { exportDir, pickExportDir } = useGitStore();
  const isConnected = useConnectionStore((s) => s.isConnected);

  const [platformOS, setPlatformOS] = useState(_platformOS ?? "darwin");
  useEffect(() => { getPlatform().then(setPlatformOS); }, []);
  const revealLabel = revealLabelFor(platformOS);

  // ── database selection ────────────────────────────────────────────────────
  const [dbs, setDbs]               = useState<string[]>([]);
  const [selected, setSelected]     = useState<Set<string>>(new Set());
  const [dbsLoading, setDbsLoading] = useState(false);

  const loadDbs = async () => {
    setDbsLoading(true);
    try {
      const list = await ListExportableDatabases();
      setDbs(list ?? []);
      setSelected(new Set(list ?? []));
    } catch {
      // not connected yet — leave list empty, export-all fallback applies
      setDbs([]);
      setSelected(new Set());
    } finally {
      setDbsLoading(false);
    }
  };

  useEffect(() => { loadDbs(); }, []);

  const allChecked  = dbs.length > 0 && selected.size === dbs.length;
  const noneChecked = selected.size === 0;

  const toggleAll = () => {
    if (allChecked) {
      setSelected(new Set());
    } else {
      setSelected(new Set(dbs));
    }
  };

  const toggleDb = (db: string) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(db)) next.delete(db);
      else next.add(db);
      return next;
    });
  };

  // ── export state ──────────────────────────────────────────────────────────
  const [running, setRunning]     = useState(false);
  const [progress, setProgress]   = useState({ done: 0, total: 0 });
  const [results, setResults]     = useState<ExportResult[]>([]);
  const [error, setError]         = useState<string | null>(null);
  const [finished, setFinished]   = useState(false);
  const [showList, setShowList]   = useState(true);

  const exportSelected = async () => {
    if (!exportDir || running) return;
    setRunning(true);
    setFinished(false);
    setResults([]);
    setError(null);
    setProgress({ done: 0, total: 0 });

    const off = EventsOn("ddl:progress", (payload: ProgressEvent) => {
      setProgress({ done: payload.done, total: payload.total });
      setResults((prev) => [...prev, payload.result]);
    });

    // Pass selected list; empty array means "export all" on the backend.
    const dbList = allChecked || noneChecked ? [] : Array.from(selected);

    try {
      await ExportAllDatabasesDDL(exportDir, dbList);
    } catch (e) {
      setError(String(e));
    } finally {
      off();
      setRunning(false);
      setFinished(true);
      window.dispatchEvent(new CustomEvent("thaw:export-complete"));
    }
  };

  const pct = progress.total > 0
    ? Math.round((progress.done / progress.total) * 100)
    : 0;

  const totalFiles   = results.reduce((s, r) => s + r.files, 0);
  const totalSkipped = results.reduce((s, r) => s + r.skipped, 0);
  const hasErrors    = results.some((r) => (r.errors?.length ?? 0) > 0);

  const exportLabel = (() => {
    if (running) return `Exporting… (${progress.done}/${progress.total})`;
    if (dbs.length === 0 || allChecked || noneChecked) return "Export All Databases";
    return `Export ${selected.size} of ${dbs.length} Databases`;
  })();

  return (
    <div style={{ padding: "10px 12px", fontSize: 12 }}>
      <Text
        type="secondary"
        style={{
          fontSize: 11,
          textTransform: "uppercase",
          letterSpacing: "0.08em",
          display: "block",
          marginBottom: 8,
        }}
      >
        Export DDL
      </Text>

      {/* Directory picker row */}
      <div style={{ display: "flex", gap: 4, alignItems: "center", marginBottom: 8 }}>
        <Text
          style={{
            flex: 1, fontSize: 11, fontFamily: "monospace",
            color: exportDir ? "var(--text)" : "var(--text-muted)",
            overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
          }}
          title={exportDir}
        >
          {exportDir || "No directory selected"}
        </Text>
        <Tooltip title="Change directory">
          <Button size="small" icon={<FolderOpenOutlined />} onClick={pickExportDir} disabled={running} />
        </Tooltip>
      </div>

      {/* Database selection */}
      {dbs.length > 0 && (
        <div style={{ marginBottom: 8 }}>
          <div style={{ display: "flex", alignItems: "center", marginBottom: 4 }}>
            <Checkbox
              checked={allChecked}
              indeterminate={!allChecked && !noneChecked}
              onChange={toggleAll}
              disabled={running}
              style={{ fontSize: 11 }}
            >
              <Text type="secondary" style={{ fontSize: 11 }}>Databases</Text>
            </Checkbox>
            <div style={{ flex: 1 }} />
            <Tooltip title="Refresh database list">
              <Button
                type="text"
                size="small"
                icon={<ReloadOutlined style={{ fontSize: 10 }} />}
                onClick={loadDbs}
                loading={dbsLoading}
                disabled={running}
                style={{ color: "var(--text-muted)", padding: "0 4px", height: 18 }}
              />
            </Tooltip>
          </div>
          <div
            style={{
              maxHeight: 130,
              overflowY: "auto",
              border: "1px solid var(--border)",
              borderRadius: 4,
              padding: "2px 0",
            }}
          >
            {dbs.map((db) => (
              <div
                key={db}
                style={{
                  display: "flex",
                  alignItems: "center",
                  padding: "2px 8px",
                  cursor: running ? "default" : "pointer",
                  userSelect: "none",
                }}
                onClick={() => !running && toggleDb(db)}
              >
                <Checkbox
                  checked={selected.has(db)}
                  disabled={running}
                  onChange={() => toggleDb(db)}
                  style={{ fontSize: 11 }}
                  onClick={(e) => e.stopPropagation()}
                />
                <DatabaseOutlined style={{ fontSize: 10, marginLeft: 6, marginRight: 4, opacity: 0.5 }} />
                <Text style={{ fontSize: 11 }} ellipsis title={db}>{db}</Text>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Export / Cancel buttons */}
      <div style={{ display: "flex", gap: 4, marginBottom: 8 }}>
        <Tooltip title={!isConnected ? "Connect to Snowflake to export" : undefined}>
          <Button
            size="small"
            type="primary"
            icon={<CloudUploadOutlined />}
            disabled={!isConnected || !exportDir || running || (dbs.length > 0 && noneChecked)}
            loading={running}
            onClick={exportSelected}
            style={{ flex: 1 }}
          >
            {exportLabel}
          </Button>
        </Tooltip>
        {running && (
          <Button size="small" danger onClick={() => CancelExport()}>
            Cancel
          </Button>
        )}
      </div>

      {/* Progress bar */}
      {running && progress.total > 0 && (
        <Progress
          percent={pct}
          size="small"
          style={{ marginBottom: 8 }}
          format={() => `${progress.done} / ${progress.total}`}
        />
      )}

      {/* Error */}
      {error && (
        <Alert
          type="error"
          message={error}
          showIcon
          style={{ fontSize: 11, marginBottom: 8 }}
        />
      )}

      {/* Summary */}
      {finished && results.length > 0 && (
        <div>
          <div style={{ display: "flex", alignItems: "center", marginBottom: 6 }}>
            <Space size={4} style={{ flex: 1, flexWrap: "wrap" }}>
              <Tag
                icon={<CheckCircleOutlined />}
                color="green"
                style={{ fontSize: 10, margin: 0 }}
              >
                {totalFiles} files
              </Tag>
              {totalSkipped > 0 && (
                <Tag style={{ fontSize: 10, margin: 0 }}>{totalSkipped} skipped</Tag>
              )}
              {hasErrors && (
                <Tag
                  icon={<WarningOutlined />}
                  color="red"
                  style={{ fontSize: 10, margin: 0 }}
                >
                  errors
                </Tag>
              )}
            </Space>
            <Tooltip title={revealLabel}>
              <Button
                type="text"
                size="small"
                icon={<FolderOpenOutlined style={{ fontSize: 11 }} />}
                onClick={() => exportDir && RevealInFinder(exportDir)}
                style={{ color: "var(--text-muted)", padding: "0 4px" }}
              />
            </Tooltip>
            <Button
              type="text"
              size="small"
              icon={showList ? <CaretDownOutlined /> : <CaretRightOutlined />}
              onClick={() => setShowList((v) => !v)}
              style={{ fontSize: 10, color: "var(--text-muted)", padding: "0 4px" }}
            />
          </div>

          {showList && (
            <Collapse
              size="small"
              ghost
              style={{ fontSize: 11 }}
              items={results.map((r) => ({
                key: r.database,
                label: (
                  <Space size={4}>
                    <DatabaseOutlined style={{ fontSize: 11 }} />
                    <span style={{ fontSize: 11 }}>{r.database}</span>
                    <Tag style={{ fontSize: 10, margin: 0 }}>{r.files} files</Tag>
                    {(r.errors?.length ?? 0) > 0 && (
                      <Tag color="red" style={{ fontSize: 10, margin: 0 }}>
                        {r.errors!.length} err
                      </Tag>
                    )}
                  </Space>
                ),
                children:
                  (r.errors?.length ?? 0) > 0 ? (
                    <div style={{ fontSize: 11, color: "#f85149" }}>
                      {r.errors!.map((e, i) => (
                        <div key={i}>{e}</div>
                      ))}
                    </div>
                  ) : (
                    <div style={{ fontSize: 11, color: "#3fb950" }}>
                      {r.files} files written, {r.skipped} skipped
                    </div>
                  ),
              }))}
            />
          )}
        </div>
      )}
    </div>
  );
}
