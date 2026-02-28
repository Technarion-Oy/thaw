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
import { Modal, Select, Switch, Input, Button, Space, Typography, Spin, Segmented } from "antd";
import { UploadOutlined, FolderOpenOutlined, CheckCircleOutlined, FileOutlined } from "@ant-design/icons";
import { ImportTableData, PickDataFileByFormat } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

const DELIMITERS = [
  { value: ",",  label: "Comma (,)" },
  { value: "\t", label: "Tab (\\t)" },
  { value: "|",  label: "Pipe (|)" },
  { value: ";",  label: "Semicolon (;)" },
];

type Format = "CSV" | "JSON" | "PARQUET";

function detectFormat(filePath: string): Format {
  const lower = filePath.toLowerCase();
  if (lower.endsWith(".parquet") || lower.endsWith(".snappy.parquet")) return "PARQUET";
  if (lower.endsWith(".json") || lower.endsWith(".ndjson") || lower.endsWith(".jsonl")) return "JSON";
  return "CSV";
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

export default function ImportTableModal({ db, schema, table, onClose, onSuccess }: Props) {
  const [filePath, setFilePath]           = useState("");
  const [format, setFormat]               = useState<Format>("CSV");
  const [delimiter, setDelimiter]         = useState(",");
  const [customDelimiter, setCustomDelimiter] = useState("");
  const [header, setHeader]               = useState(true);
  const [nullString, setNullString]       = useState("");
  const [overwrite, setOverwrite]         = useState(false);
  const [createTable, setCreateTable]     = useState(false);
  const [newTableName, setNewTableName]   = useState(table);
  const [importing, setImporting]         = useState(false);
  const [error, setError]                 = useState<string | null>(null);
  const [result, setResult]               = useState<ImportResult | null>(null);

  const effectiveTable = createTable ? newTableName.trim() : table;
  const effectiveDelimiter = delimiter === "_custom_" ? customDelimiter : delimiter;

  const pickFile = async () => {
    const path = await PickDataFileByFormat(format);
    if (path) {
      setFilePath(path);
      setFormat(detectFormat(path));
    }
  };

  const handleImport = async () => {
    if (!filePath || !effectiveTable) return;
    setError(null);
    setImporting(true);
    try {
      const r = await ImportTableData({
        database:    db,
        schema,
        table:       effectiveTable,
        filePath,
        format,
        delimiter:   format === "CSV" ? effectiveDelimiter : ",",
        header:      format === "CSV" ? header : false,
        nullString:  format === "CSV" ? nullString : "",
        overwrite:   createTable ? false : overwrite,
        createTable,
      });
      setResult({ rowsLoaded: r.rowsLoaded, filesLoaded: r.filesLoaded, tableName: effectiveTable });
      if (createTable) onSuccess();
    } catch (e) {
      setError(String(e));
    } finally {
      setImporting(false);
    }
  };

  const fileName = filePath ? filePath.split(/[/\\]/).pop() : "";

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <UploadOutlined style={{ color: "var(--link)" }} />
          <span>Import data</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{createTable ? (newTableName || "…") : table}
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
              disabled={!filePath || !effectiveTable || importing}
              loading={importing}
            >
              Import
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
            Import complete
          </div>
          <div style={{ fontSize: 13, color: "var(--text-muted)", marginBottom: 20 }}>
            {result.rowsLoaded.toLocaleString()} row{result.rowsLoaded !== 1 ? "s" : ""} loaded
            {" from "}
            {result.filesLoaded} file{result.filesLoaded !== 1 ? "s" : ""}
            {" into "}
            <Text style={{ fontFamily: "monospace" }}>{db}.{schema}.{result.tableName}</Text>
          </div>
        </div>
      ) : (
        /* ── Options ── */
        <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>
          {/* File picker */}
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>Source file</div>
            <Space.Compact style={{ width: "100%" }}>
              <Input
                value={fileName}
                placeholder="Choose a file…"
                readOnly
                onClick={pickFile}
                style={{ cursor: "pointer" }}
                prefix={filePath ? <FileOutlined style={{ color: "var(--text-muted)" }} /> : undefined}
                title={filePath}
              />
              <Button icon={<FolderOpenOutlined />} onClick={pickFile}>
                Browse
              </Button>
            </Space.Compact>
          </div>

          {/* Format */}
          <div>
            <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>
              Format
              <span style={{ color: "var(--text-faint)", marginLeft: 6 }}>(auto-detected from file extension)</span>
            </div>
            <Segmented
              value={format}
              onChange={(v) => setFormat(v as Format)}
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
                <Text style={{ fontSize: 13 }}>First row is header</Text>
                <Switch checked={header} onChange={setHeader} size="small" />
              </div>

              <div>
                <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>
                  Null value string
                  <span style={{ color: "var(--text-faint)", marginLeft: 6 }}>(empty = treat as NULL)</span>
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

          {/* Divider */}
          <div style={{ borderTop: "1px solid var(--border)" }} />

          {/* Create new table */}
          <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
            <div>
              <Text style={{ fontSize: 13 }}>Create new table from data</Text>
              <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                Infers schema from file; creates table if it doesn't exist
              </div>
            </div>
            <Switch
              checked={createTable}
              onChange={(v) => { setCreateTable(v); if (v) setOverwrite(false); }}
              size="small"
            />
          </div>

          {createTable && (
            <div>
              <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>New table name</div>
              <Input
                value={newTableName}
                onChange={(e) => setNewTableName(e.target.value)}
                placeholder="Table name"
                autoFocus
              />
            </div>
          )}

          {/* Overwrite — only when not creating a new table */}
          {!createTable && (
            <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
              <div>
                <Text style={{ fontSize: 13 }}>Overwrite existing data</Text>
                <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                  Truncates the table before importing
                </div>
              </div>
              <Switch checked={overwrite} onChange={setOverwrite} size="small" />
            </div>
          )}

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
