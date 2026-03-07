// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState } from "react";
import { Modal, Spin, Button, Input, Switch, message } from "antd";
import { CopyOutlined, EditOutlined, CheckOutlined, CloseOutlined } from "@ant-design/icons";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { main } from "../../../wailsjs/go/models";
import { SetSessionParameter, SetSessionVariable } from "../../../wailsjs/go/main/App";

interface Props {
  parameters: main.SessionParam[] | null;
  variables: main.SessionVar[] | null;
  error: string | null;
  onClose: () => void;
  // callbacks so the caller can refresh state after a save
  onParamChange: (key: string, value: string) => void;
  onVarChange:   (key: string, value: string) => void;
}

// ── shared styles ────────────────────────────────────────────────────────────

const LABEL_CELL: React.CSSProperties = {
  padding: "5px 12px 5px 0",
  color: "var(--text-muted)",
  fontFamily: "monospace",
  whiteSpace: "nowrap",
  verticalAlign: "middle",
  width: 260,
  minWidth: 200,
};

const VALUE_CELL: React.CSSProperties = {
  padding: "4px 0",
  verticalAlign: "middle",
};

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11,
  fontWeight: 600,
  color: "var(--text-muted)",
  letterSpacing: "0.05em",
  marginBottom: 8,
  textTransform: "uppercase",
};

// ── helpers ──────────────────────────────────────────────────────────────────

function isBool(type: string) {
  return type.toUpperCase() === "BOOLEAN";
}

// ── Parameters table ─────────────────────────────────────────────────────────

function ParamsTable({
  rows,
  onSave,
}: {
  rows: main.SessionParam[];
  onSave: (key: string, value: string) => void;
}) {
  const [editKey, setEditKey]   = useState<string | null>(null);
  const [editVal, setEditVal]   = useState("");
  const [saving,  setSaving]    = useState(false);

  if (rows.length === 0) {
    return <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0 8px" }}>No parameters.</div>;
  }

  const startEdit = (key: string, current: string) => { setEditKey(key); setEditVal(current); };
  const cancel    = () => { setEditKey(null); setEditVal(""); };

  const save = async (row: main.SessionParam) => {
    setSaving(true);
    try {
      await SetSessionParameter(row.key, editVal, row.type);
      onSave(row.key, editVal);
      setEditKey(null);
      message.success(`${row.key} updated`);
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (row: main.SessionParam, checked: boolean) => {
    const val = checked ? "TRUE" : "FALSE";
    try {
      await SetSessionParameter(row.key, val, row.type);
      onSave(row.key, val);
      message.success(`${row.key} ${checked ? "enabled" : "disabled"}`);
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
      <tbody>
        {rows.map((row) => {
          const isEditing = editKey === row.key;
          return (
            <tr key={row.key} style={{ borderBottom: "1px solid var(--border)" }}>
              <td style={LABEL_CELL} title={row.description || undefined}>{row.key}</td>
              <td style={VALUE_CELL}>
                {isBool(row.type) ? (
                  <Switch
                    size="small"
                    checked={row.value.toUpperCase() === "TRUE"}
                    onChange={(checked) => toggle(row, checked)}
                  />
                ) : isEditing ? (
                  <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                    <Input
                      size="small"
                      value={editVal}
                      onChange={(e) => setEditVal(e.target.value)}
                      onPressEnter={() => save(row)}
                      autoFocus
                      style={{ fontFamily: "monospace", fontSize: 12 }}
                    />
                    <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={() => save(row)} />
                    <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancel} />
                  </div>
                ) : (
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={{
                      fontFamily: "monospace",
                      color: row.value ? "var(--text)" : "var(--text-faint)",
                      fontStyle: row.value ? "normal" : "italic",
                      flex: 1,
                      wordBreak: "break-word",
                    }}>
                      {row.value || "—"}
                    </span>
                    <Button
                      size="small"
                      type="text"
                      icon={<EditOutlined />}
                      onClick={() => startEdit(row.key, row.value)}
                      style={{ flexShrink: 0, color: "var(--text-faint)" }}
                    />
                  </div>
                )}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// ── Variables table ───────────────────────────────────────────────────────────

function VarsTable({
  rows,
  onSave,
}: {
  rows: main.SessionVar[];
  onSave: (key: string, value: string) => void;
}) {
  const [editKey, setEditKey] = useState<string | null>(null);
  const [editVal, setEditVal] = useState("");
  const [saving,  setSaving]  = useState(false);

  if (rows.length === 0) {
    return <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0 8px" }}>No variables set.</div>;
  }

  const startEdit = (key: string, current: string) => { setEditKey(key); setEditVal(current); };
  const cancel    = () => { setEditKey(null); setEditVal(""); };

  const save = async (row: main.SessionVar) => {
    setSaving(true);
    try {
      await SetSessionVariable(row.key, editVal, row.type);
      onSave(row.key, editVal);
      setEditKey(null);
      message.success(`${row.key} updated`);
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (row: main.SessionVar, checked: boolean) => {
    const val = checked ? "TRUE" : "FALSE";
    try {
      await SetSessionVariable(row.key, val, row.type);
      onSave(row.key, val);
      message.success(`${row.key} ${checked ? "enabled" : "disabled"}`);
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
      <tbody>
        {rows.map((row) => {
          const isEditing = editKey === row.key;
          return (
            <tr key={row.key} style={{ borderBottom: "1px solid var(--border)" }}>
              <td style={LABEL_CELL}>{row.key}</td>
              <td style={VALUE_CELL}>
                {isBool(row.type) ? (
                  <Switch
                    size="small"
                    checked={row.value.toUpperCase() === "TRUE"}
                    onChange={(checked) => toggle(row, checked)}
                  />
                ) : isEditing ? (
                  <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                    <Input
                      size="small"
                      value={editVal}
                      onChange={(e) => setEditVal(e.target.value)}
                      onPressEnter={() => save(row)}
                      autoFocus
                      style={{ fontFamily: "monospace", fontSize: 12 }}
                    />
                    <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={() => save(row)} />
                    <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancel} />
                  </div>
                ) : (
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <span style={{
                      fontFamily: "monospace",
                      color: row.value ? "var(--text)" : "var(--text-faint)",
                      fontStyle: row.value ? "normal" : "italic",
                      flex: 1,
                      wordBreak: "break-word",
                    }}>
                      {row.value || "—"}
                    </span>
                    <Button
                      size="small"
                      type="text"
                      icon={<EditOutlined />}
                      onClick={() => startEdit(row.key, row.value)}
                      style={{ flexShrink: 0, color: "var(--text-faint)" }}
                    />
                  </div>
                )}
              </td>
            </tr>
          );
        })}
      </tbody>
    </table>
  );
}

// ── Main modal ────────────────────────────────────────────────────────────────

export default function SessionPropertiesModal({
  parameters,
  variables,
  error,
  onClose,
  onParamChange,
  onVarChange,
}: Props) {
  const loading = parameters === null && variables === null && !error;

  const copyAll = () => {
    const lines: string[] = [];
    if (parameters && parameters.length > 0) {
      lines.push("=== Parameters ===");
      parameters.forEach((r) => lines.push(`${r.key}: ${r.value}`));
    }
    if (variables && variables.length > 0) {
      if (lines.length) lines.push("");
      lines.push("=== Variables ===");
      variables.forEach((r) => lines.push(`${r.key}: ${r.value}`));
    }
    if (!lines.length) return;
    ClipboardSetText(lines.join("\n")).then(() => message.success("Copied to clipboard"));
  };

  return (
    <Modal
      open
      title="Session Properties"
      onCancel={onClose}
      width={700}
      footer={[
        <Button key="copy" icon={<CopyOutlined />} disabled={loading || !!error} onClick={copyAll}>
          Copy
        </Button>,
        <Button key="close" onClick={onClose}>Close</Button>,
      ]}
    >
      {loading && (
        <div style={{ textAlign: "center", padding: "32px 0" }}>
          <Spin />
        </div>
      )}

      {error && (
        <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>
          {error}
        </div>
      )}

      {!loading && !error && (
        <div style={{ maxHeight: "65vh", overflowY: "auto" }}>
          <div style={SECTION_HEAD}>Parameters</div>
          {parameters
            ? <ParamsTable rows={parameters} onSave={onParamChange} />
            : <Spin size="small" />}

          <div style={{ ...SECTION_HEAD, marginTop: 20, paddingTop: 12, borderTop: "1px solid var(--border)" }}>
            Variables
          </div>
          {variables
            ? <VarsTable rows={variables} onSave={onVarChange} />
            : <Spin size="small" />}
        </div>
      )}
    </Modal>
  );
}
