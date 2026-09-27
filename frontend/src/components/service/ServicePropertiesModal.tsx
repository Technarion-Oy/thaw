// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  App as AntApp, Modal, Spin, Button, Input, InputNumber, Space, Typography, Alert, Tooltip, Tag, Select,
  AutoComplete, Radio, message,
} from "antd";
import {
  DeploymentUnitOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined, PlusOutlined,
  CloudUploadOutlined,
} from "@ant-design/icons";
import Editor from "@monaco-editor/react";
import {
  GetObjectProperties, AlterService, ListServiceEndpoints, GetServiceContainers, GetServiceLogs,
  ListServiceInstances, ListServiceVolumes, ListServiceRoles, ListServiceRoleGrants,
  GrantServiceRole, RevokeServiceRole, RedeployService, ListRoles, ListDatabaseRoles,
} from "../../../wailsjs/go/app/App";
import TagsRow from "../shared/TagsRow";
import LazyResultTable from "../shared/LazyResultTable";
import StageFilePicker from "../shared/StageFilePicker";
import { useObjectTags } from "../shared/useObjectTags";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { service as svcModels, type snowflake } from "../../../wailsjs/go/models";

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

// Read one named column (case-insensitive) from every row of a QueryResult.
function column(res: snowflake.QueryResult | null, name: string): string[] {
  const idx = (res?.columns ?? []).findIndex((c) => c.toLowerCase() === name);
  return idx < 0 ? [] : (res?.rows ?? []).map((r) => String(r[idx] ?? ""));
}

// ─── ServiceRoleGrants (per service role chip editor) ────────────────────────

const GRANTEE_KINDS = [
  { value: "ROLE", label: "Role" },
  { value: "DATABASE ROLE", label: "Database role" },
  { value: "APPLICATION ROLE", label: "Application role" },
];

/**
 * Grantees of one service role as removable chips (SHOW GRANTS OF SERVICE ROLE),
 * plus an add row — grantee kind + parent (database / application) + grantee —
 * modelled on UserPropertiesModal's PolicyManager. When the account can't list
 * grantees, the chips are replaced by a note and only the grant form remains.
 */
function ServiceRoleGrants({ db, schema, name, role, accountRoles }: {
  db: string; schema: string; name: string; role: string; accountRoles: string[];
}) {
  const { modal } = AntApp.useApp();
  const [grants, setGrants] = useState<svcModels.ServiceRoleGrant[] | null>(null);
  const [listErr, setListErr] = useState<string | null>(null);
  const [kind, setKind] = useState("ROLE");
  const [parent, setParent] = useState(""); // database / application; reset per kind
  const [grantee, setGrantee] = useState("");
  const [dbRoles, setDbRoles] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);

  const reload = useCallback(async () => {
    setListErr(null);
    try {
      setGrants(await ListServiceRoleGrants(db, schema, name, role) ?? []);
    } catch (e) {
      setGrants(null);
      setListErr(String(e));
    }
  }, [db, schema, name, role]);
  useEffect(() => { reload(); }, [reload]);

  // Debounced so typing a database name doesn't fire SHOW per keystroke; the
  // cleanup flag drops responses for a parent the user has since changed.
  useEffect(() => {
    setDbRoles([]);
    if (kind !== "DATABASE ROLE" || !parent.trim()) return;
    let stale = false;
    const t = setTimeout(() => {
      ListDatabaseRoles(parent.trim())
        .then((r) => { if (!stale) setDbRoles(r ?? []); })
        .catch(() => { if (!stale) setDbRoles([]); });
    }, 300);
    return () => { stale = true; clearTimeout(t); };
  }, [kind, parent]);

  const add = async () => {
    setBusy(true);
    try {
      await GrantServiceRole(db, schema, name, svcModels.ServiceRoleGrant.createFrom({
        role, granteeKind: kind, parent: kind === "ROLE" ? "" : parent.trim(), grantee: grantee.trim(),
      }));
      message.success(`Granted ${role} to ${grantee}`);
      setGrantee("");
      await reload();
    } catch (e) {
      message.error(String(e), 6);
    } finally {
      setBusy(false);
    }
  };

  const label = (g: svcModels.ServiceRoleGrant) =>
    `${g.granteeKind.toLowerCase()}: ${g.parent ? `${g.parent}.` : ""}${g.grantee}`;

  const remove = (g: svcModels.ServiceRoleGrant) => {
    modal.confirm({
      title: `Revoke ${role} from ${g.grantee}?`,
      okText: "Revoke",
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          await RevokeServiceRole(db, schema, name, g);
          message.success(`Revoked ${role} from ${g.grantee}`);
          await reload();
        } catch (e) {
          message.error(String(e), 6);
        }
      },
    });
  };

  const options = (kind === "ROLE" ? accountRoles : kind === "DATABASE ROLE" ? dbRoles : [])
    .map((r) => ({ value: r }));

  return (
    <tr>
      <td style={LABEL_TD}>{role}</td>
      <td style={{ padding: "6px 0" }}>
        <div style={{ display: "flex", flexWrap: "wrap", gap: 6, marginBottom: 6 }}>
          {listErr ? (
            <Text type="secondary" style={{ fontSize: 11, fontStyle: "italic" }}>
              Grantees unavailable — {listErr}. You can still grant below.
            </Text>
          ) : grants === null ? (
            <Spin size="small" />
          ) : grants.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>(not granted)</Text>
          ) : grants.map((g) => (
            <Tag key={label(g)} closable onClose={(e) => { e.preventDefault(); remove(g); }}>{label(g)}</Tag>
          ))}
        </div>
        <Space wrap size={6}>
          <Select size="small" value={kind} onChange={(v) => { setKind(v); setGrantee(""); setParent(v === "DATABASE ROLE" ? db : ""); }} options={GRANTEE_KINDS} style={{ width: 140 }} />
          {kind !== "ROLE" && (
            <Input
              size="small"
              value={parent}
              onChange={(e) => setParent(e.target.value)}
              placeholder={kind === "DATABASE ROLE" ? "database" : "application"}
              style={{ width: 130 }}
            />
          )}
          <AutoComplete
            size="small"
            value={grantee}
            onChange={setGrantee}
            options={options}
            filterOption={(input, opt) => String(opt?.value ?? "").toLowerCase().includes(input.toLowerCase())}
            placeholder="grantee"
            style={{ width: 170 }}
          />
          <Button size="small" type="primary" icon={<PlusOutlined />} loading={busy} disabled={!grantee.trim() || (kind !== "ROLE" && !parent.trim())} onClick={add}>
            Grant
          </Button>
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

export default function ServicePropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const { modal } = AntApp.useApp();
  const editorTheme = useThemeStore((s) => s.resolved) === "dark" ? "vs-dark" : "vs";

  // Specification editor (redeploy via ALTER SERVICE … FROM SPECIFICATION).
  const [specSource, setSpecSource] = useState<"inline" | "stage">("inline");
  const [specDraft, setSpecDraft] = useState<string | null>(null); // null = untouched
  const [specStage, setSpecStage] = useState("");
  const [specFile, setSpecFile] = useState("");
  const [redeploying, setRedeploying] = useState(false);

  // Lazily-loaded service roles (SHOW ROLES IN SERVICE).
  const [svcRoles, setSvcRoles] = useState<string[] | null>(null);
  const [accountRoles, setAccountRoles] = useState<string[]>([]);
  const [rolesLoading, setRolesLoading] = useState(false);
  const [rolesError, setRolesError] = useState<string | null>(null);

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

  const loadRoles = async () => {
    setRolesLoading(true);
    setRolesError(null);
    try {
      setSvcRoles(column(await ListServiceRoles(db, schema, name), "name"));
      ListRoles().then((r) => setAccountRoles(r ?? [])).catch(() => setAccountRoles([]));
    } catch (e) {
      setRolesError(String(e));
    } finally {
      setRolesLoading(false);
    }
  };

  const redeploy = () => {
    modal.confirm({
      title: `Redeploy ${name}?`,
      content: "Snowflake restarts the service instances with the new spec.",
      okText: "Redeploy",
      onOk: async () => {
        setRedeploying(true);
        try {
          await RedeployService(db, schema, name, svcModels.ServiceConfig.createFrom({
            specSource, specInline: specDraft ?? "", specStage, specFile,
          }));
          message.success("Service redeployed.");
          setSpecDraft(null);
          await reload();
        } catch (e) {
          message.error(`Redeploy failed: ${String(e)}`, 6);
        } finally {
          setRedeploying(false);
        }
      },
    });
  };

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

  const specText = specDraft ?? spec;
  // GetObjectProperties drops the spec when DESCRIBE SERVICE fails, so a
  // missing key means "couldn't read", not "empty" — warn before a redeploy
  // from a blank editor replaces a spec the user can't see.
  const specMissing = !rows?.some((r) => r.key.toLowerCase() === "spec");
  const canRedeploy = specSource === "inline"
    ? specDraft !== null && specDraft !== spec && specDraft.trim() !== ""
    : specStage !== "" && specFile !== "";

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
          <Space wrap style={{ marginBottom: 8 }}>
            <Radio.Group
              size="small"
              value={specSource}
              onChange={(e) => setSpecSource(e.target.value)}
              options={[{ value: "inline", label: "Inline YAML" }, { value: "stage", label: "Staged file" }]}
              optionType="button"
            />
            <Button
              size="small"
              type="primary"
              icon={<CloudUploadOutlined />}
              onClick={redeploy}
              loading={redeploying}
              disabled={!canRedeploy}
            >
              Redeploy
            </Button>
            {specSource === "inline" && specDraft !== null && specDraft !== spec && (
              <>
                <Text type="warning" style={{ fontSize: 11 }}>Unsaved changes</Text>
                <Button size="small" onClick={() => setSpecDraft(null)}>Discard</Button>
              </>
            )}
          </Space>
          {specMissing && (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 8 }}
              message="The current specification could not be read (DESCRIBE SERVICE failed or your role lacks privileges)."
              description="The editor below is empty, not the service's spec. Redeploying replaces the live specification with whatever you enter."
            />
          )}
          {specSource === "inline" ? (
            <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
              <Editor
                height={300}
                language="yaml"
                theme={editorTheme}
                value={specText}
                onChange={(v) => setSpecDraft(v ?? "")}
                onMount={(editor) => patchMonacoClipboard(editor)}
                options={{ minimap: { enabled: false }, scrollBeyondLastLine: false, fontSize: 12, wordWrap: "on", automaticLayout: true }}
              />
            </div>
          ) : (
            <>
              <StageFilePicker
                db={db}
                schema={schema}
                label="Browse internal stage — select the new specification file"
                onPick={(stage, file) => { setSpecStage(stage); setSpecFile(file); }}
              />
              {specFile && (
                <Text style={{ fontSize: 11, fontFamily: "var(--font-mono)" }}>@{specStage}/{specFile}</Text>
              )}
            </>
          )}
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 6 }}>
            Redeploy runs ALTER SERVICE … FROM SPECIFICATION; Snowflake restarts the service instances with the new spec.
          </Text>

          <LazyResultTable title="Endpoints" noun="endpoint" load={() => ListServiceEndpoints(db, schema, name)} />
          <LazyResultTable title="Instances" noun="instance" load={() => ListServiceInstances(db, schema, name)} />
          <LazyResultTable title="Containers" noun="container" load={() => GetServiceContainers(db, schema, name)} />
          <LazyResultTable title="Volumes" noun="volume" load={() => ListServiceVolumes(db, schema, name)} />

          <div style={SECTION_HEAD}>Service roles</div>
          {rolesError && (
            <Alert type="error" message="Failed to load service roles" description={rolesError} showIcon style={{ marginBottom: 8 }} />
          )}
          {svcRoles === null ? (
            <Button size="small" icon={<ReloadOutlined />} onClick={loadRoles} loading={rolesLoading}>Load service roles</Button>
          ) : svcRoles.length === 0 ? (
            <Text type="secondary" style={{ fontSize: 12 }}>No service roles declared in the spec.</Text>
          ) : (
            <table style={{ width: "100%", borderCollapse: "collapse" }}>
              <tbody>
                {svcRoles.map((r) => (
                  <ServiceRoleGrants key={r} db={db} schema={schema} name={name} role={r} accountRoles={accountRoles} />
                ))}
              </tbody>
            </table>
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
