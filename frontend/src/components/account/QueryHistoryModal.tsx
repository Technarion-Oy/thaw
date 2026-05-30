// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef } from "react";
import {
  Modal,
  Select,
  AutoComplete,
  DatePicker,
  InputNumber,
  Checkbox,
  Button,
  Table,
  Tag,
  Spin,
  Alert,
  Space,
  Typography,
  Input,
} from "antd";
import { SearchOutlined, BarChartOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import type { Dayjs } from "dayjs";
import { GetQueryHistory, ListUsers } from "../../../wailsjs/go/app/App";
import QueryProfileModal from "../results/QueryProfileModal";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { useConnectionStore } from "../../store/connectionStore";
import { useSessionStore } from "../../store/sessionStore";
import type { queryhistory } from "../../../wailsjs/go/models";

const { Text } = Typography;
const { RangePicker } = DatePicker;

interface Props {
  onClose: () => void;
}

type FilterType = "session" | "user" | "warehouse" | "all";

export default function QueryHistoryModal({ onClose }: Props) {
  const defaultUser      = useConnectionStore((s) => s.params?.user ?? "");
  const defaultWarehouse = useSessionStore((s) => s.warehouse);
  const warehouses       = useSessionStore((s) => s.warehouses);
  const loadWarehouses   = useSessionStore((s) => s.loadWarehouses);

  const [filterType,      setFilterType]      = useState<FilterType>("session");
  const [sessionId] = useState("");
  const [userName,        setUserName]        = useState(defaultUser);
  const [warehouseName,   setWarehouseName]   = useState(defaultWarehouse);
  const [timeRange,       setTimeRange]       = useState<[Dayjs, Dayjs] | null>(null);
  const [resultLimit,     setResultLimit]     = useState(100);
  const [includeClientGen, setIncludeClientGen] = useState(false);
  const [rows,            setRows]            = useState<queryhistory.QueryHistoryRow[] | null>(null);
  const [loading,         setLoading]         = useState(false);
  const [error,           setError]           = useState<string | null>(null);
  const [querySearch,     setQuerySearch]     = useState("");
  const [userList,        setUserList]        = useState<string[]>([]);
  const [copiedId,        setCopiedId]        = useState<string | null>(null);
  const [profileQueryId,  setProfileQueryId]  = useState<string | null>(null);
  const copyTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const runQuery = async () => {
    setLoading(true);
    setError(null);
    setRows(null);
    try {
      const start = timeRange ? timeRange[0].toISOString() : "";
      const end   = timeRange ? timeRange[1].toISOString() : "";
      const data = await GetQueryHistory(
        filterType,
        sessionId,
        userName,
        warehouseName,
        start,
        end,
        resultLimit,
        includeClientGen,
      );
      setRows(data ?? []);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  // Auto-run on mount with current session defaults
  // eslint-disable-next-line react-hooks/exhaustive-deps
  useEffect(() => { runQuery(); }, []);

  const loadInEditor = (sql: string) => {
    window.dispatchEvent(new CustomEvent("load-query", { detail: { sql } }));
    onClose();
  };

  const statusColor = (status: string) => {
    if (status === "SUCCESS") return "green";
    if (status === "FAIL" || status === "FAILED") return "red";
    return "blue";
  };

  const formatDuration = (ms: number) => {
    if (ms < 1000) return `${ms}ms`;
    return `${(ms / 1000).toFixed(1)}s`;
  };

  // Highlight all occurrences of `term` in `text` with a <mark> span.
  const highlight = (text: string, term: string) => {
    if (!term) return <>{text}</>;
    const parts = text.split(new RegExp(`(${term.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")})`, "gi"));
    return (
      <>
        {parts.map((part, i) =>
          part.toLowerCase() === term.toLowerCase()
            ? <mark key={i} style={{ background: "var(--accent)", color: "var(--bg)", padding: 0, borderRadius: 2 }}>{part}</mark>
            : part
        )}
      </>
    );
  };

  const visibleRows = rows
    ? (querySearch.trim()
        ? rows.filter((r) => r.queryText.toLowerCase().includes(querySearch.toLowerCase()))
        : rows)
    : null;

  const columns: ColumnsType<queryhistory.QueryHistoryRow> = [
    {
      key: "status",
      title: "Status",
      dataIndex: "status",
      width: 90,
      render: (v: string) => <Tag color={statusColor(v)}>{v || "—"}</Tag>,
    },
    {
      key: "queryType",
      title: "Type",
      dataIndex: "queryType",
      width: 110,
    },
    {
      key: "queryText",
      title: "Query",
      dataIndex: "queryText",
      ellipsis: true,
      render: (v: string) => {
        const preview = v ? (v.length > 80 ? v.slice(0, 80) + "…" : v) : "—";
        return <span style={{ fontFamily: "monospace", fontSize: 11 }}>{highlight(preview, querySearch)}</span>;
      },
    },
    {
      key: "startTime",
      title: "Start",
      dataIndex: "startTime",
      width: 140,
      render: (v: string) => v ? dayjs(v).format("HH:mm:ss DD MMM") : "—",
    },
    {
      key: "endTime",
      title: "End",
      dataIndex: "endTime",
      width: 140,
      render: (v: string) => v ? dayjs(v).format("HH:mm:ss DD MMM") : "—",
    },
    {
      key: "elapsedMs",
      title: "Duration",
      dataIndex: "elapsedMs",
      width: 80,
      render: (v: number) => formatDuration(v),
    },
  ];

  return (
    <>
    <Modal
      open
      title="Query Activity"
      onCancel={onClose}
      width="min(1300px, 92vw)"
      footer={null}
    >
      {/* Filter form */}
      <div style={{ display: "flex", flexWrap: "wrap", gap: 8, alignItems: "flex-end", marginBottom: 12 }}>
        <div>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Scope</div>
          <Select
            size="small"
            value={filterType}
            onChange={(v) => setFilterType(v)}
            style={{ width: 160 }}
            options={[
              { value: "session",   label: "Current Session" },
              { value: "user",      label: "By User" },
              { value: "warehouse", label: "By Warehouse" },
              { value: "all",       label: "All" },
            ]}
          />
        </div>

        {filterType === "user" && (
          <div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>User name</div>
            <AutoComplete
              size="small"
              value={userName}
              onChange={setUserName}
              options={userList.map((u) => ({ value: u }))}
              filterOption={(input, option) =>
                (option?.value ?? "").toLowerCase().includes(input.toLowerCase())
              }
              onDropdownVisibleChange={(open) => {
                if (open && userList.length === 0) {
                  ListUsers().then((users) => setUserList(users.map((u) => u.name))).catch(() => {});
                }
              }}
              style={{ width: 180 }}
              placeholder="Select or type a user…"
            />
          </div>
        )}

        {filterType === "warehouse" && (
          <div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Warehouse name</div>
            <AutoComplete
              size="small"
              value={warehouseName}
              onChange={setWarehouseName}
              options={warehouses.map((w) => ({ value: w }))}
              filterOption={(input, option) =>
                (option?.value ?? "").toLowerCase().includes(input.toLowerCase())
              }
              onDropdownVisibleChange={(open) => { if (open) loadWarehouses(); }}
              style={{ width: 180 }}
              placeholder="Select or type a warehouse…"
            />
          </div>
        )}

        <div>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Time range</div>
          <RangePicker
            size="small"
            showTime
            style={{ width: 320 }}
            value={timeRange}
            onChange={(v) => setTimeRange(v as [Dayjs, Dayjs] | null)}
          />
        </div>

        <div>
          <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Limit</div>
          <InputNumber
            size="small"
            min={1}
            max={10000}
            value={resultLimit}
            onChange={(v) => setResultLimit(v ?? 100)}
            style={{ width: 80 }}
          />
        </div>

        <div style={{ paddingBottom: 2 }}>
          <Checkbox
            checked={includeClientGen}
            onChange={(e) => setIncludeClientGen(e.target.checked)}
          >
            <span style={{ fontSize: 12 }}>Include client-generated</span>
          </Checkbox>
        </div>

        <Button type="primary" size="small" onClick={runQuery} loading={loading}>
          Run
        </Button>
      </div>

      {loading && <div style={{ textAlign: "center", padding: 24 }}><Spin /></div>}
      {error && <Alert type="error" message={error} style={{ marginBottom: 8 }} />}

      {rows && (
        <>
          <Input
            size="small"
            placeholder="Filter by query text…"
            prefix={<SearchOutlined style={{ color: "var(--text-muted)", fontSize: 11 }} />}
            allowClear
            value={querySearch}
            onChange={(e) => setQuerySearch(e.target.value)}
            style={{ marginBottom: 8 }}
          />
          <Table<queryhistory.QueryHistoryRow>
            dataSource={visibleRows ?? []}
            columns={columns}
            rowKey="queryId"
            size="small"
            scroll={{ y: 420 }}
            pagination={{ pageSize: 50, showSizeChanger: false }}
            expandable={{
              expandedRowRender: (row) => {
                const details: { label: string; value: string | number | null }[] = [
                  { label: "User",          value: row.userName      || null },
                  { label: "Warehouse",     value: row.warehouseName || null },
                  { label: "Database",      value: row.databaseName  || null },
                  { label: "Schema",        value: row.schemaName    || null },
                  { label: "Rows produced", value: row.rowsProduced  ?? null },
                  { label: "Bytes scanned", value: row.bytesScanned  ?? null },
                  { label: "Query ID",      value: row.queryId       || null },
                ];
                return (
                  <div style={{ padding: "8px 0 4px" }}>
                    <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: "0 0 10px", fontFamily: "monospace" }}>
                      {highlight(row.queryText, querySearch)}
                    </pre>
                    <div style={{ display: "grid", gridTemplateColumns: "max-content 1fr", rowGap: 3, columnGap: 12, fontSize: 12, marginBottom: 8 }}>
                      {details.map(({ label, value }) => value !== null && (
                        <>
                          <span key={label + "-l"} style={{ color: "var(--text-muted)", whiteSpace: "nowrap" }}>{label}</span>
                          <span key={label + "-v"} style={{ fontFamily: label === "Query ID" ? "monospace" : undefined, wordBreak: "break-all" }}>{String(value)}</span>
                        </>
                      ))}
                    </div>
                    <Space>
                      <Button size="small" onClick={() => loadInEditor(row.queryText)}>
                        Load in Editor
                      </Button>
                      <Button
                        size="small"
                        onClick={() => {
                          ClipboardSetText(row.queryText);
                          if (copyTimer.current) clearTimeout(copyTimer.current);
                          setCopiedId(row.queryId);
                          copyTimer.current = setTimeout(() => setCopiedId(null), 1500);
                        }}
                      >
                        {copiedId === row.queryId ? "Copied!" : "Copy"}
                      </Button>
                      {row.queryId && (
                        <Button
                          size="small"
                          icon={<BarChartOutlined />}
                          onClick={() => { setProfileQueryId(row.queryId); }}
                        >
                          Profile
                        </Button>
                      )}
                      {row.errorMessage && (
                        <Text type="danger" style={{ fontSize: 11 }}>{row.errorMessage}</Text>
                      )}
                    </Space>
                  </div>
                );
              },
            }}
          />
          <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
            {visibleRows?.length ?? 0}{querySearch.trim() && visibleRows?.length !== rows.length ? ` of ${rows.length}` : ""} row{(visibleRows?.length ?? 0) !== 1 ? "s" : ""}
          </Text>
        </>
      )}
    </Modal>

    {profileQueryId && (
      <QueryProfileModal
        queryId={profileQueryId}
        onClose={() => setProfileQueryId(null)}
      />
    )}
    </>
  );
}
