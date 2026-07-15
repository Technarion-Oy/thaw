// SPDX-License-Identifier: GPL-3.0-or-later

import React, { useState, useEffect, useCallback } from "react";
import {
  Modal, Input, Button, Alert, Space, Typography, Checkbox, Select,
  Spin, Tag, message,
} from "antd";
import {
  InboxOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PlusOutlined, SyncOutlined, SearchOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, ListIntegrations, AlterStage,
  ListFileFormats,
} from "../../../wailsjs/go/app/App";
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
  width: 200,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }

// ─── EditRow component ────────────────────────────────────────────────────────

interface RowProps {
  label:     string;
  value:     string;
  type:      "text" | "number" | "select" | "checkbox";
  options?:  { label: string; value: string }[];
  hint?:     string;
  canUnset?: boolean;
  search?:   string;
  onSave:    (val: any) => Promise<void>;
  onUnset?:  () => Promise<void>;
}

function EditRow({ label, value, type, options, hint, canUnset, search, onSave, onUnset }: RowProps) {
  const [editing,   setEditing]   = useState(false);
  const [editVal,   setEditVal]   = useState(value);
  const [saving,    setSaving]    = useState(false);
  const [editError, setEditError] = useState<string | null>(null);
  const [unsetting, setUnsetting] = useState(false);

  useEffect(() => { setEditVal(value); }, [value]);

  const startEdit = () => { setEditing(true); setEditVal(value); setEditError(null); };
  const cancel    = () => { setEditing(false); setEditError(null); };

  const save = async () => {
    setSaving(true); setEditError(null);
    try { await onSave(editVal); setEditing(false); }
    catch (e) {
      const raw = String(e);
      const m = raw.match(/Insufficient privileges[^\n]*/i) ?? raw.match(/:\s*(.+)$/s);
      setEditError(m ? m[0].trim() : raw);
    } finally { setSaving(false); }
  };

  const doUnset = async () => {
    if (!onUnset) return;
    setUnsetting(true);
    try { await onUnset(); }
    catch (e) { message.error(String(e)); }
    finally { setUnsetting(false); }
  };

  const showSection = (labels: string[]) => {
    if (!search) return true;
    return labels.some(l => l.toLowerCase().includes(search.toLowerCase()));
  };

  if (!showSection([label])) return null;

  if (type === "checkbox") {
    return (
      <tr style={{ borderBottom: "1px solid var(--border)" }}>
        <td style={LABEL_TD}>{label}</td>
        <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <Checkbox
              checked={value === "true"}
              onChange={async (e) => {
                setSaving(true);
                try { await onSave(e.target.checked); }
                catch (err) { message.error(String(err)); }
                finally { setSaving(false); }
              }}
              disabled={saving}
            />
            {saving && <Spin size="small" style={{ marginLeft: 8 }} />}
          </div>
        </td>
      </tr>
    );
  }

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
        {editing ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              {type === "select" ? (
                <Select size="small" value={editVal} onChange={setEditVal}
                  options={options} style={{ minWidth: 180 }} autoFocus open />
              ) : (
                <Input size="small" value={editVal}
                  onChange={(e) => setEditVal(e.target.value)}
                  onPressEnter={save} autoFocus
                  style={{ fontFamily: "monospace", fontSize: 12 }}
                  title={hint} status={editError ? "error" : undefined} />
              )}
              <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={save} />
              <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancel} />
            </div>
            {editError && (
              <div style={{ color: "var(--error)", fontSize: 11, fontFamily: "monospace",
                lineHeight: 1.4, paddingLeft: 2 }}>
                {editError}
              </div>
            )}
          </div>
        ) : (
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{
              fontFamily: type === "select" ? undefined : "monospace",
              fontSize: 12,
              color: value ? "var(--text)" : "var(--text-faint)",
              fontStyle: value ? "normal" : "italic",
              flex: 1, wordBreak: "break-word",
            }}>
              {value || "—"}
            </span>
            <Button size="small" type="text" icon={<EditOutlined />} onClick={startEdit}
              style={{ flexShrink: 0, color: "var(--text-faint)" }} />
            {canUnset && value && onUnset && (
              <Button size="small" type="text" icon={<CloseOutlined />} loading={unsetting}
                onClick={doUnset} title="Unset" style={{ color: "var(--text-faint)" }} />
            )}
          </div>
        )}
      </td>
    </tr>
  );
}

// ─── ReadOnlyRow component ────────────────────────────────────────────────────

function ReadOnlyRow({ label, value, search }: { label: string; value: string; search?: string }) {
  if (search && !label.toLowerCase().includes(search.toLowerCase())) return null;
  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
        <span style={{ fontFamily: "monospace", fontSize: 12, color: value ? "var(--text)" : "var(--text-faint)", fontStyle: value ? "normal" : "italic", wordBreak: "break-all" }}>
          {value || "—"}
        </span>
      </td>
    </tr>
  );
}

// ─── TagsRow component ────────────────────────────────────────────────────────

function TagsRow({ db, schema, name, search, onAlter }: { db: string, schema: string, name: string, search?: string, onAlter: (c: string) => Promise<void> }) {
  const [tags, setTags] = useState<{ key: string, value: string }[]>([]);
  const [newTag, setNewTag] = useState({ key: "", value: "" });

  const loadTags = useCallback(async () => {
    try {
      // For now we use a simplified approach since TAG_REFERENCES might be slow or restricted.
      // In a real app we'd fetch them properly.
    } catch (e) {}
  }, [db, schema, name]);

  useEffect(() => { loadTags(); }, [loadTags]);

  const addTag = async () => {
    if (!newTag.key || !newTag.value) return;
    try {
      await onAlter(`SET TAG ${newTag.key} = '${newTag.value}'`);
      setTags([...tags, { ...newTag }]);
      setNewTag({ key: "", value: "" });
    } catch (e) { message.error(String(e)); }
  };

  const removeTag = async (key: string) => {
    try {
      await onAlter(`UNSET TAG ${key}`);
      setTags(tags.filter(t => t.key !== key));
    } catch (e) { message.error(String(e)); }
  };

  if (search && !"tags".includes(search.toLowerCase())) return null;

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={{ ...LABEL_TD, verticalAlign: "top", paddingTop: 8 }}>Tags</td>
      <td style={{ padding: "4px 0" }}>
        <Space direction="vertical" style={{ width: "100%" }}>
          {tags.map(t => (
            <Tag key={t.key} closable onClose={() => removeTag(t.key)} style={{ marginBottom: 4 }}>
              {t.key} = {t.value}
            </Tag>
          ))}
          <div style={{ display: "flex", gap: 4 }}>
            <Input size="small" placeholder="Key" value={newTag.key} onChange={e => setNewTag({ ...newTag, key: e.target.value })} />
            <Input size="small" placeholder="Value" value={newTag.value} onChange={e => setNewTag({ ...newTag, value: e.target.value })} />
            <Button size="small" icon={<PlusOutlined />} onClick={addTag} />
          </div>
        </Space>
      </td>
    </tr>
  );
}

// ─── Main modal ───────────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function StagePropertiesModal({ db, schema, name: initialName, onClose, onSuccess }: Props) {
  const [name, setName] = useState(initialName);
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [integrations, setIntegrations] = useState<snowflake.IntegrationRow[]>([]);
  const [fileFormats, setFileFormats] = useState<string[]>([]);
  const [refreshingDir, setRefreshingDir] = useState(false);
  const [search, setSearch] = useState("");

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const r = await GetObjectProperties(db, schema, "STAGE", name);
      setRows(r ?? []);
    } catch (e) {
      setLoadError(String(e));
      setRows([]);
    }
  }, [db, schema, name]);

  useEffect(() => {
    load();
    ListIntegrations("STORAGE").then(setIntegrations).catch(() => {});
    ListFileFormats(db, schema).then(setFileFormats).catch(() => {});
  }, [load, db, schema]);

  const get = (key: string): string => {
    const k = key.toUpperCase();
    return rows?.find((r) => r.key.toUpperCase() === k)?.value ?? "";
  };

  const alter = async (clause: string) => {
    await AlterStage(db, schema, name, clause);
    await load();
    onSuccess?.();
  };

  const handleRename = async (newName: string) => {
    const t = newName.trim();
    if (!t || t === name) return;
    await AlterStage(db, schema, name, `RENAME TO ${t}`);
    setName(t);
    message.success(`Stage renamed to ${t}`);
    onSuccess?.();
  };

  const handleRefreshDirectory = async () => {
    const subpath = window.prompt("Enter optional subpath to refresh (leave blank for all):");
    if (subpath === null) return;
    setRefreshingDir(true);
    try {
      const clause = subpath.trim() ? `REFRESH SUBPATH = '${subpath.trim()}'` : "REFRESH";
      await alter(clause);
      message.success("Directory table refreshed");
    } catch (e) {
      message.error(String(e));
    } finally {
      setRefreshingDir(false);
    }
  };

  const isExternal = get("is_external") === "true" || get("type") === "External Stage";

  const HANDLED_KEYS = new Set([
    "NAME", "DATABASE_NAME", "SCHEMA_NAME", "COMMENT",
    "URL", "STORAGE_INTEGRATION", "DIRECTORY_ENABLED",
    "OWNER", "CREATED_ON", "FORMAT_NAME", "FILE_FORMAT",
    "HAS_ENCRYPTION_KEY", "ENCRYPTION_TYPE", "KMS_KEY_ID", "USE_PRIVATELINK_ENDPOINT",
  ]);

  const readOnlyPairs = (rows || []).filter(r => !HANDLED_KEYS.has(r.key.toUpperCase()) && r.value && r.value !== "null");

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <InboxOutlined style={{ color: "var(--link)" }} />
          <span>Stage Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={640}
      styles={{ body: { maxHeight: "76vh", overflowY: "auto", paddingTop: 12 } }}
    >
      {loadError && (
        <Alert type="error" message={loadError} style={{ marginBottom: 12 }} showIcon />
      )}

      {rows === null && !loadError && (
        <div style={{ textAlign: "center", padding: 32 }}>
          <Spin tip="Loading…" />
        </div>
      )}

      {rows !== null && (
        <>
          <div style={{ marginBottom: 16 }}>
            <Input
              prefix={<SearchOutlined style={{ color: "var(--text-faint)" }} />}
              placeholder="Search properties by name…"
              allowClear
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>

          {/* ── Read-only info ────────────────────────────────────────────── */}
          <div style={{ display: "flex", gap: 24, marginBottom: 4 }}>
            {get("created_on") && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                Created: {get("created_on")}
              </Text>
            )}
            {get("owner") && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                Owner: {get("owner")}
              </Text>
            )}
          </div>

          {/* Section A: General Settings */}
          <div style={SECTION_HEAD}>General Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search} label="Name" value={name} type="text"
                onSave={handleRename}
              />
              <EditRow search={search} label="Comment" value={get("comment")} type="text" canUnset
                onSave={async (v) => await alter(`SET COMMENT = ${q1(v)}`)}
                onUnset={async () => await alter("UNSET COMMENT")}
              />
              <TagsRow search={search} db={db} schema={schema} name={name} onAlter={alter} />
              {(!search || "DCM Project".toLowerCase().includes(search.toLowerCase())) && (
                <tr style={{ borderBottom: "1px solid var(--border)" }}>
                  <td style={LABEL_TD}>DCM Project</td>
                  <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
                    <Button size="small" onClick={() => alter("UNSET DCM PROJECT")}>UNSET DCM PROJECT</Button>
                  </td>
                </tr>
              )}
            </tbody>
          </table>

          {/* Section B: File Format */}
          <div style={SECTION_HEAD}>File Format</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search}
                label="Format Name"
                value={get("file_format") || get("format_name")}
                type="select"
                options={fileFormats.map(f => ({ label: f, value: f }))}
                canUnset
                onSave={async (v) => await alter(`SET FILE_FORMAT = (FORMAT_NAME = ${v})`)}
                onUnset={async () => await alter("UNSET FILE_FORMAT")}
              />
            </tbody>
          </table>
          <div style={{ marginTop: 8, fontSize: 12, color: "var(--text-muted)" }}>
            Tip: To use an inline custom format, use the &quot;Create Stage&quot; designer or SQL editor.
          </div>

          {/* Section C: External Stage Parameters */}
          {isExternal && (
            <>
              <div style={SECTION_HEAD}>External Stage Parameters</div>
              <table style={{ width: "100%", borderCollapse: "collapse" }}>
                <tbody>
                  <EditRow search={search} label="URL" value={get("url")} type="text"
                    onSave={async (v) => await alter(`SET URL = '${v}'`)}
                  />
                  <EditRow search={search} label="Storage Integration" value={get("storage_integration")}
                    type="select"
                    options={integrations.map(i => ({ label: i.name, value: i.name }))}
                    canUnset
                    onSave={async (v) => await alter(`SET STORAGE_INTEGRATION = ${v}`)}
                    onUnset={async () => await alter("UNSET STORAGE_INTEGRATION")}
                  />
                  <EditRow search={search} label="Encryption Type" value={get("encryption_type")} type="select"
                    options={[
                      { label: "SNOWFLAKE_FULL", value: "SNOWFLAKE_FULL" },
                      { label: "SNOWFLAKE_SSE", value: "SNOWFLAKE_SSE" },
                      { label: "AWS_SSE_S3", value: "AWS_SSE_S3" },
                      { label: "AWS_SSE_KMS", value: "AWS_SSE_KMS" },
                      { label: "NONE", value: "NONE" },
                    ]}
                    onSave={async (v) => await alter(`SET ENCRYPTION = (TYPE = '${v}')`)}
                  />
                  <EditRow search={search} label="KMS Key ID" value={get("kms_key_id")} type="text" canUnset
                    onSave={async (v) => await alter(`SET ENCRYPTION = (KMS_KEY_ID = '${v}')`)}
                    onUnset={async () => await alter("UNSET ENCRYPTION")}
                  />
                  <EditRow search={search} label="PrivateLink" value={get("use_privatelink_endpoint")} type="checkbox"
                    onSave={async (v) => await alter(`SET USE_PRIVATELINK_ENDPOINT = ${v ? "TRUE" : "FALSE"}`)}
                  />
                </tbody>
              </table>
            </>
          )}

          {/* Section D: Directory Table Settings */}
          <div style={SECTION_HEAD}>Directory Table Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search} label="Enable Directory" value={get("directory_enabled")} type="checkbox"
                onSave={async (v) => await alter(`SET DIRECTORY = (ENABLE = ${v ? "TRUE" : "FALSE"})`)}
              />
              {(!search || "Actions".toLowerCase().includes(search.toLowerCase())) && (
                <tr>
                  <td style={LABEL_TD}>Actions</td>
                  <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
                    <Button
                      size="small"
                      icon={<SyncOutlined />}
                      loading={refreshingDir}
                      onClick={handleRefreshDirectory}
                      disabled={get("directory_enabled") !== "true"}
                    >
                      Refresh Directory
                    </Button>
                  </td>
                </tr>
              )}
            </tbody>
          </table>

          {readOnlyPairs.length > 0 && (
            <>
              <div style={SECTION_HEAD}>Additional Properties (Read-Only)</div>
              <table style={{ width: "100%", borderCollapse: "collapse" }}>
                <tbody>
                  {readOnlyPairs.map((r, i) => (
                    <ReadOnlyRow search={search} key={i} label={r.key} value={r.value} />
                  ))}
                </tbody>
              </table>
            </>
          )}
        </>
      )}
    </Modal>
  );
}

// @thaw-domain: Object Browser & Administration
