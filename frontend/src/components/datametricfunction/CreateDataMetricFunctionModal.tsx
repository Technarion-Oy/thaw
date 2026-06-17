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
  Form, Input, Checkbox, Button, Space, Typography,
} from "antd";
import { FundOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateDataMetricFunctionSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { datametricfunction } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Stable row ids so React keys survive insert/remove (using the array index as a
// key shifts focus when a non-last row is removed). The id is local-only; the
// backend struct ignores the extra field.
let rowUid = 0;
const nextId = () => ++rowUid;

type Column = { id: number; name: string; type: string };
type TableArg = { id: number; name: string; columns: Column[] };

const newColumn = (): Column => ({ id: nextId(), name: "", type: "VARCHAR" });
const newArg = (name: string): TableArg => ({ id: nextId(), name, columns: [newColumn()] });

// Plain data shape for form state. The Wails-generated `DataMetricFunctionConfig`
// class carries a `convertValues` method (it has a nested `args` array of nested
// `columns`), which a plain object literal can't satisfy; we cast to the generated
// type only at the IPC boundary (`cfg as any`).
type DMFConfig = Omit<datametricfunction.DataMetricFunctionConfig, "convertValues" | "args"> & {
  args: TableArg[];
};

export default function CreateDataMetricFunctionModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<DMFConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    secure: false,
    args: [newArg("table_data")],
    notNull: false,
    comment: "",
    body: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateDataMetricFunctionSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof DMFConfig>(key: K, value: DMFConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // OR REPLACE and IF NOT EXISTS are mutually exclusive — toggling one clears the
  // other so the preview (and the emitted SQL) never carry both.
  const setOrReplace = (v: boolean) =>
    setCfg((prev) => ({ ...prev, orReplace: v, ifNotExists: v ? false : prev.ifNotExists }));
  const setIfNotExists = (v: boolean) =>
    setCfg((prev) => ({ ...prev, ifNotExists: v, orReplace: v ? false : prev.orReplace }));

  // Required: name and a body expression. Every table argument must contribute at
  // least one named column, so a fully-blank argument can't slip through and emit
  // a placeholder column in the generated DDL.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.body.trim().length > 0 &&
    cfg.args.length > 0 &&
    cfg.args.every((a) => a.columns.some((c) => c.name.trim().length > 0));

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      const sql = await BuildCreateDataMetricFunctionSql(db, schema, cfg as any);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  // ── Table-argument + column editing ────────────────────────────────────────
  const updateArgs = (next: TableArg[]) => set("args", next);
  const addArg = () =>
    updateArgs([...cfg.args, newArg(`table_data_${cfg.args.length + 1}`)]);
  const removeArg = (ai: number) => updateArgs(cfg.args.filter((_, idx) => idx !== ai));
  const setArgName = (ai: number, value: string) =>
    updateArgs(cfg.args.map((a, idx) => (idx === ai ? { ...a, name: value } : a)));

  const addColumn = (ai: number) =>
    updateArgs(cfg.args.map((a, idx) => (idx === ai ? { ...a, columns: [...a.columns, newColumn()] } : a)));
  const updateColumn = (ai: number, ci: number, patch: Partial<Column>) =>
    updateArgs(cfg.args.map((a, idx) =>
      idx === ai ? { ...a, columns: a.columns.map((c, j) => (j === ci ? { ...c, ...patch } : c)) } : a));
  const removeColumn = (ai: number, ci: number) =>
    updateArgs(cfg.args.map((a, idx) =>
      idx === ai ? { ...a, columns: a.columns.filter((_, j) => j !== ci) } : a));

  return (
    <CreateModalShell
      icon={<FundOutlined />}
      title="Create Data Metric Function"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Data metric function creation failed"
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
              placeholder="MY_DATA_METRIC"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Space direction="vertical" size={4}>
              <Checkbox checked={cfg.orReplace} onChange={(e) => setOrReplace(e.target.checked)}>
                OR REPLACE
              </Checkbox>
              <Checkbox checked={cfg.ifNotExists} onChange={(e) => setIfNotExists(e.target.checked)}>
                IF NOT EXISTS
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

        <Form.Item
          label="Table arguments"
          required
          style={itemStyle}
          help="Each TABLE argument is a named set of typed columns the body measures over. Most DMFs use one; Snowflake allows several. A blank column type defaults to VARCHAR."
        >
          <Space direction="vertical" size={10} style={{ width: "100%" }}>
            {cfg.args.map((a, ai) => (
              <div
                key={a.id}
                style={{ border: "1px solid var(--border)", borderRadius: 6, padding: 10 }}
              >
                <Space align="center" style={{ marginBottom: 8, width: "100%", justifyContent: "space-between" }}>
                  <Space align="center">
                    <span style={{ fontSize: 12, color: "var(--text-muted)" }}>Argument name</span>
                    <Input
                      placeholder="table_data"
                      value={a.name}
                      onChange={(e) => setArgName(ai, e.target.value)}
                      style={{ width: 200 }}
                    />
                  </Space>
                  {cfg.args.length > 1 && (
                    <Button
                      size="small"
                      icon={<DeleteOutlined />}
                      onClick={() => removeArg(ai)}
                      danger
                    >
                      Remove argument
                    </Button>
                  )}
                </Space>
                <Space direction="vertical" size={6} style={{ width: "100%" }}>
                  {a.columns.map((c, ci) => (
                    <Space key={c.id} align="start">
                      <Input
                        placeholder="column"
                        value={c.name}
                        onChange={(e) => updateColumn(ai, ci, { name: e.target.value })}
                        style={{ width: 180 }}
                      />
                      <Input
                        placeholder="TYPE (e.g. NUMBER)"
                        value={c.type}
                        onChange={(e) => updateColumn(ai, ci, { type: e.target.value })}
                        style={{ width: 220 }}
                      />
                      <Button
                        icon={<DeleteOutlined />}
                        onClick={() => removeColumn(ai, ci)}
                        disabled={a.columns.length <= 1}
                      />
                    </Space>
                  ))}
                  <Button icon={<PlusOutlined />} onClick={() => addColumn(ai)} size="small">Add column</Button>
                </Space>
              </div>
            ))}
            <Button icon={<PlusOutlined />} onClick={addArg} size="small" type="dashed">Add table argument</Button>
          </Space>
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "auto 1fr", gap: "0 16px", alignItems: "center" }}>
          <Form.Item label="Returns" style={itemStyle}>
            <Input value="NUMBER" disabled />
          </Form.Item>
          <Form.Item label=" " style={itemStyle}>
            <Checkbox checked={cfg.notNull} onChange={(e) => set("notNull", e.target.checked)}>
              NOT NULL
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item
          label="Body"
          required
          style={itemStyle}
          help="A scalar SQL expression that returns the metric as a NUMBER, aggregating over the table argument. It must be deterministic."
        >
          <Input.TextArea
            value={cfg.body}
            onChange={(e) => set("body", e.target.value)}
            placeholder={"SELECT COUNT_IF(c IS NULL)\nFROM table_data"}
            autoSize={{ minRows: 4, maxRows: 16 }}
            style={{ fontFamily: "var(--font-mono, monospace)", fontSize: 12 }}
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 12, display: "block", marginBottom: 8 }}>
          Data metric functions define a data-quality rule that returns a numeric
          metric. Schedule one against a table or view with{" "}
          <Text code style={{ fontSize: 11 }}>ALTER TABLE … ADD DATA METRIC FUNCTION</Text>.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
