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
  Modal, Space, Typography, Button, Table, Alert, Radio, Tooltip, Input,
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
import FileFormatFields from "./FileFormatFields";

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

  const set = <K extends keyof fileformat.FileFormatConfig>(key: K, value: fileformat.FileFormatConfig[K]) => {
    if (key === "type") {
      const type = value as string;
      const typeDefaults = defaultsForType(type);
      setCfg((prev) => ({
        ...prev,
        ...typeDefaults,
        name: prev.name,
        caseSensitive: prev.caseSensitive,
        orReplace: prev.orReplace,
        ifNotExists: prev.ifNotExists,
        comment: prev.comment,
      }));
    } else {
      setCfg((prev) => ({ ...prev, [key]: value }));
    }
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
          <FileFormatFields cfg={cfg} set={set} />
          <div style={{ marginTop: -10, marginBottom: 10 }}>
            <ObjectNameCaseControl
              name={cfg.name}
              caseSensitive={cfg.caseSensitive}
              onCaseSensitiveChange={(v) => set("caseSensitive", v)}
              quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
            />
          </div>
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
                  onChange={(e) => setStagePath(e.target.value)}
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
