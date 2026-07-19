// SPDX-License-Identifier: GPL-3.0-or-later

import { useState } from "react";
import { Modal, Spin, Button, Input, Tooltip, message } from "antd";
import { CopyOutlined, EditOutlined, CheckOutlined, CloseOutlined, SearchOutlined } from "@ant-design/icons";
import { ConfirmSwitch } from "./ConfirmSwitch";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { snowflake } from "../../../wailsjs/go/models";
import { SetAccountParameter } from "../../../wailsjs/go/app/App";

interface Props {
  parameters: snowflake.SessionParam[] | null;
  error: string | null;
  onClose: () => void;
  // Lets the caller keep its own copy of the parameters in sync after a save.
  onParamChange: (key: string, value: string) => void;
}

// ── shared styles (mirrors SessionPropertiesModal) ───────────────────────────

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

// ── helpers ──────────────────────────────────────────────────────────────────

function isBool(type: string) {
  return type.toUpperCase() === "BOOLEAN";
}

// ── Editable parameters table ─────────────────────────────────────────────────
//
// Editing account parameters issues ALTER ACCOUNT SET and requires the
// ACCOUNTADMIN role; unprivileged saves fail with a Snowflake privilege error
// surfaced via message.error, leaving the displayed value unchanged.

function ParamsTable({
  rows,
  onSave,
}: {
  rows: snowflake.SessionParam[];
  onSave: (key: string, value: string) => void;
}) {
  const [editKey, setEditKey] = useState<string | null>(null);
  const [editVal, setEditVal] = useState("");
  const [saving,  setSaving]  = useState(false);

  const startEdit = (key: string, current: string) => { setEditKey(key); setEditVal(current); };
  const cancel    = () => { setEditKey(null); setEditVal(""); };

  const save = async (row: snowflake.SessionParam) => {
    setSaving(true);
    try {
      await SetAccountParameter(row.key, editVal, row.type);
      onSave(row.key, editVal);
      setEditKey(null);
      message.success(`${row.key} updated`);
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  const toggle = async (row: snowflake.SessionParam, checked: boolean) => {
    const val = checked ? "TRUE" : "FALSE";
    await SetAccountParameter(row.key, val, row.type);
    onSave(row.key, val);
    message.success(`${row.key} ${checked ? "enabled" : "disabled"}`);
  };

  return (
    <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
      <tbody>
        {rows.map((row) => {
          const isEditing = editKey === row.key;
          return (
            <tr key={row.key} style={{ borderBottom: "1px solid var(--border)" }}>
              <td style={LABEL_CELL}>
                <Tooltip title={row.description || undefined} placement="right" mouseEnterDelay={0.3}>
                  <span style={{ cursor: "help", borderBottom: "1px dotted var(--text-muted)" }}>{row.key}</span>
                </Tooltip>
              </td>
              <td style={VALUE_CELL}>
                {isBool(row.type) ? (
                  <ConfirmSwitch
                    checked={row.value.toUpperCase() === "TRUE"}
                    onConfirm={(checked) => toggle(row, checked)}
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

export default function AccountParametersModal({ parameters, error, onClose, onParamChange }: Props) {
  const loading = parameters === null && !error;
  const [search, setSearch] = useState("");
  const q = search.trim().toLowerCase();

  const filtered = parameters ? (q ? parameters.filter((p) => p.key.toLowerCase().includes(q)) : parameters) : null;

  const copyAll = () => {
    if (!parameters || parameters.length === 0) return;
    const lines = parameters.map((r) => `${r.key}: ${r.value}`);
    ClipboardSetText(lines.join("\n")).then(() => message.success("Copied to clipboard"));
  };

  return (
    <Modal
      open
      title="Account Parameters"
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
        <>
          <div style={{ color: "var(--text-muted)", fontSize: 11, marginBottom: 10 }}>
            Editing applies <code>ALTER ACCOUNT SET</code> and requires the ACCOUNTADMIN role — saves fail with a privilege error otherwise.
          </div>
          <Input
            prefix={<SearchOutlined style={{ color: "var(--text-faint)" }} />}
            placeholder="Search by name…"
            allowClear
            autoFocus
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ marginBottom: 12 }}
          />
          <div style={{ maxHeight: "60vh", overflowY: "auto" }}>
            {filtered
              ? filtered.length > 0
                ? <ParamsTable rows={filtered} onSave={onParamChange} />
                : <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0 8px" }}>
                    {parameters && parameters.length > 0
                      ? "No matches."
                      : "No account parameters visible. Your current role may lack the privileges to view them."}
                  </div>
              : <Spin size="small" />}
          </div>
        </>
      )}
    </Modal>
  );
}
