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
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Switch, Tag,
} from "antd";
import {
  EyeOutlined, EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterView, GetObjectTagReferences } from "../../../wailsjs/go/app/App";
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

// Escape a SQL text literal the way the backend's EscapeTextLit does — double
// backslashes (Snowflake interprets backslash escapes in string literals) then
// single quotes — so a comment like C:\temp round-trips intact.
function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

function truthy(v: string): boolean {
  return /^(y|yes|true|1)$/i.test(v.trim());
}

// Build the RENAME TO target. A bare name stays in the current db/schema; a
// dotted name is treated as an already-qualified path and each part is quoted.
// ponytail: splits on ".", so a name literally containing a dot can't be moved
// this way — type the fully-quoted identifier in the SQL editor for that.
export function qualifyRename(input: string, db: string, schema: string): string {
  const t = input.trim();
  const quote = (p: string) => `"${p.replace(/"/g, '""')}"`;
  if (t.includes(".")) return t.split(".").map((p) => quote(p.trim())).join(".");
  return `${quote(db)}.${quote(schema)}.${quote(t)}`;
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

// ─── Tag editor ──────────────────────────────────────────────────────────────
// Tags whose LEVEL is not the object itself (inherited from the schema/database)
// are shown for context but can't be unset here — that has to happen where they
// were applied.

interface ViewTag { name: string; qualified: string; value: string; inherited: boolean }

interface TagsRowProps {
  tags: ViewTag[];
  onSetTag: (name: string, value: string) => Promise<void>;
  onUnsetTag: (qualified: string) => Promise<void>;
}

function TagsRow({ tags, onSetTag, onUnsetTag }: TagsRowProps) {
  const [newName, setNewName] = useState("");
  const [newValue, setNewValue] = useState("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const addTag = async () => {
    if (!newName.trim()) return;
    setSaving(true);
    setError(null);
    try {
      await onSetTag(newName.trim(), newValue.trim());
      setNewName("");
      setNewValue("");
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr>
      <td style={LABEL_TD}>Tags</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "top" }}>
        <Space direction="vertical" size={6} style={{ width: "100%" }}>
          <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
            {tags.length === 0 && <Text type="secondary" style={{ fontSize: 12 }}>(none)</Text>}
            {tags.map((t) => (
              <Tag
                key={t.qualified}
                closable={!t.inherited}
                onClose={async (e) => {
                  e.preventDefault();
                  await onUnsetTag(t.qualified);
                }}
              >
                {t.name}: {t.value}{t.inherited ? " (inherited)" : ""}
              </Tag>
            ))}
          </div>
          <Space>
            <Input
              size="small"
              value={newName}
              onChange={(e) => setNewName(e.target.value)}
              placeholder="Tag name"
              style={{ width: 140 }}
            />
            <Input
              size="small"
              value={newValue}
              onChange={(e) => setNewValue(e.target.value)}
              placeholder="Tag value"
              style={{ width: 160 }}
              onPressEnter={addTag}
            />
            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={addTag}
              loading={saving}
              disabled={!newName.trim()}
            >
              Add Tag
            </Button>
          </Space>
          {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
        </Space>
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
  const [tags, setTags] = useState<ViewTag[]>([]);

  // Tags use the no-latency INFORMATION_SCHEMA.TAG_REFERENCES read so a SET/UNSET
  // reflects immediately. Best-effort: SET/UNSET still work if the read fails.
  const reloadTags = useCallback(async () => {
    try {
      const t = await GetObjectTagReferences("VIEW", db, schema, name, "");
      const cols = (t?.columns ?? []).map((c) => c.toLowerCase());
      const ci = (n: string) => cols.indexOf(n);
      const dbI = ci("tag_database"), scI = ci("tag_schema"), nmI = ci("tag_name"),
        vlI = ci("tag_value"), lvI = ci("level");
      const parsed = (t?.rows ?? []).map((row): ViewTag => {
        const tdb = dbI >= 0 ? String(row[dbI] ?? "") : "";
        const tsc = scI >= 0 ? String(row[scI] ?? "") : "";
        const tnm = nmI >= 0 ? String(row[nmI] ?? "") : "";
        const qualified = [tdb, tsc, tnm].filter(Boolean).map((p) => `"${p.replace(/"/g, '""')}"`).join(".");
        return {
          name: tnm,
          qualified,
          value: vlI >= 0 ? String(row[vlI] ?? "") : "",
          inherited: lvI >= 0 && String(row[lvI] ?? "").toUpperCase() !== "VIEW",
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

  // RENAME can move the view to another schema/db, so the modal's identity is no
  // longer valid afterwards — refresh the browser and close rather than track it.
  const rename = async (newName: string) => {
    if (!newName.trim() || newName.trim() === name) return;
    await AlterView(db, schema, name, `RENAME TO ${qualifyRename(newName, db, schema)}`);
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
  const handledKeys = new Set(["comment", "is_secure", "text", "change_tracking"]);

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
