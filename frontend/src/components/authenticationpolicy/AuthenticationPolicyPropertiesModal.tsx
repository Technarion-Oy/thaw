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

import { useState, useEffect, useCallback, useMemo } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Table, Empty,
} from "antd";
import {
  LoginOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, DescribeAuthenticationPolicy, AlterAuthenticationPolicy,
  GetAuthenticationPolicyReferences, FormatAuthPolicyList,
  ParseSqlList, NormalizeSqlScalar, QuoteSqlText, ReconcileAllExclusiveList,
  AuthenticationPolicyListParams, AuthenticationPolicyMFAEnrollmentOptions,
} from "../../../wailsjs/go/app/App";
import type { snowflake, authenticationpolicy } from "../../../wailsjs/go/models";
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
//
// The SQL-quoting (q1), DESCRIBE list/scalar parsing (parseList, cleanScalar) and
// the list-parameter metadata (LISTS, MFA_ENROLLMENT_OPTIONS) all moved to the Go
// backend so this modal carries no SQL or grammar knowledge: quoting is
// QuoteSqlText, list parsing is ParseSqlList, scalar normalization is
// NormalizeSqlScalar, and the parameter metadata comes from
// AuthenticationPolicyListParams / AuthenticationPolicyMFAEnrollmentOptions.

// ─── ListRow (token-list setting with Set / Unset) ───────────────────────────

interface ListRowProps {
  meta: authenticationpolicy.ListParamMeta;
  rawValue: string;
  onSet: (tokens: string[]) => Promise<void>;
  onUnset: () => Promise<void>;
}

function ListRow({ meta, rawValue, onSet, onUnset }: ListRowProps) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState<string[]>([]);
  const [draft, setDraft] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // DESCRIBE renders the list cell in a SQL/bracket form the backend tokenizer
  // owns — parse it there rather than in the UI. Re-runs when rawValue changes
  // (e.g. after a Set/Unset reload) so the displayed tokens stay in sync.
  useEffect(() => {
    let alive = true;
    ParseSqlList(rawValue).then((t) => { if (alive) setValue(t ?? []); });
    return () => { alive = false; };
  }, [rawValue]);

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
                // ALL is mutually exclusive with specific values — reconcile in
                // the backend (keeps whichever kind was chosen last) so an invalid
                // ('ALL', X) list can't be submitted.
                onChange={async (v) => setDraft((await ReconcileAllExclusiveList(v)) ?? [])}
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
  rawValue: string;
  options: string[];
  def: string;
  onSet: (val: string) => Promise<void>;
  onUnset: () => Promise<void>;
}

function EnumRow({ label, rawValue, options, def, onSet, onUnset }: EnumRowProps) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState<string>("");
  const [draft, setDraft] = useState<string>("");
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Strip the brackets/quotes DESCRIBE may wrap the scalar in via the backend
  // normalizer so the value matches a bare option; re-runs on rawValue change.
  useEffect(() => {
    let alive = true;
    NormalizeSqlScalar(rawValue).then((v) => { if (alive) setValue(v ?? ""); });
    return () => { alive = false; };
  }, [rawValue]);

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

  // DESCRIBE AUTHENTICATION POLICY result, already projected to property/value
  // pairs by the backend (null while loading or if the DESCRIBE failed).
  const [desc, setDesc] = useState<snowflake.PropertyPair[] | null>(null);

  // References (users/account the policy is attached to) — loaded on demand
  // because the ACCOUNT_USAGE view is slow and may be restricted.
  const [refs, setRefs] = useState<snowflake.QueryResult | null>(null);
  const [refsError, setRefsError] = useState<string | null>(null);
  const [refsLoading, setRefsLoading] = useState(false);

  // List-parameter metadata (keyword, label, allowed values) and the MFA
  // enrollment options come from the backend — static grammar data fetched once.
  const [listParams, setListParams] = useState<authenticationpolicy.ListParamMeta[]>([]);
  const [mfaOptions, setMfaOptions] = useState<string[]>([]);
  useEffect(() => {
    AuthenticationPolicyListParams().then((p) => setListParams(p ?? []));
    AuthenticationPolicyMFAEnrollmentOptions().then((o) => setMfaOptions(o ?? []));
  }, []);

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

  // Index the backend's property/value pairs by lowercased property name. Only
  // recomputed when the DESCRIBE data changes, not on every child-row re-render.
  const descByProp = useMemo(() => {
    const map: Record<string, string> = {};
    (desc ?? []).forEach((p) => { if (p.key) map[p.key.toLowerCase()] = p.value; });
    return map;
  }, [desc]);

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
    if (!mfaOptions.includes(val)) return;
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
      await AlterAuthenticationPolicy(db, schema, name, `SET COMMENT = ${await QuoteSqlText(comment)}`);
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
              {listParams.map((m) => (
                <ListRow
                  key={m.keyword}
                  meta={m}
                  rawValue={descByProp[m.keyword.toLowerCase()] ?? ""}
                  onSet={(t) => setList(m.keyword, t)}
                  onUnset={() => unsetParam(m.keyword)}
                />
              ))}
              <EnumRow
                label="MFA enrollment"
                rawValue={descByProp["mfa_enrollment"] ?? ""}
                options={mfaOptions}
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
