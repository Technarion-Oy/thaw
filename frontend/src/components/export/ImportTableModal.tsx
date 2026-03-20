// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useRef, useEffect } from "react";
import {
  Modal, Select, Switch, Input, InputNumber, Button, Space, Typography,
  Spin, Segmented, Tooltip, Collapse, Tabs,
} from "antd";
import {
  UploadOutlined, FolderOpenOutlined, CheckCircleOutlined,
  CloseOutlined, FileOutlined, SettingOutlined, InfoCircleOutlined,
} from "@ant-design/icons";
import { ImportTableData, PickDataFilesByFormat, ReadFileHead, SuggestImportOptions } from "../../../wailsjs/go/main/App";
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

// ── Preview helpers ───────────────────────────────────────────────────────────

function unescapeDelimiter(raw: string): string {
  if (raw === "\\t") return "\t";
  if (raw === "\\n") return "\n";
  if (raw === "\\r") return "\r";
  return raw;
}

function parseOneCsvRow(line: string, delimiter: string): string[] {
  const cols: string[] = [];
  let cur = "";
  let inQuote = false;
  let i = 0;
  while (i < line.length) {
    const ch = line[i];
    if (inQuote) {
      if (ch === '"' && i + 1 < line.length && line[i + 1] === '"') {
        cur += '"'; i += 2; // escaped double-quote
      } else if (ch === '"') {
        inQuote = false; i++;
      } else {
        cur += ch; i++;
      }
    } else {
      if (ch === '"' && cur === "") {
        inQuote = true; i++;
      } else if (delimiter.length > 0 && line.startsWith(delimiter, i)) {
        cols.push(cur);
        cur = "";
        i += delimiter.length;
      } else {
        cur += ch; i++;
      }
    }
  }
  cols.push(cur);
  return cols;
}

function parseCsvPreview(
  content: string,
  rawDelimiter: string,
  hasHeader: boolean,
  maxRows: number,
): { headers: string[]; rows: string[][]; truncated: boolean } {
  const delimiter = unescapeDelimiter(rawDelimiter) || ",";
  const lines = content
    .replace(/\r\n/g, "\n")
    .replace(/\r/g, "\n")
    .split("\n")
    .filter((l) => l.trim() !== "");

  if (lines.length === 0) return { headers: [], rows: [], truncated: false };

  let headers: string[];
  let dataLines: string[];

  if (hasHeader) {
    headers = parseOneCsvRow(lines[0], delimiter);
    dataLines = lines.slice(1);
  } else {
    const firstCols = parseOneCsvRow(lines[0], delimiter);
    headers = firstCols.map((_, i) => `Column ${i + 1}`);
    dataLines = lines;
  }

  const truncated = dataLines.length > maxRows;
  const rows = dataLines.slice(0, maxRows).map((l) => parseOneCsvRow(l, delimiter));
  return { headers, rows, truncated };
}

type JsonParsed =
  | { kind: "table"; headers: string[]; rows: string[][]; truncated: boolean }
  | { kind: "error"; message: string };

function parseJsonPreview(content: string, maxRows: number): JsonParsed {
  if (!content.trim()) return { kind: "error", message: "Empty file" };

  let data: unknown;

  // Try full JSON parse first
  try {
    data = JSON.parse(content.trim());
  } catch {
    // Try NDJSON / JSON Lines (one JSON value per line)
    const objects: unknown[] = [];
    for (const line of content.trim().split("\n")) {
      if (!line.trim()) continue;
      try { objects.push(JSON.parse(line.trim())); } catch { /* skip bad lines */ }
    }
    if (objects.length > 0) {
      data = objects;
    } else {
      return { kind: "error", message: "Could not parse JSON (preview content may be truncated)" };
    }
  }

  if (Array.isArray(data)) {
    const items = data.slice(0, maxRows);
    const truncated = data.length > maxRows;
    if (items.length > 0 && items[0] !== null && typeof items[0] === "object") {
      // Collect headers from the first few objects to handle sparse records
      const headerSet = new Set<string>();
      items.slice(0, 5).forEach((item) =>
        Object.keys(item as Record<string, unknown>).forEach((k) => headerSet.add(k))
      );
      const headers = Array.from(headerSet);
      const rows = items.map((item) =>
        headers.map((h) => {
          const v = (item as Record<string, unknown>)[h];
          if (v === null) return "null";
          if (v === undefined) return "";
          if (typeof v === "object") return JSON.stringify(v);
          return String(v);
        })
      );
      return { kind: "table", headers, rows, truncated };
    }
    // Array of primitives
    return {
      kind: "table",
      headers: ["Value"],
      rows: items.map((v) => [v === null ? "null" : String(v)]),
      truncated,
    };
  }

  if (typeof data === "object" && data !== null) {
    const entries = Object.entries(data as Record<string, unknown>).slice(0, maxRows);
    const truncated = Object.keys(data).length > maxRows;
    return {
      kind: "table",
      headers: ["Key", "Value"],
      rows: entries.map(([k, v]) => [
        k,
        v === null ? "null" : typeof v === "object" ? JSON.stringify(v) : String(v),
      ]),
      truncated,
    };
  }

  return { kind: "error", message: "Unsupported JSON structure for tabular preview" };
}

// ── Preview sub-components ────────────────────────────────────────────────────

function PreviewTable({
  headers,
  rows,
  truncated,
}: {
  headers: string[];
  rows: string[][];
  truncated: boolean;
}) {
  if (headers.length === 0 && rows.length === 0) {
    return <Text type="secondary" style={{ fontSize: 11 }}>No data to preview</Text>;
  }
  return (
    <div>
      <div style={{ overflowX: "auto" }}>
        <table style={{ borderCollapse: "collapse", fontSize: 11, fontFamily: "monospace" }}>
          {headers.length > 0 && (
            <thead>
              <tr>
                {headers.map((h, i) => (
                  <th
                    key={i}
                    style={{
                      border: "1px solid var(--border)",
                      padding: "2px 6px",
                      background: "var(--bg-secondary)",
                      fontWeight: 600,
                      whiteSpace: "nowrap",
                      maxWidth: 160,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                    }}
                  >
                    {h || <em style={{ color: "var(--text-muted)" }}>(empty)</em>}
                  </th>
                ))}
              </tr>
            </thead>
          )}
          <tbody>
            {rows.map((row, ri) => (
              <tr key={ri}>
                {row.map((cell, ci) => (
                  <td
                    key={ci}
                    style={{
                      border: "1px solid var(--border)",
                      padding: "2px 6px",
                      maxWidth: 160,
                      overflow: "hidden",
                      textOverflow: "ellipsis",
                      whiteSpace: "nowrap",
                    }}
                  >
                    {cell === "" ? (
                      <em style={{ color: "var(--text-muted)", fontSize: 10 }}>(empty)</em>
                    ) : (
                      cell
                    )}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {truncated && (
        <Text type="secondary" style={{ fontSize: 10, display: "block", marginTop: 4 }}>
          Showing first 10 rows
        </Text>
      )}
    </div>
  );
}

function CsvFilePrev({
  content,
  fieldDelimiter,
  parseHeader,
}: {
  content: string;
  fieldDelimiter: string;
  parseHeader: boolean;
}) {
  const [view, setView] = useState<"parsed" | "raw">("parsed");
  const { headers, rows, truncated } = parseCsvPreview(content, fieldDelimiter, parseHeader, 10);
  return (
    <div>
      <Segmented
        size="small"
        value={view}
        onChange={(v) => setView(v as "parsed" | "raw")}
        options={[
          { label: "Parsed", value: "parsed" },
          { label: "Raw", value: "raw" },
        ]}
        style={{ marginBottom: 8 }}
      />
      {view === "parsed" ? (
        <PreviewTable headers={headers} rows={rows} truncated={truncated} />
      ) : (
        <pre
          style={{
            fontSize: 11,
            fontFamily: "monospace",
            maxHeight: 180,
            overflowY: "auto",
            overflowX: "auto",
            background: "var(--bg-secondary)",
            border: "1px solid var(--border)",
            borderRadius: 4,
            padding: "6px 8px",
            margin: 0,
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
          }}
        >
          {content.length > 4096 ? content.slice(0, 4096) + "\n…(truncated)" : content}
        </pre>
      )}
    </div>
  );
}

function JsonFilePrev({ content }: { content: string }) {
  const [view, setView] = useState<"parsed" | "raw">("parsed");
  const parsed = parseJsonPreview(content, 10);
  return (
    <div>
      <Segmented
        size="small"
        value={view}
        onChange={(v) => setView(v as "parsed" | "raw")}
        options={[
          { label: "Parsed", value: "parsed" },
          { label: "Raw", value: "raw" },
        ]}
        style={{ marginBottom: 8 }}
      />
      {view === "parsed" ? (
        parsed.kind === "error" ? (
          <Text type="secondary" style={{ fontSize: 11 }}>
            {parsed.message} — switch to Raw view to inspect the content
          </Text>
        ) : (
          <PreviewTable
            headers={parsed.headers}
            rows={parsed.rows}
            truncated={parsed.truncated}
          />
        )
      ) : (
        <pre
          style={{
            fontSize: 11,
            fontFamily: "monospace",
            maxHeight: 180,
            overflowY: "auto",
            overflowX: "auto",
            background: "var(--bg-secondary)",
            border: "1px solid var(--border)",
            borderRadius: 4,
            padding: "6px 8px",
            margin: 0,
            whiteSpace: "pre-wrap",
            wordBreak: "break-all",
          }}
        >
          {content.length > 4096 ? content.slice(0, 4096) + "\n…(truncated)" : content}
        </pre>
      )}
    </div>
  );
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

  // File preview state — keyed by file path; null = loading, string = content (or "" on error)
  const [fileHeads, setFileHeads] = useState<Record<string, string | null>>({});
  const pendingLoads = useRef<Set<string>>(new Set());

  const effectiveTable = createTable ? newTableName.trim() : (table || targetTable.trim());

  const setOpt = (k: keyof FormatOptions, v: any) =>
    setOptions((prev) => ({ ...prev, [k]: v }));

  const changeFormat = (f: Format) => {
    setFormat(f);
    setOptions(defaultOptions(f));
    setAiExplanation(null);
    setAiError(null);
  };

  // AI format suggestion state
  const [aiSuggesting, setAiSuggesting]     = useState(false);
  const [aiExplanation, setAiExplanation]   = useState<string | null>(null);
  const [aiError, setAiError]               = useState<string | null>(null);
  const [collapseOpen, setCollapseOpen]     = useState<string[]>([]);

  const runAiSuggest = async () => {
    // Find first successfully loaded file sample
    const firstLoaded = filePaths.slice(0, 5).find(
      (fp) => fileHeads[fp] !== null && fileHeads[fp] !== undefined && fileHeads[fp] !== ""
    );
    if (!firstLoaded) return;
    const sample = fileHeads[firstLoaded] as string;

    setAiSuggesting(true);
    setAiError(null);
    setAiExplanation(null);
    try {
      const raw = await SuggestImportOptions(format, sample);
      if (!raw) { setAiError("No suggestion returned"); return; }
      const obj = JSON.parse(raw);
      const apply = (key: keyof FormatOptions, val: unknown) => {
        if (val !== undefined) setOpt(key, val);
      };
      if (format === "CSV") {
        if (obj.fieldDelimiter            !== undefined) apply("fieldDelimiter", obj.fieldDelimiter);
        if (obj.parseHeader               !== undefined) apply("parseHeader", obj.parseHeader);
        if (obj.fieldOptionallyEnclosedBy !== undefined) apply("fieldOptionallyEnclosedBy", obj.fieldOptionallyEnclosedBy);
        if (obj.encoding                  !== undefined) apply("encoding", obj.encoding);
        if (obj.compression               !== undefined) apply("compression", obj.compression);
        if (obj.recordDelimiter           !== undefined) apply("recordDelimiter", obj.recordDelimiter);
      } else if (format === "JSON") {
        if (obj.multiLine      !== undefined) apply("multiLine", obj.multiLine);
        if (obj.stripOuterArray !== undefined) apply("stripOuterArray", obj.stripOuterArray);
        if (obj.compression    !== undefined) apply("compression", obj.compression);
      }
      if (obj.explanation) setAiExplanation(obj.explanation);
      setCollapseOpen(["opts"]);
    } catch (e) {
      setAiError(String(e));
    } finally {
      setAiSuggesting(false);
    }
  };

  const handleAiSuggest = () => {
    Modal.confirm({
      title: "Send file content to AI provider?",
      content: (
        <div style={{ fontSize: 13, lineHeight: 1.6 }}>
          <p style={{ marginBottom: 8 }}>
            To suggest format options, a sample of your file content (up to 64 KB)
            will be sent to your configured AI provider.
          </p>
          <p style={{ margin: 0, color: "var(--text-muted)" }}>
            Do not use this feature if your files contain sensitive or confidential data
            that should not leave this machine.
          </p>
        </div>
      ),
      okText: "Send & Suggest",
      cancelText: "Cancel",
      onOk: runAiSuggest,
    });
  };

  // Load file heads for CSV / JSON previews
  useEffect(() => {
    if (format !== "CSV" && format !== "JSON") return;
    filePaths.slice(0, 5).forEach((fp) => {
      if (pendingLoads.current.has(fp)) return;
      pendingLoads.current.add(fp);
      setFileHeads((prev) => ({ ...prev, [fp]: null }));
      ReadFileHead(fp, 65536)
        .then((content) => setFileHeads((prev) => ({ ...prev, [fp]: content })))
        .catch(() => setFileHeads((prev) => ({ ...prev, [fp]: "" })));
    });
  }, [filePaths, format]);

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

  // ── File preview section (CSV / JSON only) ─────────────────────────────────

  const renderPreview = () => {
    if (format !== "CSV" && format !== "JSON") return null;
    if (filePaths.length === 0) return null;

    const previewFiles = filePaths.slice(0, 5);
    const hasMore = filePaths.length > 5;

    const renderFileContent = (fp: string) => {
      const head = fileHeads[fp];
      if (head === null || head === undefined) {
        return (
          <div style={{ padding: "8px 0", display: "flex", alignItems: "center", gap: 8 }}>
            <Spin size="small" />
            <Text type="secondary" style={{ fontSize: 11 }}>Loading preview…</Text>
          </div>
        );
      }
      if (head === "") {
        return <Text type="secondary" style={{ fontSize: 11 }}>Preview not available</Text>;
      }
      if (format === "CSV") {
        return (
          <CsvFilePrev
            content={head}
            fieldDelimiter={options.fieldDelimiter}
            parseHeader={options.parseHeader}
          />
        );
      }
      // JSON — JsonFilePrev manages its own parsed/raw toggle state
      return <JsonFilePrev content={head} />;
    };

    const wrapContent = (fp: string) => (
      <div style={{ maxHeight: 220, overflowY: "auto", padding: "6px 2px" }}>
        {renderFileContent(fp)}
      </div>
    );

    return (
      <div>
        <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>
          File preview
          {hasMore && (
            <span style={{ marginLeft: 6, fontSize: 11 }}>
              — first 5 of {filePaths.length} files
            </span>
          )}
        </div>
        {previewFiles.length === 1 ? (
          <div
            style={{
              border: "1px solid var(--border)",
              borderRadius: 6,
              padding: "8px 12px",
              overflowX: "auto",
            }}
          >
            {wrapContent(previewFiles[0])}
          </div>
        ) : (
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Tabs
              size="small"
              style={{ padding: "0 8px" }}
              items={previewFiles.map((fp) => ({
                key: fp,
                label: (
                  <Tooltip title={fp} mouseEnterDelay={0.5}>
                    <span style={{ fontSize: 12, maxWidth: 100, overflow: "hidden", textOverflow: "ellipsis", display: "inline-block", whiteSpace: "nowrap" }}>
                      {baseName(fp)}
                    </span>
                  </Tooltip>
                ),
                children: wrapContent(fp),
              }))}
            />
          </div>
        )}
      </div>
    );
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
      width={600}
      styles={{ body: { paddingTop: 20, maxHeight: "80vh", overflowY: "auto" } }}
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

          {/* ── File preview (CSV / JSON) ── */}
          {renderPreview()}

          {/* ── Format options (collapsible) ── */}
          <Collapse
            size="small"
            ghost
            activeKey={collapseOpen}
            onChange={(keys) => setCollapseOpen(Array.isArray(keys) ? keys : [keys])}
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
              extra: (format === "CSV" || format === "JSON") && filePaths.length > 0 ? (
                <Space size={4} onClick={(e) => e.stopPropagation()}>
                  <Tooltip title="Analyze file content with AI and apply suggested format options">
                    <Button
                      size="small"
                      type="text"
                      loading={aiSuggesting}
                      disabled={!filePaths.slice(0, 5).some((fp) => fileHeads[fp])}
                      onClick={handleAiSuggest}
                      style={{ fontSize: 12 }}
                    >
                      {!aiSuggesting && <span style={{ marginRight: 4 }}>✨</span>}
                      AI Suggest
                    </Button>
                  </Tooltip>
                  <Tooltip
                    title="A sample of your file content (up to 64 KB) is sent to your configured AI provider to generate these suggestions. No data is stored by Thaw."
                    overlayStyle={{ maxWidth: 300 }}
                  >
                    <InfoCircleOutlined style={{ fontSize: 12, color: "var(--text-muted)", cursor: "default" }} />
                  </Tooltip>
                </Space>
              ) : undefined,
              children: (
                <div style={{ paddingTop: 4 }}>
                  {formatOptionsPanel()}
                </div>
              ),
            }]}
          />

          {/* ── AI suggestion feedback ── */}
          {(aiExplanation || aiError) && (
            <div
              style={{
                fontSize: 11,
                padding: "6px 10px",
                borderRadius: 4,
                background: aiError ? "rgba(248,81,73,0.08)" : "rgba(63,185,80,0.08)",
                border: `1px solid ${aiError ? "rgba(248,81,73,0.3)" : "rgba(63,185,80,0.3)"}`,
                color: aiError ? "#f85149" : "var(--text)",
              }}
            >
              {aiError ? `AI Suggest failed: ${aiError}` : `AI: ${aiExplanation}`}
            </div>
          )}

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
