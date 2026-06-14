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
  Modal, Spin, Button, Input, Space, Typography, Alert, Tag, Tooltip,
} from "antd";
import {
  GlobalOutlined, EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterNetworkRule } from "../../../wailsjs/go/app/App";
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

// DESCRIBE NETWORK RULE reports value_list as a comma-separated string of
// identifiers (e.g. "example.com:443,api.example.com:443"). Parse defensively so
// an unexpected format degrades to "no values" rather than throwing.
function parseValueList(raw: string): string[] {
  const s = (raw ?? "").trim();
  if (s === "" || s.toLowerCase() === "null") return [];
  return s.split(",").map((v) => v.trim()).filter((v) => v !== "");
}

// SET VALUE_LIST replaces the whole list (it is not additive), so every edit
// resends the full list. An empty list maps to UNSET VALUE_LIST.
function setValueListClause(values: string[]): string {
  if (values.length === 0) return "UNSET VALUE_LIST";
  return `SET VALUE_LIST = (${values.map(q1).join(", ")})`;
}

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

export default function NetworkRulePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [newValue, setNewValue] = useState("");

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "NETWORK RULE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const ruleRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const runAlter = async (clause: string, label: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterNetworkRule(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterNetworkRule(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterNetworkRule(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const values = parseValueList(find("value_list"));

  // `find` returns "" both when the value list is genuinely empty and when the
  // DESCRIBE NETWORK RULE enrichment was omitted (e.g. insufficient privileges).
  // Distinguish the two so a populated rule whose list couldn't be loaded isn't
  // shown as empty (and isn't editable, since SET VALUE_LIST would clobber it).
  const valueListLoaded = rows ? rows.some((r) => r.key.toLowerCase() === "value_list") : false;
  const entriesCount = Number.parseInt(find("entries_in_valuelist"), 10) || 0;
  const valueListUnavailable = !valueListLoaded && entriesCount > 0;

  const addValue = async () => {
    const v = newValue.trim();
    if (v === "") return;
    // De-dupe: SET VALUE_LIST replaces the whole list, and a duplicate would
    // collide on the React key and make removeValue drop both copies.
    if (values.includes(v)) { setNewValue(""); return; }
    await runAlter(setValueListClause([...values, v]), "Add value");
    setNewValue("");
  };

  const removeValue = (v: string) =>
    runAlter(setValueListClause(values.filter((x) => x !== v)), "Remove value");

  const comment = find("comment");
  const type = find("type");
  const mode = find("mode");

  // Keys handled by dedicated sections above the generic Properties table.
  const handledKeys = new Set(["comment", "type", "mode", "value_list", "entries_in_valuelist"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <GlobalOutlined style={{ color: "var(--link)" }} />
          <span>Network Rule Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {ruleRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={760}
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
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Type and mode are fixed at creation — to change them, recreate the rule.
          </Text>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Type</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", fontFamily: "var(--font-mono)" }}>
                  {type || <Text type="secondary">(unknown)</Text>}
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Mode</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", fontFamily: "var(--font-mono)" }}>
                  {mode || <Text type="secondary">(unknown)</Text>}
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Value list</div>
          {valueListUnavailable ? (
            // DESCRIBE NETWORK RULE didn't return the identifiers (likely a
            // privilege issue), but SHOW reports a non-zero count — surface that
            // rather than misrepresent the rule as empty, and disable editing so
            // SET VALUE_LIST can't overwrite values we can't see.
            <Alert
              type="warning"
              showIcon
              message={`Value list unavailable (${entriesCount} ${entriesCount === 1 ? "entry" : "entries"})`}
              description="DESCRIBE NETWORK RULE did not return the identifiers — this usually means insufficient privileges. Editing is disabled to avoid overwriting the existing values."
              style={{ marginBottom: 8 }}
            />
          ) : (
            <>
              <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
                {values.length === 0
                  ? "No network identifiers defined."
                  : "Network identifiers this rule matches."}
              </Text>
              <Space wrap size={[4, 8]} style={{ marginBottom: 8 }}>
                {values.map((v) => (
                  <Tag
                    key={v}
                    closable
                    onClose={(e) => {
                      e.preventDefault();
                      removeValue(v);
                    }}
                    style={{ fontSize: 12, fontFamily: "var(--font-mono)" }}
                  >
                    {v}
                  </Tag>
                ))}
              </Space>
              <Space>
                <Input
                  size="small"
                  value={newValue}
                  onChange={(e) => setNewValue(e.target.value)}
                  onPressEnter={addValue}
                  placeholder="Add value…"
                  style={{ width: 280 }}
                  disabled={busy}
                />
                <Button size="small" icon={<PlusOutlined />} onClick={addValue} loading={busy} disabled={newValue.trim() === ""}>
                  Add
                </Button>
                {values.length > 0 && (
                  <Button size="small" onClick={() => runAlter("UNSET VALUE_LIST", "Clear value list")} loading={busy}>
                    Clear all
                  </Button>
                )}
              </Space>
            </>
          )}

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
