// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Switch, Tag,
} from "antd";
import {
  EyeOutlined, EditOutlined, CheckOutlined, CloseOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterView, GetObjectTagReferences } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import TagsRow, { EditableTag } from "../shared/TagsRow";
import { quoteIdent, identToken } from "../shared/ObjectNameCaseControl";

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

// Escape a SQL text literal the way the backend's EscapeTextLit does — double
// backslashes (Snowflake interprets backslash escapes in string literals) then
// single quotes — so a comment like C:\temp round-trips intact.
function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

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
  onSuccess?: () => void;
}

export default function ViewPropertiesModal({ db, schema, name, onClose, onSuccess }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [secureSaving, setSecureSaving] = useState(false);
  const [ctSaving, setCtSaving] = useState(false);
  const [tags, setTags] = useState<EditableTag[]>([]);

  // Tags use the no-latency INFORMATION_SCHEMA.TAG_REFERENCES read so a SET/UNSET
  // reflects immediately. Best-effort: SET/UNSET still work if the read fails.
  // Tags whose LEVEL is not the object itself (inherited from schema/database)
  // are shown for context but can't be unset here — that has to happen where
  // they were applied.
  const reloadTags = useCallback(async () => {
    try {
      const t = await GetObjectTagReferences("VIEW", db, schema, name, "");
      const cols = (t?.columns ?? []).map((c) => c.toLowerCase());
      const ci = (n: string) => cols.indexOf(n);
      const dbI = ci("tag_database"), scI = ci("tag_schema"), nmI = ci("tag_name"),
        vlI = ci("tag_value"), lvI = ci("level");
      const parsed = (t?.rows ?? []).map((row): EditableTag => {
        const tdb = dbI >= 0 ? String(row[dbI] ?? "") : "";
        const tsc = scI >= 0 ? String(row[scI] ?? "") : "";
        const tnm = nmI >= 0 ? String(row[nmI] ?? "") : "";
        const qualified = [tdb, tsc, tnm].filter(Boolean).map(quoteIdent).join(".");
        const inherited = lvI >= 0 && String(row[lvI] ?? "").toUpperCase() !== "VIEW";
        return {
          key: qualified,
          name: tnm,
          value: vlI >= 0 ? String(row[vlI] ?? "") : "",
          removable: !inherited,
          suffix: inherited ? " (inherited)" : "",
        };
      });
      setTags(parsed);
    } catch {
      setTags([]);
    }
  }, [db, schema, name]);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "VIEW", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
    reloadTags();
  }, [db, schema, name, reloadTags]);

  useEffect(() => { reload(); }, [reload]);

  const tableRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterView(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterView(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const toggleSecure = async (next: boolean) => {
    setSecureSaving(true);
    setActionError(null);
    try {
      await AlterView(db, schema, name, next ? "SET SECURE" : "UNSET SECURE");
      await reload();
    } catch (e) {
      setActionError(`${next ? "Set" : "Unset"} SECURE failed: ${String(e)}`);
    } finally {
      setSecureSaving(false);
    }
  };

  const setChangeTracking = async (value: string) => {
    setCtSaving(true);
    setActionError(null);
    try {
      await AlterView(db, schema, name, `SET CHANGE_TRACKING = ${value}`);
      await reload();
    } catch (e) {
      setActionError(`Change tracking update failed: ${String(e)}`);
    } finally {
      setCtSaving(false);
    }
  };

  // In-place rename within the same schema — mirrors the sidebar's Rename dialog
  // (same-schema, identToken folding by default) so both entry points produce the
  // same stored identifier. The modal's identity changes, so refresh the browser
  // and close rather than track the new name.
  const rename = async (newName: string) => {
    const t = newName.trim();
    if (!t || t === name) return;
    const target = `${quoteIdent(db)}.${quoteIdent(schema)}.${identToken(t, false)}`;
    await AlterView(db, schema, name, `RENAME TO ${target}`);
    onSuccess?.();
    onClose();
  };

  const setTag = async (tagName: string, tagValue: string) => {
    // Tag name may be a qualified identifier (db.schema.tag) — inserted verbatim;
    // the value is a quoted string literal.
    await AlterView(db, schema, name, `SET TAG ${tagName} = ${q1(tagValue)}`);
    await reloadTags();
  };

  const unsetTag = async (qualified: string) => {
    await AlterView(db, schema, name, `UNSET TAG ${qualified}`);
    await reloadTags();
  };

  const comment = find("comment");
  const definingQuery = find("text");
  const isSecure = truthy(find("is_secure"));
  const changeTracking = find("change_tracking");
  const ctOn = /^(on|true)$/i.test(changeTracking.trim());

  // Keys handled by the editable Settings section or rendered elsewhere.
  const handledKeys = new Set(["name", "comment", "is_secure", "text", "change_tracking"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <EyeOutlined style={{ color: "var(--link)" }} />
          <span>View Properties</span>
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

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow
                label="Rename to"
                value={name}
                onSave={rename}
              />
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
              <tr>
                <td style={LABEL_TD}>Change tracking</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space>
                    <Tag color={ctOn ? "green" : "default"}>{ctOn ? "ON" : "OFF"}</Tag>
                    <Select
                      size="small"
                      value={ctOn ? "TRUE" : "FALSE"}
                      onChange={setChangeTracking}
                      loading={ctSaving}
                      style={{ width: 100 }}
                      options={[{ value: "TRUE", label: "On" }, { value: "FALSE", label: "Off" }]}
                    />
                  </Space>
                </td>
              </tr>
              <TagsRow tags={tags} onSetTag={setTag} onUnsetTag={unsetTag} />
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
