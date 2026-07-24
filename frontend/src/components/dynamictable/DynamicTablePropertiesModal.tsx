// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tag, Tooltip,
} from "antd";
import {
  RetweetOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PauseCircleOutlined, PlayCircleOutlined, SyncOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterDynamicTable, ListWarehouses } from "../../../wailsjs/go/app/App";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";
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

// TARGET_LAG accepts either a string literal ('1 minute') or the bare keyword
// DOWNSTREAM. Render the clause accordingly.
function targetLagAssign(lag: string): string {
  const t = lag.trim();
  if (t.toUpperCase() === "DOWNSTREAM") return "SET TARGET_LAG = DOWNSTREAM";
  return `SET TARGET_LAG = ${q1(t)}`;
}

// ─── EditRow ─────────────────────────────────────────────────────────────────

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

// ─── SelectEditRow ───────────────────────────────────────────────────────────
// Like EditRow but edits via a Select sourced from a canonical option list, so
// the saved value is always a valid (correctly-cased) identifier rather than
// free-typed text.

interface SelectEditRowProps {
  label: string;
  value: string;
  options: string[];
  loading?: boolean;
  onSave: (val: string) => Promise<void>;
}

function SelectEditRow({ label, value, options, loading, onSave }: SelectEditRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    if (!draft) return;
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

  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Select
                size="small"
                showSearch
                loading={loading}
                value={draft || undefined}
                onChange={(v) => setDraft(v ?? "")}
                style={{ width: 280 }}
                placeholder="Select warehouse…"
                options={(options || []).map((n) => ({ value: n, label: n }))}
                notFoundContent={loading ? "Loading…" : "No warehouses found"}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={!draft} />
              </Tooltip>
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

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function DynamicTablePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  useEffect(() => {
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "DYNAMIC TABLE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const objTags = useObjectTags({
    kind: "DYNAMIC TABLE", db, schema, name,
    alter: (clause) => AlterDynamicTable(db, schema, name, clause),
  });

  const tableRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const runAction = async (clause: string, label: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterDynamicTable(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterDynamicTable(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterDynamicTable(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const saveTargetLag = async (lag: string) => {
    await AlterDynamicTable(db, schema, name, targetLagAssign(lag));
    await reload();
  };

  const saveWarehouse = async (wh: string) => {
    await AlterDynamicTable(db, schema, name, `SET WAREHOUSE = "${wh.replace(/"/g, '""')}"`);
    await reload();
  };

  const comment = find("comment");
  const targetLag = find("target_lag");
  const warehouse = find("warehouse");
  const definingQuery = find("text");
  const schedulingState = find("scheduling_state");

  // Keys handled by the editable Settings section or rendered elsewhere.
  const handledKeys = new Set(["comment", "target_lag", "warehouse", "text"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <RetweetOutlined style={{ color: "var(--link)" }} />
          <span>Dynamic Table Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {tableRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={720}
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

          <Space wrap>
            {schedulingState && (
              <Tag color={/active|running/i.test(schedulingState) ? "green" : "orange"}>
                {schedulingState}
              </Tag>
            )}
            <Button size="small" icon={<SyncOutlined />} loading={busy} onClick={() => runAction("REFRESH", "Refresh")}>
              Refresh
            </Button>
            <Button size="small" icon={<PauseCircleOutlined />} loading={busy} onClick={() => runAction("SUSPEND", "Suspend")}>
              Suspend
            </Button>
            <Button size="small" icon={<PlayCircleOutlined />} loading={busy} onClick={() => runAction("RESUME", "Resume")}>
              Resume
            </Button>
          </Space>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="Target Lag" value={targetLag} onSave={saveTargetLag} />
              <SelectEditRow
                label="Warehouse"
                value={warehouse}
                options={warehouses}
                loading={loadingWarehouses}
                onSave={saveWarehouse}
              />
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
              <TagsRow tags={objTags.tags} nameOptions={objTags.nameOptions} onSetTag={objTags.setTag} onUnsetTag={objTags.unsetTag} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <tr key={r.key}>
                    <td style={LABEL_TD}>{r.key}</td>
                    <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
                      {r.value || <Text type="secondary">(empty)</Text>}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>

          {definingQuery && (
            <>
              <div style={SECTION_HEAD}>Defining Query</div>
              <pre
                style={{
                  margin: 0,
                  padding: "10px 12px",
                  background: "var(--bg)",
                  border: "1px solid var(--border)",
                  borderRadius: 6,
                  color: "var(--text)",
                  fontSize: 11,
                  fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                }}
              >
                {definingQuery}
              </pre>
            </>
          )}
        </>
      )}
    </Modal>
  );
}
