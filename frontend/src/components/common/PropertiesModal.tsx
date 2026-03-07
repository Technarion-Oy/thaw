// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Modal, Spin, Button, Input, InputNumber, Switch, message } from "antd";
import { CopyOutlined, EditOutlined, CheckOutlined, CloseOutlined } from "@ant-design/icons";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { main } from "../../../wailsjs/go/models";
import { GetColumnComments, SetColumnComment, GetTableSettings, AlterTableProperty } from "../../../wailsjs/go/main/App";

interface TableContext {
  db: string;
  schema: string;
  table: string;
}

interface Props {
  title: string;
  rows: main.PropertyPair[] | null;  // null = loading
  error: string | null;
  onClose: () => void;
  tableContext?: TableContext;
}

function ColumnCommentsSection({ db, schema, table }: TableContext) {
  const [colComments, setColComments] = useState<main.ColumnComment[] | null>(null);
  const [loadError, setLoadError]     = useState<string | null>(null);
  const [editingCol, setEditingCol]   = useState<string | null>(null);
  const [editValue, setEditValue]     = useState("");
  const [saving, setSaving]           = useState(false);

  useEffect(() => {
    GetColumnComments(db, schema, table)
      .then(setColComments)
      .catch((e) => setLoadError(String(e)));
  }, [db, schema, table]);

  const startEdit = (col: string, current: string) => {
    setEditingCol(col);
    setEditValue(current);
  };

  const cancelEdit = () => {
    setEditingCol(null);
    setEditValue("");
  };

  const saveEdit = async (col: string) => {
    setSaving(true);
    try {
      await SetColumnComment(db, schema, table, col, editValue);
      setColComments((prev) =>
        prev ? prev.map((c) => c.column === col ? { ...c, comment: editValue } : c) : prev
      );
      setEditingCol(null);
      message.success("Comment updated");
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <div style={{ marginTop: 20, borderTop: "1px solid var(--border)", paddingTop: 12 }}>
      <div style={{ fontSize: 11, fontWeight: 600, color: "var(--text-muted)", letterSpacing: "0.05em", marginBottom: 8, textTransform: "uppercase" }}>
        Column Comments
      </div>

      {colComments === null && !loadError && (
        <div style={{ textAlign: "center", padding: "12px 0" }}>
          <Spin size="small" />
        </div>
      )}

      {loadError && (
        <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12 }}>{loadError}</div>
      )}

      {colComments && colComments.length === 0 && (
        <div style={{ color: "var(--text-muted)", fontSize: 12 }}>No columns found.</div>
      )}

      {colComments && colComments.length > 0 && (
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
          <tbody>
            {colComments.map((c) => (
              <tr key={c.column} style={{ borderBottom: "1px solid var(--border)" }}>
                {/* Column name */}
                <td style={{
                  padding: "5px 12px 5px 0",
                  color: "var(--text-muted)",
                  fontFamily: "monospace",
                  whiteSpace: "nowrap",
                  verticalAlign: "middle",
                  width: 200,
                  minWidth: 160,
                }}>
                  {c.column}
                </td>

                {/* Comment value / editor */}
                <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
                  {editingCol === c.column ? (
                    <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                      <Input
                        size="small"
                        value={editValue}
                        onChange={(e) => setEditValue(e.target.value)}
                        onPressEnter={() => saveEdit(c.column)}
                        autoFocus
                        style={{ fontFamily: "monospace", fontSize: 12 }}
                      />
                      <Button
                        size="small"
                        type="primary"
                        icon={<CheckOutlined />}
                        loading={saving}
                        onClick={() => saveEdit(c.column)}
                      />
                      <Button
                        size="small"
                        icon={<CloseOutlined />}
                        disabled={saving}
                        onClick={cancelEdit}
                      />
                    </div>
                  ) : (
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <span style={{
                        fontFamily: "monospace",
                        color: c.comment ? "var(--text)" : "var(--text-faint)",
                        fontStyle: c.comment ? "normal" : "italic",
                        flex: 1,
                        wordBreak: "break-word",
                      }}>
                        {c.comment || "—"}
                      </span>
                      <Button
                        size="small"
                        type="text"
                        icon={<EditOutlined />}
                        onClick={() => startEdit(c.column, c.comment)}
                        style={{ flexShrink: 0, color: "var(--text-faint)" }}
                      />
                    </div>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  );
}

// ── Table-level settings section ──────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11,
  fontWeight: 600,
  color: "var(--text-muted)",
  letterSpacing: "0.05em",
  marginBottom: 8,
  textTransform: "uppercase",
};

type PropKey = keyof main.TableSettings;

interface PropDef {
  key:   PropKey;
  label: string;
  type:  "text" | "number" | "boolean";
  hint?: string;
}

const PROP_DEFS: PropDef[] = [
  { key: "comment",               label: "Comment",                  type: "text"    },
  { key: "clusterBy",             label: "Cluster By",               type: "text",    hint: "Comma-separated column expressions. Clear to drop clustering key." },
  { key: "enableSchemaEvolution", label: "Enable Schema Evolution",  type: "boolean" },
  { key: "changeTracking",        label: "Change Tracking",          type: "boolean" },
  { key: "dataRetentionDays",     label: "Data Retention (days)",    type: "number"  },
  { key: "maxDataExtensionDays",  label: "Max Data Extension (days)",type: "number"  },
  { key: "defaultDDLCollation",   label: "Default DDL Collation",    type: "text"    },
];

function TableSettingsSection({ db, schema, table }: { db: string; schema: string; table: string }) {
  const [settings, setSettings]   = useState<main.TableSettings | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [editKey, setEditKey]     = useState<PropKey | null>(null);
  const [editVal, setEditVal]     = useState<string>("");
  const [saving, setSaving]       = useState(false);

  useEffect(() => {
    GetTableSettings(db, schema, table)
      .then(setSettings)
      .catch((e) => setLoadError(String(e)));
  }, [db, schema, table]);

  const startEdit = (key: PropKey, current: string) => {
    setEditKey(key);
    setEditVal(current);
  };

  const cancelEdit = () => { setEditKey(null); setEditVal(""); };

  const saveEdit = async (def: PropDef) => {
    setSaving(true);
    try {
      await AlterTableProperty(db, schema, table, def.key, editVal);
      setSettings((prev) => prev ? { ...prev, [def.key]: def.type === "number" ? parseInt(editVal, 10) : editVal } : prev);
      setEditKey(null);
      message.success(`${def.label} updated`);
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  const toggleBool = async (def: PropDef, checked: boolean) => {
    try {
      await AlterTableProperty(db, schema, table, def.key, checked ? "TRUE" : "FALSE");
      setSettings((prev) => prev ? { ...prev, [def.key]: checked } : prev);
      message.success(`${def.label} ${checked ? "enabled" : "disabled"}`);
    } catch (e) {
      message.error(String(e));
    }
  };

  return (
    <div style={{ marginTop: 20, borderTop: "1px solid var(--border)", paddingTop: 12 }}>
      <div style={SECTION_HEAD}>Table Settings</div>

      {loadError && <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12 }}>{loadError}</div>}
      {!settings && !loadError && <div style={{ textAlign: "center", padding: "12px 0" }}><Spin size="small" /></div>}

      {settings && (
        <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
          <tbody>
            {PROP_DEFS.map((def) => {
              const rawVal = settings[def.key];
              const strVal = def.type === "boolean" ? String(rawVal) : String(rawVal ?? "");
              const isEditing = editKey === def.key;

              return (
                <tr key={def.key} style={{ borderBottom: "1px solid var(--border)" }}>
                  {/* Label */}
                  <td style={{
                    padding: "6px 12px 6px 0",
                    color: "var(--text-muted)",
                    fontFamily: "monospace",
                    whiteSpace: "nowrap",
                    verticalAlign: "middle",
                    width: 200,
                    minWidth: 160,
                  }}>
                    {def.label}
                  </td>

                  {/* Value / editor */}
                  <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
                    {def.type === "boolean" ? (
                      <Switch
                        size="small"
                        checked={Boolean(rawVal)}
                        onChange={(checked) => toggleBool(def, checked)}
                      />
                    ) : isEditing ? (
                      <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                        {def.type === "number" ? (
                          <InputNumber
                            size="small"
                            min={0}
                            value={parseInt(editVal, 10) || 0}
                            onChange={(v) => setEditVal(String(v ?? 0))}
                            style={{ width: 100 }}
                          />
                        ) : (
                          <Input
                            size="small"
                            value={editVal}
                            onChange={(e) => setEditVal(e.target.value)}
                            onPressEnter={() => saveEdit(def)}
                            autoFocus
                            style={{ fontFamily: "monospace", fontSize: 12 }}
                            title={def.hint}
                          />
                        )}
                        <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={() => saveEdit(def)} />
                        <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancelEdit} />
                      </div>
                    ) : (
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <span style={{
                          fontFamily: "monospace",
                          color: strVal ? "var(--text)" : "var(--text-faint)",
                          fontStyle: strVal ? "normal" : "italic",
                          flex: 1,
                          wordBreak: "break-word",
                        }}>
                          {strVal || "—"}
                        </span>
                        <Button
                          size="small"
                          type="text"
                          icon={<EditOutlined />}
                          onClick={() => startEdit(def.key, strVal)}
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
      )}
    </div>
  );
}

export default function PropertiesModal({ title, rows, error, onClose, tableContext }: Props) {
  const loading = rows === null && !error;

  const copyAll = () => {
    if (!rows) return;
    const text = rows.map((r) => `${r.key}: ${r.value}`).join("\n");
    ClipboardSetText(text).then(() => message.success("Copied to clipboard"));
  };

  return (
    <Modal
      open
      title={title}
      onCancel={onClose}
      width={620}
      footer={[
        <Button
          key="copy"
          icon={<CopyOutlined />}
          disabled={!rows || rows.length === 0}
          onClick={copyAll}
        >
          Copy
        </Button>,
        <Button key="close" onClick={onClose}>
          Close
        </Button>,
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

      {rows && rows.length === 0 && !error && (
        <div style={{ color: "var(--text-muted)", fontSize: 13, padding: 8 }}>
          No properties found.
        </div>
      )}

      <div style={{ maxHeight: "65vh", overflowY: "auto" }}>
        {rows && rows.length > 0 && (
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              {rows.map((row) => (
                <tr
                  key={row.key}
                  style={{ borderBottom: "1px solid var(--border)" }}
                >
                  <td
                    style={{
                      padding: "5px 12px 5px 0",
                      color: "var(--text-muted)",
                      fontFamily: "monospace",
                      whiteSpace: "nowrap",
                      verticalAlign: "top",
                      width: 200,
                      minWidth: 160,
                    }}
                  >
                    {row.key}
                  </td>
                  <td
                    style={{
                      padding: "5px 0",
                      color: "var(--text)",
                      fontFamily: "monospace",
                      wordBreak: "break-word",
                      verticalAlign: "top",
                    }}
                  >
                    {row.value || <span style={{ color: "var(--text-muted)", fontStyle: "italic" }}>—</span>}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}

        {tableContext && <TableSettingsSection {...tableContext} />}
        {tableContext && <ColumnCommentsSection {...tableContext} />}
      </div>
    </Modal>
  );
}
