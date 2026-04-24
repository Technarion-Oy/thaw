// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useCallback, useRef } from "react";
import { Modal, Table, Tag, Button, Spin, Alert, Tooltip, Space, Typography, Divider } from "antd";
import { ReloadOutlined, BulbOutlined } from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import { GetQueryOperatorStats, SendChatMessage, GetAIConfig } from "../../../wailsjs/go/main/App";
import type { queryprofile } from "../../../wailsjs/go/models";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";

const { Text } = Typography;

interface Props {
  queryId: string;
  onClose: () => void;
  /** When true, auto-refreshes every 3 s (used while a query is still running). */
  liveRefresh?: boolean;
  /** SQL that produced this query ID — enables the AI suggestion button. */
  sql?: string;
}

const OPERATOR_TYPE_COLORS: Record<string, string> = {
  RESULT: "blue",
  TABLESCAN: "green",
  INMEMTABLESCAN: "green",
  FILTER: "orange",
  AGGREGATE: "purple",
  SORT: "cyan",
  JOIN: "magenta",
  MERGEJOIN: "magenta",
  HASHJOIN: "magenta",
  NESTEDLOOP: "magenta",
  WITHREFERENCE: "geekblue",
  EXTERNALFUNCTION: "volcano",
  UNION: "gold",
  LIMIT: "lime",
};

// ── AI helpers ──────────────────────────────────────────────────────────────

type Segment = { type: "text" | "code"; content: string; lang?: string };

function parseSegments(text: string): Segment[] {
  const parts: Segment[] = [];
  const regex = /```(\w*)\n?([\s\S]*?)```/g;
  let last = 0;
  let match: RegExpExecArray | null;
  while ((match = regex.exec(text)) !== null) {
    if (match.index > last) parts.push({ type: "text", content: text.slice(last, match.index) });
    parts.push({ type: "code", lang: match[1] || undefined, content: match[2] });
    last = match.index + match[0].length;
  }
  if (last < text.length) parts.push({ type: "text", content: text.slice(last) });
  return parts;
}

function buildAiPrompt(sql: string, rows: queryprofile.OperatorStat[]): string {
  const header = "step_id | operator_id | parent_operators | operator_type | operator_statistics | execution_time_breakdown | operator_attributes";
  const body = rows.map((r) =>
    [
      r.stepId ?? "—",
      r.operatorId ?? "—",
      r.parentOperators?.length ? r.parentOperators.join(",") : "—",
      r.operatorType || "—",
      r.operatorStatistics != null ? JSON.stringify(r.operatorStatistics) : "—",
      r.executionTimeBreakdown != null ? JSON.stringify(r.executionTimeBreakdown) : "—",
      r.operatorAttributes != null ? JSON.stringify(r.operatorAttributes) : "—",
    ].join(" | ")
  ).join("\n");

  return `You are a Snowflake SQL performance expert. Rewrite the query below to be as fast as possible based on its execution plan.

Return only the improved SQL with brief inline comments explaining each change. Do not suggest schema changes, clustering keys, materialized views, or any other infrastructure changes — focus exclusively on rewriting the query itself.

SQL:
\`\`\`sql
${sql.trim()}
\`\`\`

Execution plan (GET_QUERY_OPERATOR_STATS):
\`\`\`
${header}
${body}
\`\`\``;
}

// ── End AI helpers ───────────────────────────────────────────────────────────

function operatorTypeColor(type: string): string {
  const key = (type ?? "").toUpperCase().replace(/[\s_]/g, "");
  return OPERATOR_TYPE_COLORS[key] ?? "default";
}

function renderJsonCell(value: unknown): React.ReactNode {
  if (value == null) return <span style={{ color: "var(--text-faint)" }}>—</span>;
  const text = JSON.stringify(value, null, 2);
  if (!text || text === "null") return <span style={{ color: "var(--text-faint)" }}>—</span>;
  return (
    <pre
      style={{
        fontFamily: "monospace",
        fontSize: 10,
        margin: 0,
        maxHeight: 130,
        overflow: "auto",
        whiteSpace: "pre-wrap",
        wordBreak: "break-word",
        background: "var(--bg-subtle, rgba(0,0,0,0.03))",
        padding: "2px 5px",
        borderRadius: 3,
        lineHeight: 1.45,
      }}
    >
      {text}
    </pre>
  );
}

const TABLE_COLS: ColumnsType<queryprofile.OperatorStat> = [
  {
    key: "stepId",
    title: <span style={{ fontSize: 11 }}>step&nbsp;id</span>,
    dataIndex: "stepId",
    width: 65,
    render: (v: number) => (
      <span style={{ fontFamily: "monospace", fontSize: 11 }}>{v ?? "—"}</span>
    ),
  },
  {
    key: "operatorId",
    title: <span style={{ fontSize: 11 }}>operator&nbsp;id</span>,
    dataIndex: "operatorId",
    width: 65,
    render: (v: number) => (
      <span style={{ fontFamily: "monospace", fontSize: 11 }}>{v ?? "—"}</span>
    ),
  },
  {
    key: "parentOperators",
    title: <span style={{ fontSize: 11 }}>parent&nbsp;operators</span>,
    dataIndex: "parentOperators",
    width: 110,
    render: (v: number[]) =>
      v?.length ? (
        <span>{v.join(", ")}</span>
      ) : (
        <span style={{ color: "var(--text-faint)" }}>—</span>
      ),
  },
  {
    key: "operatorType",
    title: <span style={{ fontSize: 11 }}>operator&nbsp;type</span>,
    dataIndex: "operatorType",
    width: 165,
    render: (v: string) =>
      v ? (
        <Tag
          color={operatorTypeColor(v)}
          style={{ fontFamily: "monospace", fontSize: 10, margin: 0, lineHeight: "18px" }}
        >
          {v}
        </Tag>
      ) : (
        <span style={{ color: "var(--text-faint)" }}>—</span>
      ),
  },
  {
    key: "operatorStatistics",
    title: <span style={{ fontSize: 11 }}>operator&nbsp;statistics</span>,
    dataIndex: "operatorStatistics",
    width: 290,
    render: renderJsonCell,
  },
  {
    key: "executionTimeBreakdown",
    title: <span style={{ fontSize: 11 }}>execution&nbsp;time&nbsp;breakdown</span>,
    dataIndex: "executionTimeBreakdown",
    width: 260,
    render: renderJsonCell,
  },
  {
    key: "operatorAttributes",
    title: <span style={{ fontSize: 11 }}>operator&nbsp;attributes</span>,
    dataIndex: "operatorAttributes",
    width: 290,
    render: renderJsonCell,
  },
];

export default function QueryProfileModal({ queryId, onClose, liveRefresh, sql }: Props) {
  const [loading,       setLoading]       = useState(false);
  const [rows,          setRows]          = useState<queryprofile.OperatorStat[]>([]);
  const [error,         setError]         = useState<string | null>(null);
  const [lastRefreshed, setLastRefreshed] = useState<Date | null>(null);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const [aiEnabled,   setAiEnabled]   = useState(false);
  const [aiLoading,   setAiLoading]   = useState(false);
  const [aiSegments,  setAiSegments]  = useState<Segment[] | null>(null);
  const [aiError,     setAiError]     = useState<string | null>(null);
  const [copiedIdx,   setCopiedIdx]   = useState<number | null>(null);
  const copyTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const fetchStats = useCallback(async () => {
    setLoading(true);
    try {
      const stats = await GetQueryOperatorStats(queryId);
      setRows(stats ?? []);
      setError(null);
      setLastRefreshed(new Date());
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  }, [queryId]);

  useEffect(() => {
    fetchStats();
    if (liveRefresh) {
      timerRef.current = setInterval(fetchStats, 3000);
    }
    return () => { if (timerRef.current) clearInterval(timerRef.current); };
  }, [fetchStats, liveRefresh]);

  // Check whether AI is configured so we can show/hide the suggestion button.
  useEffect(() => {
    if (sql) {
      GetAIConfig().then((cfg) => setAiEnabled(cfg.enabled)).catch(() => {});
    }
  }, [sql]);

  const getSuggestions = async () => {
    if (!sql) return;
    setAiLoading(true);
    setAiSegments(null);
    setAiError(null);
    try {
      const prompt = buildAiPrompt(sql, rows);
      const msgs = await SendChatMessage([], prompt, sql, "", false);
      const assistant = msgs.find((m) => m.role === "assistant");
      setAiSegments(parseSegments(assistant?.text ?? ""));
    } catch (e) {
      setAiError(String(e));
    } finally {
      setAiLoading(false);
    }
  };

  const shortQid =
    queryId.length > 36 ? `${queryId.slice(0, 16)}…${queryId.slice(-16)}` : queryId;

  return (
    <Modal
      open
      title={
        <Space size={8}>
          <span>Query Profile</span>
          <Text
            style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}
            title={queryId}
          >
            {shortQid}
          </Text>
          {liveRefresh && (
            <Tag
              color="green"
              style={{ fontSize: 10, padding: "0 5px", lineHeight: "18px", marginInlineEnd: 0 }}
            >
              ● Live
            </Tag>
          )}
        </Space>
      }
      onCancel={onClose}
      width="min(1200px, 95vw)"
      footer={null}
      styles={{ body: { paddingTop: 12 } }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 8 }}>
        {lastRefreshed && (
          <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
            Updated {lastRefreshed.toLocaleTimeString()}
          </Text>
        )}
        <Tooltip title="Refresh">
          <Button
            size="small"
            type="text"
            icon={<ReloadOutlined style={{ fontSize: 11 }} />}
            loading={loading && rows.length > 0}
            onClick={fetchStats}
            style={{ height: 22, padding: "0 4px" }}
          />
        </Tooltip>
      </div>

      {error && (
        <Alert
          type="error"
          message={error}
          style={{ marginBottom: 8 }}
          closable
          onClose={() => setError(null)}
        />
      )}

      {loading && rows.length === 0 ? (
        <div style={{ textAlign: "center", padding: 40 }}>
          <Spin />
        </div>
      ) : !error && rows.length === 0 ? (
        <Alert
          type="info"
          message="No profiling data available yet"
          description="Operator statistics are populated once the query has started executing operators. If the query just started, wait a moment and click Refresh."
          style={{ marginBottom: 8 }}
        />
      ) : (
        <Table<queryprofile.OperatorStat>
          dataSource={rows}
          columns={TABLE_COLS}
          rowKey={(r) => `${r.stepId}-${r.operatorId}`}
          size="small"
          pagination={false}
          scroll={{ x: "max-content", y: 500 }}
        />
      )}

      {/* ── AI Suggestions ─────────────────────────────────────────────── */}
      {sql && aiEnabled && (
        <>
          <Divider style={{ margin: "14px 0 10px" }} />
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginBottom: 10 }}>
            <Text style={{ fontSize: 12, fontWeight: 600 }}>AI Suggestions</Text>
            <Button
              size="small"
              type={aiSegments ? "default" : "primary"}
              icon={<BulbOutlined />}
              loading={aiLoading}
              disabled={rows.length === 0}
              onClick={getSuggestions}
            >
              {aiSegments ? "Re-analyze" : "Analyze & Suggest"}
            </Button>
            {rows.length === 0 && (
              <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
                Load profile data first
              </Text>
            )}
          </div>

          {aiError && (
            <Alert type="error" message={aiError} closable onClose={() => setAiError(null)} style={{ marginBottom: 8 }} />
          )}

          {aiLoading && !aiSegments && (
            <div style={{ textAlign: "center", padding: 24 }}>
              <Spin />
              <div style={{ marginTop: 8, fontSize: 12, color: "var(--text-muted)" }}>
                Analyzing execution plan…
              </div>
            </div>
          )}

          {aiSegments && (
            <div style={{ fontSize: 13, lineHeight: 1.65, color: "var(--text)" }}>
              {aiSegments.map((seg, i) => {
                if (seg.type === "code") {
                  const isSql = !seg.lang || seg.lang.toLowerCase() === "sql";
                  const copied = copiedIdx === i;
                  return (
                    <div key={i} style={{ marginBottom: 10 }}>
                      <pre style={{
                        background: "var(--bg-overlay)",
                        border: "1px solid var(--border)",
                        borderRadius: 4,
                        padding: "8px 10px",
                        fontSize: 12,
                        fontFamily: "monospace",
                        overflowX: "auto",
                        whiteSpace: "pre-wrap",
                        wordBreak: "break-all",
                        margin: 0,
                        color: "var(--text)",
                      }}>
                        {seg.content}
                      </pre>
                      <Space size={4} style={{ marginTop: 4 }}>
                        <button
                          onClick={() => {
                            ClipboardSetText(seg.content);
                            if (copyTimerRef.current) clearTimeout(copyTimerRef.current);
                            setCopiedIdx(i);
                            copyTimerRef.current = setTimeout(() => setCopiedIdx(null), 1500);
                          }}
                          style={{ fontSize: 11, padding: "2px 8px", background: "none", border: "1px solid var(--border)", borderRadius: 3, color: "var(--text-muted)", cursor: "pointer" }}
                        >
                          {copied ? "Copied!" : "Copy"}
                        </button>
                        {isSql && (
                          <button
                            onClick={() => {
                              window.dispatchEvent(new CustomEvent("run-ai-sql", { detail: { sql: seg.content, run: false } }));
                              onClose();
                            }}
                            style={{ fontSize: 11, padding: "2px 8px", background: "none", border: "1px solid var(--border)", borderRadius: 3, color: "var(--accent)", cursor: "pointer" }}
                          >
                            Load in Editor
                          </button>
                        )}
                      </Space>
                    </div>
                  );
                }
                return (
                  <span key={i} style={{ whiteSpace: "pre-wrap" }}>
                    {seg.content}
                  </span>
                );
              })}
            </div>
          )}
        </>
      )}
    </Modal>
  );
}
