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
import {
  Modal, Form, Input, Select, Checkbox, Space,
  Typography, Divider, InputNumber, Button, Table, Alert, Radio,
} from "antd";
import {
  FileTextOutlined, PlusOutlined, FileSearchOutlined,
  CloudOutlined, FileOutlined,
} from "@ant-design/icons";
import {
  ExecDDL, GetQuotedIdentifiersIgnoreCase, BuildCreateFileFormatSql,
  PickFileForFormatPreview, GetLocalFilePreview, GetStageFilePreview,
} from "../../../wailsjs/go/main/App";
import { fileformat } from "../../../wailsjs/go/models";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULTS: fileformat.FileFormatConfig = {
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
  validateUTF8: true,
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
  // Wails-generated models might have more fields, but these are the ones we care about.
} as fileformat.FileFormatConfig;

export default function CreateFileFormatModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<fileformat.FileFormatConfig>(DEFAULTS);
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

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase().then(setQuotedIdentifiersIgnoreCase);
  }, []);

  useEffect(() => {
    BuildCreateFileFormatSql(db, schema, cfg).then(setSqlPreview);
  }, [db, schema, cfg]);

  const set = <K extends keyof fileformat.FileFormatConfig>(key: K, value: fileformat.FileFormatConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const handlePickFile = async () => {
    const path = await PickFileForFormatPreview();
    if (path) setLocalPath(path);
  };

  const handlePreview = async () => {
    setError(null);
    setPreviewLoading(true);
    try {
      let res: fileformat.PreviewResult;
      if (previewSource === "LOCAL") {
        if (!localPath) throw new Error("Please select a local file first.");
        res = await GetLocalFilePreview(localPath, cfg);
      } else {
        if (!stagePath) throw new Error("Please enter a stage path (e.g. @MY_STAGE/file.csv).");
        res = await GetStageFilePreview(stagePath, cfg);
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
  };

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

  const divider = (label: string) => (
    <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "16px 0 8px" }}>
      {label}
    </Divider>
  );

  const renderPreviewTable = () => {
    if (!previewData) return null;
    if (previewData.columns.length === 0) {
      return <div style={{ padding: 16, textAlign: "center", color: "var(--text-muted)" }}>No data to preview</div>;
    }

    const columns = previewData.columns.map(c => ({
      title: c,
      dataIndex: c,
      key: c,
      width: 150,
      render: (text: string) => (
        <div style={{
          maxWidth: 134, // 150 - 16px padding
          overflow: "hidden",
          textOverflow: "ellipsis",
          whiteSpace: "nowrap"
        }} title={text}>
          {text}
        </div>
      ),
    }));

    return (
      <Table
        dataSource={previewData.rows.map((r, i) => ({ ...r, key: i }))}
        columns={columns}
        size="small"
        pagination={false}
        tableLayout="fixed"
        scroll={{ x: "max-content", y: 300 }}
        style={{ marginTop: 12, border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}
      />
    );
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <FileTextOutlined style={{ color: "var(--link)" }} />
          <span>Create file format</span>
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
      width={1000}
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

      <div style={{ display: "grid", gridTemplateColumns: "350px 1fr", gap: 24 }}>
        {/* Left Column: Configuration */}
        <div>
          <Form layout="vertical" size="small">
            <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 12px", alignItems: "end" }}>
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
            <Form.Item style={{ marginBottom: 12 }}>
              <ObjectNameCaseControl
                name={cfg.name}
                caseSensitive={cfg.caseSensitive}
                onCaseSensitiveChange={(v) => set("caseSensitive", v)}
                quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
              />
            </Form.Item>

            <Form.Item label="Format type" style={{ marginBottom: 12 }}>
              <Select
                value={cfg.type}
                onChange={(v) => set("type", v)}
                options={[
                  { value: "CSV", label: "CSV" },
                  { value: "JSON", label: "JSON" },
                  { value: "AVRO", label: "AVRO" },
                  { value: "ORC", label: "ORC" },
                  { value: "PARQUET", label: "PARQUET" },
                  { value: "XML", label: "XML" },
                ]}
              />
            </Form.Item>

            {divider("General Options")}
            <Form.Item label="Compression" style={{ marginBottom: 12 }}>
              <Select
                value={cfg.compression}
                onChange={v => set("compression", v)}
                options={["AUTO", "GZIP", "BZ2", "BROTLI", "ZSTD", "DEFLATE", "RAW_DEFLATE", "NONE"].map(o => ({ value: o, label: o }))}
              />
            </Form.Item>
            <Form.Item label="Comment" style={{ marginBottom: 12 }}>
              <Input value={cfg.comment} onChange={e => set("comment", e.target.value)} placeholder="Format description" />
            </Form.Item>

            {cfg.type === "CSV" && (
              <>
                {divider("CSV Options")}
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 12px" }}>
                  <Form.Item label="Record delimiter" style={{ marginBottom: 12 }}>
                    <Input value={cfg.recordDelimiter} onChange={e => set("recordDelimiter", e.target.value)} />
                  </Form.Item>
                  <Form.Item label="Field delimiter" style={{ marginBottom: 12 }}>
                    <Input value={cfg.fieldDelimiter} onChange={e => set("fieldDelimiter", e.target.value)} />
                  </Form.Item>
                </div>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 12px" }}>
                  <Form.Item label="Skip header" style={{ marginBottom: 12 }}>
                    <InputNumber min={0} value={cfg.skipHeader} onChange={v => set("skipHeader", v ?? 0)} style={{ width: "100%" }} />
                  </Form.Item>
                  <Form.Item label="Binary format" style={{ marginBottom: 12 }}>
                    <Select value={cfg.binaryFormat} onChange={v => set("binaryFormat", v)} options={["HEX", "BASE64", "UTF8"].map(o => ({ value: o, label: o }))} />
                  </Form.Item>
                </div>
                <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 12px" }}>
                  <Form.Item label="Escape char" style={{ marginBottom: 12 }}>
                    <Input value={cfg.escape} onChange={e => set("escape", e.target.value)} />
                  </Form.Item>
                  <Form.Item label="Optionally enclosed by" style={{ marginBottom: 12 }}>
                    <Select value={cfg.fieldOptionallyEnclosedBy} onChange={v => set("fieldOptionallyEnclosedBy", v)} options={["NONE", "'", "\""].map(o => ({ value: o, label: o }))} />
                  </Form.Item>
                </div>
                <Space direction="vertical" size={4} style={{ marginBottom: 12 }}>
                  <Checkbox checked={cfg.trimSpace} onChange={e => set("trimSpace", e.target.checked)}>Trim space</Checkbox>
                  <Checkbox checked={cfg.skipBlankLines} onChange={e => set("skipBlankLines", e.target.checked)}>Skip blank lines</Checkbox>
                  <Checkbox checked={cfg.errorOnColumnCountMismatch} onChange={e => set("errorOnColumnCountMismatch", e.target.checked)}>Error on column mismatch</Checkbox>
                  <Checkbox checked={cfg.emptyFieldAsNull} onChange={e => set("emptyFieldAsNull", e.target.checked)}>Empty field as NULL</Checkbox>
                </Space>
              </>
            )}

            {cfg.type === "JSON" && (
              <>
                {divider("JSON Options")}
                <Space direction="vertical" size={4} style={{ marginBottom: 12 }}>
                  <Checkbox checked={cfg.enableOctal} onChange={e => set("enableOctal", e.target.checked)}>Enable octal</Checkbox>
                  <Checkbox checked={cfg.allowDuplicate} onChange={e => set("allowDuplicate", e.target.checked)}>Allow duplicate keys</Checkbox>
                  <Checkbox checked={cfg.stripOuterArray} onChange={e => set("stripOuterArray", e.target.checked)}>Strip outer array</Checkbox>
                  <Checkbox checked={cfg.stripNullValues} onChange={e => set("stripNullValues", e.target.checked)}>Strip NULL values</Checkbox>
                  <Checkbox checked={cfg.ignoreUTF8Errors} onChange={e => set("ignoreUTF8Errors", e.target.checked)}>Ignore UTF-8 errors</Checkbox>
                </Space>
              </>
            )}

            {cfg.type === "PARQUET" && (
              <>
                {divider("Parquet Options")}
                <Form.Item label="Snappy compression level" style={{ marginBottom: 12 }}>
                  <InputNumber min={0} value={cfg.snappyCompressionLevel} onChange={v => set("snappyCompressionLevel", v ?? 0)} style={{ width: "100%" }} />
                </Form.Item>
                <Space direction="vertical" size={4} style={{ marginBottom: 12 }}>
                  <Checkbox checked={cfg.binaryAsText} onChange={e => set("binaryAsText", e.target.checked)}>Binary as text</Checkbox>
                  <Checkbox checked={cfg.useLogicalType} onChange={e => set("useLogicalType", e.target.checked)}>Use logical type</Checkbox>
                </Space>
              </>
            )}

            {cfg.type === "XML" && (
              <>
                {divider("XML Options")}
                <Space direction="vertical" size={4} style={{ marginBottom: 12 }}>
                  <Checkbox checked={cfg.preserveSpace} onChange={e => set("preserveSpace", e.target.checked)}>Preserve space</Checkbox>
                  <Checkbox checked={cfg.stripOuterElement} onChange={e => set("stripOuterElement", e.target.checked)}>Strip outer element</Checkbox>
                  <Checkbox checked={cfg.disableSnowflakeData} onChange={e => set("disableSnowflakeData", e.target.checked)}>Disable Snowflake data</Checkbox>
                  <Checkbox checked={cfg.disableAutoConvert} onChange={e => set("disableAutoConvert", e.target.checked)}>Disable auto convert</Checkbox>
                </Space>
              </>
            )}
          </Form>
        </div>

        {/* Right Column: Preview & SQL */}
        <div style={{ display: "flex", flexDirection: "column", gap: 20, minWidth: 0 }}>
          {/* Preview Section */}
          <div style={{ padding: 16, background: "color-mix(in srgb, var(--text) 2%, transparent)", borderRadius: 8, border: "1px solid var(--border)" }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 12 }}>
              <Space size={8}>
                <FileSearchOutlined style={{ color: "var(--accent)" }} />
                <span style={{ fontWeight: 600 }}>Data Preview</span>
              </Space>
              <Radio.Group value={previewSource} onChange={e => setPreviewSource(e.target.value)} size="small">
                <Radio.Button value="LOCAL"><FileOutlined /> Local</Radio.Button>
                <Radio.Button value="STAGE"><CloudOutlined /> Stage</Radio.Button>
              </Radio.Group>
            </div>

            <div style={{ display: "flex", gap: 8 }}>
              {previewSource === "LOCAL" ? (
                <Input
                  size="small"
                  value={localPath}
                  placeholder="Select a local CSV/JSON file..."
                  readOnly
                  onClick={handlePickFile}
                  suffix={<Button type="text" size="small" icon={<PlusOutlined />} onClick={handlePickFile} />}
                />
              ) : (
                <Input
                  size="small"
                  value={stagePath}
                  placeholder="@DB.SCHEMA.STAGE/path/to/file.csv"
                  onChange={e => setStagePath(e.target.value)}
                />
              )}
              <Button
                type="primary"
                size="small"
                loading={previewLoading}
                onClick={handlePreview}
              >
                Preview
              </Button>
            </div>

            {renderPreviewTable()}
          </div>

          {/* SQL Preview Section */}
          <div
            style={{
              padding: "12px 16px",
              background: "var(--bg)",
              borderRadius: 8,
              border: "1px solid var(--border)",
              flexGrow: 1,
              display: "flex",
              flexDirection: "column",
            }}
          >
            <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8, fontWeight: 600, textTransform: "uppercase", letterSpacing: "0.05em" }}>
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
                lineHeight: 1.5,
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
