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
  Typography, Button, Alert, Collapse, Tag, Radio, Tooltip, Breadcrumb, Spin, Empty,
} from "antd";
import {
  CloudServerOutlined, PlusOutlined, DeleteOutlined,
  FolderOutlined, FileOutlined, DatabaseOutlined, ReloadOutlined,
} from "@ant-design/icons";
import {
  BuildCreateExternalTableSql, ExecDDL, GetQuotedIdentifiersIgnoreCase,
  ListDatabases, ListSchemas, ListObjects, ListStages, ListStageEntries,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import { formatBytes } from "../../utils/formatBytes";
import type { externaltable, snowflake } from "../../../wailsjs/go/models";

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
  const [stageOptions, setStageOptions] = useState<{ name: string; url: string }[]>([]);
  const [formatOptions, setFormatOptions] = useState<string[]>([]);
  const [pickerStage, setPickerStage] = useState("");
  const [loadingSchemas, setLoadingSchemas] = useState(false);
  const [loadingObjects, setLoadingObjects] = useState(false);

  // Stage browser: the directory currently being browsed (relative to the stage
  // root, "" = root) and its immediate entries. Navigating keeps the editable
  // Location field in sync.
  const [browsePath, setBrowsePath] = useState("");
  const [entries, setEntries] = useState<snowflake.GitRepoEntry[]>([]);
  const [loadingEntries, setLoadingEntries] = useState(false);
  const [browseError, setBrowseError] = useState<string | null>(null);
  // Bumped to force a re-fetch of the current directory even when browsePath is
  // unchanged (the "Reload entries" button).
  const [reloadTick, setReloadTick] = useState(0);

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
    Promise.all([
      ListStages(pickerDb, pickerSchema).catch(() => []),
      ListObjects(pickerDb, pickerSchema).catch(() => []),
    ])
      .then(([stages, objs]) => {
        // External tables may only reference an EXTERNAL stage, so filter out
        // INTERNAL stages from the picker.
        setStageOptions((stages ?? []).filter((s) => s.type === "EXTERNAL").map((s) => ({ name: s.name, url: s.url })));
        setFormatOptions((objs ?? []).filter((o) => o.kind === "FILE FORMAT").map((o) => o.name));
      })
      .finally(() => setLoadingObjects(false));
  }, [pickerDb, pickerSchema]);

  const resetBrowse = () => { setBrowsePath(""); setEntries([]); setBrowseError(null); };
  const onPickDb = (v?: string) => { setPickerDb(v ?? ""); setPickerSchema(""); setPickerStage(""); resetBrowse(); };
  const onPickSchema = (v?: string) => { setPickerSchema(v ?? ""); setPickerStage(""); resetBrowse(); };

  // Compose @"db"."schema"."stage"[/path] for the given directory path.
  const composeLocation = (path: string) => {
    const esc = (s: string) => s.replace(/"/g, '""');
    const base = `@"${esc(pickerDb)}"."${esc(pickerSchema)}"."${esc(pickerStage)}"`;
    const p = path.replace(/^\/+/, "").replace(/\/+$/, "");
    return p ? `${base}/${p}` : base;
  };

  // Selecting a stage starts a browse session at its root and points LOCATION at
  // the stage root.
  const onPickStage = (v?: string) => {
    const stage = v ?? "";
    setPickerStage(stage);
    setBrowsePath("");
    setBrowseError(null);
    if (stage) {
      const esc = (s: string) => s.replace(/"/g, '""');
      set("location", `@"${esc(pickerDb)}"."${esc(pickerSchema)}"."${esc(stage)}"`);
    }
  };

  // Navigate to a directory inside the browsed stage; keeps LOCATION in sync.
  const navigateTo = (path: string) => {
    setBrowsePath(path);
    set("location", composeLocation(path));
  };

  // Load the immediate entries of the current browse path whenever it changes.
  useEffect(() => {
    if (!pickerDb || !pickerSchema || !pickerStage) { setEntries([]); return; }
    let cancelled = false;
    setLoadingEntries(true);
    setBrowseError(null);
    ListStageEntries(pickerDb, pickerSchema, pickerStage, browsePath)
      .then((e) => { if (!cancelled) setEntries(e ?? []); })
      .catch((err) => { if (!cancelled) { setEntries([]); setBrowseError(String(err)); } })
      .finally(() => { if (!cancelled) setLoadingEntries(false); });
    return () => { cancelled = true; };
  }, [pickerDb, pickerSchema, pickerStage, browsePath, reloadTick]);

  // Breadcrumb segments for the current browse path (each carries the cumulative
  // path to navigate to when clicked).
  const pathSegments = browsePath.split("/").filter(Boolean);
  const segmentPaths = pathSegments.map((_, i) => pathSegments.slice(0, i + 1).join("/") + "/");

  // FORMAT_NAME is resolved relative to the external table's own schema
  // (db.schema). When the picked format lives in a different schema, store the
  // fully-qualified (quoted) name so creation references the right object.
  const useNamedFormat = (name: string) => {
    const esc = (s: string) => s.replace(/"/g, '""');
    const sameSchema = pickerDb === db && pickerSchema === schema;
    set(
      "fileFormatName",
      sameSchema ? name : `"${esc(pickerDb)}"."${esc(pickerSchema)}"."${esc(name)}"`,
    );
    setFmtMode("named");
  };

  // Switching to "named" clears the inline TYPE so the builder emits a
  // FORMAT_NAME placeholder (instead of a contradictory TYPE) until a format is
  // chosen; switching back restores a concrete default.
  const changeFmtMode = (mode: "type" | "named") => {
    setFmtMode(mode);
    if (mode === "named") {
      set("fileFormatType", "");
    } else if (!cfg.fileFormatType.trim()) {
      set("fileFormatType", "CSV");
    }
  };

  // Partition columns must carry a metadata-derived expression; an empty one
  // makes the builder emit `AS (value)`, which Snowflake rejects in PARTITION BY.
  // Surfaced as an inline warning AND a hard block on submit, since it's a
  // guaranteed-fail DDL the UI already knows about.
  const partitionColsMissingExpr = cfg.columns.filter(
    (c) => c.partition && c.name.trim() !== "" && c.expression.trim() === "",
  );

  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.location.trim().length > 0 &&
    (fmtMode === "type" ? true : cfg.fileFormatName.trim().length > 0) &&
    partitionColsMissingExpr.length === 0;

  const handleRun = async () => {
    if (!canSubmit) return;
    setCreating(true);
    setCreateError(null);
    try {
      // Build the statement from the current cfg at submit time rather than
      // reusing the `preview` state, which is refreshed by an async effect and
      // can lag a keystroke behind the latest cfg.
      const sql = await BuildCreateExternalTableSql(db, schema, cfg as any);
      await ExecDDL(sql);
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
      {partitionColsMissingExpr.length > 0 && (
        <Alert
          type="warning"
          showIcon
          style={{ padding: "4px 10px" }}
          message={
            <span style={{ fontSize: 11 }}>
              Partition {partitionColsMissingExpr.length === 1 ? "column" : "columns"}{" "}
              {partitionColsMissingExpr.map((c) => c.name.trim()).join(", ")} {partitionColsMissingExpr.length === 1 ? "needs" : "need"} an
              expression (e.g. <code>metadata$filename</code>-derived) — Snowflake rejects a partition column defined as <code>AS (value)</code>.
            </span>
          }
        />
      )}
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
        <Form.Item label="Location" required style={itemStyle} help="External stage and optional path holding the data files — browse below or edit directly">
          <Input
            value={cfg.location}
            onChange={(e) => set("location", e.target.value)}
            placeholder="@my_stage/path/"
          />
          <div style={{ display: "flex", gap: 8, marginTop: 8, flexWrap: "wrap", alignItems: "center" }}>
            <Text type="secondary" style={{ fontSize: 11 }}>Browse a stage:</Text>
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
              value={pickerStage || undefined} onChange={onPickStage}
              disabled={!pickerSchema} loading={loadingObjects}
              options={stageOptions.map((s) => ({ value: s.name, label: s.name, url: s.url }))}
              // Surface each external stage's storage URL in the dropdown so two
              // stages pointing at different buckets are distinguishable; the
              // selected control still shows just the name.
              optionRender={(opt) => (
                <Space direction="vertical" size={0}>
                  <span>{opt.data.value}</span>
                  {opt.data.url && <Text type="secondary" style={{ fontSize: 11 }}>{opt.data.url}</Text>}
                </Space>
              )}
              notFoundContent={loadingObjects ? "Loading…" : "No external stages"}
            />
            {pickerStage && (
              <Tooltip title="Reload entries">
                <Button size="small" icon={<ReloadOutlined />} loading={loadingEntries} onClick={() => setReloadTick((t) => t + 1)} />
              </Tooltip>
            )}
          </div>

          {pickerStage && (
            <div
              style={{
                marginTop: 8,
                border: "1px solid var(--border)",
                borderRadius: 6,
                overflow: "hidden",
              }}
            >
              {/* Breadcrumb */}
              <div style={{ padding: "6px 10px", background: "var(--bg)", borderBottom: "1px solid var(--border)" }}>
                <Breadcrumb
                  separator="/"
                  items={[
                    {
                      title: (
                        <a onClick={(e) => { e.preventDefault(); navigateTo(""); }}>
                          <DatabaseOutlined style={{ marginRight: 4 }} />{pickerStage}
                        </a>
                      ),
                    },
                    ...pathSegments.map((seg, i) => ({
                      title:
                        i === pathSegments.length - 1
                          ? <span>{seg}</span>
                          : <a onClick={(e) => { e.preventDefault(); navigateTo(segmentPaths[i]); }}>{seg}</a>,
                    })),
                  ]}
                />
              </div>

              {/* Entry list */}
              <div style={{ maxHeight: 180, overflowY: "auto", padding: "4px 0" }}>
                {loadingEntries ? (
                  <div style={{ textAlign: "center", padding: 20 }}><Spin size="small" /></div>
                ) : browseError ? (
                  <div style={{ padding: "8px 12px" }}>
                    <Text type="danger" style={{ fontSize: 11 }}>{browseError}</Text>
                  </div>
                ) : entries.length === 0 ? (
                  <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="Empty folder" style={{ margin: "12px 0" }} />
                ) : (
                  entries.map((en) => (
                    <div
                      key={en.path}
                      onClick={() => { if (en.isDir) navigateTo(en.path); }}
                      style={{
                        display: "flex", alignItems: "center", gap: 8,
                        padding: "4px 12px", fontSize: 12,
                        cursor: en.isDir ? "pointer" : "default",
                        color: en.isDir ? "var(--text)" : "var(--text-muted)",
                      }}
                      onMouseEnter={(e) => { if (en.isDir) (e.currentTarget.style.background = "var(--bg-hover)"); }}
                      onMouseLeave={(e) => { e.currentTarget.style.background = "transparent"; }}
                    >
                      {en.isDir
                        ? <FolderOutlined style={{ color: "var(--icon-stage)" }} />
                        : <FileOutlined style={{ color: "var(--text-muted)" }} />}
                      <span style={{ flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{en.name}</span>
                      {!en.isDir && (en.size ?? 0) > 0 && (
                        <Text type="secondary" style={{ fontSize: 11 }}>{formatBytes(en.size ?? 0)}</Text>
                      )}
                    </div>
                  ))
                )}
              </div>
            </div>
          )}
        </Form.Item>

        {/* File format */}
        <Form.Item label="File Format" required style={itemStyle}>
          <Radio.Group
            value={fmtMode}
            onChange={(e) => changeFmtMode(e.target.value)}
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
