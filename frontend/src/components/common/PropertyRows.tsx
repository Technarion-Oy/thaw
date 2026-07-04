// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Shared building blocks for the per-property "Properties: <OBJECT>" modals
// (WarehousePropertiesModal, UserPropertiesModal): a typed inline-editable
// property row, a read-only info row, and the section/label styles they share.

import { useState } from "react";
import { Input, InputNumber, Select, Switch, Button, message } from "antd";
import { EditOutlined, CheckOutlined, CloseOutlined } from "@ant-design/icons";

export const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  marginBottom: 8, marginTop: 16,
};

export const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontFamily: "monospace", whiteSpace: "nowrap",
  verticalAlign: "middle", width: 200, minWidth: 160,
};

// Strip gosnowflake noise — show only the human-readable part after the last
// ":" (e.g. "003001 (42501): SQL access control error:\nInsufficient
// privileges…" → "Insufficient privileges…").
export function friendlyError(e: unknown): string {
  const raw = String(e);
  const priv = raw.match(/Insufficient privileges[^\n]*/i);
  if (priv) return priv[0].trim();
  // The greedy [\s\S]* prefix pushes the match to the LAST colon; capture
  // what follows it (must start with a non-space so an empty tail falls back).
  const m = raw.match(/^[\s\S]*:\s*(\S[\s\S]*)$/);
  return m ? m[1].trim() : raw;
}

export interface Option { label: string; value: string }

export interface EditRowProps {
  label:    string;
  value:    string;
  type:     "text" | "number" | "select" | "boolean";
  /** Static select options. */
  options?: Option[];
  /** Lazy select options — fetched once, the first time the row enters edit
   *  mode, so modals don't front-load lists the user may never open. */
  loadOptions?: () => Promise<Option[]>;
  min?:     number;
  max?:     number;
  /** Number rows only: when true, clearing the input yields "" (the caller
   *  treats it as UNSET). Default false — a cleared input coerces to "0",
   *  preserving the original WarehousePropertiesModal behavior whose builder
   *  rejects empty strings. */
  allowEmpty?: boolean;
  hint?:    string;
  search?:  string;
  onSave:   (val: string) => Promise<void>;
}

export function EditRow({ label, value, type, options, loadOptions, min, max, allowEmpty, hint, search, onSave }: EditRowProps) {
  const [editing,   setEditing]   = useState(false);
  const [editVal,   setEditVal]   = useState(value);
  const [saving,    setSaving]    = useState(false);
  const [editError, setEditError] = useState<string | null>(null);
  const [lazyOpts,  setLazyOpts]  = useState<Option[] | null>(null);

  const startEdit = () => {
    setEditing(true);
    setEditVal(value);
    setEditError(null);
    if (loadOptions && lazyOpts === null) {
      loadOptions().then(setLazyOpts).catch(() => setLazyOpts([]));
    }
  };
  const cancel = () => { setEditing(false); setEditError(null); };

  const save = async () => {
    // No-op guard: saving an untouched value must not fire an ALTER — for
    // values the read path can't surface, that would silently UNSET them.
    if (editVal === value) { setEditing(false); return; }
    setSaving(true);
    setEditError(null);
    try {
      await onSave(editVal);
      setEditing(false);
    } catch (e) {
      setEditError(friendlyError(e));
    } finally {
      setSaving(false);
    }
  };

  if (search && !label.toLowerCase().includes(search.toLowerCase())) return null;

  if (type === "boolean") {
    return (
      <tr style={{ borderBottom: "1px solid var(--border)" }}>
        <td style={LABEL_TD}>{label}</td>
        <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
          <Switch
            size="small"
            checked={value.toLowerCase() === "true"}
            disabled={saving}
            onChange={async (checked) => {
              setSaving(true);
              try {
                await onSave(checked ? "TRUE" : "FALSE");
              } catch (e) {
                message.error(friendlyError(e), 6);
              } finally {
                setSaving(false);
              }
            }}
          />
        </td>
      </tr>
    );
  }

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
        {editing ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              {type === "select" ? (
                <Select
                  size="small"
                  value={editVal}
                  onChange={setEditVal}
                  options={lazyOpts ?? options}
                  loading={loadOptions !== undefined && lazyOpts === null}
                  showSearch
                  style={{ minWidth: 180 }}
                  autoFocus
                />
              ) : type === "number" ? (
                <InputNumber
                  size="small"
                  min={min ?? 0}
                  max={max}
                  value={editVal === "" ? undefined : parseInt(editVal, 10)}
                  onChange={(v) => setEditVal(
                    v === null || v === undefined
                      ? (allowEmpty ? "" : String(min ?? 0))
                      : String(v),
                  )}
                  placeholder={allowEmpty ? "— unset —" : undefined}
                  style={{ width: 140 }}
                  title={hint}
                />
              ) : (
                <Input
                  size="small"
                  value={editVal}
                  onChange={(e) => setEditVal(e.target.value)}
                  onPressEnter={save}
                  autoFocus
                  style={{ fontFamily: "monospace", fontSize: 12 }}
                  title={hint}
                  status={editError ? "error" : undefined}
                />
              )}
              <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={save} />
              <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancel} />
            </div>
            {hint && !editError && (
              <div style={{ color: "var(--text-faint)", fontSize: 11, paddingLeft: 2 }}>{hint}</div>
            )}
            {editError && (
              <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace", lineHeight: 1.4, paddingLeft: 2 }}>
                {editError}
              </div>
            )}
          </div>
        ) : (
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{
              fontFamily: "monospace", fontSize: 12,
              color: value ? "var(--text)" : "var(--text-faint)",
              fontStyle: value ? "normal" : "italic",
              flex: 1, wordBreak: "break-word",
            }}>
              {value || "—"}
            </span>
            <Button
              size="small" type="text" icon={<EditOutlined />}
              onClick={startEdit}
              style={{ flexShrink: 0, color: "var(--text-faint)" }}
            />
          </div>
        )}
      </td>
    </tr>
  );
}

export function InfoRow({ label, value, search, extra }: { label: string; value: string; search?: string; extra?: React.ReactNode }) {
  if (search && !label.toLowerCase().includes(search.toLowerCase())) return null;
  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "5px 0", fontFamily: "monospace", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
        {value || <span style={{ color: "var(--text-faint)", fontStyle: "italic" }}>—</span>}
        {extra}
      </td>
    </tr>
  );
}
