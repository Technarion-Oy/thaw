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
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip, Table, Empty,
} from "antd";
import {
  EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined, ColumnWidthOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterProjectionPolicy, GetProjectionPolicyReferences } from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { setActiveSnippetEditor } from "../editor/SqlEditor";

const { Text } = Typography;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "top",
  width: 160,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }

// ─── EditRow (single-line settings, e.g. comment) ────────────────────────────

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

export default function ProjectionPolicyPropertiesModal({ db, schema, name, onClose }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Body editing — seeded from the loaded body; SET BODY -> <expr> on save.
  const [editingBody, setEditingBody] = useState(false);
  const [bodyDraft, setBodyDraft] = useState("");
  const [savingBody, setSavingBody] = useState(false);

  // References (where the policy is applied) — loaded on demand because the
  // ACCOUNT_USAGE view is slow and may be restricted.
  const [refs, setRefs] = useState<snowflake.QueryResult | null>(null);
  const [refsError, setRefsError] = useState<string | null>(null);
  const [refsLoading, setRefsLoading] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "PROJECTION POLICY", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const policyRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterProjectionPolicy(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterProjectionPolicy(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const saveBody = async () => {
    setSavingBody(true);
    setActionError(null);
    try {
      // ALTER PROJECTION POLICY ... SET BODY -> <expr> (the body is raw SQL, not
      // a string literal, so it is interpolated verbatim).
      await AlterProjectionPolicy(db, schema, name, `SET BODY -> ${bodyDraft}`);
      setEditingBody(false);
      await reload();
    } catch (e) {
      setActionError(`Update body failed: ${String(e)}`);
    } finally {
      setSavingBody(false);
    }
  };

  const loadReferences = async () => {
    setRefsLoading(true);
    setRefsError(null);
    try {
      const r = await GetProjectionPolicyReferences(db, schema, name);
      setRefs(r);
    } catch (e) {
      setRefsError(String(e));
    } finally {
      setRefsLoading(false);
    }
  };

  const comment = find("comment");
  const body = find("body");

  // Keys handled by dedicated sections above the generic Properties table.
  // signature / return_type are always () / PROJECTION_CONSTRAINT, so they add
  // no information and are hidden.
  const handledKeys = new Set(["comment", "signature", "return_type", "body"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ColumnWidthOutlined style={{ color: "var(--link)" }} />
          <span>Projection Policy Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {policyRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={820}
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

          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", margin: "8px 0 8px" }}>
            <div style={{ ...SECTION_HEAD, margin: 0 }}>Body</div>
            {!editingBody ? (
              <Button size="small" icon={<EditOutlined />} onClick={() => { setBodyDraft(body); setEditingBody(true); }}>
                Edit
              </Button>
            ) : (
              <Space>
                <Button size="small" icon={<CloseOutlined />} onClick={() => setEditingBody(false)}>Cancel</Button>
                <Button size="small" type="primary" icon={<CheckOutlined />} onClick={saveBody} loading={savingBody} disabled={bodyDraft.trim() === ""}>
                  Save
                </Button>
              </Space>
            )}
          </div>
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={160}
              language="sql"
              theme={editorTheme}
              value={editingBody ? bodyDraft : body}
              onChange={(v) => setBodyDraft(v ?? "")}
              onMount={(editor) => {
                patchMonacoClipboard(editor);
                // Register this editor as the active snippet target so the global
                // "SQL Snippets" context-menu commands insert here, not into the
                // main SQL editor behind the modal.
                editor.onContextMenu(() => setActiveSnippetEditor(editor));
                editor.onDidDispose(() => setActiveSnippetEditor(null));
              }}
              options={{
                readOnly: !editingBody,
                minimap: { enabled: false },
                lineNumbers: "off",
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
              }}
            />
          </div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 4 }}>
            Returns <code>PROJECTION_CONSTRAINT(ALLOW =&gt; true)</code> or{" "}
            <code>PROJECTION_CONSTRAINT(ALLOW =&gt; false)</code> — controls whether the protected column can be projected (selected).
          </Text>

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

          <div style={SECTION_HEAD}>References</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Columns this policy is applied to (from ACCOUNT_USAGE — requires
            governance privileges and may lag recent changes).
          </Text>
          {refsError && (
            <Alert type="warning" message="Could not load references" description={refsError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Button size="small" icon={<ReloadOutlined />} onClick={loadReferences} loading={refsLoading} style={{ marginBottom: 8 }}>
            {refs ? "Refresh references" : "Load references"}
          </Button>
          {refs && (
            refs.rows && refs.rows.length > 0 ? (
              <Table
                size="small"
                rowKey={(_r, i) => String(i)}
                pagination={refs.rows.length > 10 ? { pageSize: 10 } : false}
                columns={(refs.columns ?? []).map((c, ci) => ({
                  title: c,
                  dataIndex: ci,
                  key: String(ci),
                  ellipsis: true,
                  render: (v: unknown) => (v === null || v === undefined ? "" : String(v)),
                }))}
                dataSource={refs.rows.map((row) => {
                  const obj: Record<number, unknown> = {};
                  row.forEach((cell, ci) => { obj[ci] = cell; });
                  return obj;
                })}
              />
            ) : (
              <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No references found" />
            )
          )}
        </>
      )}
    </Modal>
  );
}
