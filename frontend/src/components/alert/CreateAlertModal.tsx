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
  Modal, Form, Input, Checkbox, Select, Space,
  Typography, Button, Alert, Collapse, Tag,
} from "antd";
import { AlertOutlined, PlusOutlined } from "@ant-design/icons";
import {
  BuildCreateAlertSql, ExecDDL, GetQuotedIdentifiersIgnoreCase,
  ListWarehouses, ListDatabases, ListSchemas, ListObjects, GetTableColumns,
} from "../../../wailsjs/go/app/App";
import type * as monaco from "monaco-editor";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import type { alert as alertModels } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import CallProcedureModal from "../procedure/CallProcedureModal";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_CONDITION = "SELECT *\n  FROM my_source_table\n  WHERE my_metric > 100";
const DEFAULT_ACTION = "INSERT INTO my_alert_log\n  SELECT CURRENT_TIMESTAMP()";

// Plain data shape for form state. The Wails-generated `AlertConfig` class
// carries a `convertValues` method (it has a nested `tags` array), which a plain
// object literal can't satisfy; we cast to the generated type only at the IPC
// boundary (`cfg as any`).
type AlertCfg = Omit<alertModels.AlertConfig, "convertValues" | "tags"> & {
  tags: { name: string; value: string }[];
};

export default function CreateAlertModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<AlertCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    warehouse: "",
    schedule: "60 MINUTE",
    comment: "",
    tags: [],
    condition: DEFAULT_CONDITION,
    action: DEFAULT_ACTION,
  });

  // New-tag draft inputs.
  const [tagName, setTagName] = useState("");
  const [tagValue, setTagValue] = useState("");

  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");
  const [warehouseOptions, setWarehouseOptions] = useState<string[]>([]);

  // "Insert from table" picker for the condition editor — generates the same
  // SELECT as a drag-and-drop from the object store, inserted at the cursor.
  const conditionEditorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  const [pickerDb, setPickerDb] = useState(db);
  const [pickerSchema, setPickerSchema] = useState(schema);
  const [pickerTable, setPickerTable] = useState("");
  const [dbOptions, setDbOptions] = useState<string[]>([]);
  const [schemaOptions, setSchemaOptions] = useState<string[]>([]);
  const [tableOptions, setTableOptions] = useState<string[]>([]);
  const [loadingSchemas, setLoadingSchemas] = useState(false);
  const [loadingTables, setLoadingTables] = useState(false);
  const [inserting, setInserting] = useState(false);

  // "Insert CALL procedure" picker — shares the database/schema selection with
  // the table picker; each option carries the procedure's overload signature so
  // the reused CallProcedureModal can resolve the correct overload.
  const [procOptions, setProcOptions] = useState<{ name: string; args: string }[]>([]);
  const [pickerProcIdx, setPickerProcIdx] = useState<string>("");
  // The procedure whose parameters are being filled in the CallProcedureModal.
  const [callModal, setCallModal] = useState<{ db: string; schema: string; name: string; rawArgs: string } | null>(null);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
    ListWarehouses().then((w) => setWarehouseOptions(w ?? [])).catch(() => {});
  }, []);

  useEffect(() => {
    BuildCreateAlertSql(db, schema, cfg as any).then(setPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = <K extends keyof AlertCfg>(key: K, value: AlertCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

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
    if (!pickerDb || !pickerSchema) { setTableOptions([]); setProcOptions([]); return; }
    setLoadingTables(true);
    ListObjects(pickerDb, pickerSchema)
      .then((objs) => {
        const list = objs ?? [];
        setTableOptions(
          list.filter((o) => o.kind === "TABLE" || o.kind === "VIEW").map((o) => o.name),
        );
        // An alert condition may be a CALL to a stored procedure; carry each
        // procedure's overload signature so the reused CallProcedureModal can
        // resolve the right overload.
        setProcOptions(
          list
            .filter((o) => o.kind === "PROCEDURE")
            .map((o) => ({ name: o.name, args: o.arguments ?? "" })),
        );
      })
      .catch(() => { setTableOptions([]); setProcOptions([]); })
      .finally(() => setLoadingTables(false));
  }, [pickerDb, pickerSchema]);

  const onPickDb = (v?: string) => {
    setPickerDb(v ?? "");
    setPickerSchema("");
    setPickerTable("");
    setPickerProcIdx("");
  };
  const onPickSchema = (v?: string) => {
    setPickerSchema(v ?? "");
    setPickerTable("");
    setPickerProcIdx("");
  };

  // Drops a snippet into the condition editor: replaces the whole body when it's
  // empty or still the placeholder, otherwise inserts at the cursor. Shared by
  // the table SELECT picker and the procedure CALL picker.
  const insertIntoCondition = (snippet: string, editId: string) => {
    const ed = conditionEditorRef.current;
    const current = (ed?.getValue() ?? cfg.condition).trim();
    if (!current || current === DEFAULT_CONDITION.trim()) {
      set("condition", snippet);
    } else if (ed) {
      const sel = ed.getSelection();
      const pos = ed.getPosition() ?? { lineNumber: 1, column: 1 };
      const range = sel ?? {
        startLineNumber: pos.lineNumber, startColumn: pos.column,
        endLineNumber: pos.lineNumber, endColumn: pos.column,
      };
      ed.executeEdits(editId, [{ range, text: snippet, forceMoveMarkers: true }]);
      ed.pushUndoStop();
      ed.focus();
    } else {
      set("condition", `${cfg.condition}\n${snippet}`);
    }
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
      insertIntoCondition(snippet, "insert-select");
    } finally {
      setInserting(false);
    }
  };

  // Opens the shared CallProcedureModal for the selected procedure; its
  // onInsert callback drops the built CALL statement into the condition editor.
  const openCallProcedure = () => {
    if (!pickerDb || !pickerSchema || pickerProcIdx === "") return;
    const proc = procOptions[Number(pickerProcIdx)];
    if (!proc) return;
    setCallModal({ db: pickerDb, schema: pickerSchema, name: proc.name, rawArgs: proc.args });
  };

  const onCallInsert = (sql: string) => {
    // The CALL goes inside IF (EXISTS (…)); strip the statement terminator the
    // builder appends so it reads naturally in the condition context.
    insertIntoCondition(sql.trim().replace(/;\s*$/, ""), "insert-call");
  };

  // The editors seed placeholder templates; treat the untouched placeholders as
  // "not ready" so Create can't fire a statement that references the obviously
  // fake objects and fails server-side.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.schedule.trim().length > 0 &&
    cfg.condition.trim().length > 0 &&
    cfg.condition.trim() !== DEFAULT_CONDITION.trim() &&
    cfg.action.trim().length > 0 &&
    cfg.action.trim() !== DEFAULT_ACTION.trim();

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

  const advancedBody = (
    <Form.Item label="Tags" style={{ marginBottom: 4 }} help="Alert-level tags applied at creation">
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
  );

  return (
    <>
    <Modal
      open
      title={
        <Space size={6}>
          <AlertOutlined style={{ color: "var(--link)" }} />
          <span>Create Alert</span>
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
            icon={<AlertOutlined />}
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
          message="Alert creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Alert name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_ALERT"
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

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item
            label="Schedule"
            required
            style={itemStyle}
            help="e.g. 60 MINUTE or USING CRON 0 9 * * * UTC"
          >
            <Input
              value={cfg.schedule}
              onChange={(e) => set("schedule", e.target.value)}
              placeholder="60 MINUTE"
            />
          </Form.Item>
          <Form.Item
            label="Warehouse"
            style={itemStyle}
            help="Leave empty for a serverless alert"
          >
            <Select
              showSearch
              allowClear
              placeholder="(serverless)"
              value={cfg.warehouse || undefined}
              onChange={(v) => set("warehouse", v ?? "")}
              options={warehouseOptions.map((n) => ({ value: n, label: n }))}
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

        <Form.Item label="Condition — IF (EXISTS (…))" required style={itemStyle} help="The alert fires when this query returns at least one row">
          <div style={{ display: "flex", gap: 8, marginBottom: 8, flexWrap: "wrap", alignItems: "center" }}>
            <Text type="secondary" style={{ fontSize: 11 }}>Insert from table:</Text>
            <Select
              size="small"
              showSearch
              placeholder="Database"
              style={{ width: 140 }}
              value={pickerDb || undefined}
              onChange={onPickDb}
              options={dbOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              size="small"
              showSearch
              placeholder="Schema"
              style={{ width: 140 }}
              value={pickerSchema || undefined}
              onChange={onPickSchema}
              disabled={!pickerDb}
              loading={loadingSchemas}
              options={schemaOptions.map((n) => ({ value: n, label: n }))}
            />
            <Select
              size="small"
              showSearch
              placeholder="Table / view"
              style={{ width: 160 }}
              value={pickerTable || undefined}
              onChange={(v) => setPickerTable(v ?? "")}
              disabled={!pickerSchema}
              loading={loadingTables}
              options={tableOptions.map((n) => ({ value: n, label: n }))}
              notFoundContent={loadingTables ? "Loading…" : "No tables or views"}
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
          <div style={{ display: "flex", gap: 8, marginBottom: 8, flexWrap: "wrap", alignItems: "center" }}>
            <Text type="secondary" style={{ fontSize: 11 }}>Insert CALL procedure:</Text>
            <Select
              size="small"
              showSearch
              placeholder="Procedure"
              style={{ width: 280 }}
              value={pickerProcIdx || undefined}
              onChange={(v) => setPickerProcIdx(v ?? "")}
              disabled={!pickerSchema}
              loading={loadingTables}
              options={procOptions.map((p, i) => ({
                value: String(i),
                label: p.args ? `${p.name}(${p.args})` : `${p.name}()`,
              }))}
              notFoundContent={loadingTables ? "Loading…" : "No procedures"}
            />
            <Button
              size="small"
              icon={<PlusOutlined />}
              onClick={openCallProcedure}
              disabled={pickerProcIdx === ""}
            >
              Insert CALL…
            </Button>
            <Text type="secondary" style={{ fontSize: 10 }}>
              (uses the same database / schema selected above)
            </Text>
          </div>
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={120}
              language="sql"
              theme={editorTheme}
              value={cfg.condition}
              onChange={(v) => set("condition", v ?? "")}
              onMount={(editor) => { conditionEditorRef.current = editor; patchMonacoClipboard(editor); }}
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

        <Form.Item label="Action — THEN" required style={itemStyle} help="The statement executed when the condition is met">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={100}
              language="sql"
              theme={editorTheme}
              value={cfg.action}
              onChange={(v) => set("action", v ?? "")}
              onMount={(editor) => { patchMonacoClipboard(editor); }}
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

    {callModal && (
      <CallProcedureModal
        db={callModal.db}
        schema={callModal.schema}
        name={callModal.name}
        rawArgs={callModal.rawArgs}
        onClose={() => setCallModal(null)}
        onInsert={onCallInsert}
      />
    )}
    </>
  );
}
