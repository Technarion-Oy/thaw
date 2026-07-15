// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect, useMemo } from "react";
import { Form, Input, InputNumber, Checkbox, Select, AutoComplete, Alert, Divider, Space } from "antd";
import { LineChartOutlined } from "@ant-design/icons";
import {
  BuildCreateModelMonitorSql, ExecDDL, ListWarehouses, ListObjects,
  ListModelVersions, GetTableColumnsWithTypes,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Object kinds that can serve as a monitor SOURCE or BASELINE (table-like).
const SOURCE_KINDS = new Set([
  "TABLE", "VIEW", "MATERIALIZED VIEW", "DYNAMIC TABLE",
  "EXTERNAL TABLE", "ICEBERG TABLE", "HYBRID TABLE", "EVENT TABLE",
]);

type RefreshUnit = "seconds" | "minutes" | "hours" | "days";

// Plain data shape for form state, mirroring modelmonitor.ModelMonitorConfig.
// The Wails-generated config class carries a `convertValues` method (it has
// nested string arrays) that a plain object literal can't satisfy; we cast to the
// generated type only at the IPC boundary (`cfg as any`).
interface MMConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  model: string;
  version: string;
  function: string;
  source: string;
  warehouse: string;
  refreshInterval: string;
  aggregationWindow: string;
  timestampColumn: string;
  baseline: string;
  idColumns: string[];
  predictionClassColumns: string[];
  predictionScoreColumns: string[];
  actualClassColumns: string[];
  actualScoreColumns: string[];
  segmentColumns: string[];
  customMetricColumns: string[];
}

// A tags-mode Select for the column-array parameters. When the source columns are
// known they are offered as a dropdown; the user can still type a name that isn't
// listed (e.g. for a cross-schema source) — tags mode keeps both behaviours.
function ColumnTags({
  value, onChange, placeholder, options, maxCount,
}: { value: string[]; onChange: (v: string[]) => void; placeholder: string; options: string[]; maxCount?: number }) {
  return (
    <Select
      mode="tags"
      size="small"
      style={{ width: "100%" }}
      value={value}
      // Cap at maxCount when set (e.g. SEGMENT_COLUMNS allows at most 5) so the
      // form can't build a statement Snowflake will reject.
      onChange={(v) => onChange(maxCount && v.length > maxCount ? v.slice(0, maxCount) : v)}
      tokenSeparators={[",", " "]}
      options={options.map((c) => ({ value: c, label: c }))}
      placeholder={placeholder}
    />
  );
}

export default function CreateModelMonitorModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<MMConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    model: "",
    version: "",
    function: "",
    source: "",
    warehouse: "",
    refreshInterval: "1 hour",
    aggregationWindow: "1 day",
    timestampColumn: "",
    baseline: "",
    idColumns: [],
    predictionClassColumns: [],
    predictionScoreColumns: [],
    actualClassColumns: [],
    actualScoreColumns: [],
    segmentColumns: [],
    customMetricColumns: [],
  });

  // Refresh-interval and aggregation-window composers. The composed strings live
  // in cfg.refreshInterval / cfg.aggregationWindow (which the builder consumes).
  const [refreshNum, setRefreshNum] = useState<number>(1);
  const [refreshUnit, setRefreshUnit] = useState<RefreshUnit>("hours");
  const [aggNum, setAggNum] = useState<number>(1);
  // Snowflake's minimum refresh interval is 60 seconds.
  const refreshMin = refreshUnit === "seconds" ? 60 : 1;

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateModelMonitorSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof MMConfig>(key: K, value: MMConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // ── Schema objects (models + table-like sources) ──────────────────────────
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);
  const [schemaObjects, setSchemaObjects] = useState<{ name: string; kind: string }[]>([]);
  const [loadingObjects, setLoadingObjects] = useState(false);

  useEffect(() => {
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  useEffect(() => {
    setLoadingObjects(true);
    ListObjects(db, schema)
      .then((objs) => setSchemaObjects((objs ?? []).map((o) => ({ name: o.name, kind: o.kind }))))
      .catch(() => {})
      .finally(() => setLoadingObjects(false));
  }, [db, schema]);

  const models = useMemo(
    () => schemaObjects.filter((o) => o.kind === "MODEL").map((o) => o.name).sort(),
    [schemaObjects],
  );
  const tablesViews = useMemo(
    () => schemaObjects.filter((o) => SOURCE_KINDS.has(o.kind)).map((o) => o.name).sort(),
    [schemaObjects],
  );

  // ── Versions of the selected model ────────────────────────────────────────
  const [versions, setVersions] = useState<string[]>([]);

  useEffect(() => {
    if (!cfg.model.trim()) { setVersions([]); return; }
    ListModelVersions(db, schema, cfg.model)
      .then((res) => {
        const cols = res?.columns ?? [];
        const idx = cols.findIndex((c) => c.toLowerCase() === "name");
        const rows = res?.rows ?? [];
        setVersions(idx >= 0 ? rows.map((r) => String(r[idx])).filter(Boolean) : []);
      })
      .catch(() => setVersions([]));
  }, [db, schema, cfg.model]);

  // ── Columns of the selected source table ──────────────────────────────────
  const [sourceCols, setSourceCols] = useState<{ name: string; dataType: string }[]>([]);

  useEffect(() => {
    if (!cfg.source.trim()) { setSourceCols([]); return; }
    GetTableColumnsWithTypes(db, schema, cfg.source)
      .then((cols) => setSourceCols((cols ?? []).map((c) => ({ name: c.name, dataType: c.dataType }))))
      .catch(() => setSourceCols([]));
  }, [db, schema, cfg.source]);

  // Every column feeds the prediction/actual/id/segment/custom tag pickers.
  const sourceColumns = useMemo(() => sourceCols.map((c) => c.name), [sourceCols]);
  // TIMESTAMP_COLUMN must be a TIMESTAMP_NTZ column (Snowflake requirement), so the
  // timestamp picker only suggests those — the AutoComplete stays free-typeable as
  // a fallback if the type can't be matched.
  const timestampColumns = useMemo(
    () => sourceCols.filter((c) => /^TIMESTAMP_NTZ\b/i.test(c.dataType.trim())).map((c) => c.name),
    [sourceCols],
  );

  // ── Composers → cfg ───────────────────────────────────────────────────────
  // Use the singular unit for a quantity of 1 ("1 hour", not "1 hours") so the
  // emitted duration is unambiguously grammatical.
  const durationLabel = (n: number, plural: string) =>
    `${n} ${n === 1 ? plural.replace(/s$/, "") : plural}`;

  useEffect(() => {
    if (refreshNum && refreshNum > 0) set("refreshInterval", durationLabel(refreshNum, refreshUnit));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [refreshNum, refreshUnit]);

  useEffect(() => {
    if (aggNum && aggNum > 0) set("aggregationWindow", durationLabel(aggNum, "days"));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [aggNum]);

  // At least one prediction column (score or class) is mandatory.
  const hasPrediction =
    cfg.predictionScoreColumns.length > 0 || cfg.predictionClassColumns.length > 0;
  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.model.trim().length > 0 &&
    cfg.version.trim().length > 0 &&
    cfg.function.trim().length > 0 &&
    cfg.source.trim().length > 0 &&
    cfg.warehouse.trim().length > 0 &&
    cfg.refreshInterval.trim().length > 0 &&
    cfg.aggregationWindow.trim().length > 0 &&
    cfg.timestampColumn.trim().length > 0 &&
    hasPrediction;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };
  const colsPlaceholder = cfg.source ? "Select or type columns" : "Select a source first";

  return (
    <CreateModalShell
      icon={<LineChartOutlined />}
      title="Create Model Monitor"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Model monitor creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="A model monitor tracks performance, prediction quality, and data drift for a registered model by aggregating a source table/view on a refresh schedule. At least one prediction column (score or class) is required."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Monitor name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_MONITOR"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={cfg.orReplace}
              onChange={(e) => setCfg((prev) => ({ ...prev, orReplace: e.target.checked, ifNotExists: e.target.checked ? false : prev.ifNotExists }))}
            >
              OR REPLACE
            </Checkbox>
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={cfg.ifNotExists}
              onChange={(e) => setCfg((prev) => ({ ...prev, ifNotExists: e.target.checked, orReplace: e.target.checked ? false : prev.orReplace }))}
            >
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Divider orientation="left" style={{ margin: "4px 0 12px", fontSize: 12 }}>Model</Divider>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 12px" }}>
          <Form.Item label="Model" required style={itemStyle} help="Model in this schema">
            <Select
              showSearch
              loading={loadingObjects}
              value={cfg.model || undefined}
              onChange={(v) => setCfg((prev) => ({ ...prev, model: v ?? "", version: "" }))}
              placeholder="Select model…"
              options={models.map((m) => ({ value: m, label: m }))}
              notFoundContent={loadingObjects ? "Loading…" : "No models in schema"}
            />
          </Form.Item>
          <Form.Item label="Version" required style={itemStyle}>
            {/* AutoComplete (not a plain Select) so a version can still be typed
                when SHOW VERSIONS returns nothing — keeps this required field
                satisfiable, consistent with the source/timestamp/column fields. */}
            <AutoComplete
              allowClear
              disabled={!cfg.model}
              value={cfg.version}
              onChange={(v) => set("version", v ?? "")}
              placeholder={cfg.model ? "Select or type version…" : "Select a model first"}
              options={versions.map((v) => ({ value: v }))}
              filterOption={(input, option) =>
                (option?.value ?? "").toLowerCase().includes(input.toLowerCase())
              }
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Function" required style={itemStyle} help="Model method, e.g. predict">
            <Input value={cfg.function} onChange={(e) => set("function", e.target.value)} placeholder="predict" />
          </Form.Item>
        </div>

        <Divider orientation="left" style={{ margin: "4px 0 12px", fontSize: 12 }}>Source & schedule</Divider>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 12px" }}>
          <Form.Item label="Source table / view" required style={itemStyle}>
            <Select
              showSearch
              allowClear
              loading={loadingObjects}
              value={cfg.source || undefined}
              onChange={(v) => setCfg((prev) => ({
                ...prev,
                source: v ?? "",
                // The timestamp + column-array selections belong to the previous
                // source's columns; clear them so a switched source can't submit
                // references to columns that don't exist in the new table.
                timestampColumn: "",
                idColumns: [],
                predictionClassColumns: [],
                predictionScoreColumns: [],
                actualClassColumns: [],
                actualScoreColumns: [],
                segmentColumns: [],
                customMetricColumns: [],
              }))}
              placeholder="Select source…"
              options={tablesViews.map((t) => ({ value: t, label: t }))}
              notFoundContent={loadingObjects ? "Loading…" : "No tables / views in schema"}
            />
          </Form.Item>
          <Form.Item label="Baseline table" style={itemStyle} help="Optional — used for drift computation">
            <Select
              showSearch
              allowClear
              loading={loadingObjects}
              value={cfg.baseline || undefined}
              onChange={(v) => set("baseline", v ?? "")}
              placeholder="Select baseline…"
              options={tablesViews.map((t) => ({ value: t, label: t }))}
              notFoundContent={loadingObjects ? "Loading…" : "No tables / views in schema"}
            />
          </Form.Item>
          <Form.Item label="Warehouse" required style={itemStyle}>
            <Select
              showSearch
              loading={loadingWarehouses}
              value={cfg.warehouse || undefined}
              onChange={(v) => set("warehouse", v ?? "")}
              placeholder="Select warehouse…"
              options={warehouses.map((n) => ({ value: n, label: n }))}
              notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
            />
          </Form.Item>
          <Form.Item label="Timestamp column" required style={itemStyle} help="Must be a TIMESTAMP_NTZ column in the source">
            {/* AutoComplete (not a plain Select) so a column name can still be
                typed when DESCRIBE returns nothing — keeps this required field
                satisfiable, consistent with the free-typeable column tags below.
                Only TIMESTAMP_NTZ columns are suggested (Snowflake requirement). */}
            <AutoComplete
              allowClear
              disabled={!cfg.source}
              value={cfg.timestampColumn}
              onChange={(v) => set("timestampColumn", v ?? "")}
              placeholder={cfg.source ? "Select or type TIMESTAMP_NTZ column…" : "Select a source first"}
              options={timestampColumns.map((c) => ({ value: c }))}
              filterOption={(input, option) =>
                (option?.value ?? "").toLowerCase().includes(input.toLowerCase())
              }
              style={{ width: "100%" }}
            />
          </Form.Item>
          {/* Snowflake enforces a 60-second minimum refresh interval, so when
              the unit is "seconds" the number is floored (and clamped on unit
              switch) at 60 rather than letting the value be rejected at run. */}
          <Form.Item label="Refresh interval" required style={itemStyle} help="Minimum 60 seconds">
            <Space.Compact block>
              <InputNumber
                min={refreshMin}
                value={refreshNum}
                onChange={(v) => setRefreshNum(Math.max(refreshMin, v ?? refreshMin))}
                style={{ width: "45%" }}
              />
              <Select
                value={refreshUnit}
                onChange={(v) => {
                  setRefreshUnit(v);
                  if (v === "seconds") setRefreshNum((n) => Math.max(60, n));
                }}
                style={{ width: "55%" }}
                options={[
                  { value: "seconds", label: "seconds" },
                  { value: "minutes", label: "minutes" },
                  { value: "hours", label: "hours" },
                  { value: "days", label: "days" },
                ]}
              />
            </Space.Compact>
          </Form.Item>
          <Form.Item label="Aggregation window" required style={itemStyle} help="Days only">
            <InputNumber
              min={1}
              value={aggNum}
              onChange={(v) => setAggNum(v ?? 1)}
              addonAfter="days"
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <Divider orientation="left" style={{ margin: "4px 0 12px", fontSize: 12 }}>Columns</Divider>

        <Form.Item label="Prediction score columns" style={itemStyle} required={!hasPrediction}
          help="At least one prediction column (score or class) is required">
          <ColumnTags value={cfg.predictionScoreColumns} onChange={(v) => set("predictionScoreColumns", v)} options={sourceColumns} placeholder={colsPlaceholder} />
        </Form.Item>
        <Form.Item label="Prediction class columns" style={itemStyle}>
          <ColumnTags value={cfg.predictionClassColumns} onChange={(v) => set("predictionClassColumns", v)} options={sourceColumns} placeholder={colsPlaceholder} />
        </Form.Item>
        <Form.Item label="Actual score columns" style={itemStyle}>
          <ColumnTags value={cfg.actualScoreColumns} onChange={(v) => set("actualScoreColumns", v)} options={sourceColumns} placeholder={colsPlaceholder} />
        </Form.Item>
        <Form.Item label="Actual class columns" style={itemStyle}>
          <ColumnTags value={cfg.actualClassColumns} onChange={(v) => set("actualClassColumns", v)} options={sourceColumns} placeholder={colsPlaceholder} />
        </Form.Item>
        <Form.Item label="ID columns" style={itemStyle} help="Columns that uniquely identify a row">
          <ColumnTags value={cfg.idColumns} onChange={(v) => set("idColumns", v)} options={sourceColumns} placeholder={colsPlaceholder} />
        </Form.Item>
        <Form.Item label="Segment columns" style={itemStyle} help="Up to 5 STRING columns for segmentation">
          <ColumnTags value={cfg.segmentColumns} onChange={(v) => set("segmentColumns", v)} options={sourceColumns} placeholder={colsPlaceholder} maxCount={5} />
        </Form.Item>
        <Form.Item label="Custom metric columns" style={itemStyle}>
          <ColumnTags value={cfg.customMetricColumns} onChange={(v) => set("customMetricColumns", v)} options={sourceColumns} placeholder={colsPlaceholder} />
        </Form.Item>

        <div style={{ height: 12 }} />
        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
