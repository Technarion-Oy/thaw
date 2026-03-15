// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Modal, Select, Switch, Input, Button, Space, Typography, Spin, Segmented } from "antd";
import { DownloadOutlined, FolderOpenOutlined, CheckCircleOutlined } from "@ant-design/icons";
import { ExportTableData, PickDirectory, ListObjects } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

const CSV_COMPRESSIONS = [
  { value: "NONE",   label: "None" },
  { value: "GZIP",   label: "GZIP" },
  { value: "BROTLI", label: "Brotli" },
  { value: "ZSTD",   label: "Zstd" },
];

const JSON_COMPRESSIONS = [
  { value: "NONE",   label: "None" },
  { value: "GZIP",   label: "GZIP" },
  { value: "BROTLI", label: "Brotli" },
  { value: "ZSTD",   label: "Zstd" },
];

const PARQUET_COMPRESSIONS = [
  { value: "SNAPPY", label: "Snappy (default)" },
  { value: "NONE",   label: "None" },
  { value: "ZSTD",   label: "Zstd" },
];

const DELIMITERS = [
  { value: ",",  label: "Comma (,)" },
  { value: "\t", label: "Tab (\\t)" },
  { value: "|",  label: "Pipe (|)" },
  { value: ";",  label: "Semicolon (;)" },
];

type Format = "CSV" | "JSON" | "PARQUET";

interface ExportResult {
  rowsUnloaded: number;
  files: string[];
  outputDir: string;
}

interface Props {
  db: string;
  schema: string;
  table: string;
  onClose: () => void;
}

export default function ExportTableModal({ db, schema, table, onClose }: Props) {
  const [format, setFormat]               = useState<Format>("CSV");
  const [compression, setCompression]     = useState("NONE");
  const [delimiter, setDelimiter]         = useState(",");
  const [customDelimiter, setCustomDelimiter] = useState("");
  const [header, setHeader]               = useState(true);
  const [nullString, setNullString]       = useState("");
  const [outputDir, setOutputDir]         = useState("");
  const [exporting, setExporting]         = useState(false);
  const [error, setError]                 = useState<string | null>(null);
  const [result, setResult]               = useState<ExportResult | null>(null);
  const [selectedTable, setSelectedTable] = useState(table);
  const [tableOptions, setTableOptions]   = useState<string[]>([]);
  const [loadingTables, setLoadingTables] = useState(false);

  useEffect(() => {
    if (table !== "") return;
    setLoadingTables(true);
    ListObjects(db, schema)
      .then((objs) => {
        const tables = objs
          .filter((o) => o.kind?.toUpperCase() === "TABLE")
          .map((o) => o.name)
          .sort();
        setTableOptions(tables);
      })
      .catch(() => {})
      .finally(() => setLoadingTables(false));
  }, [db, schema, table]);

  const effectiveTable = table || selectedTable;

  const changeFormat = (f: Format) => {
    setFormat(f);
    setCompression(f === "PARQUET" ? "SNAPPY" : "NONE");
  };

  const pickDir = async () => {
    const dir = await PickDirectory();
    if (dir) setOutputDir(dir);
  };

  const compressionOptions =
    format === "PARQUET" ? PARQUET_COMPRESSIONS :
    format === "JSON"    ? JSON_COMPRESSIONS    :
    CSV_COMPRESSIONS;

  const effectiveDelimiter = delimiter === "_custom_" ? customDelimiter : delimiter;

  const handleExport = async () => {
    if (!outputDir || !effectiveTable) return;
    setError(null);
    setExporting(true);
    try {
      const r = await ExportTableData({
        database:    db,
        schema,
        table:       effectiveTable,
        outputDir,
        format,
        compression,
        delimiter:   format === "CSV" ? effectiveDelimiter : ",",
        header:      format === "CSV" ? header : false,
        nullString:  format === "CSV" ? nullString : "",
      });
      setResult({
        rowsUnloaded: r.rowsUnloaded,
        files:        r.files ?? [],
        outputDir:    r.outputDir,
      });
    } catch (e) {
      setError(String(e));
    } finally {
      setExporting(false);
    }
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <DownloadOutlined style={{ color: "var(--link)" }} />
          <span>Export table data</span>
          {effectiveTable && (
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
              {db}.{schema}.{effectiveTable}
            </Text>
          )}
        </Space>
      }
      onCancel={onClose}
      footer={
        result ? (
          <Button type="primary" onClick={onClose}>Done</Button>
        ) : (
          <Space style={{ justifyContent: "flex-end", display: "flex" }}>
            <Button onClick={onClose} disabled={exporting}>Cancel</Button>
            <Button
              type="primary"
              icon={<DownloadOutlined />}
              onClick={handleExport}
              disabled={!outputDir || !effectiveTable || exporting}
              loading={exporting}
            >
              Export
            </Button>
          </Space>
        )
      }
      width={520}
      styles={{ body: { paddingTop: 20 } }}
    >
      {result ? (
        /* ── Success state ── */
        <div style={{ textAlign: "center", padding: "16px 0" }}>
          <CheckCircleOutlined style={{ fontSize: 40, color: "#3fb950", marginBottom: 16 }} />
          <div style={{ fontSize: 15, fontWeight: 600, color: "var(--text)", marginBottom: 8 }}>
            Export complete
          </div>
          <div style={{ fontSize: 13, color: "var(--text-muted)", marginBottom: 20 }}>
            {result.rowsUnloaded.toLocaleString()} row{result.rowsUnloaded !== 1 ? "s" : ""} exported
            {result.files.length > 0 && ` to ${result.files.length} file${result.files.length !== 1 ? "s" : ""}`}
          </div>
          <div
            style={{
              padding: "10px 14px",
              background: "var(--bg)",
              border: "1px solid var(--border)",
              borderRadius: 6,
              textAlign: "left",
            }}
          >
            <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>Output directory</Text>
            <Text style={{ fontFamily: "monospace", fontSize: 12, wordBreak: "break-all" }}>
              {result.outputDir}
            </Text>
            {result.files.length > 0 && (
              <div style={{ marginTop: 10 }}>
                <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>Files</Text>
                {result.files.map((f) => (
                  <div key={f} style={{ fontFamily: "monospace", fontSize: 11, color: "var(--text-muted)", marginBottom: 2 }}>
                    {f}
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      ) : (
        /* ── Options ── */
        <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>
          {/* Table selector (schema-level mode) */}
          {table === "" && (
            <div>
              <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Table</div>
              <Select
                value={selectedTable || undefined}
                onChange={setSelectedTable}
                options={tableOptions.map((t) => ({ value: t, label: t }))}
                placeholder="Select a table…"
                style={{ width: "100%" }}
                showSearch
                loading={loadingTables}
                notFoundContent={loadingTables ? <Spin size="small" /> : "No tables found"}
              />
            </div>
          )}

          {/* Format */}
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Format</div>
            <Segmented
              value={format}
              onChange={(v) => changeFormat(v as Format)}
              options={["CSV", "JSON", "PARQUET"]}
              block
            />
          </div>

          {/* CSV-specific options */}
          {format === "CSV" && (
            <>
              <div>
                <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Delimiter</div>
                <Space direction="vertical" style={{ width: "100%" }} size={6}>
                  <Select
                    value={delimiter}
                    onChange={setDelimiter}
                    options={[...DELIMITERS, { value: "_custom_", label: "Custom…" }]}
                    style={{ width: "100%" }}
                  />
                  {delimiter === "_custom_" && (
                    <Input
                      value={customDelimiter}
                      onChange={(e) => setCustomDelimiter(e.target.value)}
                      placeholder="Enter delimiter character"
                      maxLength={3}
                    />
                  )}
                </Space>
              </div>

              <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
                <Text style={{ fontSize: 13 }}>Include header row</Text>
                <Switch checked={header} onChange={setHeader} size="small" />
              </div>

              <div>
                <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>
                  Null value string
                  <span style={{ color: "var(--text-faint)", marginLeft: 6 }}>(empty = default NULL)</span>
                </div>
                <Input
                  value={nullString}
                  onChange={(e) => setNullString(e.target.value)}
                  placeholder="e.g. \\N or NULL"
                  allowClear
                />
              </div>
            </>
          )}

          {/* Compression */}
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Compression</div>
            <Select
              value={compression}
              onChange={setCompression}
              options={compressionOptions}
              style={{ width: "100%" }}
            />
          </div>

          {/* Output directory */}
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Output directory</div>
            <Space.Compact style={{ width: "100%" }}>
              <Input
                value={outputDir}
                placeholder="Choose a folder…"
                readOnly
                onClick={pickDir}
                style={{ cursor: "pointer" }}
              />
              <Button icon={<FolderOpenOutlined />} onClick={pickDir}>
                Browse
              </Button>
            </Space.Compact>
          </div>

          {/* Error */}
          {error && (
            <div
              style={{
                padding: "10px 14px",
                background: "rgba(248,81,73,0.08)",
                border: "1px solid rgba(248,81,73,0.3)",
                borderRadius: 6,
                color: "#f85149",
                fontFamily: "monospace",
                fontSize: 12,
                wordBreak: "break-word",
              }}
            >
              {error}
            </div>
          )}

          {exporting && (
            <div style={{ textAlign: "center", padding: "4px 0" }}>
              <Spin size="small" />
              <span style={{ marginLeft: 10, fontSize: 12, color: "var(--text-muted)" }}>
                Exporting… this may take a while for large tables
              </span>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}
