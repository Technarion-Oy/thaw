// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import { Form, Input, Select, Space, InputNumber, Radio, Switch, Typography } from "antd";
import { FileSearchOutlined } from "@ant-design/icons";
import {
  BuildCreateCortexSearchServiceSql, ExecDDL, ListWarehouses,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import MonacoSqlField from "../shared/MonacoSqlField";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { cortexsearchservice } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_QUERY = "SELECT id, body, category\n  FROM my_source_table";

type LagUnit = "seconds" | "minutes" | "hours" | "days";

// Embedding models Snowflake offers for Cortex Search (availability varies by
// region). The field is a combobox, so a model not listed here can still be
// typed in.
const EMBEDDING_MODELS = [
  "snowflake-arctic-embed-m-v1.5",
  "snowflake-arctic-embed-l-v2.0",
  "snowflake-arctic-embed-m",
  "voyage-multilingual-2",
  "nv-embed-qa-4",
  "e5-base-v2",
];

// Plain data shape for form state. The Wails-generated `CortexSearchServiceConfig`
// class carries a `convertValues` method (it has a nested `attributes` array),
// which a plain object literal can't satisfy; we cast to the generated type only
// at the IPC boundary (`cfg as any`).
type CSSConfig = Omit<cortexsearchservice.CortexSearchServiceConfig, "convertValues">;

export default function CreateCortexSearchServiceModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<CSSConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    indexMode: "single",
    searchColumn: "",
    textIndexes: [],
    vectorIndexes: [],
    primaryKey: [],
    attributes: [],
    warehouse: "",
    targetLag: "1 hour",
    embeddingModel: "",
    refreshMode: "",
    initialize: "",
    fullIndexBuildIntervalDays: 0,
    requestLogging: false,
    autoSuspend: 0,
    comment: "",
    query: DEFAULT_QUERY,
  });

  const multi = cfg.indexMode === "multi";

  // Target-lag composer state. The composed string lives in cfg.targetLag.
  const [lagNum, setLagNum] = useState<number>(1);
  const [lagUnit, setLagUnit] = useState<LagUnit>("hours");

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateCortexSearchServiceSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  useEffect(() => {
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  const set = <K extends keyof CSSConfig>(key: K, value: CSSConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Recompose the TARGET_LAG string whenever the composer inputs change. Use the
  // singular unit for a value of 1 (e.g. "1 hour", not "1 hours") — Snowflake
  // accepts both, but the singular reads correctly.
  useEffect(() => {
    const unit = lagNum === 1 ? lagUnit.replace(/s$/, "") : lagUnit;
    set("targetLag", `${lagNum} ${unit}`);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lagNum, lagUnit]);

  const canSubmit =
    cfg.name.trim().length > 0 &&
    (multi
      ? cfg.vectorIndexes.filter((v) => v.trim().length > 0).length > 0
      : cfg.searchColumn.trim().length > 0) &&
    cfg.warehouse.trim().length > 0 &&
    cfg.targetLag.trim().length > 0 &&
    cfg.query.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const warehouseOptions = (warehouses || []).map((n) => ({ value: n, label: n }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<FileSearchOutlined />}
      title="Create Cortex Search Service"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Cortex search service creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Service name"
          placeholder="MY_SEARCH_SERVICE"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
        />

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Index mode" style={itemStyle} help={multi ? "Multi-index: combine keyword (TEXT) and vector indexes. IF NOT EXISTS / EMBEDDING_MODEL are not available in this form." : "Single-index: index one text column."}>
          <Radio.Group
            value={cfg.indexMode}
            onChange={(e) => {
              const m = e.target.value as string;
              // The multi-index form does not support IF NOT EXISTS.
              setCfg((prev) => ({ ...prev, indexMode: m, ifNotExists: m === "multi" ? false : prev.ifNotExists }));
            }}
            optionType="button"
            buttonStyle="solid"
            options={[
              { value: "single", label: "Single column (ON)" },
              { value: "multi", label: "Multi-index (TEXT / VECTOR)" },
            ]}
          />
        </Form.Item>

        {multi ? (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
            <Form.Item label="Vector indexes" required style={itemStyle} help="Each: a vector column, or a text column with managed embeddings e.g. BODY (model='snowflake-arctic-embed-m')">
              <Select
                mode="tags"
                value={cfg.vectorIndexes}
                onChange={(v) => set("vectorIndexes", v)}
                placeholder="BODY (model='snowflake-arctic-embed-m')"
                tokenSeparators={[]}
                open={false}
                suffixIcon={null}
                style={{ width: "100%" }}
              />
            </Form.Item>
            <Form.Item label="Text indexes" style={itemStyle} help="Optional text columns indexed for keyword search">
              <Select
                mode="tags"
                value={cfg.textIndexes}
                onChange={(v) => set("textIndexes", v)}
                placeholder="TITLE, BODY"
                tokenSeparators={[","]}
                open={false}
                suffixIcon={null}
                style={{ width: "100%" }}
              />
            </Form.Item>
          </div>
        ) : (
          <Form.Item label="Search column (ON)" required style={itemStyle} help="The text column to index for search">
            <Input
              value={cfg.searchColumn}
              onChange={(e) => set("searchColumn", e.target.value)}
              placeholder="BODY"
            />
          </Form.Item>
        )}

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Primary key" style={itemStyle} help="Optional key columns (PRIMARY KEY)">
            <Select
              mode="tags"
              value={cfg.primaryKey}
              onChange={(v) => set("primaryKey", v)}
              placeholder="ID"
              tokenSeparators={[","]}
              open={false}
              suffixIcon={null}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Attributes" style={itemStyle} help="Optional columns exposed for filtering">
            <Select
              mode="tags"
              value={cfg.attributes}
              onChange={(v) => set("attributes", v)}
              placeholder="CATEGORY, AUTHOR"
              tokenSeparators={[","]}
              open={false}
              suffixIcon={null}
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Target Lag" required style={itemStyle} help="Maximum staleness before the index refreshes">
            <Space>
              <InputNumber
                min={1}
                value={lagNum}
                onChange={(v) => setLagNum(v == null ? 1 : Math.max(1, v))}
                style={{ width: 90 }}
              />
              <Select
                value={lagUnit}
                onChange={(u) => setLagUnit(u)}
                style={{ width: 120 }}
                options={[
                  { value: "seconds", label: "seconds" },
                  { value: "minutes", label: "minutes" },
                  { value: "hours", label: "hours" },
                  { value: "days", label: "days" },
                ]}
              />
            </Space>
          </Form.Item>
          <Form.Item label="Warehouse" required style={itemStyle} help="Warehouse used to build and refresh the index">
            <Select
              showSearch
              loading={loadingWarehouses}
              value={cfg.warehouse || undefined}
              onChange={(v) => set("warehouse", v ?? "")}
              placeholder="Select warehouse…"
              options={warehouseOptions}
              style={{ width: "100%" }}
              notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
            />
          </Form.Item>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          {!multi && (
            <Form.Item label="Embedding Model" style={itemStyle} help="Optional. Cannot be changed after creation.">
              <Select
                allowClear
                showSearch
                value={cfg.embeddingModel || undefined}
                onChange={(v) => set("embeddingModel", v ?? "")}
                placeholder="default (Snowflake-selected model)"
                options={EMBEDDING_MODELS.map((m) => ({ value: m, label: m }))}
                style={{ width: "100%" }}
              />
            </Form.Item>
          )}
          <Form.Item label="Comment" style={itemStyle}>
            <Input
              value={cfg.comment}
              onChange={(e) => set("comment", e.target.value)}
              placeholder="optional comment"
            />
          </Form.Item>
        </div>

        <Typography.Text type="secondary" style={{ fontSize: 11, fontWeight: 600, letterSpacing: "0.05em", textTransform: "uppercase" }}>
          Advanced
        </Typography.Text>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px", marginTop: 8 }}>
          <Form.Item label="Refresh mode" style={itemStyle} help="REFRESH_MODE (default INCREMENTAL)">
            <Select
              allowClear
              value={cfg.refreshMode || undefined}
              onChange={(v) => set("refreshMode", v ?? "")}
              placeholder="default"
              options={[
                { value: "INCREMENTAL", label: "INCREMENTAL" },
                { value: "FULL", label: "FULL" },
              ]}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Initialize" style={itemStyle} help="INITIALIZE (default ON_CREATE)">
            <Select
              allowClear
              value={cfg.initialize || undefined}
              onChange={(v) => set("initialize", v ?? "")}
              placeholder="default"
              options={[
                { value: "ON_CREATE", label: "ON_CREATE" },
                { value: "ON_SCHEDULE", label: "ON_SCHEDULE" },
              ]}
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Full index rebuild (days)" style={itemStyle} help="FULL_INDEX_BUILD_INTERVAL_DAYS (0 = default)">
            <InputNumber
              min={0}
              value={cfg.fullIndexBuildIntervalDays}
              onChange={(v) => set("fullIndexBuildIntervalDays", v == null ? 0 : Math.max(0, v))}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Auto suspend (s)" style={itemStyle} help="AUTO_SUSPEND (0 = default)">
            <InputNumber
              min={0}
              value={cfg.autoSuspend}
              onChange={(v) => set("autoSuspend", v == null ? 0 : Math.max(0, v))}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Request logging" style={itemStyle} help="REQUEST_LOGGING">
            <Switch
              checked={cfg.requestLogging}
              onChange={(v) => set("requestLogging", v)}
            />
          </Form.Item>
        </div>

        <MonacoSqlField
          label="Base Query (AS)"
          required
          value={cfg.query}
          onChange={(v) => set("query", v)}
          placeholder={DEFAULT_QUERY}
          objectKinds={["TABLE", "VIEW", "DYNAMIC TABLE"]}
          defaultDb={db}
          defaultSchema={schema}
          notFoundText="No tables, views, or dynamic tables"
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
