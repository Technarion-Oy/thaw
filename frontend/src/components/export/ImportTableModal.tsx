// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState } from "react";
import {
  Modal, Select, Switch, Input, InputNumber, Button, Space, Typography,
  Spin, Segmented, Tooltip, Collapse,
} from "antd";
import {
  UploadOutlined, FolderOpenOutlined, CheckCircleOutlined,
  CloseOutlined, FileOutlined, SettingOutlined,
} from "@ant-design/icons";
import { ImportTableData, PickDataFilesByFormat } from "../../../wailsjs/go/main/App";
import { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

// ── Types ────────────────────────────────────────────────────────────────────

type Format = "CSV" | "JSON" | "AVRO" | "ORC" | "PARQUET";

interface FormatOptions {
  // Common
  compression: string;
  trimSpace: boolean;
  replaceInvalidCharacters: boolean;
  nullIf: string[];
  // CSV + JSON
  dateFormat: string;
  timeFormat: string;
  timestampFormat: string;
  binaryFormat: string;
  fileExtension: string;
  multiLine: boolean;
  skipByteOrderMark: boolean;
  ignoreUtf8Errors: boolean;
  // CSV
  recordDelimiter: string;
  fieldDelimiter: string;
  parseHeader: boolean;
  skipHeader: number;
  skipBlankLines: boolean;
  escape: string;
  escapeUnenclosedField: string;
  fieldOptionallyEnclosedBy: string;
  errorOnColumnCountMismatch: boolean;
  emptyFieldAsNull: boolean;
  encoding: string;
  // JSON
  enableOctal: boolean;
  allowDuplicate: boolean;
  stripOuterArray: boolean;
  stripNullValues: boolean;
  // PARQUET
  snappyCompression: boolean;
  binaryAsText: boolean;
  useLogicalType: boolean;
  useVectorizedScanner: boolean;
}

interface ImportResult {
  rowsLoaded: number;
  filesLoaded: number;
  tableName: string;
}

interface Props {
  db: string;
  schema: string;
  table: string;
  onClose: () => void;
  onSuccess: () => void;
}

// ── Defaults ─────────────────────────────────────────────────────────────────

const BASE_OPTIONS: FormatOptions = {
  compression: "AUTO",
  trimSpace: false,
  replaceInvalidCharacters: false,
  nullIf: [],
  dateFormat: "AUTO",
  timeFormat: "AUTO",
  timestampFormat: "AUTO",
  binaryFormat: "HEX",
  fileExtension: "NONE",
  multiLine: false,
  skipByteOrderMark: true,
  ignoreUtf8Errors: false,
  recordDelimiter: "\\n",
  fieldDelimiter: ",",
  parseHeader: false,
  skipHeader: 0,
  skipBlankLines: false,
  escape: "NONE",
  escapeUnenclosedField: "\\\\",
  fieldOptionallyEnclosedBy: "NONE",
  errorOnColumnCountMismatch: true,
  emptyFieldAsNull: true,
  encoding: "UTF8",
  enableOctal: false,
  allowDuplicate: false,
  stripOuterArray: false,
  stripNullValues: false,
  snappyCompression: true,
  binaryAsText: true,
  useLogicalType: true,
  useVectorizedScanner: false,
};

function defaultOptions(fmt: Format): FormatOptions {
  switch (fmt) {
    case "CSV":    return { ...BASE_OPTIONS, nullIf: ["\\N"], skipByteOrderMark: true };
    case "JSON":   return { ...BASE_OPTIONS, nullIf: [] };
    case "AVRO":   return { ...BASE_OPTIONS, nullIf: [] };
    case "ORC":    return { ...BASE_OPTIONS, nullIf: [] };
    case "PARQUET":return { ...BASE_OPTIONS, nullIf: [] };
  }
}

// ── Helpers ───────────────────────────────────────────────────────────────────

function detectFormat(p: string): Format {
  const l = p.toLowerCase();
  if (l.endsWith(".parquet") || l.endsWith(".snappy.parquet")) return "PARQUET";
  if (l.endsWith(".json") || l.endsWith(".ndjson") || l.endsWith(".jsonl")) return "JSON";
  if (l.endsWith(".avro")) return "AVRO";
  if (l.endsWith(".orc"))  return "ORC";
  return "CSV";
}

function baseName(p: string): string {
  return p.split(/[/\\]/).pop() ?? p;
}

// ── UI sub-components ─────────────────────────────────────────────────────────

const ROW: React.CSSProperties = {
  display: "flex", alignItems: "center", justifyContent: "space-between",
  gap: 8, minHeight: 28,
};
const LABEL: React.CSSProperties = { fontSize: 13, color: "var(--text)", flex: 1 };
const SECTION_TITLE: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  textTransform: "uppercase", letterSpacing: "0.05em", marginBottom: 10,
};
const GRID2: React.CSSProperties = {
  display: "grid", gridTemplateColumns: "1fr 1fr", gap: "10px 16px",
};

function ToggleRow({ label, value, onChange }: { label: string; value: boolean; onChange: (v: boolean) => void }) {
  return (
    <div style={ROW}>
      <span style={LABEL}>{label}</span>
      <Switch checked={value} onChange={onChange} size="small" />
    </div>
  );
}

function StrRow({ label, value, onChange, placeholder }: {
  label: string; value: string; onChange: (v: string) => void; placeholder?: string;
}) {
  return (
    <div style={{ ...ROW, alignItems: "flex-start", flexDirection: "column", gap: 4 }}>
      <span style={{ ...LABEL, flex: "unset" }}>{label}</span>
      <Input
        size="small" value={value} onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder} style={{ fontFamily: "monospace", fontSize: 12 }}
      />
    </div>
  );
}

function SelectRow({ label, value, onChange, options }: {
  label: string; value: string; onChange: (v: string) => void;
  options: { value: string; label: string }[];
}) {
  return (
    <div style={{ ...ROW, alignItems: "flex-start", flexDirection: "column", gap: 4 }}>
      <span style={{ ...LABEL, flex: "unset" }}>{label}</span>
      <Select size="small" value={value} onChange={onChange} options={options} style={{ width: "100%" }} />
    </div>
  );
}

// ── Format option panels ───────────────────────────────────────────────────────

const COMPRESSION_CSV  = ["AUTO","GZIP","BZ2","BROTLI","ZSTD","DEFLATE","RAW_DEFLATE","NONE"].map(v=>({value:v,label:v}));
const COMPRESSION_AVRO = ["AUTO","GZIP","BROTLI","ZSTD","DEFLATE","RAW_DEFLATE","NONE"].map(v=>({value:v,label:v}));
const COMPRESSION_PQET = ["AUTO","LZO","SNAPPY","NONE"].map(v=>({value:v,label:v}));
const BINARY_FORMATS   = ["HEX","BASE64","UTF8"].map(v=>({value:v,label:v}));

function CsvOptions({ o, set }: { o: FormatOptions; set: (k: keyof FormatOptions, v: any) => void }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      {/* Fields */}
      <div>
        <div style={SECTION_TITLE}>Fields</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <StrRow label="Field delimiter" value={o.fieldDelimiter} onChange={v => set("fieldDelimiter", v)} placeholder="," />
          <StrRow label="Record delimiter" value={o.recordDelimiter} onChange={v => set("recordDelimiter", v)} placeholder="\n" />
          <StrRow label="Field optionally enclosed by" value={o.fieldOptionallyEnclosedBy} onChange={v => set("fieldOptionallyEnclosedBy", v)} placeholder={'NONE or "'} />
          <StrRow label="Escape" value={o.escape} onChange={v => set("escape", v)} placeholder="NONE or \\" />
          <StrRow label="Escape unenclosed field" value={o.escapeUnenclosedField} onChange={v => set("escapeUnenclosedField", v)} placeholder="\\\\" />
          <div style={GRID2}>
            <ToggleRow label="Trim space" value={o.trimSpace} onChange={v => set("trimSpace", v)} />
            <ToggleRow label="Multi-line" value={o.multiLine} onChange={v => set("multiLine", v)} />
          </div>
        </div>
      </div>
      {/* Header */}
      <div>
        <div style={SECTION_TITLE}>Header</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <ToggleRow label="Parse header (first row as column names)" value={o.parseHeader} onChange={v => set("parseHeader", v)} />
          {!o.parseHeader && (
            <div style={ROW}>
              <span style={LABEL}>Skip header rows</span>
              <InputNumber
                size="small" min={0} value={o.skipHeader}
                onChange={v => set("skipHeader", v ?? 0)} style={{ width: 80 }}
              />
            </div>
          )}
          <ToggleRow label="Skip blank lines" value={o.skipBlankLines} onChange={v => set("skipBlankLines", v)} />
        </div>
      </div>
      {/* NULL handling */}
      <div>
        <div style={SECTION_TITLE}>NULL handling</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <div>
            <div style={{ ...LABEL, marginBottom: 4 }}>NULL if (values treated as NULL)</div>
            <Select
              mode="tags" size="small" style={{ width: "100%" }}
              value={o.nullIf} onChange={v => set("nullIf", v)}
              placeholder="Type value and press Enter…"
              tokenSeparators={[]}
            />
          </div>
          <div style={GRID2}>
            <ToggleRow label="Empty field as NULL" value={o.emptyFieldAsNull} onChange={v => set("emptyFieldAsNull", v)} />
            <ToggleRow label="Error on column count mismatch" value={o.errorOnColumnCountMismatch} onChange={v => set("errorOnColumnCountMismatch", v)} />
          </div>
        </div>
      </div>
      {/* Encoding & compression */}
      <div>
        <div style={SECTION_TITLE}>Encoding & compression</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <SelectRow label="Compression" value={o.compression} onChange={v => set("compression", v)} options={COMPRESSION_CSV} />
          <StrRow label="Encoding" value={o.encoding} onChange={v => set("encoding", v)} placeholder="UTF8" />
          <SelectRow label="Binary format" value={o.binaryFormat} onChange={v => set("binaryFormat", v)} options={BINARY_FORMATS} />
          <ToggleRow label="Skip byte order mark (BOM)" value={o.skipByteOrderMark} onChange={v => set("skipByteOrderMark", v)} />
        </div>
      </div>
      {/* Date/time */}
      <div>
        <div style={SECTION_TITLE}>Date & time formats</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <StrRow label="Date format" value={o.dateFormat} onChange={v => set("dateFormat", v)} placeholder="AUTO" />
          <StrRow label="Time format" value={o.timeFormat} onChange={v => set("timeFormat", v)} placeholder="AUTO" />
          <StrRow label="Timestamp format" value={o.timestampFormat} onChange={v => set("timestampFormat", v)} placeholder="AUTO" />
        </div>
      </div>
      {/* Other */}
      <div>
        <div style={SECTION_TITLE}>Other</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <ToggleRow label="Replace invalid characters" value={o.replaceInvalidCharacters} onChange={v => set("replaceInvalidCharacters", v)} />
          <StrRow label="File extension" value={o.fileExtension} onChange={v => set("fileExtension", v)} placeholder="NONE" />
        </div>
      </div>
    </div>
  );
}

function JsonOptions({ o, set }: { o: FormatOptions; set: (k: keyof FormatOptions, v: any) => void }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div>
        <div style={SECTION_TITLE}>Structure</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <SelectRow label="Compression" value={o.compression} onChange={v => set("compression", v)} options={COMPRESSION_CSV} />
          <div style={GRID2}>
            <ToggleRow label="Multi-line" value={o.multiLine} onChange={v => set("multiLine", v)} />
            <ToggleRow label="Strip outer array" value={o.stripOuterArray} onChange={v => set("stripOuterArray", v)} />
            <ToggleRow label="Strip null values" value={o.stripNullValues} onChange={v => set("stripNullValues", v)} />
            <ToggleRow label="Allow duplicate keys" value={o.allowDuplicate} onChange={v => set("allowDuplicate", v)} />
            <ToggleRow label="Enable octal" value={o.enableOctal} onChange={v => set("enableOctal", v)} />
            <ToggleRow label="Trim space" value={o.trimSpace} onChange={v => set("trimSpace", v)} />
          </div>
        </div>
      </div>
      <div>
        <div style={SECTION_TITLE}>NULL handling</div>
        <Select
          mode="tags" size="small" style={{ width: "100%" }}
          value={o.nullIf} onChange={v => set("nullIf", v)}
          placeholder="Type value and press Enter…"
          tokenSeparators={[]}
        />
      </div>
      <div>
        <div style={SECTION_TITLE}>Encoding</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <SelectRow label="Binary format" value={o.binaryFormat} onChange={v => set("binaryFormat", v)} options={BINARY_FORMATS} />
          <div style={GRID2}>
            <ToggleRow label="Skip byte order mark" value={o.skipByteOrderMark} onChange={v => set("skipByteOrderMark", v)} />
            <ToggleRow label="Ignore UTF-8 errors" value={o.ignoreUtf8Errors} onChange={v => set("ignoreUtf8Errors", v)} />
          </div>
        </div>
      </div>
      <div>
        <div style={SECTION_TITLE}>Date & time formats</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <StrRow label="Date format" value={o.dateFormat} onChange={v => set("dateFormat", v)} placeholder="AUTO" />
          <StrRow label="Time format" value={o.timeFormat} onChange={v => set("timeFormat", v)} placeholder="AUTO" />
          <StrRow label="Timestamp format" value={o.timestampFormat} onChange={v => set("timestampFormat", v)} placeholder="AUTO" />
        </div>
      </div>
      <div>
        <div style={SECTION_TITLE}>Other</div>
        <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
          <ToggleRow label="Replace invalid characters" value={o.replaceInvalidCharacters} onChange={v => set("replaceInvalidCharacters", v)} />
          <StrRow label="File extension" value={o.fileExtension} onChange={v => set("fileExtension", v)} placeholder="NONE" />
        </div>
      </div>
    </div>
  );
}

function AvroOptions({ o, set }: { o: FormatOptions; set: (k: keyof FormatOptions, v: any) => void }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      <SelectRow label="Compression" value={o.compression} onChange={v => set("compression", v)} options={COMPRESSION_AVRO} />
      <div>
        <div style={{ ...LABEL, marginBottom: 4 }}>NULL if</div>
        <Select
          mode="tags" size="small" style={{ width: "100%" }}
          value={o.nullIf} onChange={v => set("nullIf", v)}
          placeholder="Type value and press Enter…"
          tokenSeparators={[]}
        />
      </div>
      <div style={GRID2}>
        <ToggleRow label="Trim space" value={o.trimSpace} onChange={v => set("trimSpace", v)} />
        <ToggleRow label="Replace invalid characters" value={o.replaceInvalidCharacters} onChange={v => set("replaceInvalidCharacters", v)} />
      </div>
    </div>
  );
}

function OrcOptions({ o, set }: { o: FormatOptions; set: (k: keyof FormatOptions, v: any) => void }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      <div>
        <div style={{ ...LABEL, marginBottom: 4 }}>NULL if</div>
        <Select
          mode="tags" size="small" style={{ width: "100%" }}
          value={o.nullIf} onChange={v => set("nullIf", v)}
          placeholder="Type value and press Enter…"
          tokenSeparators={[]}
        />
      </div>
      <div style={GRID2}>
        <ToggleRow label="Trim space" value={o.trimSpace} onChange={v => set("trimSpace", v)} />
        <ToggleRow label="Replace invalid characters" value={o.replaceInvalidCharacters} onChange={v => set("replaceInvalidCharacters", v)} />
      </div>
    </div>
  );
}

function ParquetOptions({ o, set }: { o: FormatOptions; set: (k: keyof FormatOptions, v: any) => void }) {
  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      <SelectRow label="Compression" value={o.compression} onChange={v => set("compression", v)} options={COMPRESSION_PQET} />
      <div style={GRID2}>
        <ToggleRow label="Snappy compression" value={o.snappyCompression} onChange={v => set("snappyCompression", v)} />
        <ToggleRow label="Binary as text" value={o.binaryAsText} onChange={v => set("binaryAsText", v)} />
        <ToggleRow label="Use logical type" value={o.useLogicalType} onChange={v => set("useLogicalType", v)} />
        <ToggleRow label="Use vectorized scanner" value={o.useVectorizedScanner} onChange={v => set("useVectorizedScanner", v)} />
        <ToggleRow label="Trim space" value={o.trimSpace} onChange={v => set("trimSpace", v)} />
        <ToggleRow label="Replace invalid characters" value={o.replaceInvalidCharacters} onChange={v => set("replaceInvalidCharacters", v)} />
      </div>
      <div>
        <div style={{ ...LABEL, marginBottom: 4 }}>NULL if</div>
        <Select
          mode="tags" size="small" style={{ width: "100%" }}
          value={o.nullIf} onChange={v => set("nullIf", v)}
          placeholder="Type value and press Enter…"
          tokenSeparators={[]}
        />
      </div>
    </div>
  );
}

// ── Main modal ────────────────────────────────────────────────────────────────

export default function ImportTableModal({ db, schema, table, onClose, onSuccess }: Props) {
  const [filePaths, setFilePaths]     = useState<string[]>([]);
  const [format, setFormat]           = useState<Format>("CSV");
  const [options, setOptions]         = useState<FormatOptions>(() => defaultOptions("CSV"));
  const [overwrite, setOverwrite]     = useState(false);
  const [createTable, setCreateTable] = useState(table === "");
  const [newTableName, setNewTableName] = useState(table);
  const [targetTable, setTargetTable] = useState(table);
  const [importing, setImporting]     = useState(false);
  const [error, setError]             = useState<string | null>(null);
  const [result, setResult]           = useState<ImportResult | null>(null);

  const effectiveTable = createTable ? newTableName.trim() : (table || targetTable.trim());

  const setOpt = (k: keyof FormatOptions, v: any) =>
    setOptions((prev) => ({ ...prev, [k]: v }));

  const changeFormat = (f: Format) => {
    setFormat(f);
    setOptions(defaultOptions(f));
  };

  const addFiles = async () => {
    const picked = await PickDataFilesByFormat(format);
    if (!picked || picked.length === 0) return;
    setFilePaths((prev) => {
      const existing = new Set(prev);
      const added = picked.filter((p) => !existing.has(p));
      if (prev.length === 0 && added.length > 0) {
        const detected = detectFormat(added[0]);
        changeFormat(detected);
      }
      return [...prev, ...added];
    });
  };

  const removeFile = (idx: number) =>
    setFilePaths((prev) => prev.filter((_, i) => i !== idx));

  const handleImport = async () => {
    if (filePaths.length === 0 || !effectiveTable) return;
    setError(null);
    setImporting(true);
    try {
      const r = await ImportTableData(snowflake.ImportTableParams.createFrom({
        database: db,
        schema,
        table: effectiveTable,
        filePaths,
        format,
        overwrite: createTable ? false : overwrite,
        createTable,
        options,
      }));
      setResult({ rowsLoaded: r.rowsLoaded, filesLoaded: r.filesLoaded, tableName: effectiveTable });
      if (createTable) onSuccess();
    } catch (e) {
      setError(String(e));
    } finally {
      setImporting(false);
    }
  };

  const formatOptionsPanel = () => {
    switch (format) {
      case "CSV":     return <CsvOptions     o={options} set={setOpt} />;
      case "JSON":    return <JsonOptions    o={options} set={setOpt} />;
      case "AVRO":    return <AvroOptions    o={options} set={setOpt} />;
      case "ORC":     return <OrcOptions     o={options} set={setOpt} />;
      case "PARQUET": return <ParquetOptions o={options} set={setOpt} />;
    }
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <UploadOutlined style={{ color: "var(--link)" }} />
          <span>Import data</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{createTable ? (newTableName || "…") : (table || targetTable || "…")}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        result ? (
          <Button type="primary" onClick={onClose}>Done</Button>
        ) : (
          <Space style={{ justifyContent: "flex-end", display: "flex" }}>
            <Button onClick={onClose} disabled={importing}>Cancel</Button>
            <Button
              type="primary"
              icon={<UploadOutlined />}
              onClick={handleImport}
              disabled={filePaths.length === 0 || !effectiveTable || importing}
              loading={importing}
            >
              Import
            </Button>
          </Space>
        )
      }
      width={580}
      styles={{ body: { paddingTop: 20, maxHeight: "75vh", overflowY: "auto" } }}
    >
      {result ? (
        /* ── Success ── */
        <div style={{ textAlign: "center", padding: "16px 0" }}>
          <CheckCircleOutlined style={{ fontSize: 40, color: "#3fb950", marginBottom: 16 }} />
          <div style={{ fontSize: 15, fontWeight: 600, color: "var(--text)", marginBottom: 8 }}>
            Import complete
          </div>
          <div style={{ fontSize: 13, color: "var(--text-muted)" }}>
            {result.rowsLoaded.toLocaleString()} row{result.rowsLoaded !== 1 ? "s" : ""} loaded
            {" from "}{result.filesLoaded} file{result.filesLoaded !== 1 ? "s" : ""}
            {" into "}
            <Text style={{ fontFamily: "monospace" }}>{db}.{schema}.{result.tableName}</Text>
          </div>
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>

          {/* ── File list ── */}
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Source files</div>
            {filePaths.length > 0 && (
              <div style={{ maxHeight: 130, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 6, marginBottom: 8 }}>
                {filePaths.map((fp, idx) => (
                  <div key={fp} style={{ display: "flex", alignItems: "center", gap: 8, padding: "5px 10px", borderBottom: idx < filePaths.length - 1 ? "1px solid var(--border)" : undefined }}>
                    <FileOutlined style={{ color: "var(--text-muted)", flexShrink: 0 }} />
                    <Tooltip title={fp} mouseEnterDelay={0.6}>
                      <span style={{ flex: 1, fontSize: 12, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap", color: "var(--text)" }}>
                        {baseName(fp)}
                      </span>
                    </Tooltip>
                    <Button type="text" size="small" icon={<CloseOutlined style={{ fontSize: 10 }} />}
                      onClick={() => removeFile(idx)} style={{ flexShrink: 0, color: "var(--text-muted)" }} />
                  </div>
                ))}
              </div>
            )}
            <Button icon={<FolderOpenOutlined />} onClick={addFiles} style={{ width: "100%" }}>
              {filePaths.length === 0 ? "Browse…" : "Add more files…"}
            </Button>
          </div>

          {/* ── Format ── */}
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>
              Format
              <span style={{ color: "var(--text-faint)", marginLeft: 6 }}>(auto-detected from extension)</span>
            </div>
            <Segmented
              value={format}
              onChange={(v) => changeFormat(v as Format)}
              options={["CSV", "JSON", "AVRO", "ORC", "PARQUET"]}
              block
            />
          </div>

          {/* ── Format options (collapsible) ── */}
          <Collapse
            size="small"
            ghost
            items={[{
              key: "opts",
              label: (
                <Space size={6}>
                  <SettingOutlined />
                  <span style={{ fontSize: 13 }}>Format options</span>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    ({format} defaults pre-filled)
                  </Text>
                </Space>
              ),
              children: (
                <div style={{ paddingTop: 4 }}>
                  {formatOptionsPanel()}
                </div>
              ),
            }]}
          />

          <div style={{ borderTop: "1px solid var(--border)" }} />

          {/* ── Create new table ── */}
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <Text style={{ fontSize: 13 }}>Create new table from data</Text>
              <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                Infers schema from file; creates table if it doesn't exist
              </div>
            </div>
            <Switch checked={createTable} onChange={(v) => { setCreateTable(v); if (v) setOverwrite(false); }} size="small" />
          </div>

          {createTable && (
            <div>
              <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>New table name</div>
              <Input value={newTableName} onChange={(e) => setNewTableName(e.target.value)} placeholder="Table name" autoFocus />
            </div>
          )}

          {/* ── Target table (schema-level mode, existing table) ── */}
          {!createTable && !table && (
            <div>
              <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Target table name</div>
              <Input
                value={targetTable}
                onChange={(e) => setTargetTable(e.target.value)}
                placeholder="Existing table name"
                autoFocus
              />
            </div>
          )}

          {/* ── Overwrite ── */}
          {!createTable && (
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <div>
                <Text style={{ fontSize: 13 }}>Overwrite existing data</Text>
                <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>Truncates the table before importing</div>
              </div>
              <Switch checked={overwrite} onChange={setOverwrite} size="small" />
            </div>
          )}

          {/* ── Error ── */}
          {error && (
            <div style={{ padding: "10px 14px", background: "rgba(248,81,73,0.08)", border: "1px solid rgba(248,81,73,0.3)", borderRadius: 6, color: "#f85149", fontFamily: "monospace", fontSize: 12, wordBreak: "break-word" }}>
              {error}
            </div>
          )}

          {importing && (
            <div style={{ textAlign: "center", padding: "4px 0" }}>
              <Spin size="small" />
              <span style={{ marginLeft: 10, fontSize: 12, color: "var(--text-muted)" }}>
                Importing… this may take a while for large files
              </span>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
