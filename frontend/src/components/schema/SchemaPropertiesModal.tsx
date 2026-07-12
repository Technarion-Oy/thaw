// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Tag,
} from "antd";
import {
  FolderOutlined, EditOutlined, CheckOutlined, CloseOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterSchema, GetSchemaParameters,
  ListExternalVolumes, ListIntegrations, ListComputePools, ListWarehouses,
  ListSchemas, GetObjectTagReferences,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import TagsRow, { EditableTag } from "../shared/TagsRow";
import { quoteIdent } from "../shared/ObjectNameCaseControl";

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

// Escape a SQL text literal the way the backend's EscapeTextLit does — double
// backslashes (Snowflake interprets backslash escapes in string literals) then
// single quotes — so a value like C:\temp round-trips intact.
function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

// Fixed-choice ALTER SCHEMA parameters, read from SHOW PARAMETERS IN SCHEMA.
const opts = (...vs: string[]) => vs.map((v) => ({ value: v, label: v }));
const LOG_LEVELS = opts("TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF");
const TRACE_LEVELS = opts("ALWAYS", "ON_EVENT", "PROPAGATE", "OFF");
const SERIALIZATION = opts("COMPATIBLE", "OPTIMIZED");
const BOOLS = opts("TRUE", "FALSE");
const MERGE_BEHAVIOR = opts("AUTO", "ENABLED", "DISABLED");
const YES_NO = opts("YES", "NO");
const VISIBILITY = opts("PRIVILEGED");

// ─── EditRow ─────────────────────────────────────────────────────────────────

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

function InfoRow({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <tr>
      <td style={LABEL_TD}>{label}</td>
      <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)", wordBreak: "break-word" }}>
        {value || <Text type="secondary">(empty)</Text>}
      </td>
    </tr>
  );
}

// A fixed-choice parameter row: a Select that applies the change on pick, plus an
// Unset button (reset to default / inherited) when a value is currently set.
function SelectRow({ label, value, options, busy, onSet, onUnset }: {
  label: string;
  value: string;
  options: { value: string; label: string }[];
  busy: boolean;
  onSet: (v: string) => void;
  onUnset: () => void;
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
          {cur && (
            <Tooltip title="Unset (reset to default)">
              <Button size="small" onClick={onUnset} loading={busy}>Unset</Button>
            </Tooltip>
          )}
        </Space>
      </td>
    </tr>
  );
}

// An identifier-valued parameter row: a searchable Select populated from a live
// list (external volumes, catalog integrations, compute pools, warehouses …).
// The picked name is set case-sensitively (double-quoted) by the caller's onSet;
// onUnset clears it. If the list read fails the current value is still shown and
// unsettable — a fresh pick just isn't offered (use the SQL editor instead).
function PickerRow({ label, value, load, busy, onSet, onUnset }: {
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

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function SchemaPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [params, setParams] = useState<snowflake.QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [managedBusy, setManagedBusy] = useState(false);
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [tags, setTags] = useState<EditableTag[]>([]);
  const [siblings, setSiblings] = useState<string[]>([]);
  const [swapTarget, setSwapTarget] = useState<string | undefined>();
  const [swapBusy, setSwapBusy] = useState(false);

  const reload = useCallback(async () => {
    // Keep prior data rendered while refetching so an inline edit doesn't collapse
    // the modal to a spinner. The centered spinner shows only on first load.
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "SCHEMA", name);
      // SHOW SCHEMAS omits MAX_DATA_EXTENSION_TIME_IN_DAYS / DEFAULT_DDL_COLLATION;
      // SHOW PARAMETERS is the fallback source. Failure here is non-fatal.
      let p: snowflake.QueryResult | null = null;
      try {
        p = (await GetSchemaParameters(db, schema)) ?? null;
      } catch {
        p = null;
      }
      setRows(props ?? []);
      setParams(p);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  // Tags use the no-latency INFORMATION_SCHEMA.TAG_REFERENCES read so a SET/UNSET
  // reflects immediately. Best-effort: SET/UNSET still work if the read fails.
  // Tags inherited from the database/account are shown for context but can't be
  // unset here — that has to happen where they were applied.
  const reloadTags = useCallback(async () => {
    try {
      const t = await GetObjectTagReferences("SCHEMA", db, schema, name, "");
      const cols = (t?.columns ?? []).map((c) => c.toLowerCase());
      const ci = (n: string) => cols.indexOf(n);
      const dbI = ci("tag_database"), scI = ci("tag_schema"), nmI = ci("tag_name"),
        vlI = ci("tag_value"), lvI = ci("level");
      setTags((t?.rows ?? []).map((row): EditableTag => {
        const tdb = dbI >= 0 ? String(row[dbI] ?? "") : "";
        const tsc = scI >= 0 ? String(row[scI] ?? "") : "";
        const tnm = nmI >= 0 ? String(row[nmI] ?? "") : "";
        const qualified = [tdb, tsc, tnm].filter(Boolean).map(quoteIdent).join(".");
        const inherited = lvI >= 0 && String(row[lvI] ?? "").toUpperCase() !== "SCHEMA";
        return {
          key: qualified,
          name: tnm,
          value: vlI >= 0 ? String(row[vlI] ?? "") : "",
          removable: !inherited,
          suffix: inherited ? " (inherited)" : "",
        };
      }));
    } catch {
      setTags([]);
    }
  }, [db, schema, name]);

  useEffect(() => { reloadTags(); }, [reloadTags]);

  // Sibling schemas in the same database, for the SWAP WITH target picker.
  useEffect(() => {
    ListSchemas(db)
      .then((s) => setSiblings((s ?? []).filter((n) => n.toUpperCase() !== name.toUpperCase())))
      .catch(() => setSiblings([]));
  }, [db, name]);

  const schemaRef = `"${db}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  // Pull a parameter's current value out of the SHOW PARAMETERS result (columns
  // are key / value / default / …; we want the row whose key matches).
  const paramVal = (key: string): string => {
    if (!params) return "";
    const cols = (params.columns ?? []).map((c) => c.toLowerCase());
    const keyCi = cols.indexOf("key");
    const valCi = cols.indexOf("value");
    if (keyCi < 0 || valCi < 0) return "";
    const row = (params.rows ?? []).find((r) => String(r[keyCi] ?? "").toLowerCase() === key.toLowerCase());
    return row ? String(row[valCi] ?? "") : "";
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterSchema(db, schema, "UNSET COMMENT");
    } else {
      await AlterSchema(db, schema, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  // SET/UNSET a non-negative-integer parameter. EditRow surfaces thrown errors inline.
  const saveIntParam = (param: string) => async (val: string) => {
    const v = val.trim();
    if (v === "") {
      await AlterSchema(db, schema, `UNSET ${param}`);
    } else {
      if (!/^\d+$/.test(v)) throw new Error("Must be a non-negative integer.");
      await AlterSchema(db, schema, `SET ${param} = ${v}`);
    }
    await reload();
  };

  // SET/UNSET a free-text string parameter (quoted as a string literal on SET,
  // UNSET when cleared). Used for the Iceberg text params and BASE_LOCATION_PREFIX.
  const saveTextParam = (param: string) => async (val: string) => {
    const v = val.trim();
    if (v === "") {
      await AlterSchema(db, schema, `UNSET ${param}`);
    } else {
      await AlterSchema(db, schema, `SET ${param} = ${q1(v)}`);
    }
    await reload();
  };

  const saveCollation = async (val: string) => {
    if (val.trim() === "") {
      await AlterSchema(db, schema, "UNSET DEFAULT_DDL_COLLATION");
    } else {
      await AlterSchema(db, schema, `SET DEFAULT_DDL_COLLATION = ${q1(val)}`);
    }
    await reload();
  };

  const saveRename = async (val: string) => {
    const newName = val.trim();
    if (newName === "" || newName === name) return;
    // RENAME TO takes a fully-qualified target; keep the schema in the same db.
    await AlterSchema(db, schema, `RENAME TO "${db.replace(/"/g, '""')}"."${newName.replace(/"/g, '""')}"`);
    // The modal's name/schema props are now stale — close and let the sidebar refresh.
    onClose();
  };

  // Apply a fixed-choice SET/UNSET parameter change (value comes from a closed
  // option list, so the clause is safe to interpolate). busyKey drives per-row
  // spinners; errors surface in the modal-level Alert.
  const applyParam = (key: string) => async (clause: string) => {
    setBusyKey(key);
    setActionError(null);
    try {
      await AlterSchema(db, schema, clause);
      await reload();
    } catch (e) {
      setActionError(`${key} update failed: ${String(e)}`);
    } finally {
      setBusyKey(null);
    }
  };

  const setManagedAccess = async (enable: boolean) => {
    setManagedBusy(true);
    setActionError(null);
    try {
      await AlterSchema(db, schema, enable ? "ENABLE MANAGED ACCESS" : "DISABLE MANAGED ACCESS");
      await reload();
    } catch (e) {
      setActionError(`Managed access update failed: ${String(e)}`);
    } finally {
      setManagedBusy(false);
    }
  };

  const setTag = async (tagName: string, tagValue: string) => {
    // Tag name may be a qualified identifier (db.schema.tag) — inserted verbatim;
    // the value is a quoted string literal.
    await AlterSchema(db, schema, `SET TAG ${tagName} = ${q1(tagValue)}`);
    await reloadTags();
  };

  const unsetTag = async (qualified: string) => {
    await AlterSchema(db, schema, `UNSET TAG ${qualified}`);
    await reloadTags();
  };

  // SWAP WITH exchanges ALL contents of two schemas — destructive, so confirm
  // first. On success the modal's name context is stale, so close and let the
  // sidebar refresh.
  const doSwap = () => {
    if (!swapTarget) return;
    Modal.confirm({
      title: "Swap schema contents?",
      content: `This exchanges every object between "${name}" and "${swapTarget}". It is disruptive and can't be undone automatically.`,
      okText: "Swap",
      okButtonProps: { danger: true },
      onOk: async () => {
        setSwapBusy(true);
        setActionError(null);
        try {
          await AlterSchema(db, schema, `SWAP WITH ${quoteIdent(swapTarget)}`);
          onClose();
        } catch (e) {
          setActionError(`Swap failed: ${String(e)}`);
          setSwapBusy(false);
        }
      },
    });
  };

  const comment = find("comment");
  const owner = find("owner");
  const createdOn = find("created_on");
  // Prefer the SHOW dump; fall back to SHOW PARAMETERS.
  const retention = find("retention_time") || paramVal("DATA_RETENTION_TIME_IN_DAYS");
  const maxExtension = paramVal("MAX_DATA_EXTENSION_TIME_IN_DAYS");
  const collation = paramVal("DEFAULT_DDL_COLLATION");
  const managed = /managed\s+access/i.test(find("options"));
  const logLevel = paramVal("LOG_LEVEL");
  const traceLevel = paramVal("TRACE_LEVEL");
  const serialization = paramVal("STORAGE_SERIALIZATION_POLICY");
  const replaceInvalid = paramVal("REPLACE_INVALID_CHARACTERS");
  const externalVolume = paramVal("EXTERNAL_VOLUME");
  const catalog = paramVal("CATALOG");
  const catalogSync = paramVal("CATALOG_SYNC");
  const notebookCpu = paramVal("DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU");
  const notebookGpu = paramVal("DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU");
  const streamlitWh = paramVal("DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE");
  const icebergCollation = paramVal("ICEBERG_DEFAULT_DDL_COLLATION");
  const icebergVersion = paramVal("ICEBERG_VERSION_DEFAULT");
  const icebergMerge = paramVal("ICEBERG_MERGE_ON_READ_BEHAVIOR");
  const enableIcebergMerge = paramVal("ENABLE_ICEBERG_MERGE_ON_READ");
  const baseLocationPrefix = paramVal("BASE_LOCATION_PREFIX");
  const objectVisibility = paramVal("OBJECT_VISIBILITY");
  const dataCompaction = paramVal("ENABLE_DATA_COMPACTION");
  const replicableFailover = paramVal("REPLICABLE_WITH_FAILOVER_GROUPS");

  const catalogNames = () => ListIntegrations("CATALOG").then((rs) => (rs ?? []).map((r) => r.name));

  // Keys rendered in the dedicated sections, hidden from the generic Properties dump.
  const handledKeys = new Set([
    "comment", "owner", "created_on", "retention_time", "options", "name",
  ]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <FolderOutlined style={{ color: "var(--icon-schema)" }} />
          <span>Schema Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {schemaRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={720}
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

          <div style={SECTION_HEAD}>Overview</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <InfoRow label="Owner" value={owner} />
              <InfoRow label="Created on" value={createdOn} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
              {/* Editable (not InfoRow, which reads as read-only): the Tag shows
                  the current state and the Select applies the change. */}
              <tr>
                <td style={LABEL_TD}>Managed access</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space>
                    <Tag color={managed ? "green" : "default"}>{managed ? "ENABLED" : "DISABLED"}</Tag>
                    <Select
                      size="small"
                      value={managed ? "on" : "off"}
                      onChange={(v) => setManagedAccess(v === "on")}
                      loading={managedBusy}
                      style={{ width: 110 }}
                      options={[{ value: "on", label: "Enabled" }, { value: "off", label: "Disabled" }]}
                    />
                  </Space>
                </td>
              </tr>
              <EditRow
                label="Data retention (days)"
                value={retention}
                canUnset={retention !== ""}
                onSave={saveIntParam("DATA_RETENTION_TIME_IN_DAYS")}
                onUnset={() => saveIntParam("DATA_RETENTION_TIME_IN_DAYS")("")}
              />
              <EditRow
                label="Max data extension (days)"
                value={maxExtension}
                canUnset={maxExtension !== ""}
                onSave={saveIntParam("MAX_DATA_EXTENSION_TIME_IN_DAYS")}
                onUnset={() => saveIntParam("MAX_DATA_EXTENSION_TIME_IN_DAYS")("")}
              />
              <EditRow
                label="Default DDL collation"
                value={collation}
                canUnset={collation !== ""}
                onSave={saveCollation}
                onUnset={() => saveCollation("")}
              />
              <EditRow
                label="Rename to"
                value={name}
                onSave={saveRename}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Tags</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <TagsRow tags={tags} onSetTag={setTag} onUnsetTag={unsetTag} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Storage &amp; Iceberg</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <PickerRow
                label="External volume"
                value={externalVolume}
                load={ListExternalVolumes}
                busy={busyKey === "EXTERNAL_VOLUME"}
                onSet={(v) => applyParam("EXTERNAL_VOLUME")(`SET EXTERNAL_VOLUME = ${quoteIdent(v)}`)}
                onUnset={() => applyParam("EXTERNAL_VOLUME")("UNSET EXTERNAL_VOLUME")}
              />
              <PickerRow
                label="Catalog"
                value={catalog}
                load={catalogNames}
                busy={busyKey === "CATALOG"}
                onSet={(v) => applyParam("CATALOG")(`SET CATALOG = ${quoteIdent(v)}`)}
                onUnset={() => applyParam("CATALOG")("UNSET CATALOG")}
              />
              <PickerRow
                label="Catalog sync"
                value={catalogSync}
                load={catalogNames}
                busy={busyKey === "CATALOG_SYNC"}
                onSet={(v) => applyParam("CATALOG_SYNC")(`SET CATALOG_SYNC = ${q1(v)}`)}
                onUnset={() => applyParam("CATALOG_SYNC")("UNSET CATALOG_SYNC")}
              />
              <EditRow
                label="Iceberg default DDL collation"
                value={icebergCollation}
                canUnset={icebergCollation !== ""}
                onSave={saveTextParam("ICEBERG_DEFAULT_DDL_COLLATION")}
                onUnset={() => saveTextParam("ICEBERG_DEFAULT_DDL_COLLATION")("")}
              />
              <EditRow
                label="Iceberg version default"
                value={icebergVersion}
                canUnset={icebergVersion !== ""}
                onSave={saveIntParam("ICEBERG_VERSION_DEFAULT")}
                onUnset={() => saveIntParam("ICEBERG_VERSION_DEFAULT")("")}
              />
              <SelectRow
                label="Iceberg merge-on-read behavior"
                value={icebergMerge}
                options={MERGE_BEHAVIOR}
                busy={busyKey === "ICEBERG_MERGE_ON_READ_BEHAVIOR"}
                onSet={(v) => applyParam("ICEBERG_MERGE_ON_READ_BEHAVIOR")(`SET ICEBERG_MERGE_ON_READ_BEHAVIOR = ${q1(v)}`)}
                onUnset={() => applyParam("ICEBERG_MERGE_ON_READ_BEHAVIOR")("UNSET ICEBERG_MERGE_ON_READ_BEHAVIOR")}
              />
              <SelectRow
                label="Enable Iceberg merge-on-read"
                value={enableIcebergMerge}
                options={BOOLS}
                busy={busyKey === "ENABLE_ICEBERG_MERGE_ON_READ"}
                onSet={(v) => applyParam("ENABLE_ICEBERG_MERGE_ON_READ")(`SET ENABLE_ICEBERG_MERGE_ON_READ = ${v}`)}
                onUnset={() => applyParam("ENABLE_ICEBERG_MERGE_ON_READ")("UNSET ENABLE_ICEBERG_MERGE_ON_READ")}
              />
              <EditRow
                label="Base location prefix"
                value={baseLocationPrefix}
                canUnset={baseLocationPrefix !== ""}
                onSave={saveTextParam("BASE_LOCATION_PREFIX")}
                onUnset={() => saveTextParam("BASE_LOCATION_PREFIX")("")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Notebook &amp; Streamlit</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <PickerRow
                label="Default notebook compute pool (CPU)"
                value={notebookCpu}
                load={ListComputePools}
                busy={busyKey === "DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU"}
                onSet={(v) => applyParam("DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU")(`SET DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU = ${q1(v)}`)}
                onUnset={() => applyParam("DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU")("UNSET DEFAULT_NOTEBOOK_COMPUTE_POOL_CPU")}
              />
              <PickerRow
                label="Default notebook compute pool (GPU)"
                value={notebookGpu}
                load={ListComputePools}
                busy={busyKey === "DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU"}
                onSet={(v) => applyParam("DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU")(`SET DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU = ${q1(v)}`)}
                onUnset={() => applyParam("DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU")("UNSET DEFAULT_NOTEBOOK_COMPUTE_POOL_GPU")}
              />
              <PickerRow
                label="Default Streamlit notebook warehouse"
                value={streamlitWh}
                load={ListWarehouses}
                busy={busyKey === "DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE"}
                onSet={(v) => applyParam("DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE")(`SET DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE = ${q1(v)}`)}
                onUnset={() => applyParam("DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE")("UNSET DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Parameters</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <SelectRow
                label="Log level"
                value={logLevel}
                options={LOG_LEVELS}
                busy={busyKey === "LOG_LEVEL"}
                onSet={(v) => applyParam("LOG_LEVEL")(`SET LOG_LEVEL = ${q1(v)}`)}
                onUnset={() => applyParam("LOG_LEVEL")("UNSET LOG_LEVEL")}
              />
              <SelectRow
                label="Trace level"
                value={traceLevel}
                options={TRACE_LEVELS}
                busy={busyKey === "TRACE_LEVEL"}
                onSet={(v) => applyParam("TRACE_LEVEL")(`SET TRACE_LEVEL = ${q1(v)}`)}
                onUnset={() => applyParam("TRACE_LEVEL")("UNSET TRACE_LEVEL")}
              />
              <SelectRow
                label="Storage serialization policy"
                value={serialization}
                options={SERIALIZATION}
                busy={busyKey === "STORAGE_SERIALIZATION_POLICY"}
                onSet={(v) => applyParam("STORAGE_SERIALIZATION_POLICY")(`SET STORAGE_SERIALIZATION_POLICY = ${v}`)}
                onUnset={() => applyParam("STORAGE_SERIALIZATION_POLICY")("UNSET STORAGE_SERIALIZATION_POLICY")}
              />
              <SelectRow
                label="Replace invalid characters"
                value={replaceInvalid}
                options={BOOLS}
                busy={busyKey === "REPLACE_INVALID_CHARACTERS"}
                onSet={(v) => applyParam("REPLACE_INVALID_CHARACTERS")(`SET REPLACE_INVALID_CHARACTERS = ${v}`)}
                onUnset={() => applyParam("REPLACE_INVALID_CHARACTERS")("UNSET REPLACE_INVALID_CHARACTERS")}
              />
              <SelectRow
                label="Object visibility"
                value={objectVisibility}
                options={VISIBILITY}
                busy={busyKey === "OBJECT_VISIBILITY"}
                onSet={(v) => applyParam("OBJECT_VISIBILITY")(`SET OBJECT_VISIBILITY = ${v}`)}
                onUnset={() => applyParam("OBJECT_VISIBILITY")("UNSET OBJECT_VISIBILITY")}
              />
              <SelectRow
                label="Enable data compaction"
                value={dataCompaction}
                options={BOOLS}
                busy={busyKey === "ENABLE_DATA_COMPACTION"}
                onSet={(v) => applyParam("ENABLE_DATA_COMPACTION")(`SET ENABLE_DATA_COMPACTION = ${v}`)}
                onUnset={() => applyParam("ENABLE_DATA_COMPACTION")("UNSET ENABLE_DATA_COMPACTION")}
              />
              <SelectRow
                label="Replicable with failover groups"
                value={replicableFailover}
                options={YES_NO}
                busy={busyKey === "REPLICABLE_WITH_FAILOVER_GROUPS"}
                onSet={(v) => applyParam("REPLICABLE_WITH_FAILOVER_GROUPS")(`SET REPLICABLE_WITH_FAILOVER_GROUPS = ${q1(v)}`)}
                onUnset={() => applyParam("REPLICABLE_WITH_FAILOVER_GROUPS")("UNSET REPLICABLE_WITH_FAILOVER_GROUPS")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Danger zone</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Swap with</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space>
                    <Select
                      size="small"
                      showSearch
                      value={swapTarget}
                      placeholder="Target schema"
                      style={{ width: 240 }}
                      options={siblings.map((s) => ({ value: s, label: s }))}
                      onChange={setSwapTarget}
                    />
                    <Button size="small" danger disabled={!swapTarget} loading={swapBusy} onClick={doSwap}>
                      Swap…
                    </Button>
                  </Space>
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Properties</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {rows
                .filter((r) => !handledKeys.has(r.key.toLowerCase()))
                .map((r) => (
                  <InfoRow key={r.key} label={r.key} value={r.value} />
                ))}
            </tbody>
          </table>
        </>
      )}
    </Modal>
  );
}
