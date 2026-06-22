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
  Modal, Spin, Button, Input, Space, Typography, Alert, Tag, Tooltip, Table, Empty, Form,
} from "antd";
import {
  TagsOutlined, EditOutlined, CheckOutlined, CloseOutlined, PlusOutlined, ReloadOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterTag, GetTagReferences } from "../../../wailsjs/go/app/App";
import TagPropagationFields, { ALLOWED_VALUES_SEQUENCE } from "./TagPropagationFields";
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
  width: 220,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }

// SHOW TAGS reports allowed_values as a JSON array string (e.g. ["a","b"]) or an
// empty/null value when the tag accepts any string. Parse defensively so a
// future format change degrades to "no restriction" rather than throwing.
function parseAllowedValues(raw: string): string[] {
  const s = (raw ?? "").trim();
  if (s === "" || s.toLowerCase() === "null" || s === "[]") return [];
  try {
    const parsed = JSON.parse(s);
    if (Array.isArray(parsed)) return parsed.map((v) => String(v));
  } catch {
    /* fall through */
  }
  return [];
}

// ─── EditRow ─────────────────────────────────────────────────────────────────

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

export default function TagPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [newValue, setNewValue] = useState("");

  // Propagation drafts (PROPAGATE / ON_CONFLICT). Initialized from the loaded
  // SHOW TAGS values and resynced on every reload; Apply is enabled only when a
  // draft differs from what is currently set, so the user can't accidentally
  // clobber propagation by clicking Apply without making a change.
  const [propDraft, setPropDraft] = useState("");
  const [conflictDraft, setConflictDraft] = useState("");

  // References (where the tag is applied) — loaded on demand because the
  // ACCOUNT_USAGE view is slow and may be restricted.
  const [refs, setRefs] = useState<snowflake.QueryResult | null>(null);
  const [refsError, setRefsError] = useState<string | null>(null);
  const [refsLoading, setRefsLoading] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "TAG", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const tagRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const runAlter = async (clause: string, label: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterTag(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterTag(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterTag(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const addAllowedValue = async () => {
    const v = newValue.trim();
    if (v === "") return;
    await runAlter(`ADD ALLOWED_VALUES ${q1(v)}`, "Add allowed value");
    setNewValue("");
  };

  const loadReferences = async () => {
    setRefsLoading(true);
    setRefsError(null);
    try {
      const r = await GetTagReferences(db, schema, name);
      setRefs(r);
    } catch (e) {
      setRefsError(String(e));
    } finally {
      setRefsLoading(false);
    }
  };

  const comment = find("comment");
  const allowedValues = parseAllowedValues(find("allowed_values"));

  // Current propagation settings, as reported by SHOW TAGS. The propagate column
  // is only meaningful when it names one of the three modes; anything else (or a
  // missing column on older accounts) is treated as disabled. ON_CONFLICT is
  // reported as a bare keyword (ALLOWED_VALUES_SEQUENCE) or a quoted value.
  const propModes = ["ON_DEPENDENCY_AND_DATA_MOVEMENT", "ON_DEPENDENCY", "ON_DATA_MOVEMENT"];
  const rawPropagate = find("propagate").trim().toUpperCase();
  const currentPropagate = propModes.includes(rawPropagate) ? rawPropagate : "";
  const currentConflict = currentPropagate ? find("on_conflict").trim() : "";

  // Resync the propagation drafts whenever the loaded properties change.
  useEffect(() => {
    setPropDraft(currentPropagate);
    setConflictDraft(currentConflict);
  }, [currentPropagate, currentConflict]);

  const propagationDirty = propDraft !== currentPropagate || conflictDraft !== currentConflict;

  const applyPropagation = async () => {
    if (propDraft === "") {
      // UNSET PROPAGATE also clears any ON_CONFLICT.
      await runAlter("UNSET PROPAGATE", "Set propagation");
      return;
    }
    let clause = `SET PROPAGATE = ${propDraft}`;
    if (conflictDraft !== "") {
      clause += conflictDraft === ALLOWED_VALUES_SEQUENCE
        ? ` ON_CONFLICT = ${ALLOWED_VALUES_SEQUENCE}`
        : ` ON_CONFLICT = ${q1(conflictDraft)}`;
    }
    await runAlter(clause, "Set propagation");
    // Clear a previously-set ON_CONFLICT the user removed while keeping PROPAGATE.
    if (conflictDraft === "" && currentConflict !== "") {
      await runAlter("UNSET ON_CONFLICT", "Clear on-conflict");
    }
  };

  // Keys handled by dedicated sections above the generic Properties table.
  const handledKeys = new Set(["comment", "allowed_values", "propagate", "on_conflict"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <TagsOutlined style={{ color: "var(--link)" }} />
          <span>Tag Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {tagRef}
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

          <div style={SECTION_HEAD}>Allowed values</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            {allowedValues.length === 0
              ? "No restriction — the tag accepts any string value."
              : "Only these values may be assigned when the tag is applied."}
          </Text>
          <Space wrap size={[4, 8]} style={{ marginBottom: 8 }}>
            {allowedValues.map((v) => (
              <Tag
                key={v}
                closable
                onClose={(e) => {
                  e.preventDefault();
                  runAlter(`DROP ALLOWED_VALUES ${q1(v)}`, "Drop allowed value");
                }}
                style={{ fontSize: 12 }}
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
              onPressEnter={addAllowedValue}
              placeholder="Add value…"
              style={{ width: 220 }}
              disabled={busy}
            />
            <Button size="small" icon={<PlusOutlined />} onClick={addAllowedValue} loading={busy} disabled={newValue.trim() === ""}>
              Add
            </Button>
            {allowedValues.length > 0 && (
              <Button size="small" onClick={() => runAlter("UNSET ALLOWED_VALUES", "Unset allowed values")} loading={busy}>
                Clear all
              </Button>
            )}
          </Space>

          <div style={SECTION_HEAD}>Propagation (tag lineage)</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Automatically propagate this tag from source objects to target objects, and choose
            how to resolve conflicts between propagated values.
          </Text>
          <Form layout="vertical" size="small" component="div">
            <TagPropagationFields
              propagate={propDraft}
              onConflict={conflictDraft}
              onChange={({ propagate, onConflict }) => { setPropDraft(propagate); setConflictDraft(onConflict); }}
              itemStyle={{ marginBottom: 12 }}
              disabled={busy}
            />
          </Form>
          <Space>
            <Button
              size="small"
              type="primary"
              onClick={applyPropagation}
              loading={busy}
              disabled={!propagationDirty}
            >
              Apply
            </Button>
            {propagationDirty && (
              <Button
                size="small"
                onClick={() => { setPropDraft(currentPropagate); setConflictDraft(currentConflict); }}
                disabled={busy}
              >
                Reset
              </Button>
            )}
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

          <div style={SECTION_HEAD}>References</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Objects and columns this tag is applied to (from ACCOUNT_USAGE — requires
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
