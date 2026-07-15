// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Table, Tag, Popconfirm, Checkbox,
} from "antd";
import {
  MergeCellsOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
  PlusOutlined, DeleteOutlined, KeyOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterHybridTable, ListHybridTableIndexes,
  CreateHybridTableIndex, DropHybridTableIndex, GetTableColumnsWithTypes,
  HybridIndexColumnOptions,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle",
  width: 220,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }

// ─── EditRow (inline comment editor) ─────────────────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  canUnset?: boolean;
  onSave: (val: string) => Promise<void>;
  onUnset?: () => Promise<void>;
}

function EditRow({ label, value, canUnset, onSave, onUnset }: EditRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(draft);
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const unset = async () => {
    if (!onUnset) return;
    setSaving(true);
    setError(null);
    try {
      await onUnset();
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Input
                size="small"
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                style={{ width: 280 }}
                onPressEnter={save}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} />
              </Tooltip>
              {canUnset && onUnset && (
                <Tooltip title="Unset (remove)">
                  <Button size="small" onClick={unset} loading={saving}>Unset</Button>
                </Tooltip>
              )}
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} />
              </Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">(not set)</Text>}</span>
            <Tooltip title="Edit">
              <Button
                type="text"
                size="small"
                icon={<EditOutlined style={{ fontSize: 11 }} />}
                onClick={() => { setDraft(value); setEditing(true); }}
                style={{ color: "var(--text-muted)" }}
              />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
        {value || <Text type="secondary">(empty)</Text>}
      </td>
    </tr>
  );
}

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function HybridTablePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Indexes (including the primary key, which surfaces here as an index).
  const [indexes, setIndexes] = useState<snowflake.QueryResult | null>(null);
  const [indexesLoading, setIndexesLoading] = useState(false);
  const [indexesError, setIndexesError] = useState<string | null>(null);

  // Index-eligible column names for the add-index dropdowns, computed by the
  // backend from the table's columns (datatype rules live in
  // internal/hybridtable, surfaced via HybridIndexColumnOptions).
  const [keyEligible, setKeyEligible] = useState<string[]>([]);
  const [includeEligible, setIncludeEligible] = useState<string[]>([]);

  // Add-index inline form.
  const [adding, setAdding] = useState(false);
  const [newIdxName, setNewIdxName] = useState("");
  const [newIdxCols, setNewIdxCols] = useState<string[]>([]);
  const [newIdxInclude, setNewIdxInclude] = useState<string[]>([]);
  const [newIdxCaseSensitive, setNewIdxCaseSensitive] = useState(false);
  const [creatingIdx, setCreatingIdx] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "HYBRID TABLE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  const loadIndexes = useCallback(async () => {
    setIndexesLoading(true);
    setIndexesError(null);
    try {
      const res = await ListHybridTableIndexes(db, schema, name);
      setIndexes(res ?? null);
    } catch (e) {
      setIndexesError(String(e));
    } finally {
      setIndexesLoading(false);
    }
  }, [db, schema, name]);

  const loadColumns = useCallback(async () => {
    try {
      const cols = await GetTableColumnsWithTypes(db, schema, name);
      const opts = await HybridIndexColumnOptions(
        (cols ?? []).map((c) => ({ name: c.name, type: c.dataType })) as any,
      );
      setKeyEligible(opts?.keyColumns ?? []);
      setIncludeEligible(opts?.includeColumns ?? []);
    } catch {
      // Non-fatal: the add-index dropdowns simply stay empty.
      setKeyEligible([]);
      setIncludeEligible([]);
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); loadIndexes(); loadColumns(); }, [reload, loadIndexes, loadColumns]);

  const tableRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterHybridTable(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterHybridTable(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const createIndex = async () => {
    setCreatingIdx(true);
    setActionError(null);
    try {
      await CreateHybridTableIndex(db, schema, name, {
        name: newIdxName,
        columns: newIdxCols,
        include: newIdxInclude,
      } as any, newIdxCaseSensitive);
      setNewIdxName(""); setNewIdxCols([]); setNewIdxInclude([]); setNewIdxCaseSensitive(false); setAdding(false);
      await loadIndexes();
    } catch (e) {
      setActionError(`Create index failed: ${String(e)}`);
    } finally {
      setCreatingIdx(false);
    }
  };

  const dropIndex = async (indexName: string) => {
    setActionError(null);
    try {
      await DropHybridTableIndex(db, schema, name, indexName);
      await loadIndexes();
    } catch (e) {
      setActionError(`Drop index failed: ${String(e)}`);
    }
  };

  const comment = find("comment");
  const owner = find("owner");
  const rowCount = find("rows");
  const bytes = find("bytes");
  // Keys rendered in the Overview / Settings sections, hidden from the generic
  // Properties dump below.
  const handledKeys = new Set(["comment", "owner", "rows", "bytes"]);

  // Add-index column choices come from the backend-computed eligible sets. A
  // column cannot be both a key and an INCLUDE column of the same index
  // (Snowflake rejects it as a duplicate), so each dropdown also hides what the
  // other has already selected.
  const keyColumnOptions = keyEligible
    .filter((n) => !newIdxInclude.includes(n))
    .map((n) => ({ value: n, label: n }));
  const includeColumnOptions = includeEligible
    .filter((n) => !newIdxCols.includes(n))
    .map((n) => ({ value: n, label: n }));

  // ── Index table ───────────────────────────────────────────────────────────
  const cols = indexes?.columns ?? [];
  const lower = cols.map((c) => c.toLowerCase());
  const nameCi = lower.indexOf("name");
  const colsCi = lower.indexOf("columns");
  const inclCi = lower.indexOf("included_columns");
  const uniqCi = lower.indexOf("is_unique");
  const indexData = (indexes?.rows ?? []).map((row, ri) => ({
    key: ri,
    idxName: nameCi >= 0 ? String(row[nameCi] ?? "") : "",
    columns: colsCi >= 0 ? String(row[colsCi] ?? "") : "",
    included: inclCi >= 0 ? String(row[inclCi] ?? "") : "",
    unique: uniqCi >= 0 ? String(row[uniqCi] ?? "") : "",
  }));
  const isUnique = (v: string) => v.toLowerCase() === "true" || v.toLowerCase() === "y";

  const indexColumns = [
    {
      title: "Index", dataIndex: "idxName", key: "idxName",
      render: (v: string, r: typeof indexData[number]) => (
        <Space size={4}>
          {isUnique(r.unique) && <KeyOutlined style={{ color: "var(--icon-hybridtable)", fontSize: 11 }} />}
          <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v}</span>
        </Space>
      ),
    },
    {
      title: "Columns", dataIndex: "columns", key: "columns",
      render: (v: string) => <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v}</span>,
    },
    {
      title: "Include", dataIndex: "included", key: "included",
      render: (v: string) => <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v}</span>,
    },
    {
      title: "Unique", dataIndex: "unique", key: "unique",
      render: (v: string) => isUnique(v) ? <Tag color="blue">UNIQUE</Tag> : null,
    },
    {
      title: "", key: "actions", width: 44,
      // The primary key surfaces here as the (only) UNIQUE index, and Snowflake
      // won't let you DROP INDEX the enforcing PK index — so offer Drop only on
      // secondary (non-unique) indexes.
      render: (_: unknown, r: typeof indexData[number]) => isUnique(r.unique) ? null : (
        <Popconfirm
          title="Drop this index?"
          description={`DROP INDEX ${r.idxName}`}
          okText="Drop"
          okButtonProps={{ danger: true }}
          onConfirm={() => dropIndex(r.idxName)}
        >
          <Tooltip title="Drop index">
            <Button type="text" size="small" danger icon={<DeleteOutlined style={{ fontSize: 11 }} />} />
          </Tooltip>
        </Popconfirm>
      ),
    },
  ];

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <MergeCellsOutlined style={{ color: "var(--icon-hybridtable)" }} />
          <span>Hybrid Table Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {tableRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={780}
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

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <InfoRow label="Owner" value={owner} />
              <InfoRow label="Rows" value={rowCount} />
              <InfoRow label="Bytes" value={bytes} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Indexes & Primary Key</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            The primary key is enforced as a unique index. Add secondary indexes to speed up point
            lookups on other columns.
          </Text>
          {indexesError && (
            <Alert type="error" message="Failed to load indexes" description={indexesError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Space style={{ marginBottom: 8 }}>
            <Button size="small" icon={<ReloadOutlined />} onClick={loadIndexes} loading={indexesLoading}>
              Refresh
            </Button>
            <Button size="small" type="primary" icon={<PlusOutlined />} onClick={() => setAdding((a) => !a)}>
              Add index
            </Button>
          </Space>
          {adding && (
            <Space align="start" style={{ width: "100%", marginBottom: 8 }} wrap>
              <Input size="small" placeholder="Index name" value={newIdxName} onChange={(e) => setNewIdxName(e.target.value)} style={{ width: 150 }} />
              <Select
                size="small"
                mode="multiple"
                placeholder="Key columns"
                value={newIdxCols}
                onChange={setNewIdxCols}
                options={keyColumnOptions}
                style={{ width: 220 }}
                notFoundContent="No eligible columns"
              />
              <Select
                size="small"
                mode="multiple"
                placeholder="Include (optional)"
                value={newIdxInclude}
                onChange={setNewIdxInclude}
                options={includeColumnOptions}
                style={{ width: 200 }}
                notFoundContent="No eligible columns"
              />
              <Checkbox
                checked={newIdxCaseSensitive}
                onChange={(e) => setNewIdxCaseSensitive(e.target.checked)}
              >
                <span style={{ fontSize: 12 }}>Case-sensitive name</span>
              </Checkbox>
              <Button
                size="small"
                type="primary"
                loading={creatingIdx}
                disabled={newIdxName.trim() === "" || newIdxCols.length === 0}
                onClick={createIndex}
              >
                Create
              </Button>
            </Space>
          )}
          {indexes && (
            <Table
              size="small"
              columns={indexColumns}
              dataSource={indexData}
              pagination={false}
              locale={{ emptyText: "No indexes found." }}
            />
          )}

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <InfoRow key={r.key} label={r.key} value={r.value} />
                ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
