// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useCallback } from "react";
import {
  Modal, Spin, Button, Input, InputNumber, Select, Space, Typography, Alert, Tooltip, Table, Empty, Popconfirm,
} from "antd";
import {
  ApartmentOutlined, EditOutlined, CheckOutlined, CloseOutlined, ReloadOutlined,
  PauseCircleOutlined, PlayCircleOutlined, SyncOutlined, DeleteOutlined, PlusOutlined,
} from "@ant-design/icons";
import {
  GetObjectProperties, AlterSemanticView, ListWarehouses, ExecDDL,
  BuildAddSemanticViewMaterializationSql,
  DescribeSemanticView, ListSemanticDimensions, ListSemanticFacts, ListSemanticMetrics,
  ListSemanticDimensionsForMetric,
} from "../../../wailsjs/go/app/App";
import TagsRow from "../shared/TagsRow";
import { useObjectTags } from "../shared/useObjectTags";
import { identToken } from "../shared/ObjectNameCaseControl";
import {
  isMaterializationValid, qualifiedOptionsFromResult, NEW_MATERIALIZATION,
  type NewMaterialization, type RefreshMode,
} from "./semanticViewMaterialization";
import type { snowflake } from "../../../wailsjs/go/models";

// Snowflake's documented floor for MAX_STALENESS (seconds) — mirrors
// MinMaxStaleness in internal/semanticview/sql.go and MIN_MAX_STALENESS in
// CreateSemanticViewModal.tsx.
const MIN_MAX_STALENESS = 120;

// The four name-driven materialization actions — see matAction below.
type MatVerb = "SUSPEND" | "RESUME" | "REFRESH" | "DROP";

const { Text } = Typography;

// ─── Styles ──────────────────────────────────────────────────────────────────

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase",
  margin: "20px 0 8px",
};

const LABEL_TD: React.CSSProperties = {
  padding: "6px 12px 6px 0", color: "var(--text-muted)",
  fontSize: 12, whiteSpace: "nowrap", verticalAlign: "top",
  width: 160,
};

// ─── Helpers ─────────────────────────────────────────────────────────────────

// Escape a SQL text literal the way the backend's EscapeTextLit does — double
// backslashes (Snowflake interprets backslash escapes in string literals) then
// single quotes — so a comment like C:\temp round-trips intact.
function q1(s: string) { return "'" + s.replace(/\\/g, "\\\\").replace(/'/g, "''") + "'"; }

// Render a raw QueryResult (columns + rows) as an antd Table. Shared by the
// Dimensions / Facts / Metrics / Describe sections, all of which expose
// SHOW/DESCRIBE output verbatim.
function ResultTable({ res }: { res: snowflake.QueryResult }) {
  if (!res.rows || res.rows.length === 0) {
    return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="No rows" />;
  }
  return (
    <Table
      size="small"
      rowKey={(_r, i) => String(i)}
      pagination={res.rows.length > 10 ? { pageSize: 10 } : false}
      scroll={{ x: true }}
      columns={(res.columns ?? []).map((c, ci) => ({
        title: c,
        dataIndex: ci,
        key: String(ci),
        ellipsis: true,
        render: (v: unknown) => (v === null || v === undefined ? "" : String(v)),
      }))}
      dataSource={res.rows.map((row) => {
        const obj: Record<number, unknown> = {};
        row.forEach((cell, ci) => { obj[ci] = cell; });
        return obj;
      })}
    />
  );
}

// A lazily-loaded section: a Load/Refresh button that fetches a QueryResult on
// demand (SHOW SEMANTIC … can be slow and shouldn't run until the user asks).
function LazySection({
  title, description, loader,
}: {
  title: string;
  description?: string;
  loader: () => Promise<snowflake.QueryResult>;
}) {
  const [res, setRes] = useState<snowflake.QueryResult | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      setRes(await loader());
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <div style={SECTION_HEAD}>{title}</div>
      {description && (
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          {description}
        </Text>
      )}
      {error && (
        <Alert type="warning" message={`Could not load ${title.toLowerCase()}`} description={error} showIcon style={{ marginBottom: 8 }} />
      )}
      <Button size="small" icon={<ReloadOutlined />} onClick={load} loading={loading} style={{ marginBottom: 8 }}>
        {res ? "Refresh" : "Load"}
      </Button>
      {res && <ResultTable res={res} />}
    </>
  );
}

// ─── EditRow (single-line settings, e.g. comment) ────────────────────────────

interface EditRowProps {
  label: string;
  value: string;
  canUnset?: boolean;
  // Numeric fields (currently just MAX_STALENESS) render a bounded
  // InputNumber instead of a free-text Input; draft/onSave still carry a
  // string so the two modes share one save/unset/render implementation.
  numeric?: boolean;
  min?: number;
  max?: number;
  precision?: number;
  help?: string;
  emptyHint?: string;
  onSave: (val: string) => Promise<void>;
  onUnset?: () => Promise<void>;
}

function EditRow({
  label, value, canUnset, numeric, min, max, precision, help, emptyHint = "not set", onSave, onUnset,
}: EditRowProps) {
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
              {numeric ? (
                <InputNumber
                  size="small"
                  min={min}
                  max={max}
                  precision={precision}
                  value={draft === "" ? undefined : Number(draft)}
                  onChange={(v) => setDraft(v == null ? "" : String(v))}
                  style={{ width: 140 }}
                  onPressEnter={save}
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
                <Button size="small" icon={<CheckOutlined />} type="primary" onClick={save} loading={saving} disabled={numeric && draft === ""} />
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
            {help && <Text type="secondary" style={{ fontSize: 11 }}>{help}</Text>}
            {error && <Text type="danger" style={{ fontSize: 11 }}>{error}</Text>}
          </Space>
        ) : (
          <Space>
            <span style={{ color: "var(--text)" }}>{value || <Text type="secondary">({emptyHint})</Text>}</span>
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

export default function SemanticViewPropertiesModal({ db, schema, name, onClose }: Props) {
  const [rows, setRows] = useState<snowflake.PropertyPair[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  // Dimensions-for-metric lookup — which dimensions are queryable alongside a
  // given metric.
  const [metricName, setMetricName] = useState("");
  const [forMetric, setForMetric] = useState<snowflake.QueryResult | null>(null);
  const [forMetricError, setForMetricError] = useState<string | null>(null);
  const [forMetricLoading, setForMetricLoading] = useState(false);

  // Materializations — Snowflake exposes no SHOW/DESCRIBE for existing ones
  // (see semanticview README), so Suspend/Resume/Refresh/Drop act by
  // typed-in name rather than a picked row. matAction tracks which one of
  // those four is currently in flight (or null when idle), so clicking one
  // doesn't spin every button's loading state — the Add Materialization form
  // has its own separate addMatBusy so the two groups don't affect each other.
  const [maxStaleness, setMaxStaleness] = useState("");
  const [matName, setMatName] = useState("");
  const [matAction, setMatAction] = useState<MatVerb | null>(null);
  const [addMatBusy, setAddMatBusy] = useState(false);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);
  const [addingMat, setAddingMat] = useState(false);
  const [loadingMatOptions, setLoadingMatOptions] = useState(false);
  const [dimOptions, setDimOptions] = useState<string[]>([]);
  const [metricOptions, setMetricOptions] = useState<string[]>([]);
  const [newMat, setNewMat] = useState<NewMaterialization>(NEW_MATERIALIZATION);

  // Cache of the last SHOW SEMANTIC DIMENSIONS/METRICS result, shared between
  // the Dimensions/Metrics LazySection above and openAddMaterialization below
  // — expanding those sections first and then opening the Add form shouldn't
  // fire the same two live Snowflake round-trips again.
  const [dimsResult, setDimsResult] = useState<snowflake.QueryResult | null>(null);
  const [metricsResult, setMetricsResult] = useState<snowflake.QueryResult | null>(null);
  const loadDimensions = useCallback(async () => {
    const res = await ListSemanticDimensions(db, schema, name);
    setDimsResult(res);
    return res;
  }, [db, schema, name]);
  const loadMetrics = useCallback(async () => {
    const res = await ListSemanticMetrics(db, schema, name);
    setMetricsResult(res);
    return res;
  }, [db, schema, name]);

  const reload = useCallback(async () => {
    setRows(null);
    setError(null);
    try {
      const props = await GetObjectProperties(db, schema, "SEMANTIC VIEW", name);
      setRows(props ?? []);
    } catch (e) {
      setError(String(e));
    }
  }, [db, schema, name]);

  useEffect(() => { reload(); }, [reload]);

  const viewRef = `"${db}"."${schema}"."${name}"`;

  const find = (key: string) =>
    rows ? (rows.find((r) => r.key.toLowerCase() === key.toLowerCase())?.value ?? "") : "";

  const objTags = useObjectTags({
    kind: "SEMANTIC VIEW", db, schema, name,
    alter: (clause) => AlterSemanticView(db, schema, name, clause),
  });

  const saveComment = async (comment: string) => {
    if (comment.trim() === "") {
      await AlterSemanticView(db, schema, name, "UNSET COMMENT");
    } else {
      await AlterSemanticView(db, schema, name, `SET COMMENT = ${q1(comment)}`);
    }
    await reload();
  };

  const saveMaxStaleness = async (val: string) => {
    const n = Number(val);
    // InputNumber's min/max props aren't an absolute input-blocking
    // constraint in every interaction path (paste, Enter before blur-clamp),
    // so re-check here rather than relying on Snowflake's server-side
    // rejection for a constraint the CREATE flow already enforces
    // client-side (BuildCreateSemanticViewSql rejects the same floor in Go).
    // Number.isSafeInteger also catches a value too large to round-trip
    // through `${n}` as a valid integer literal — e.g. pasting
    // 99999999999999999999 serializes via JS exponential notation
    // ("1e+20"), which Snowflake rejects as a syntax error rather than the
    // clear client-side message this check gives instead.
    if (!Number.isSafeInteger(n) || n < MIN_MAX_STALENESS) {
      throw new Error(`MAX_STALENESS must be a whole number of at least ${MIN_MAX_STALENESS} seconds`);
    }
    await AlterSemanticView(db, schema, name, `SET MAX_STALENESS = ${n}`);
    setMaxStaleness(String(n));
  };

  const unsetMaxStaleness = async () => {
    await AlterSemanticView(db, schema, name, "UNSET MAX_STALENESS");
    setMaxStaleness("");
  };

  const runMatAction = async (verb: MatVerb, label: string) => {
    const trimmed = matName.trim();
    if (trimmed === "") return;
    setMatAction(verb);
    setActionError(null);
    try {
      // identToken(trimmed, false): only quote when Snowflake actually
      // requires it (special characters, reserved word), rather than forcing
      // a case-sensitive match — a materialization created unquoted (folded
      // to uppercase by Snowflake) still resolves when the user types it back
      // in its original lowercase form. Matches ViewPropertiesModal.tsx's
      // identToken(t, false) for the same kind of free-typed identifier.
      await AlterSemanticView(db, schema, name, `${verb} MATERIALIZATION ${identToken(trimmed, false)}`);
    } catch (e) {
      setActionError(`${label} materialization failed: ${String(e)}`);
    } finally {
      setMatAction(null);
    }
  };

  // Warehouses/dimensions/metrics are only needed for this form, so they're
  // fetched lazily here rather than unconditionally on modal mount.
  const openAddMaterialization = async () => {
    setAddingMat(true);
    setLoadingMatOptions(true);
    setLoadingWarehouses(true);
    setActionError(null);
    try {
      const [dims, metrics, whs] = await Promise.all([
        dimsResult ?? loadDimensions(),
        metricsResult ?? loadMetrics(),
        ListWarehouses(),
      ]);
      setDimOptions(qualifiedOptionsFromResult(dims));
      setMetricOptions(qualifiedOptionsFromResult(metrics));
      setWarehouses(whs ?? []);
    } catch (e) {
      // Surface the failure rather than swallowing it — an empty picker from
      // a real error (dropped session, no privilege) would otherwise look
      // identical to "this view genuinely has no dimensions/metrics/warehouses."
      // Close the form too: it was just opened (nothing typed into it yet),
      // so leaving it rendered would show dead "No … found" pickers instead
      // of a clear failure state; the user retries via "Add materialization…"
      // again, which re-runs this same fetch.
      setActionError(`Failed to load materialization form options: ${String(e)}`);
      setAddingMat(false);
    } finally {
      setLoadingMatOptions(false);
      setLoadingWarehouses(false);
    }
  };

  const submitAddMaterialization = async () => {
    setAddMatBusy(true);
    setActionError(null);
    try {
      const sql = await BuildAddSemanticViewMaterializationSql(db, schema, name, newMat);
      await ExecDDL(sql);
      setAddingMat(false);
      setNewMat(NEW_MATERIALIZATION);
    } catch (e) {
      setActionError(`Add materialization failed: ${String(e)}`);
    } finally {
      setAddMatBusy(false);
    }
  };

  const loadForMetric = async () => {
    if (metricName.trim() === "") return;
    setForMetricLoading(true);
    setForMetricError(null);
    try {
      setForMetric(await ListSemanticDimensionsForMetric(db, schema, name, metricName.trim()));
    } catch (e) {
      setForMetricError(String(e));
    } finally {
      setForMetricLoading(false);
    }
  };

  const comment = find("comment");
  const owner = find("owner");
  const createdOn = find("created_on");

  // Keys handled by dedicated sections above the generic Properties table.
  const handledKeys = new Set(["comment", "owner", "created_on"]);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ApartmentOutlined style={{ color: "var(--link)" }} />
          <span>Semantic View Properties</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {viewRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={<Button onClick={onClose}>Close</Button>}
      width={860}
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
              <tr>
                <td style={LABEL_TD}>Owner</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>{owner || <Text type="secondary">(unknown)</Text>}</td>
              </tr>
              <tr>
                <td style={LABEL_TD}>Created</td>
                <td style={{ padding: "6px 0", fontSize: 12, color: "var(--text)" }}>{createdOn || <Text type="secondary">(unknown)</Text>}</td>
              </tr>
              <EditRow
                label="Comment"
                value={comment}
                canUnset={comment !== ""}
                onSave={saveComment}
                onUnset={() => saveComment("")}
              />
              <EditRow
                label="Max Staleness (sec)"
                value={maxStaleness}
                canUnset
                numeric
                min={MIN_MAX_STALENESS}
                max={Number.MAX_SAFE_INTEGER}
                precision={0}
                help={`Seconds a materialization result may lag behind the source. Minimum ${MIN_MAX_STALENESS}. Must be set before adding a materialization, and can't be unset while one exists.`}
                emptyHint="unknown — Snowflake doesn't report this back"
                onSave={saveMaxStaleness}
                onUnset={unsetMaxStaleness}
              />
            </tbody>
          </table>

          <div style={SECTION_HEAD}>Tags</div>
          <table style={{ width: "100%", borderCollapse: "collapse" }}>
            <tbody>
              <TagsRow tags={objTags.tags} nameOptions={objTags.nameOptions} onSetTag={objTags.setTag} onUnsetTag={objTags.unsetTag} />
            </tbody>
          </table>

          <LazySection
            title="Structure"
            description="DESCRIBE SEMANTIC VIEW — one row per logical table, relationship, dimension, fact, or metric property."
            loader={() => DescribeSemanticView(db, schema, name)}
          />

          <LazySection
            title="Dimensions"
            description="SHOW SEMANTIC DIMENSIONS — the dimensions exposed by this view."
            loader={loadDimensions}
          />

          <LazySection
            title="Facts"
            description="SHOW SEMANTIC FACTS — the facts exposed by this view."
            loader={() => ListSemanticFacts(db, schema, name)}
          />

          <LazySection
            title="Metrics"
            description="SHOW SEMANTIC METRICS — the metrics exposed by this view."
            loader={loadMetrics}
          />

          <div style={SECTION_HEAD}>Materializations</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            Snowflake exposes no SHOW/DESCRIBE for existing materializations, so act by name.
            Requires MAX_STALENESS to be set above.
          </Text>
          <Space style={{ marginBottom: 12 }} wrap>
            <Input
              size="small"
              placeholder="materialization name"
              value={matName}
              onChange={(e) => setMatName(e.target.value)}
              style={{ width: 200 }}
            />
            <Button size="small" icon={<PauseCircleOutlined />} loading={matAction === "SUSPEND"} disabled={!matName.trim() || matAction !== null} onClick={() => runMatAction("SUSPEND", "Suspend")}>
              Suspend
            </Button>
            <Button size="small" icon={<PlayCircleOutlined />} loading={matAction === "RESUME"} disabled={!matName.trim() || matAction !== null} onClick={() => runMatAction("RESUME", "Resume")}>
              Resume
            </Button>
            <Button size="small" icon={<SyncOutlined />} loading={matAction === "REFRESH"} disabled={!matName.trim() || matAction !== null} onClick={() => runMatAction("REFRESH", "Refresh")}>
              Refresh
            </Button>
            <Popconfirm
              title={`Drop materialization "${matName.trim()}"?`}
              onConfirm={() => runMatAction("DROP", "Drop")}
              disabled={!matName.trim() || matAction !== null}
            >
              <Button size="small" danger icon={<DeleteOutlined />} loading={matAction === "DROP"} disabled={!matName.trim() || matAction !== null}>
                Drop
              </Button>
            </Popconfirm>
          </Space>

          {!addingMat ? (
            <Button size="small" icon={<PlusOutlined />} onClick={openAddMaterialization} style={{ marginBottom: 12 }}>
              Add materialization…
            </Button>
          ) : (
            <Space direction="vertical" size={8} style={{ width: "100%", marginBottom: 12 }}>
              <Space wrap>
                <Input
                  size="small"
                  placeholder="name"
                  value={newMat.name}
                  onChange={(e) => setNewMat({ ...newMat, name: e.target.value })}
                  style={{ width: 160 }}
                />
                <Select
                  size="small"
                  showSearch
                  loading={loadingWarehouses}
                  placeholder="Warehouse"
                  value={newMat.warehouse || undefined}
                  onChange={(v) => setNewMat({ ...newMat, warehouse: v ?? "" })}
                  style={{ width: 160 }}
                  options={warehouses.map((w) => ({ value: w, label: w }))}
                  notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
                />
                <Select<RefreshMode>
                  size="small"
                  value={newMat.refreshMode}
                  onChange={(v) => setNewMat({ ...newMat, refreshMode: v })}
                  style={{ width: 140 }}
                  options={[
                    { value: "AUTO", label: "REFRESH_MODE: AUTO" },
                    { value: "FULL", label: "REFRESH_MODE: FULL" },
                    { value: "INCREMENTAL", label: "REFRESH_MODE: INCREMENTAL" },
                  ]}
                />
              </Space>
              <Space wrap style={{ width: "100%" }}>
                <Select
                  mode="multiple"
                  size="small"
                  loading={loadingMatOptions}
                  placeholder="Dimensions (table.name)"
                  value={newMat.dimensions}
                  onChange={(v) => setNewMat({ ...newMat, dimensions: v })}
                  style={{ minWidth: 220 }}
                  options={dimOptions.map((d) => ({ value: d, label: d }))}
                  notFoundContent={loadingMatOptions ? "Loading…" : "No dimensions found"}
                />
                <Select
                  mode="multiple"
                  size="small"
                  loading={loadingMatOptions}
                  placeholder="Metrics (table.name)"
                  value={newMat.metrics}
                  onChange={(v) => setNewMat({ ...newMat, metrics: v })}
                  style={{ minWidth: 220 }}
                  options={metricOptions.map((m) => ({ value: m, label: m }))}
                  notFoundContent={loadingMatOptions ? "Loading…" : "No metrics found"}
                />
              </Space>
              <Input
                size="small"
                placeholder="IMMUTABLE WHERE condition (optional, raw SQL)"
                value={newMat.immutableWhere}
                onChange={(e) => setNewMat({ ...newMat, immutableWhere: e.target.value })}
              />
              <Input
                size="small"
                placeholder="WHERE filter (optional, raw SQL)"
                value={newMat.where}
                onChange={(e) => setNewMat({ ...newMat, where: e.target.value })}
              />
              <Space>
                <Button
                  size="small"
                  type="primary"
                  loading={addMatBusy}
                  disabled={!isMaterializationValid(newMat)}
                  onClick={submitAddMaterialization}
                >
                  Add
                </Button>
                <Button size="small" disabled={addMatBusy} onClick={() => { setAddingMat(false); setNewMat(NEW_MATERIALIZATION); }}>
                  Cancel
                </Button>
              </Space>
            </Space>
          )}

          <div style={SECTION_HEAD}>Dimensions for metric</div>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
            SHOW SEMANTIC DIMENSIONS … FOR METRIC — which dimensions can be queried alongside a specific metric.
          </Text>
          {forMetricError && (
            <Alert type="warning" message="Could not load dimensions for metric" description={forMetricError} showIcon style={{ marginBottom: 8 }} />
          )}
          <Space style={{ marginBottom: 8 }}>
            <Input
              size="small"
              placeholder="metric name"
              value={metricName}
              onChange={(e) => setMetricName(e.target.value)}
              style={{ width: 220 }}
              onPressEnter={loadForMetric}
            />
            <Button size="small" icon={<ReloadOutlined />} onClick={loadForMetric} loading={forMetricLoading} disabled={metricName.trim() === ""}>
              Show
            </Button>
          </Space>
          {forMetric && <ResultTable res={forMetric} />}

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
