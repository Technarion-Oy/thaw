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
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Space, Typography, Alert, Tag, Tooltip, Switch,
} from "antd";
import {
  BlockOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PauseCircleOutlined, PlayCircleOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterMaterializedView } from "../../../wailsjs/go/app/App";
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

function truthy(v: string): boolean {
  return /^(y|yes|true|1)$/i.test(v.trim());
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

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function MaterializedViewPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [secureSaving, setSecureSaving] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "MATERIALIZED VIEW", name);
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
      await AlterMaterializedView(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterMaterializedView(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterMaterializedView(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const toggleSecure = async (next: boolean) => {
    setSecureSaving(true);
    setActionError(null);
    try {
      await AlterMaterializedView(db, schema, name, next ? "SET SECURE" : "UNSET SECURE");
      await reload();
    } catch (e) {
      setActionError(`${next ? "Set" : "Unset"} SECURE failed: ${String(e)}`);
    } finally {
      setSecureSaving(false);
    }
  };

  const comment = find("comment");
  const definingQuery = find("text");
  const isSecure = truthy(find("is_secure"));
  const invalid = truthy(find("invalid"));
  const invalidReason = find("invalid_reason");
  const behindBy = find("behind_by");

  // Keys handled by the editable Settings section or rendered elsewhere.
  const handledKeys = new Set(["comment", "is_secure", "text"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <BlockOutlined style={{ color: "var(--link)" }} />
          <span>Materialized View Properties</span>
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
            <Tag color={invalid ? "red" : "green"}>{invalid ? "Invalid" : "Valid"}</Tag>
            {behindBy && <Tag color="blue">Behind by: {behindBy}</Tag>}
            <Button size="small" icon={<PauseCircleOutlined />} loading={busy} onClick={() => runAction("SUSPEND", "Suspend")}>
              Suspend
            </Button>
            <Button size="small" icon={<PlayCircleOutlined />} loading={busy} onClick={() => runAction("RESUME", "Resume")}>
              Resume
            </Button>
          </Space>

          {invalid && invalidReason && (
            <Alert type="warning" message={invalidReason} showIcon style={{ marginTop: 12 }} />
          )}

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
              <tr>
                <td style={LABEL_TD}>Secure</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Switch
                    size="small"
                    checked={isSecure}
                    loading={secureSaving}
                    onChange={toggleSecure}
                  />
                </td>
              </tr>
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
