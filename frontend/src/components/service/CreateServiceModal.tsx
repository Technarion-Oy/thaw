// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useEffect, useState } from "react";
import { Form, Input, Checkbox, Select, InputNumber } from "antd";
import { DeploymentUnitOutlined } from "@ant-design/icons";
import {
  BuildCreateServiceSql, ExecDDL, ListComputePools, ListWarehouses,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import ServiceSpecFields, { specReady, type TemplateVar } from "./ServiceSpecFields";
import type { service as svcModels } from "../../../wailsjs/go/models";

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

  const canSubmit =
    cfg.name.trim().length > 0 && cfg.computePool.trim().length > 0 && specReady(cfg);

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

        <ServiceSpecFields
          db={db}
          schema={schema}
          value={cfg}
          onChange={(patch) => setCfg((prev) => ({ ...prev, ...patch }))}
          placeholder={SPEC_PLACEHOLDER}
          noun="Service"
        />

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
