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
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip, Tag, Select,
} from "antd";
import {
  ApiOutlined, EditOutlined, CheckOutlined, CloseOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, DescribeExternalFunction, AlterExternalFunction } from "../../../wailsjs/go/app/App";
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

// The DESCRIBE FUNCTION property names that carry the external-function transport
// detail, rendered first (in this order) under a dedicated section.
const DETAIL_ORDER = [
  "signature", "returns", "language", "null handling", "volatility",
  "api_integration", "headers", "context_headers", "max_batch_rows",
  "compression", "request_translator", "response_translator", "body",
];

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
  /** Argument type list (e.g. "NUMBER, VARCHAR") needed to resolve the overload
   *  for DESCRIBE / ALTER FUNCTION. */
  args: string;
  onClose: () => void;
}

export default function ExternalFunctionPropertiesModal({ db, schema, name, args, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [detail, setDetail] = useState<snowflake.QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [secureBusy, setSecureBusy] = useState(false);

  const reload = useCallback(async () => {
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "EXTERNAL FUNCTION", name);
      // DESCRIBE FUNCTION supplies the transport detail SHOW EXTERNAL FUNCTIONS
      // omits. Failure is non-fatal — the overview/comment still render.
      let d: snowflake.QueryResult | null = null;
      try {
        d = (await DescribeExternalFunction(db, schema, name, args)) ?? null;
      } catch {
        d = null;
      }
      setRows(props ?? []);
      setDetail(d);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name, args]);

  useEffect(() => { reload(); }, [reload]);

  const fnRef = `"${db}"."${schema}"."${name}"(${args})`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  // Project the DESCRIBE FUNCTION result (property / value columns) into a map.
  const describeMap = (): Record<string, string> => {
    const out: Record<string, string> = {};
    if (!detail) return out;
    const cols = (detail.columns ?? []).map((c) => c.toLowerCase());
    const propCi = cols.indexOf("property");
    const valCi = cols.indexOf("value");
    if (propCi < 0 || valCi < 0) return out;
    for (const r of detail.rows ?? []) {
      const k = String(r[propCi] ?? "").toLowerCase();
      if (k) out[k] = String(r[valCi] ?? "");
    }
    return out;
  };

  const dmap = describeMap();

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterExternalFunction(db, schema, name, args, "UNSET COMMENT");
    } else {
      await AlterExternalFunction(db, schema, name, args, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const setSecure = async (secure: boolean) => {
    setSecureBusy(true);
    setActionError(null);
    try {
      await AlterExternalFunction(db, schema, name, args, secure ? "SET SECURE" : "UNSET SECURE");
      await reload();
    } catch (e) {
      setActionError(`Secure update failed: ${String(e)}`);
    } finally {
      setSecureBusy(false);
    }
  };

  const comment = find("description") || find("comment");
  const owner = find("owner");
  const createdOn = find("created_on");
  const language = find("language") || dmap["language"];
  const isSecure = (find("is_secure") || "").toUpperCase() === "Y" || (find("is_secure") || "").toUpperCase() === "TRUE";

  // Keys rendered in the dedicated sections, hidden from the generic SHOW dump.
  const handledKeys = new Set(["description", "comment", "owner", "created_on", "language", "is_secure"]);
  // The detail keys already shown in the External Function section.
  const detailHandled = new Set(DETAIL_ORDER);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ApiOutlined style={{ color: "var(--icon-externalfunction)" }} />
          <span>External Function Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {fnRef}
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

          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            External functions call code outside Snowflake through an API
            integration. The transport detail below comes from DESCRIBE FUNCTION.
          </Text>

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <InfoRow label="Owner" value={owner} />
              <InfoRow label="Created on" value={createdOn} />
              <InfoRow label="Language" value={language} />
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
                <td style={LABEL_TD}>Secure</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space>
                    <Tag color={isSecure ? "green" : "default"}>{isSecure ? "SECURE" : "NOT SECURE"}</Tag>
                    <Select
                      size="small"
                      value={isSecure ? "TRUE" : "FALSE"}
                      onChange={(v) => setSecure(v === "TRUE")}
                      loading={secureBusy}
                      style={{ width: 100 }}
                      options={[{ value: "TRUE", label: "Secure" }, { value: "FALSE", label: "Not secure" }]}
                    />
                  </Space>
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>External Function Detail</div>
          {detail ? (
            <table style={{ width: "100%", borderCollapse: "collapse" }}>
              <tbody>
                {DETAIL_ORDER.filter((k) => dmap[k] !== undefined).map((k) => (
                  <InfoRow key={k} label={k} value={dmap[k]} />
                ))}
                {/* Any remaining DESCRIBE rows not in the canonical order. */}
                {Object.keys(dmap)
                  .filter((k) => !detailHandled.has(k))
                  .map((k) => (
                    <InfoRow key={k} label={k} value={dmap[k]} />
                  ))}
              </tbody>
            </table>
          ) : (
            <Text type="secondary" style={{ fontSize: 12 }}>
              DESCRIBE FUNCTION returned no detail (insufficient privileges, or the
              function was dropped).
            </Text>
          )}

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
