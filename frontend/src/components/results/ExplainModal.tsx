// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Table, Tag, Spin, Alert, Space, Typography, Statistic, Row, Col, Divider } from "antd";
import type { ColumnsType } from "antd/es/table";
import { WarningOutlined, CheckCircleOutlined } from "@ant-design/icons";
import { RunExplain } from "../../../wailsjs/go/app/App";
import type { queryprofile } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  /** The SQL statement (selection or active statement) to EXPLAIN. */
  sql: string;
  tabId?: string;
  onClose: () => void;
}

// ── helpers ──────────────────────────────────────────────────────────────────

function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return "—";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1_048_576) return `${(bytes / 1024).toFixed(1)} KB`;
  if (bytes < 1_073_741_824) return `${(bytes / 1_048_576).toFixed(1)} MB`;
  return `${(bytes / 1_073_741_824).toFixed(2)} GB`;
}

// Flatten Operations (step → node[]) into a display-friendly list.
interface FlatNode {
  key: string;
  stepIndex: number;
  id: number;
  parent: number | null;
  operation: string;
  objects: string[];
  partitionsScanned: number;
  partitionsTotal: number;
  joinType: string;
}

function flattenNodes(operations: queryprofile.ExplainNode[][]): FlatNode[] {
  const rows: FlatNode[] = [];
  operations.forEach((step, si) => {
    step.forEach((node) => {
      rows.push({
        key: `${si}-${node.id}`,
        stepIndex: si,
        id: node.id,
        parent: node.parent ?? null,
        operation: node.operation ?? "",
        objects: node.objects ?? [],
        partitionsScanned: node.partitionsScanned ?? 0,
        partitionsTotal: node.partitionsTotal ?? 0,
        joinType: node.joinType ?? "",
      });
    });
  });
  return rows;
}

const OPERATION_COLORS: Record<string, string> = {
  TABLESCAN:    "green",
  FILTER:       "orange",
  AGGREGATE:    "purple",
  SORT:         "cyan",
  JOIN:         "magenta",
  WITHCLAUSE:   "geekblue",
  RESULT:       "blue",
  UNION:        "gold",
};

function operationColor(op: string): string {
  const key = (op ?? "").toUpperCase().replace(/\s/g, "");
  return OPERATION_COLORS[key] ?? "default";
}

// ── Column definitions ────────────────────────────────────────────────────────

const NODE_COLS: ColumnsType<FlatNode> = [
  {
    key: "stepIndex",
    title: <span style={{ fontSize: 11 }}>step</span>,
    dataIndex: "stepIndex",
    width: 50,
    render: (v: number) => <span style={{ fontFamily: "monospace", fontSize: 11 }}>{v}</span>,
  },
  {
    key: "id",
    title: <span style={{ fontSize: 11 }}>id</span>,
    dataIndex: "id",
    width: 50,
    render: (v: number) => <span style={{ fontFamily: "monospace", fontSize: 11 }}>{v}</span>,
  },
  {
    key: "operation",
    title: <span style={{ fontSize: 11 }}>operation</span>,
    dataIndex: "operation",
    width: 160,
    render: (v: string) =>
      v ? (
        <Tag
          color={operationColor(v)}
          style={{ fontFamily: "monospace", fontSize: 10, margin: 0, lineHeight: "18px" }}
        >
          {v}
        </Tag>
      ) : null,
  },
  {
    key: "objects",
    title: <span style={{ fontSize: 11 }}>objects</span>,
    dataIndex: "objects",
    render: (v: string[]) =>
      v?.length ? (
        <Space size={4} wrap>
          {v.map((o, i) => (
            <Text key={i} style={{ fontFamily: "monospace", fontSize: 10 }}>
              {o.split(".").pop() ?? o}
            </Text>
          ))}
        </Space>
      ) : (
        <span style={{ color: "var(--text-faint)" }}>—</span>
      ),
  },
  {
    key: "partitions",
    title: <span style={{ fontSize: 11 }}>partitions (scanned / total)</span>,
    width: 200,
    render: (_: unknown, r: FlatNode) => {
      if (!r.partitionsTotal) return <span style={{ color: "var(--text-faint)" }}>—</span>;
      const pct = Math.round((r.partitionsScanned / r.partitionsTotal) * 100);
      const color = pct >= 90 ? "#ff4d4f" : pct >= 50 ? "#faad14" : "#52c41a";
      return (
        <span style={{ fontFamily: "monospace", fontSize: 11, color }}>
          {r.partitionsScanned} / {r.partitionsTotal} ({pct}%)
        </span>
      );
    },
  },
  {
    key: "joinType",
    title: <span style={{ fontSize: 11 }}>join type</span>,
    dataIndex: "joinType",
    width: 100,
    render: (v: string) =>
      v ? (
        <Tag
          color={v.toLowerCase() === "cartesian" ? "red" : "default"}
          style={{ fontSize: 10, margin: 0, lineHeight: "18px" }}
        >
          {v}
        </Tag>
      ) : (
        <span style={{ color: "var(--text-faint)" }}>—</span>
      ),
  },
];

// ── Component ─────────────────────────────────────────────────────────────────

export default function ExplainModal({ sql, tabId, onClose }: Props) {
  const [loading, setLoading] = useState(true);
  const [result, setResult] = useState<queryprofile.ExplainResult | null>(null);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    setError(null);
    RunExplain(tabId || "", sql)
      .then((r) => setResult(r))
      .catch((e) => setError(String(e)))
      .finally(() => setLoading(false));
  }, [sql, tabId]);

  const shortSql = sql.length > 80 ? `${sql.slice(0, 78)}…` : sql;
  const flatNodes = result?.plan ? flattenNodes(result.plan.Operations ?? []) : [];
  const diags = result?.diagnostics ?? [];

  return (
    <Modal
      open
      title={
        <Space size={8}>
          <span>Explain Plan</span>
          <Text
            style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)" }}
            title={sql}
          >
            {shortSql}
          </Text>
        </Space>
      }
      onCancel={onClose}
      width="min(1100px, 95vw)"
      footer={null}
      styles={{ body: { paddingTop: 12 } }}
    >
      {loading && (
        <div style={{ textAlign: "center", padding: 48 }}>
          <Spin />
          <div style={{ marginTop: 10, fontSize: 12, color: "var(--text-muted)" }}>
            Running EXPLAIN…
          </div>
        </div>
      )}

      {!loading && error && (
        <Alert type="error" message={error} />
      )}

      {!loading && result?.plan && (
        <>
          {/* ── Global stats ──────────────────────────────────────────── */}
          <Row gutter={24} style={{ marginBottom: 16 }}>
            <Col>
              <Statistic
                title="Partitions Scanned"
                value={result.plan.GlobalStats?.partitionsScanned ?? 0}
                suffix={`/ ${result.plan.GlobalStats?.partitionsTotal ?? 0}`}
                valueStyle={{ fontSize: 18 }}
              />
            </Col>
            <Col>
              <Statistic
                title="Estimated Bytes"
                value={formatBytes(result.plan.GlobalStats?.bytesAssigned ?? 0)}
                valueStyle={{ fontSize: 18 }}
              />
            </Col>
          </Row>

          {/* ── Plan nodes ────────────────────────────────────────────── */}
          <Table<FlatNode>
            dataSource={flatNodes}
            columns={NODE_COLS}
            rowKey="key"
            size="small"
            pagination={false}
            scroll={{ x: "max-content", y: 340 }}
            style={{ marginBottom: diags.length > 0 ? 0 : undefined }}
          />

          {/* ── Performance issues ────────────────────────────────────── */}
          {diags.length > 0 && (
            <>
              <Divider style={{ margin: "14px 0 10px" }} />
              <Text style={{ fontSize: 12, fontWeight: 600 }}>
                Performance Issues ({diags.length})
              </Text>
              <div style={{ marginTop: 8, display: "flex", flexDirection: "column", gap: 6 }}>
                {diags.map((d, i) => (
                  <Alert
                    key={i}
                    type={d.severity === 8 ? "error" : "warning"}
                    icon={d.severity === 8 ? <WarningOutlined /> : undefined}
                    message={
                      <span style={{ fontFamily: "monospace", fontSize: 11, whiteSpace: "pre-wrap" }}>
                        {d.message}
                      </span>
                    }
                    style={{ padding: "6px 10px" }}
                    showIcon
                  />
                ))}
              </div>
            </>
          )}

          {diags.length === 0 && !loading && (
            <>
              <Divider style={{ margin: "14px 0 10px" }} />
              <Alert
                type="success"
                icon={<CheckCircleOutlined />}
                message="No performance issues detected."
                showIcon
                style={{ padding: "6px 10px" }}
              />
            </>
          )}
        </>
      )}
    </Modal>
  );
}
