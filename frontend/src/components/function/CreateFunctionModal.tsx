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

import { useState } from "react";
import {
  Form, Input, Select, Checkbox, Button, Space, Switch, Collapse,
} from "antd";
import { FunctionOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateFunctionSql, ExecDDL } from "../../../wailsjs/go/app/App";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import MonacoSqlField from "../shared/MonacoSqlField";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain form-state shape. It mirrors the Go `function.FunctionConfig` field set,
// but we keep a self-contained interface here rather than reaching into the
// Wails-generated models: the package is named `function` (a TS reserved word),
// so its generated namespace name is awkward to import and may differ across
// regenerations. We cast to `any` at the IPC boundary instead.
interface FnConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  secure: boolean;
  ifNotExists: boolean;
  args: { name: string; dataType: string }[];
  returnType: string;
  returnsTable: boolean;
  tableColumns: { name: string; dataType: string }[];
  language: string;
  nullHandling: string;
  volatility: string;
  runtimeVersion: string;
  packages: string[];
  imports: string[];
  handler: string;
  comment: string;
  body: string;
}

const BODY_PLACEHOLDER = "-- function body (SQL expression or handler code)";

const LANGUAGE_OPTIONS = ["SQL", "PYTHON", "JAVA", "JAVASCRIPT", "SCALA"].map((v) => ({
  value: v,
  label: v,
}));
const NULL_HANDLING_OPTIONS = ["CALLED ON NULL INPUT", "RETURNS NULL ON NULL INPUT"].map((v) => ({
  value: v,
  label: v,
}));
const VOLATILITY_OPTIONS = ["VOLATILE", "IMMUTABLE"].map((v) => ({ value: v, label: v }));

// Per-language example hints for the handler-only fields, so the ghost
// placeholders match the selected runtime (Python / Java / Scala) instead of
// always showing Python examples.
const LANG_HINTS: Record<string, { runtime: string; handler: string; packages: string; imports: string }> = {
  PYTHON: { runtime: "e.g. 3.11", handler: "my_module.my_func", packages: "numpy, pandas, …", imports: "@stage/handler.py, …" },
  JAVA:   { runtime: "e.g. 17",   handler: "MyClass.myMethod",  packages: "com.example:my-lib:1.0, …", imports: "@stage/lib.jar, …" },
  SCALA:  { runtime: "e.g. 2.12", handler: "MyObject.myFunc",   packages: "com.example:my-lib:1.0, …", imports: "@stage/lib.jar, …" },
};

export default function CreateFunctionModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<FnConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    secure: false,
    ifNotExists: false,
    args: [],
    returnType: "VARIANT",
    returnsTable: false,
    tableColumns: [],
    language: "SQL",
    nullHandling: "",
    volatility: "",
    runtimeVersion: "",
    packages: [],
    imports: [],
    handler: "",
    comment: "",
    body: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateFunctionSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof FnConfig>(key: K, value: FnConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // RUNTIME_VERSION / PACKAGES / IMPORTS / HANDLER apply only to the handler
  // languages (Python / Java / Scala). SQL and JavaScript functions carry their
  // logic inline in the body, so those fields are hidden for them (and the Go
  // builder drops any stale values too).
  const isHandlerLang = ["PYTHON", "JAVA", "SCALA"].includes(cfg.language.toUpperCase());
  const hints = LANG_HINTS[cfg.language.toUpperCase()] ?? LANG_HINTS.PYTHON;

  // Required: name and a function body. Handler languages (Python/Java/Scala)
  // additionally require RUNTIME_VERSION and HANDLER — Snowflake rejects the
  // CREATE otherwise, so block submission until they're filled.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.body.trim().length > 0 &&
    (!isHandlerLang || (cfg.runtimeVersion.trim().length > 0 && cfg.handler.trim().length > 0));

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      const sql = await BuildCreateFunctionSql(db, schema, cfg as any);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  // ── Argument rows ──────────────────────────────────────────────────────────
  const addArg = () => set("args", [...cfg.args, { name: "", dataType: "VARCHAR" }]);
  const updateArg = (i: number, patch: Partial<{ name: string; dataType: string }>) =>
    set("args", cfg.args.map((a, idx) => (idx === i ? { ...a, ...patch } : a)));
  const removeArg = (i: number) => set("args", cfg.args.filter((_, idx) => idx !== i));

  // ── Table-column rows (RETURNS TABLE) ──────────────────────────────────────
  const addCol = () => set("tableColumns", [...cfg.tableColumns, { name: "", dataType: "VARCHAR" }]);
  const updateCol = (i: number, patch: Partial<{ name: string; dataType: string }>) =>
    set("tableColumns", cfg.tableColumns.map((c, idx) => (idx === i ? { ...c, ...patch } : c)));
  const removeCol = (i: number) =>
    set("tableColumns", cfg.tableColumns.filter((_, idx) => idx !== i));

  const argEditor = (
    rows: { name: string; dataType: string }[],
    onUpdate: (i: number, patch: Partial<{ name: string; dataType: string }>) => void,
    onRemove: (i: number) => void,
    onAdd: () => void,
    addLabel: string,
  ) => (
    <Space direction="vertical" size={6} style={{ width: "100%" }}>
      {rows.map((a, i) => (
        <Space key={i} align="start">
          <Input
            placeholder="name"
            value={a.name}
            onChange={(e) => onUpdate(i, { name: e.target.value })}
            style={{ width: 180 }}
          />
          <Input
            placeholder="TYPE (e.g. NUMBER)"
            value={a.dataType}
            onChange={(e) => onUpdate(i, { dataType: e.target.value })}
            style={{ width: 220 }}
          />
          <Button icon={<DeleteOutlined />} onClick={() => onRemove(i)} />
        </Space>
      ))}
      <Button icon={<PlusOutlined />} onClick={onAdd} size="small">{addLabel}</Button>
    </Space>
  );

  const advancedBody = (
    <>
      <Form.Item style={itemStyle}>
        <Checkbox checked={cfg.secure} onChange={(e) => set("secure", e.target.checked)}>
          SECURE
        </Checkbox>
      </Form.Item>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Null handling" style={itemStyle} help="How the function treats NULL inputs.">
          <Select
            allowClear
            value={cfg.nullHandling || undefined}
            onChange={(v) => set("nullHandling", v ?? "")}
            placeholder="(default — CALLED ON NULL INPUT)"
            options={NULL_HANDLING_OPTIONS}
          />
        </Form.Item>
        <Form.Item label="Volatility" style={itemStyle} help="Whether repeated calls with the same input may differ.">
          <Select
            allowClear
            value={cfg.volatility || undefined}
            onChange={(v) => set("volatility", v ?? "")}
            placeholder="(default — VOLATILE)"
            options={VOLATILITY_OPTIONS}
          />
        </Form.Item>
      </div>

      {isHandlerLang && (
        <>
          <Form.Item label="Runtime version" required style={itemStyle} help={`RUNTIME_VERSION — required for the ${cfg.language.toUpperCase()} handler.`}>
            <Input
              value={cfg.runtimeVersion}
              onChange={(e) => set("runtimeVersion", e.target.value)}
              placeholder={hints.runtime}
            />
          </Form.Item>

          <Form.Item label="Packages" style={itemStyle} help="PACKAGES — Snowflake-provided libraries the handler imports.">
            <Select
              mode="tags"
              value={cfg.packages}
              onChange={(v) => set("packages", v)}
              placeholder={hints.packages}
              open={false}
              style={{ width: "100%" }}
            />
          </Form.Item>

          <Form.Item label="Imports" style={itemStyle} help={`IMPORTS — stage paths to files the handler loads (e.g. ${hints.imports.split(",")[0]}).`}>
            <Select
              mode="tags"
              value={cfg.imports}
              onChange={(v) => set("imports", v)}
              placeholder={hints.imports}
              open={false}
              style={{ width: "100%" }}
            />
          </Form.Item>

          <Form.Item label="Handler" required style={itemStyle} help={`HANDLER — entry point for the ${cfg.language.toUpperCase()} handler (e.g. ${hints.handler}).`}>
            <Input
              value={cfg.handler}
              onChange={(e) => set("handler", e.target.value)}
              placeholder={hints.handler}
            />
          </Form.Item>
        </>
      )}
    </>
  );

  return (
    <CreateModalShell
      icon={<FunctionOutlined />}
      title="Create Function"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Function creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Function name"
          placeholder="MY_FUNCTION"
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

        <Form.Item label="Arguments" style={itemStyle} help="Function parameters. A blank type defaults to VARIANT.">
          {argEditor(cfg.args, updateArg, removeArg, addArg, "Add argument")}
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Language" style={itemStyle} help="SQL functions return an expression; others run handler code.">
            <Select
              value={cfg.language}
              onChange={(v) => set("language", v)}
              options={LANGUAGE_OPTIONS}
            />
          </Form.Item>
          <Form.Item label="Returns TABLE" style={itemStyle}>
            <Switch
              checked={cfg.returnsTable}
              onChange={(v) => set("returnsTable", v)}
            />
          </Form.Item>
        </div>

        {cfg.returnsTable ? (
          <Form.Item label="Table columns" style={itemStyle} help="The RETURNS TABLE (...) output columns.">
            {argEditor(cfg.tableColumns, updateCol, removeCol, addCol, "Add column")}
          </Form.Item>
        ) : (
          <Form.Item label="Return type" style={itemStyle} help="Scalar result data type.">
            <Input
              value={cfg.returnType}
              onChange={(e) => set("returnType", e.target.value)}
              placeholder="VARIANT"
            />
          </Form.Item>
        )}

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

        <MonacoSqlField
          label="Function Body"
          required
          help="A SQL expression for SQL functions, or handler source code for other languages."
          value={cfg.body}
          onChange={(v) => set("body", v)}
          placeholder={BODY_PLACEHOLDER}
          objectKinds={["TABLE", "VIEW"]}
          defaultDb={db}
          defaultSchema={schema}
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
