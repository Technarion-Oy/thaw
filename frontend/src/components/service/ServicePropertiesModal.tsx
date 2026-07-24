// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, InputNumber, Space, Typography, Alert, Tooltip, Table, Tag, Select,
} from "antd";
import {
  DeploymentUnitOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterService, ListServiceEndpoints, GetServiceContainers, GetServiceLogs,
} from "../../../wailsjs/go/app/App";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Single-quote-escape a SQL string literal (doubles embedded single quotes).
const q1 = (s: string) => `'${s.replace(/'/g, "''")}'`;
// Double-quote a SQL identifier.
const qid = (s: string) => `"${s.replace(/"/g, '""')}"`;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "middle",
  width: 200,
};

// ─── EditRow (single-line settings) ──────────────────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  numeric?: boolean;
  options?: string[]; // when set, render a Select instead of a free input
  canUnset?: boolean;
  onSave: (val: string) => Promise<void>;
  onUnset?: () => Promise<void>;
}

function EditRow({ label, value, numeric, options, canUnset, onSave, onUnset }: EditRowProps) {
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
              {options ? (
                <Select
                  size="small"
                  value={draft || undefined}
                  onChange={(v) => setDraft(v ?? "")}
                  options={options.map((o) => ({ value: o, label: o }))}
                  style={{ width: 200 }}
                  placeholder="(default)"
                  allowClear
                />
              ) : numeric ? (
                <InputNumber
                  size="small"
                  min={1}
                  value={draft ? Number(draft) : undefined}
                  onChange={(v) => setDraft(v == null ? "" : String(v))}
                  style={{ width: 200 }}
                />
              ) : (
                <Input
                  size="small"
                  value={draft}
                  onChange={(e) => setDraft(e.target.value)}
                  style={{ width: 280 }}
                  onPressEnter={save}
                />
              )}
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

// Build antd Table columns/data from a raw QueryResult shape.
function tableFromResult(res: snowflake.QueryResult | null) {
  const columns = (res?.columns ?? []).map((col, idx) => ({
    title: col,
    dataIndex: String(idx),
    key: String(idx),
    ellipsis: true,
    render: (v: unknown) => (
      <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>{v == null ? "" : String(v)}</span>
    ),
  }));
  const data = (res?.rows ?? []).map((row, ri) => {
    const obj: Record<string, unknown> = { key: ri };
    row.forEach((cell, ci) => { obj[String(ci)] = cell; });
    return obj;
  });
  return { columns, data };
}

// ─── Main component ──────────────────────────────────────────────────────────

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function ServicePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Lazily-loaded endpoints (SHOW ENDPOINTS IN SERVICE).
  const [endpoints, setEndpoints] = useState<snowflake.QueryResult | null>(null);
  const [endpointsLoading, setEndpointsLoading] = useState(false);
  const [endpointsError, setEndpointsError] = useState<string | null>(null);

  // Lazily-loaded container status (SHOW SERVICE CONTAINERS IN SERVICE).
  const [containers, setContainers] = useState<snowflake.QueryResult | null>(null);
  const [containersLoading, setContainersLoading] = useState(false);
  const [containersError, setContainersError] = useState<string | null>(null);

  // Lazily-loaded logs (SYSTEM$GET_SERVICE_LOGS).
  const [logContainer, setLogContainer] = useState("");
  const [logInstance, setLogInstance] = useState(0);
  const [logLines, setLogLines] = useState(100);
  const [logs, setLogs] = useState<string | null>(null);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logsError, setLogsError] = useState<string | null>(null);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "SERVICE", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const objTags = useObjectTags({
    kind: "SERVICE", db, schema, name,
    alter: (clause) => AlterService(db, schema, name, clause),
  });

  const svcRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  // Run a SET/UNSET clause then refresh, surfacing failures inline.
  const runAlter = async (clause: string, label: string) => {
    setActionError(null);
    try {
      await AlterService(db, schema, name, clause);
      await reload();
    } catch (e) {
      setActionError(`${label} failed: ${String(e)}`);
      throw e;
    }
  };

  const saveComment = (v: string) =>
    runAlter(v.trim() === "" ? "UNSET COMMENT" : `SET COMMENT = ${q1(v)}`, "Update comment");
  const saveMinInstances = (v: string) =>
    runAlter(v.trim() === "" ? "UNSET MIN_INSTANCES" : `SET MIN_INSTANCES = ${Number(v)}`, "Update min instances");
  const saveMaxInstances = (v: string) =>
    runAlter(v.trim() === "" ? "UNSET MAX_INSTANCES" : `SET MAX_INSTANCES = ${Number(v)}`, "Update max instances");
  const saveAutoResume = (v: string) =>
    runAlter(v.trim() === "" ? "UNSET AUTO_RESUME" : `SET AUTO_RESUME = ${v.toUpperCase()}`, "Update auto resume");
  const saveQueryWarehouse = (v: string) =>
    runAlter(v.trim() === "" ? "UNSET QUERY_WAREHOUSE" : `SET QUERY_WAREHOUSE = ${qid(v.trim())}`, "Update query warehouse");

  const loadEndpoints = useCallback(async () => {
    setEndpointsLoading(true);
    setEndpointsError(null);
    try {
      setEndpoints(await ListServiceEndpoints(db, schema, name) ?? null);
    } catch (e) {
      setEndpointsError(String(e));
    } finally {
      setEndpointsLoading(false);
    }
  }, [db, schema, name]);

  const loadContainers = useCallback(async () => {
    setContainersLoading(true);
    setContainersError(null);
    try {
      setContainers(await GetServiceContainers(db, schema, name) ?? null);
    } catch (e) {
      setContainersError(String(e));
    } finally {
      setContainersLoading(false);
    }
  }, [db, schema, name]);

  const loadLogs = useCallback(async () => {
    setLogsLoading(true);
    setLogsError(null);
    try {
      const text = await GetServiceLogs(db, schema, name, logContainer.trim(), logInstance, logLines);
      setLogs(text ?? "");
    } catch (e) {
      setLogsError(String(e));
    } finally {
      setLogsLoading(false);
    }
  }, [db, schema, name, logContainer, logInstance, logLines]);

  const status = find("status");
  const spec = find("spec");
  const comment = find("comment");
  const minInstances = find("min_instances");
  const maxInstances = find("max_instances");
  const autoResume = find("auto_resume");
  const queryWarehouse = find("query_warehouse");

  // Keys handled by dedicated sections above the generic Properties table.
  const handledKeys = new Set([
    "status", "spec", "comment", "min_instances", "max_instances", "auto_resume", "query_warehouse",
  ]);

  const ep = tableFromResult(endpoints);
  const ct = tableFromResult(containers);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <DeploymentUnitOutlined style={{ color: "var(--link)" }} />
          <span>Service Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {svcRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={860}
      styles={{ body: { maxHeight: "76vh", overflowY: "auto", paddingTop: 16 } }}
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

          <div style={SECTION_HEAD}>Status</div>
          {status ? (
            <Tag color={/running|ready|active/i.test(status) ? "green" : /suspend|pending|fail|error/i.test(status) ? "orange" : "default"}>
              {status}
            </Tag>
          ) : (
            <Text type="secondary">(unknown)</Text>
          )}

          <div style={SECTION_HEAD}>Settings</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <EditRow label="Comment" value={comment} canUnset={comment !== ""} onSave={saveComment} onUnset={() => saveComment("")} />
              <EditRow label="Min instances" value={minInstances} numeric canUnset={minInstances !== ""} onSave={saveMinInstances} onUnset={() => saveMinInstances("")} />
              <EditRow label="Max instances" value={maxInstances} numeric canUnset={maxInstances !== ""} onSave={saveMaxInstances} onUnset={() => saveMaxInstances("")} />
              <EditRow label="Auto resume" value={autoResume} options={["TRUE", "FALSE"]} canUnset={autoResume !== ""} onSave={saveAutoResume} onUnset={() => saveAutoResume("")} />
              <EditRow label="Query warehouse" value={queryWarehouse} canUnset={queryWarehouse !== ""} onSave={saveQueryWarehouse} onUnset={() => saveQueryWarehouse("")} />
              <TagsRow tags={objTags.tags} nameOptions={objTags.nameOptions} onSetTag={objTags.setTag} onUnsetTag={objTags.unsetTag} />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Specification</div>
          {spec ? (
            <Input.TextArea
              value={spec}
              readOnly
              autoSize={{ minRows: 4, maxRows: 16 }}
              style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}
            />
          ) : (
            <Text type="secondary">(unavailable)</Text>
          )}

          <div style={SECTION_HEAD}>Endpoints</div>
          {endpointsError && (
            <Alert type="error" message="Failed to load endpoints" description={endpointsError} showIcon style={{ marginBottom: 8 }} />
          )}
          {endpoints ? (
            <>
              <Space style={{ marginBottom: 8 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {ep.data.length === 0 ? "No endpoints." : `${ep.data.length} endpoint${ep.data.length === 1 ? "" : "s"}.`}
                </Text>
                <Button size="small" icon={<ReloadOutlined />} onClick={loadEndpoints} loading={endpointsLoading}>Refresh</Button>
              </Space>
              {ep.data.length > 0 && (
                <Table size="small" columns={ep.columns} dataSource={ep.data} pagination={false} scroll={{ x: true }} />
              )}
            </>
          ) : (
            <Button size="small" icon={<ReloadOutlined />} onClick={loadEndpoints} loading={endpointsLoading}>Load endpoints</Button>
          )}

          <div style={SECTION_HEAD}>Containers</div>
          {containersError && (
            <Alert type="error" message="Failed to load containers" description={containersError} showIcon style={{ marginBottom: 8 }} />
          )}
          {containers ? (
            <>
              <Space style={{ marginBottom: 8 }}>
                <Text type="secondary" style={{ fontSize: 11 }}>
                  {ct.data.length === 0 ? "No containers." : `${ct.data.length} container${ct.data.length === 1 ? "" : "s"}.`}
                </Text>
                <Button size="small" icon={<ReloadOutlined />} onClick={loadContainers} loading={containersLoading}>Refresh</Button>
              </Space>
              {ct.data.length > 0 && (
                <Table size="small" columns={ct.columns} dataSource={ct.data} pagination={false} scroll={{ x: true }} />
              )}
            </>
          ) : (
            <Button size="small" icon={<ReloadOutlined />} onClick={loadContainers} loading={containersLoading}>Load containers</Button>
          )}

          <div style={SECTION_HEAD}>Logs</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Fetch container logs via SYSTEM$GET_SERVICE_LOGS. The container name comes from the service spec.
          </Text>
          {logsError && (
            <Alert type="error" message="Failed to load logs" description={logsError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Space wrap style={{ marginBottom: 8 }}>
            <Input
              size="small"
              addonBefore="Container"
              value={logContainer}
              onChange={(e) => setLogContainer(e.target.value)}
              placeholder="main"
              style={{ width: 220 }}
            />
            <Space size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>Instance</Text>
              <InputNumber size="small" min={0} value={logInstance} onChange={(v) => setLogInstance(Number(v ?? 0))} style={{ width: 70 }} />
            </Space>
            <Space size={4}>
              <Text type="secondary" style={{ fontSize: 12 }}>Lines</Text>
              <InputNumber size="small" min={1} value={logLines} onChange={(v) => setLogLines(Number(v ?? 100))} style={{ width: 90 }} />
            </Space>
            <Button
              size="small"
              icon={<ReloadOutlined />}
              onClick={loadLogs}
              loading={logsLoading}
              disabled={logContainer.trim() === ""}
            >
              Fetch logs
            </Button>
          </Space>
          {logs !== null && (
            <Input.TextArea
              value={logs || "(no log output)"}
              readOnly
              autoSize={{ minRows: 4, maxRows: 20 }}
              style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}
            />
          )}

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
