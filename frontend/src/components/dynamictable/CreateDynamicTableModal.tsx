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

import { useState, useEffect, useRef } from "react";
import {
  Modal, Form, Input, InputNumber, Checkbox, Select, Space,
  Typography, Button, Alert, Collapse, Tag, Radio,
} from "antd";
import { RetweetOutlined, PlusOutlined } from "@ant-design/icons";
import {
  BuildCreateDynamicTableSql, ExecDDL, GetQuotedIdentifiersIgnoreCase, ListWarehouses,
  ListDatabases, ListSchemas, ListBasicObjects, GetTableColumns,
} from "../../../wailsjs/go/app/App";
import type * as monaco from "monaco-editor";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import type { dynamictable } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

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
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

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

  // New-tag draft inputs.
  const [tagName, setTagName] = useState("");
  const [tagValue, setTagValue] = useState("");

  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  // "Insert from table" picker — generates the same SELECT as a drag-and-drop
  // from the object store, inserted at the cursor in the query editor.
  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const [pickerDb, setPickerDb] = useState(db);
  const [pickerSchema, setPickerSchema] = useState(schema);
  const [pickerTable, setPickerTable] = useState("");
  const [dbOptions, setDbOptions] = useState<string[]>([]);
  const [schemaOptions, setSchemaOptions] = useState<string[]>([]);
  const [tableOptions, setTableOptions] = useState<string[]>([]);
  const [loadingSchemas, setLoadingSchemas] = useState(false);
  const [loadingTables, setLoadingTables] = useState(false);
  const [inserting, setInserting] = useState(false);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});

    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  useEffect(() => {
    BuildCreateDynamicTableSql(db, schema, cfg as any).then(setPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = <K extends keyof DTConfig>(key: K, value: DTConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Recompute the TARGET_LAG string whenever the composer inputs change.
  useEffect(() => {
    const composed = lagMode === "downstream" ? "DOWNSTREAM" : `${lagNum} ${lagUnit}`;
    set("targetLag", composed);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lagMode, lagNum, lagUnit]);

  const addTag = () => {
    const name = tagName.trim();
    if (!name) return;
    set("tags", [...(cfg.tags ?? []).filter((t) => t.name !== name), { name, value: tagValue.trim() }]);
    setTagName("");
    setTagValue("");
  };

  const removeTag = (name: string) => set("tags", (cfg.tags ?? []).filter((t) => t.name !== name));

  // Load databases once; schemas/tables react to the current picker selection.
  useEffect(() => {
    ListDatabases().then((d) => setDbOptions(d ?? [])).catch(() => {});
  }, []);

  useEffect(() => {
    if (!pickerDb) { setSchemaOptions([]); return; }
    setLoadingSchemas(true);
    ListSchemas(pickerDb)
      .then((s) => setSchemaOptions(s ?? []))
      .catch(() => setSchemaOptions([]))
      .finally(() => setLoadingSchemas(false));
  }, [pickerDb]);

  useEffect(() => {
    if (!pickerDb || !pickerSchema) { setTableOptions([]); return; }
    setLoadingTables(true);
    ListBasicObjects(pickerDb, pickerSchema)
      .then((objs) => setTableOptions(
        (objs ?? []).filter((o) => o.kind === "TABLE" || o.kind === "VIEW").map((o) => o.name),
      ))
      .catch(() => setTableOptions([]))
      .finally(() => setLoadingTables(false));
  }, [pickerDb, pickerSchema]);

  const onPickDb = (v?: string) => {
    setPickerDb(v ?? "");
    setPickerSchema("");
    setPickerTable("");
  };
  const onPickSchema = (v?: string) => {
    setPickerSchema(v ?? "");
    setPickerTable("");
  };

  // Builds the same SELECT a table drag-and-drop into the SQL editor produces:
  // every column double-quoted (one per line) from the 3-part FQN, falling back
  // to SELECT * if the column list can't be fetched.
  const insertSelect = async () => {
    if (!pickerDb || !pickerSchema || !pickerTable) return;
    setInserting(true);
    try {
      const esc = (s: string) => s.replace(/"/g, '""');
      const fqn = `"${esc(pickerDb)}"."${esc(pickerSchema)}"."${esc(pickerTable)}"`;
      let snippet: string;
      try {
        const cols = await GetTableColumns(pickerDb, pickerSchema, pickerTable);
        const colList = (cols ?? []).map((c) => `    "${esc(c)}"`).join(",\n");
        snippet = colList ? `SELECT\n${colList}\nFROM ${fqn}` : `SELECT *\nFROM ${fqn}`;
      } catch {
        snippet = `SELECT *\nFROM ${fqn}`;
      }

      const ed = editorRef.current;
      const current = (ed?.getValue() ?? cfg.query).trim();
      // Replace the whole body when it's empty or still the placeholder;
      // otherwise insert at the cursor so multi-table queries can be assembled.
      if (!current || current === DEFAULT_QUERY.trim()) {
        // Controlled `value` prop pushes this into the editor model on re-render.
        set("query", snippet);
      } else if (ed) {
        const sel = ed.getSelection();
        const pos = ed.getPosition() ?? { lineNumber: 1, column: 1 };
        const range = sel ?? {
          startLineNumber: pos.lineNumber, startColumn: pos.column,
          endLineNumber: pos.lineNumber, endColumn: pos.column,
        };
        ed.executeEdits("insert-select", [{ range, text: snippet, forceMoveMarkers: true }]);
        ed.pushUndoStop();
        ed.focus();
      } else {
        set("query", `${cfg.query}\n${snippet}`);
      }
    } finally {
      setInserting(false);
    }
  };

  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.warehouse.trim().length > 0 &&
    cfg.targetLag.trim().length > 0 &&
    cfg.query.trim().length > 0;

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
            <Input
              size="small"
              value={tagName}
              onChange={(e) => setTagName(e.target.value)}
              placeholder="Tag name"
              style={{ width: 160 }}
            />
            <Input
              size="small"
              value={tagValue}
              onChange={(e) => setTagValue(e.target.value)}
              placeholder="Tag value"
              style={{ width: 180 }}
              onPressEnter={addTag}
            />
            <Button size="small" icon={<PlusOutlined />} onClick={addTag} disabled={!tagName.trim()}>
              Add
            </Button>
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
          <RetweetOutlined style={{ color: "var(--link)" }} />
          <span>Create Dynamic Table</span>
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
            icon={<RetweetOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={720}
      styles={{ body: { paddingTop: 16, maxHeight: "80vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Dynamic table creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Dynamic table name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_DYNAMIC_TABLE"
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
              <Checkbox
                checked={cfg.transient}
                onChange={(e) => set("transient", e.target.checked)}
              >
                TRANSIENT
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
                  min={1}
                  value={lagNum}
                  onChange={(v) => setLagNum(v ?? 1)}
                  style={{ width: 90 }}
                />
                <Select
                  value={lagUnit}
                  onChange={(v) => setLagUnit(v)}
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

        <Form.Item label="Defining Query (AS)" required style={itemStyle}>
          <div style={{ display: "flex", gap: 8, marginBottom: 8, flexWrap: "wrap", alignItems: "center" }}>
            <Text type="secondary" style={{ fontSize: 11 }}>Insert from table:</Text>
            <Select
              size="small"
              showSearch
              placeholder="Database"
              style={{ width: 150 }}
              value={pickerDb || undefined}
              onChange={onPickDb}
              options={dbOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              size="small"
              showSearch
              placeholder="Schema"
              style={{ width: 150 }}
              value={pickerSchema || undefined}
              onChange={onPickSchema}
              disabled={!pickerDb}
              loading={loadingSchemas}
              options={schemaOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              size="small"
              showSearch
              placeholder="Table / View"
              style={{ width: 180 }}
              value={pickerTable || undefined}
              onChange={(v) => setPickerTable(v ?? "")}
              disabled={!pickerSchema}
              loading={loadingTables}
              options={tableOptions.map((n) => ({ value: n, label: n }))}
              notFoundContent={loadingTables ? "Loading…" : "No tables/views"}
            />
            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={insertSelect}
              loading={inserting}
              disabled={!pickerTable}
            >
              Insert SELECT
            </Button>
          </div>
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={140}
              language="sql"
              theme={editorTheme}
              value={cfg.query}
              onChange={(v) => set("query", v ?? "")}
              onMount={(editor) => { editorRef.current = editor; patchMonacoClipboard(editor); }}
              options={{
                minimap: { enabled: false },
                lineNumbers: "off",
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
              }}
            />
          </div>
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
