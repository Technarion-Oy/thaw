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
  Modal, Spin, Button, Input, InputNumber, Space, Typography, Alert, Tooltip, Table, Empty,
} from "antd";
import {
  SafetyCertificateOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, DescribePasswordPolicy, AlterPasswordPolicy, GetPasswordPolicyReferences,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";

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

// Quote a free-text value as a single-quoted SQL literal. Snowflake treats the
// backslash as an escape character inside single-quoted literals, so a literal
// backslash must be doubled (else "C:\temp" is read as "C:temp"); single-quotes
// are doubled too. Mirrors the backend snowflake.EscapeTextLit.
function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

// The 11 password-policy parameters, in Snowflake's documented order, paired
// with their ALTER keyword and valid range. The current value/default is read
// from DESCRIBE PASSWORD POLICY (one property/value/default row each).
interface ParamMeta { keyword: string; label: string; min: number; max: number; }

const PARAMS: ParamMeta[] = [
  { keyword: "PASSWORD_MIN_LENGTH", label: "Min length", min: 8, max: 256 },
  { keyword: "PASSWORD_MAX_LENGTH", label: "Max length", min: 8, max: 256 },
  { keyword: "PASSWORD_MIN_UPPER_CASE_CHARS", label: "Min uppercase", min: 0, max: 256 },
  { keyword: "PASSWORD_MIN_LOWER_CASE_CHARS", label: "Min lowercase", min: 0, max: 256 },
  { keyword: "PASSWORD_MIN_NUMERIC_CHARS", label: "Min numeric", min: 0, max: 256 },
  { keyword: "PASSWORD_MIN_SPECIAL_CHARS", label: "Min special", min: 0, max: 256 },
  { keyword: "PASSWORD_MIN_AGE_DAYS", label: "Min age (days)", min: 0, max: 999 },
  { keyword: "PASSWORD_MAX_AGE_DAYS", label: "Max age (days)", min: 0, max: 999 },
  { keyword: "PASSWORD_MAX_RETRIES", label: "Max retries", min: 1, max: 10 },
  { keyword: "PASSWORD_LOCKOUT_TIME_MINS", label: "Lockout time (mins)", min: 1, max: 999 },
  { keyword: "PASSWORD_HISTORY", label: "History (reuse)", min: 0, max: 24 },
];

// ─── ParamRow (numeric setting with Set / Unset) ─────────────────────────────

interface ParamRowProps {
  meta: ParamMeta;
  value: string;
  def: string;
  onSet: (val: number) => Promise<void>;
  onUnset: () => Promise<void>;
}

function ParamRow({ meta, value, def, onSet, onUnset }: ParamRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<number | null>(value === "" ? null : Number(value));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    if (draft === null) return;
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
            <Space>
              <InputNumber
                size="small"
                value={draft}
                min={meta.min}
                max={meta.max}
                onChange={(v) => setDraft(v ?? null)}
                style={{ width: 120 }}
                onPressEnter={save}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={draft === null} />
              </Tooltip>
              <Tooltip title="Reset to Snowflake default">
                <Button size="small" onClick={unset} loading={saving}>Unset</Button>
              </Tooltip>
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value === "" ? null : Number(value)); setError(null); }} />
              </Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)", fontFamily: "var(--font-mono)" }}>
              {value || <Text type="secondary">(unknown)</Text>}
            </span>
            {def !== "" && (
              <Text type="secondary" style={{ fontSize: 11 }}>default {def}</Text>
            )}
            <Tooltip title="Edit">
              <Button
                type="text"
                size="small"
                icon={<EditOutlined style={{ fontSize: 11 }} />}
                onClick={() => { setDraft(value === "" ? null : Number(value)); setEditing(true); }}
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

export default function PasswordPolicyPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // DESCRIBE PASSWORD POLICY result (property/value/default/description rows).
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
        GetObjectProperties(db, schema, "PASSWORD POLICY", name),
        DescribePasswordPolicy(db, schema, name).catch(() => null),
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

  // Index the DESCRIBE rows by property name → { value, default }.
  const descByProp: Record<string, { value: string; def: string }> = {};
  if (desc && desc.columns && desc.rows) {
    const propIdx = desc.columns.findIndex((c) => c.toLowerCase() === "property");
    const valIdx = desc.columns.findIndex((c) => c.toLowerCase() === "value");
    const defIdx = desc.columns.findIndex((c) => c.toLowerCase() === "default");
    if (propIdx >= 0) {
      for (const row of desc.rows) {
        const prop = row[propIdx] === null || row[propIdx] === undefined ? "" : String(row[propIdx]).toUpperCase();
        const value = valIdx >= 0 && row[valIdx] != null ? String(row[valIdx]) : "";
        const def = defIdx >= 0 && row[defIdx] != null ? String(row[defIdx]) : "";
        if (prop) descByProp[prop] = { value, def };
      }
    }
  }

  const setParam = async (keyword: string, val: number) => {
    await AlterPasswordPolicy(db, schema, name, `SET ${keyword} = ${val}`);
    await reload();
  };

  const unsetParam = async (keyword: string) => {
    await AlterPasswordPolicy(db, schema, name, `UNSET ${keyword}`);
    await reload();
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterPasswordPolicy(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterPasswordPolicy(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const loadReferences = async () => {
    setRefsLoading(true);
    setRefsError(null);
    try {
      const r = await GetPasswordPolicyReferences(db, schema, name);
      setRefs(r);
    } catch (e) {
      setRefsError(String(e));
    } finally {
      setRefsLoading(false);
    }
  };

  const policyRef = `"${db}"."${schema}"."${name}"`;
  const comment = find("comment");

  // Keys surfaced through dedicated sections above the generic Properties table.
  const handledKeys = new Set(["comment"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <SafetyCertificateOutlined style={{ color: "var(--link)" }} />
          <span>Password Policy Properties</span>
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
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Edit a value to <code>SET</code> it, or choose <em>Unset</em> to restore Snowflake's default.
          </Text>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {PARAMS.map((m) => {
                const d = descByProp[m.keyword] ?? { value: "", def: "" };
                return (
                  <ParamRow
                    key={m.keyword}
                    meta={m}
                    value={d.value}
                    def={d.def}
                    onSet={(v) => setParam(m.keyword, v)}
                    onUnset={() => unsetParam(m.keyword)}
                  />
                );
              })}
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
