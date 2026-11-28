// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useCallback, useEffect, useState } from "react";
import { Modal, Spin, Button, Input, Tooltip, Tag, message } from "antd";
import {
  CopyOutlined, EditOutlined, CheckOutlined, CloseOutlined, SearchOutlined, RollbackOutlined,
} from "@ant-design/icons";
import { ConfirmSwitch } from "./ConfirmSwitch";
import { friendlyError } from "./errors";
import { ClipboardSetText } from "../../utils/wailsRuntime";
import type { snowflake } from "../../../wailsjs/go/models";
import {
  GetObjectParameters, SetObjectParameter, UnsetObjectParameter,
} from "../../../wailsjs/go/app/App";

export type ParamObjectType = "DATABASE" | "SCHEMA" | "TABLE" | "WAREHOUSE" | "TASK" | "USER";

interface Props {
  objectType: ParamObjectType;
  // Identifier parts of the object's qualified name: ["DB"], ["DB","SCHEMA"], ["WH"].
  nameParts: string[];
  // Display label for the modal header, e.g. `"MY_DB"."MY_SCHEMA"`.
  title: string;
  // Render parameter values without edit/reset controls — used for read-only
  // system objects (e.g. INFORMATION_SCHEMA) where ALTER … SET always fails.
  readOnly?: boolean;
  onClose: () => void;
}

// ── shared styles (mirrors AccountParametersModal) ───────────────────────────

const LABEL_CELL: React.CSSProperties = {
  padding: "5px 12px 5px 0",
  color: "var(--text-muted)",
  fontFamily: "monospace",
  whiteSpace: "nowrap",
  verticalAlign: "middle",
  width: 260,
  minWidth: 200,
};

const LEVEL_CELL: React.CSSProperties = {
  padding: "5px 12px 5px 0",
  verticalAlign: "middle",
  width: 110,
  whiteSpace: "nowrap",
};

const VALUE_CELL: React.CSSProperties = {
  padding: "4px 0",
  verticalAlign: "middle",
};

function isBool(type: string) {
  return type.toUpperCase() === "BOOLEAN";
}

// ── Editable parameters table ─────────────────────────────────────────────────
//
// Editing issues ALTER <TYPE> <name> SET/UNSET and requires ownership or the
// relevant privilege on the object; unprivileged saves fail with a Snowflake
// privilege error surfaced via message.error, leaving the displayed value
// unchanged. `level` highlights values overridden at THIS object (green) vs
// inherited from a higher scope (muted), and gates the reset-to-inherited action.

function ParamsTable({
  rows,
  objectType,
  readOnly,
  onSet,
  onUnset,
}: {
  rows: snowflake.SessionParam[];
  objectType: ParamObjectType;
  readOnly?: boolean;
  onSet: (row: snowflake.SessionParam, value: string) => Promise<void>;
  onUnset: (row: snowflake.SessionParam) => Promise<void>;
}) {
  const [editKey, setEditKey] = useState<string | null>(null);
  const [editVal, setEditVal] = useState("");
  const [saving, setSaving] = useState(false);

  const startEdit = (key: string, current: string) => { setEditKey(key); setEditVal(current); };
  const cancel = () => { setEditKey(null); setEditVal(""); };

  const save = async (row: snowflake.SessionParam) => {
    setSaving(true);
    try {
      await onSet(row, editVal);
      setEditKey(null);
    } catch (e) {
      message.error(friendlyError(e));
    } finally {
      setSaving(false);
    }
  };

  return (
    <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
      <tbody>
        {rows.map((row) => {
          const isEditing = editKey === row.key;
          // Value is set on THIS object when the level equals its object type;
          // any other non-empty level is inherited from a higher scope.
          const setHere = row.level.toUpperCase() === objectType;
          const level = row.level.toUpperCase();
          return (
            <tr key={row.key} style={{ borderBottom: "1px solid var(--border)" }}>
              <td style={LABEL_CELL}>
                <Tooltip title={row.description || undefined} placement="right" mouseEnterDelay={0.3}>
                  <span style={{ cursor: "help", borderBottom: "1px dotted var(--text-muted)" }}>{row.key}</span>
                </Tooltip>
              </td>
              <td style={LEVEL_CELL}>
                {level
                  ? <Tag color={setHere ? "green" : "default"} style={{ fontSize: 10, marginInlineEnd: 0 }}>{level}</Tag>
                  : <span style={{ color: "var(--text-faint)", fontSize: 11 }}>default</span>}
              </td>
              <td style={VALUE_CELL}>
                {readOnly ? (
                  <span style={{
                    fontFamily: "monospace",
                    color: row.value ? "var(--text)" : "var(--text-faint)",
                    fontStyle: row.value ? "normal" : "italic",
                    wordBreak: "break-word",
                  }}>
                    {row.value || "—"}
                  </span>
                ) : isBool(row.type) ? (
                  <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                    <ConfirmSwitch
                      checked={row.value.toUpperCase() === "TRUE"}
                      onConfirm={(checked) => onSet(row, checked ? "TRUE" : "FALSE")}
                    />
                    {setHere && (
                      <Tooltip title="Reset to inherited value">
                        <Button size="small" type="text" icon={<RollbackOutlined />}
                          onClick={() => onUnset(row)} style={{ color: "var(--text-faint)" }} />
                      </Tooltip>
                    )}
                  </div>
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
                    {setHere && (
                      <Tooltip title="Reset to inherited value">
                        <Button size="small" type="text" icon={<RollbackOutlined />}
                          onClick={() => onUnset(row)} style={{ flexShrink: 0, color: "var(--text-faint)" }} />
                      </Tooltip>
                    )}
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

export default function ObjectParametersModal({ objectType, nameParts, title, readOnly, onClose }: Props) {
  const [params, setParams] = useState<snowflake.SessionParam[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [search, setSearch] = useState("");

  // nameParts is a fresh array per render; join to a stable dependency key.
  const partsKey = nameParts.join(" ");

  const reload = useCallback(async () => {
    setError(null);
    try {
      const p = await GetObjectParameters(objectType, nameParts);
      setParams(p ?? []);
    } catch (e) {
      setError(String(e));
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [objectType, partsKey]);

  useEffect(() => { reload(); }, [reload]);

  const loading = params === null && !error;
  const q = search.trim().toLowerCase();
  const filtered = params ? (q ? params.filter((p) => p.key.toLowerCase().includes(q)) : params) : null;

  const onSet = async (row: snowflake.SessionParam, value: string) => {
    await SetObjectParameter(objectType, nameParts, row.key, value, row.type);
    message.success(`${row.key} updated`);
    await reload();
  };

  const onUnset = async (row: snowflake.SessionParam) => {
    try {
      await UnsetObjectParameter(objectType, nameParts, row.key);
      message.success(`${row.key} reset to inherited value`);
      await reload();
    } catch (e) {
      message.error(friendlyError(e));
    }
  };

  const copyAll = () => {
    if (!params || params.length === 0) return;
    const lines = params.map((r) => `${r.key}: ${r.value}`);
    ClipboardSetText(lines.join("\n")).then(() => message.success("Copied to clipboard"));
  };

  return (
    <Modal
      open
      title={<span>Parameters <span style={{ color: "var(--text-muted)", fontSize: 12, fontFamily: "monospace", fontWeight: 400 }}>{title}</span></span>}
      onCancel={onClose}
      width={760}
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
            {readOnly ? (
              <>Read-only — this is a system object whose parameters can't be altered. A <Tag color="green" style={{ fontSize: 10 }}>{objectType}</Tag> level means the value is set here; anything else is inherited from a higher scope.</>
            ) : (
              <>Editing applies <code>ALTER {objectType} … SET</code> and requires ownership or the relevant privilege — saves fail with a privilege error otherwise. A <Tag color="green" style={{ fontSize: 10 }}>{objectType}</Tag> level means the value is set here; reset it to inherit from the higher scope.</>
            )}
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
                ? <ParamsTable rows={filtered} objectType={objectType} readOnly={readOnly} onSet={onSet} onUnset={onUnset} />
                : <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0 8px" }}>
                    {params && params.length > 0
                      ? "No matches."
                      : "No parameters visible. Your current role may lack the privileges to view them."}
                  </div>
              : <Spin size="small" />}
          </div>
        </>
      )}
    </Modal>
  );
}
