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

import { useEffect, useState } from "react";
import { Form, Input, Checkbox, Select, Radio, InputNumber, Button, Space, Typography } from "antd";
import { DeploymentUnitOutlined, PlusOutlined, MinusCircleOutlined } from "@ant-design/icons";
import {
  BuildCreateServiceSql, ExecDDL, ListComputePools, ListWarehouses,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import StageFilePicker from "./StageFilePicker";
import type { service as svcModels } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// The Wails-generated config class carries a `convertValues` method that a plain
// object literal can't satisfy; we cast to the generated type only at the IPC
// boundary (`cfg as any`).
type ServiceCfg = Omit<svcModels.ServiceConfig, "convertValues" | "templateVars"> & {
  templateVars: TemplateVar[];
};
type TemplateVar = Omit<svcModels.TemplateVar, "convertValues">;

const SPEC_PLACEHOLDER = `spec:
  containers:
  - name: main
    image: /db/schema/repo/image:latest
  endpoints:
  - name: api
    port: 8080
    public: true`;

export default function CreateServiceModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<ServiceCfg>({
    name: "",
    caseSensitive: false,
    ifNotExists: false,
    computePool: "",
    specSource: "inline",
    template: false,
    specInline: "",
    specStage: "",
    specFile: "",
    templateVars: [],
    externalAccessIntegrations: "",
    autoResume: "",
    minInstances: "",
    maxInstances: "",
    queryWarehouse: "",
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateServiceSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const [pools, setPools] = useState<string[]>([]);
  const [loadingPools, setLoadingPools] = useState(false);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  useEffect(() => {
    setLoadingPools(true);
    ListComputePools()
      .then((names) => setPools(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingPools(false));
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  const set = <K extends keyof ServiceCfg>(key: K, value: ServiceCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Template-variable (USING) editor helpers.
  const addVar = () => setCfg((prev) => ({ ...prev, templateVars: [...prev.templateVars, { key: "", value: "" }] }));
  const updateVar = (i: number, field: "key" | "value", val: string) =>
    setCfg((prev) => ({
      ...prev,
      templateVars: prev.templateVars.map((v, idx) => (idx === i ? { ...v, [field]: val } : v)),
    }));
  const removeVar = (i: number) =>
    setCfg((prev) => ({ ...prev, templateVars: prev.templateVars.filter((_, idx) => idx !== i) }));

  const specReady =
    cfg.specSource === "stage"
      ? cfg.specStage.trim().length > 0 && cfg.specFile.trim().length > 0
      : cfg.specInline.trim().length > 0;
  const canSubmit =
    cfg.name.trim().length > 0 && cfg.computePool.trim().length > 0 && specReady;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const poolOptions = (pools || []).map((n) => ({ value: n, label: n }));
  const warehouseOptions = (warehouses || []).map((n) => ({ value: n, label: n }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<DeploymentUnitOutlined />}
      title="Create Service"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Service creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Service name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_SERVICE"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            {/* CREATE SERVICE has no OR REPLACE in Snowflake — only IF NOT EXISTS. */}
            <Checkbox
              checked={cfg.ifNotExists}
              onChange={(e) => set("ifNotExists", e.target.checked)}
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

        <Form.Item label="Compute pool" required style={itemStyle} help="The compute pool that hosts the service containers.">
          <Select
            showSearch
            value={cfg.computePool || undefined}
            onChange={(v) => set("computePool", v ?? "")}
            options={poolOptions}
            placeholder="Select a compute pool"
            loading={loadingPools}
            notFoundContent={loadingPools ? "Loading…" : "No compute pools found"}
          />
        </Form.Item>

        <Form.Item label="Specification" style={{ marginBottom: 6 }}>
          <Space size={16} wrap>
            <Radio.Group
              value={cfg.specSource}
              onChange={(e) => set("specSource", e.target.value)}
              optionType="button"
              buttonStyle="solid"
              size="small"
            >
              <Radio.Button value="inline">Inline YAML</Radio.Button>
              <Radio.Button value="stage">From stage file</Radio.Button>
            </Radio.Group>
            <Checkbox checked={cfg.template} onChange={(e) => set("template", e.target.checked)}>
              Template (with variables)
            </Checkbox>
          </Space>
        </Form.Item>

        {cfg.specSource === "stage" ? (
          <>
            <Form.Item style={{ marginBottom: 12 }}>
              <StageFilePicker
                db={db}
                schema={schema}
                onPick={(stage, file) => setCfg((prev) => ({ ...prev, specStage: stage, specFile: file }))}
              />
            </Form.Item>
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
              <Form.Item label="Stage" required style={itemStyle} help="Internal stage (browse above, or type @stage / db.schema.stage).">
                <Input
                  value={cfg.specStage}
                  onChange={(e) => set("specStage", e.target.value)}
                  placeholder="@my_stage"
                />
              </Form.Item>
              <Form.Item label="Specification file" required style={itemStyle} help="Path to the YAML file within the stage.">
                <Input
                  value={cfg.specFile}
                  onChange={(e) => set("specFile", e.target.value)}
                  placeholder="service/spec.yaml"
                />
              </Form.Item>
            </div>
          </>
        ) : (
          <Form.Item
            label={cfg.template ? "Service specification template (YAML, with {{ variables }})" : "Service specification (YAML)"}
            required
            style={itemStyle}
          >
            <Input.TextArea
              value={cfg.specInline}
              onChange={(e) => set("specInline", e.target.value)}
              placeholder={SPEC_PLACEHOLDER}
              autoSize={{ minRows: 8, maxRows: 18 }}
              style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}
            />
          </Form.Item>
        )}

        {cfg.template && (
          <Form.Item
            label="Template variables (USING)"
            style={itemStyle}
            help="Bound to the template's {{ variables }}. Numbers and TRUE/FALSE/NULL are emitted unquoted; everything else is a string literal."
          >
            <Space direction="vertical" style={{ width: "100%" }} size={6}>
              {cfg.templateVars.map((v, i) => (
                <Space key={i} style={{ width: "100%" }} align="center">
                  <Input
                    value={v.key}
                    onChange={(e) => updateVar(i, "key", e.target.value)}
                    placeholder="name"
                    style={{ width: 200 }}
                  />
                  <Text type="secondary">=&gt;</Text>
                  <Input
                    value={v.value}
                    onChange={(e) => updateVar(i, "value", e.target.value)}
                    placeholder="value"
                    style={{ width: 260 }}
                  />
                  <Button type="text" size="small" icon={<MinusCircleOutlined />} onClick={() => removeVar(i)} />
                </Space>
              ))}
              <Button type="dashed" size="small" icon={<PlusOutlined />} onClick={addVar}>
                Add variable
              </Button>
            </Space>
          </Form.Item>
        )}

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Min instances" style={itemStyle}>
            <InputNumber
              min={1}
              value={cfg.minInstances ? Number(cfg.minInstances) : undefined}
              onChange={(v) => set("minInstances", v == null ? "" : String(v))}
              placeholder="1"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Max instances" style={itemStyle}>
            <InputNumber
              min={1}
              value={cfg.maxInstances ? Number(cfg.maxInstances) : undefined}
              onChange={(v) => set("maxInstances", v == null ? "" : String(v))}
              placeholder="1"
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Auto resume" style={itemStyle} help="Resume the service when a function/endpoint is called.">
            <Select
              allowClear
              value={cfg.autoResume || undefined}
              onChange={(v) => set("autoResume", v ?? "")}
              placeholder="Default (TRUE)"
              options={[
                { value: "TRUE", label: "TRUE" },
                { value: "FALSE", label: "FALSE" },
              ]}
            />
          </Form.Item>
          <Form.Item label="Query warehouse" style={itemStyle} help="Warehouse used by the service's SQL queries.">
            <Select
              showSearch
              allowClear
              value={cfg.queryWarehouse || undefined}
              onChange={(v) => set("queryWarehouse", v ?? "")}
              options={warehouseOptions}
              placeholder="(optional)"
              loading={loadingWarehouses}
              notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
            />
          </Form.Item>
        </div>

        <Form.Item label="External access integrations" style={itemStyle} help="Comma-separated EAI names granting outbound network access.">
          <Input
            value={cfg.externalAccessIntegrations}
            onChange={(e) => set("externalAccessIntegrations", e.target.value)}
            placeholder="EAI_ONE, EAI_TWO"
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
