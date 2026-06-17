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
import {
  Form, Input, InputNumber, Select, AutoComplete, Checkbox, Button, Space, Typography, Collapse,
} from "antd";
import { ApiOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateExternalFunctionSql, ExecDDL, ListApiIntegrations, ListUserFunctions, GetExternalFunctionOptions } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { externalfunction, snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain data shape for form state. The Wails-generated `ExternalFunctionConfig`
// class carries a `convertValues` method (it has nested `args` / `headers`
// arrays), which a plain object literal can't satisfy; we cast to the generated
// type only at the IPC boundary (`cfg as any`).
type EFConfig = Omit<externalfunction.ExternalFunctionConfig, "convertValues" | "args" | "headers"> & {
  args: { name: string; type: string }[];
  headers: { name: string; value: string }[];
};

export default function CreateExternalFunctionModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<EFConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    secure: false,
    args: [],
    returns: "VARIANT",
    notNull: false,
    nullHandling: "",
    volatility: "",
    comment: "",
    apiIntegration: "",
    headers: [],
    contextHeaders: [],
    maxBatchRows: "",
    compression: "",
    requestTranslator: "",
    responseTranslator: "",
    url: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateExternalFunctionSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  // API integrations are required for external functions — populate the picker
  // from the ones the current role can see. Failure is non-fatal: the field
  // still accepts a free-typed name (AutoComplete).
  const [apis, setApis] = useState<snowflake.ApiIntegration[]>([]);
  useEffect(() => {
    ListApiIntegrations()
      .then((opts) => setApis(opts ?? []))
      .catch(() => setApis([]));
  }, []);

  // The fixed CREATE EXTERNAL FUNCTION choice lists (compression, null handling,
  // volatility, context-header functions) come from the backend so the SQL
  // grammar and its UI options stay defined in one place.
  const [options, setOptions] = useState<externalfunction.BuilderOptions | null>(null);
  useEffect(() => {
    GetExternalFunctionOptions()
      .then((o) => setOptions(o ?? null))
      .catch(() => setOptions(null));
  }, []);
  const compressionOptions = (options?.compression ?? []).map((v) => ({ value: v, label: v }));
  const nullHandlingOptions = (options?.nullHandling ?? []).map((v) => ({ value: v, label: v }));
  const volatilityOptions = (options?.volatility ?? []).map((v) => ({ value: v, label: v }));
  // Stored as the bare function name; the label shows the `()` so it reads as a
  // function.
  const contextHeaderOptions = (options?.contextHeaders ?? []).map((v) => ({ value: v, label: `${v}()` }));

  // The request/response translators are UDFs (SHOW USER FUNCTIONS), scoped to
  // this database. Failure is non-fatal: the fields still accept a free-typed
  // qualified name (AutoComplete).
  const [udfs, setUdfs] = useState<snowflake.UserFunction[]>([]);
  useEffect(() => {
    ListUserFunctions(db)
      .then((fns) => setUdfs(fns ?? []))
      .catch(() => setUdfs([]));
  }, [db]);

  const translatorOptions = udfs.map((u) => ({
    value: u.qualified,
    label: `${u.database}.${u.schema}.${u.arguments || u.name}`,
  }));
  const translatorFilter = (input: string, option?: { label?: string; value?: string }) =>
    `${option?.label ?? ""} ${option?.value ?? ""}`.toUpperCase().includes(input.toUpperCase());

  const set = <K extends keyof EFConfig>(key: K, value: EFConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Required: name, return type, API integration, and the remote URL.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.returns.trim().length > 0 &&
    cfg.apiIntegration.trim().length > 0 &&
    cfg.url.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      const sql = await BuildCreateExternalFunctionSql(db, schema, cfg as any);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  // ── Argument rows ──────────────────────────────────────────────────────────
  const addArg = () => set("args", [...cfg.args, { name: "", type: "VARCHAR" }]);
  const updateArg = (i: number, patch: Partial<{ name: string; type: string }>) =>
    set("args", cfg.args.map((a, idx) => (idx === i ? { ...a, ...patch } : a)));
  const removeArg = (i: number) => set("args", cfg.args.filter((_, idx) => idx !== i));

  // ── Header rows ────────────────────────────────────────────────────────────
  const addHeader = () => set("headers", [...cfg.headers, { name: "", value: "" }]);
  const updateHeader = (i: number, patch: Partial<{ name: string; value: string }>) =>
    set("headers", cfg.headers.map((h, idx) => (idx === i ? { ...h, ...patch } : h)));
  const removeHeader = (i: number) => set("headers", cfg.headers.filter((_, idx) => idx !== i));

  const advancedBody = (
    <>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Null handling" style={itemStyle} help="How the function treats NULL inputs.">
          <Select
            allowClear
            value={cfg.nullHandling || undefined}
            onChange={(v) => set("nullHandling", v ?? "")}
            placeholder="(default — CALLED ON NULL INPUT)"
            options={nullHandlingOptions}
          />
        </Form.Item>
        <Form.Item label="Volatility" style={itemStyle} help="Whether repeated calls with the same input may differ.">
          <Select
            allowClear
            value={cfg.volatility || undefined}
            onChange={(v) => set("volatility", v ?? "")}
            placeholder="(default — VOLATILE)"
            options={volatilityOptions}
          />
        </Form.Item>
      </div>

      <Form.Item label="Headers" style={itemStyle} help="Static HTTP headers sent to the remote service on every call.">
        <Space direction="vertical" size={6} style={{ width: "100%" }}>
          {cfg.headers.map((h, i) => (
            <Space key={i} align="start">
              <Input
                placeholder="header"
                value={h.name}
                onChange={(e) => updateHeader(i, { name: e.target.value })}
                style={{ width: 180 }}
              />
              <Input
                placeholder="value"
                value={h.value}
                onChange={(e) => updateHeader(i, { value: e.target.value })}
                style={{ width: 220 }}
              />
              <Button icon={<DeleteOutlined />} onClick={() => removeHeader(i)} />
            </Space>
          ))}
          <Button icon={<PlusOutlined />} onClick={addHeader} size="small">Add header</Button>
        </Space>
      </Form.Item>

      <Form.Item label="Context headers" style={itemStyle} help="Snowflake context functions whose values are sent as headers on every call.">
        <Select
          mode="multiple"
          showSearch
          value={cfg.contextHeaders}
          onChange={(v) => set("contextHeaders", v)}
          placeholder="CURRENT_TIMESTAMP(), CURRENT_USER(), …"
          options={contextHeaderOptions}
          optionFilterProp="label"
          style={{ width: "100%" }}
        />
      </Form.Item>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Max batch rows" style={itemStyle} help="MAX_BATCH_ROWS — caps the rows per remote request.">
          <InputNumber
            min={1}
            style={{ width: "100%" }}
            value={cfg.maxBatchRows === "" ? undefined : Number(cfg.maxBatchRows)}
            onChange={(v) => set("maxBatchRows", v == null ? "" : String(v))}
            placeholder="(default)"
          />
        </Form.Item>
        <Form.Item label="Compression" style={itemStyle} help="Compression of the request/response payloads.">
          <Select
            allowClear
            value={cfg.compression || undefined}
            onChange={(v) => set("compression", v ?? "")}
            placeholder="(default — AUTO)"
            options={compressionOptions}
          />
        </Form.Item>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Request translator" style={itemStyle} help="UDF that reshapes Snowflake rows into the request body.">
          <AutoComplete
            value={cfg.requestTranslator}
            onChange={(v) => set("requestTranslator", v)}
            placeholder="DB.SCHEMA.REQUEST_UDF"
            options={translatorOptions}
            filterOption={translatorFilter}
            style={{ width: "100%" }}
          />
        </Form.Item>
        <Form.Item label="Response translator" style={itemStyle} help="UDF that reshapes the remote response back into rows.">
          <AutoComplete
            value={cfg.responseTranslator}
            onChange={(v) => set("responseTranslator", v)}
            placeholder="DB.SCHEMA.RESPONSE_UDF"
            options={translatorOptions}
            filterOption={translatorFilter}
            style={{ width: "100%" }}
          />
        </Form.Item>
      </div>
    </>
  );

  return (
    <CreateModalShell
      icon={<ApiOutlined />}
      title="Create External Function"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="External function creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Function name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_EXTERNAL_FUNCTION"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Space direction="vertical" size={4}>
              <Checkbox checked={cfg.orReplace} onChange={(e) => set("orReplace", e.target.checked)}>
                OR REPLACE
              </Checkbox>
              <Checkbox checked={cfg.secure} onChange={(e) => set("secure", e.target.checked)}>
                SECURE
              </Checkbox>
            </Space>
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

        <Form.Item label="Arguments" style={itemStyle} help="Function parameters passed to the remote service. A blank type defaults to VARIANT.">
          <Space direction="vertical" size={6} style={{ width: "100%" }}>
            {cfg.args.map((a, i) => (
              <Space key={i} align="start">
                <Input
                  placeholder="name"
                  value={a.name}
                  onChange={(e) => updateArg(i, { name: e.target.value })}
                  style={{ width: 180 }}
                />
                <Input
                  placeholder="TYPE (e.g. NUMBER)"
                  value={a.type}
                  onChange={(e) => updateArg(i, { type: e.target.value })}
                  style={{ width: 220 }}
                />
                <Button icon={<DeleteOutlined />} onClick={() => removeArg(i)} />
              </Space>
            ))}
            <Button icon={<PlusOutlined />} onClick={addArg} size="small">Add argument</Button>
          </Space>
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Returns" required style={itemStyle} help="Result data type.">
            <Input
              value={cfg.returns}
              onChange={(e) => set("returns", e.target.value)}
              placeholder="VARIANT"
            />
          </Form.Item>
          <Form.Item style={itemStyle}>
            <Checkbox checked={cfg.notNull} onChange={(e) => set("notNull", e.target.checked)}>
              NOT NULL
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item label="API integration" required style={itemStyle} help="The API integration that authorizes and proxies the remote call.">
          <AutoComplete
            value={cfg.apiIntegration}
            onChange={(v) => set("apiIntegration", v)}
            placeholder="MY_API_INTEGRATION"
            options={apis.map((a) => ({ value: a.name, label: a.name }))}
            filterOption={(input, option) =>
              String(option?.value ?? "").toUpperCase().includes(input.toUpperCase())}
            style={{ width: "100%" }}
          />
        </Form.Item>

        <Form.Item label="URL" required style={itemStyle} help="The proxy/resource URL the integration forwards calls to (AS '<url>').">
          <Input
            value={cfg.url}
            onChange={(e) => set("url", e.target.value)}
            placeholder="https://abc123.execute-api.us-east-1.amazonaws.com/prod/echo"
          />
        </Form.Item>

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[{ key: "advanced", label: "Advanced options", children: advancedBody }]}
        />

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 8 }}>
          External functions call code outside Snowflake (AWS Lambda, Azure / GCP
          functions) through the selected API integration.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
