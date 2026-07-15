// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Space, Typography, Alert, Table, Form, Dropdown,
} from "antd";
import {
  DotChartOutlined, ReloadOutlined, PlusOutlined, MoreOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterDataset, ListDatasetVersions } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Single-quote-escape a SQL string literal. Snowflake processes backslash escape
// sequences inside single-quoted literals, so backslashes must be doubled too
// (escape backslashes first, then single quotes) — matters for JSON METADATA.
// Dataset version names are string literals (ADD VERSION 'v1' / DROP VERSION 'v1'),
// unlike model version names which are identifiers.
const q1 = (s: string) => `'${s.replace(/\\/g, "\\\\").replace(/'/g, "''")}'`;

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle",
  width: 200,
};

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function DatasetPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Lazily-loaded version listing (SHOW VERSIONS IN DATASET).
  const [versions, setVersions] = useState<snowflake.QueryResult | null>(null);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [versionsError, setVersionsError] = useState<string | null>(null);

  const [addOpen, setAddOpen] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "DATASET", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const datasetRef = `"${db}"."${schema}"."${name}"`;

  const loadVersions = useCallback(async () => {
    setVersionsLoading(true);
    setVersionsError(null);
    try {
      const res = await ListDatasetVersions(db, schema, name);
      setVersions(res ?? null);
    } catch (e) {
      setVersionsError(String(e));
    } finally {
      setVersionsLoading(false);
    }
  }, [db, schema, name]);

  // Run a version-scoped clause, then refresh versions + the SHOW DATASETS header.
  const runVersionClause = async (clause: string) => {
    setActionError(null);
    await AlterDataset(db, schema, name, clause);
    await loadVersions();
    await reload();
  };

  // ── Versions → table ──────────────────────────────────────────────────────
  const versionCols = versions?.columns ?? [];
  // SHOW VERSIONS reports the version name in a column whose exact label isn't
  // contractually fixed across Snowflake editions. Probe in priority order with
  // exact matches first ("version" / "version_name" / "name"), then fall back to
  // any column containing "version" (e.g. a future "dataset_version") so the
  // per-row Drop action keeps working rather than silently disappearing.
  const versionNameIdx = (() => {
    for (const want of ["version", "version_name", "name"]) {
      const i = versionCols.findIndex((c) => c.toLowerCase() === want);
      if (i >= 0) return i;
    }
    return versionCols.findIndex((c) => c.toLowerCase().includes("version"));
  })();
  const versionColumns = versionCols.map((col, idx) => ({
    title: col,
    dataIndex: String(idx),
    key: String(idx),
    ellipsis: true,
    render: (v: unknown) => (
      <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v == null ? "" : String(v)}</span>
    ),
  }));
  const versionData = (versions?.rows ?? []).map((row, ri) => {
    const obj: Record<string, unknown> = { key: ri };
    row.forEach((cell, ci) => { obj[String(ci)] = cell; });
    obj.__version = versionNameIdx >= 0 ? String(row[versionNameIdx] ?? "") : "";
    return obj;
  });

  // Append an Actions column exposing DROP VERSION (the only per-version ALTER op).
  if (versionNameIdx >= 0) {
    versionColumns.push({
      title: "Actions",
      dataIndex: "__actions",
      key: "__actions",
      ellipsis: false,
      render: ((_: unknown, rec: Record<string, unknown>) => {
        const ver = String(rec.__version ?? "");
        if (!ver) return null;
        return (
          <Dropdown
            trigger={["click"]}
            menu={{
              items: [
                { key: "drop", danger: true, label: "Drop version…" },
              ],
              onClick: ({ key }) => {
                if (key === "drop") {
                  Modal.confirm({
                    title: `Drop version "${ver}"?`,
                    content: "This permanently removes the version from the dataset.",
                    okButtonProps: { danger: true },
                    okText: "Drop",
                    onOk: () => runVersionClause(`DROP VERSION ${q1(ver)}`).catch((e) => setActionError(String(e))),
                  });
                }
              },
            }}
          >
            <Button size="small" type="text" icon={<MoreOutlined />} />
          </Dropdown>
        );
      }) as never,
    } as never);
  }

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <DotChartOutlined style={{ color: "var(--link)" }} />
          <span>Dataset Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {datasetRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={860}
      styles={{ body: { maxHeight: "74vh", overflowY: "auto", paddingTop: 16 } }}
    >
      {!rows && !error && (
        <div style={{ textAlign: "center", padding: 32 }}>
          <Spin />
        </div>
      )}
      {error && (
        <Alert type="error" message="Failed to load properties" description={error} showIcon />
      )}
      {rows && (
        <>
          {actionError && (
            <Alert
              type="error"
              message={actionError}
              showIcon
              closable
              onClose={() => setActionError(null)}
              style={{ marginBottom: 12 }}
            />
          )}

          <div style={SECTION_HEAD}>Versions</div>
          {versionsError && (
            <Alert type="error" message="Failed to load versions" description={versionsError} showIcon style={{ marginBottom: 8 }} />
          )}
          {versions ? (
            <>
              <Space style={{ marginBottom: 8 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {versionData.length === 0 ? "No versions in this dataset." : `${versionData.length} version${versionData.length === 1 ? "" : "s"}.`}
                </Text>
                <Button size="small" icon={<ReloadOutlined />} onClick={loadVersions} loading={versionsLoading}>
                  Refresh
                </Button>
                <Button size="small" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
                  Add version…
                </Button>
              </Space>
              {versionData.length > 0 && (
                <Table
                  size="small"
                  columns={versionColumns}
                  dataSource={versionData}
                  pagination={false}
                  scroll={{ x: true }}
                />
              )}
            </>
          ) : (
            <Space>
              <Button size="small" icon={<ReloadOutlined />} onClick={loadVersions} loading={versionsLoading}>
                Load versions
              </Button>
              <Button size="small" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
                Add version…
              </Button>
            </Space>
          )}

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows.map((r) => (
                <tr key={r.key}>
                  <td style={LABEL_TD}>{r.key}</td>
                  <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
                    {r.value || <Text type="secondary">(empty)</Text>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}

      {/* Add a version from a query. */}
      <AddVersionDialog
        open={addOpen}
        onCancel={() => setAddOpen(false)}
        onSubmit={async (clause) => {
          await runVersionClause(clause);
          setAddOpen(false);
        }}
      />
    </Modal>
  );
}

// ─── Add-version dialog ──────────────────────────────────────────────────────

interface AddVersionDialogProps {
  open: boolean;
  onCancel: () => void;
  onSubmit: (clause: string) => Promise<void>;
}

function AddVersionDialog({ open, onCancel, onSubmit }: AddVersionDialogProps) {
  const [version, setVersion] = useState("");
  const [query, setQuery] = useState("");
  const [partitionBy, setPartitionBy] = useState("");
  const [comment, setComment] = useState("");
  const [metadata, setMetadata] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reset = () => {
    setVersion(""); setQuery(""); setPartitionBy("");
    setComment(""); setMetadata(""); setError(null);
  };

  const canSubmit = version.trim() !== "" && query.trim() !== "";

  const submit = async () => {
    if (!canSubmit) return;
    // ALTER DATASET <name> ADD VERSION 'v1' FROM ( <query> )
    //   [PARTITION BY <expr>] [COMMENT = '...'] [METADATA = '...']
    // Version names are string literals; the FROM query is wrapped in parentheses
    // (matching the documented grammar); PARTITION BY is a bare column expression.
    let clause = `ADD VERSION ${q1(version.trim())} FROM (\n${query.trim()}\n)`;
    if (partitionBy.trim() !== "") clause += `\nPARTITION BY ${partitionBy.trim()}`;
    if (comment.trim() !== "") clause += `\nCOMMENT = ${q1(comment.trim())}`;
    if (metadata.trim() !== "") clause += `\nMETADATA = ${q1(metadata.trim())}`;
    setBusy(true);
    setError(null);
    try {
      await onSubmit(clause);
      reset();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      open={open}
      title="Add dataset version"
      onCancel={() => { reset(); onCancel(); }}
      onOk={submit}
      okText="Add version"
      okButtonProps={{ disabled: !canSubmit }}
      confirmLoading={busy}
      width={620}
      destroyOnClose
    >
      <Form layout="vertical" size="small">
        <Form.Item label="Version name" required style={{ marginBottom: 12 }}>
          <Input value={version} onChange={(e) => setVersion(e.target.value)} placeholder="v1" />
        </Form.Item>
        <Form.Item label="Source query (FROM)" required style={{ marginBottom: 12 }} help="A SELECT statement producing the version's data. Wrapped in parentheses automatically.">
          <Input.TextArea
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={"SELECT * FROM my_table"}
            rows={5}
            style={{ fontFamily: "var(--font-mono)" }}
          />
        </Form.Item>
        <Form.Item label="Partition by" style={{ marginBottom: 12 }} help="Optional. A column or expression to partition the version's files by.">
          <Input value={partitionBy} onChange={(e) => setPartitionBy(e.target.value)} placeholder="PART" />
        </Form.Item>
        <Form.Item label="Comment" style={{ marginBottom: 12 }}>
          <Input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="Initial version" />
        </Form.Item>
        <Form.Item label="Metadata (JSON)" style={{ marginBottom: 4 }} help="Optional. Arbitrary JSON metadata stored with the version.">
          <Input.TextArea
            value={metadata}
            onChange={(e) => setMetadata(e.target.value)}
            placeholder={'{"source":"some_table"}'}
            rows={3}
            style={{ fontFamily: "var(--font-mono)" }}
          />
        </Form.Item>
        {error && <Text type="danger" style={{ fontSize: 11, display: "block", marginTop: 8 }}>{error}</Text>}
      </Form>
    </Modal>
  );
}
