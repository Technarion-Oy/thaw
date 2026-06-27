// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef, Fragment } from "react";
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

// Largest int64 — Snowflake session IDs are int64, so the box must reject
// over-long pastes the backend would otherwise error on. Kept in sync with the
// backend's snowflake.IsNumericID / strconv.ParseInt(_, 10, 64) guard.
const INT64_MAX = 9223372036854775807n;

// isValidSessionId reports whether s is a non-empty decimal integer that fits in
// an int64 (digits only — no sign, whitespace, or leading zeros, so the value
// embedded unquoted matches what the user sees). Mirrors snowflake.IsNumericID.
function isValidSessionId(s: string): boolean {
  const t = s.trim();
  if (!/^(?:0|[1-9]\d*)$/.test(t)) return false;
  try {
    return BigInt(t) <= INT64_MAX;
  } catch {
    return false;
  }
}

// "Today" as an end-of-day-bounded range. A function (not a constant) so the
// bounds are evaluated when the range is applied, not at module load.
const todayRange = (): [Dayjs, Dayjs] => [dayjs().startOf("day"), dayjs().endOf("day")];

interface Props {
  onClose: () => void;
}

type FilterType = "session" | "user" | "warehouse" | "all";

export default function QueryHistoryModal({ onClose }: Props) {
  const defaultUser      = useConnectionStore((s) => s.params?.user ?? "");
  const defaultWarehouse = useSessionStore((s) => s.warehouse);
  const warehouses       = useSessionStore((s) => s.warehouses);
  const loadWarehouses   = useSessionStore((s) => s.loadWarehouses);

  const [filterType,      setFilterType]      = useState<FilterType>("user");
  const [sessionId,       setSessionId]       = useState("");
  const [userName,        setUserName]        = useState(defaultUser);
  const [warehouseName,   setWarehouseName]   = useState(defaultWarehouse);
  // Default to "today" so the modal opens on the current user's recent activity
  // rather than an empty range. The upper bound is end-of-day (not "now") so a
  // re-run later in the same session still includes queries that completed in
  // the meantime. The picker stays adjustable.
  const [timeRange,       setTimeRange]       = useState<[Dayjs, Dayjs] | null>(todayRange);
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
  // True when entering session scope cleared a previously-set range — only then
  // do we restore "today" on leaving, so a range the user deliberately cleared on
  // another scope is preserved.
  const sessionClearedRange = useRef(false);

  // runQuery accepts optional overrides so callers (e.g. "Filter by this
  // session") can run with new scope/session/range values before the
  // corresponding setState has been flushed. `timeRange` uses `in` detection so
  // an explicit `null` (clear the range) is distinguishable from "not provided".
  const runQuery = async (override?: {
    filterType?: FilterType;
    sessionId?: string;
    userName?: string;
    warehouseName?: string;
    timeRange?: [Dayjs, Dayjs] | null;
  }) => {
    const ft    = override?.filterType ?? filterType;
    const sid   = (override?.sessionId ?? sessionId).trim();
    const un    = (override?.userName ?? userName).trim();
    const wn    = (override?.warehouseName ?? warehouseName).trim();
    const range = override && "timeRange" in override ? override.timeRange : timeRange;
    setLoading(true);
    setError(null);
    setRows(null);
    try {
      const start = range ? range[0].toISOString() : "";
      const end   = range ? range[1].toISOString() : "";
      const data = await GetQueryHistory(
        ft,
        sid,
        un,
        wn,
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

  // Switch to the "session" scope for a specific session id and re-run. Used by
  // the per-row "Filter by this session" action so users can drill down from a
  // query they recognise to everything that ran in the same session. The time
  // range is cleared so sessions that started before today are not filtered out.
  const filterBySession = (sid: string) => {
    // Symmetric with the manual input's Run guard — never enter a stuck state
    // (scope=session, Run disabled) from a malformed id.
    if (!isValidSessionId(sid)) return;
    // Single source of truth for the drill-down filters: the same object drives
    // both the state updates (for the UI) and the override (for the immediate
    // run, since setState hasn't flushed yet in this handler). When adding a new
    // runQuery param, thread it through `params` here only.
    const params = { filterType: "session" as FilterType, sessionId: sid, timeRange: null };
    // Remember whether session scope cleared a real range, so a later switch away
    // restores "today" only if it did. Only latch on the *first* entry into
    // session scope — a session→session drill-down (timeRange already null) must
    // not reset the latch to false.
    if (filterType !== "session") {
      sessionClearedRange.current = timeRange !== null;
    }
    setFilterType(params.filterType);
    setSessionId(params.sessionId);
    setTimeRange(params.timeRange);
    setQuerySearch("");
    runQuery(params);
  };

  // Manual "Run" from the form: clear the stale query-text filter so new results
  // aren't silently pre-filtered by an old search term (filterBySession already
  // clears it for the drill-down path).
  const handleRun = () => {
    setQuerySearch("");
    runQuery();
  };

  // "session" scope needs an explicit int64 id. An empty id would silently fall
  // back to QUERY_HISTORY_BY_SESSION() of the pooled metadata connection (never
  // ran the user's editor SQL); a non-numeric / overflowing id is rejected by
  // the backend. Disable Run rather than issue a request that can't succeed.
  const sessionScopeInvalid = filterType === "session" && !isValidSessionId(sessionId);
  // Distinct from the above: flag the field red once the user has typed anything
  // invalid (checked on the raw value so whitespace-only input — which disables
  // Run — still shows a cue), but not the instant they switch to session scope.
  const sessionIdHasError = sessionId !== "" && sessionScopeInvalid;
  // "user"/"warehouse" scope each need an explicit name — an empty filter widens
  // the query (user) or resolves to the pooled metadata connection (warehouse);
  // the backend rejects both. Disable Run; use "all" scope to query across users.
  const userScopeInvalid = filterType === "user" && userName.trim() === "";
  const warehouseScopeInvalid = filterType === "warehouse" && warehouseName.trim() === "";
  const runDisabled = sessionScopeInvalid || userScopeInvalid || warehouseScopeInvalid;

  // Auto-run on mount once the current user is known. If the connection store
  // hasn't hydrated yet, `defaultUser` is "" at mount; wait for it. `filterType`
  // is a dependency so that if the user switched scope before the user arrived,
  // switching back to "user" later re-evaluates and finally fires (the latch
  // ensures it still happens at most once).
  const didAutoRun = useRef(false);
  useEffect(() => {
    if (didAutoRun.current) return;
    if (!defaultUser.trim()) return;   // still waiting for the connection user — don't latch yet
    if (filterType !== "user") return; // user is driving another scope — stay eligible
    didAutoRun.current = true;         // only consume the latch when we actually auto-run
    // Prefer a name the user already typed (slow-hydration race) over the
    // connection default, so the query and the visible input never diverge.
    const user = userName.trim() || defaultUser;
    if (user !== userName) setUserName(user);
    runQuery({ userName: user });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [defaultUser, filterType]);

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
            disabled={loading}
            onChange={(v) => {
              if (v === "session") {
                // Entering session scope (always from a non-session scope here):
                // clear the range so a pasted id for an older session isn't
                // silently bounded by today's window. Record whether we actually
                // cleared a range, so the inverse switch knows whether restoring
                // "today" is warranted.
                sessionClearedRange.current = timeRange !== null;
                setTimeRange(null);
              } else if (filterType === "session") {
                // Leaving session scope: clear the now-stale id so switching back
                // doesn't silently re-query it, and restore "today" only if session
                // scope is what cleared the range — never clobber a range the user
                // deliberately cleared while on user/warehouse/all.
                setSessionId("");
                if (sessionClearedRange.current && timeRange === null) {
                  setTimeRange(todayRange());
                }
                sessionClearedRange.current = false;
              }
              setFilterType(v);
            }}
            style={{ width: 160 }}
            options={[
              { value: "session",   label: "By Session" },
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
              status={userScopeInvalid ? "error" : undefined}
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

        {filterType === "session" && (
          <div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Session ID</div>
            <Input
              size="small"
              value={sessionId}
              status={sessionIdHasError ? "error" : undefined}
              onChange={(e) => setSessionId(e.target.value)}
              onPressEnter={() => { if (!runDisabled && !loading) handleRun(); }}
              style={{ width: 180 }}
              placeholder="Paste a numeric session ID…"
            />
          </div>
        )}

        {filterType === "warehouse" && (
          <div>
            <div style={{ fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>Warehouse name</div>
            <AutoComplete
              size="small"
              value={warehouseName}
              status={warehouseScopeInvalid ? "error" : undefined}
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

        <Button
          type="primary"
          size="small"
          onClick={handleRun}
          loading={loading}
          disabled={runDisabled}
        >
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
                const details: { label: string; value: string | number | null; mono?: boolean }[] = [
                  { label: "User",          value: row.userName      || null },
                  { label: "Warehouse",     value: row.warehouseName || null },
                  { label: "Database",      value: row.databaseName  || null },
                  { label: "Schema",        value: row.schemaName    || null },
                  { label: "Rows produced", value: row.rowsProduced  ?? null },
                  { label: "Bytes scanned", value: row.bytesScanned  ?? null },
                  { label: "Session ID",    value: row.sessionId     || null, mono: true },
                  { label: "Query ID",      value: row.queryId       || null, mono: true },
                ];
                return (
                  <div style={{ padding: "8px 0 4px" }}>
                    <pre style={{ whiteSpace: "pre-wrap", fontSize: 12, margin: "0 0 10px", fontFamily: "monospace" }}>
                      {highlight(row.queryText, querySearch)}
                    </pre>
                    <div style={{ display: "grid", gridTemplateColumns: "max-content 1fr", rowGap: 3, columnGap: 12, fontSize: 12, marginBottom: 8 }}>
                      {details.filter(({ value }) => value !== null).map(({ label, value, mono }) => (
                        <Fragment key={label}>
                          <span style={{ color: "var(--text-muted)", whiteSpace: "nowrap" }}>{label}</span>
                          <span style={{ fontFamily: mono ? "monospace" : undefined, wordBreak: "break-all" }}>{String(value)}</span>
                        </Fragment>
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
                      {row.sessionId && (
                        <Button
                          size="small"
                          disabled={loading}
                          onClick={() => filterBySession(row.sessionId)}
                        >
                          Filter by this session
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
