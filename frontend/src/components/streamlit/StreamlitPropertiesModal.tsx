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
  Modal, Spin, Button, Input, Space, Typography, Alert, Tooltip,
} from "antd";
import {
  AppstoreOutlined, EditOutlined, CheckOutlined, CloseOutlined, CopyOutlined, LinkOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterStreamlit, GetSnowsightURL } from "../../../wailsjs/go/app/App";
import { ClipboardSetText, BrowserOpenURL } from "../../../wailsjs/runtime/runtime";
import { quoteIdent } from "../shared/ObjectNameCaseControl";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Single-quote-escape a SQL string literal (doubles embedded single quotes).
const q1 = (s: string) => `'${s.replace(/'/g, "''")}'`;

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

// ─── EditRow (single-line settings) ──────────────────────────────────────────

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

export default function StreamlitPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Snowsight account base (https://app.snowflake.com/<org>/<account>), used to
  // build the app's clickable deep-link. Reuses GetSnowsightURL so org / account
  // resolution lives in one place.
  const [snowsightBase, setSnowsightBase] = useState<string>("");

  useEffect(() => {
    GetSnowsightURL().then((u) => setSnowsightBase(u ?? "")).catch(() => {});
  }, []);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "STREAMLIT", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const appRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  // Each setting builds its own ALTER clause: string-valued fields are emitted
  // as quoted literals, the query warehouse as a bare identifier.
  const runAlter = async (clause: string, failMsg: string) => {
    setActionError(null);
    try {
      await AlterStreamlit(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${failMsg}: ${String(e)}`);
      throw e;
    }
  };

  const saveTitle = (v: string) =>
    v.trim() === "" ? runAlter("UNSET TITLE", "Update title failed")
                    : runAlter(`SET TITLE = ${q1(v)}`, "Update title failed");
  const saveComment = (v: string) =>
    v.trim() === "" ? runAlter("UNSET COMMENT", "Update comment failed")
                    : runAlter(`SET COMMENT = ${q1(v)}`, "Update comment failed");
  // Quote the warehouse identifier (matching BuildCreateStreamlitSql) so a
  // case-sensitive / quoted-created warehouse name resolves instead of being
  // upper-cased by Snowflake. The stored SHOW value preserves the exact case.
  const saveWarehouse = (v: string) =>
    v.trim() === "" ? runAlter("UNSET QUERY_WAREHOUSE", "Update query warehouse failed")
                    : runAlter(`SET QUERY_WAREHOUSE = ${quoteIdent(v.trim())}`, "Update query warehouse failed");
  // MAIN_FILE has no UNSET form (it's required) and Snowflake rejects an empty
  // value, so guard against clearing it rather than issuing invalid SQL.
  const saveMainFile = (v: string) => {
    if (v.trim() === "") return Promise.reject(new Error("Main file cannot be empty."));
    return runAlter(`SET MAIN_FILE = ${q1(v.trim())}`, "Update main file failed");
  };

  const title = find("title");
  const comment = find("comment");
  const queryWarehouse = find("query_warehouse");
  const mainFile = find("main_file");
  const urlId = find("url_id");

  // The Snowsight deep-link is name-based (org / account / DB.SCHEMA.NAME), not
  // built from url_id. The #/streamlit-apps/<fqn> fragment routes to the app on
  // the Snowsight host. Only available once the account base has resolved.
  const appUrl = snowsightBase
    ? `${snowsightBase}/#/streamlit-apps/${db}.${schema}.${name}`
    : "";

  // Keys handled by the dedicated sections above the generic Properties table.
  const handledKeys = new Set(["title", "comment", "query_warehouse", "main_file", "url_id"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <AppstoreOutlined style={{ color: "var(--link)" }} />
          <span>Streamlit Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {appRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={820}
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

          <div style={SECTION_HEAD}>URL endpoint</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Open the app in Snowsight. The link is name-based (organization / account / fully-qualified name).
          </Text>
          {appUrl ? (
            <Space>
              <a
                onClick={() => BrowserOpenURL(appUrl)}
                style={{ fontFamily: "var(--font-mono)", fontSize: 12, wordBreak: "break-all", cursor: "pointer" }}
              >
                {appUrl}
              </a>
              <Tooltip title="Open in browser">
                <Button
                  type="text"
                  size="small"
                  icon={<LinkOutlined style={{ fontSize: 12 }} />}
                  onClick={() => BrowserOpenURL(appUrl)}
                />
              </Tooltip>
              <Tooltip title="Copy URL">
                <Button
                  type="text"
                  size="small"
                  icon={<CopyOutlined style={{ fontSize: 12 }} />}
                  onClick={() => ClipboardSetText(appUrl)}
                />
              </Tooltip>
            </Space>
          ) : (
            <Text type="secondary">(resolving account…)</Text>
          )}
          {urlId && (
            <div style={{ marginTop: 6 }}>
              <Text type="secondary" style={{ fontSize: 11 }}>
                URL ID:{" "}
                <Text style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{urlId}</Text>
              </Text>
            </div>
          )}

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="Title" value={title} canUnset={title !== ""} onSave={saveTitle} onUnset={() => saveTitle("")} />
              <EditRow label="Main file" value={mainFile} onSave={saveMainFile} />
              <EditRow label="Query warehouse" value={queryWarehouse} canUnset={queryWarehouse !== ""} onSave={saveWarehouse} onUnset={() => saveWarehouse("")} />
              <EditRow label="Comment" value={comment} canUnset={comment !== ""} onSave={saveComment} onUnset={() => saveComment("")} />
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
