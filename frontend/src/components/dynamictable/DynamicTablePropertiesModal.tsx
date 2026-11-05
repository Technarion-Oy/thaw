// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Select, Space, Typography, Alert, Tag, Tooltip,
} from "antd";
import {
  RetweetOutlined, EditOutlined, CheckOutlined, CloseOutlined,
  PauseCircleOutlined, PlayCircleOutlined, SyncOutlined, HistoryOutlined,
} from "@ant-design/icons";
import { GetObjectProperties, GetObjectParameters, AlterDynamicTable, ListWarehouses } from "../../../wailsjs/go/app/App";
import TagsRow from "../shared/TagsRow";
import { SECTION_HEAD, LABEL_TD, q1, opts, EditRow, SelectRow, PickerRow, ActionRow } from "../shared/PropertyRows";
import GovernanceRows from "../common/GovernanceRows";
import { quoteIdent } from "../shared/ObjectNameCaseControl";
import DynamicTableRefreshHistoryModal from "./DynamicTableRefreshHistoryModal";
import * as dtc from "./dynamicTableClauses";
import { useObjectTags } from "../shared/useObjectTags";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// TARGET_LAG accepts either a string literal ('1 minute') or the bare keyword
// DOWNSTREAM. Render the clause accordingly.
function targetLagAssign(lag: string): string {
  const t = lag.trim();
  if (t.toUpperCase() === "DOWNSTREAM") return "SET TARGET_LAG = DOWNSTREAM";
  return `SET TARGET_LAG = ${q1(t)}`;
}

function days(v: string): string {
  if (!/^\d+$/.test(v.trim())) throw new Error("Enter a whole number of days");
  return v.trim();
}

// ─── SelectEditRow ───────────────────────────────────────────────────────────
// Like EditRow but edits via a Select sourced from a canonical option list, so
// the saved value is always a valid (correctly-cased) identifier rather than
// free-typed text.

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

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function DynamicTablePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);
  const [showHistory, setShowHistory] = useState(false);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);
  const [params, setParams] = useState<snowflake.SessionParam[]>([]);
  const [rowBusy, setRowBusy] = useState(false);

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
      const props = await GetObjectProperties(db, schema, "DYNAMIC TABLE", name);
      // SHOW PARAMETERS IN TABLE also covers dynamic tables; best-effort — the
      // rows below still edit fine (showing "not set") if it fails.
      const ps = await GetObjectParameters("TABLE", [db, schema, name]).catch(() => []);
      setParams(ps ?? []);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const objTags = useObjectTags({
    kind: "DYNAMIC TABLE", db, schema, name,
    alter: (clause) => AlterDynamicTable(db, schema, name, clause),
  });

  const tableRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const runAction = async (clause: string, label: string) => {
    setBusy(true);
    setActionError(null);
    try {
      await AlterDynamicTable(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
    } finally {
      setBusy(false);
    }
  };

  const param = (key: string) => params.find((p) => p.key.toUpperCase() === key)?.value ?? "";

  // Shared by every Settings / Governance row: run the clause, then reload.
  const apply = async (clause: string) => {
    await AlterDynamicTable(db, schema, name, clause);
    await reload();
  };
  // SelectRow / PickerRow handlers are fire-and-forget, so surface errors in the banner.
  const applyRow = (clause: string) => {
    setRowBusy(true);
    setActionError(null);
    apply(clause)
      .catch((e) => setActionError(String(e)))
      .finally(() => setRowBusy(false));
  };

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterDynamicTable(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterDynamicTable(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const saveTargetLag = async (lag: string) => {
    await AlterDynamicTable(db, schema, name, targetLagAssign(lag));
    await reload();
  };

  const saveWarehouse = async (wh: string) => {
    await AlterDynamicTable(db, schema, name, `SET WAREHOUSE = "${wh.replace(/"/g, '""')}"`);
    await reload();
  };

  const comment = find("comment");
  const targetLag = find("target_lag");
  const warehouse = find("warehouse");
  const definingQuery = find("text");
  const schedulingState = find("scheduling_state");
  const initWarehouse = find("initialization_warehouse");
  const scheduler = find("scheduler");
  // SHOW reports the key as LINEAR(a, b); edit just the expression list.
  const clusterKey = find("cluster_by").replace(/^LINEAR\((.*)\)$/is, "$1");
  const frozenWhere = find("immutable_where");
  const executeAs = find("execute_as_user");
  const rowTimestamp = find("row_timestamp");
  const retention = param("DATA_RETENTION_TIME_IN_DAYS");
  const maxExt = param("MAX_DATA_EXTENSION_TIME_IN_DAYS");
  const collation = param("DEFAULT_DDL_COLLATION");
  const logLevel = param("LOG_LEVEL");

  // Keys handled by the editable Settings section or rendered elsewhere.
  const handledKeys = new Set(["comment", "target_lag", "warehouse", "text", "initialization_warehouse",
    "scheduler", "cluster_by", "immutable_where", "execute_as_user", "row_timestamp"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <RetweetOutlined style={{ color: "var(--link)" }} />
          <span>Dynamic Table Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {tableRef}
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

          <Space wrap>
            {schedulingState && (
              <Tag color={/active|running/i.test(schedulingState) ? "green" : "orange"}>
                {schedulingState}
              </Tag>
            )}
            <Button size="small" icon={<SyncOutlined />} loading={busy} onClick={() => runAction("REFRESH", "Refresh")}>
              Refresh
            </Button>
            <Button size="small" icon={<PauseCircleOutlined />} loading={busy} onClick={() => runAction("SUSPEND", "Suspend")}>
              Suspend
            </Button>
            <Button size="small" icon={<PlayCircleOutlined />} loading={busy} onClick={() => runAction("RESUME", "Resume")}>
              Resume
            </Button>
            <Button size="small" icon={<HistoryOutlined />} onClick={() => setShowHistory(true)}>
              Refresh History
            </Button>
          </Space>

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
              <PickerRow
                label="Initialization Warehouse"
                value={initWarehouse}
                load={ListWarehouses}
                busy={rowBusy}
                onSet={(wh) => applyRow(`SET INITIALIZATION_WAREHOUSE = ${quoteIdent(wh)}`)}
                onUnset={() => applyRow("UNSET INITIALIZATION_WAREHOUSE")}
              />
              <SelectRow
                label="Scheduler"
                value={scheduler}
                options={opts("ENABLE", "DISABLE")}
                busy={rowBusy}
                onSet={(v) => applyRow(`SET SCHEDULER = ${v}`)}
              />
              <EditRow
                label="Cluster By"
                value={clusterKey}
                canUnset={clusterKey !== ""}
                onSave={(v) => apply(dtc.clusterBy(v))}
                onUnset={() => apply("DROP CLUSTERING KEY")}
              />
              <EditRow
                label="Data Retention (days)"
                value={retention}
                canUnset
                onSave={(v) => apply(`SET DATA_RETENTION_TIME_IN_DAYS = ${days(v)}`)}
                onUnset={() => apply("UNSET DATA_RETENTION_TIME_IN_DAYS")}
              />
              <EditRow
                label="Max Data Extension (days)"
                value={maxExt}
                canUnset
                onSave={(v) => apply(`SET MAX_DATA_EXTENSION_TIME_IN_DAYS = ${days(v)}`)}
                onUnset={() => apply("UNSET MAX_DATA_EXTENSION_TIME_IN_DAYS")}
              />
              <EditRow
                label="Default DDL Collation"
                value={collation}
                canUnset
                onSave={(v) => apply(v.trim() ? `SET DEFAULT_DDL_COLLATION = ${q1(v.trim())}` : "UNSET DEFAULT_DDL_COLLATION")}
                onUnset={() => apply("UNSET DEFAULT_DDL_COLLATION")}
              />
              <SelectRow
                label="Log Level"
                value={logLevel}
                options={opts("TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF")}
                busy={rowBusy}
                onSet={(v) => applyRow(`SET LOG_LEVEL = ${q1(v)}`)}
                onUnset={() => applyRow("UNSET LOG_LEVEL")}
              />
              <EditRow
                label="Frozen Where"
                value={frozenWhere}
                canUnset={frozenWhere !== ""}
                onSave={(v) => apply(v.trim() ? `SET FROZEN WHERE (${v.trim()})` : "UNSET FROZEN WHERE")}
                onUnset={() => apply("UNSET FROZEN WHERE")}
              />
              <EditRow
                label="Execute As User"
                value={executeAs}
                canUnset={executeAs !== ""}
                onSave={(v) => apply(v.trim() ? dtc.executeAsUser(v) : "UNSET EXECUTE AS USER")}
                onUnset={() => apply("UNSET EXECUTE AS USER")}
              />
              <SelectRow
                label="Row Timestamp"
                value={rowTimestamp}
                options={opts("TRUE", "FALSE")}
                busy={rowBusy}
                onSet={(v) => applyRow(`SET ROW_TIMESTAMP = ${v}`)}
                onUnset={() => applyRow("UNSET ROW_TIMESTAMP")}
              />
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
              <TagsRow tags={objTags.tags} nameOptions={objTags.nameOptions} onSetTag={objTags.setTag} onUnsetTag={objTags.unsetTag} />
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

          {/* Column-level masking / projection policies, tags and comments are on
              the column node's context menu. */}
          <GovernanceRows apply={apply} />

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

          {definingQuery && (
            <>
              <div style={SECTION_HEAD}>Defining Query</div>
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
                {definingQuery}
              </pre>
            </>
          )}
        </>
      )}
      {showHistory && (
        <DynamicTableRefreshHistoryModal db={db} schema={schema} name={name} onClose={() => setShowHistory(false)} />
      )}
    </Modal>
  );
}
