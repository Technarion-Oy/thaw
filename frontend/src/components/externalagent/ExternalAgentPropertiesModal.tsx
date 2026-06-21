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
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip,
} from "antd";
import {
  GlobalOutlined, EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterExternalAgent } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Single-quote-escape a SQL string literal (backslashes doubled first, then
// single quotes).
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

// ALTER EXTERNAL AGENT has no UNSET, so the comment row only supports SET.
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

export default function ExternalAgentPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Add-version dialog.
  const [addOpen, setAddOpen] = useState(false);
  const [newVersion, setNewVersion] = useState("");
  const [adding, setAdding] = useState(false);
  const [addError, setAddError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setRows(null); setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "EXTERNAL AGENT", name);
      setRows(props ?? []);
    } catch (e) { setError(String(e)); }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const agentRef = `"${db}"."${schema}"."${name}"`;
  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const alterAgent = async (clause: string) => { await AlterExternalAgent(db, schema, name, clause); };

  const saveComment = async (comment: string) => {
    setActionError(null);
    try {
      await alterAgent(`SET COMMENT = ${q1(comment)}`);
      await reload();
    } catch (e) { setActionError(`Update comment failed: ${String(e)}`); throw e; }
  };

  const addVersion = async () => {
    const v = newVersion.trim();
    if (v === "") { setAddError("Version name is required."); return; }
    setAdding(true); setAddError(null);
    try {
      // Version names are emitted unquoted so Snowflake folds them to uppercase,
      // matching the CREATE … WITH VERSION builder and the V1/V2 convention.
      await alterAgent(`ADD VERSION ${v}`);
      setAddOpen(false);
      setNewVersion("");
      await reload();
    } catch (e) { setAddError(String(e)); }
    finally { setAdding(false); }
  };

  const comment = find("comment");
  const versions = find("versions");
  const defaultVersion = find("default_version_name");
  const owner = find("owner");
  const handledKeys = new Set(["comment", "versions", "default_version_name", "owner"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <GlobalOutlined style={{ color: "var(--link)" }} />
          <span>External Agent Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{agentRef}</Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={820}
      styles={{ body: { maxHeight: "76vh", overflowY: "auto", paddingTop: 16 } }}
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
              <tr>
                <td style={LABEL_TD}>Default version</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
                  {defaultVersion || <Text type="secondary">(none)</Text>}
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

          <div style={SECTION_HEAD}>Versions</div>
          <Space direction="vertical" size={8} style={{ width: "100%" }}>
            <div style={{ fontFamily: "var(--font-mono)", fontSize: 11, color: "var(--text)", wordBreak: "break-word" }}>
              {versions || <Text type="secondary">(none)</Text>}
            </div>
            <Button size="small" icon={<PlusOutlined />} onClick={() => { setNewVersion(""); setAddError(null); setAddOpen(true); }}>
              Add version…
            </Button>
          </Space>

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
        open={addOpen}
        title="Add external agent version"
        onCancel={() => setAddOpen(false)}
        onOk={addVersion}
        okText="Add version"
        confirmLoading={adding}
        destroyOnClose
      >
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="A new version represents a different implementation — an alternative retriever, prompt, LLM, or inference configuration."
        />
        <Input value={newVersion} onChange={(e) => setNewVersion(e.target.value)} placeholder="V2" onPressEnter={addVersion} />
        {addError && <Text type="danger" style={{ fontSize: 11, display: "block", marginTop: 8 }}>{addError}</Text>}
      </Modal>
    </Modal>
  );
}
