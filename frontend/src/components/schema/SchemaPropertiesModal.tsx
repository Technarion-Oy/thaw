// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Select, Space, Typography, Alert, Tag,
} from "antd";
import { FolderOutlined } from "@ant-design/icons";
import {
  GetObjectProperties, AlterSchema, GetSchemaParameters,
  ListExternalVolumes, ListIntegrations, ListComputePools, ListWarehouses,
  ListUserSchemas,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";
import ObjectParametersModal from "../common/ObjectParametersModal";
import { quoteIdent } from "../shared/ObjectNameCaseControl";
import {
  SECTION_HEAD, LABEL_TD, q1, opts, paramValue,
  EditRow, InfoRow, SelectRow, PickerRow,
} from "../shared/PropertyRows";

const { Text } = Typography;

// Fixed-choice ALTER SCHEMA parameters, read from SHOW PARAMETERS IN SCHEMA.
const LOG_LEVELS = opts("TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF");
const TRACE_LEVELS = opts("ALWAYS", "ON_EVENT", "PROPAGATE", "OFF");
const SERIALIZATION = opts("COMPATIBLE", "OPTIMIZED");
const BOOLS = opts("TRUE", "FALSE");
const MERGE_BEHAVIOR = opts("AUTO", "ENABLED", "DISABLED");
const YES_NO = opts("YES", "NO");
const VISIBILITY = opts("PRIVILEGED");

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
  const [siblings, setSiblings] = useState<string[]>([]);
  const [swapTarget, setSwapTarget] = useState<string | undefined>();
  const [swapBusy, setSwapBusy] = useState(false);
  const [showParams, setShowParams] = useState(false);

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

  const objTags = useObjectTags({
    kind: "SCHEMA", db, schema, name,
    alter: (clause) => AlterSchema(db, schema, clause),
  });

  // Sibling schemas in the same database, for the SWAP WITH target picker.
  // ListUserSchemas excludes the read-only INFORMATION_SCHEMA (not swappable).
  useEffect(() => {
    ListUserSchemas(db)
      .then((s) => setSiblings((s ?? []).filter((n) => n.toUpperCase() !== name.toUpperCase())))
      .catch(() => setSiblings([]));
  }, [db, name]);

  const schemaRef = `"${db}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const paramVal = (key: string) => paramValue(params, key);

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
    <>
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
              <TagsRow tags={objTags.tags} nameOptions={objTags.nameOptions} onSetTag={objTags.setTag} onUnsetTag={objTags.unsetTag} />
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
    {showParams && (
      <ObjectParametersModal
        objectType="SCHEMA"
        nameParts={[db, schema]}
        title={schemaRef}
        onClose={() => setShowParams(false)}
      />
    )}
    </>
  );
}
