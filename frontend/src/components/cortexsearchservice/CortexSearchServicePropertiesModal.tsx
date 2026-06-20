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
  Modal, Spin, Button, Input, InputNumber, Select, Space, Typography, Alert, Tag,
  Tooltip, Dropdown, Popconfirm,
} from "antd";
import {
  FileSearchOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PauseCircleOutlined, PlayCircleOutlined, SyncOutlined, DownOutlined,
  PlusOutlined, DeleteOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterCortexSearchService, ListWarehouses,
  FormatCortexSearchAttributes, GetCortexSearchServiceTags,
} from "../../../wailsjs/go/app/App";
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

const VALUE_TD: React.CSSProperties = {
  padding: "6px 0", fontSize: 12, color: "var(--text)",
  wordBreak: "break-word", verticalAlign: "middle",
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

// Quote a human-entered value as a SQL string literal. Snowflake treats the
// backslash as an escape character inside single-quoted literals, so backslashes
// must be doubled (first, so the doubled quotes aren't themselves read as an
// escape) as well as single-quotes — mirroring the backend's EscapeTextLit. Used
// for COMMENT / TARGET_LAG / TAG values.
function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

// Split a DESCRIBE comma list ("CAT, AUTHOR") into trimmed, non-blank tokens.
function splitList(s: string): string[] {
  return s.split(",").map((t) => t.trim()).filter((t) => t.length > 0);
}

// ─── EditRow (text) ──────────────────────────────────────────────────────────

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
      <td style={VALUE_TD}>
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

// ─── SelectEditRow (warehouse picker) ────────────────────────────────────────

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
      <td style={VALUE_TD}>
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
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={!draft} />
              </Tooltip>
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

// ─── ColumnsRow (column-list editor, e.g. Attributes / Primary key) ───────────

interface ColumnsRowProps {
  label: string;
  placeholder?: string;
  value: string[];
  onSave: (cols: string[]) => Promise<void>;
  onUnset: () => Promise<void>;
}

function ColumnsRow({ label, placeholder, value, onSave, onUnset }: ColumnsRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<string[]>(value);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (fn: () => Promise<void>) => {
    setSaving(true);
    setError(null);
    try {
      await fn();
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
      <td style={VALUE_TD}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Select
              size="small"
              mode="tags"
              value={draft}
              onChange={(v) => setDraft(v)}
              placeholder={placeholder ?? "CATEGORY, AUTHOR"}
              tokenSeparators={[","]}
              open={false}
              suffixIcon={null}
              style={{ width: 360 }}
            />
            <Space>
              <Tooltip title={`Save (SET ${label.toUpperCase()})`}>
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={() => run(() => onSave(draft))} loading={saving} disabled={draft.length === 0} />
              </Tooltip>
              <Tooltip title={`Unset ${label.toLowerCase()}`}>
                <Button size="small" onClick={() => run(onUnset)} loading={saving}>Unset</Button>
              </Tooltip>
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setDraft(value); setError(null); }} />
              </Tooltip>
            </Space>
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space wrap>
            {value.length > 0
              ? value.map((c) => <Tag key={c}>{c}</Tag>)
              : <Text type="secondary">(none)</Text>}
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

// ─── BoolRow (TRUE / FALSE setter, e.g. REQUEST_LOGGING) ──────────────────────

interface BoolRowProps {
  label: string;
  value: string;
  onSave: (val: boolean) => Promise<void>;
}

function BoolRow({ label, value, onSave }: BoolRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<boolean>(/^(true|on|1|yes)$/i.test(value.trim()));
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

  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={VALUE_TD}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <Select
                size="small"
                value={draft}
                onChange={(v) => setDraft(v)}
                style={{ width: 120 }}
                options={[
                  { value: true, label: "TRUE" },
                  { value: false, label: "FALSE" },
                ]}
              />
              <Tooltip title="Save">
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} />
              </Tooltip>
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setError(null); }} />
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
                onClick={() => { setDraft(/^(true|on|1|yes)$/i.test(value.trim())); setEditing(true); }}
                style={{ color: "var(--text-muted)" }}
              />
            </Tooltip>
          </Space>
        )}
      </td>
    </tr>
  );
}

// ─── NumberRow (numeric setter with optional unset, e.g. AUTO_SUSPEND) ─────────

interface NumberRowProps {
  label: string;
  value: string;
  // When canNull is set, the Unset button issues SET <prop> = NULL instead of an
  // UNSET clause (AUTO_SUSPEND has no UNSET; NULL clears it).
  unsetMode?: "unset" | "null" | "none";
  min?: number;
  onSave: (val: number) => Promise<void>;
  onClear?: () => Promise<void>;
}

function NumberRow({ label, value, unsetMode = "none", min, onSave, onClear }: NumberRowProps) {
  const [editing, setEditing] = useState(false);
  const [draft, setDraft] = useState<number | null>(value === "" ? null : Number(value));
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = async (fn: () => Promise<void>) => {
    setSaving(true);
    setError(null);
    try {
      await fn();
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
      <td style={VALUE_TD}>
        {editing ? (
          <Space direction="vertical" size={4} style={{ width: "100%" }}>
            <Space>
              <InputNumber
                size="small"
                min={min}
                value={draft}
                onChange={(v) => setDraft(v as number | null)}
                style={{ width: 140 }}
              />
              <Tooltip title="Save">
                <Button
                  size="small"
                  icon={<CheckOutlined />}
                  type="primary"
                  onClick={() => run(() => onSave(draft ?? 0))}
                  loading={saving}
                  disabled={draft == null}
                />
              </Tooltip>
              {unsetMode !== "none" && onClear && (
                <Tooltip title={unsetMode === "null" ? "Set to NULL" : "Unset"}>
                  <Button size="small" onClick={() => run(onClear)} loading={saving}>
                    {unsetMode === "null" ? "NULL" : "Unset"}
                  </Button>
                </Tooltip>
              )}
              <Tooltip title="Cancel">
                <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditing(false); setError(null); }} />
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
                onClick={() => { setDraft(value === "" ? null : Number(value)); setEditing(true); }}
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

export default function CortexSearchServicePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  // Tags currently applied (best-effort; TAG_REFERENCES needs privileges).
  const [tags, setTags] = useState<{ name: string; value: string }[]>([]);
  const [newTagName, setNewTagName] = useState("");
  const [newTagValue, setNewTagValue] = useState("");

  // Scoring-profile composer.
  const [profileName, setProfileName] = useState("");
  const [profileDef, setProfileDef] = useState("");
  const [dropProfileName, setDropProfileName] = useState("");

  useEffect(() => {
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  const reloadTags = useCallback(async () => {
    try {
      const res = await GetCortexSearchServiceTags(db, schema, name);
      const cols = (res?.columns ?? []).map((c) => c.toLowerCase());
      const ni = cols.indexOf("tag_name");
      const vi = cols.indexOf("tag_value");
      const out: { name: string; value: string }[] = [];
      for (const row of res?.rows ?? []) {
        out.push({
          name: ni >= 0 ? String(row[ni] ?? "") : "",
          value: vi >= 0 ? String(row[vi] ?? "") : "",
        });
      }
      setTags(out.filter((t) => t.name !== ""));
    } catch {
      // No governance privilege / unsupported domain — SET/UNSET still work.
      setTags([]);
    }
  }, [db, schema, name]);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "CORTEX SEARCH SERVICE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); reloadTags(); }, [reload, reloadTags]);

  const objRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const runAction = async (clause: string, label: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterCortexSearchService(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterCortexSearchService(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterCortexSearchService(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const saveTargetLag = async (lag: string) => {
    await AlterCortexSearchService(db, schema, name, `SET TARGET_LAG = ${q1(lag.trim())}`);
    await reload();
  };

  const saveWarehouse = async (wh: string) => {
    await AlterCortexSearchService(db, schema, name, `SET WAREHOUSE = "${wh.replace(/"/g, '""')}"`);
    await reload();
  };

  const saveAttributes = async (cols: string[]) => {
    const list = await FormatCortexSearchAttributes(cols);
    if (!list) return;
    await AlterCortexSearchService(db, schema, name, `SET ATTRIBUTES ( ${list} )`);
    await reload();
  };

  const unsetAttributes = async () => {
    await AlterCortexSearchService(db, schema, name, "UNSET ATTRIBUTES");
    await reload();
  };

  const savePrimaryKey = async (cols: string[]) => {
    const list = await FormatCortexSearchAttributes(cols);
    if (!list) return;
    await AlterCortexSearchService(db, schema, name, `SET PRIMARY KEY ( ${list} )`);
    await reload();
  };

  const unsetPrimaryKey = async () => {
    await AlterCortexSearchService(db, schema, name, "UNSET PRIMARY KEY");
    await reload();
  };

  const saveFullIndexInterval = async (days: number) => {
    await AlterCortexSearchService(db, schema, name, `SET FULL_INDEX_BUILD_INTERVAL_DAYS = ${days}`);
    await reload();
  };

  const saveRequestLogging = async (on: boolean) => {
    await AlterCortexSearchService(db, schema, name, `SET REQUEST_LOGGING = ${on ? "TRUE" : "FALSE"}`);
    await reload();
  };

  const saveAutoSuspend = async (secs: number) => {
    await AlterCortexSearchService(db, schema, name, `SET AUTO_SUSPEND = ${secs}`);
    await reload();
  };

  const clearAutoSuspend = async () => {
    await AlterCortexSearchService(db, schema, name, "SET AUTO_SUSPEND = NULL");
    await reload();
  };

  const setTag = async () => {
    const tn = newTagName.trim();
    if (tn === "") return;
    setBusy(true);
    setActionError(null);
    try {
      // Tag name may be a qualified identifier the user typed verbatim; the value
      // is a string literal.
      await AlterCortexSearchService(db, schema, name, `SET TAG ${tn} = ${q1(newTagValue)}`);
      setNewTagName("");
      setNewTagValue("");
      await reloadTags();
    } catch (e) {
      setActionError(`Set tag failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const unsetTag = async (tagName: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterCortexSearchService(db, schema, name, `UNSET TAG ${tagName}`);
      await reloadTags();
    } catch (e) {
      setActionError(`Unset tag failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const addScoringProfile = async () => {
    const pn = profileName.trim();
    const def = profileDef.trim();
    if (pn === "" || def === "") return;
    setBusy(true);
    setActionError(null);
    try {
      await AlterCortexSearchService(db, schema, name, `ADD SCORING PROFILE "${pn.replace(/"/g, '""')}"\n${def}`);
      setProfileName("");
      setProfileDef("");
      await reload();
    } catch (e) {
      setActionError(`Add scoring profile failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const dropScoringProfile = async () => {
    const pn = dropProfileName.trim();
    if (pn === "") return;
    setBusy(true);
    setActionError(null);
    try {
      await AlterCortexSearchService(db, schema, name, `DROP SCORING PROFILE IF EXISTS "${pn.replace(/"/g, '""')}"`);
      setDropProfileName("");
      await reload();
    } catch (e) {
      setActionError(`Drop scoring profile failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const comment = find("comment");
  const targetLag = find("target_lag");
  const warehouse = find("warehouse");
  const searchColumn = find("search_column");
  const attributeColumns = find("attribute_columns");
  const embeddingModel = find("embedding_model");
  const definition = find("definition");
  const servingState = find("serving_state");
  const indexingState = find("indexing_state");
  const indexingError = find("indexing_error");
  const primaryKey = find("primary_key");
  const autoSuspend = find("auto_suspend");
  const requestLogging = find("request_logging");
  const fullIndexInterval = find("full_index_build_interval_days");

  // Keys handled by the editable Settings / Overview sections or rendered
  // elsewhere; the catch-all Properties table skips them.
  const handledKeys = new Set([
    "comment", "target_lag", "warehouse", "search_column", "attribute_columns",
    "embedding_model", "definition", "serving_state", "indexing_state",
    "indexing_error", "primary_key", "auto_suspend", "request_logging",
    "full_index_build_interval_days",
  ]);

  const stateColor = (s: string) => (/active|running/i.test(s) ? "green" : "orange");

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <FileSearchOutlined style={{ color: "var(--link)" }} />
          <span>Cortex Search Service Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {objRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={740}
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

          <Space wrap>
            {indexingState && <Tag color={stateColor(indexingState)}>indexing: {indexingState}</Tag>}
            {servingState && <Tag color={stateColor(servingState)}>serving: {servingState}</Tag>}
            <Button size="small" icon={<SyncOutlined />} loading={busy} onClick={() => runAction("REFRESH", "Refresh")}>
              Refresh
            </Button>
            <Dropdown
              trigger={["click"]}
              menu={{
                items: [
                  { key: "all", label: "Suspend (indexing + serving)" },
                  { key: "indexing", label: "Suspend indexing" },
                  { key: "serving", label: "Suspend serving" },
                ],
                onClick: ({ key }) =>
                  runAction(key === "all" ? "SUSPEND" : `SUSPEND ${key.toUpperCase()}`, "Suspend"),
              }}
            >
              <Button size="small" icon={<PauseCircleOutlined />} loading={busy}>
                Suspend <DownOutlined style={{ fontSize: 9 }} />
              </Button>
            </Dropdown>
            <Dropdown
              trigger={["click"]}
              menu={{
                items: [
                  { key: "all", label: "Resume (indexing + serving)" },
                  { key: "indexing", label: "Resume indexing" },
                  { key: "serving", label: "Resume serving" },
                ],
                onClick: ({ key }) =>
                  runAction(key === "all" ? "RESUME" : `RESUME ${key.toUpperCase()}`, "Resume"),
              }}
            >
              <Button size="small" icon={<PlayCircleOutlined />} loading={busy}>
                Resume <DownOutlined style={{ fontSize: 9 }} />
              </Button>
            </Dropdown>
          </Space>

          {indexingError && (
            <Alert
              type="warning"
              message="Indexing error"
              description={indexingError}
              showIcon
              style={{ marginTop: 12 }}
            />
          )}

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Search column</td>
                <td style={VALUE_TD}>{searchColumn || <Text type="secondary">(unknown)</Text>}</td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Embedding model</td>
                <td style={VALUE_TD}>{embeddingModel || <Text type="secondary">(default)</Text>}</td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="Target Lag" value={targetLag} onSave={saveTargetLag} />
              <SelectEditRow
                label="Warehouse"
                value={warehouse}
                options={warehouses}
                loading={loadingWarehouses}
                onSave={saveWarehouse}
              />
              <ColumnsRow
                label="Attributes"
                placeholder="CATEGORY, AUTHOR"
                value={splitList(attributeColumns)}
                onSave={saveAttributes}
                onUnset={unsetAttributes}
              />
              <ColumnsRow
                label="Primary key"
                placeholder="ID"
                value={splitList(primaryKey)}
                onSave={savePrimaryKey}
                onUnset={unsetPrimaryKey}
              />
              <NumberRow
                label="Auto Suspend (s)"
                value={autoSuspend}
                unsetMode="null"
                min={0}
                onSave={saveAutoSuspend}
                onClear={clearAutoSuspend}
              />
              <NumberRow
                label="Full Index Build Interval (days)"
                value={fullIndexInterval}
                min={1}
                onSave={saveFullIndexInterval}
              />
              <BoolRow
                label="Request Logging"
                value={requestLogging}
                onSave={saveRequestLogging}
              />
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Tags</div>
          <Space direction="vertical" size={8} style={{ width: "100%" }}>
            <Space wrap>
              {tags.length > 0
                ? tags.map((t) => (
                    <Popconfirm
                      key={`${t.name}=${t.value}`}
                      title={`Unset tag ${t.name}?`}
                      onConfirm={() => unsetTag(t.name)}
                      okText="Unset"
                      cancelText="Cancel"
                    >
                      <Tag closable onClose={(e) => e.preventDefault()} style={{ cursor: "pointer" }}>
                        {t.name}{t.value ? ` = ${t.value}` : ""}
                      </Tag>
                    </Popconfirm>
                  ))
                : <Text type="secondary" style={{ fontSize: 12 }}>(no tags, or insufficient privilege to read them)</Text>}
            </Space>
            <Space>
              <Input
                size="small"
                placeholder="tag name (e.g. COST_CENTER)"
                value={newTagName}
                onChange={(e) => setNewTagName(e.target.value)}
                style={{ width: 220 }}
              />
              <Input
                size="small"
                placeholder="tag value"
                value={newTagValue}
                onChange={(e) => setNewTagValue(e.target.value)}
                onPressEnter={setTag}
                style={{ width: 180 }}
              />
              <Button size="small" type="primary" icon={<PlusOutlined />} loading={busy} disabled={newTagName.trim() === ""} onClick={setTag}>
                Set tag
              </Button>
            </Space>
          </Space>

          <div style={SECTION_HEAD}>Scoring Profiles</div>
          <Space direction="vertical" size={8} style={{ width: "100%" }}>
            <Input
              size="small"
              placeholder="profile name"
              value={profileName}
              onChange={(e) => setProfileName(e.target.value)}
              style={{ width: 280 }}
            />
            <Input.TextArea
              value={profileDef}
              onChange={(e) => setProfileDef(e.target.value)}
              placeholder={"functions = (\n  numeric_boosts = [ { column = 'POPULARITY' } ]\n)"}
              autoSize={{ minRows: 3, maxRows: 8 }}
              style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 11 }}
            />
            <Space>
              <Button size="small" type="primary" icon={<PlusOutlined />} loading={busy} disabled={profileName.trim() === "" || profileDef.trim() === ""} onClick={addScoringProfile}>
                Add scoring profile
              </Button>
            </Space>
            <Space>
              <Input
                size="small"
                placeholder="profile name to drop"
                value={dropProfileName}
                onChange={(e) => setDropProfileName(e.target.value)}
                onPressEnter={dropScoringProfile}
                style={{ width: 280 }}
              />
              <Button size="small" danger icon={<DeleteOutlined />} loading={busy} disabled={dropProfileName.trim() === ""} onClick={dropScoringProfile}>
                Drop scoring profile
              </Button>
            </Space>
          </Space>

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <tr key={r.key}>
                    <td style={LABEL_TD}>{r.key}</td>
                    <td style={VALUE_TD}>
                      {r.value || <Text type="secondary">(empty)</Text>}
                    </td>
                  </tr>
                ))}
            </tbody>
          </table>

          {definition && (
            <>
              <div style={SECTION_HEAD}>Base Query</div>
              <pre
                style={{
                  margin: 0,
                  padding: "10px 12px",
                  background: "var(--bg)",
                  border: "1px solid var(--border)",
                  borderRadius: 6,
                  color: "var(--text)",
                  fontSize: 11,
                  fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
                  whiteSpace: "pre-wrap",
                  wordBreak: "break-word",
                }}
              >
                {definition}
              </pre>
            </>
          )}
        </>
      )}
    </Modal>
  );
}
