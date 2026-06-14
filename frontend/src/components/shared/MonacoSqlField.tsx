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

import { useState, useEffect, useRef, useCallback } from "react";
import type { ReactNode } from "react";
import { Form, Select, Button, Typography } from "antd";
import { PlusOutlined } from "@ant-design/icons";
import type * as monaco from "monaco-editor";
import Editor from "@monaco-editor/react";
import {
  ListDatabases, ListSchemas, ListObjects, GetTableColumns,
} from "../../../wailsjs/go/app/App";
import type { snowflake } from "../../../wailsjs/go/models";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

/** Context handed to `extraPickerRow` so callers can build additional insert UIs. */
export interface ExtraPickerCtx {
  db: string;
  schema: string;
  /** All objects in the selected db/schema (unfiltered). */
  objects: snowflake.SnowflakeObject[];
  loading: boolean;
  /** Insert a snippet: replaces an untouched placeholder body, else inserts at the cursor. */
  insert: (snippet: string, editId: string) => void;
}

interface Props {
  label: string;
  required?: boolean;
  help?: string;
  value: string;
  onChange: (v: string) => void;
  /** Template text; a body still equal to this is replaced wholesale on insert. */
  placeholder: string;
  height?: number;
  /** Object kinds shown in the table picker, e.g. ["TABLE", "VIEW"]. */
  objectKinds: string[];
  defaultDb: string;
  defaultSchema: string;
  tablePlaceholder?: string;
  notFoundText?: string;
  /** Render extra picker rows (e.g. an "Insert CALL procedure" row). */
  extraPickerRow?: (ctx: ExtraPickerCtx) => ReactNode;
  /**
   * Fired when the picker's source database or schema changes — lets the parent
   * reset any selection state it owns that depends on the source (e.g. an
   * `extraPickerRow` procedure index).
   */
  onSourceChange?: () => void;
  itemStyle?: React.CSSProperties;
}

const esc = (s: string) => s.replace(/"/g, '""');

/**
 * A SQL editor field with a built-in "Insert from table" source picker. Picking
 * a database / schema / table and clicking Insert SELECT drops a fully-qualified,
 * column-listed SELECT (the same shape a drag-and-drop into the editor produces)
 * into the Monaco editor — replacing the body when it's still the placeholder,
 * otherwise inserting at the cursor. Used by the materialized-view, dynamic-table
 * and alert create modals; `extraPickerRow` lets the alert modal add its
 * procedure-CALL picker.
 */
export default function MonacoSqlField({
  label,
  required,
  help,
  value,
  onChange,
  placeholder,
  height = 140,
  objectKinds,
  defaultDb,
  defaultSchema,
  tablePlaceholder = "Table / view",
  notFoundText = "No tables or views",
  extraPickerRow,
  onSourceChange,
  itemStyle,
}: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const editorRef = useRef<monaco.editor.IStandaloneCodeEditor | null>(null);
  // `defaultDb`/`defaultSchema` seed the picker once on mount; the picker does
  // not follow later prop changes. Fine today (db/schema are fixed per modal
  // instance) — lift to a controlled prop if ever reused where the source moves.
  const [pickerDb, setPickerDb] = useState(defaultDb);
  const [pickerSchema, setPickerSchema] = useState(defaultSchema);
  const [pickerTable, setPickerTable] = useState("");
  const [dbOptions, setDbOptions] = useState<string[]>([]);
  const [schemaOptions, setSchemaOptions] = useState<string[]>([]);
  const [objects, setObjects] = useState<snowflake.SnowflakeObject[]>([]);
  const [loadingSchemas, setLoadingSchemas] = useState(false);
  const [loadingTables, setLoadingTables] = useState(false);
  const [inserting, setInserting] = useState(false);

  // Load databases once; schemas/objects react to the current picker selection.
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
    if (!pickerDb || !pickerSchema) { setObjects([]); return; }
    setLoadingTables(true);
    ListObjects(pickerDb, pickerSchema)
      .then((objs) => setObjects(objs ?? []))
      .catch(() => setObjects([]))
      .finally(() => setLoadingTables(false));
  }, [pickerDb, pickerSchema]);

  const tableOptions = objects
    .filter((o) => objectKinds.includes(o.kind))
    .map((o) => o.name);

  const onPickDb = (v?: string) => {
    setPickerDb(v ?? "");
    setPickerSchema("");
    setPickerTable("");
    onSourceChange?.();
  };
  const onPickSchema = (v?: string) => {
    setPickerSchema(v ?? "");
    setPickerTable("");
    onSourceChange?.();
  };

  // Replace the whole body when it's empty or still the placeholder; otherwise
  // insert at the cursor so multi-table queries can be assembled.
  const insert = useCallback((snippet: string, editId: string) => {
    const ed = editorRef.current;
    const current = (ed?.getValue() ?? value).trim();
    if (!current || current === placeholder.trim()) {
      onChange(snippet);
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
      onChange(`${value}\n${snippet}`);
    }
  }, [value, placeholder, onChange]);

  // Builds the same SELECT a table drag-and-drop into the SQL editor produces:
  // every column double-quoted (one per line) from the 3-part FQN, falling back
  // to SELECT * if the column list can't be fetched.
  const insertSelect = async () => {
    if (!pickerDb || !pickerSchema || !pickerTable) return;
    setInserting(true);
    try {
      const fqn = `"${esc(pickerDb)}"."${esc(pickerSchema)}"."${esc(pickerTable)}"`;
      let snippet: string;
      try {
        const cols = await GetTableColumns(pickerDb, pickerSchema, pickerTable);
        const colList = (cols ?? []).map((c) => `    "${esc(c)}"`).join(",\n");
        snippet = colList ? `SELECT\n${colList}\nFROM ${fqn}` : `SELECT *\nFROM ${fqn}`;
      } catch {
        snippet = `SELECT *\nFROM ${fqn}`;
      }
      insert(snippet, "insert-select");
    } finally {
      setInserting(false);
    }
  };

  return (
    <Form.Item label={label} required={required} help={help} style={itemStyle ?? { marginBottom: 12 }}>
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
          placeholder={tablePlaceholder}
          style={{ width: 180 }}
          value={pickerTable || undefined}
          onChange={(v) => setPickerTable(v ?? "")}
          disabled={!pickerSchema}
          loading={loadingTables}
          options={tableOptions.map((n) => ({ value: n, label: n }))}
          notFoundContent={loadingTables ? "Loading…" : notFoundText}
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
      {extraPickerRow?.({ db: pickerDb, schema: pickerSchema, objects, loading: loadingTables, insert })}
      <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
        <Editor
          height={height}
          language="sql"
          theme={editorTheme}
          value={value}
          onChange={(v) => onChange(v ?? "")}
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
  );
}
