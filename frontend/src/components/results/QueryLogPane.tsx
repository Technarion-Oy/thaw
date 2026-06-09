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
// @thaw-domain: SQL Editor & Diagnostics

import { useCallback, useEffect, useRef, useState } from "react";
import { Button, Input, Select, Table, Tag, Tooltip, Typography, message } from "antd";
import { ClearOutlined, CopyOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { EventsOn } from "../../../wailsjs/runtime/runtime";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { GetQueryLogEntries, ClearQueryLog } from "../../../wailsjs/go/app/App";

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
  tabID: string;
}

interface Props {
  onClose: () => void;
}

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
  return `[${ts}] [${e.status}] ${dur} [${e.source}] ${e.queryID || "-"}\n  ${e.sql}`;
}

export default function QueryLogPane({ onClose: _onClose }: Props) {
  const [entries, setEntries] = useState<QueryLogEntry[]>([]);
  const [sourceFilter, setSourceFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [search, setSearch] = useState("");
  const mountedRef = useRef(true);

  // Initial load.
  useEffect(() => {
    mountedRef.current = true;
    GetQueryLogEntries().then((data) => {
      if (mountedRef.current && data) setEntries(data as QueryLogEntry[]);
    }).catch(() => {});
    return () => { mountedRef.current = false; };
  }, []);

  // Live event subscriptions.
  useEffect(() => {
    const offEntry = EventsOn("querylog:entry", (entry: QueryLogEntry) => {
      if (!mountedRef.current) return;
      setEntries((prev) => [...prev, entry]);
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
    return () => { offEntry(); offUpdate(); };
  }, []);

  const handleClear = useCallback(() => {
    ClearQueryLog().then(() => setEntries([])).catch(() => {});
  }, []);

  // Client-side filtering.
  const filtered = entries.filter((e) => {
    if (sourceFilter !== "all" && e.source !== sourceFilter) return false;
    if (statusFilter !== "all" && e.status !== statusFilter) return false;
    if (search && !e.sql.toLowerCase().includes(search.toLowerCase()) && !e.queryID.toLowerCase().includes(search.toLowerCase())) return false;
    return true;
  });

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
      <div style={{ flex: 1, overflow: "auto" }}>
        <Table<QueryLogEntry>
          dataSource={filtered}
          columns={columns}
          rowKey="id"
          size="small"
          pagination={false}
          scroll={{ y: "100%" }}
          style={{ fontSize: 11 }}
          locale={{ emptyText: entries.length === 0 ? "No queries logged yet. Run a query to see entries here." : "No matching entries." }}
        />
      </div>
    </div>
  );
}
