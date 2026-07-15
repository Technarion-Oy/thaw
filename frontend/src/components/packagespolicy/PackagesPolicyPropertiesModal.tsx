// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip,
} from "antd";
import {
  CodeSandboxOutlined, EditOutlined, CheckOutlined, CloseOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterPackagesPolicy, FormatPackagesPolicyList, ParsePackagesPolicyList, QuoteSqlText,
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

// ─── ListRow (package-spec list with Set / Unset) ────────────────────────────
//
// The list parameters aren't ALL-exclusive (a packages policy lists concrete
// package specs, not the ALL token), so this is a plain tags editor — no
// reconciliation. rawValue is the DESCRIBE list cell, parsed into tokens by the
// shared backend tokenizer.

interface ListRowProps {
  label: string;
  rawValue: string;
  defaultHint: string;
  onSet: (tokens: string[]) => Promise<void>;
  onUnset: () => Promise<void>;
}

function ListRow({ label, rawValue, defaultHint, onSet, onUnset }: ListRowProps) {
  const [editing, setEditing] = useState(false);
  const [value, setValue] = useState<string[]>([]);
  const [draft, setDraft] = useState<string[]>([]);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // DESCRIBE renders the list cell in a SQL/bracket form the backend parser owns
  // — parse it there. ParsePackagesPolicyList (not the generic ParseSqlList)
  // preserves package version specifiers like "numpy==1.26.4" whether or not
  // Snowflake quotes the entries. Re-runs when rawValue changes (e.g. after a
  // Set/Unset reload).
  useEffect(() => {
    let alive = true;
    ParsePackagesPolicyList(rawValue).then((t) => { if (alive) setValue(t ?? []); });
    return () => { alive = false; };
  }, [rawValue]);

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
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space align="start">
              <Select
                size="small"
                mode="tags"
                value={draft}
                onChange={setDraft}
                placeholder="package specs (e.g. numpy==1.26.4)"
                tokenSeparators={[",", " "]}
                options={[{ value: "*", label: "* (all)" }]}
                style={{ width: 360 }}
              />
              <Tooltip title={draftEmpty ? "Use Unset to restore the default — an empty list is not valid SQL" : "Save"}>
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={draftEmpty} />
              </Tooltip>
              <Tooltip title={`Reset to Snowflake default (${defaultHint})`}>
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
              {value.length > 0 ? value.join(", ") : <Text type="secondary">(default: {defaultHint})</Text>}
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

export default function PackagesPolicyPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "PACKAGES POLICY", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const policyRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const setList = async (keyword: string, tokens: string[]) => {
    try {
      await AlterPackagesPolicy(db, schema, name, `SET ${keyword} = ${await FormatPackagesPolicyList(tokens)}`);
      await reload();
    } catch (e) {
      setActionError(String(e));
      throw e;
    }
  };
  const unsetParam = async (keyword: string) => {
    try {
      await AlterPackagesPolicy(db, schema, name, `UNSET ${keyword}`);
      await reload();
    } catch (e) {
      setActionError(String(e));
      throw e;
    }
  };
  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterPackagesPolicy(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterPackagesPolicy(db, schema, name, `SET COMMENT = ${await QuoteSqlText(comment)}`);
    }
    await reload();
  };

  const comment = find("comment");
  const language = find("language") || "PYTHON";

  // Keys rendered by dedicated sections — hidden from the generic Properties table.
  const handledKeys = new Set([
    "comment", "language", "allowlist", "blocklist", "additional_creation_blocklist",
  ]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <CodeSandboxOutlined style={{ color: "var(--link)" }} />
          <span>Packages Policy Properties</span>
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

          <div style={{ ...SECTION_HEAD, marginTop: 4 }}>Package controls</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Edit a list to <code>SET</code> it, or choose <em>Unset</em> to restore Snowflake's default.
            The blocklist takes precedence over the allowlist.
          </Text>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Language</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", fontFamily: "var(--font-mono)" }}>{language}</td>
              </tr>
              <ListRow
                label="Allowlist"
                rawValue={find("allowlist")}
                defaultHint="* — all allowed"
                onSet={(t) => setList("ALLOWLIST", t)}
                onUnset={() => unsetParam("ALLOWLIST")}
              />
              <ListRow
                label="Blocklist"
                rawValue={find("blocklist")}
                defaultHint="none blocked"
                onSet={(t) => setList("BLOCKLIST", t)}
                onUnset={() => unsetParam("BLOCKLIST")}
              />
              <ListRow
                label="Additional creation blocklist"
                rawValue={find("additional_creation_blocklist")}
                defaultHint="none blocked"
                onSet={(t) => setList("ADDITIONAL_CREATION_BLOCKLIST", t)}
                onUnset={() => unsetParam("ADDITIONAL_CREATION_BLOCKLIST")}
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
