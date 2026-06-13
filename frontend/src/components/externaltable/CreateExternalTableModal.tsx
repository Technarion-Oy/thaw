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
  Modal, Form, Input, Checkbox, Select, Space,
  Typography, Button, Alert, Collapse, Tag, Radio, Tooltip,
} from "antd";
import { CloudServerOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import {
  BuildCreateExternalTableSql, ExecDDL, GetQuotedIdentifiersIgnoreCase,
  ListDatabases, ListSchemas, ListObjects,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import type { externaltable } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain data shape for form state. The Wails-generated `ExternalTableConfig`
// class carries a `convertValues` method (it has nested `columns`/`tags`
// arrays), which a plain object literal can't satisfy; we cast to the generated
// type only at the IPC boundary (`cfg as any`).
type ETColumn = { name: string; type: string; expression: string; partition: boolean };
type ETConfig = Omit<externaltable.ExternalTableConfig, "convertValues" | "columns" | "tags"> & {
  columns: ETColumn[];
  tags: { name: string; value: string }[];
};

const FILE_FORMAT_TYPES = ["CSV", "JSON", "AVRO", "ORC", "PARQUET"];

export default function CreateExternalTableModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<ETConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    columns: [],
    location: "",
    refreshOnCreate: "",
    autoRefresh: "",
    pattern: "",
    fileFormatName: "",
    fileFormatType: "CSV",
    awsSnsTopic: "",
    copyGrants: false,
    comment: "",
    tags: [],
  });

  // File-format mode: a named FILE FORMAT object or an inline TYPE.
  const [fmtMode, setFmtMode] = useState<"type" | "named">("type");

  // New-tag draft inputs.
  const [tagName, setTagName] = useState("");
  const [tagValue, setTagValue] = useState("");

  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");

  // Stage / file-format pickers (database → schema → object).
  const [pickerDb, setPickerDb] = useState(db);
  const [pickerSchema, setPickerSchema] = useState(schema);
  const [dbOptions, setDbOptions] = useState<string[]>([]);
  const [schemaOptions, setSchemaOptions] = useState<string[]>([]);
  const [stageOptions, setStageOptions] = useState<string[]>([]);
  const [formatOptions, setFormatOptions] = useState<string[]>([]);
  const [pickerStage, setPickerStage] = useState("");
  const [stagePath, setStagePath] = useState("");
  const [loadingSchemas, setLoadingSchemas] = useState(false);
  const [loadingObjects, setLoadingObjects] = useState(false);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
    ListDatabases().then((d) => setDbOptions(d ?? [])).catch(() => {});
  }, []);

  useEffect(() => {
    BuildCreateExternalTableSql(db, schema, cfg as any).then(setPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = <K extends keyof ETConfig>(key: K, value: ETConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // ── Column editing ──────────────────────────────────────────────────────
  const addColumn = () =>
    set("columns", [...cfg.columns, { name: "", type: "VARCHAR", expression: "", partition: false }]);
  const updateColumn = (i: number, patch: Partial<ETColumn>) =>
    set("columns", cfg.columns.map((c, idx) => (idx === i ? { ...c, ...patch } : c)));
  const removeColumn = (i: number) =>
    set("columns", cfg.columns.filter((_, idx) => idx !== i));

  // ── Tags ────────────────────────────────────────────────────────────────
  const addTag = () => {
    const name = tagName.trim();
    if (!name) return;
    set("tags", [...(cfg.tags ?? []).filter((t) => t.name !== name), { name, value: tagValue.trim() }]);
    setTagName("");
    setTagValue("");
  };
  const removeTag = (name: string) => set("tags", (cfg.tags ?? []).filter((t) => t.name !== name));

  // ── Stage / file-format pickers ─────────────────────────────────────────
  useEffect(() => {
    if (!pickerDb) { setSchemaOptions([]); return; }
    setLoadingSchemas(true);
    ListSchemas(pickerDb)
      .then((s) => setSchemaOptions(s ?? []))
      .catch(() => setSchemaOptions([]))
      .finally(() => setLoadingSchemas(false));
  }, [pickerDb]);

  useEffect(() => {
    if (!pickerDb || !pickerSchema) { setStageOptions([]); setFormatOptions([]); return; }
    setLoadingObjects(true);
    ListObjects(pickerDb, pickerSchema)
      .then((objs) => {
        setStageOptions((objs ?? []).filter((o) => o.kind === "STAGE").map((o) => o.name));
        setFormatOptions((objs ?? []).filter((o) => o.kind === "FILE FORMAT").map((o) => o.name));
      })
      .catch(() => { setStageOptions([]); setFormatOptions([]); })
      .finally(() => setLoadingObjects(false));
  }, [pickerDb, pickerSchema]);

  const onPickDb = (v?: string) => { setPickerDb(v ?? ""); setPickerSchema(""); setPickerStage(""); };
  const onPickSchema = (v?: string) => { setPickerSchema(v ?? ""); setPickerStage(""); };

  // Compose @"db"."schema"."stage"[/path] from the stage picker into LOCATION.
  const useStageLocation = () => {
    if (!pickerDb || !pickerSchema || !pickerStage) return;
    const esc = (s: string) => s.replace(/"/g, '""');
    const path = stagePath.trim().replace(/^\/+/, "");
    const base = `@"${esc(pickerDb)}"."${esc(pickerSchema)}"."${esc(pickerStage)}"`;
    set("location", path ? `${base}/${path}` : base);
  };

  const useNamedFormat = (name: string) => {
    set("fileFormatName", name);
    setFmtMode("named");
  };

  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.location.trim().length > 0 &&
    (fmtMode === "type" ? true : cfg.fileFormatName.trim().length > 0);

  const handleRun = async () => {
    if (!canSubmit) return;
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const columnsBody = (
    <Space direction="vertical" size={6} style={{ width: "100%" }}>
      {cfg.columns.length === 0 && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          No columns — the external table will expose only the default <code>VALUE</code> variant column.
        </Text>
      )}
      {cfg.columns.map((c, i) => (
        <Space key={i} align="start" style={{ width: "100%" }} wrap={false}>
          <Input
            size="small"
            placeholder="Column name"
            value={c.name}
            onChange={(e) => updateColumn(i, { name: e.target.value })}
            style={{ width: 150 }}
          />
          <Input
            size="small"
            placeholder="Type"
            value={c.type}
            onChange={(e) => updateColumn(i, { type: e.target.value })}
            style={{ width: 110 }}
          />
          <Input
            size="small"
            placeholder="AS expression — e.g. (value:c1::varchar)"
            value={c.expression}
            onChange={(e) => updateColumn(i, { expression: e.target.value })}
            style={{ width: 280 }}
          />
          <Tooltip title="Include in PARTITION BY">
            <Checkbox
              checked={c.partition}
              onChange={(e) => updateColumn(i, { partition: e.target.checked })}
              style={{ marginTop: 4 }}
            >
              Part
            </Checkbox>
          </Tooltip>
          <Button size="small" type="text" icon={<DeleteOutlined />} onClick={() => removeColumn(i)} />
        </Space>
      ))}
      <Button size="small" icon={<PlusOutlined />} onClick={addColumn}>Add column</Button>
    </Space>
  );

  const advancedBody = (
    <>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Refresh On Create" style={itemStyle} help="Refresh the table metadata immediately after creation">
          <Select
            allowClear
            value={cfg.refreshOnCreate || undefined}
            onChange={(v) => set("refreshOnCreate", v ?? "")}
            placeholder="TRUE (default)"
            style={{ width: "100%" }}
            options={[{ value: "TRUE", label: "TRUE" }, { value: "FALSE", label: "FALSE" }]}
          />
        </Form.Item>
        <Form.Item label="Auto Refresh" style={itemStyle} help="Automatically refresh metadata on new files (requires event notifications)">
          <Select
            allowClear
            value={cfg.autoRefresh || undefined}
            onChange={(v) => set("autoRefresh", v ?? "")}
            placeholder="TRUE (default)"
            style={{ width: "100%" }}
            options={[{ value: "TRUE", label: "TRUE" }, { value: "FALSE", label: "FALSE" }]}
          />
        </Form.Item>
      </div>

      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
        <Form.Item label="Pattern" style={itemStyle} help="Regex matching the file paths to include">
          <Input
            value={cfg.pattern}
            onChange={(e) => set("pattern", e.target.value)}
            placeholder=".*[.]parquet"
          />
        </Form.Item>
        <Form.Item label="AWS SNS Topic" style={itemStyle} help="ARN of the SNS topic for S3 auto-refresh">
          <Input
            value={cfg.awsSnsTopic}
            onChange={(e) => set("awsSnsTopic", e.target.value)}
            placeholder="arn:aws:sns:…"
          />
        </Form.Item>
      </div>

      <Form.Item style={{ marginBottom: 8 }}>
        <Checkbox checked={cfg.copyGrants} onChange={(e) => set("copyGrants", e.target.checked)}>
          COPY GRANTS
        </Checkbox>
      </Form.Item>

      <Form.Item label="Tags" style={itemStyle} help="Table-level tags applied at creation">
        <Space direction="vertical" size={6} style={{ width: "100%" }}>
          {(cfg.tags ?? []).length > 0 && (
            <div style={{ display: "flex", flexWrap: "wrap", gap: 6 }}>
              {(cfg.tags ?? []).map((t) => (
                <Tag key={t.name} closable onClose={(e) => { e.preventDefault(); removeTag(t.name); }}>
                  {t.name}{t.value ? `: ${t.value}` : ""}
                </Tag>
              ))}
            </div>
          )}
          <Space>
            <Input size="small" value={tagName} onChange={(e) => setTagName(e.target.value)} placeholder="Tag name" style={{ width: 160 }} />
            <Input size="small" value={tagValue} onChange={(e) => setTagValue(e.target.value)} placeholder="Tag value" style={{ width: 180 }} onPressEnter={addTag} />
            <Button size="small" icon={<PlusOutlined />} onClick={addTag} disabled={!tagName.trim()}>Add</Button>
          </Space>
        </Space>
      </Form.Item>
    </>
  );

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <CloudServerOutlined style={{ color: "var(--link)" }} />
          <span>Create External Table</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={creating}>Cancel</Button>
          <Button
            type="primary"
            icon={<CloudServerOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={760}
      styles={{ body: { paddingTop: 16, maxHeight: "80vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="External table creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="External table name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_EXTERNAL_TABLE"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Space direction="vertical" size={4}>
              <Checkbox
                checked={cfg.orReplace}
                onChange={(e) => {
                  set("orReplace", e.target.checked);
                  if (e.target.checked) set("ifNotExists", false);
                }}
              >
                OR REPLACE
              </Checkbox>
              <Checkbox
                checked={cfg.ifNotExists}
                disabled={cfg.orReplace}
                onChange={(e) => set("ifNotExists", e.target.checked)}
              >
                IF NOT EXISTS
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

        {/* Location */}
        <Form.Item label="Location" required style={itemStyle} help="External stage and optional path holding the data files">
          <Input
            value={cfg.location}
            onChange={(e) => set("location", e.target.value)}
            placeholder="@my_stage/path/"
          />
          <div style={{ display: "flex", gap: 8, marginTop: 8, flexWrap: "wrap", alignItems: "center" }}>
            <Text type="secondary" style={{ fontSize: 11 }}>Pick a stage:</Text>
            <Select
              size="small" showSearch placeholder="Database" style={{ width: 140 }}
              value={pickerDb || undefined} onChange={onPickDb}
              options={dbOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              size="small" showSearch placeholder="Schema" style={{ width: 140 }}
              value={pickerSchema || undefined} onChange={onPickSchema} disabled={!pickerDb}
              loading={loadingSchemas}
              options={schemaOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              size="small" showSearch placeholder="Stage" style={{ width: 150 }}
              value={pickerStage || undefined} onChange={(v) => setPickerStage(v ?? "")}
              disabled={!pickerSchema} loading={loadingObjects}
              options={stageOptions.map((n) => ({ value: n, label: n }))}
              notFoundContent={loadingObjects ? "Loading…" : "No stages"}
            />
            <Input
              size="small" placeholder="path/" style={{ width: 120 }}
              value={stagePath} onChange={(e) => setStagePath(e.target.value)}
            />
            <Button size="small" onClick={useStageLocation} disabled={!pickerStage}>Use stage</Button>
          </div>
        </Form.Item>

        {/* File format */}
        <Form.Item label="File Format" required style={itemStyle}>
          <Radio.Group
            value={fmtMode}
            onChange={(e) => setFmtMode(e.target.value)}
            optionType="button"
            buttonStyle="solid"
            size="small"
            options={[{ value: "type", label: "Inline type" }, { value: "named", label: "Named format" }]}
          />
          {fmtMode === "type" ? (
            <div style={{ marginTop: 8 }}>
              <Select
                value={cfg.fileFormatType || "CSV"}
                onChange={(v) => set("fileFormatType", v)}
                style={{ width: 200 }}
                options={FILE_FORMAT_TYPES.map((t) => ({ value: t, label: t }))}
              />
            </div>
          ) : (
            <div style={{ display: "flex", gap: 8, marginTop: 8, flexWrap: "wrap", alignItems: "center" }}>
              <Input
                placeholder="FORMAT_NAME"
                value={cfg.fileFormatName}
                onChange={(e) => set("fileFormatName", e.target.value)}
                style={{ width: 240 }}
              />
              <Text type="secondary" style={{ fontSize: 11 }}>or pick:</Text>
              <Select
                size="small" showSearch placeholder="File format" style={{ width: 200 }}
                value={undefined} onChange={(v) => v && useNamedFormat(v)}
                disabled={!pickerSchema} loading={loadingObjects}
                options={formatOptions.map((n) => ({ value: n, label: n }))}
                notFoundContent={loadingObjects ? "Loading…" : "Pick a database/schema above"}
              />
            </div>
          )}
        </Form.Item>

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[
            { key: "columns", label: `Columns${cfg.columns.length ? ` (${cfg.columns.length})` : ""}`, children: columnsBody },
            { key: "advanced", label: "Advanced options", children: advancedBody },
          ]}
        />

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <div
          style={{
            padding: "10px 12px",
            background: "var(--bg)",
            borderRadius: 6,
            border: "1px solid var(--border)",
            marginTop: 4,
          }}
        >
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            SQL Preview
          </Text>
          <pre
            style={{
              margin: 0,
              color: "var(--text)",
              fontSize: 11,
              fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {preview}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
