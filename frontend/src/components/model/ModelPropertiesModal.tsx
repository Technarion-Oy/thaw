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
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip, Table,
  Tag, Popconfirm, Dropdown, Form,
} from "antd";
import {
  RobotOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
  PlusOutlined, MoreOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterModel, ListModelVersions, GetModelTags } from "../../../wailsjs/go/app/App";
import ModelSourcePicker, { type ModelSourceValue } from "./ModelSourcePicker";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Single-quote-escape a SQL string literal. Snowflake processes backslash escape
// sequences inside single-quoted literals, so backslashes must be doubled too
// (escape backslashes first, then single quotes) — matters for JSON METADATA and
// Windows-style paths in comments. Mirrors passwordpolicy/sessionpolicy.
const q1 = (s: string) => `'${s.replace(/\\/g, "\\\\").replace(/'/g, "''")}'`;
// Double-quote a SQL identifier (doubles embedded double quotes).
const qId = (s: string) => `"${s.replace(/"/g, '""')}"`;

// Version-name quoting is split by clause, matching the ALTER MODEL grammar
// (https://docs.snowflake.com/en/sql-reference/sql/alter-model):
//   • SET DEFAULT_VERSION = '<name>'                    → string literal → q1
//   • DROP/MODIFY VERSION <name>, VERSION <name> SET/UNSET ALIAS → identifier → qId
// These can't be unified — DEFAULT_VERSION takes a literal, the rest take an
// identifier. The DROP/MODIFY/ALIAS paths feed qId the name from the SHOW VERSIONS
// `name` column (Snowflake's stored name), so the quoted identifier always matches.
// DEFAULT_VERSION is free-text user input (pre-filled with default_version_name);
// q1 just escapes it safely — Snowflake matches the literal against the stored
// name. New version names (ADD VERSION here, WITH VERSION in the create builder)
// are emitted UNQUOTED so they fold to uppercase like the registry's V1/V2.

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

// ─── EditRow (single-line settings, e.g. comment / default version) ──────────

interface EditRowProps {
  label: string;
  value: string;
  placeholder?: string;
  canUnset?: boolean;
  onSave: (val: string) => Promise<void>;
  onUnset?: () => Promise<void>;
}

function EditRow({ label, value, placeholder, canUnset, onSave, onUnset }: EditRowProps) {
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
                placeholder={placeholder}
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

// Per-version action that needs a single text input (rendered in a small modal).
type VersionInputAction = { version: string; kind: "alias" | "comment" | "metadata" };

export default function ModelPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Lazily-loaded version listing (SHOW VERSIONS IN MODEL).
  const [versions, setVersions] = useState<snowflake.QueryResult | null>(null);
  const [versionsLoading, setVersionsLoading] = useState(false);
  const [versionsError, setVersionsError] = useState<string | null>(null);

  // Tags (INFORMATION_SCHEMA.TAG_REFERENCES, object domain MODEL).
  const [tags, setTags] = useState<snowflake.QueryResult | null>(null);
  const [tagsLoading, setTagsLoading] = useState(false);
  const [tagsError, setTagsError] = useState<string | null>(null);
  const [newTagName, setNewTagName] = useState("");
  const [newTagValue, setNewTagValue] = useState("");

  // Per-version input action + the add-version dialog.
  const [versionInput, setVersionInput] = useState<VersionInputAction | null>(null);
  const [versionInputDraft, setVersionInputDraft] = useState("");
  const [versionInputBusy, setVersionInputBusy] = useState(false);
  const [versionInputError, setVersionInputError] = useState<string | null>(null);

  const [addOpen, setAddOpen] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "MODEL", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const modelRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const alterModel = async (clause: string) => {
    await AlterModel(db, schema, name, clause);
  };

  const saveComment = async (comment: string) => {
    setActionError(null);
    try {
      if (comment.trim() === "") {
        await alterModel("UNSET COMMENT");
      } else {
        await alterModel(`SET COMMENT = ${q1(comment)}`);
      }
      await reload();
    } catch (e) {
      setActionError(`Update comment failed: ${String(e)}`);
      throw e;
    }
  };

  const saveDefaultVersion = async (version: string) => {
    setActionError(null);
    const v = version.trim();
    if (v === "") return; // a model always has a default version; nothing to unset
    try {
      await alterModel(`SET DEFAULT_VERSION = ${q1(v)}`);
      await reload();
    } catch (e) {
      setActionError(`Update default version failed: ${String(e)}`);
      throw e;
    }
  };

  const loadVersions = useCallback(async () => {
    setVersionsLoading(true);
    setVersionsError(null);
    try {
      const res = await ListModelVersions(db, schema, name);
      setVersions(res ?? null);
    } catch (e) {
      setVersionsError(String(e));
    } finally {
      setVersionsLoading(false);
    }
  }, [db, schema, name]);

  const loadTags = useCallback(async () => {
    setTagsLoading(true);
    setTagsError(null);
    try {
      const res = await GetModelTags(db, schema, name);
      setTags(res ?? null);
    } catch (e) {
      setTagsError(String(e));
    } finally {
      setTagsLoading(false);
    }
  }, [db, schema, name]);

  // Load tags once on open (cheap, no governance latency on INFORMATION_SCHEMA).
  useEffect(() => { loadTags(); }, [loadTags]);

  const addTag = async () => {
    const n = newTagName.trim();
    if (n === "") return;
    setActionError(null);
    try {
      // Tag name may be qualified (db.schema.tag); insert verbatim, quote value.
      await alterModel(`SET TAG ${n} = ${q1(newTagValue)}`);
      setNewTagName("");
      setNewTagValue("");
      await loadTags();
    } catch (e) {
      setActionError(`Set tag failed: ${String(e)}`);
    }
  };

  const removeTag = async (qualifiedName: string) => {
    setActionError(null);
    try {
      await alterModel(`UNSET TAG ${qualifiedName}`);
      await loadTags();
    } catch (e) {
      setActionError(`Unset tag failed: ${String(e)}`);
    }
  };

  // Run a version-scoped clause, then refresh versions + the SHOW MODELS header
  // (aliases / default version may have changed).
  const runVersionClause = async (clause: string) => {
    setActionError(null);
    await alterModel(clause);
    await loadVersions();
    await reload();
  };

  const submitVersionInput = async () => {
    if (!versionInput) return;
    const v = qId(versionInput.version);
    const draft = versionInputDraft.trim();
    let clause = "";
    if (versionInput.kind === "alias") {
      if (draft === "") { setVersionInputError("Alias is required."); return; }
      // Aliases are arbitrary user-chosen labels, so qId preserves the exact
      // casing typed (a case-sensitive "prod") — intentionally unlike version
      // names, which are emitted unquoted to fold to the registry's V1/V2.
      clause = `VERSION ${v} SET ALIAS = ${qId(draft)}`;
    } else if (versionInput.kind === "comment") {
      clause = `MODIFY VERSION ${v} SET COMMENT = ${q1(versionInputDraft)}`;
    } else {
      clause = `MODIFY VERSION ${v} SET METADATA = ${q1(versionInputDraft)}`;
    }
    setVersionInputBusy(true);
    setVersionInputError(null);
    try {
      await runVersionClause(clause);
      setVersionInput(null);
      setVersionInputDraft("");
    } catch (e) {
      setVersionInputError(String(e));
    } finally {
      setVersionInputBusy(false);
    }
  };

  const comment = find("comment");
  const defaultVersion = find("default_version_name");
  const modelType = find("model_type");
  const aliases = find("aliases");

  // Keys surfaced by dedicated sections above the generic Properties table
  // (Overview: model_type/aliases; Settings: default_version_name/comment) — so
  // they aren't duplicated in the raw SHOW MODELS dump at the bottom.
  const handledKeys = new Set(["comment", "default_version_name", "model_type", "aliases"]);

  // ── Tags → chips ──────────────────────────────────────────────────────────
  const tagChips = (tags?.rows ?? []).map((row) => {
    const cols = tags?.columns ?? [];
    const cell = (label: string) => {
      const idx = cols.findIndex((c) => c.toLowerCase() === label);
      return idx >= 0 ? String(row[idx] ?? "") : "";
    };
    const tdb = cell("tag_database");
    const tsc = cell("tag_schema");
    const tnm = cell("tag_name");
    const tval = cell("tag_value");
    const qualified = [tdb, tsc, tnm].filter(Boolean).map(qId).join(".");
    return { qualified, label: `${tnm}${tval ? ` = ${tval}` : ""}` };
  }).filter((t) => t.qualified);

  // ── Versions → table ──────────────────────────────────────────────────────
  const versionCols = versions?.columns ?? [];
  const versionNameIdx = versionCols.findIndex((c) => c.toLowerCase() === "name");
  const versionColumns = versionCols.map((col, idx) => ({
    title: col,
    dataIndex: String(idx),
    key: String(idx),
    ellipsis: true,
    render: (v: unknown) => (
      <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v == null ? "" : String(v)}</span>
    ),
  }));
  const versionData = (versions?.rows ?? []).map((row, ri) => {
    const obj: Record<string, unknown> = { key: ri };
    row.forEach((cell, ci) => { obj[String(ci)] = cell; });
    obj.__version = versionNameIdx >= 0 ? String(row[versionNameIdx] ?? "") : "";
    return obj;
  });

  // Append an Actions column exposing every version-scoped ALTER MODEL clause.
  if (versionNameIdx >= 0) {
    versionColumns.push({
      title: "Actions",
      dataIndex: "__actions",
      key: "__actions",
      ellipsis: false,
      render: ((_: unknown, rec: Record<string, unknown>) => {
        const ver = String(rec.__version ?? "");
        if (!ver) return null;
        return (
          <Dropdown
            trigger={["click"]}
            menu={{
              items: [
                { key: "alias", label: "Set alias…" },
                { key: "unset-alias", label: "Unset alias" },
                { key: "comment", label: "Set comment…" },
                { key: "metadata", label: "Set metadata…" },
                { type: "divider" as const },
                { key: "drop", danger: true, label: "Drop version…" },
              ],
              onClick: ({ key }) => {
                if (key === "alias" || key === "comment" || key === "metadata") {
                  setVersionInputDraft("");
                  setVersionInputError(null);
                  setVersionInput({ version: ver, kind: key });
                } else if (key === "unset-alias") {
                  Modal.confirm({
                    title: `Unset alias for version "${ver}"?`,
                    onOk: () => runVersionClause(`VERSION ${qId(ver)} UNSET ALIAS`).catch((e) => setActionError(String(e))),
                  });
                } else if (key === "drop") {
                  Modal.confirm({
                    title: `Drop version "${ver}"?`,
                    content: "This permanently removes the version. The default version cannot be dropped.",
                    okButtonProps: { danger: true },
                    okText: "Drop",
                    onOk: () => runVersionClause(`DROP VERSION ${qId(ver)}`).catch((e) => setActionError(String(e))),
                  });
                }
              },
            }}
          >
            <Button size="small" type="text" icon={<MoreOutlined />} />
          </Dropdown>
        );
      }) as never,
    } as never);
  }

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <RobotOutlined style={{ color: "var(--link)" }} />
          <span>Model Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {modelRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={860}
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

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Model type</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>
                  {modelType || <Text type="secondary">(unknown)</Text>}
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Aliases</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
                  {aliases || <Text type="secondary">(none)</Text>}
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow
                label="Default version"
                value={defaultVersion}
                placeholder="V1"
                onSave={saveDefaultVersion}
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

          <div style={SECTION_HEAD}>Tags</div>
          {tagsError && (
            <Alert type="warning" message="Could not read current tags" description={tagsError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Space size={[6, 6]} wrap style={{ marginBottom: 8 }}>
            {tagsLoading && <Spin size="small" />}
            {!tagsLoading && tagChips.length === 0 && !tagsError && (
              <Text type="secondary" style={{ fontSize: 11 }}>No tags applied.</Text>
            )}
            {tagChips.map((t) => (
              <Popconfirm
                key={t.qualified}
                title="Unset this tag?"
                onConfirm={() => removeTag(t.qualified)}
                okText="Unset"
              >
                <Tag closable onClose={(e) => e.preventDefault()} style={{ cursor: "pointer" }}>
                  {t.label}
                </Tag>
              </Popconfirm>
            ))}
          </Space>
          <Space.Compact style={{ display: "flex", maxWidth: 560 }}>
            <Input
              size="small"
              placeholder="tag name (e.g. cost_center or DB.SCHEMA.TAG)"
              value={newTagName}
              onChange={(e) => setNewTagName(e.target.value)}
            />
            <Input
              size="small"
              placeholder="value"
              value={newTagValue}
              onChange={(e) => setNewTagValue(e.target.value)}
              onPressEnter={addTag}
            />
            <Button size="small" type="primary" icon={<PlusOutlined />} onClick={addTag}>Set tag</Button>
          </Space.Compact>

          <div style={SECTION_HEAD}>Versions</div>
          {versionsError && (
            <Alert type="error" message="Failed to load versions" description={versionsError} showIcon style={{ marginBottom: 8 }} />
          )}
          {versions ? (
            <>
              <Space style={{ marginBottom: 8 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {versionData.length === 0 ? "No versions in this model." : `${versionData.length} version${versionData.length === 1 ? "" : "s"}.`}
                </Text>
                <Button size="small" icon={<ReloadOutlined />} onClick={loadVersions} loading={versionsLoading}>
                  Refresh
                </Button>
                <Button size="small" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
                  Add version…
                </Button>
              </Space>
              {versionData.length > 0 && (
                <Table
                  size="small"
                  columns={versionColumns}
                  dataSource={versionData}
                  pagination={false}
                  scroll={{ x: true }}
                />
              )}
            </>
          ) : (
            <Space>
              <Button size="small" icon={<ReloadOutlined />} onClick={loadVersions} loading={versionsLoading}>
                Load versions
              </Button>
              <Button size="small" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
                Add version…
              </Button>
            </Space>
          )}

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

      {/* Per-version single-input action (set alias / set comment / set metadata). */}
      <Modal
        open={versionInput !== null}
        title={
          versionInput
            ? `${versionInput.kind === "alias" ? "Set alias" : versionInput.kind === "comment" ? "Set comment" : "Set metadata"} — version "${versionInput.version}"`
            : ""
        }
        onCancel={() => { setVersionInput(null); setVersionInputError(null); }}
        onOk={submitVersionInput}
        okText="Save"
        confirmLoading={versionInputBusy}
        destroyOnClose
      >
        {versionInput?.kind === "alias" ? (
          <Input
            value={versionInputDraft}
            onChange={(e) => setVersionInputDraft(e.target.value)}
            placeholder="PROD"
            onPressEnter={submitVersionInput}
          />
        ) : (
          <Input.TextArea
            value={versionInputDraft}
            onChange={(e) => setVersionInputDraft(e.target.value)}
            placeholder={versionInput?.kind === "metadata" ? '{"key": "value"}' : "comment"}
            rows={versionInput?.kind === "metadata" ? 5 : 3}
            style={versionInput?.kind === "metadata" ? { fontFamily: "var(--font-mono)" } : undefined}
          />
        )}
        {versionInputError && (
          <Text type="danger" style={{ fontSize: 11, display: "block", marginTop: 8 }}>{versionInputError}</Text>
        )}
      </Modal>

      {/* Add a version from an existing model or an internal stage. */}
      <AddVersionDialog
        open={addOpen}
        db={db}
        schema={schema}
        onCancel={() => setAddOpen(false)}
        onSubmit={async (clause) => {
          await runVersionClause(clause);
          setAddOpen(false);
        }}
      />
    </Modal>
  );
}

// ─── Add-version dialog ──────────────────────────────────────────────────────

interface AddVersionDialogProps {
  open: boolean;
  db: string;
  schema: string;
  onCancel: () => void;
  onSubmit: (clause: string) => Promise<void>;
}

const EMPTY_SOURCE: ModelSourceValue = { sourceType: "model", sourceModel: "", sourceVersion: "", stageLocation: "" };

function AddVersionDialog({ open, db, schema, onCancel, onSubmit }: AddVersionDialogProps) {
  const [version, setVersion] = useState("");
  const [source, setSource] = useState<ModelSourceValue>(EMPTY_SOURCE);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const reset = () => { setVersion(""); setSource(EMPTY_SOURCE); setError(null); };

  const canSubmit =
    version.trim() !== "" &&
    (source.sourceType === "stage" ? source.stageLocation.trim() !== "" : source.sourceModel.trim() !== "");

  const submit = async () => {
    if (!canSubmit) return;
    // Emit the new version name unquoted so Snowflake folds it to uppercase,
    // matching the CREATE MODEL … WITH VERSION builder (internal/model/sql.go) and
    // the registry's V1/V2 convention — otherwise typing `v2` here would create a
    // case-sensitive "v2" that diverges from versions created elsewhere.
    let clause = `ADD VERSION ${version.trim()} FROM `;
    if (source.sourceType === "stage") {
      clause += source.stageLocation.trim();
    } else {
      clause += `MODEL ${source.sourceModel.trim()}`;
      if (source.sourceVersion.trim() !== "") clause += ` VERSION ${source.sourceVersion.trim()}`;
    }
    setBusy(true);
    setError(null);
    try {
      await onSubmit(clause);
      reset();
    } catch (e) {
      setError(String(e));
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal
      open={open}
      title="Add model version"
      onCancel={() => { reset(); onCancel(); }}
      onOk={submit}
      okText="Add version"
      okButtonProps={{ disabled: !canSubmit }}
      confirmLoading={busy}
      destroyOnClose
    >
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message="In SQL you can only add a version by copying from an existing model version or loading serialized artifacts from an internal stage. Use the Snowpark ML Python API to create versions from scratch."
      />
      <Form layout="vertical" size="small">
        <Form.Item label="Version name" required style={{ marginBottom: 12 }}>
          <Input value={version} onChange={(e) => setVersion(e.target.value)} placeholder="V2" />
        </Form.Item>
        <ModelSourcePicker
          db={db}
          schema={schema}
          value={source}
          onChange={(patch) => setSource((prev) => ({ ...prev, ...patch }))}
        />
        {error && <Text type="danger" style={{ fontSize: 11, display: "block", marginTop: 8 }}>{error}</Text>}
      </Form>
    </Modal>
  );
}
