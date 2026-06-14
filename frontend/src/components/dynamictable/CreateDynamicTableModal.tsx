// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import {
  Form, Input, InputNumber, Checkbox, Select, Space, Collapse, Radio,
} from "antd";
import { RetweetOutlined } from "@ant-design/icons";
import {
  BuildCreateDynamicTableSql, ExecDDL, ListWarehouses,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import TagInput from "../shared/TagInput";
import MonacoSqlField from "../shared/MonacoSqlField";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { dynamictable } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_QUERY = "SELECT *\n  FROM my_source_table";

type LagUnit = "seconds" | "minutes" | "hours" | "days";

// Plain data shape for form state. The Wails-generated `DynamicTableConfig`
// class carries a `convertValues` method (it has a nested `tags` array), which a
// plain object literal can't satisfy; we cast to the generated type only at the
// IPC boundary (`cfg as any`).
type DTConfig = Omit<dynamictable.DynamicTableConfig, "convertValues" | "tags"> & {
  tags: { name: string; value: string }[];
};

export default function CreateDynamicTableModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<DTConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    transient: false,
    targetLag: "1 minutes",
    scheduler: "",
    warehouse: "",
    initializationWarehouse: "",
    refreshMode: "",
    initialize: "",
    clusterBy: "",
    dataRetentionTimeInDays: "",
    maxDataExtensionTimeInDays: "",
    comment: "",
    copyGrants: false,
    requireUser: false,
    rowTimestamp: "",
    tags: [],
    query: DEFAULT_QUERY,
  });

  // Target-lag composer state. The composed string lives in cfg.targetLag.
  const [lagMode, setLagMode] = useState<"interval" | "downstream">("interval");
  const [lagNum, setLagNum] = useState<number>(1);
  const [lagUnit, setLagUnit] = useState<LagUnit>("minutes");

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateDynamicTableSql(db, schema, cfg as any),
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

  const set = <K extends keyof DTConfig>(key: K, value: DTConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Recompute the TARGET_LAG string whenever the composer inputs change.
  useEffect(() => {
    const composed = lagMode === "downstream" ? "DOWNSTREAM" : `${lagNum} ${lagUnit}`;
    set("targetLag", composed);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lagMode, lagNum, lagUnit]);

  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.warehouse.trim().length > 0 &&
    cfg.targetLag.trim().length > 0 &&
    cfg.query.trim().length > 0;

  const handleRun = () => submit(async () => {
    await ExecDDL(preview);
    onSuccess?.();
    onClose();
  });

  const warehouseOptions = (warehouses || []).map((n) => ({ value: n, label: n }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const advancedBody = (
    <>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Refresh Mode" style={itemStyle} help="How rows are refreshed">
          <Select
            allowClear
            value={cfg.refreshMode || undefined}
            onChange={(v) => set("refreshMode", v ?? "")}
            placeholder="AUTO (default)"
            style={{ width: "100%" }}
            options={[
              { value: "AUTO", label: "AUTO" },
              { value: "FULL", label: "FULL" },
              { value: "INCREMENTAL", label: "INCREMENTAL" },
              { value: "ADAPTIVE", label: "ADAPTIVE" },
              { value: "CUSTOM_INCREMENTAL", label: "CUSTOM_INCREMENTAL" },
            ]}
          />
        </Form.Item>
        <Form.Item label="Initialize" style={itemStyle} help="When the first refresh runs">
          <Select
            allowClear
            value={cfg.initialize || undefined}
            onChange={(v) => set("initialize", v ?? "")}
            placeholder="ON_CREATE (default)"
            style={{ width: "100%" }}
            options={[
              { value: "ON_CREATE", label: "ON_CREATE" },
              { value: "ON_SCHEDULE", label: "ON_SCHEDULE" },
            ]}
          />
        </Form.Item>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Scheduler" style={itemStyle} help="Enable or disable the automatic refresh scheduler">
          <Select
            allowClear
            value={cfg.scheduler || undefined}
            onChange={(v) => set("scheduler", v ?? "")}
            placeholder="ENABLE (default)"
            style={{ width: "100%" }}
            options={[
              { value: "ENABLE", label: "ENABLE" },
              { value: "DISABLE", label: "DISABLE" },
            ]}
          />
        </Form.Item>
        <Form.Item label="Initialization Warehouse" style={itemStyle} help="Warehouse used only for the initial refresh">
          <Select
            allowClear
            showSearch
            loading={loadingWarehouses}
            value={cfg.initializationWarehouse || undefined}
            onChange={(v) => set("initializationWarehouse", v ?? "")}
            placeholder="Same as Warehouse"
            options={warehouseOptions}
            style={{ width: "100%" }}
            notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
          />
        </Form.Item>
      </div>

      <Form.Item label="Cluster By" style={itemStyle} help="Optional comma-separated clustering expressions">
        <Input
          value={cfg.clusterBy}
          onChange={(e) => set("clusterBy", e.target.value)}
          placeholder="col1, col2"
        />
      </Form.Item>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Data Retention (days)" style={itemStyle} help="Time Travel retention">
          <InputNumber
            min={0}
            value={cfg.dataRetentionTimeInDays === "" ? null : Number(cfg.dataRetentionTimeInDays)}
            onChange={(v) => set("dataRetentionTimeInDays", v == null ? "" : String(v))}
            placeholder="default"
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item label="Max Data Extension (days)" style={itemStyle} help="Max auto-extension of retention">
          <InputNumber
            min={0}
            value={cfg.maxDataExtensionTimeInDays === "" ? null : Number(cfg.maxDataExtensionTimeInDays)}
            onChange={(v) => set("maxDataExtensionTimeInDays", v == null ? "" : String(v))}
            placeholder="default"
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item label="Row Timestamp" style={itemStyle} help="Expose a row-refresh timestamp column">
          <Select
            allowClear
            value={cfg.rowTimestamp || undefined}
            onChange={(v) => set("rowTimestamp", v ?? "")}
            placeholder="default"
            style={{ width: "100%" }}
            options={[
              { value: "TRUE", label: "TRUE" },
              { value: "FALSE", label: "FALSE" },
            ]}
          />
        </Form.Item>
      </div>

      <Form.Item style={{ marginBottom: 8 }}>
        <Space size={16} wrap>
          <Checkbox checked={cfg.copyGrants} onChange={(e) => set("copyGrants", e.target.checked)}>
            COPY GRANTS
          </Checkbox>
          <Checkbox checked={cfg.requireUser} onChange={(e) => set("requireUser", e.target.checked)}>
            REQUIRE USER
          </Checkbox>
        </Space>
      </Form.Item>

      <TagInput
        tags={cfg.tags}
        onChange={(tags) => set("tags", tags)}
        help="Table-level tags applied at creation"
        itemStyle={itemStyle}
      />
    </>
  );

  return (
    <CreateModalShell
      icon={<RetweetOutlined />}
      title="Create Dynamic Table"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Dynamic table creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Dynamic table name"
          placeholder="MY_DYNAMIC_TABLE"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
          extra={
            <Checkbox
              checked={cfg.transient}
              onChange={(e) => set("transient", e.target.checked)}
            >
              TRANSIENT
            </Checkbox>
          }
        />

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Target Lag" required style={itemStyle} help="Maximum staleness before a refresh (minimum 1 minute), or DOWNSTREAM to lag behind dependents">
            <Space.Compact style={{ width: "100%" }}>
              <Radio.Group
                value={lagMode}
                onChange={(e) => setLagMode(e.target.value)}
                optionType="button"
                buttonStyle="solid"
                size="small"
                options={[
                  { value: "interval", label: "Interval" },
                  { value: "downstream", label: "Downstream" },
                ]}
              />
            </Space.Compact>
            {lagMode === "interval" && (
              <Space style={{ marginTop: 8 }}>
                <InputNumber
                  // Snowflake enforces a 1-minute minimum target lag, so seconds
                  // must be >= 60.
                  min={lagUnit === "seconds" ? 60 : 1}
                  value={lagNum}
                  onChange={(v) => {
                    const floor = lagUnit === "seconds" ? 60 : 1;
                    setLagNum(v == null ? floor : Math.max(v, floor));
                  }}
                  style={{ width: 90 }}
                />
                <Select
                  value={lagUnit}
                  onChange={(u) => {
                    setLagUnit(u);
                    if (u === "seconds" && lagNum < 60) setLagNum(60);
                  }}
                  style={{ width: 120 }}
                  options={[
                    { value: "seconds", label: "seconds" },
                    { value: "minutes", label: "minutes" },
                    { value: "hours", label: "hours" },
                    { value: "days", label: "days" },
                  ]}
                />
              </Space>
            )}
          </Form.Item>
          <Form.Item label="Warehouse" required style={itemStyle} help="Warehouse used to refresh the table">
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

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[{ key: "advanced", label: "Advanced options", children: advancedBody }]}
        />

        <MonacoSqlField
          label="Defining Query (AS)"
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
