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
import { CodeOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateProcedureSql, ExecDDL } from "../../../wailsjs/go/app/App";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import MonacoSqlField from "../shared/MonacoSqlField";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { procedure } from "../../../wailsjs/go/models";

const LANGUAGES = ["SQL", "PYTHON", "JAVA", "JAVASCRIPT", "SCALA"];
const NULL_HANDLING = ["CALLED ON NULL INPUT", "RETURNS NULL ON NULL INPUT"];
const VOLATILITY = ["VOLATILE", "IMMUTABLE"];

// Plain data shape for form state. The Wails-generated `ProcedureConfig` class
// carries a `convertValues` method (it has nested `args` / `tableColumns`
// arrays), which a plain object literal can't satisfy; we cast to the generated
// type only at the IPC boundary (`cfg as any`).
type PConfig = Omit<
  procedure.ProcedureConfig,
  "convertValues" | "args" | "tableColumns" | "packages" | "imports"
> & {
  args: { name: string; dataType: string }[];
  tableColumns: { name: string; dataType: string }[];
  packages: string[];
  imports: string[];
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateProcedureModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<PConfig>({
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
    runtimeVersion: "",
    packages: [],
    imports: [],
    handler: "",
    nullHandling: "",
    volatility: "",
    executeAs: "",
    comment: "",
    body: "BEGIN\n  RETURN 1;\nEND",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateProcedureSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof PConfig>(key: K, value: PConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // RUNTIME_VERSION / HANDLER / PACKAGES / IMPORTS apply only to the handler
  // languages (Python / Java / Scala). SQL (Snowflake Scripting) and JavaScript
  // procedures carry their logic inline in the body, so those fields are hidden
  // for them (and the Go builder drops any stale values too).
  const isHandlerLang = ["PYTHON", "JAVA", "SCALA"].includes(cfg.language.toUpperCase());

  const canSubmit = cfg.name.trim().length > 0 && cfg.body.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      const sql = await BuildCreateProcedureSql(db, schema, cfg as any);
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

  // ── RETURNS TABLE column rows ──────────────────────────────────────────────
  const addCol = () => set("tableColumns", [...cfg.tableColumns, { name: "", dataType: "VARCHAR" }]);
  const updateCol = (i: number, patch: Partial<{ name: string; dataType: string }>) =>
    set("tableColumns", cfg.tableColumns.map((c, idx) => (idx === i ? { ...c, ...patch } : c)));
  const removeCol = (i: number) => set("tableColumns", cfg.tableColumns.filter((_, idx) => idx !== i));

  const argRows = (
    rows: { name: string; dataType: string }[],
    update: (i: number, patch: Partial<{ name: string; dataType: string }>) => void,
    remove: (i: number) => void,
    add: () => void,
    addLabel: string,
  ) => (
    <Space direction="vertical" size={6} style={{ width: "100%" }}>
      {rows.map((a, i) => (
        <Space key={i} align="start">
          <Input
            placeholder="name"
            value={a.name}
            onChange={(e) => update(i, { name: e.target.value })}
            style={{ width: 180 }}
          />
          <Input
            placeholder="TYPE (e.g. NUMBER)"
            value={a.dataType}
            onChange={(e) => update(i, { dataType: e.target.value })}
            style={{ width: 220 }}
          />
          <Button icon={<DeleteOutlined />} onClick={() => remove(i)} />
        </Space>
      ))}
      <Button icon={<PlusOutlined />} onClick={add} size="small">{addLabel}</Button>
    </Space>
  );

  const advancedBody = (
    <>
      <Form.Item style={itemStyle}>
        <Checkbox checked={cfg.secure} onChange={(e) => set("secure", e.target.checked)}>
          SECURE
        </Checkbox>
      </Form.Item>

      {isHandlerLang && (
        <>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
            <Form.Item label="Runtime version" style={itemStyle} help="RUNTIME_VERSION for Python / Java / Scala handlers, e.g. 3.10.">
              <Input
                value={cfg.runtimeVersion}
                onChange={(e) => set("runtimeVersion", e.target.value)}
                placeholder="3.10"
              />
            </Form.Item>
            <Form.Item label="Handler" style={itemStyle} help="Entry point, e.g. module.function or Class.method.">
              <Input
                value={cfg.handler}
                onChange={(e) => set("handler", e.target.value)}
                placeholder="main.run"
              />
            </Form.Item>
          </div>

          <Form.Item label="Packages" style={itemStyle} help="PACKAGES for Python / Java / Scala handlers (e.g. snowflake-snowpark-python).">
            <Select
              mode="tags"
              value={cfg.packages}
              onChange={(v) => set("packages", v)}
              placeholder="snowflake-snowpark-python, pandas"
              style={{ width: "100%" }}
              tokenSeparators={[",", " "]}
              open={false}
            />
          </Form.Item>

          <Form.Item label="Imports" style={itemStyle} help="IMPORTS — staged files (e.g. @stage/handler.py).">
            <Select
              mode="tags"
              value={cfg.imports}
              onChange={(v) => set("imports", v)}
              placeholder="@my_stage/handler.py"
              style={{ width: "100%" }}
              tokenSeparators={[",", " "]}
              open={false}
            />
          </Form.Item>
        </>
      )}

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Null handling" style={itemStyle}>
          <Select
            allowClear
            value={cfg.nullHandling || undefined}
            onChange={(v) => set("nullHandling", v ?? "")}
            placeholder="(default)"
            options={NULL_HANDLING.map((v) => ({ value: v, label: v }))}
          />
        </Form.Item>
        <Form.Item label="Volatility" style={itemStyle}>
          <Select
            allowClear
            value={cfg.volatility || undefined}
            onChange={(v) => set("volatility", v ?? "")}
            placeholder="(default)"
            options={VOLATILITY.map((v) => ({ value: v, label: v }))}
          />
        </Form.Item>
        <Form.Item label="Execute as" style={itemStyle}>
          <Select
            value={cfg.executeAs || undefined}
            onChange={(v) => set("executeAs", v ?? "")}
            placeholder="Default"
            allowClear
            options={[
              { value: "CALLER", label: "CALLER" },
              { value: "OWNER", label: "OWNER" },
            ]}
          />
        </Form.Item>
      </div>
    </>
  );

  return (
    <CreateModalShell
      icon={<CodeOutlined />}
      title="Create Procedure"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Procedure creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Procedure name"
          placeholder="MY_PROCEDURE"
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

        <Form.Item label="Arguments" style={itemStyle} help="Procedure parameters. A blank type defaults to VARIANT.">
          {argRows(cfg.args, updateArg, removeArg, addArg, "Add argument")}
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Language" style={itemStyle}>
            <Select
              value={cfg.language}
              onChange={(v) => set("language", v)}
              options={LANGUAGES.map((v) => ({ value: v, label: v }))}
            />
          </Form.Item>
          <Form.Item label="Returns a table" style={itemStyle} help="RETURNS TABLE (...) vs a scalar type.">
            <Switch checked={cfg.returnsTable} onChange={(v) => set("returnsTable", v)} />
          </Form.Item>
        </div>

        {cfg.returnsTable ? (
          <Form.Item label="Table columns" style={itemStyle} help="Columns of the returned table.">
            {argRows(cfg.tableColumns, updateCol, removeCol, addCol, "Add column")}
          </Form.Item>
        ) : (
          <Form.Item label="Returns" style={itemStyle} help="Scalar result data type.">
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
          label="Procedure Body"
          required
          value={cfg.body}
          onChange={(v) => set("body", v)}
          placeholder="BEGIN\n  RETURN 1;\nEND"
          height={180}
          objectKinds={["TABLE", "VIEW"]}
          defaultDb={db}
          defaultSchema={schema}
          itemStyle={itemStyle}
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
