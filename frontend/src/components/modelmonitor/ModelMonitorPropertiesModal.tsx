// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tag, Tooltip, Popconfirm,
} from "antd";
import {
  LineChartOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PauseCircleOutlined, PlayCircleOutlined, PlusOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterModelMonitor, ListWarehouses, ParseSqlList } from "../../../wailsjs/go/app/App";
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

// ─── EditRow ─────────────────────────────────────────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  help?: string;
  onSave: (val: string) => Promise<void>;
}

function EditRow({ label, value, help, onSave }: EditRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    if (!draft.trim()) return;
    setSaving(true);
    setError(null);
    try {
      await onSave(draft.trim());
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
              <Tooltip title="Save"><Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={!draft.trim()} /></Tooltip>
              <Tooltip title="Cancel"><Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} /></Tooltip>
            </Space>
            {help && <Text type="secondary" style={{ fontSize: 11 }}>{help}</Text>}
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">(not set)</Text>}</span>
            <Tooltip title="Edit">
              <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />} onClick={() => { setDraft(value); setEditing(true); }} style={{ color: "var(--text-muted)" }} />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

// ─── SelectEditRow (warehouse) ───────────────────────────────────────────────

interface SelectEditRowProps {
  label: string;
  value: string;
  options: string[];
  loading?: boolean;
  onSave: (val: string) => Promise<void>;
}

function SelectEditRow({ label, value, options, loading, onSave }: SelectEditRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const save = async () => {
    if (!draft) return;
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

  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Select
                size="small"
                showSearch
                loading={loading}
                value={draft || undefined}
                onChange={(v) => setDraft(v ?? "")}
                style={{ width: 280 }}
                placeholder="Select warehouse…"
                options={(options || []).map((n) => ({ value: n, label: n }))}
                notFoundContent={loading ? "Loading…" : "No warehouses found"}
              />
              <Tooltip title="Save"><Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={!draft} /></Tooltip>
              <Tooltip title="Cancel"><Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} /></Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">(not set)</Text>}</span>
            <Tooltip title="Edit">
              <Button type="text" size="small" icon={<EditOutlined style={{ fontSize: 11 }} />} onClick={() => { setDraft(value); setEditing(true); }} style={{ color: "var(--text-muted)" }} />
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

export default function ModelMonitorPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);
  const [newSegment, setNewSegment] = useState("");
  const [addingSegment, setAddingSegment] = useState(false);
  const [segments, setSegments] = useState<string[]>([]);

  useEffect(() => {
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "MODEL MONITOR", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  // Parse the segment-column list from the (variably-shaped) SHOW cell via the
  // shared backend tokenizer (App.ParseSqlList handles SQL tuples, bracketed
  // lists, and JSON arrays — quote/comma-safe). It is async, so the result lives
  // in state rather than being computed inline during render.
  useEffect(() => {
    const get = (key: string) => rows?.find((r) => r.key.toLowerCase() === key)?.value ?? "";
    const raw = get("segment_columns") || get("segment_column");
    if (!raw.trim()) { setSegments([]); return; }
    let cancelled = false;
    ParseSqlList(raw)
      .then((toks) => { if (!cancelled) setSegments(toks ?? []); })
      .catch(() => { if (!cancelled) setSegments([]); });
    return () => { cancelled = true; };
  }, [rows]);

  const monitorRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const runAlter = async (clause: string, label: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterModelMonitor(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
      throw e;
    } finally {
      setBusy(false);
    }
  };

  const saveBaseline = (v: string) => runAlter(`SET BASELINE = ${q1(v)}`, "Set baseline");
  const saveRefreshInterval = (v: string) => runAlter(`SET REFRESH_INTERVAL = ${q1(v)}`, "Set refresh interval");
  const saveWarehouse = (wh: string) => runAlter(`SET WAREHOUSE = "${wh.replace(/"/g, '""')}"`, "Set warehouse");

  const addSegment = async () => {
    const seg = newSegment.trim();
    if (!seg) return;
    // Snowflake allows at most 5 segment columns per monitor; match the create
    // modal's cap so both surfaces are consistent.
    if (segments.length >= 5) {
      setActionError("A model monitor can have at most 5 segment columns.");
      return;
    }
    setAddingSegment(true);
    try {
      await runAlter(`ADD segment_column = ${q1(seg)}`, "Add segment column");
      setNewSegment("");
    } catch { /* surfaced via actionError */ } finally {
      setAddingSegment(false);
    }
  };

  const dropSegment = (seg: string) => runAlter(`DROP segment_column = ${q1(seg)}`, "Drop segment column").catch(() => {});

  const baseline = find("baseline");
  const refreshInterval = find("refresh_interval");
  const warehouse = find("warehouse");
  const state = find("state") || find("status") || find("scheduling_state");

  // Keys handled by the editable Settings section (rendered above the raw table).
  const handledKeys = new Set([
    "baseline", "refresh_interval", "warehouse",
    "state", "status", "scheduling_state",
    "segment_columns", "segment_column",
  ]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <LineChartOutlined style={{ color: "var(--link)" }} />
          <span>Model Monitor Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>{monitorRef}</Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={720}
      styles={{ body: { maxHeight: "74vh", overflowY: "auto", paddingTop: 16 } }}
    >
      {!rows && !error && (
        <div style={{ textAlign: "center", padding: 32 }}><Spin /></div>
      )}
      {error && (
        <Alert type="error" message="Failed to load properties" description={error} showIcon />
      )}
      {rows && (
        <>
          {actionError && (
            <Alert type="error" message={actionError} showIcon closable onClose={() => setActionError(null)} style={{ marginBottom: 12 }} />
          )}

          <Space wrap>
            {state && (
              <Tag color={/active|running|started/i.test(state) ? "green" : "orange"}>{state}</Tag>
            )}
            <Button size="small" icon={<PauseCircleOutlined />} loading={busy} onClick={() => runAlter("SUSPEND", "Suspend").catch(() => {})}>
              Suspend
            </Button>
            <Button size="small" icon={<PlayCircleOutlined />} loading={busy} onClick={() => runAlter("RESUME", "Resume").catch(() => {})}>
              Resume
            </Button>
          </Space>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="Baseline" value={baseline} onSave={saveBaseline} />
              <EditRow label="Refresh Interval" value={refreshInterval} help="e.g. 1 hour, 30 minutes, 1 day" onSave={saveRefreshInterval} />
              <SelectEditRow label="Warehouse" value={warehouse} options={warehouses} loading={loadingWarehouses} onSave={saveWarehouse} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Segment Columns</div>
          <Space wrap size={[4, 8]} style={{ marginBottom: 8 }}>
            {segments.length === 0 && <Text type="secondary" style={{ fontSize: 12 }}>(none)</Text>}
            {segments.map((s) => (
              <Popconfirm
                key={s}
                title={`Drop segment column "${s}"?`}
                onConfirm={() => dropSegment(s)}
                okText="Drop"
                okButtonProps={{ danger: true }}
              >
                <Tag closable onClose={(e) => e.preventDefault()} color="blue">{s}</Tag>
              </Popconfirm>
            ))}
          </Space>
          <Space>
            <Input
              size="small"
              value={newSegment}
              onChange={(e) => setNewSegment(e.target.value)}
              placeholder="Segment column name"
              style={{ width: 240 }}
              onPressEnter={addSegment}
              disabled={addingSegment || segments.length >= 5}
            />
            <Button size="small" icon={<PlusOutlined />} onClick={addSegment} loading={addingSegment} disabled={!newSegment.trim() || segments.length >= 5}>
              Add
            </Button>
            {segments.length >= 5 && <Text type="secondary" style={{ fontSize: 11 }}>Maximum 5 reached</Text>}
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
        </>
      )}
    </Modal>
  );
}
