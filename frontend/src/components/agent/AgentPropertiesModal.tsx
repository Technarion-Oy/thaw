// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip,
} from "antd";
import {
  ApiOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined, SaveOutlined,
  FolderOpenOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterAgent, DescribeAgent } from "../../../wailsjs/go/app/App";
import Editor from "@monaco-editor/react";
import StageFilePicker from "../shared/StageFilePicker";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { buildProfileJson, parseProfileJson } from "./profile";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Single-quote-escape a SQL string literal (backslashes doubled first, then
// single quotes) — matters for JSON profiles and comments. Mirrors the model /
// policy modals.
const q1 = (s: string) => `'${s.replace(/\\/g, "\\\\").replace(/'/g, "''")}'`;

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};
const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle", width: 200,
};

// ─── EditRow (single-line settings, e.g. comment) ─────────────────────────────
// ALTER AGENT has no UNSET, so this row only supports SET.

function EditRow({ label, value, placeholder, onSave }: {
  label: string; value: string; placeholder?: string; onSave: (val: string) => Promise<void>;
}) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    setSaving(true); setError(null);
    try { await onSave(draft); setEditing(false); }
    catch (e) { setError(String(e)); }
    finally { setSaving(false); }
  };

  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Input size="small" value={draft} placeholder={placeholder}
                onChange={(e) => setDraft(e.target.value)} style={{ width: 320 }} onPressEnter={save} />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} />
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
              <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />}
                onClick={() => { setDraft(value); setEditing(true); }} style={{ color: "var(--text-muted)" }} />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function AgentPropertiesModal({ db, schema, name, onClose }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Profile editor state.
  const [editingProfile, setEditingProfile] = useState(false);
  const [displayName, setDisplayName] = useState("");
  const [avatar, setAvatar] = useState("");
  const [color, setColor] = useState("");
  const [savingProfile, setSavingProfile] = useState(false);
  const [avatarBrowse, setAvatarBrowse] = useState(false);

  // Specification (DESCRIBE AGENT agent_spec) editor state.
  const [spec, setSpec] = useState<string | null>(null);
  const [specDraft, setSpecDraft] = useState("");
  const [specLoading, setSpecLoading] = useState(false);
  const [specError, setSpecError] = useState<string | null>(null);
  const [savingSpec, setSavingSpec] = useState(false);

  const reload = useCallback(async () => {
    setRows(null); setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "AGENT", name);
      setRows(props ?? []);
    } catch (e) { setError(String(e)); }
  }, [db, schema, name]);

  const loadSpec = useCallback(async () => {
    setSpecLoading(true); setSpecError(null);
    try {
      const res = await DescribeAgent(db, schema, name);
      const cols = res?.columns ?? [];
      const idx = cols.findIndex((c) => c.toLowerCase() === "agent_spec");
      const val = idx >= 0 && res?.rows?.[0] ? String(res.rows[0][idx] ?? "") : "";
      setSpec(val);
      setSpecDraft(val);
    } catch (e) {
      setSpecError(String(e));
      setSpec("");
    } finally { setSpecLoading(false); }
  }, [db, schema, name]);

  useEffect(() => { reload(); loadSpec(); }, [reload, loadSpec]);

  const agentRef = `"${db}"."${schema}"."${name}"`;
  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const alterAgent = async (clause: string) => { await AlterAgent(db, schema, name, clause); };

  const saveComment = async (comment: string) => {
    setActionError(null);
    try {
      // ALTER AGENT has no UNSET COMMENT; an empty value sets an empty comment.
      await alterAgent(`SET COMMENT = ${q1(comment)}`);
      await reload();
    } catch (e) { setActionError(`Update comment failed: ${String(e)}`); throw e; }
  };

  const comment = find("comment");
  const profileRaw = find("profile");
  const owner = find("owner");

  // Seed the profile editor from the current value whenever it opens.
  const beginEditProfile = () => {
    const p = parseProfileJson(profileRaw);
    setDisplayName(p.display_name); setAvatar(p.avatar); setColor(p.color);
    setEditingProfile(true);
  };

  const onPickAvatar = (stage: string, file: string) => {
    setAvatar(`@${stage}/${file}`);
    setAvatarBrowse(false);
  };

  const saveProfile = async () => {
    setSavingProfile(true); setActionError(null);
    try {
      const json = buildProfileJson({ display_name: displayName, avatar, color });
      // An empty profile still needs a valid JSON object literal for SET PROFILE.
      await alterAgent(`SET PROFILE = ${q1(json === "" ? "{}" : json)}`);
      setEditingProfile(false);
      await reload();
    } catch (e) { setActionError(`Update profile failed: ${String(e)}`); }
    finally { setSavingProfile(false); }
  };

  const saveSpec = async () => {
    setSavingSpec(true); setActionError(null);
    try {
      // Tagged $THAW$ dollar-quote so multi-line YAML/JSON needs no escaping and a
      // literal `$$` inside an instruction / tool description can't prematurely
      // close the block. The new specification completely replaces the live one.
      await alterAgent(`MODIFY LIVE VERSION SET SPECIFICATION = $THAW$\n${specDraft}\n$THAW$`);
      await loadSpec();
    } catch (e) { setActionError(`Update specification failed: ${String(e)}`); }
    finally { setSavingSpec(false); }
  };

  const handledKeys = new Set(["comment", "profile", "owner"]);
  const specDirty = spec !== null && specDraft !== spec;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ApiOutlined style={{ color: "var(--link)" }} />
          <span>Agent Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{agentRef}</Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={900}
      styles={{ body: { maxHeight: "78vh", overflowY: "auto", paddingTop: 16 } }}
    >
      {!rows && !error && (
        <div style={{ textAlign: "center", padding: 32 }}><Spin /></div>
      )}
      {error && <Alert type="error" message="Failed to load properties" description={error} showIcon />}
      {rows && (
        <>
          {actionError && (
            <Alert type="error" message={actionError} showIcon closable
              onClose={() => setActionError(null)} style={{ marginBottom: 12 }} />
          )}

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Owner</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
                  {owner || <Text type="secondary">(unknown)</Text>}
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="Comment" value={comment} onSave={saveComment} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Profile</div>
          {editingProfile ? (
            <Space direction="vertical" size={8} style={{ width: "100%", maxWidth: 560 }}>
              <Input addonBefore="display_name" size="small" value={displayName} onChange={(e) => setDisplayName(e.target.value)} />
              <Input
                addonBefore="avatar"
                size="small"
                value={avatar}
                onChange={(e) => setAvatar(e.target.value)}
                addonAfter={
                  <FolderOpenOutlined
                    title="Browse internal stage for an image"
                    style={{ cursor: "pointer" }}
                    onClick={() => setAvatarBrowse(true)}
                  />
                }
              />
              <Input addonBefore="color" size="small" value={color} onChange={(e) => setColor(e.target.value)} placeholder="color theme (e.g. blue)" />
              <Space>
                <Button size="small" type="primary" icon={<CheckOutlined />} onClick={saveProfile} loading={savingProfile}>Save profile</Button>
                <Button size="small" icon={<CloseOutlined />} onClick={() => setEditingProfile(false)}>Cancel</Button>
              </Space>
            </Space>
          ) : (
            <Space>
              <span style={{ fontFamily: "var(--font-mono)", fontSize: 11, color: "var(--text)", wordBreak: "break-all" }}>
                {profileRaw || <Text type="secondary">(not set)</Text>}
              </span>
              <Tooltip title="Edit profile">
                <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />}
                  onClick={beginEditProfile} style={{ color: "var(--text-muted)" }} />
              </Tooltip>
            </Space>
          )}

          <div style={SECTION_HEAD}>Specification (live version)</div>
          {specError && (
            <Alert type="warning" message="Could not read specification" description={specError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Space style={{ marginBottom: 8 }}>
            <Button size="small" icon={<ReloadOutlined />} onClick={loadSpec} loading={specLoading}>Reload</Button>
            <Button size="small" type="primary" icon={<SaveOutlined />} onClick={saveSpec} loading={savingSpec} disabled={!specDirty}>
              Save specification
            </Button>
            {specDirty && <Text type="secondary" style={{ fontSize: 11 }}>Unsaved changes</Text>}
          </Space>
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={300}
              language="yaml"
              theme={editorTheme}
              value={specDraft}
              onChange={(v) => setSpecDraft(v ?? "")}
              onMount={(editor) => { patchMonacoClipboard(editor); }}
              options={{
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
                readOnly: specLoading,
              }}
            />
          </div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 6 }}>
            Saving replaces the entire live specification — fields omitted from the new spec are removed.
          </Text>

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

      <Modal
        open={avatarBrowse}
        title="Select avatar image from an internal stage"
        onCancel={() => setAvatarBrowse(false)}
        footer={<Button onClick={() => setAvatarBrowse(false)}>Cancel</Button>}
        width={640}
        destroyOnClose
      >
        <StageFilePicker
          db={db}
          schema={schema}
          label="Browse internal stage — select the avatar image file"
          onPick={onPickAvatar}
        />
      </Modal>
    </Modal>
  );
}
