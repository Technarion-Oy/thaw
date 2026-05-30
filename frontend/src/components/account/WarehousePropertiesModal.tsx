// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, InputNumber, Switch, Select, Tag, Popconfirm, message,
} from "antd";
import {
  CopyOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PauseCircleOutlined, PlayCircleOutlined, StopOutlined, FontSizeOutlined, SearchOutlined,
} from "@ant-design/icons";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import {
  GetObjectProperties,
  GetWarehouseParameters,
  AlterWarehouseProperty,
  AlterWarehouseSuspend,
  AlterWarehouseResume,
  AlterWarehouseAbortAllQueries,
  AlterWarehouseRename,
} from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";

// ─── Size mappings ──────────────────────────────────────────────────────────

const SIZE_OPTIONS = [
  { label: "X-Small",  value: "XSMALL"   },
  { label: "Small",    value: "SMALL"     },
  { label: "Medium",   value: "MEDIUM"    },
  { label: "Large",    value: "LARGE"     },
  { label: "X-Large",  value: "XLARGE"   },
  { label: "2X-Large", value: "XXLARGE"  },
  { label: "3X-Large", value: "XXXLARGE" },
  { label: "4X-Large", value: "X4LARGE"  },
  { label: "5X-Large", value: "X5LARGE"  },
  { label: "6X-Large", value: "X6LARGE"  },
];

// Map display name from SHOW WAREHOUSES → SELECT value key.
function sizeToKey(showSize: string): string {
  const map: Record<string, string> = {
    "x-small":  "XSMALL",   "small":    "SMALL",    "medium":   "MEDIUM",
    "large":    "LARGE",    "x-large":  "XLARGE",   "2x-large": "XXLARGE",
    "3x-large": "XXXLARGE", "4x-large": "X4LARGE",  "5x-large": "X5LARGE",
    "6x-large": "X6LARGE",
  };
  return map[showSize.toLowerCase()] ?? showSize.toUpperCase();
}

const SCALING_OPTIONS = [
  { label: "Standard", value: "STANDARD" },
  { label: "Economy",  value: "ECONOMY"  },
];

const TYPE_OPTIONS = [
  { label: "Standard",            value: "STANDARD"          },
  { label: "Snowpark-Optimized",  value: "SNOWPARK-OPTIMIZED" },
];

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  marginBottom: 8, marginTop: 16,
};

// ─── Helper: one editable row ────────────────────────────────────────────────

interface RowProps {
  label:     string;
  value:     string;
  type:      "text" | "number" | "select" | "boolean";
  options?:  { label: string; value: string }[];
  min?:      number;
  max?:      number;
  saving?:   boolean;
  disabled?: boolean;
  hint?:     string;
  search?:   string;
  onSave:    (val: string) => Promise<void>;
}

function EditRow({ label, value, type, options, min, max, saving: externalSaving, disabled, hint, search, onSave }: RowProps) {
  const [editing,   setEditing]   = useState(false);
  const [editVal,   setEditVal]   = useState(value);
  const [saving,    setSaving]    = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const startEdit = () => { setEditing(true); setEditVal(value); setEditError(null); };
  const cancel    = () => { setEditing(false); setEditError(null); };

  const save = async () => {
    setSaving(true);
    setEditError(null);
    try {
      await onSave(editVal);
      setEditing(false);
    } catch (e) {
      // Strip gosnowflake noise — show only the human-readable part after the last ":"
      // e.g. "003001 (42501): SQL access control error:\nInsufficient privileges…"
      const raw = String(e);
      const match = raw.match(/Insufficient privileges[^\n]*/i) ?? raw.match(/:\s*(.+)$/s);
      setEditError(match ? match[0].trim() : raw);
    } finally {
      setSaving(false);
    }
  };

  const showSection = (labels: string[]) => {
    if (!search) return true;
    return labels.some(l => l.toLowerCase().includes(search.toLowerCase()));
  };

  if (!showSection([label])) return null;

  if (type === "boolean") {
    return (
      <tr style={{ borderBottom: "1px solid var(--border)" }}>
        <td style={LABEL_TD}>{label}</td>
        <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
          <Switch
            size="small"
            checked={value === "true" || value === "TRUE"}
            disabled={disabled || externalSaving}
            onChange={async (checked) => {
              try {
                await onSave(checked ? "TRUE" : "FALSE");
              } catch (e) {
                const raw = String(e);
                const match = raw.match(/Insufficient privileges[^\n]*/i) ?? raw.match(/:\s*(.+)$/s);
                message.error(match ? match[0].trim() : raw, 6);
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
                  options={options}
                  style={{ minWidth: 160 }}
                  autoFocus
                />
              ) : type === "number" ? (
                <InputNumber
                  size="small"
                  min={min ?? 0}
                  max={max}
                  value={parseInt(editVal, 10) || 0}
                  onChange={(v) => setEditVal(String(v ?? 0))}
                  style={{ width: 120 }}
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
            {!disabled && (
              <Button
                size="small" type="text" icon={<EditOutlined />}
                onClick={startEdit}
                style={{ flexShrink: 0, color: "var(--text-faint)" }}
              />
            )}
          </div>
        )}
      </td>
    </tr>
  );
}

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontFamily: "monospace", whiteSpace: "nowrap",
  verticalAlign: "middle", width: 200, minWidth: 160,
};

// ─── Helper: read-only info row ──────────────────────────────────────────────

function InfoRow({ label, value }: { label: string; value: string }) {
  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "5px 0", fontFamily: "monospace", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
        {value || <span style={{ color: "var(--text-faint)", fontStyle: "italic" }}>—</span>}
      </td>
    </tr>
  );
}

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  name:     string;
  onClose:  () => void;
  onRename: (newName: string) => void;
}

export default function WarehousePropertiesModal({ name: initialName, onClose, onRename }: Props) {
  const [name,       setName]       = useState(initialName);
  const [rows,       setRows]       = useState<app.PropertyPair[] | null>(null);
  const [params,     setParams]     = useState<app.PropertyPair[] | null>(null);
  const [loadError,  setLoadError]  = useState<string | null>(null);
  const [search,     setSearch]     = useState("");

  // Rename state
  const [renaming,     setRenaming]     = useState(false);
  const [renameVal,    setRenameVal]    = useState("");
  const [renameSaving, setRenameSaving] = useState(false);
  const [renameError,  setRenameError]  = useState<string | null>(null);

  // Action busy state
  const [suspending, setSuspending] = useState(false);
  const [resuming,   setResuming]   = useState(false);

  const load = useCallback(async (wh: string) => {
    setRows(null);
    setParams(null);
    setLoadError(null);
    try {
      const [propRows, paramRows] = await Promise.all([
        GetObjectProperties("", "", "WAREHOUSE", wh),
        GetWarehouseParameters(wh).catch(() => [] as app.PropertyPair[]),
      ]);
      setRows(propRows ?? []);
      setParams(paramRows ?? []);
    } catch (e) {
      setLoadError(String(e));
    }
  }, []);

  useEffect(() => { load(name); }, [name, load]);

  // Build a lookup from the SHOW WAREHOUSES row pairs.
  const get = (key: string) => rows?.find((r) => r.key === key)?.value ?? "";
  const getParam = (key: string) => params?.find((r) => r.key === key)?.value ?? "";

  const state = get("state");  // STARTED, SUSPENDED, RESUMING, QUIESCING, …

  // ── Property save helpers ──────────────────────────────────────────────────

  const setProp = async (property: string, value: string, label: string) => {
    await AlterWarehouseProperty(name, property, value);
    message.success(`${label} updated`);
    await load(name);
  };

  // ── Actions ────────────────────────────────────────────────────────────────

  const suspend = async () => {
    setSuspending(true);
    try {
      await AlterWarehouseSuspend(name);
      message.success("Warehouse suspended");
      await load(name);
    } catch (e) {
      message.error(String(e));
    } finally {
      setSuspending(false);
    }
  };

  const resume = async () => {
    setResuming(true);
    try {
      await AlterWarehouseResume(name);
      message.success("Warehouse resumed");
      await load(name);
    } catch (e) {
      message.error(String(e));
    } finally {
      setResuming(false);
    }
  };

  const abortAll = async () => {
    try {
      await AlterWarehouseAbortAllQueries(name);
      message.success("All queries aborted");
    } catch (e) {
      message.error(String(e));
    }
  };

  const startRename = () => { setRenaming(true); setRenameVal(name); setRenameError(null); };
  const cancelRename = () => { setRenaming(false); setRenameError(null); };

  const saveRename = async () => {
    const trimmed = renameVal.trim();
    if (!trimmed || trimmed === name) { setRenaming(false); return; }
    setRenameSaving(true);
    setRenameError(null);
    try {
      await AlterWarehouseRename(name, trimmed);
      message.success(`Renamed to ${trimmed}`);
      setName(trimmed);
      onRename(trimmed);
      setRenaming(false);
    } catch (e) {
      const raw = String(e);
      const match = raw.match(/Insufficient privileges[^\n]*/i) ?? raw.match(/:\s*(.+)$/s);
      setRenameError(match ? match[0].trim() : raw);
    } finally {
      setRenameSaving(false);
    }
  };

  // ── Copy all properties ────────────────────────────────────────────────────

  const copyAll = () => {
    if (!rows) return;
    const text = rows.map((r) => `${r.key}: ${r.value}`).join("\n");
    ClipboardSetText(text).then(() => message.success("Copied to clipboard"));
  };

  // ── State badge ────────────────────────────────────────────────────────────

  const stateColor = (): string => {
    switch (state.toUpperCase()) {
      case "STARTED":  return "green";
      case "SUSPENDED": return "orange";
      case "RESUMING": case "STARTING": return "blue";
      case "QUIESCING": return "gold";
      default: return "default";
    }
  };

  const isSuspended = state.toUpperCase() === "SUSPENDED";
  const isStarted   = state.toUpperCase() === "STARTED";

  // Multi-cluster: max_cluster_count > 1
  const maxCluster = parseInt(get("max_cluster_count") || "1", 10);
  const isMultiCluster = maxCluster > 1;

  // Auto-suspend: 0 = disabled
  const autoSuspendVal = get("auto_suspend");
  const autoSuspendDisplay = autoSuspendVal === "0" || autoSuspendVal === ""
    ? "0 (disabled)"
    : `${autoSuspendVal} s`;

  return (
    <Modal
      open
      title={`Warehouse: ${name}`}
      onCancel={onClose}
      width={660}
      styles={{ body: { maxHeight: "75vh", overflowY: "auto" } }}
      footer={[
        <Button key="copy" icon={<CopyOutlined />} disabled={!rows} onClick={copyAll}>Copy</Button>,
        <Button key="close" onClick={onClose}>Close</Button>,
      ]}
    >
      {/* ── Loading ─────────────────────────────────────────────────────────── */}
      {!rows && !loadError && (
        <div style={{ textAlign: "center", padding: "32px 0" }}><Spin /></div>
      )}
      {loadError && (
        <div style={{ color: "#f85149", fontFamily: "monospace", fontSize: 12, padding: 8 }}>{loadError}</div>
      )}

      {rows && (
        <>
          <div style={{ marginBottom: 16 }}>
            <Input
              prefix={<SearchOutlined style={{ color: "var(--text-faint)" }} />}
              placeholder="Search properties by name…"
              allowClear
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
          </div>

          {/* ── Status + actions ──────────────────────────────────────────── */}
          <div style={{ display: "flex", alignItems: "center", gap: 8, flexWrap: "wrap", marginBottom: 12 }}>
            <Tag color={stateColor()} style={{ fontWeight: 600, fontSize: 12 }}>{state || "—"}</Tag>
            <span style={{ fontSize: 11, color: "var(--text-muted)", fontFamily: "monospace" }}>
              {get("type")} · {get("size")} · owner: {get("owner")}
            </span>
            <div style={{ marginLeft: "auto", display: "flex", gap: 6 }}>
              {isStarted && (
                <Button size="small" icon={<PauseCircleOutlined />} loading={suspending} onClick={suspend}>
                  Suspend
                </Button>
              )}
              {isSuspended && (
                <Button size="small" icon={<PlayCircleOutlined />} loading={resuming} onClick={resume} type="primary">
                  Resume
                </Button>
              )}
              <Popconfirm
                title="Abort all running queries on this warehouse?"
                onConfirm={abortAll}
                okText="Abort"
                okButtonProps={{ danger: true }}
              >
                <Button size="small" icon={<StopOutlined />} danger>Abort All</Button>
              </Popconfirm>
              <Button size="small" icon={<FontSizeOutlined />} onClick={startRename}>Rename</Button>
            </div>
          </div>

          {/* ── Rename inline ─────────────────────────────────────────────── */}
          {renaming && (
            <div style={{ display: "flex", flexDirection: "column", gap: 4, marginBottom: 12 }}>
              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <Input
                  size="small"
                  value={renameVal}
                  onChange={(e) => { setRenameVal(e.target.value); setRenameError(null); }}
                  onPressEnter={saveRename}
                  autoFocus
                  style={{ fontFamily: "monospace", fontSize: 12, maxWidth: 280 }}
                  addonBefore="New name:"
                  status={renameError ? "error" : undefined}
                />
                <Button size="small" type="primary" icon={<CheckOutlined />} loading={renameSaving} onClick={saveRename} />
                <Button size="small" icon={<CloseOutlined />} disabled={renameSaving} onClick={cancelRename} />
              </div>
              {renameError && (
                <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace", lineHeight: 1.4, paddingLeft: 2 }}>
                  {renameError}
                </div>
              )}
            </div>
          )}

          {/* ── Compute ───────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Compute</div>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              <EditRow search={search} label="Warehouse Size"
                value={sizeToKey(get("size"))}
                type="select"
                options={SIZE_OPTIONS}
                onSave={async (v) => setProp("size", v, "Warehouse Size")}
              />
              <EditRow search={search} label="Warehouse Type"
                value={get("type").toUpperCase()}
                type="select"
                options={TYPE_OPTIONS}
                onSave={async (v) => setProp("warehouseType", v, "Warehouse Type")}
              />
              {isMultiCluster && (
                <>
                  <EditRow search={search} label="Max Cluster Count"
                    value={get("max_cluster_count")}
                    type="number"
                    min={1} max={10}
                    onSave={async (v) => setProp("maxClusterCount", v, "Max Cluster Count")}
                  />
                  <EditRow search={search} label="Min Cluster Count"
                    value={get("min_cluster_count")}
                    type="number"
                    min={1} max={10}
                    onSave={async (v) => setProp("minClusterCount", v, "Min Cluster Count")}
                  />
                  <EditRow search={search} label="Scaling Policy"
                    value={get("scaling_policy")}
                    type="select"
                    options={SCALING_OPTIONS}
                    onSave={async (v) => setProp("scalingPolicy", v, "Scaling Policy")}
                  />
                </>
              )}
            </tbody>
          </table>

          {/* ── Behavior ──────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Behavior</div>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              <EditRow search={search} label="Auto Suspend (s)"
                value={autoSuspendVal}
                type="number"
                min={0}
                hint="Seconds of inactivity before auto-suspend. Set to 0 to disable."
                onSave={async (v) => setProp("autoSuspend", v, "Auto Suspend")}
              />
              <tr style={{ borderBottom: "1px solid var(--border)" }}>
                <td style={LABEL_TD}>Auto Suspend Display</td>
                <td style={{ padding: "5px 0", fontFamily: "monospace", fontSize: 12, color: "var(--text-muted)" }}>
                  {autoSuspendDisplay}
                </td>
              </tr>
              <EditRow search={search} label="Auto Resume"
                value={get("auto_resume")}
                type="boolean"
                onSave={async (v) => setProp("autoResume", v, "Auto Resume")}
              />
            </tbody>
          </table>

          {/* ── Query Acceleration ────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Query Acceleration</div>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              <EditRow search={search} label="Enable"
                value={get("enable_query_acceleration")}
                type="boolean"
                onSave={async (v) => setProp("enableQueryAcceleration", v, "Query Acceleration")}
              />
              <EditRow search={search} label="Max Scale Factor"
                value={get("query_acceleration_max_scale_factor")}
                type="number"
                min={0} max={100}
                hint="0–100 (0 = unlimited)"
                onSave={async (v) => setProp("queryAccelerationMaxScaleFactor", v, "Max Scale Factor")}
              />
            </tbody>
          </table>

          {/* ── Resource & Timeouts ───────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Resource & Timeouts</div>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              <EditRow search={search} label="Resource Monitor"
                value={get("resource_monitor")}
                type="text"
                hint="Name of resource monitor, or empty to clear"
                onSave={async (v) => setProp("resourceMonitor", v, "Resource Monitor")}
              />
              <EditRow search={search} label="Max Concurrency Level"
                value={getParam("MAX_CONCURRENCY_LEVEL")}
                type="number"
                min={1}
                hint="Max number of concurrent queries per cluster"
                onSave={async (v) => setProp("maxConcurrencyLevel", v, "Max Concurrency Level")}
              />
              <EditRow search={search} label="Statement Queued Timeout (s)"
                value={getParam("STATEMENT_QUEUED_TIMEOUT_IN_SECONDS")}
                type="number"
                min={0}
                hint="Seconds a query can queue before being cancelled (0 = no limit)"
                onSave={async (v) => setProp("statementQueuedTimeout", v, "Statement Queued Timeout")}
              />
              <EditRow search={search} label="Statement Timeout (s)"
                value={getParam("STATEMENT_TIMEOUT_IN_SECONDS")}
                type="number"
                min={0}
                hint="Seconds a query can run before being cancelled (0 = no limit)"
                onSave={async (v) => setProp("statementTimeout", v, "Statement Timeout")}
              />
            </tbody>
          </table>

          {/* ── General ───────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>General</div>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              <EditRow search={search} label="Comment"
                value={get("comment")}
                type="text"
                onSave={async (v) => setProp("comment", v, "Comment")}
              />
            </tbody>
          </table>

          {/* ── Read-only info ────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Info</div>
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              <InfoRow label="owner"       value={get("owner")} />
              <InfoRow label="created_on"  value={get("created_on")} />
              <InfoRow label="resumed_on"  value={get("resumed_on")} />
              <InfoRow label="updated_on"  value={get("updated_on")} />
              <InfoRow label="running"     value={get("running")} />
              <InfoRow label="queued"      value={get("queued")} />
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
