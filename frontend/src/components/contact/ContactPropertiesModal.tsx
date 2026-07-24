// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Radio, Space, Typography, Alert, Tag, Tooltip,
} from "antd";
import {
  ContactsOutlined, EditOutlined, CheckOutlined, CloseOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterContact, ListUsers, FormatContactUsers, ParseSqlList,
} from "../../../wailsjs/go/app/App";
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

type Method = "users" | "email" | "url";

// ─── EditRow (comment) ───────────────────────────────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  onSave: (val: string) => Promise<void>;
}

function EditRow({ label, value, onSave }: EditRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(draft.trim());
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

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function ContactPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Contact-method editor state.
  const [editingMethod, setEditingMethod] = useState(false);
  const [savingMethod, setSavingMethod] = useState(false);
  const [method, setMethod] = useState<Method>("email");
  const [draftUsers, setDraftUsers] = useState<string[]>([]);
  const [draftEmail, setDraftEmail] = useState("");
  const [draftUrl, setDraftUrl] = useState("");

  const [userOptions, setUserOptions] = useState<string[]>([]);
  const [loadingUsers, setLoadingUsers] = useState(false);

  useEffect(() => {
    setLoadingUsers(true);
    ListUsers()
      .then((list) => setUserOptions((list ?? []).map((u) => u.name).filter(Boolean)))
      .catch(() => {})
      .finally(() => setLoadingUsers(false));
  }, []);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "CONTACT", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const objTags = useObjectTags({
    kind: "CONTACT", db, schema, name,
    alter: (clause) => AlterContact(db, schema, name, clause),
  });

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const email = find("email_distribution_list");
  const url = find("url");
  const usersRaw = find("users");
  const comment = find("comment");

  // Parse the users array cell (SHOW CONTACTS renders it like ["ALICE","BOB"])
  // via the shared backend tokenizer, which is quote/comma/bracket-safe.
  const [usersParsed, setUsersParsed] = useState<string[]>([]);
  useEffect(() => {
    if (!usersRaw.trim()) { setUsersParsed([]); return; }
    let cancelled = false;
    ParseSqlList(usersRaw)
      .then((toks) => { if (!cancelled) setUsersParsed(toks ?? []); })
      .catch(() => { if (!cancelled) setUsersParsed([]); });
    return () => { cancelled = true; };
  }, [usersRaw]);

  // Derive the contact's current method from whichever column is populated.
  const currentMethod: Method = email ? "email" : url ? "url" : "users";

  const beginEditMethod = () => {
    setMethod(currentMethod);
    setDraftEmail(email);
    setDraftUrl(url);
    setDraftUsers(usersParsed);
    setActionError(null);
    setEditingMethod(true);
  };

  const saveMethod = async () => {
    let clause = "";
    if (method === "email") {
      if (!draftEmail.trim()) return;
      clause = `SET EMAIL_DISTRIBUTION_LIST = ${q1(draftEmail.trim())}`;
    } else if (method === "url") {
      if (!draftUrl.trim()) return;
      clause = `SET URL = ${q1(draftUrl.trim())}`;
    } else {
      const clean = draftUsers.map((u) => u.trim()).filter(Boolean);
      if (clean.length === 0) return;
      clause = `SET USERS = ${await FormatContactUsers(clean)}`;
    }
    setSavingMethod(true);
    setActionError(null);
    try {
      await AlterContact(db, schema, name, clause);
      setEditingMethod(false);
      await reload();
    } catch (e) {
      setActionError(`Set contact method failed: ${String(e)}`);
    } finally {
      setSavingMethod(false);
    }
  };

  const saveComment = async (v: string) => {
    await AlterContact(db, schema, name, `SET COMMENT = ${q1(v)}`);
    await reload();
  };

  const contactRef = `"${db}"."${schema}"."${name}"`;

  // Keys rendered by the dedicated sections above the raw table. The contact
  // method columns and comment are surfaced in the Contact Method / Settings
  // sections; everything else — including entries_in_users — falls through to
  // the raw Properties table below.
  const handledKeys = new Set([
    "email_distribution_list", "url", "users", "comment",
  ]);

  const methodLabel = currentMethod === "email" ? "Email distribution list" : currentMethod === "url" ? "URL" : "Snowflake users";
  const methodValue =
    currentMethod === "email" ? email : currentMethod === "url" ? url : (usersParsed.join(", ") || usersRaw);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ContactsOutlined style={{ color: "var(--link)" }} />
          <span>Contact Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{contactRef}</Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={640}
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

          <Space>
            <Tag color="blue">{methodLabel}</Tag>
          </Space>

          <div style={SECTION_HEAD}>Contact Method</div>
          {editingMethod ? (
            <Space direction="vertical" size={8} style={{ width: "100%" }}>
              <Radio.Group value={method} onChange={(e) => setMethod(e.target.value)} optionType="button" buttonStyle="solid" size="small">
                <Radio value="email">Email distribution list</Radio>
                <Radio value="users">Snowflake users</Radio>
                <Radio value="url">URL</Radio>
              </Radio.Group>
              {method === "email" && (
                <Input size="small" value={draftEmail} onChange={(e) => setDraftEmail(e.target.value)} placeholder="support@example.com" style={{ width: 360 }} />
              )}
              {method === "url" && (
                <Input size="small" value={draftUrl} onChange={(e) => setDraftUrl(e.target.value)} placeholder="https://example.com/oncall" style={{ width: 360 }} />
              )}
              {/* mode="tags" (not "multiple") so a user name can still be typed
                  manually when SHOW USERS is unavailable. */}
              {method === "users" && (
                <Select
                  mode="tags"
                  size="small"
                  showSearch
                  loading={loadingUsers}
                  value={draftUsers}
                  onChange={(v) => setDraftUsers(v)}
                  placeholder="Select or type users…"
                  options={userOptions.map((u) => ({ value: u, label: u }))}
                  notFoundContent={loadingUsers ? "Loading…" : "No users found"}
                  style={{ width: 360 }}
                />
              )}
              <Space>
                <Button size="small" type="primary" icon={<CheckOutlined />} onClick={saveMethod} loading={savingMethod}>Save</Button>
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditingMethod(false); setActionError(null); }}>Cancel</Button>
              </Space>
              <Text type="secondary" style={{ fontSize: 11 }}>A contact has a single method; saving replaces the current one.</Text>
            </Space>
          ) : (
            <Space>
              <span style={{ color: "var(--text)", fontSize: 12 }}>
                <Text type="secondary" style={{ fontSize: 12 }}>{methodLabel}:</Text>{" "}
                {methodValue || <Text type="secondary">(not set)</Text>}
              </span>
              <Tooltip title="Edit">
                <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />} onClick={beginEditMethod} style={{ color: "var(--text-muted)" }} />
              </Tooltip>
            </Space>
          )}

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="Comment" value={comment} onSave={saveComment} />
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
        </>
      )}
    </Modal>
  );
}
