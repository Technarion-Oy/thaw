// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration
//
// Shared building blocks for the per-object-type Properties modals
// (SchemaPropertiesModal, DatabasePropertiesModal, …): the two-column table row
// components and the small SQL/parameter helpers they all use. Kept here so a
// fix to one row's behavior lands in every Properties modal at once.

import { useState, useEffect } from "react";
import { Button, Input, Select, Space, Typography, Tooltip } from "antd";
import { EditOutlined, CheckOutlined, CloseOutlined } from "@ant-design/icons";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// ─── Styles ──────────────────────────────────────────────────────────────────

export const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

export const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle",
  width: 220,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

// Escape a SQL text literal the way the backend's EscapeTextLit does — double
// backslashes (Snowflake interprets backslash escapes in string literals) then
// single quotes — so a value like C:\temp round-trips intact.
export function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

// Build a fixed-choice option list ({value,label}) from bare strings.
export const opts = (...vs: string[]) => vs.map((v) => ({ value: v, label: v }));

// Pull a parameter's current value out of a SHOW PARAMETERS result (columns are
// key / value / default / …; we want the row whose key matches, case-insensitive).
export function paramValue(params: snowflake.QueryResult | null, key: string): string {
  if (!params) return "";
  const cols = (params.columns ?? []).map((c) => c.toLowerCase());
  const keyCi = cols.indexOf("key");
  const valCi = cols.indexOf("value");
  if (keyCi < 0 || valCi < 0) return "";
  const row = (params.rows ?? []).find((r) => String(r[keyCi] ?? "").toLowerCase() === key.toLowerCase());
  return row ? String(row[valCi] ?? "") : "";
}

// ─── EditRow ─────────────────────────────────────────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  canUnset?: boolean;
  onSave: (val: string) => Promise<void>;
  onUnset?: () => Promise<void>;
}

export function EditRow({ label, value, canUnset, onSave, onUnset }: EditRowProps) {
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
                <Tooltip title="Unset (reset to default)">
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

// ─── InfoRow ─────────────────────────────────────────────────────────────────

export function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
        {value || <Text type="secondary">(empty)</Text>}
      </td>
    </tr>
  );
}

// ─── SelectRow ───────────────────────────────────────────────────────────────

// A fixed-choice parameter row: a Select that applies the change on pick, plus an
// Unset button (reset to default / inherited) when a value is set. The Unset
// button is only shown when onUnset is supplied — some parameters are settable
// but not UNSET-able (e.g. LOG_LEVEL / TRACE_LEVEL at the database level).
export function SelectRow({ label, value, options, busy, onSet, onUnset }: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  busy: boolean;
  onSet: (v: string) => void;
  onUnset?: () => void;
}) {
  const cur = value ? value.toUpperCase() : undefined;
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        <Space>
          <Select
            size="small"
            value={cur}
            placeholder="(default)"
            style={{ width: 200 }}
            options={options}
            onChange={onSet}
            loading={busy}
          />
          {cur && onUnset && (
            <Tooltip title="Unset (reset to default)">
              <Button size="small" onClick={onUnset} loading={busy}>Unset</Button>
            </Tooltip>
          )}
        </Space>
      </td>
    </tr>
  );
}

// ─── PickerRow ───────────────────────────────────────────────────────────────

// An identifier-valued parameter row: a searchable Select populated from a live
// list (external volumes, catalog integrations, compute pools, warehouses …).
// The picked name is set case-sensitively (double-quoted) by the caller's onSet;
// onUnset clears it. If the list read fails the current value is still shown and
// unsettable — a fresh pick just isn't offered (use the SQL editor instead).
export function PickerRow({ label, value, load, busy, onSet, onUnset }: {
  label: string;
  value: string;
  load: () => Promise<string[]>;
  busy: boolean;
  onSet: (name: string) => void;
  onUnset: () => void;
}) {
  const [names, setNames] = useState<string[]>([]);
  const [loadErr, setLoadErr] = useState(false);
  useEffect(() => {
    load().then((ns) => setNames(ns ?? [])).catch(() => setLoadErr(true));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps
  // Always include the current value so it renders even if the list omitted it.
  const options = Array.from(new Set([...(value ? [value] : []), ...names]))
    .map((n) => ({ value: n, label: n }));
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        <Space>
          <Select
            size="small"
            showSearch
            value={value || undefined}
            placeholder={loadErr ? "(list unavailable)" : "(not set)"}
            style={{ width: 240 }}
            options={options}
            onChange={onSet}
            loading={busy}
          />
          {value && (
            <Tooltip title="Unset">
              <Button size="small" onClick={onUnset} loading={busy}>Unset</Button>
            </Tooltip>
          )}
        </Space>
      </td>
    </tr>
  );
}
