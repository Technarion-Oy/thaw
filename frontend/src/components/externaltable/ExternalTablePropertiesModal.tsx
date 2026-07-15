// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tag, Tooltip,
} from "antd";
import {
  CloudServerOutlined, EditOutlined, CheckOutlined, CloseOutlined, SyncOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterExternalTable, ExecDDL } from "../../../wailsjs/go/app/App";
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

// Normalizes the assorted truthy representations SHOW EXTERNAL TABLES may use
// (true/false, Y/N, TRUE/FALSE) into a canonical "TRUE"/"FALSE"/"" for the
// Select editor.
function normBool(v: string): string {
  const t = v.trim().toLowerCase();
  if (t === "true" || t === "y" || t === "yes") return "TRUE";
  if (t === "false" || t === "n" || t === "no") return "FALSE";
  return "";
}

// ─── EditRow (free-text) ─────────────────────────────────────────────────────

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
              <Input size="small" value={draft} onChange={(e) => setDraft(e.target.value)} style={{ width: 280 }} onPressEnter={save} />
              <Tooltip title="Save"><Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} /></Tooltip>
              {canUnset && onUnset && (
                <Tooltip title="Unset (remove)"><Button size="small" onClick={unset} loading={saving}>Unset</Button></Tooltip>
              )}
              <Tooltip title="Cancel"><Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} /></Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">(not set)</Text>}</span>
            <Tooltip title="Edit">
              <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />} onClick={() => { setDraft(value); setEditing(true); }} style={{ color: "var(--text-muted)" }} />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

// ─── SelectEditRow ───────────────────────────────────────────────────────────

interface SelectEditRowProps {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  hint?: string;
  onSave: (val: string) => Promise<void>;
}

function SelectEditRow({ label, value, options, hint, onSave }: SelectEditRowProps) {
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
              <Select size="small" value={draft || undefined} onChange={(v) => setDraft(v ?? "")} style={{ width: 200 }} options={options} />
              <Tooltip title="Save"><Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={!draft} /></Tooltip>
              <Tooltip title="Cancel"><Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} /></Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">(not set)</Text>}</span>
            {hint && <Text type="secondary" style={{ fontSize: 11 }}>{hint}</Text>}
            <Tooltip title="Edit">
              <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />} onClick={() => { setDraft(value); setEditing(true); }} style={{ color: "var(--text-muted)" }} />
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

export default function ExternalTablePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "EXTERNAL TABLE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const tableRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const runAction = async (clause: string, label: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterExternalTable(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  // The ALTER EXTERNAL TABLE grammar does not accept SET/UNSET COMMENT, so use
  // the general-purpose COMMENT ON TABLE statement (external tables are tables);
  // clearing the comment is COMMENT … IS ''.
  const saveComment = async (comment: string) => {
    await ExecDDL(`COMMENT ON TABLE ${tableRef} IS ${q1(comment.trim())}`);
    await reload();
  };

  const saveAutoRefresh = async (val: string) => {
    await AlterExternalTable(db, schema, name, `SET AUTO_REFRESH = ${val}`);
    await reload();
  };

  const comment = find("comment");
  // SHOW EXTERNAL TABLES does not reliably expose an `auto_refresh` column on
  // all editions. When it's present, use it; otherwise infer the state from
  // `notification_channel` — a non-empty channel means the auto-refresh event
  // notification pipe is wired up. The `SET AUTO_REFRESH = …` write is
  // unaffected either way.
  const autoRefreshRaw = find("auto_refresh");
  const notificationChannel = find("notification_channel");
  const autoRefreshInferred = autoRefreshRaw === "" && notificationChannel.trim() !== "";
  const autoRefresh =
    autoRefreshRaw !== ""
      ? normBool(autoRefreshRaw)
      : autoRefreshInferred
        ? "TRUE"
        : "";
  const invalid = find("invalid");

  // Keys handled by the editable Settings section. `notification_channel` is
  // left in the Properties table so the inferred Auto Refresh state is auditable.
  const handledKeys = new Set(["comment", "auto_refresh"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <CloudServerOutlined style={{ color: "var(--link)" }} />
          <span>External Table Properties</span>
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
        <div style={{ textAlign: "center", padding: 32 }}><Spin /></div>
      )}
      {error && (
        <Alert type="error" message="Failed to load properties" description={error} showIcon />
      )}
      {rows && (
        <>
          {actionError && (
            <Alert type="error" message={actionError} showIcon closable onClose={() => setActionError(null)} style={{ marginBottom: 12 }} />
          )}

          <Space wrap>
            {invalid && (
              <Tag color={/^(true|y|yes)$/i.test(invalid.trim()) ? "red" : "green"}>
                {/^(true|y|yes)$/i.test(invalid.trim()) ? "Invalid metadata" : "Valid"}
              </Tag>
            )}
            <Button size="small" icon={<SyncOutlined />} loading={busy} onClick={() => runAction("REFRESH", "Refresh")}>
              Refresh
            </Button>
          </Space>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <SelectEditRow
                label="Auto Refresh"
                value={autoRefresh}
                hint={autoRefreshInferred ? "(inferred from notification channel)" : undefined}
                options={[{ value: "TRUE", label: "TRUE" }, { value: "FALSE", label: "FALSE" }]}
                onSave={saveAutoRefresh}
              />
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
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
        </>
      )}
    </Modal>
  );
}
