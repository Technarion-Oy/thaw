// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect, useRef, useCallback } from "react";
import {
  Modal, Form, Input, Select, Checkbox, Space,
  Typography, Divider, InputNumber, Button, Table, Alert, Radio, Tooltip,
} from "antd";
import {
  FileTextOutlined, PlusOutlined, FileSearchOutlined,
  CloudOutlined, FileOutlined, InfoCircleOutlined,
} from "@ant-design/icons";
import {
  ExecDDL, GetQuotedIdentifiersIgnoreCase, BuildCreateFileFormatSql,
  PickFileForFormatPreview, GetLocalFilePreview, GetStageFilePreview,
} from "../../../wailsjs/go/main/App";
import type { fileformat } from "../../../wailsjs/go/models";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// ── Per-type defaults ────────────────────────────────────────────────────────

function defaultsForType(type: string): Partial<fileformat.FileFormatConfig> {
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
      nullIf: [],
      enableOctal: false,
      allowDuplicate: false,
      stripOuterArray: false,
      stripNullValues: false,
      ignoreUTF8Errors: false,
      skipByteOrderMark: true,
    };
    case "AVRO": return { ...base, nullIf: [] };
    case "ORC": return { trimSpace: false, replaceInvalid: false, nullIf: [], type };
    case "PARQUET": return {
      ...base,
      binaryAsText: true,
      useLogicalType: false,
      snappyCompressionLevel: 0,
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

const BASE_DEFAULTS: fileformat.FileFormatConfig = {
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

// ── Helpers ──────────────────────────────────────────────────────────────────

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

// ── Modal ────────────────────────────────────────────────────────────────────

export default function CreateFileFormatModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<fileformat.FileFormatConfig>(BASE_DEFAULTS);
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sqlPreview, setSqlPreview] = useState("");
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);

  // Preview state
  const [previewSource, setPreviewSource] = useState<"LOCAL" | "STAGE">("LOCAL");
  const [localPath, setLocalPath] = useState("");
  const [stagePath, setStagePath] = useState("");
  const [previewData, setPreviewData] = useState<fileformat.PreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  // tracks whether the user has triggered at least one preview (enables auto-refresh)
  const hasPreviewRef = useRef(false);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then(setQuotedIdentifiersIgnoreCase)
      .catch(() => {});
  }, []);

  useEffect(() => {
    BuildCreateFileFormatSql(db, schema, cfg as any).then(setSqlPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = <K extends keyof fileformat.FileFormatConfig>(key: K, value: fileformat.FileFormatConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const handleTypeChange = (type: string) => {
    const typeDefaults = defaultsForType(type);
    setCfg((prev) => ({ ...prev, ...typeDefaults, name: prev.name, caseSensitive: prev.caseSensitive, orReplace: prev.orReplace, ifNotExists: prev.ifNotExists, comment: prev.comment }));
  };

  const handlePickFile = async () => {
    const path = await PickFileForFormatPreview();
    if (path) setLocalPath(path);
  };

  // Core preview fetcher — accepts explicit arguments so debounced callers
  // always use the values captured at schedule time, not stale closure values.
  const runPreview = useCallback(async (
    path: string,
    stagePth: string,
    source: "LOCAL" | "STAGE",
    currentCfg: fileformat.FileFormatConfig,
  ) => {
    setError(null);
    setPreviewLoading(true);
    try {
      let res: fileformat.PreviewResult;
      if (source === "LOCAL") {
        if (!path) throw new Error("Please select a local file first.");
        res = await GetLocalFilePreview(path, currentCfg as any);
      } else {
        if (!stagePth.trim()) throw new Error("Please enter a stage path, e.g. @MY_STAGE/path/file.csv");
        res = await GetStageFilePreview(stagePth.trim(), currentCfg as any);
      }
      if (res.error) {
        setError(res.error);
        setPreviewData(null);
      } else {
        setPreviewData(res);
      }
    } catch (err) {
      setError(String(err));
      setPreviewData(null);
    } finally {
      setPreviewLoading(false);
    }
  }, []);

  // Manual Preview button — marks the session as "preview loaded" and runs immediately.
  const handlePreview = () => {
    hasPreviewRef.current = true;
    runPreview(localPath, stagePath, previewSource, cfg);
  };

  // Auto-refresh: re-run preview (debounced 500 ms) whenever format options,
  // source, or file path changes — but only after the user has triggered at
  // least one manual preview in this session.
  useEffect(() => {
    if (!hasPreviewRef.current) return;
    const hasTarget = previewSource === "LOCAL" ? !!localPath : !!stagePath.trim();
    if (!hasTarget) return;
    const timer = setTimeout(() => {
      runPreview(localPath, stagePath, previewSource, cfg);
    }, 500);
    return () => clearTimeout(timer);
  }, [cfg, previewSource, localPath, stagePath, runPreview]);

  const handleCreate = async () => {
    if (!cfg.name.trim()) return;
    setCreating(true);
    setError(null);
    try {
      await ExecDDL(sqlPreview);
      onSuccess?.();
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const itemSm: React.CSSProperties = { marginBottom: 8 };

  // ── Render preview table ─────────────────────────────────────────────────

  const renderPreviewTable = () => {
    if (!previewData) return null;
    if (!previewData.columns || previewData.columns.length === 0) {
      return (
        <div style={{ padding: "12px 0", textAlign: "center", color: "var(--text-muted)", fontSize: 12 }}>
          No data to preview
        </div>
      );
    }
    // Use stable index-based keys for dataIndex so that unusual column
    // names (e.g. "a,b,c" from parse-header on a comma CSV with space
    // delimiter) never reach Ant Design's CSS/path internals.
    const colKeys = previewData.columns.map((_, idx) => `_col${idx}`);
    const columns = previewData.columns.map((c, idx) => ({
      title: c,
      dataIndex: colKeys[idx],
      key: colKeys[idx],
      width: 140,
      ellipsis: { showTitle: false },
      render: (text: string) => (
        <Tooltip title={text} placement="topLeft">
          <span style={{ fontFamily: "monospace", fontSize: 11 }}>{text}</span>
        </Tooltip>
      ),
    }));
    const safeRows = (previewData.rows ?? []).map((r, i) => {
      const row: Record<string, string | number> = { key: i };
      previewData.columns.forEach((col, idx) => { row[colKeys[idx]] = r[col]; });
      return row;
    });
    return (
      <Table
        dataSource={safeRows}
        columns={columns}
        size="small"
        pagination={false}
        tableLayout="fixed"
        scroll={{ x: "max-content", y: 240 }}
        style={{ marginTop: 10, border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}
      />
    );
  };

  // ── CSV params form ──────────────────────────────────────────────────────

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

  // ── JSON params form ─────────────────────────────────────────────────────

  const jsonParams = cfg.type === "JSON" && (
    <>
      {divider("JSON Options")}
      <Space direction="vertical" size={4} style={{ marginBottom: 10 }}>
        <Checkbox checked={cfg.stripOuterArray} onChange={e => set("stripOuterArray", e.target.checked)}>
          Strip outer array{help("Remove the outermost [ ] from a JSON array, loading each element as a separate row.")}
        </Checkbox>
        <Checkbox checked={cfg.stripNullValues} onChange={e => set("stripNullValues", e.target.checked)}>
          Strip NULL values{help("Remove key-value pairs where the value is null.")}
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

  // ── AVRO/ORC shared params ───────────────────────────────────────────────

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

  // ── Parquet params ───────────────────────────────────────────────────────

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

  // ── XML params ───────────────────────────────────────────────────────────

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

  // ── Render ───────────────────────────────────────────────────────────────

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <FileTextOutlined style={{ color: "var(--link)" }} />
          <span>Create File Format</span>
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
            icon={<PlusOutlined />}
            onClick={handleCreate}
            disabled={!cfg.name.trim()}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={1040}
      styles={{ body: { paddingTop: 16, maxHeight: "85vh", overflowY: "auto" } }}
    >
      {error && (
        <Alert
          type="error"
          message="Action failed"
          description={error}
          showIcon
          closable
          onClose={() => setError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <div style={{ display: "grid", gridTemplateColumns: "380px 1fr", gap: 24 }}>
        {/* ── Left: Configuration ─────────────────────────────────────── */}
        <div>
          <Form layout="vertical" size="small">
            {/* Name + OR REPLACE / IF NOT EXISTS */}
            <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 10px", alignItems: "end" }}>
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
            </div>

            <Form.Item style={{ marginBottom: 10 }}>
              <ObjectNameCaseControl
                name={cfg.name}
                caseSensitive={cfg.caseSensitive}
                onCaseSensitiveChange={(v) => set("caseSensitive", v)}
                quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
              />
            </Form.Item>

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

            {/* Comment */}
            <Form.Item label="Comment" style={{ marginBottom: 10 }}>
              <Input value={cfg.comment} onChange={e => set("comment", e.target.value)} placeholder="Format description" />
            </Form.Item>

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
        </div>

        {/* ── Right: Preview & SQL ────────────────────────────────────── */}
        <div style={{ display: "flex", flexDirection: "column", gap: 16, minWidth: 0 }}>
          {/* Data Preview */}
          <div style={{
            padding: 14,
            background: "color-mix(in srgb, var(--text) 2%, transparent)",
            borderRadius: 8,
            border: "1px solid var(--border)",
          }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
              <Space size={6}>
                <FileSearchOutlined style={{ color: "var(--accent)" }} />
                <span style={{ fontWeight: 600, fontSize: 13 }}>Data Preview</span>
                <Text type="secondary" style={{ fontSize: 11 }}>up to 50 rows</Text>
              </Space>
              <Radio.Group
                value={previewSource}
                onChange={e => { setPreviewSource(e.target.value); setPreviewData(null); hasPreviewRef.current = false; }}
                size="small"
              >
                <Radio.Button value="LOCAL"><FileOutlined /> Local file</Radio.Button>
                <Radio.Button value="STAGE">
                  <Tooltip title="Stage preview uses Snowflake's compute engine and consumes warehouse credits.">
                    <CloudOutlined /> Stage
                  </Tooltip>
                </Radio.Button>
              </Radio.Group>
            </div>

            <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
              {previewSource === "LOCAL" ? (
                <Input
                  size="small"
                  value={localPath}
                  placeholder="Click to select a local CSV or JSON file…"
                  readOnly
                  style={{ cursor: "pointer", flex: 1 }}
                  onClick={handlePickFile}
                />
              ) : (
                <Input
                  size="small"
                  value={stagePath}
                  placeholder="@DB.SCHEMA.STAGE/path/to/file.csv"
                  onChange={e => setStagePath(e.target.value)}
                  style={{ flex: 1 }}
                />
              )}
              <Button type="primary" size="small" loading={previewLoading} onClick={handlePreview}>
                Preview
              </Button>
            </div>

            {previewSource === "STAGE" && (
              <div style={{ marginTop: 6, fontSize: 11, color: "var(--text-muted)" }}>
                <InfoCircleOutlined style={{ marginRight: 4 }} />
                Stage preview requires an active warehouse and consumes compute credits.
              </div>
            )}

            {renderPreviewTable()}
          </div>

          {/* Generated SQL */}
          <div style={{
            padding: "12px 14px",
            background: "var(--bg)",
            borderRadius: 8,
            border: "1px solid var(--border)",
            flexGrow: 1,
          }}>
            <Text
              type="secondary"
              style={{ fontSize: 11, display: "block", marginBottom: 8, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em" }}
            >
              Generated SQL
            </Text>
            <pre
              style={{
                margin: 0,
                color: "var(--text)",
                fontSize: 12,
                fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
                whiteSpace: "pre-wrap",
                wordBreak: "break-all",
                lineHeight: 1.6,
              }}
            >
              {sqlPreview}
            </pre>
          </div>
        </div>
      </div>
    </Modal>
  );
}
