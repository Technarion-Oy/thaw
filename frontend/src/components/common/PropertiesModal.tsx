// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import { Modal, Spin, Button, Input, InputNumber, message } from "antd";
import { CopyOutlined, EditOutlined, CheckOutlined, CloseOutlined, SearchOutlined } from "@ant-design/icons";
import { ConfirmSwitch } from "./ConfirmSwitch";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import type { snowflake, table } from "../../../wailsjs/go/models";
import { GetTableSettings, AlterTableProperty, AlterTable, GetObjectParameters } from "../../../wailsjs/go/app/App";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";
import { q1, opts, EditRow, SelectRow, ActionRow } from "../shared/PropertyRows";
import * as dtc from "../dynamictable/dynamicTableClauses";
import { dataMetricSchedule } from "./tableClauses";
import GovernanceRows from "./GovernanceRows";

interface TableContext {
  db: string;
  schema: string;
  table: string;
}

interface Props {
  title: string;
  rows: snowflake.PropertyPair[] | null;  // null = loading
  error: string | null;
  onClose: () => void;
  tableContext?: TableContext;
  /** Called after a Table Settings change so the caller can re-fetch `rows` (and drop stale caches). */
  onChanged?: () => void;
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

type PropKey = keyof table.TableSettings;

interface PropDef {
  key:   PropKey;
  label: string;
  type:  "text" | "number" | "boolean";
  hint?: string;
  unset?: string; // the UNSET clause that resets the property (Reset button)
}

const PROP_DEFS: PropDef[] = [
  { key: "comment",               label: "Comment",                  type: "text",    unset: "UNSET COMMENT" },
  { key: "clusterBy",             label: "Cluster By",               type: "text",    hint: "Comma-separated column expressions. Clear to drop clustering key." },
  { key: "enableSchemaEvolution", label: "Enable Schema Evolution",  type: "boolean", unset: "UNSET ENABLE_SCHEMA_EVOLUTION" },
  { key: "changeTracking",        label: "Change Tracking",          type: "boolean", unset: "UNSET CHANGE_TRACKING" },
  { key: "dataRetentionDays",     label: "Data Retention (days)",    type: "number",  unset: "UNSET DATA_RETENTION_TIME_IN_DAYS" },
  { key: "maxDataExtensionDays",  label: "Max Data Extension (days)",type: "number",  unset: "UNSET MAX_DATA_EXTENSION_TIME_IN_DAYS" },
  { key: "defaultDDLCollation",   label: "Default DDL Collation",    type: "text",    unset: "UNSET DEFAULT_DDL_COLLATION" },
];

function TableSettingsSection({ db, schema, table, rows, onChanged }: { db: string; schema: string; table: string; rows: snowflake.PropertyPair[] | null; onChanged?: () => void }) {
  const [settings, setSettings]   = useState<table.TableSettings | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [editKey, setEditKey]     = useState<PropKey | null>(null);
  const [editVal, setEditVal]     = useState<string>("");
  const [saving, setSaving]       = useState(false);

  const [params, setParams]       = useState<snowflake.SessionParam[]>([]);
  const [rowBusy, setRowBusy]     = useState(false);
  // ERROR_LOGGING / ROW_TIMESTAMP aren't documented SHOW TABLES columns; show them
  // if present, else whatever was last set here (so Unset appears after a set).
  // SHOW TABLES reports these as ON / OFF; the Select offers TRUE / FALSE.
  const showVal = (k: string) => {
    const v = (rows?.find((r) => r.key.toLowerCase() === k)?.value ?? "").toUpperCase();
    return v === "ON" ? "TRUE" : v === "OFF" ? "FALSE" : v;
  };
  const [errorLoggingSet, setErrorLogging] = useState<string | null>(null);
  const [rowTimestampSet, setRowTimestamp] = useState<string | null>(null);
  const errorLogging = errorLoggingSet ?? showVal("error_logging");
  const rowTimestamp = rowTimestampSet ?? showVal("row_timestamp");

  const reload = () => {
    GetTableSettings(db, schema, table)
      .then(setSettings)
      .catch((e) => setLoadError(String(e)));
    // Best-effort: the rows below still edit fine (showing "not set") if it fails.
    GetObjectParameters("TABLE", [db, schema, table]).then((ps) => setParams(ps ?? [])).catch(() => {});
  };
  useEffect(reload, [db, schema, table]); // eslint-disable-line react-hooks/exhaustive-deps

  const param = (key: string) => params.find((p) => p.key.toUpperCase() === key)?.value ?? "";
  const apply = async (clause: string) => {
    await AlterTable(db, schema, table, clause);
    reload();
    onChanged?.();
  };
  // SelectRow handlers are fire-and-forget, so surface errors as a toast.
  const applyRow = (clause: string, after?: () => void) => {
    setRowBusy(true);
    apply(clause)
      .then(() => { after?.(); message.success("Table updated"); })
      .catch((e) => message.error(String(e)))
      .finally(() => setRowBusy(false));
  };

  const objTags = useObjectTags({
    kind: "TABLE", db, schema, name: table,
    alter: (clause) => AlterTable(db, schema, table, clause),
  });

  const startEdit = (key: PropKey, current: string) => {
    setEditKey(key);
    setEditVal(current);
  };

  const cancelEdit = () => { setEditKey(null); setEditVal(""); };

  const saveEdit = async (def: PropDef) => {
    setSaving(true);
    try {
      // A blank text value clears the property (SET COMMENT = '' would store an
      // empty string; UNSET resets collation to the inherited default).
      if (def.type === "text" && def.unset && editVal.trim() === "" && def.key !== "clusterBy") {
        await AlterTable(db, schema, table, def.unset);
      } else {
        await AlterTableProperty(db, schema, table, def.key, editVal);
      }
      setSettings((prev) => prev ? { ...prev, [def.key]: def.type === "number" ? parseInt(editVal, 10) : editVal } : prev);
      setEditKey(null);
      onChanged?.();
      message.success(`${def.label} updated`);
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  const toggleBool = async (def: PropDef, checked: boolean) => {
    await AlterTableProperty(db, schema, table, def.key, checked ? "TRUE" : "FALSE");
    setSettings((prev) => prev ? { ...prev, [def.key]: checked } : prev);
    onChanged?.();
    message.success(`${def.label} ${checked ? "enabled" : "disabled"}`);
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
                      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
                        <ConfirmSwitch
                          checked={Boolean(rawVal)}
                          onConfirm={(checked) => toggleBool(def, checked)}
                        />
                        {def.unset && (
                          <Button size="small" type="text" loading={rowBusy} title="Unset (reset to the inherited default)"
                            onClick={() => applyRow(def.unset!)} style={{ color: "var(--text-faint)" }}>
                            Reset
                          </Button>
                        )}
                      </div>
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
                        {def.unset && strVal !== "" && (
                          <Button size="small" type="text" loading={rowBusy} title="Unset (reset to the inherited default)"
                            onClick={() => applyRow(def.unset!)} style={{ flexShrink: 0, color: "var(--text-faint)" }}>
                            Reset
                          </Button>
                        )}
                      </div>
                    )}
                  </td>
                </tr>
              );
            })}
            <SelectRow
              label="Error Logging"
              value={errorLogging}
              options={opts("TRUE", "FALSE")}
              busy={rowBusy}
              onSet={(v) => applyRow(`SET ERROR_LOGGING = ${v}`, () => setErrorLogging(v))}
              onUnset={() => applyRow("UNSET ERROR_LOGGING", () => setErrorLogging(""))}
            />
            <SelectRow
              label="Row Timestamp"
              value={rowTimestamp}
              options={opts("TRUE", "FALSE")}
              busy={rowBusy}
              onSet={(v) => applyRow(`SET ROW_TIMESTAMP = ${v}`, () => setRowTimestamp(v))}
              onUnset={() => applyRow("UNSET ROW_TIMESTAMP", () => setRowTimestamp(""))}
            />
            <EditRow
              label="Iceberg Default DDL Collation"
              value={param("ICEBERG_DEFAULT_DDL_COLLATION")}
              canUnset
              onSave={(v) => apply(v.trim() ? `SET ICEBERG_DEFAULT_DDL_COLLATION = ${q1(v.trim())}` : "UNSET ICEBERG_DEFAULT_DDL_COLLATION")}
              onUnset={() => apply("UNSET ICEBERG_DEFAULT_DDL_COLLATION")}
            />
            <EditRow
              label="Data Metric Schedule"
              value={param("DATA_METRIC_SCHEDULE")}
              canUnset
              onSave={(v) => apply(dataMetricSchedule(v))}
              onUnset={() => apply("UNSET DATA_METRIC_SCHEDULE")}
            />
            <TagsRow tags={objTags.tags} nameOptions={objTags.nameOptions} onSetTag={objTags.setTag} onUnsetTag={objTags.unsetTag} />
            {/* SET / UNSET CONTACT can't share a statement with other properties. */}
            <ActionRow
              label="Contact"
              fields={["purpose (e.g. SUPPORT)", "contact name"]}
              actions={[
                { label: "Set", clause: ([p, c]) => dtc.setContact(p, c) },
                { label: "Unset", clause: ([p]) => `UNSET CONTACT ${dtc.contactPurpose(p)}` },
              ]}
              run={apply}
            />
          </tbody>
        </table>
      )}
      {settings && <GovernanceRows apply={apply} joinPolicy />}
    </div>
  );
}

export default function PropertiesModal({ title, rows, error, onClose, tableContext, onChanged }: Props) {
  const loading = rows === null && !error;
  const [search, setSearch] = useState("");
  const q = search.trim().toLowerCase();

  const filteredRows = rows ? (q ? rows.filter((r) => r.key.toLowerCase().includes(q)) : rows) : null;

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

      {rows && rows.length > 0 && (
        <Input
          prefix={<SearchOutlined style={{ color: "var(--text-faint)" }} />}
          placeholder="Search by name…"
          allowClear
          autoFocus
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          style={{ marginBottom: 12 }}
        />
      )}

      {rows && rows.length === 0 && !error && (
        <div style={{ color: "var(--text-muted)", fontSize: 13, padding: 8 }}>
          No properties found.
        </div>
      )}

      <div style={{ maxHeight: "65vh", overflowY: "auto" }}>
        {filteredRows && filteredRows.length > 0 && (
          <table style={{ width: "100%", borderCollapse: "collapse", fontSize: 12 }}>
            <tbody>
              {filteredRows.map((row) => (
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

        {filteredRows && filteredRows.length === 0 && q && (
          <div style={{ color: "var(--text-muted)", fontSize: 12, padding: "4px 0 8px" }}>
            No matches.
          </div>
        )}

        {tableContext && <TableSettingsSection {...tableContext} rows={rows} onChanged={onChanged} />}
      </div>
    </Modal>
  );
}
