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
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Tag, Popconfirm,
} from "antd";
import {
  AuditOutlined, EditOutlined, CheckOutlined, CloseOutlined, ThunderboltOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterEventTable, GetEventTableParameters } from "../../../wailsjs/go/app/App";
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

// ─── EditRow (inline comment editor) ─────────────────────────────────────────

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

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
        {value || <Text type="secondary">(empty)</Text>}
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

export default function EventTablePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [params, setParams] = useState<snowflake.QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [changeTrackingBusy, setChangeTrackingBusy] = useState(false);
  const [searchOptBusy, setSearchOptBusy] = useState(false);

  const reload = useCallback(async () => {
    setRows(null);
    setParams(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "EVENT TABLE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
    // SHOW EVENT TABLES may omit some configurable values; SHOW PARAMETERS is
    // the fallback source for the object parameters (retention / max-extension).
    // Failure here is non-fatal — the settings rows just fall back to the SHOW
    // dump or show their defaults.
    try {
      const p = await GetEventTableParameters(db, schema, name);
      setParams(p ?? null);
    } catch {
      setParams(null);
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const tableRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  // Pull a parameter's current value out of the SHOW PARAMETERS result (columns
  // are key / value / default / …; we want the row whose key matches and its
  // value column).
  const paramVal = (key: string): string => {
    if (!params) return "";
    const cols = (params.columns ?? []).map((c) => c.toLowerCase());
    const keyCi = cols.indexOf("key");
    const valCi = cols.indexOf("value");
    if (keyCi < 0 || valCi < 0) return "";
    const row = (params.rows ?? []).find((r) => String(r[keyCi] ?? "").toLowerCase() === key.toLowerCase());
    return row ? String(row[valCi] ?? "") : "";
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterEventTable(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterEventTable(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  // SET/UNSET a non-negative-integer parameter (DATA_RETENTION_TIME_IN_DAYS /
  // MAX_DATA_EXTENSION_TIME_IN_DAYS). EditRow surfaces a thrown error inline.
  const saveIntParam = (param: string) => async (val: string) => {
    const v = val.trim();
    if (v === "") {
      await AlterEventTable(db, schema, name, `UNSET ${param}`);
    } else {
      if (!/^\d+$/.test(v)) throw new Error("Must be a non-negative integer.");
      await AlterEventTable(db, schema, name, `SET ${param} = ${v}`);
    }
    await reload();
  };

  const setChangeTracking = async (value: string) => {
    setChangeTrackingBusy(true);
    setActionError(null);
    try {
      await AlterEventTable(db, schema, name, `SET CHANGE_TRACKING = ${value}`);
      await reload();
    } catch (e) {
      setActionError(`Change tracking update failed: ${String(e)}`);
    } finally {
      setChangeTrackingBusy(false);
    }
  };

  const setSearchOptimization = async (enable: boolean) => {
    setSearchOptBusy(true);
    setActionError(null);
    try {
      await AlterEventTable(db, schema, name, enable ? "ADD SEARCH OPTIMIZATION" : "DROP SEARCH OPTIMIZATION");
      await reload();
    } catch (e) {
      setActionError(`Search optimization update failed: ${String(e)}`);
    } finally {
      setSearchOptBusy(false);
    }
  };

  const comment = find("comment");
  const owner = find("owner");
  const createdOn = find("created_on");
  // Prefer the value from the SHOW dump (mirrors the proven regular-table path
  // in internal/table, where change_tracking / retention are read straight from
  // SHOW). CHANGE_TRACKING is a table *property*, not an object parameter, so it
  // may not appear in SHOW PARAMETERS at all; the object parameters (retention /
  // max-extension) do, and serve as the fallback when the SHOW dump omits them.
  const setting = (showKey: string, paramKey: string) => find(showKey) || paramVal(paramKey);
  const changeTracking = setting("change_tracking", "CHANGE_TRACKING");
  const retention = setting("retention_time", "DATA_RETENTION_TIME_IN_DAYS");
  const maxExtension = setting("max_data_extension_time_in_days", "MAX_DATA_EXTENSION_TIME_IN_DAYS");
  // Keys rendered in the dedicated sections, hidden from the generic Properties
  // dump below (in case the SHOW dump does include them on some editions).
  const handledKeys = new Set([
    "comment", "owner", "created_on",
    "change_tracking", "retention_time", "max_data_extension_time_in_days",
  ]);

  const ctOn = changeTracking.toUpperCase() === "ON" || changeTracking.toUpperCase() === "TRUE";

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <AuditOutlined style={{ color: "var(--icon-eventtable)" }} />
          <span>Event Table Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {tableRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={720}
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

          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            Event tables store telemetry (logs, traces, metrics) in a fixed,
            predefined schema.
          </Text>

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <InfoRow label="Owner" value={owner} />
              <InfoRow label="Created on" value={createdOn} />
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
              {/* Editable row (not InfoRow, which reads as read-only): the Tag
                  shows the current state and the Select applies the change. */}
              <tr>
                <td style={LABEL_TD}>Change tracking</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space>
                    {/* Snowflake reports change tracking as ON/OFF; keep that
                        vocabulary in the dropdown too (the value still drives
                        SET CHANGE_TRACKING = TRUE/FALSE under the hood). */}
                    <Tag color={ctOn ? "green" : "default"}>{ctOn ? "ON" : "OFF"}</Tag>
                    <Select
                      size="small"
                      value={ctOn ? "TRUE" : "FALSE"}
                      onChange={setChangeTracking}
                      loading={changeTrackingBusy}
                      style={{ width: 100 }}
                      options={[{ value: "TRUE", label: "On" }, { value: "FALSE", label: "Off" }]}
                    />
                  </Space>
                </td>
              </tr>
              <EditRow
                label="Data retention (days)"
                value={retention}
                canUnset={retention !== ""}
                onSave={saveIntParam("DATA_RETENTION_TIME_IN_DAYS")}
                onUnset={() => saveIntParam("DATA_RETENTION_TIME_IN_DAYS")("")}
              />
              <EditRow
                label="Max data extension (days)"
                value={maxExtension}
                canUnset={maxExtension !== ""}
                onSave={saveIntParam("MAX_DATA_EXTENSION_TIME_IN_DAYS")}
                onUnset={() => saveIntParam("MAX_DATA_EXTENSION_TIME_IN_DAYS")("")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Search Optimization</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Speeds up selective point-lookup queries against the event table.
          </Text>
          <Space>
            <Popconfirm
              title="Enable search optimization?"
              description="ADD SEARCH OPTIMIZATION — this may incur additional storage and maintenance cost."
              okText="Enable"
              onConfirm={() => setSearchOptimization(true)}
            >
              <Button size="small" icon={<ThunderboltOutlined />} loading={searchOptBusy}>
                Add search optimization
              </Button>
            </Popconfirm>
            <Popconfirm
              title="Disable search optimization?"
              description="DROP SEARCH OPTIMIZATION removes the search access path from the whole table."
              okText="Disable"
              okButtonProps={{ danger: true }}
              onConfirm={() => setSearchOptimization(false)}
            >
              <Button size="small" danger loading={searchOptBusy}>
                Drop search optimization
              </Button>
            </Popconfirm>
          </Space>

          <Text type="secondary" style={{ fontSize: 11, display: "block", margin: "16px 0 0" }}>
            Row access policies, tags, contacts, and clustering keys are managed via
            the SQL editor (<code>ALTER TABLE … ADD ROW ACCESS POLICY / SET TAG / SET CONTACT / CLUSTER BY</code>).
          </Text>

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <InfoRow key={r.key} label={r.key} value={r.value} />
                ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
