// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Button, Input, Select, Table, Tag, Tooltip, Typography, message } from "antd";
import { ClearOutlined, CopyOutlined, DownloadOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { EventsOn, ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { GetQueryLogEntries, ClearQueryLog, IsQueryLogEnabled, SetQueryLogEnabled, PickQueryLogExportFile, SaveFile } from "../../../wailsjs/go/app/App";

const { Text } = Typography;

interface QueryLogEntry {
  id: number;
  timestamp: string;
  sql: string;
  queryID: string;
  status: "RUNNING" | "SUCCESS" | "FAIL" | "CANCELED";
  durationMs: number;
  error: string;
  source: "user" | "internal";
  feature: string;
  tabID: string;
}

const MAX_FRONTEND_ENTRIES = 5000;

const statusColors: Record<string, string> = {
  SUCCESS: "green",
  FAIL: "red",
  CANCELED: "orange",
  RUNNING: "blue",
};

const sourceColors: Record<string, string> = {
  user: "blue",
  internal: "default",
};

function formatTime(ts: string): string {
  try {
    const d = new Date(ts);
    const hh = String(d.getHours()).padStart(2, "0");
    const mm = String(d.getMinutes()).padStart(2, "0");
    const ss = String(d.getSeconds()).padStart(2, "0");
    const ms = String(d.getMilliseconds()).padStart(3, "0");
    return `${hh}:${mm}:${ss}.${ms}`;
  } catch {
    return ts;
  }
}

function formatEntryForCopy(e: QueryLogEntry): string {
  const ts = formatTime(e.timestamp);
  const dur = e.durationMs > 0 ? `${e.durationMs}ms` : "-";
  const feature = e.feature ? ` [${e.feature}]` : "";
  return `[${ts}] [${e.status}] ${dur} [${e.source}]${feature} ${e.queryID || "-"}\n  ${e.sql}`;
}

const columns: ColumnsType<QueryLogEntry> = [
  {
    title: "Time",
    dataIndex: "timestamp",
    key: "timestamp",
    width: 100,
    render: (ts: string) => (
      <Text style={{ fontFamily: "monospace", fontSize: 11 }}>{formatTime(ts)}</Text>
    ),
  },
  {
    title: "SQL",
    dataIndex: "sql",
    key: "sql",
    ellipsis: true,
    render: (sql: string) => (
      <Tooltip title={sql} placement="topLeft">
        <Text style={{ fontFamily: "monospace", fontSize: 11 }}>
          {sql.length > 120 ? sql.slice(0, 120) + "..." : sql}
        </Text>
      </Tooltip>
    ),
  },
  {
    title: "Source",
    dataIndex: "source",
    key: "source",
    width: 80,
    render: (src: string) => <Tag color={sourceColors[src] ?? "default"} style={{ fontSize: 10 }}>{src}</Tag>,
  },
  {
    title: "Feature",
    dataIndex: "feature",
    key: "feature",
    width: 120,
    render: (feature: string) =>
      feature ? (
        <Tag color="geekblue" style={{ fontSize: 10 }}>{feature}</Tag>
      ) : (
        <Text style={{ fontSize: 11, color: "var(--text-faint)" }}>-</Text>
      ),
  },
  {
    title: "Status",
    dataIndex: "status",
    key: "status",
    width: 90,
    render: (status: string) => <Tag color={statusColors[status] ?? "default"} style={{ fontSize: 10 }}>{status}</Tag>,
  },
  {
    title: "Duration",
    dataIndex: "durationMs",
    key: "durationMs",
    width: 80,
    align: "right",
    render: (ms: number, record: QueryLogEntry) =>
      record.status === "RUNNING" ? (
        <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>...</Text>
      ) : (
        <Text style={{ fontFamily: "monospace", fontSize: 11 }}>{ms > 0 ? `${ms}ms` : "-"}</Text>
      ),
  },
  {
    title: "Query ID",
    dataIndex: "queryID",
    key: "queryID",
    width: 160,
    render: (qid: string) =>
      qid ? (
        <span style={{ display: "flex", alignItems: "center", gap: 4 }}>
          <Text style={{ fontFamily: "monospace", fontSize: 10 }}>{qid}</Text>
          <Button
            type="text"
            size="small"
            icon={<CopyOutlined style={{ fontSize: 10, color: "var(--text-muted)" }} />}
            style={{ height: 16, padding: "0 2px", minWidth: 0 }}
            onClick={() => ClipboardSetText(qid).then(() => message.success("Query ID copied"))}
          />
        </span>
      ) : (
        <Text style={{ fontSize: 11, color: "var(--text-faint)" }}>-</Text>
      ),
  },
  {
    title: "",
    key: "actions",
    width: 32,
    render: (_: unknown, record: QueryLogEntry) => (
      <Tooltip title="Copy entry">
        <Button
          type="text"
          size="small"
          icon={<CopyOutlined style={{ fontSize: 11, color: "var(--text-muted)" }} />}
          style={{ height: 20, padding: "0 4px", minWidth: 0 }}
          onClick={() =>
            ClipboardSetText(formatEntryForCopy(record)).then(() => message.success("Entry copied"))
          }
        />
      </Tooltip>
    ),
  },
];

export default function QueryLogPane() {
  const [entries, setEntries] = useState<QueryLogEntry[]>([]);
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [featureFilter, setFeatureFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [search, setSearch] = useState("");
  const mountedRef = useRef(true);
  const tableContainerRef = useRef<HTMLDivElement>(null);
  const [tableHeight, setTableHeight] = useState<number>(300);

  // Intentionally always enable backend logging on mount (and never disable on
  // unmount). The log is session-scoped and lightweight — keeping it running in
  // the background ensures entries are captured even when the pane isn't visible,
  // so opening it later still shows the full session history.
  useEffect(() => {
    mountedRef.current = true;
    IsQueryLogEnabled().then((enabled) => {
      if (!enabled) SetQueryLogEnabled(true);
    }).catch(() => {});
    GetQueryLogEntries().then((data) => {
      if (mountedRef.current && data) setEntries(data as QueryLogEntry[]);
    }).catch(() => {});
    return () => { mountedRef.current = false; };
  }, []);

  // Live event subscriptions.
  useEffect(() => {
    const offEntry = EventsOn("querylog:entry", (entry: QueryLogEntry) => {
      if (!mountedRef.current) return;
      setEntries((prev) => {
        const next = [...prev, entry];
        return next.length > MAX_FRONTEND_ENTRIES ? next.slice(next.length - MAX_FRONTEND_ENTRIES) : next;
      });
    });
    const offUpdate = EventsOn("querylog:update", (update: {
      id: number; status: string; durationMs: number; error?: string; queryID?: string;
    }) => {
      if (!mountedRef.current) return;
      setEntries((prev) =>
        prev.map((e) =>
          e.id === update.id
            ? {
                ...e,
                status: update.status as QueryLogEntry["status"],
                durationMs: update.durationMs,
                error: update.error ?? e.error,
                queryID: update.queryID || e.queryID,
              }
            : e,
        ),
      );
    });
    const offFilter = EventsOn("menu:query-log-filter", (filter: string) => {
      if (!mountedRef.current) return;
      setSourceFilter(filter);
    });
    const offCleared = EventsOn("querylog:cleared", () => {
      if (!mountedRef.current) return;
      setEntries([]);
    });
    return () => { offEntry(); offUpdate(); offFilter(); offCleared(); };
  }, []);

  // Measure the table container so Ant Table gets a concrete pixel scroll height.
  useEffect(() => {
    const el = tableContainerRef.current;
    if (!el) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        // Subtract Ant Table header height (~39px for size="small").
        const h = Math.max(entry.contentRect.height - 39, 100);
        setTableHeight(h);
      }
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, []);

  const handleClear = useCallback(() => {
    ClearQueryLog().then(() => setEntries([])).catch(() => {});
  }, []);

  // Client-side filtering (memoized to avoid re-filtering on every render).
  const filtered = useMemo(() =>
    entries.filter((e) => {
      if (sourceFilter !== "all" && e.source !== sourceFilter) return false;
      if (featureFilter !== "all" && e.feature !== featureFilter) return false;
      if (statusFilter !== "all" && e.status !== statusFilter) return false;
      if (search && !e.sql.toLowerCase().includes(search.toLowerCase()) && !e.queryID.toLowerCase().includes(search.toLowerCase())) return false;
      return true;
    }),
    [entries, sourceFilter, featureFilter, statusFilter, search],
  );

  // Feature filter options are derived from the features actually present in the
  // log so the dropdown only ever offers meaningful choices.
  const featureOptions = useMemo(() => {
    const seen = new Set<string>();
    for (const e of entries) if (e.feature) seen.add(e.feature);
    return [
      { value: "all", label: "All Features" },
      ...Array.from(seen).sort().map((f) => ({ value: f, label: f })),
    ];
  }, [entries]);

  const handleExport = async () => {
    if (filtered.length === 0) {
      message.info("No entries to export");
      return;
    }
    try {
      const path = await PickQueryLogExportFile("thaw-query-log.log");
      if (!path) return;
      const header = `# Thaw Query Log — exported ${new Date().toISOString()}\n# ${filtered.length} entries\n\n`;
      const body = filtered.map((e) => formatEntryForCopy(e)).join("\n\n");
      await SaveFile(path, header + body + "\n");
      message.success("Query log exported");
    } catch (err) {
      message.error(String(err));
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", height: "100%", overflow: "hidden" }}>
      {/* Toolbar */}
      <div
        style={{
          display: "flex",
          alignItems: "center",
          gap: 8,
          padding: "4px 12px",
          background: "var(--bg-raised)",
          borderBottom: "1px solid var(--border)",
          flexShrink: 0,
        }}
      >
        <Select
          size="small"
          value={sourceFilter}
          onChange={setSourceFilter}
          style={{ width: 110, fontSize: 11 }}
          options={[
            { value: "all", label: "All Sources" },
            { value: "user", label: "User" },
            { value: "internal", label: "Internal" },
          ]}
        />
        <Select
          size="small"
          value={featureFilter}
          onChange={setFeatureFilter}
          style={{ width: 140, fontSize: 11 }}
          options={featureOptions}
        />
        <Select
          size="small"
          value={statusFilter}
          onChange={setStatusFilter}
          style={{ width: 110, fontSize: 11 }}
          options={[
            { value: "all", label: "All Statuses" },
            { value: "SUCCESS", label: "Success" },
            { value: "FAIL", label: "Failed" },
            { value: "CANCELED", label: "Canceled" },
            { value: "RUNNING", label: "Running" },
          ]}
        />
        <Input
          size="small"
          placeholder="Search SQL or Query ID..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          allowClear
          style={{ width: 200, fontSize: 11 }}
        />
        <Tooltip title="Export log to file">
          <Button
            size="small"
            icon={<DownloadOutlined style={{ fontSize: 11 }} />}
            onClick={handleExport}
            disabled={filtered.length === 0}
          />
        </Tooltip>
        <Tooltip title="Clear all entries">
          <Button
            size="small"
            icon={<ClearOutlined style={{ fontSize: 11 }} />}
            onClick={handleClear}
          />
        </Tooltip>
        <div style={{ marginLeft: "auto" }}>
          <Text style={{ fontSize: 11, color: "var(--text-faint)" }}>
            {filtered.length} entr{filtered.length === 1 ? "y" : "ies"}
          </Text>
        </div>
      </div>

      {/* Table */}
      <div ref={tableContainerRef} style={{ flex: 1, overflow: "hidden" }}>
        <Table<QueryLogEntry>
          dataSource={filtered}
          columns={columns}
          rowKey="id"
          size="small"
          pagination={false}
          scroll={{ y: tableHeight }}
          style={{ fontSize: 11 }}
          locale={{ emptyText: entries.length === 0 ? "No queries logged yet. Run a query to see entries here." : "No matching entries." }}
        />
      </div>
    </div>
  );
}
