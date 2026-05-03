// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import type { CSSProperties } from "react";
import {
  Form, Input, Select, Checkbox, Space,
  Divider, InputNumber, Tooltip,
} from "antd";
import { InfoCircleOutlined } from "@ant-design/icons";
import type { fileformat } from "../../../wailsjs/go/models";

export const BASE_DEFAULTS: fileformat.FileFormatConfig = {
  name: "",
  caseSensitive: false,
  orReplace: false,
  ifNotExists: false,
  type: "CSV",
  comment: "",
  compression: "AUTO",
  trimSpace: false,
  replaceInvalid: false,
  fileExtension: "",
  recordDelimiter: "\\n",
  fieldDelimiter: ",",
  multiLine: false,
  parseHeader: false,
  skipHeader: 0,
  skipBlankLines: false,
  dateFormat: "AUTO",
  timeFormat: "AUTO",
  timestampFormat: "AUTO",
  binaryFormat: "HEX",
  escape: "NONE",
  escapeUnenclosedField: "\\\\",
  fieldOptionallyEnclosedBy: "NONE",
  nullIf: ["\\N"],
  errorOnColumnCountMismatch: true,
  emptyFieldAsNull: true,
  skipByteOrderMark: true,
  encoding: "UTF8",
  enableOctal: false,
  allowDuplicate: false,
  stripOuterArray: false,
  stripNullValues: false,
  ignoreUTF8Errors: false,
  preserveSpace: false,
  stripOuterElement: false,
  disableSnowflakeData: false,
  disableAutoConvert: false,
  binaryAsText: true,
  useLogicalType: false,
  snappyCompressionLevel: 0,
} as fileformat.FileFormatConfig;

export function defaultsForType(type: string): Partial<fileformat.FileFormatConfig> {
  const base: Partial<fileformat.FileFormatConfig> = { type, compression: "AUTO", trimSpace: false, replaceInvalid: false, fileExtension: "" };
  switch (type) {
    case "CSV": return {
      ...base,
      recordDelimiter: "\\n",
      fieldDelimiter: ",",
      multiLine: false,
      parseHeader: false,
      skipHeader: 0,
      skipBlankLines: false,
      dateFormat: "AUTO",
      timeFormat: "AUTO",
      timestampFormat: "AUTO",
      binaryFormat: "HEX",
      escape: "NONE",
      escapeUnenclosedField: "\\\\",
      fieldOptionallyEnclosedBy: "NONE",
      nullIf: ["\\N"],
      errorOnColumnCountMismatch: true,
      emptyFieldAsNull: true,
      skipByteOrderMark: true,
      encoding: "UTF8",
    };
    case "JSON": return {
      ...base,
      dateFormat: "AUTO",
      timeFormat: "AUTO",
      timestampFormat: "AUTO",
      binaryFormat: "HEX",
      multiLine: false,
      nullIf: [],
      enableOctal: false,
      allowDuplicate: false,
      stripOuterArray: false,
      stripNullValues: false,
      ignoreUTF8Errors: false,
    };
    case "AVRO": return { ...base, nullIf: [] };
    case "ORC": return { trimSpace: false, replaceInvalid: false, nullIf: [], type };
    case "PARQUET": return {
      ...base,
      binaryAsText: true,
      useLogicalType: false,
      snappyCompression: false,
      snappyCompressionLevel: 0,
      useVectorizedScanner: false,
      nullIf: [],
    };
    case "XML": return {
      ...base,
      ignoreUTF8Errors: false,
      preserveSpace: false,
      stripOuterElement: false,
      disableSnowflakeData: false,
      disableAutoConvert: false,
      skipByteOrderMark: true,
    };
    default: return base;
  }
}

interface Props {
  cfg: fileformat.FileFormatConfig;
  set: <K extends keyof fileformat.FileFormatConfig>(key: K, value: fileformat.FileFormatConfig[K]) => void;
  hideNameFields?: boolean;
}

const COMPRESSION_OPTIONS = ["AUTO", "GZIP", "BZ2", "BROTLI", "ZSTD", "DEFLATE", "RAW_DEFLATE", "NONE"].map(o => ({ value: o, label: o }));
const BINARY_FORMAT_OPTIONS = ["HEX", "BASE64", "UTF8"].map(o => ({ value: o, label: o }));
const ENCLOSED_BY_OPTIONS = [
  { value: "NONE", label: "NONE" },
  { value: "'", label: "Single quote (')" },
  { value: "\"", label: "Double quote (\")" },
];

function divider(label: string) {
  return (
    <Divider
      orientation="left"
      orientationMargin={0}
      style={{ fontSize: 11, color: "var(--text-muted)", margin: "14px 0 8px" }}
    >
      {label}
    </Divider>
  );
}

function help(tip: string) {
  return (
    <Tooltip title={tip}>
      <InfoCircleOutlined style={{ fontSize: 11, color: "var(--text-muted)", marginLeft: 4 }} />
    </Tooltip>
  );
}

const itemSm: CSSProperties = { marginBottom: 8 };

export default function FileFormatFields({ cfg, set, hideNameFields }: Props) {
  const handleTypeChange = (type: string) => {
    set("type", type as any);
    const defs = defaultsForType(type);
    for (const [k, v] of Object.entries(defs)) {
      if (k !== "type") {
        set(k as any, v as any);
      }
    }
  };

  const csvParams = cfg.type === "CSV" && (
    <>
      {divider("Delimiters & Structure")}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 10px" }}>
        <Form.Item
          label={<>Record delimiter{help("Character(s) separating rows. Default: newline (\\n). Use NONE to disable.")}</>}
          style={itemSm}
        >
          <Input value={cfg.recordDelimiter} onChange={e => set("recordDelimiter", e.target.value)} placeholder="\n" />
        </Form.Item>
        <Form.Item
          label={<>Field delimiter{help("Character separating columns. Default: comma (,). Use NONE to disable.")}</>}
          style={itemSm}
        >
          <Input value={cfg.fieldDelimiter} onChange={e => set("fieldDelimiter", e.target.value)} placeholder="," />
        </Form.Item>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 10px" }}>
        <Form.Item label="Skip header rows" style={itemSm}>
          <InputNumber min={0} value={cfg.skipHeader} onChange={v => set("skipHeader", v ?? 0)} style={{ width: "100%" }} />
        </Form.Item>
        <Form.Item
          label={<>Enclosed by{help("Character used to enclose field values. NONE disables enclosing.")}</>}
          style={itemSm}
        >
          <Select value={cfg.fieldOptionallyEnclosedBy} onChange={v => set("fieldOptionallyEnclosedBy", v)} options={ENCLOSED_BY_OPTIONS} />
        </Form.Item>
      </div>
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 10px" }}>
        <Form.Item label={<>Escape{help("Escape character for enclosed fields. Default: NONE.")}</>} style={itemSm}>
          <Input value={cfg.escape} onChange={e => set("escape", e.target.value)} placeholder="NONE" />
        </Form.Item>
        <Form.Item label={<>Escape unenclosed{help("Escape character for unenclosed fields. Default: \\")}</>} style={itemSm}>
          <Input value={cfg.escapeUnenclosedField} onChange={e => set("escapeUnenclosedField", e.target.value)} placeholder="\\" />
        </Form.Item>
      </div>
      <Space direction="vertical" size={4} style={{ marginBottom: 10 }}>
        <Checkbox checked={cfg.multiLine} onChange={e => set("multiLine", e.target.checked)}>
          Multi-line{help("Allow field values to span multiple lines.")}
        </Checkbox>
        <Checkbox checked={cfg.parseHeader} onChange={e => set("parseHeader", e.target.checked)}>
          Parse header{help("Use the first row as column names instead of $1, $2 … in variant output.")}
        </Checkbox>
        <Checkbox checked={cfg.skipBlankLines} onChange={e => set("skipBlankLines", e.target.checked)}>
          Skip blank lines
        </Checkbox>
        <Checkbox checked={cfg.trimSpace} onChange={e => set("trimSpace", e.target.checked)}>
          Trim whitespace
        </Checkbox>
      </Space>

      {divider("Data Handling")}
      <Space direction="vertical" size={4} style={{ marginBottom: 10 }}>
        <Checkbox checked={cfg.errorOnColumnCountMismatch} onChange={e => set("errorOnColumnCountMismatch", e.target.checked)}>
          Error on column count mismatch
        </Checkbox>
        <Checkbox checked={cfg.emptyFieldAsNull} onChange={e => set("emptyFieldAsNull", e.target.checked)}>
          Empty field as NULL
        </Checkbox>
        <Checkbox checked={cfg.replaceInvalid} onChange={e => set("replaceInvalid", e.target.checked)}>
          Replace invalid characters
        </Checkbox>
        <Checkbox checked={cfg.skipByteOrderMark} onChange={e => set("skipByteOrderMark", e.target.checked)}>
          Skip byte-order mark (BOM)
        </Checkbox>
      </Space>
      <Form.Item label={<>NULL_IF values{help("Comma-separated strings treated as SQL NULL. E.g. \\N, NULL, ''")}</>} style={itemSm}>
        <Input
          value={(cfg.nullIf ?? []).join(", ")}
          onChange={e => set("nullIf", e.target.value.split(",").map(s => s.trim()).filter(Boolean))}
          placeholder="\N"
        />
      </Form.Item>

      {divider("Date / Time Formats")}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 10px" }}>
        <Form.Item label={<>Date{help("AUTO or pattern like YYYY-MM-DD")}</>} style={itemSm}>
          <Input value={cfg.dateFormat} onChange={e => set("dateFormat", e.target.value)} placeholder="AUTO" />
        </Form.Item>
        <Form.Item label={<>Time{help("AUTO or pattern like HH24:MI:SS")}</>} style={itemSm}>
          <Input value={cfg.timeFormat} onChange={e => set("timeFormat", e.target.value)} placeholder="AUTO" />
        </Form.Item>
        <Form.Item label={<>Timestamp{help("AUTO or pattern like YYYY-MM-DD HH24:MI:SS")}</>} style={itemSm}>
          <Input value={cfg.timestampFormat} onChange={e => set("timestampFormat", e.target.value)} placeholder="AUTO" />
        </Form.Item>
      </div>

      {divider("Encoding & Binary")}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 10px" }}>
        <Form.Item label="Binary format" style={itemSm}>
          <Select value={cfg.binaryFormat} onChange={v => set("binaryFormat", v)} options={BINARY_FORMAT_OPTIONS} />
        </Form.Item>
        <Form.Item label={<>Encoding{help("Character encoding. Default: UTF8.")}</>} style={itemSm}>
          <Input value={cfg.encoding} onChange={e => set("encoding", e.target.value)} placeholder="UTF8" />
        </Form.Item>
      </div>
    </>
  );

  const jsonParams = cfg.type === "JSON" && (
    <>
      {divider("JSON Options")}
      <Space direction="vertical" size={4} style={{ marginBottom: 10 }}>
        <Checkbox checked={cfg.stripOuterArray} onChange={e => set("stripOuterArray", e.target.checked)}>
          Strip outer array{help("Remove the outermost [ ] from a JSON array, loading each element as a separate row.")}
        </Checkbox>
        <Checkbox checked={cfg.stripNullValues} onChange={e => set("stripNullValues", e.target.checked)}>
          Strip null values{help("Remove key-value pairs where the value is null.")}
        </Checkbox>
        <Checkbox checked={cfg.multiLine} onChange={e => set("multiLine", e.target.checked)}>
          Multi-line{help("Allow records to span multiple lines.")}
        </Checkbox>
        <Checkbox checked={cfg.allowDuplicate} onChange={e => set("allowDuplicate", e.target.checked)}>
          Allow duplicate keys{help("Keep the last occurrence of a duplicate key (default: error).")}
        </Checkbox>
        <Checkbox checked={cfg.enableOctal} onChange={e => set("enableOctal", e.target.checked)}>
          Enable octal{help("Allow octal number literals in JSON (e.g. 0777).")}
        </Checkbox>
        <Checkbox checked={cfg.ignoreUTF8Errors} onChange={e => set("ignoreUTF8Errors", e.target.checked)}>
          Ignore UTF-8 errors
        </Checkbox>
        <Checkbox checked={cfg.skipByteOrderMark} onChange={e => set("skipByteOrderMark", e.target.checked)}>
          Skip byte-order mark (BOM)
        </Checkbox>
        <Checkbox checked={cfg.trimSpace} onChange={e => set("trimSpace", e.target.checked)}>
          Trim whitespace
        </Checkbox>
        <Checkbox checked={cfg.replaceInvalid} onChange={e => set("replaceInvalid", e.target.checked)}>
          Replace invalid characters
        </Checkbox>
      </Space>
      {divider("Date / Time Formats")}
      <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 10px" }}>
        <Form.Item label="Date" style={itemSm}>
          <Input value={cfg.dateFormat} onChange={e => set("dateFormat", e.target.value)} placeholder="AUTO" />
        </Form.Item>
        <Form.Item label="Time" style={itemSm}>
          <Input value={cfg.timeFormat} onChange={e => set("timeFormat", e.target.value)} placeholder="AUTO" />
        </Form.Item>
        <Form.Item label="Timestamp" style={itemSm}>
          <Input value={cfg.timestampFormat} onChange={e => set("timestampFormat", e.target.value)} placeholder="AUTO" />
        </Form.Item>
      </div>
      <Form.Item label="Binary format" style={itemSm}>
        <Select value={cfg.binaryFormat} onChange={v => set("binaryFormat", v)} options={BINARY_FORMAT_OPTIONS} style={{ width: 160 }} />
      </Form.Item>
      <Form.Item label={<>NULL_IF values{help("Comma-separated strings treated as SQL NULL.")}</>} style={itemSm}>
        <Input
          value={(cfg.nullIf ?? []).join(", ")}
          onChange={e => set("nullIf", e.target.value.split(",").map(s => s.trim()).filter(Boolean))}
          placeholder="(empty list)"
        />
      </Form.Item>
    </>
  );

  const avroOrcParams = (cfg.type === "AVRO" || cfg.type === "ORC") && (
    <>
      {divider(`${cfg.type} Options`)}
      <Space direction="vertical" size={4} style={{ marginBottom: 10 }}>
        <Checkbox checked={cfg.trimSpace} onChange={e => set("trimSpace", e.target.checked)}>Trim whitespace</Checkbox>
        <Checkbox checked={cfg.replaceInvalid} onChange={e => set("replaceInvalid", e.target.checked)}>Replace invalid characters</Checkbox>
      </Space>
      <Form.Item label={<>NULL_IF values{help("Comma-separated strings treated as SQL NULL.")}</>} style={itemSm}>
        <Input
          value={(cfg.nullIf ?? []).join(", ")}
          onChange={e => set("nullIf", e.target.value.split(",").map(s => s.trim()).filter(Boolean))}
          placeholder="(empty list)"
        />
      </Form.Item>
    </>
  );

  const parquetParams = cfg.type === "PARQUET" && (
    <>
      {divider("Parquet Options")}
      <Space direction="vertical" size={4} style={{ marginBottom: 10 }}>
        <Checkbox checked={cfg.binaryAsText} onChange={e => set("binaryAsText", e.target.checked)}>
          Binary as text{help("Treat Parquet BINARY columns as VARCHAR instead of BINARY.")}
        </Checkbox>
        <Checkbox checked={cfg.useLogicalType} onChange={e => set("useLogicalType", e.target.checked)}>
          Use logical type{help("Map Parquet logical types (DATE, TIME, TIMESTAMP) to the corresponding Snowflake types.")}
        </Checkbox>
        <Checkbox checked={cfg.snappyCompression} onChange={e => set("snappyCompression", e.target.checked)}>
          Snappy compression{help("Use Snappy compression for Parquet.")}
        </Checkbox>
        <Checkbox checked={cfg.useVectorizedScanner} onChange={e => set("useVectorizedScanner", e.target.checked)}>
          Use vectorized scanner{help("Enable vectorized scanning for Parquet files.")}
        </Checkbox>
        <Checkbox checked={cfg.trimSpace} onChange={e => set("trimSpace", e.target.checked)}>Trim whitespace</Checkbox>
        <Checkbox checked={cfg.replaceInvalid} onChange={e => set("replaceInvalid", e.target.checked)}>Replace invalid characters</Checkbox>
      </Space>
      <Form.Item label={<>Snappy compression level{help("0 = default. 1 (fastest) – 22 (best compression) for ZSTD, or 1–9 for Snappy.")}</>} style={itemSm}>
        <InputNumber min={0} value={cfg.snappyCompressionLevel} onChange={v => set("snappyCompressionLevel", v ?? 0)} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item label={<>NULL_IF values{help("Comma-separated strings treated as SQL NULL.")}</>} style={itemSm}>
        <Input
          value={(cfg.nullIf ?? []).join(", ")}
          onChange={e => set("nullIf", e.target.value.split(",").map(s => s.trim()).filter(Boolean))}
          placeholder="(empty list)"
        />
      </Form.Item>
    </>
  );

  const xmlParams = cfg.type === "XML" && (
    <>
      {divider("XML Options")}
      <Space direction="vertical" size={4} style={{ marginBottom: 10 }}>
        <Checkbox checked={cfg.preserveSpace} onChange={e => set("preserveSpace", e.target.checked)}>
          Preserve space{help("Preserve leading and trailing whitespace in element text content.")}
        </Checkbox>
        <Checkbox checked={cfg.stripOuterElement} onChange={e => set("stripOuterElement", e.target.checked)}>
          Strip outer element{help("Remove the root element wrapper and load each child element as a separate row.")}
        </Checkbox>
        <Checkbox checked={cfg.disableSnowflakeData} onChange={e => set("disableSnowflakeData", e.target.checked)}>
          Disable Snowflake data{help("Disable the special $snowflake_data wrapper used for staging Snowflake data.")}
        </Checkbox>
        <Checkbox checked={cfg.disableAutoConvert} onChange={e => set("disableAutoConvert", e.target.checked)}>
          Disable auto convert{help("Disable automatic type conversion of XML element values.")}
        </Checkbox>
        <Checkbox checked={cfg.ignoreUTF8Errors} onChange={e => set("ignoreUTF8Errors", e.target.checked)}>
          Ignore UTF-8 errors
        </Checkbox>
        <Checkbox checked={cfg.replaceInvalid} onChange={e => set("replaceInvalid", e.target.checked)}>
          Replace invalid characters
        </Checkbox>
        <Checkbox checked={cfg.skipByteOrderMark} onChange={e => set("skipByteOrderMark", e.target.checked)}>
          Skip byte-order mark (BOM)
        </Checkbox>
      </Space>
    </>
  );

  return (
    <Form layout="vertical" size="small">
      {!hideNameFields && (
        <>
          <Form.Item label="Format name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_CSV_FORMAT"
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
          <Form.Item label="Comment" style={{ marginBottom: 10 }}>
            <Input value={cfg.comment} onChange={e => set("comment", e.target.value)} placeholder="Format description" />
          </Form.Item>
        </>
      )}

      {/* Format type */}
      <Form.Item label="Format type" style={{ marginBottom: 10 }}>
        <Select
          value={cfg.type}
          onChange={handleTypeChange}
          options={["CSV", "JSON", "AVRO", "ORC", "PARQUET", "XML"].map(t => ({ value: t, label: t }))}
        />
      </Form.Item>

      {/* Compression (not for ORC) */}
      {cfg.type !== "ORC" && (
        <Form.Item label="Compression" style={{ marginBottom: 10 }}>
          <Select value={cfg.compression} onChange={v => set("compression", v)} options={COMPRESSION_OPTIONS} />
        </Form.Item>
      )}

      {/* File extension (CSV/JSON) */}
      {(cfg.type === "CSV" || cfg.type === "JSON") && (
        <Form.Item label={<>File extension{help("Override the file extension used when writing. Leave blank to use the default.")}</>} style={{ marginBottom: 10 }}>
          <Input value={cfg.fileExtension} onChange={e => set("fileExtension", e.target.value)} placeholder=".csv" />
        </Form.Item>
      )}

      {/* Per-type options */}
      {csvParams}
      {jsonParams}
      {avroOrcParams}
      {parquetParams}
      {xmlParams}
    </Form>
  );
}
