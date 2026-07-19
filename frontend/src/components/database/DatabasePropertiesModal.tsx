// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, Select, Space, Typography, Alert, Tooltip, Checkbox,
} from "antd";
import { DatabaseOutlined } from "@ant-design/icons";
import {
  GetObjectProperties, AlterDatabase, GetDatabaseParameters,
  ListExternalVolumes, ListIntegrations, ListComputePools, ListWarehouses,
  ListUserDatabases, ListEventTables, GetObjectTagReferences,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import TagsRow, { EditableTag } from "../shared/TagsRow";
import ObjectParametersModal from "../common/ObjectParametersModal";
import { quoteIdent } from "../shared/ObjectNameCaseControl";
import {
  SECTION_HEAD, LABEL_TD, q1, opts, paramValue,
  EditRow, InfoRow, SelectRow, PickerRow,
} from "../shared/PropertyRows";

const { Text } = Typography;

// Fixed-choice ALTER DATABASE parameters, read from SHOW PARAMETERS IN DATABASE.
const LOG_LEVELS = opts("TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF");
const METRIC_LEVELS = opts("ALL", "NONE");
const TRACE_LEVELS = opts("ALWAYS", "ON_EVENT", "PROPAGATE", "OFF");
const SERIALIZATION = opts("COMPATIBLE", "OPTIMIZED");
const BOOLS = opts("TRUE", "FALSE");
const MERGE_BEHAVIOR = opts("AUTO", "ENABLED", "DISABLED");
const YES_NO = opts("YES", "NO");
const VISIBILITY = opts("PRIVILEGED");

// Quote a dotted qualified identifier (DB.SCHEMA.NAME) part-by-part.
// ponytail: names containing literal dots aren't handled — SHOW never returns
// those unquoted, and the picker only feeds SHOW output here.
const quoteQualified = (v: string) => v.split(".").map(quoteIdent).join(".");


// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  name: string;
  onClose: () => void;
}

export default function DatabasePropertiesModal({ db, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [params, setParams] = useState<snowflake.QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busyKey, setBusyKey] = useState<string | null>(null);
  const [tags, setTags] = useState<EditableTag[]>([]);
  const [siblings, setSiblings] = useState<string[]>([]);
  const [swapTarget, setSwapTarget] = useState<string | undefined>();
  const [swapBusy, setSwapBusy] = useState(false);
  // Replication / failover account list (comma-separated org.account identifiers).
  const [accounts, setAccounts] = useState("");
  const [ignoreEdition, setIgnoreEdition] = useState(false);
  const [showParams, setShowParams] = useState(false);
  const [replBusy, setReplBusy] = useState<string | null>(null);
  // CONTACT <purpose> = <contact_name> editor (one pair at a time).
  const [contactPurpose, setContactPurpose] = useState("");
  const [contactName, setContactName] = useState("");
  const [contactBusy, setContactBusy] = useState<string | null>(null);
  // DATA_QUALITY_MONITORING_SETTINGS YAML spec.
  const [dqms, setDqms] = useState("");
  const [dqmsBusy, setDqmsBusy] = useState(false);

  const reload = useCallback(async () => {
    // Keep prior data rendered while refetching so an inline edit doesn't collapse
    // the modal to a spinner. The centered spinner shows only on first load.
    setError(null);
    try {
      const props = await GetObjectProperties(db, "", "DATABASE", name);
      // SHOW DATABASES omits most parameters; SHOW PARAMETERS is the fallback
      // source. Failure here is non-fatal.
      let p: snowflake.QueryResult | null = null;
      try {
        p = (await GetDatabaseParameters(db)) ?? null;
      } catch {
        p = null;
      }
      setRows(props ?? []);
      setParams(p);
    } catch (e) {
      setError(String(e));
    }
  }, [db, name]);

  useEffect(() => { reload(); }, [reload]);

  // Tags use the no-latency INFORMATION_SCHEMA.TAG_REFERENCES read so a SET/UNSET
  // reflects immediately. Best-effort. Inherited (account-level) tags are shown
  // for context but can't be unset here.
  const reloadTags = useCallback(async () => {
    try {
      const t = await GetObjectTagReferences("DATABASE", db, "", name, "");
      const cols = (t?.columns ?? []).map((c) => c.toLowerCase());
      const ci = (n: string) => cols.indexOf(n);
      const dbI = ci("tag_database"), scI = ci("tag_schema"), nmI = ci("tag_name"),
        vlI = ci("tag_value"), lvI = ci("level");
      setTags((t?.rows ?? []).map((row): EditableTag => {
        const tdb = dbI >= 0 ? String(row[dbI] ?? "") : "";
        const tsc = scI >= 0 ? String(row[scI] ?? "") : "";
        const tnm = nmI >= 0 ? String(row[nmI] ?? "") : "";
        const qualified = [tdb, tsc, tnm].filter(Boolean).map(quoteIdent).join(".");
        const inherited = lvI >= 0 && String(row[lvI] ?? "").toUpperCase() !== "DATABASE";
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
  }, [db, name]);

  useEffect(() => { reloadTags(); }, [reloadTags]);

  // Sibling databases, for the SWAP WITH target picker. ListUserDatabases
  // excludes shared / imported databases (not swappable).
  useEffect(() => {
    ListUserDatabases()
      .then((s) => setSiblings((s ?? []).filter((n) => n.toUpperCase() !== name.toUpperCase())))
      .catch(() => setSiblings([]));
  }, [name]);

  const dbRef = `"${db}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const paramVal = (key: string) => paramValue(params, key);

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterDatabase(db, "UNSET COMMENT");
    } else {
      await AlterDatabase(db, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  // SET/UNSET a non-negative-integer parameter. EditRow surfaces thrown errors inline.
  const saveIntParam = (param: string) => async (val: string) => {
    const v = val.trim();
    if (v === "") {
      await AlterDatabase(db, `UNSET ${param}`);
    } else {
      if (!/^\d+$/.test(v)) throw new Error("Must be a non-negative integer.");
      await AlterDatabase(db, `SET ${param} = ${v}`);
    }
    await reload();
  };

  // SET/UNSET a free-text string parameter (quoted as a string literal on SET).
  const saveTextParam = (param: string) => async (val: string) => {
    const v = val.trim();
    if (v === "") {
      await AlterDatabase(db, `UNSET ${param}`);
    } else {
      await AlterDatabase(db, `SET ${param} = ${q1(v)}`);
    }
    await reload();
  };

  const saveCollation = async (val: string) => {
    if (val.trim() === "") {
      await AlterDatabase(db, "UNSET DEFAULT_DDL_COLLATION");
    } else {
      await AlterDatabase(db, `SET DEFAULT_DDL_COLLATION = ${q1(val)}`);
    }
    await reload();
  };

  const saveRename = async (val: string) => {
    const newName = val.trim();
    if (newName === "" || newName === name) return;
    await AlterDatabase(db, `RENAME TO ${quoteIdent(newName)}`);
    // The modal's name/db props are now stale — close and let the sidebar refresh.
    onClose();
  };

  // Apply a fixed-choice SET/UNSET parameter change (value comes from a closed
  // option list, so the clause is safe to interpolate).
  const applyParam = (key: string) => async (clause: string) => {
    setBusyKey(key);
    setActionError(null);
    try {
      await AlterDatabase(db, clause);
      await reload();
    } catch (e) {
      setActionError(`${key} update failed: ${String(e)}`);
    } finally {
      setBusyKey(null);
    }
  };

  const setTag = async (tagName: string, tagValue: string) => {
    await AlterDatabase(db, `SET TAG ${tagName} = ${q1(tagValue)}`);
    await reloadTags();
  };

  const unsetTag = async (qualified: string) => {
    await AlterDatabase(db, `UNSET TAG ${qualified}`);
    await reloadTags();
  };

  // SWAP WITH exchanges ALL contents of two databases — destructive, so confirm
  // first. On success the modal's name context is stale, so close and refresh.
  const doSwap = () => {
    if (!swapTarget) return;
    Modal.confirm({
      title: "Swap database contents?",
      content: `This exchanges every object between "${name}" and "${swapTarget}". It is disruptive and can't be undone automatically.`,
      okText: "Swap",
      okButtonProps: { danger: true },
      onOk: async () => {
        setSwapBusy(true);
        setActionError(null);
        try {
          await AlterDatabase(db, `SWAP WITH ${quoteIdent(swapTarget)}`);
          onClose();
        } catch (e) {
          setActionError(`Swap failed: ${String(e)}`);
          setSwapBusy(false);
        }
      },
    });
  };

  // Account identifiers are org_name.account_name and are spliced unquoted into
  // the ALTER clause (the grammar's parseIdentPath accepts the dotted form), so
  // they MUST be validated — an unquoted free-text field is otherwise a SQL
  // injection / multi-statement vector (a stray ';' would smuggle a second
  // statement past Client.Execute's statement splitter). A Snowflake account
  // identifier is a dot-separated path of unquoted identifiers: letters, digits
  // and underscores only. Anything else disables the replication/failover actions.
  const accountTokens = accounts.split(",").map((s) => s.trim()).filter(Boolean);
  const accountsValid = accountTokens.every((a) => /^[A-Za-z0-9_]+(\.[A-Za-z0-9_]+)*$/.test(a));
  const accountList = () => accountTokens.join(", ");

  const runRepl = (key: string, clause: string) => async () => {
    setReplBusy(key);
    setActionError(null);
    try {
      await AlterDatabase(db, clause);
      await reload();
    } catch (e) {
      setActionError(`${key} failed: ${String(e)}`);
    } finally {
      setReplBusy(null);
    }
  };

  // PRIMARY / REFRESH are lifecycle operations on secondary databases — PRIMARY
  // (promote) is disruptive, so confirm it.
  const doPromote = () => {
    Modal.confirm({
      title: "Promote to primary?",
      content: `This makes "${name}" the primary database in its replication group. Other replicas become secondary.`,
      okText: "Promote",
      okButtonProps: { danger: true },
      onOk: runRepl("PRIMARY", "PRIMARY"),
    });
  };

  // A contact purpose is spliced unquoted into the clause, so it MUST be a bare
  // identifier (letters, digits, underscores) — otherwise it's an injection
  // vector. The contact name is a qualified identifier, double-quoted per part.
  const purposeValid = /^[A-Za-z0-9_]+$/.test(contactPurpose.trim());
  const runContact = (key: string, clause: string) => async () => {
    setContactBusy(key);
    setActionError(null);
    try {
      await AlterDatabase(db, clause);
      setContactName("");
    } catch (e) {
      setActionError(`Contact ${key} failed: ${String(e)}`);
    } finally {
      setContactBusy(null);
    }
  };

  const saveDqms = async () => {
    setDqmsBusy(true);
    setActionError(null);
    try {
      const v = dqms.trim();
      await AlterDatabase(db, v === ""
        ? "UNSET DATA_QUALITY_MONITORING_SETTINGS"
        : `SET DATA_QUALITY_MONITORING_SETTINGS = ${q1(dqms)}`);
    } catch (e) {
      setActionError(`Data quality monitoring settings update failed: ${String(e)}`);
    } finally {
      setDqmsBusy(false);
    }
  };

  const comment = find("comment");
  const owner = find("owner");
  const createdOn = find("created_on");
  const kind = find("kind");
  const origin = find("origin");
  // Prefer the SHOW dump; fall back to SHOW PARAMETERS.
  const retention = find("retention_time") || paramVal("DATA_RETENTION_TIME_IN_DAYS");
  const maxExtension = paramVal("MAX_DATA_EXTENSION_TIME_IN_DAYS");
  const collation = paramVal("DEFAULT_DDL_COLLATION");
  const logLevel = paramVal("LOG_LEVEL");
  const metricLevel = paramVal("METRIC_LEVEL");
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
  const eventTable = paramVal("EVENT_TABLE");
  const classificationProfile = paramVal("CLASSIFICATION_PROFILE");

  const catalogNames = () => ListIntegrations("CATALOG").then((rs) => (rs ?? []).map((r) => r.name));

  // Keys rendered in the dedicated sections, hidden from the generic Properties dump.
  const handledKeys = new Set([
    "comment", "owner", "created_on", "retention_time", "options", "name", "kind", "origin",
  ]);

  return (
    <>
    <Modal
      open
      title={
        <Space size={6}>
          <DatabaseOutlined style={{ color: "var(--icon-database)" }} />
          <span>Database Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {dbRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={[
        <Button key="params" onClick={() => setShowParams(true)} style={{ float: "left" }}>
          Parameters…
        </Button>,
        <Button key="close" onClick={onClose}>Close</Button>,
      ]}
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
              {kind && <InfoRow label="Kind" value={kind} />}
              {origin && <InfoRow label="Origin" value={origin} />}
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
                onSet={(v) => applyParam("DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE")(`SET DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE = ${quoteIdent(v)}`)}
                onUnset={() => applyParam("DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE")("UNSET DEFAULT_STREAMLIT_NOTEBOOK_WAREHOUSE")}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Governance</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <PickerRow
                label="Event table"
                value={eventTable}
                // Always offer the account's built-in default event table — it
                // lives in the shared SNOWFLAKE database and may not surface in
                // SHOW EVENT TABLES IN ACCOUNT, but is always a valid target.
                load={() => ListEventTables().then((ns) => ["SNOWFLAKE.TELEMETRY.EVENTS", ...(ns ?? [])])}
                busy={busyKey === "EVENT_TABLE"}
                onSet={(v) => applyParam("EVENT_TABLE")(`SET EVENT_TABLE = ${quoteQualified(v)}`)}
                onUnset={() => applyParam("EVENT_TABLE")("UNSET EVENT_TABLE")}
              />
              {/* CLASSIFICATION_PROFILE is a string value (parseString) — free-text
                  rather than a picker, so no classification-profile list IPC is needed. */}
              <EditRow
                label="Classification profile"
                value={classificationProfile}
                canUnset={classificationProfile !== ""}
                onSave={saveTextParam("CLASSIFICATION_PROFILE")}
                onUnset={() => saveTextParam("CLASSIFICATION_PROFILE")("")}
              />
              {/* CONTACT <purpose> = <contact_name>. Current contacts aren't read
                  back (needs SHOW CONTACTS); this is a set/unset-by-purpose editor. */}
              <tr>
                <td style={LABEL_TD}>Contact</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space direction="vertical" size={4} style={{ width: "100%" }}>
                    <Space wrap>
                      <Input
                        size="small"
                        value={contactPurpose}
                        onChange={(e) => setContactPurpose(e.target.value)}
                        placeholder="purpose (e.g. SUPPORT)"
                        style={{ width: 180 }}
                      />
                      <Input
                        size="small"
                        value={contactName}
                        onChange={(e) => setContactName(e.target.value)}
                        placeholder="contact name"
                        style={{ width: 180 }}
                      />
                      <Button
                        size="small"
                        type="primary"
                        disabled={!purposeValid || contactName.trim() === ""}
                        loading={contactBusy === "SET"}
                        onClick={runContact("SET",
                          `SET CONTACT ${contactPurpose.trim()} = ${quoteQualified(contactName.trim())}`)}
                      >
                        Set
                      </Button>
                      <Button
                        size="small"
                        disabled={!purposeValid}
                        loading={contactBusy === "UNSET"}
                        onClick={runContact("UNSET", `UNSET CONTACT ${contactPurpose.trim()}`)}
                      >
                        Unset
                      </Button>
                    </Space>
                    {contactPurpose.trim() !== "" && !purposeValid && (
                      <Text type="danger" style={{ fontSize: 11 }}>
                        Purpose must be a bare identifier (letters, digits, underscores).
                      </Text>
                    )}
                  </Space>
                </td>
              </tr>
              {/* DATA_QUALITY_MONITORING_SETTINGS is a YAML spec — multi-line editor. */}
              <tr>
                <td style={{ ...LABEL_TD, verticalAlign: "top", paddingTop: 10 }}>
                  Data quality monitoring
                </td>
                <td style={{ padding: "6px 0", fontSize: 12 }}>
                  <Space direction="vertical" size={4} style={{ width: "100%" }}>
                    <Input.TextArea
                      value={dqms}
                      onChange={(e) => setDqms(e.target.value)}
                      placeholder="YAML spec"
                      autoSize={{ minRows: 3, maxRows: 10 }}
                      style={{ width: 360, fontFamily: "monospace", fontSize: 12 }}
                    />
                    <Space>
                      <Button size="small" type="primary" loading={dqmsBusy} onClick={saveDqms}>
                        {dqms.trim() === "" ? "Unset" : "Set"}
                      </Button>
                    </Space>
                  </Space>
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>DCM project</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Button
                    size="small"
                    loading={busyKey === "DCM PROJECT"}
                    onClick={() => applyParam("DCM PROJECT")("UNSET DCM PROJECT")}
                  >
                    Unset
                  </Button>
                </td>
              </tr>
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Parameters</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              {/* LOG_LEVEL / METRIC_LEVEL / TRACE_LEVEL are settable but not
                  UNSET-able at the database level (grammar omits them), so no
                  Unset affordance — it would send SQL Snowflake rejects. */}
              <SelectRow
                label="Log level"
                value={logLevel}
                options={LOG_LEVELS}
                busy={busyKey === "LOG_LEVEL"}
                onSet={(v) => applyParam("LOG_LEVEL")(`SET LOG_LEVEL = ${q1(v)}`)}
              />
              <SelectRow
                label="Metric level"
                value={metricLevel}
                options={METRIC_LEVELS}
                busy={busyKey === "METRIC_LEVEL"}
                onSet={(v) => applyParam("METRIC_LEVEL")(`SET METRIC_LEVEL = ${q1(v)}`)}
              />
              <SelectRow
                label="Trace level"
                value={traceLevel}
                options={TRACE_LEVELS}
                busy={busyKey === "TRACE_LEVEL"}
                onSet={(v) => applyParam("TRACE_LEVEL")(`SET TRACE_LEVEL = ${q1(v)}`)}
              />
              <SelectRow
                label="Storage serialization policy"
                value={serialization}
                options={SERIALIZATION}
                busy={busyKey === "STORAGE_SERIALIZATION_POLICY"}
                onSet={(v) => applyParam("STORAGE_SERIALIZATION_POLICY")(`SET STORAGE_SERIALIZATION_POLICY = ${v}`)}
                onUnset={() => applyParam("STORAGE_SERIALIZATION_POLICY")("UNSET STORAGE_SERIALIZATION_POLICY")}
              />
              {/* REPLACE_INVALID_CHARACTERS is settable but not UNSET-able at the
                  database level (unlike ALTER SCHEMA) — no Unset affordance. */}
              <SelectRow
                label="Replace invalid characters"
                value={replaceInvalid}
                options={BOOLS}
                busy={busyKey === "REPLACE_INVALID_CHARACTERS"}
                onSet={(v) => applyParam("REPLACE_INVALID_CHARACTERS")(`SET REPLACE_INVALID_CHARACTERS = ${v}`)}
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

          <div style={SECTION_HEAD}>Replication &amp; Failover</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <tr>
                <td style={LABEL_TD}>Target accounts</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Input
                    size="small"
                    value={accounts}
                    onChange={(e) => setAccounts(e.target.value)}
                    placeholder="org1.acctA, org1.acctB"
                    style={{ width: 320 }}
                  />
                  <div style={{ marginTop: 4 }}>
                    <Checkbox checked={ignoreEdition} onChange={(e) => setIgnoreEdition(e.target.checked)}>
                      <span style={{ fontSize: 11 }}>Ignore edition check (replication)</span>
                    </Checkbox>
                  </div>
                  {accountTokens.length > 0 && !accountsValid && (
                    <Text type="danger" style={{ fontSize: 11, display: "block", marginTop: 4 }}>
                      Accounts must be org_name.account_name (letters, digits, underscores only).
                    </Text>
                  )}
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Replication</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space wrap>
                    <Button
                      size="small"
                      disabled={!accountTokens.length || !accountsValid}
                      loading={replBusy === "ENABLE REPLICATION"}
                      onClick={runRepl("ENABLE REPLICATION",
                        `ENABLE REPLICATION TO ACCOUNTS ${accountList()}${ignoreEdition ? " IGNORE EDITION CHECK" : ""}`)}
                    >
                      Enable…
                    </Button>
                    <Button
                      size="small"
                      disabled={accountTokens.length > 0 && !accountsValid}
                      loading={replBusy === "DISABLE REPLICATION"}
                      onClick={runRepl("DISABLE REPLICATION",
                        accountTokens.length ? `DISABLE REPLICATION TO ACCOUNTS ${accountList()}` : "DISABLE REPLICATION")}
                    >
                      Disable
                    </Button>
                  </Space>
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Failover</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space wrap>
                    <Button
                      size="small"
                      disabled={!accountTokens.length || !accountsValid}
                      loading={replBusy === "ENABLE FAILOVER"}
                      onClick={runRepl("ENABLE FAILOVER", `ENABLE FAILOVER TO ACCOUNTS ${accountList()}`)}
                    >
                      Enable…
                    </Button>
                    <Button
                      size="small"
                      disabled={accountTokens.length > 0 && !accountsValid}
                      loading={replBusy === "DISABLE FAILOVER"}
                      onClick={runRepl("DISABLE FAILOVER",
                        accountTokens.length ? `DISABLE FAILOVER TO ACCOUNTS ${accountList()}` : "DISABLE FAILOVER")}
                    >
                      Disable
                    </Button>
                  </Space>
                </td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Secondary database</td>
                <td style={{ padding: "6px 0", fontSize: 12, verticalAlign: "middle" }}>
                  <Space wrap>
                    <Tooltip title="Pull the latest changes from the primary">
                      <Button size="small" loading={replBusy === "REFRESH"} onClick={runRepl("REFRESH", "REFRESH")}>
                        Refresh
                      </Button>
                    </Tooltip>
                    <Button size="small" danger loading={replBusy === "PRIMARY"} onClick={doPromote}>
                      Promote to primary…
                    </Button>
                  </Space>
                </td>
              </tr>
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
                      placeholder="Target database"
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
    {showParams && (
      <ObjectParametersModal
        objectType="DATABASE"
        nameParts={[db]}
        title={dbRef}
        onClose={() => setShowParams(false)}
      />
    )}
    </>
  );
}
