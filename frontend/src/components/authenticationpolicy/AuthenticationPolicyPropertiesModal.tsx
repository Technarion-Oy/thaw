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
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Table, Empty,
} from "antd";
import {
  LoginOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, DescribeAuthenticationPolicy, AlterAuthenticationPolicy,
  GetAuthenticationPolicyReferences, FormatAuthPolicyList,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { MFAPolicyRow, PATPolicyRow, WorkloadIdentityPolicyRow, ClientPolicyRow } from "./PolicyBagRows";

const { Text } = Typography;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase", margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "top", width: 220,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

// Quote a free-text value as a single-quoted SQL literal. Mirrors the backend
// snowflake.EscapeTextLit (backslash doubled, single-quotes doubled).
function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

// Parse a DESCRIBE list cell into individual tokens. DESCRIBE AUTHENTICATION
// POLICY renders the list parameters as e.g. "[ALL]", "[PASSWORD, SAML]", or a
// bare value — strip the surrounding brackets/quotes, split on commas, drop blanks.
function parseList(raw: string): string[] {
  if (!raw) return [];
  let s = raw.trim();
  if (s.startsWith("[") && s.endsWith("]")) s = s.slice(1, -1);
  return s
    .split(",")
    .map((t) => t.trim().replace(/^['"]|['"]$/g, "").trim())
    .filter((t) => t !== "");
}

// Normalize a DESCRIBE scalar (e.g. MFA_ENROLLMENT) for comparison against the
// bare option strings. The exact DESCRIBE rendering isn't confirmed against a
// live account; if it comes back JSON-encoded the value carries surrounding
// quotes (and possibly brackets), which would match no option and show stray
// quotes — so strip them defensively.
function cleanScalar(raw: string): string {
  let s = raw.trim();
  if (s.startsWith("[") && s.endsWith("]")) s = s.slice(1, -1).trim();
  return s.replace(/^['"]|['"]$/g, "").trim();
}

// The list parameters, each paired with its ALTER keyword and (for the fixed
// enumerations) the option set offered in the tag editor. SECURITY_INTEGRATIONS
// is free-form (integration names) plus the ALL token.
interface ListMeta { keyword: string; label: string; options?: string[]; freeform?: boolean; }

const LISTS: ListMeta[] = [
  {
    keyword: "AUTHENTICATION_METHODS", label: "Authentication methods",
    options: ["ALL", "SAML", "PASSWORD", "OAUTH", "KEYPAIR", "PROGRAMMATIC_ACCESS_TOKEN", "WORKLOAD_IDENTITY"],
  },
  {
    keyword: "CLIENT_TYPES", label: "Client types",
    options: ["ALL", "SNOWFLAKE_UI", "DRIVERS", "SNOWFLAKE_CLI", "SNOWSQL"],
  },
  { keyword: "SECURITY_INTEGRATIONS", label: "Security integrations", options: ["ALL"], freeform: true },
];

const MFA_ENROLLMENT_OPTIONS = ["REQUIRED", "REQUIRED_PASSWORD_ONLY", "OPTIONAL"];

// ─── ListRow (token-list setting with Set / Unset) ───────────────────────────

interface ListRowProps {
  meta: ListMeta;
  value: string[];
  onSet: (tokens: string[]) => Promise<void>;
  onUnset: () => Promise<void>;
}

function ListRow({ meta, value, onSet, onUnset }: ListRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<string[]>(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // An all-blank draft would serialize to SET … = (), which Snowflake rejects —
  // Save is disabled and the user is steered to Unset.
  const draftEmpty = draft.filter((t) => t.trim() !== "").length === 0;

  const save = async () => {
    setSaving(true);
    setError(null);
    try {
      await onSet(draft);
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const unset = async () => {
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
      <td style={LABEL_TD}>{meta.label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space align="start">
              <Select
                size="small"
                mode={meta.freeform ? "tags" : "multiple"}
                value={draft}
                onChange={setDraft}
                placeholder={meta.freeform ? "ALL or integration names" : "select methods"}
                tokenSeparators={[","]}
                style={{ width: 320 }}
                options={(meta.options ?? []).map((o) => ({ value: o, label: o }))}
              />
              <Tooltip title={draftEmpty ? "Use Unset to clear — an empty list is not valid SQL" : "Save"}>
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={draftEmpty} />
              </Tooltip>
              <Tooltip title="Reset to Snowflake default (ALL)">
                <Button size="small" onClick={unset} loading={saving}>Unset</Button>
              </Tooltip>
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} />
              </Tooltip>
            </Space>
            {draftEmpty && (
              <Text type="secondary" style={{ fontSize: 11 }}>Empty list — use <em>Unset</em> to restore the default.</Text>
            )}
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)", fontFamily: "var(--font-mono)" }}>
              {value.length > 0 ? value.join(", ") : <Text type="secondary">(default: ALL)</Text>}
            </span>
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

// ─── EnumRow (single-choice setting with Set / Unset) ────────────────────────

interface EnumRowProps {
  label: string;
  value: string;
  options: string[];
  def: string;
  onSet: (val: string) => Promise<void>;
  onUnset: () => Promise<void>;
}

function EnumRow({ label, value, options, def, onSet, onUnset }: EnumRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<string>(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    if (!draft) return;
    setSaving(true);
    setError(null);
    try {
      await onSet(draft);
      setEditing(false);
    } catch (e) {
      setError(String(e));
    } finally {
      setSaving(false);
    }
  };

  const unset = async () => {
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
              <Select
                size="small"
                value={draft || undefined}
                onChange={setDraft}
                placeholder="choose"
                style={{ width: 220 }}
                options={options.map((o) => ({ value: o, label: o }))}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={!draft} />
              </Tooltip>
              <Tooltip title="Reset to Snowflake default">
                <Button size="small" onClick={unset} loading={saving}>Unset</Button>
              </Tooltip>
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} />
              </Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)", fontFamily: "var(--font-mono)" }}>
              {value || <Text type="secondary">(default: {def})</Text>}
            </span>
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

// ─── EditRow (single-line text setting, e.g. comment) ────────────────────────

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
                style={{ width: 320 }}
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

export default function AuthenticationPolicyPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // DESCRIBE AUTHENTICATION POLICY result (one row per property: property/value).
  const [desc, setDesc] = useState<snowflake.QueryResult | null>(null);

  // References (users/account the policy is attached to) — loaded on demand
  // because the ACCOUNT_USAGE view is slow and may be restricted.
  const [refs, setRefs] = useState<snowflake.QueryResult | null>(null);
  const [refsError, setRefsError] = useState<string | null>(null);
  const [refsLoading, setRefsLoading] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const [props, d] = await Promise.all([
        GetObjectProperties(db, schema, "AUTHENTICATION POLICY", name),
        DescribeAuthenticationPolicy(db, schema, name).catch(() => null),
      ]);
      setRows(props ?? []);
      setDesc(d);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  // Index the DESCRIBE rows by the `property` column (lowercased) → value string.
  // DESCRIBE returns one row per property with property/value columns.
  const descByProp: Record<string, string> = {};
  if (desc && desc.columns && desc.rows) {
    const cols = desc.columns.map((c) => c.toLowerCase());
    const pi = cols.indexOf("property");
    const vi = cols.indexOf("value");
    if (pi >= 0 && vi >= 0) {
      desc.rows.forEach((row) => {
        const prop = pi < row.length && row[pi] != null ? String(row[pi]) : "";
        const val = vi < row.length && row[vi] != null ? String(row[vi]) : "";
        if (prop) descByProp[prop.toLowerCase()] = val;
      });
    }
  }

  const descFailed = desc === null;

  const setList = async (keyword: string, tokens: string[]) => {
    await AlterAuthenticationPolicy(db, schema, name, `SET ${keyword} = ${await FormatAuthPolicyList(tokens)}`);
    await reload();
  };
  const unsetParam = async (keyword: string) => {
    await AlterAuthenticationPolicy(db, schema, name, `UNSET ${keyword}`);
    await reload();
  };
  // MFA_ENROLLMENT is interpolated bare into the ALTER clause, so the value is
  // restricted to the known keywords before it reaches the SQL — the EnumRow
  // Select only offers these, this is defense-in-depth against an unexpected value.
  const setEnum = async (keyword: string, val: string) => {
    if (!MFA_ENROLLMENT_OPTIONS.includes(val)) return;
    await AlterAuthenticationPolicy(db, schema, name, `SET ${keyword} = ${val}`);
    await reload();
  };
  // Property-bag setter: `value` is the `( … )` clause already serialized by the
  // backend Build*Value (the bag editors never build SQL themselves).
  const setBag = async (keyword: string, value: string) => {
    await AlterAuthenticationPolicy(db, schema, name, `SET ${keyword} = ${value}`);
    await reload();
  };
  const unsetBag = (keyword: string) => async () => { await unsetParam(keyword); };
  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterAuthenticationPolicy(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterAuthenticationPolicy(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  // DCM PROJECT is UNSET-only (detaches the policy from a Declarative Change
  // Management project that manages it). There is no corresponding SET clause.
  const [dcmBusy, setDcmBusy] = useState(false);
  const unsetDcmProject = async () => {
    setDcmBusy(true);
    setActionError(null);
    try {
      await AlterAuthenticationPolicy(db, schema, name, "UNSET DCM PROJECT");
      await reload();
    } catch (e) {
      setActionError(String(e));
    } finally {
      setDcmBusy(false);
    }
  };

  const loadReferences = async () => {
    setRefsLoading(true);
    setRefsError(null);
    try {
      const r = await GetAuthenticationPolicyReferences(db, schema, name);
      setRefs(r);
    } catch (e) {
      setRefsError(String(e));
    } finally {
      setRefsLoading(false);
    }
  };

  const policyRef = `"${db}"."${schema}"."${name}"`;
  // Comment is read from DESCRIBE for consistency, falling back to the SHOW row.
  const comment = "comment" in descByProp ? descByProp["comment"] : find("comment");

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <LoginOutlined style={{ color: "var(--link)" }} />
          <span>Authentication Policy Properties</span>
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
        <div style={{ textAlign: "center", padding: 32 }}><Spin /></div>
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

          <div style={{ ...SECTION_HEAD, marginTop: 4 }}>Parameters</div>
          {descFailed && (
            <Alert
              type="warning"
              message="Could not read current parameter values (DESCRIBE failed)"
              description="Editing a parameter sets it directly regardless of the displayed value."
              showIcon
              style={{ marginBottom: 8 }}
            />
          )}
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Edit a value to <code>SET</code> it, or choose <em>Unset</em> to restore Snowflake's default
            (the list parameters default to <code>ALL</code>).
          </Text>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {LISTS.map((m) => (
                <ListRow
                  key={m.keyword}
                  meta={m}
                  value={parseList(descByProp[m.keyword.toLowerCase()] ?? "")}
                  onSet={(t) => setList(m.keyword, t)}
                  onUnset={() => unsetParam(m.keyword)}
                />
              ))}
              <EnumRow
                label="MFA enrollment"
                value={cleanScalar(descByProp["mfa_enrollment"] ?? "")}
                options={MFA_ENROLLMENT_OPTIONS}
                def="OPTIONAL"
                onSet={(v) => setEnum("MFA_ENROLLMENT", v)}
                onUnset={() => unsetParam("MFA_ENROLLMENT")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Advanced policies</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            The nested MFA / PAT / workload-identity / client property bags. Edit to set the
            sub-properties (only those you set are written); <em>Unset</em> restores Snowflake's default.
          </Text>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <MFAPolicyRow
                rawValue={descByProp["mfa_policy"] ?? ""}
                onSet={(v) => setBag("MFA_POLICY", v)}
                onUnset={unsetBag("MFA_POLICY")}
              />
              <PATPolicyRow
                rawValue={descByProp["pat_policy"] ?? ""}
                onSet={(v) => setBag("PAT_POLICY", v)}
                onUnset={unsetBag("PAT_POLICY")}
              />
              <WorkloadIdentityPolicyRow
                rawValue={descByProp["workload_identity_policy"] ?? ""}
                onSet={(v) => setBag("WORKLOAD_IDENTITY_POLICY", v)}
                onUnset={unsetBag("WORKLOAD_IDENTITY_POLICY")}
              />
              <ClientPolicyRow
                rawValue={descByProp["client_policy"] ?? ""}
                onSet={(v) => setBag("CLIENT_POLICY", v)}
                onUnset={unsetBag("CLIENT_POLICY")}
              />
            </tbody>
          </table>

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
                <td style={LABEL_TD}>DCM project</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space>
                    <Button size="small" onClick={unsetDcmProject} loading={dcmBusy}>Detach from DCM project</Button>
                    <Text type="secondary" style={{ fontSize: 11 }}>
                      Removes the policy's association with a Declarative Change Management project (<code>UNSET DCM PROJECT</code>).
                    </Text>
                  </Space>
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => r.key.toLowerCase() !== "comment")
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
            Users and the account this policy is attached to (from ACCOUNT_USAGE —
            requires governance privileges and may lag recent changes).
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
