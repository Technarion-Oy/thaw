// SPDX-License-Identifier: GPL-3.0-or-later

import React, { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, InputNumber, Select, AutoComplete, Tag, Space,
  Typography, Tooltip, message, Alert,
} from "antd";
import {
  ClockCircleOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PlayCircleOutlined, PauseCircleOutlined, PlusOutlined, DeleteOutlined, FlagOutlined, SearchOutlined, HistoryOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, AlterTask, ListNotificationIntegrations, ListFinalizableTasks, TaskHasChildren, GetTaskStatuses, SuspendTaskList, ResumeTaskList } from "../../../wailsjs/go/app/App";
import CreateTaskModal from "./CreateTaskModal";
import TaskHistoryModal from "./TaskHistoryModal";
import type { snowflake, tasks } from "../../../wailsjs/go/models";
import { parsePredecessors as parsePredecessorsUtil, extractName } from "../../utils/taskHierarchy";
import WhenConditionBuilder from "./WhenConditionBuilder";
import ScheduleEditor from "./ScheduleEditor";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";

const { Text } = Typography;
const { TextArea } = Input;

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

// ─── Options ─────────────────────────────────────────────────────────────────

const OVERLAP_OPTIONS = [
  { label: "No Overlap",         value: "NO_OVERLAP"          },
  { label: "Allow Child Overlap", value: "ALLOW_CHILD_OVERLAP" },
  { label: "Allow All Overlap",  value: "ALLOW_ALL_OVERLAP"   },
];

const SIZE_OPTIONS = [
  { label: "X-Small",  value: "XSMALL"  },
  { label: "Small",    value: "SMALL"   },
  { label: "Medium",   value: "MEDIUM"  },
  { label: "Large",    value: "LARGE"   },
  { label: "X-Large",  value: "XLARGE"  },
  { label: "2X-Large", value: "XXLARGE" },
];

const LOG_LEVEL_OPTIONS = [
  { label: "TRACE",   value: "TRACE"   },
  { label: "DEBUG",   value: "DEBUG"   },
  { label: "INFO",    value: "INFO"    },
  { label: "WARNING", value: "WARNING" },
  { label: "ERROR",   value: "ERROR"   },
  { label: "FATAL",   value: "FATAL"   },
  { label: "OFF",     value: "OFF"     },
];

// ─── Helpers ─────────────────────────────────────────────────────────────────

function q1(s: string) { return "'" + s.replace(/'/g, "''") + "'"; }
function isValidJson(s: string) { try { JSON.parse(s); return true; } catch { return false; } }

/** Parse the predecessors column, which may be a JSON array string or empty. */
function parsePredecessors(raw: string): string[] {
  if (!raw || raw === "[]") return [];
  try {
    const arr = JSON.parse(raw);
    if (Array.isArray(arr)) return arr.map(String);
  } catch { /* fall through */ }
  // comma-separated fallback
  return raw.split(",").map((s) => s.trim()).filter(Boolean);
}

// ─── EditRow component ────────────────────────────────────────────────────────

interface RowProps {
  label:     string;
  value:     string;
  type:      "text" | "number" | "select" | "textarea";
  options?:  { label: string; value: string }[];
  min?:      number;
  hint?:     string;
  canUnset?: boolean;
  search?:   string;
  onSave:    (val: string) => Promise<void>;
  onUnset?:  () => Promise<void>;
}

function EditRow({ label, value, type, options, min, hint, canUnset, search, onSave, onUnset }: RowProps) {
  const [editing,   setEditing]   = useState(false);
  const [editVal,   setEditVal]   = useState(value);
  const [saving,    setSaving]    = useState(false);
  const [editError, setEditError] = useState<string | null>(null);
  const [unsetting, setUnsetting] = useState(false);

  const startEdit = () => { setEditing(true); setEditVal(value); setEditError(null); };
  const cancel    = () => { setEditing(false); setEditError(null); };

  const save = async () => {
    setSaving(true); setEditError(null);
    try { await onSave(editVal); setEditing(false); }
    catch (e) {
      const raw = String(e);
      const m = raw.match(/Insufficient privileges[^\n]*/i) ?? raw.match(/:\s*(.+)$/s);
      setEditError(m ? m[0].trim() : raw);
    } finally { setSaving(false); }
  };

  const doUnset = async () => {
    if (!onUnset) return;
    setUnsetting(true);
    try { await onUnset(); }
    catch (e) { message.error(String(e)); }
    finally { setUnsetting(false); }
  };

  const showSection = (labels: string[]) => {
    if (!search) return true;
    return labels.some(l => l.toLowerCase().includes(search.toLowerCase()));
  };

  if (!showSection([label])) return null;

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
        {editing ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <div style={{ display: "flex", gap: 6, alignItems: "flex-start" }}>
              {type === "select" ? (
                <Select size="small" value={editVal} onChange={setEditVal}
                  options={options} style={{ minWidth: 180 }} autoFocus />
              ) : type === "number" ? (
                <InputNumber size="small" min={min ?? 0}
                  value={parseInt(editVal, 10) || 0}
                  onChange={(v) => setEditVal(String(v ?? 0))}
                  style={{ width: 130 }} title={hint} />
              ) : type === "textarea" ? (
                <TextArea value={editVal} onChange={(e) => setEditVal(e.target.value)}
                  autoSize={{ minRows: 3, maxRows: 12 }}
                  style={{ fontFamily: "monospace", fontSize: 12, flex: 1 }}
                  status={editError ? "error" : undefined} autoFocus />
              ) : (
                <Input size="small" value={editVal}
                  onChange={(e) => setEditVal(e.target.value)}
                  onPressEnter={save} autoFocus
                  style={{ fontFamily: "monospace", fontSize: 12 }}
                  title={hint} status={editError ? "error" : undefined} />
              )}
              <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={save} />
              <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancel} />
            </div>
            {editError && (
              <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace",
                lineHeight: 1.4, paddingLeft: 2 }}>
                {editError}
              </div>
            )}
          </div>
        ) : (
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{
              fontFamily: type === "select" ? undefined : "monospace",
              fontSize: 12,
              color: value ? "var(--text)" : "var(--text-faint)",
              fontStyle: value ? "normal" : "italic",
              flex: 1, wordBreak: "break-word",
              whiteSpace: type === "textarea" ? "pre-wrap" : undefined,
            }}>
              {value || "—"}
            </span>
            <Button size="small" type="text" icon={<EditOutlined />} onClick={startEdit}
              style={{ flexShrink: 0, color: "var(--text-faint)" }} />
            {canUnset && value && onUnset && (
              <Button size="small" type="text" icon={<CloseOutlined />} loading={unsetting}
                onClick={doUnset} title="Unset" style={{ color: "var(--text-faint)" }} />
            )}
          </div>
        )}
      </td>
    </tr>
  );
}

// ─── Predecessor list ─────────────────────────────────────────────────────────

interface PredecessorsProps {
  predecessors: string[];
  onAdd:     (name: string) => Promise<void>;
  onRemove:  (name: string) => Promise<void>;
  disabled?: boolean;
  search?:   string;
}

function PredecessorsList({ predecessors, onAdd, onRemove, disabled = false, search }: PredecessorsProps) {
  const [addVal, setAddVal] = useState("");
  const [adding, setAdding] = useState(false);

  if (search && !"predecessors after".includes(search.toLowerCase())) return null;

  const doAdd = async () => {
    const v = addVal.trim();
    if (!v) return;
    setAdding(true);
    try { await onAdd(v); setAddVal(""); }
    catch (e) { message.error(String(e)); }
    finally { setAdding(false); }
  };

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={{ ...LABEL_TD, opacity: disabled ? 0.45 : 1 }}>Predecessors (AFTER)</td>
      <td style={{ padding: "6px 0", verticalAlign: "top" }}>
        {disabled ? (
          <span style={{ fontSize: 12, color: "var(--text-faint)", fontStyle: "italic" }}>
            Not available — tasks with a finalizer cannot have predecessors
          </span>
        ) : (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            {predecessors.length === 0 && (
              <span style={{ fontSize: 12, color: "var(--text-faint)", fontStyle: "italic" }}>—</span>
            )}
            {predecessors.map((p) => (
              <div key={p} style={{ display: "flex", alignItems: "center", gap: 6 }}>
                <span style={{ fontFamily: "monospace", fontSize: 12, flex: 1 }}>{p}</span>
                <Button size="small" type="text" danger icon={<DeleteOutlined />}
                  title={`REMOVE AFTER ${p}`}
                  onClick={async () => {
                    try { await onRemove(p); }
                    catch (e) { message.error(String(e)); }
                  }} />
              </div>
            ))}
            <div style={{ display: "flex", gap: 6, marginTop: 4 }}>
              <Input size="small" value={addVal}
                placeholder="task name to add…"
                onChange={(e) => setAddVal(e.target.value)}
                onPressEnter={doAdd}
                style={{ fontFamily: "monospace", fontSize: 12, flex: 1 }} />
              <Button size="small" icon={<PlusOutlined />} loading={adding} onClick={doAdd}
                type="primary" ghost>ADD AFTER</Button>
            </div>
          </div>
        )}
      </td>
    </tr>
  );
}

// ─── MultilineRow (SQL body / WHEN condition) ─────────────────────────────────

interface MultilineRowProps {
  label:     string;
  value:     string;
  onSave:    (val: string) => Promise<void>;
  onUnset?:  () => Promise<void>;
  unsetLabel?: string;
}

function MultilineRow({ label, value, onSave, onUnset, unsetLabel = "Remove" }: MultilineRowProps) {
  const [editing,   setEditing]   = useState(false);
  const [editVal,   setEditVal]   = useState(value);
  const [saving,    setSaving]    = useState(false);
  const [unsetting, setUnsetting] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const startEdit = () => { setEditing(true); setEditVal(value); setEditError(null); };
  const cancel    = () => { setEditing(false); setEditError(null); };

  const save = async () => {
    setSaving(true); setEditError(null);
    try { await onSave(editVal); setEditing(false); }
    catch (e) {
      const raw = String(e);
      const m = raw.match(/:\s*(.+)$/s);
      setEditError(m ? m[0].trim() : raw);
    } finally { setSaving(false); }
  };

  const doUnset = async () => {
    if (!onUnset) return;
    setUnsetting(true);
    try { await onUnset(); setEditing(false); }
    catch (e) { message.error(String(e)); }
    finally { setUnsetting(false); }
  };

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={{ ...LABEL_TD, verticalAlign: "top", paddingTop: 10 }}>{label}</td>
      <td style={{ padding: "6px 0" }}>
        {editing ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
            <TextArea value={editVal} onChange={(e) => setEditVal(e.target.value)}
              autoSize={{ minRows: 4, maxRows: 16 }}
              style={{ fontFamily: "monospace", fontSize: 12 }}
              status={editError ? "error" : undefined} autoFocus />
            <div style={{ display: "flex", gap: 6 }}>
              <Button size="small" type="primary" icon={<CheckOutlined />}
                loading={saving} onClick={save}>Save</Button>
              <Button size="small" icon={<CloseOutlined />} disabled={saving || unsetting}
                onClick={cancel}>Cancel</Button>
              {onUnset && (
                <Button size="small" danger loading={unsetting}
                  disabled={saving} onClick={doUnset}>{unsetLabel}</Button>
              )}
            </div>
            {editError && (
              <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace" }}>
                {editError}
              </div>
            )}
          </div>
        ) : (
          <div style={{ display: "flex", alignItems: "flex-start", gap: 8 }}>
            <pre style={{
              margin: 0, fontFamily: "monospace", fontSize: 12,
              color: value ? "var(--text)" : "var(--text-faint)",
              fontStyle: value ? "normal" : "italic",
              whiteSpace: "pre-wrap", wordBreak: "break-word", flex: 1,
              maxHeight: 200, overflow: "auto",
            }}>
              {value || "—"}
            </pre>
            <Button size="small" type="text" icon={<EditOutlined />} onClick={startEdit}
              style={{ flexShrink: 0, color: "var(--text-faint)" }} />
          </div>
        )}
      </td>
    </tr>
  );
}

// ─── FinalizeTaskRow ──────────────────────────────────────────────────────────

interface FinalizeRowProps {
  value:                  string;
  options:                { label: React.ReactNode; value: string; disabled?: boolean }[];
  currentTaskHasChildren: boolean;
  search?:                string;
  onSave:                 (val: string) => Promise<void>;
  onUnset:                () => Promise<void>;
}

function FinalizeTaskRow({ value, options, currentTaskHasChildren, search, onSave, onUnset }: FinalizeRowProps) {
  const [editing,   setEditing]   = useState(false);
  const [editVal,   setEditVal]   = useState(value);
  const [saving,    setSaving]    = useState(false);
  const [unsetting, setUnsetting] = useState(false);
  const [editError, setEditError] = useState<string | null>(null);

  const startEdit = () => { setEditing(true); setEditVal(value); setEditError(null); };
  const cancel    = () => { setEditing(false); setEditError(null); };

  const save = async () => {
    setSaving(true); setEditError(null);
    try { await onSave(editVal); setEditing(false); }
    catch (e) {
      const raw = String(e);
      const m = raw.match(/:\s*(.+)$/s);
      setEditError(m ? m[0].trim() : raw);
    } finally { setSaving(false); }
  };

  const doUnset = async () => {
    setUnsetting(true);
    try { await onUnset(); setEditing(false); }
    catch (e) { message.error(String(e)); }
    finally { setUnsetting(false); }
  };

  const showSection = (labels: string[]) => {
    if (!search) return true;
    return labels.some(l => l.toLowerCase().includes(search.toLowerCase()));
  };

  if (!showSection(["Finalizes root task"])) return null;

  return (
    <tr style={{ borderBottom: "1px solid var(--border)" }}>
      <td style={{ ...LABEL_TD, whiteSpace: "normal", verticalAlign: "top", paddingTop: 8 }}>
        <span>Finalizes root task</span>
        <div style={{ fontSize: 11, color: "var(--text-faint)", fontStyle: "italic", marginTop: 2, lineHeight: 1.3 }}>
          This task runs after the named root task&apos;s graph completes
        </div>
      </td>
      <td style={{ padding: "4px 0", verticalAlign: "middle" }}>
        {currentTaskHasChildren ? (
          /* Tasks with successors cannot themselves be finalizers */
          <span style={{ fontSize: 12, color: "var(--text-faint)", fontStyle: "italic" }}>
            Not available — tasks with successors cannot be finalizers
          </span>
        ) : editing ? (
          <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              <AutoComplete
                size="small"
                value={editVal}
                onChange={setEditVal}
                options={options}
                filterOption={(input, opt) =>
                  (opt?.value ?? "").toLowerCase().includes(input.toLowerCase())
                }
                style={{ flex: 1, fontFamily: "monospace", fontSize: 12 }}
                placeholder={options.length > 0 ? "Select or type a task name…" : "Task name…"}
                autoFocus
              />
              <Button size="small" type="primary" icon={<CheckOutlined />} loading={saving} onClick={save} />
              <Button size="small" icon={<CloseOutlined />} disabled={saving} onClick={cancel} />
            </div>
            {editError && (
              <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace", paddingLeft: 2 }}>
                {editError}
              </div>
            )}
            <div style={{ fontSize: 11, color: "var(--text-faint)", paddingLeft: 2 }}>
              {options.length > 0
                ? "Greyed-out tasks are ineligible (hover for reason) — type to filter"
                : "No tasks found in this schema — type a task name manually"}
            </div>
          </div>
        ) : (
          <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
            <span style={{
              fontFamily: "monospace", fontSize: 12, flex: 1, wordBreak: "break-word",
              color: value ? "var(--text)" : "var(--text-faint)",
              fontStyle: value ? "normal" : "italic",
            }}>
              {value || "—"}
            </span>
            <Button size="small" type="text" icon={<EditOutlined />} onClick={startEdit}
              style={{ flexShrink: 0, color: "var(--text-faint)" }} />
            {value && (
              <Button size="small" type="text" icon={<CloseOutlined />} loading={unsetting}
                onClick={doUnset} title="Unset" style={{ color: "var(--text-faint)" }} />
            )}
          </div>
        )}
      </td>
    </tr>
  );
}

// ─── Main modal ───────────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  isFinalizer?: boolean;
  onClose: () => void;
}

export default function TaskPropertiesModal({ db, schema, name, isFinalizer = false, onClose }: Props) {
  const [rows,         setRows]         = useState<snowflake.PropertyPair[] | null>(null);
  const [loadError,    setLoadError]    = useState<string | null>(null);
  const [toggling,     setToggling]     = useState(false);
  const [togglingGraph, setTogglingGraph] = useState(false);
  const [integrations, setIntegrations] = useState<{ label: string; value: string }[]>([]);
  const [showHistory,  setShowHistory]  = useState(false);
  const [whenEditing,  setWhenEditing]  = useState(false);
  const [whenDraft,    setWhenDraft]    = useState("");
  const [whenSaving,   setWhenSaving]   = useState(false);
  const [whenError,    setWhenError]    = useState<string | null>(null);
  const [schedEditing,   setSchedEditing]   = useState(false);
  const [schedDraft,     setSchedDraft]     = useState("");
  const [schedEditKey,   setSchedEditKey]   = useState(0);
  const [schedSaving,    setSchedSaving]    = useState(false);
  const [schedError,     setSchedError]     = useState<string | null>(null);
  const [rootTasks,           setRootTasks]           = useState<{ label: React.ReactNode; value: string; disabled?: boolean }[]>([]);
  const [hasChildren,         setHasChildren]         = useState(false);
  const [rootHasFinalizer,    setRootHasFinalizer]    = useState(false);
  const [showCreateFinalizer, setShowCreateFinalizer] = useState(false);
  const [finalizeTarget,      setFinalizeTarget]      = useState("");
  const [allTaskRows,         setAllTaskRows]         = useState<tasks.StatusRow[]>([]);
  const [setFinalizerFor,     setSetFinalizerFor]     = useState("");
  const [settingFinalizer,    setSettingFinalizer]    = useState(false);
  const [setFinalizerError,   setSetFinalizerError]   = useState<string | null>(null);
  const [search,              setSearch]              = useState("");

  const loadTaskStatuses = useCallback(() => {
    GetTaskStatuses(db, schema)
      .then((result) => {
        const nameUpper = name.toUpperCase();
        const rows = result.rows ?? [];
        setAllTaskRows(rows);
        setRootHasFinalizer(
          rows.some((t) => t.finalize && extractName(t.finalize).toUpperCase() === nameUpper),
        );
        const own = rows.find((t) => t.name.toUpperCase() === nameUpper);
        if (own?.finalize) setFinalizeTarget(own.finalize);
        else setFinalizeTarget("");
      })
      .catch(() => {});
  }, [db, schema, name]);

  useEffect(() => {
    ListNotificationIntegrations()
      .then((list) => setIntegrations((list ?? []).map((n) => ({ label: n, value: n }))))
      .catch(() => {});
    ListFinalizableTasks(db, schema)
      .then((list) => setRootTasks((list ?? []).map((row) => ({
        value: row.name,
        disabled: !!row.disabledReason,
        label: row.disabledReason ? (
          <span>
            {row.name}
            <span style={{ color: "var(--text-faint)", fontSize: 11, marginLeft: 6 }}>
              — {row.disabledReason}
            </span>
          </span>
        ) : row.name,
      }))))
      .catch(() => {});
    TaskHasChildren(db, schema, name)
      .then(setHasChildren)
      .catch(() => {});
    loadTaskStatuses();
  }, [db, schema, name, loadTaskStatuses]);

  // ── Data loading ────────────────────────────────────────────────────────────

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const r = await GetObjectProperties(db, schema, "TASK", name);
      setRows(r ?? []);
    } catch (e) {
      setLoadError(String(e));
      setRows([]);
    }
  }, [db, schema, name]);

  useEffect(() => { load(); }, [load]);

  const objTags = useObjectTags({
    kind: "TASK", db, schema, name,
    alter: (clause) => AlterTask(db, schema, name, clause),
  });

  // ── Property lookup (case-insensitive) ──────────────────────────────────────

  const get = (key: string): string => {
    const k = key.toLowerCase();
    return rows?.find((r) => r.key.toLowerCase() === k)?.value ?? "";
  };

  // ── ALTER helpers ───────────────────────────────────────────────────────────

  const alter = async (clause: string) => {
    await AlterTask(db, schema, name, clause);
    await load();
  };

  /** SET a text property; UNSET if value is empty and canUnset is set. */
  const setText = (prop: string, canUnset = true) => async (val: string) => {
    const v = val.trim();
    if (!v && canUnset) await alter(`UNSET ${prop}`);
    else await alter(`SET ${prop} = ${q1(v)}`);
  };

  /** SET a numeric property. */
  const setNum = (prop: string) => async (val: string) => {
    await alter(`SET ${prop} = ${parseInt(val, 10) || 0}`);
  };

  /** SET a bare-word value (enum, no quotes). */
  const setEnum = (prop: string) => async (val: string) => {
    await alter(`SET ${prop} = ${val}`);
  };

  // ── RESUME / SUSPEND ────────────────────────────────────────────────────────

  const state = get("state").toUpperCase();
  const isStarted = state === "STARTED" || state === "RUNNING";

  const toggleState = async () => {
    setToggling(true);
    try {
      await alter(isStarted ? "SUSPEND" : "RESUME");
      message.success(`Task ${isStarted ? "suspended" : "resumed"}`);
    } catch (e) { message.error(String(e)); }
    finally { setToggling(false); }
  };

  const toggleGraph = async () => {
    setTogglingGraph(true);
    try {
      const result = await GetTaskStatuses(db, schema);
      const rows = result.rows ?? [];

      // Build parent/children maps using the proven frontend parsePredecessors.
      const byName = new Map<string, string>(); // UPPER → original name
      const childrenOf = new Map<string, string[]>();
      const parentOf   = new Map<string, string>();
      rows.forEach((t) => {
        byName.set(t.name.toUpperCase(), t.name);
        for (const p of parsePredecessorsUtil(t.predecessors ?? "")) {
          const pu = extractName(p).toUpperCase();
          if (!childrenOf.has(pu)) childrenOf.set(pu, []);
          childrenOf.get(pu)!.push(t.name);
          parentOf.set(t.name.toUpperCase(), pu);
        }
      });

      // Walk up from `name` to find the true root of the graph.
      let rootUpper = name.toUpperCase();
      while (parentOf.has(rootUpper)) rootUpper = parentOf.get(rootUpper)!;
      const rootName = byName.get(rootUpper) ?? name;

      // BFS from root → [root, …, leaves].
      const bfsOrder: string[] = [];
      const visited = new Set<string>();
      const queue = [rootName];
      visited.add(rootUpper);
      while (queue.length > 0) {
        const cur = queue.shift()!;
        bfsOrder.push(cur);
        for (const child of childrenOf.get(cur.toUpperCase()) ?? []) {
          if (!visited.has(child.toUpperCase())) {
            visited.add(child.toUpperCase());
            queue.push(child);
          }
        }
      }

      // Collect finalizer tasks for this root.
      const finalizerNames = rows
        .filter((t) => t.finalize && extractName(t.finalize).toUpperCase() === rootUpper)
        .map((t) => t.name)
        .filter((fn) => !visited.has(fn.toUpperCase()));

      if (isStarted) {
        // Suspend: root first, then descendants, finalizers last.
        await SuspendTaskList(db, schema, [...bfsOrder, ...finalizerNames]);
        message.success("Task graph suspended");
      } else {
        // Resume: leaves first (reverse BFS excl. root), finalizers, root last.
        const reversedBfs = [...bfsOrder].reverse();
        const resumeOrder = [
          ...reversedBfs.slice(0, -1),
          ...finalizerNames,
          reversedBfs[reversedBfs.length - 1],
        ];
        await ResumeTaskList(db, schema, resumeOrder);
        message.success("Task graph resumed");
      }
      await load();
    } catch (e) { message.error(String(e)); }
    finally { setTogglingGraph(false); }
  };

  // ── Predecessors ─────────────────────────────────────────────────────────────

  const predecessors = parsePredecessors(get("predecessors"));

  const addAfter = async (taskName: string) => {
    await alter(`ADD AFTER ${q1(taskName)}`);
  };

  const removeAfter = async (taskName: string) => {
    await alter(`REMOVE AFTER ${q1(taskName)}`);
  };

  // ── Trigger presence check ──────────────────────────────────────────────────
  // A task with none of SCHEDULE / AFTER / FINALIZE / WHEN has no trigger and
  // will never run automatically. Finalizer tasks are an exception: their
  // trigger is the FINALIZE clause itself. The `isFinalizer` prop is set by
  // the sidebar tree (which detects it reliably via GET_DDL) so we use that
  // as the authoritative signal rather than trying to parse SHOW TASKS columns
  // that may not expose the FINALIZE property in all Snowflake editions.
  const taskRelations = get("task_relations");
  // Parse task_relations once; extract the finalizer root-task name if present.
  // The string-search fallback (.includes("finalize")) is only used when JSON
  // parsing itself fails — NOT when parsing succeeds but has no finalize field,
  // because a root task's task_relations JSON may contain a null "finalize" key
  // that would otherwise trigger a false positive.
  const { finalizeFromTaskRelations, finalizeInTaskRelations } = (() => {
    if (!taskRelations || taskRelations === "null")
      return { finalizeFromTaskRelations: "", finalizeInTaskRelations: false };
    try {
      const r = JSON.parse(taskRelations);
      const val = String(r?.finalize || r?.finalize_task || "");
      return { finalizeFromTaskRelations: val, finalizeInTaskRelations: !!val };
    } catch {
      return {
        finalizeFromTaskRelations: "",
        finalizeInTaskRelations: taskRelations.toLowerCase().includes("finalize"),
      };
    }
  })();

  // Resolved finalize value: GetTaskStatuses is the most reliable source (same data
  // used by the task graph); fall back to direct columns and task_relations JSON.
  const finalizeValue = finalizeTarget || get("finalize") || get("finalize_task") || finalizeFromTaskRelations;
  // True when this task IS a finalizer: the isFinalizer prop (sidebar-detected via
  // GET_DDL) is the primary signal; the actual finalize/task_relations properties
  // are fallbacks for cases where the prop was not set.
  const isThisTaskAFinalizer =
    isFinalizer ||
    !!finalizeValue ||
    finalizeInTaskRelations;

  // ── "Set as Finalizer For" eligibility ──────────────────────────────────────
  // Evaluated only when the task is NOT already a finalizer.
  const thisTaskDisabledReason = isThisTaskAFinalizer ? "" : (() => {
    if (predecessors.length > 0)  return "Already a child task (has predecessors)";
    if (get("schedule"))          return "Has its own schedule";
    if (hasChildren)              return "Has child tasks";
    return "";
  })();

  // Root tasks that are eligible to receive this task as their finalizer:
  // no predecessors, not itself a finalizer, and no finalizer assigned yet.
  const finalizersForRoots = new Set(
    allTaskRows.filter((t) => t.finalize).map((t) => extractName(t.finalize).toUpperCase()),
  );
  const eligibleRootTasks = isThisTaskAFinalizer ? [] : allTaskRows.filter((t) => {
    const isBlank = (s: string) => !s || s === "[]" || s === "<nil>";
    return isBlank(t.predecessors)
      && !t.finalize
      && !finalizersForRoots.has(t.name.toUpperCase())
      && t.name.toUpperCase() !== name.toUpperCase();
  });

  const hasTrigger =
    isThisTaskAFinalizer ||
    !!get("schedule") ||
    predecessors.length > 0 ||
    !!(get("condition") || get("condition_text"));

  // ── Overlap policy ────────────────────────────────────────────────────────

  // Snowflake may expose either `overlap_policy` (newer) or
  // `allow_overlapping_execution` (older boolean).  Normalise to enum value.
  const rawOverlap = get("overlap_policy") || get("allow_overlapping_execution");
  const overlapValue = rawOverlap === "true" || rawOverlap === "TRUE"
    ? "ALLOW_ALL_OVERLAP"
    : rawOverlap === "false" || rawOverlap === "FALSE"
    ? "NO_OVERLAP"
    : rawOverlap || "";

  // ─────────────────────────────────────────────────────────────────────────────

  return (
    <>
    <Modal
      open
      title={
        <Space size={6}>
          <ClockCircleOutlined style={{ color: "var(--link)" }} />
          <span>Task Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={640}
      styles={{ body: { maxHeight: "76vh", overflowY: "auto", paddingTop: 12 } }}
    >
      {loadError && (
        <Alert type="error" message={loadError} style={{ marginBottom: 12 }} showIcon />
      )}

      {rows === null && !loadError && (
        <div style={{ textAlign: "center", padding: 32 }}>
          <Spin tip="Loading…" />
        </div>
      )}

      {rows !== null && (
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

          {/* ── Owner line ───────────────────────────────────────────────── */}
          {get("owner") && (
            <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 6 }}>
              Owner: {get("owner")}
            </Text>
          )}

          {/* ── Status bar ───────────────────────────────────────────────── */}
          <div style={{
            display: "flex", alignItems: "center", gap: 12,
            padding: "10px 12px", borderRadius: 6,
            border: "1px solid var(--border)", background: "var(--bg)",
            marginBottom: 8,
          }}>
            <Tag color={isStarted ? "green" : "default"} style={{ margin: 0 }}>
              {state || "UNKNOWN"}
            </Tag>
            <Button
              size="small"
              icon={isStarted ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
              onClick={toggleState}
              loading={toggling}
              disabled={togglingGraph || (!isStarted && !hasTrigger)}
              title={!isStarted && !hasTrigger ? "Cannot resume: task has no schedule, predecessor, finalize, or WHEN condition" : undefined}
            >
              {isStarted ? "Suspend" : "Resume"}
            </Button>
            <Button
              size="small"
              icon={isStarted ? <PauseCircleOutlined /> : <PlayCircleOutlined />}
              onClick={toggleGraph}
              loading={togglingGraph}
              disabled={toggling || (!isStarted && !hasTrigger)}
              title={!isStarted && !hasTrigger
                ? "Cannot resume: task has no schedule, predecessor, finalize, or WHEN condition"
                : isStarted
                ? "Suspend this task and all child tasks"
                : "Resume all child tasks (leaf-first), then this task"}
            >
              {isStarted ? "Suspend Graph" : "Resume Graph"}
            </Button>
            {!isThisTaskAFinalizer && predecessors.length === 0 && (
              <Tooltip title={rootHasFinalizer ? "This task graph already has a finalizer" : undefined}>
                <Button
                  size="small"
                  icon={<FlagOutlined />}
                  disabled={rootHasFinalizer}
                  onClick={() => setShowCreateFinalizer(true)}
                >
                  Create Finalizer Task…
                </Button>
              </Tooltip>
            )}
            <Button
              size="small"
              icon={<HistoryOutlined />}
              onClick={() => setShowHistory(true)}
            >
              History
            </Button>
          </div>

          {/* ── Trigger warning ──────────────────────────────────────────── */}
          {!hasTrigger && (
            <Alert
              type="warning"
              showIcon
              message="No trigger configured"
              description="This task has no SCHEDULE, AFTER predecessor, FINALIZE, or WHEN condition set — it will never run automatically."
              style={{ marginBottom: 8 }}
            />
          )}

          {/* ── Read-only info ────────────────────────────────────────────── */}
          <div style={{ display: "flex", gap: 24, marginBottom: 4 }}>
            {get("created_on") && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                Created: {get("created_on")}
              </Text>
            )}
            {get("last_committed_on") && (
              <Text type="secondary" style={{ fontSize: 11 }}>
                Last commit: {get("last_committed_on")}
              </Text>
            )}
          </div>

          {/* ── Compute ──────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Compute</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search} label="Warehouse" value={get("warehouse")} type="text"
                hint="Leave blank to use serverless compute" canUnset
                onSave={async (v) => {
                  const t = v.trim();
                  if (!t) await alter("UNSET WAREHOUSE");
                  else await alter(`SET WAREHOUSE = ${t}`); // bare identifier
                  await load();
                }}
                onUnset={async () => { await alter("UNSET WAREHOUSE"); await load(); }}
              />
              <EditRow search={search} label="Initial WH Size (serverless)" type="select"
                value={get("user_task_managed_initial_warehouse_size")}
                options={SIZE_OPTIONS} canUnset
                onSave={setEnum("USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE")}
                onUnset={async () => { await alter("UNSET USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE"); await load(); }}
              />
              <EditRow search={search} label="Min Statement Size" type="select"
                value={get("serverless_task_min_statement_size") || get("serverless_task_min_warehouse_size")}
                options={SIZE_OPTIONS} canUnset
                onSave={setEnum("SERVERLESS_TASK_MIN_STATEMENT_SIZE")}
                onUnset={async () => { await alter("UNSET SERVERLESS_TASK_MIN_STATEMENT_SIZE"); await load(); }}
              />
              <EditRow search={search} label="Max Statement Size" type="select"
                value={get("serverless_task_max_statement_size") || get("serverless_task_max_warehouse_size")}
                options={SIZE_OPTIONS} canUnset
                onSave={setEnum("SERVERLESS_TASK_MAX_STATEMENT_SIZE")}
                onUnset={async () => { await alter("UNSET SERVERLESS_TASK_MAX_STATEMENT_SIZE"); await load(); }}
              />
              <EditRow search={search} label="Min Trigger Interval (s)" type="number" min={0}
                value={get("user_task_minimum_trigger_interval_in_seconds")}
                hint="USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS"
                onSave={setNum("USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS")} />
            </tbody>
          </table>

          {/* ── Schedule ─────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Schedule</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {/* Schedule — inline editor using ScheduleEditor */}
              <tr style={{ borderBottom: schedEditing ? "none" : "1px solid var(--border)" }}>
                <td style={{ ...LABEL_TD, verticalAlign: schedEditing ? "top" : "middle", paddingTop: schedEditing ? 10 : undefined }}>
                  Schedule
                </td>
                <td style={{ padding: "6px 0" }}>
                  {predecessors.length > 0 ? (
                    <span style={{ fontSize: 12, color: "var(--text-faint)", fontStyle: "italic" }}>
                      Not available — child tasks cannot have a schedule
                    </span>
                  ) : !schedEditing ? (
                    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                      <span style={{
                        fontFamily: "monospace", fontSize: 12, flex: 1, wordBreak: "break-word",
                        color: get("schedule") ? "var(--text)" : "var(--text-faint)",
                        fontStyle: get("schedule") ? "normal" : "italic",
                      }}>
                        {get("schedule") || "—"}
                      </span>
                      <Button size="small" type="text" icon={<EditOutlined />}
                        style={{ flexShrink: 0, color: "var(--text-faint)" }}
                        onClick={() => {
                          setSchedDraft(get("schedule"));
                          setSchedEditKey((k) => k + 1);
                          setSchedError(null);
                          setSchedEditing(true);
                        }} />
                    </div>
                  ) : (
                    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                      <ScheduleEditor
                        key={schedEditKey}
                        value={schedDraft}
                        onChange={setSchedDraft}
                      />
                      {schedError && (
                        <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace" }}>{schedError}</div>
                      )}
                      <div style={{ display: "flex", gap: 6 }}>
                        <Button size="small" type="primary" loading={schedSaving}
                          onClick={async () => {
                            setSchedSaving(true); setSchedError(null);
                            try {
                              const t = schedDraft.trim();
                              if (!t) await alter("UNSET SCHEDULE");
                              else    await alter(`SET SCHEDULE = '${t}'`);
                              setSchedEditing(false);
                            } catch (e) {
                              const raw = String(e);
                              const m = raw.match(/:\s*(.+)$/s);
                              setSchedError(m ? m[0].trim() : raw);
                            } finally { setSchedSaving(false); }
                          }}>Save</Button>
                        <Button size="small" disabled={schedSaving}
                          onClick={() => setSchedEditing(false)}>Cancel</Button>
                        {get("schedule") && (
                          <Button size="small" danger disabled={schedSaving}
                            onClick={async () => {
                              setSchedSaving(true);
                              try { await alter("UNSET SCHEDULE"); setSchedEditing(false); }
                              catch (e) { setSchedError(String(e)); }
                              finally { setSchedSaving(false); }
                            }}>Remove</Button>
                        )}
                      </div>
                    </div>
                  )}
                </td>
              </tr>
              <EditRow search={search} label="Overlap Policy" value={overlapValue}
                type="select" options={OVERLAP_OPTIONS}
                onSave={setEnum("OVERLAP_POLICY")} />
              <EditRow search={search} label="Target Completion Interval" type="text"
                value={get("target_completion_interval")}
                hint="e.g. '1 HOURS'" canUnset
                onSave={setText("TARGET_COMPLETION_INTERVAL")}
                onUnset={async () => { await alter("UNSET TARGET_COMPLETION_INTERVAL"); await load(); }}
              />
            </tbody>
          </table>

          {/* ── Dependencies ─────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Dependencies</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <PredecessorsList search={search}                 predecessors={predecessors}
                onAdd={addAfter}
                onRemove={removeAfter}
                disabled={rootHasFinalizer}
              />
              {isThisTaskAFinalizer && (
                <FinalizeTaskRow search={search}                   value={finalizeValue}
                  options={rootTasks}
                  currentTaskHasChildren={hasChildren}
                  onSave={async (v) => {
                    const t = v.trim();
                    if (!t) await alter("UNSET FINALIZE");
                    else {
                      // FINALIZE requires a fully-qualified task identifier.
                      // If the user typed or selected a bare name, qualify it.
                      const esc = (s: string) => s.replace(/"/g, '""');
                      const qualified = (t.startsWith('"') || t.includes('.'))
                        ? t
                        : `"${esc(db)}"."${esc(schema)}"."${esc(t)}"`;
                      await alter(`SET FINALIZE = ${qualified}`);
                    }
                    await load();
                    loadTaskStatuses();
                  }}
                  onUnset={async () => {
                    await alter("UNSET FINALIZE");
                    await load();
                    loadTaskStatuses();
                  }}
                />
              )}

              {/* ── Set as Finalizer For — shown for non-finalizer tasks ── */}
              {!isThisTaskAFinalizer && (
                <tr style={{ borderBottom: "1px solid var(--border)" }}>
                  <td style={{ ...LABEL_TD, whiteSpace: "normal", verticalAlign: "top", paddingTop: 8 }}>
                    <span>Set as Finalizer For</span>
                    <div style={{ fontSize: 11, color: "var(--text-faint)", fontStyle: "italic", marginTop: 2, lineHeight: 1.3 }}>
                      Run this task after a root task&apos;s graph completes
                    </div>
                  </td>
                  <td style={{ padding: "6px 0", verticalAlign: "middle" }}>
                    {thisTaskDisabledReason ? (
                      <span style={{ fontSize: 12, color: "var(--text-faint)", fontStyle: "italic" }}>
                        Not eligible — {thisTaskDisabledReason}
                      </span>
                    ) : eligibleRootTasks.length === 0 ? (
                      <span style={{ fontSize: 12, color: "var(--text-faint)", fontStyle: "italic" }}>
                        No root tasks available — all root tasks already have a finalizer
                      </span>
                    ) : (
                      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
                        <div style={{ display: "flex", gap: 6 }}>
                          <Select
                            size="small"
                            value={setFinalizerFor || undefined}
                            onChange={setSetFinalizerFor}
                            options={eligibleRootTasks.map((t) => ({ label: t.name, value: t.name }))}
                            placeholder="Select a root task…"
                            style={{ flex: 1, fontFamily: "monospace", fontSize: 12 }}
                            showSearch
                            filterOption={(input, opt) =>
                              (opt?.value ?? "").toLowerCase().includes(input.toLowerCase())
                            }
                          />
                          <Button
                            size="small"
                            type="primary"
                            icon={<CheckOutlined />}
                            loading={settingFinalizer}
                            disabled={!setFinalizerFor}
                            onClick={async () => {
                              if (!setFinalizerFor) return;
                              setSettingFinalizer(true);
                              setSetFinalizerError(null);
                              try {
                                const esc = (s: string) => s.replace(/"/g, '""');
                                await alter(`SET FINALIZE = "${esc(db)}"."${esc(schema)}"."${esc(setFinalizerFor)}"`);
                                message.success(`${name} set as finalizer for ${setFinalizerFor}`);
                                setSetFinalizerFor("");
                                loadTaskStatuses();
                              } catch (e) {
                                const raw = String(e);
                                const m = raw.match(/:\s*(.+)$/s);
                                setSetFinalizerError(m ? m[0].trim() : raw);
                              } finally {
                                setSettingFinalizer(false);
                              }
                            }}
                          >
                            Set
                          </Button>
                        </div>
                        {setFinalizerError && (
                          <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace", paddingLeft: 2 }}>
                            {setFinalizerError}
                          </div>
                        )}
                      </div>
                    )}
                  </td>
                </tr>
              )}
            </tbody>
          </table>

          {/* ── Condition (WHEN) ──────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Condition</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr style={{ borderBottom: whenEditing ? "none" : "1px solid var(--border)" }}>
                <td style={{ ...LABEL_TD, verticalAlign: whenEditing ? "top" : "middle", paddingTop: whenEditing ? 10 : undefined }}>
                  WHEN condition
                </td>
                <td style={{ padding: "6px 0" }}>
                  {!whenEditing ? (
                    <div style={{ display: "flex", alignItems: "flex-start", gap: 8 }}>
                      <pre style={{
                        margin: 0, fontFamily: "monospace", fontSize: 12, flex: 1,
                        color: (get("condition") || get("condition_text")) ? "var(--text)" : "var(--text-faint)",
                        fontStyle: (get("condition") || get("condition_text")) ? "normal" : "italic",
                        whiteSpace: "pre-wrap", wordBreak: "break-word",
                        maxHeight: 100, overflow: "auto",
                      }}>
                        {get("condition") || get("condition_text") || "—"}
                      </pre>
                      <Button size="small" type="text" icon={<EditOutlined />}
                        style={{ flexShrink: 0, color: "var(--text-faint)" }}
                        onClick={() => {
                          setWhenDraft(get("condition") || get("condition_text"));
                          setWhenError(null);
                          setWhenEditing(true);
                        }} />
                    </div>
                  ) : (
                    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
                      <WhenConditionBuilder
                        db={db} schema={schema}
                        value={whenDraft}
                        onChange={setWhenDraft}
                      />
                      {whenError && (
                        <div style={{ color: "#f85149", fontSize: 11, fontFamily: "monospace" }}>{whenError}</div>
                      )}
                      <div style={{ display: "flex", gap: 6 }}>
                        <Button size="small" type="primary" loading={whenSaving}
                          onClick={async () => {
                            setWhenSaving(true); setWhenError(null);
                            try {
                              const t = whenDraft.trim();
                              if (!t) await alter("REMOVE WHEN");
                              else await alter(`MODIFY WHEN ${t}`);
                              setWhenEditing(false);
                            } catch (e) {
                              const raw = String(e);
                              const m = raw.match(/:\s*(.+)$/s);
                              setWhenError(m ? m[0].trim() : raw);
                            } finally { setWhenSaving(false); }
                          }}>Save</Button>
                        <Button size="small" disabled={whenSaving}
                          onClick={() => setWhenEditing(false)}>Cancel</Button>
                        <Button size="small" danger disabled={whenSaving}
                          onClick={async () => {
                            setWhenSaving(true);
                            try { await alter("REMOVE WHEN"); setWhenEditing(false); }
                            catch (e) { setWhenError(String(e)); }
                            finally { setWhenSaving(false); }
                          }}>Remove WHEN</Button>
                      </div>
                    </div>
                  )}
                </td>
              </tr>
            </tbody>
          </table>

          {/* ── SQL Body ─────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>SQL Body</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <MultilineRow
                label="Definition (AS)"
                value={get("definition")}
                onSave={async (v) => {
                  await alter(`MODIFY AS ${v}`);
                  await load();
                }}
              />
            </tbody>
          </table>

          {/* ── Configuration ─────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Configuration</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search} label="Config (JSON)" value={get("config")} type="textarea" canUnset
                hint="Valid JSON — merged with the task's default config at runtime"
                onSave={async (v) => {
                  const t = v.trim();
                  if (!t) { await alter("UNSET CONFIG"); await load(); return; }
                  if (!isValidJson(t)) throw new Error("Invalid JSON");
                  await alter(`SET CONFIG = $$${t}$$`);
                  await load();
                }}
                onUnset={async () => { await alter("UNSET CONFIG"); await load(); }}
              />
              <EditRow search={search} label="Execute as User" type="text"
                value={get("execute_as") || get("execute_as_user")} canUnset
                hint="User name to execute the task as"
                onSave={async (v) => {
                  const t = v.trim();
                  if (!t) await alter("UNSET EXECUTE AS USER");
                  else await alter(`SET EXECUTE AS USER ${t}`); // identifier, no quotes
                  await load();
                }}
                onUnset={async () => { await alter("UNSET EXECUTE AS USER"); await load(); }}
              />
            </tbody>
          </table>

          {/* ── Limits ───────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Limits</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search} label="Timeout (ms)" type="number" min={0}
                value={get("user_task_timeout_ms")}
                hint="USER_TASK_TIMEOUT_MS — 0 means no timeout"
                onSave={setNum("USER_TASK_TIMEOUT_MS")} />
              <EditRow search={search} label="Suspend After N Failures" type="number" min={0}
                value={get("suspend_task_after_num_failures")}
                hint="0 = never suspend"
                onSave={setNum("SUSPEND_TASK_AFTER_NUM_FAILURES")} />
              <EditRow search={search} label="Auto-Retry Attempts" type="number" min={0}
                value={get("task_auto_retry_attempts")}
                onSave={setNum("TASK_AUTO_RETRY_ATTEMPTS")} />
            </tbody>
          </table>

          {/* ── Notifications ─────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>Notifications</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search} label="Error Integration" type="select" canUnset
                value={get("error_integration")}
                options={integrations}
                onSave={setText("ERROR_INTEGRATION")}
                onUnset={async () => { await alter("UNSET ERROR_INTEGRATION"); await load(); }}
              />
              <EditRow search={search} label="Success Integration" type="select" canUnset
                value={get("success_integration")}
                options={integrations}
                onSave={setText("SUCCESS_INTEGRATION")}
                onUnset={async () => { await alter("UNSET SUCCESS_INTEGRATION"); await load(); }}
              />
              <EditRow search={search} label="Log Level" type="select" value={get("log_level")}
                options={LOG_LEVEL_OPTIONS} canUnset
                onSave={setText("LOG_LEVEL")}
                onUnset={async () => { await alter("UNSET LOG_LEVEL"); await load(); }}
              />
            </tbody>
          </table>

          {/* ── General ──────────────────────────────────────────────────── */}
          <div style={SECTION_HEAD}>General</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow search={search} label="Comment" type="text" canUnset value={get("comment")}
                onSave={setText("COMMENT")}
                onUnset={async () => { await alter("UNSET COMMENT"); await load(); }}
              />
              <TagsRow tags={objTags.tags} nameOptions={objTags.nameOptions} onSetTag={objTags.setTag} onUnsetTag={objTags.unsetTag} />
            </tbody>
          </table>
        </>
      )}
    </Modal>

    {showCreateFinalizer && (
      <CreateTaskModal
        db={db}
        schema={schema}
        mode="finalizer"
        finalizerForTask={name}
        onClose={() => setShowCreateFinalizer(false)}
        onSuccess={() => {
          setShowCreateFinalizer(false);
          setRootHasFinalizer(true);
        }}
      />
    )}

    {showHistory && (
      <TaskHistoryModal
        db={db}
        schema={schema}
        name={name}
        isRoot={!isFinalizer && predecessors.length === 0}
        onClose={() => setShowHistory(false)}
      />
    )}
    </>
  );
}
