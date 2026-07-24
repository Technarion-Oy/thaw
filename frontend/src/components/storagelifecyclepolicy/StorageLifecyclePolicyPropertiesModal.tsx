// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, InputNumber, Select, Space, Typography, Alert, Tooltip, Table, Empty,
} from "antd";
import {
  HddOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterStorageLifecyclePolicy, GetStorageLifecyclePolicyReferences,
} from "../../../wailsjs/go/app/App";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";
import type { snowflake } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { setActiveSnippetEditor } from "../editor/SqlEditor";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

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

// ─── EditRow (single-line text settings, e.g. comment) ───────────────────────

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

// ─── ArchivingRow (archive tier + days) ──────────────────────────────────────
//
// Snowflake validates the *combined* state of ARCHIVE_TIER and ARCHIVE_FOR_DAYS:
// a tier with no days, or days with no tier, is rejected ("invalid property
// combination"). And once ARCHIVE_TIER is set it is *immutable* — it can neither
// be changed nor unset. So:
//   • when archiving is off, enabling it sets tier + days together in one ALTER;
//   • once a tier is set, the tier dropdown is locked and only the retention days
//     can be edited (re-issuing SET ARCHIVE_TIER — even to the same value — is
//     rejected, so we send SET ARCHIVE_FOR_DAYS alone).
function ArchivingRow({ tier, days, onSave }: {
  tier: string; days: string;
  onSave: (tier: string, days: number | null) => Promise<void>;
}) {
  const [editing, setEditing] = useState(false);
  const [draftTier, setDraftTier] = useState(tier);
  const [draftDays, setDraftDays] = useState<number | null>(days ? Number(days) : null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // The tier is immutable once committed — lock the dropdown when one is set.
  const tierLocked = tier !== "";
  const enabled = draftTier !== "";
  const minDays = draftTier === "COLD" ? 180 : 90;
  const canSave = !enabled || (draftDays !== null && draftDays >= minDays);

  const begin = () => {
    setDraftTier(tier);
    setDraftDays(days ? Number(days) : null);
    setError(null);
    setEditing(true);
  };

  const onTierChange = (v?: string) => {
    const t = v ?? "";
    setDraftTier(t);
    // Enabling a tier requires days; default to the per-tier minimum so the
    // pair is never half-set. Disabling clears the days.
    if (t === "") setDraftDays(null);
    else if (draftDays === null) setDraftDays(t === "COLD" ? 180 : 90);
  };

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSave(draftTier, draftDays);
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <tr>
      <td style={LABEL_TD}>Archiving</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Select
                size="small"
                value={draftTier === "" ? undefined : draftTier}
                placeholder="Disabled"
                allowClear={!tierLocked}
                disabled={tierLocked}
                style={{ width: 180 }}
                onChange={onTierChange}
                options={[
                  { value: "COOL", label: "COOL (min 90 days)" },
                  { value: "COLD", label: "COLD (min 180 days)" },
                ]}
              />
              <InputNumber
                size="small"
                value={draftDays}
                min={minDays}
                disabled={!enabled}
                placeholder={enabled ? String(minDays) : "—"}
                style={{ width: 140 }}
                onChange={(v) => setDraftDays(v === null || v === undefined ? null : Number(v))}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={!canSave} />
              </Tooltip>
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => {
                  setDraftTier(tier);
                  setDraftDays(days ? Number(days) : null);
                  setError(null);
                  setEditing(false);
                }} />
              </Tooltip>
            </Space>
            <Text type="secondary" style={{ fontSize: 11 }}>
              {tierLocked
                ? `The archive tier (${tier}) is immutable once set — only the retention days can be changed (min ${minDays} days).`
                : enabled
                  ? `Tier and retention days are set together (min ${minDays} days for ${draftTier}). The tier cannot be changed once set.`
                  : "Choosing a tier enables archiving (tier + retention days are set together); leave it as Disabled for no archiving."}
            </Text>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>
              {tier
                ? `${tier}${days ? ` · ${days} days` : ""}`
                : <Text type="secondary">Disabled (rows expire without archiving)</Text>}
            </span>
            <Tooltip title="Edit">
              <Button
                type="text"
                size="small"
                icon={<EditOutlined style={{ fontSize: 11 }} />}
                onClick={begin}
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

export default function StorageLifecyclePolicyPropertiesModal({ db, schema, name, onClose }: Props) {
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
      const props = await GetObjectProperties(db, schema, "STORAGE LIFECYCLE POLICY", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const policyRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const objTags = useObjectTags({
    kind: "STORAGE LIFECYCLE POLICY", db, schema, name,
    alter: (clause) => AlterStorageLifecyclePolicy(db, schema, name, clause),
  });

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterStorageLifecyclePolicy(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterStorageLifecyclePolicy(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const saveBody = async () => {
    setSavingBody(true);
    setActionError(null);
    try {
      // ALTER STORAGE LIFECYCLE POLICY ... SET BODY -> <expr> (the body is raw
      // SQL, not a string literal, so it is interpolated verbatim).
      await AlterStorageLifecyclePolicy(db, schema, name, `SET BODY -> ${bodyDraft}`);
      setEditingBody(false);
      await reload();
    } catch (e) {
      setActionError(`Update body failed: ${String(e)}`);
    } finally {
      setSavingBody(false);
    }
  };

  // ARCHIVE_TIER and ARCHIVE_FOR_DAYS must be set together — Snowflake rejects a
  // half-set pair — and ARCHIVE_TIER is immutable once set (re-issuing it, even to
  // the same value, errors with "cannot be modified after being set"). So:
  //   • disabling (no tier) → UNSET ARCHIVE_FOR_DAYS (no UNSET ARCHIVE_TIER exists);
  //   • tier unchanged from the current one → SET ARCHIVE_FOR_DAYS only;
  //   • newly enabling a tier → SET both together (ARCHIVE_TIER is an unquoted keyword).
  const saveArchiving = async (tier: string, days: number | null) => {
    if (tier === "" || days === null) {
      await AlterStorageLifecyclePolicy(db, schema, name, "UNSET ARCHIVE_FOR_DAYS");
    } else if (tier === archiveTier) {
      await AlterStorageLifecyclePolicy(db, schema, name, `SET ARCHIVE_FOR_DAYS = ${days}`);
    } else {
      await AlterStorageLifecyclePolicy(db, schema, name, `SET ARCHIVE_TIER = ${tier} ARCHIVE_FOR_DAYS = ${days}`);
    }
    await reload();
  };

  const loadReferences = async () => {
    setRefsLoading(true);
    setRefsError(null);
    try {
      const r = await GetStorageLifecyclePolicyReferences(db, schema, name);
      setRefs(r);
    } catch (e) {
      setRefsError(String(e));
    } finally {
      setRefsLoading(false);
    }
  };

  const comment = find("comment");
  const signature = find("signature");
  const returnType = find("return_type");
  const body = find("body");
  const archiveTier = find("archive_tier");
  const archiveForDays = find("archive_for_days");

  // Keys handled by dedicated sections above the generic Properties table.
  const handledKeys = new Set(["comment", "signature", "return_type", "body", "archive_tier", "archive_for_days"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <HddOutlined style={{ color: "var(--link)" }} />
          <span>Storage Lifecycle Policy Properties</span>
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

          <div style={SECTION_HEAD}>Definition</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Signature</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", fontFamily: "var(--font-mono)", wordBreak: "break-word" }}>
                  {signature || "()"}
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Returns</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", fontFamily: "var(--font-mono)" }}>
                  {returnType || "BOOLEAN"}
                </td>
              </tr>
            </tbody>
          </table>

          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", margin: "16px 0 8px" }}>
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
              height={140}
              language="sql"
              theme={editorTheme}
              value={editingBody ? bodyDraft : body}
              onChange={(v) => setBodyDraft(v ?? "")}
              onMount={(editor) => {
                patchMonacoClipboard(editor);
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

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <ArchivingRow tier={archiveTier} days={archiveForDays} onSave={saveArchiving} />
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Tags</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
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

          <div style={SECTION_HEAD}>References</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Tables this policy is applied to (from ACCOUNT_USAGE — requires
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
