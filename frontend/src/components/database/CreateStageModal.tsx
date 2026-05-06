// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { useState, useEffect, useRef, useCallback } from "react";
import {
  Modal, Form, Input, Select, Checkbox, Radio, Space,
  Typography, Divider, Button, Alert, Table, Tooltip,
} from "antd";
import {
  InboxOutlined, PlusOutlined, FileSearchOutlined,
  CloudOutlined, FileOutlined, InfoCircleOutlined,
} from "@ant-design/icons";
import {
  ExecDDL, GetQuotedIdentifiersIgnoreCase, ListIntegrations, BuildCreateStageSql, ListFileFormats,
  PickFileForFormatPreview, GetLocalFilePreview, GetStageFilePreview, SuggestImportOptions, ReadFileHead,
} from "../../../wailsjs/go/main/App";

import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import FileFormatFields, { BASE_DEFAULTS } from "./FileFormatFields";
import type { snowflake, stage, fileformat } from "../../../wailsjs/go/models";

const { Text } = Typography;

const DEFAULTS: any = {
  name: "",
  database: "",
  schema: "",
  caseSensitive: false,
  orReplace: false,
  ifNotExists: false,
  type: "INTERNAL",
  url: "",
  storageIntegration: "",
  usePrivatelinkEndpoint: false,
  encryptionType: "SNOWFLAKE_FULL",
  kmsKeyId: "",
  directoryEnabled: false,
  directoryAutoRefresh: false,
  directoryRefreshOnCreate: false,
  directoryNotificationIntegration: "",
  fileFormatName: "",
  fileFormat: BASE_DEFAULTS,
  comment: "",
  tags: "",
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateStageModal({ db, schema, onClose, onSuccess }: Props) {
  const featureFlags = useFeatureFlagsStore((s) => s.flags);
  const [cfg, setCfg] = useState<any>({ ...DEFAULTS, database: db, schema });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [integrations, setIntegrations] = useState<snowflake.IntegrationRow[]>([]);
  const [fileFormats, setFileFormats] = useState<string[]>([]);
  const [preview, setPreview] = useState("");
  const [formatSource, setFormatSource] = useState<"named" | "inline" | "none">("none");

  // Preview state
  const [previewSource, setPreviewSource] = useState<"LOCAL" | "STAGE">("LOCAL");
  const [localPath, setLocalPath] = useState("");
  const [stagePath, setStagePath] = useState("");
  const [previewData, setPreviewData] = useState<fileformat.PreviewResult | null>(null);
  const [previewLoading, setPreviewLoading] = useState(false);
  const hasPreviewRef = useRef(false);

  // AI Suggest state
  const [aiSuggesting, setAiSuggesting] = useState(false);
  const [aiExplanation, setAiExplanation] = useState<string | null>(null);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase().then((v) => setQuotedIdentifiersIgnoreCase(v ?? false)).catch(() => {});
    ListIntegrations("STORAGE").then(setIntegrations).catch(() => {});
    ListFileFormats(db, schema).then(setFileFormats).catch(() => {});
  }, [db, schema]);

  useEffect(() => {
    BuildCreateStageSql(cfg as stage.StageConfig).then(setPreview).catch(() => {});
  }, [cfg]);

  const handlePickFile = async () => {
    const path = await PickFileForFormatPreview();
    if (path) setLocalPath(path);
  };

  const runPreview = useCallback(async (
    path: string,
    stagePth: string,
    source: "LOCAL" | "STAGE",
    currentCfg: fileformat.FileFormatConfig,
  ) => {
    setCreateError(null);
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
        setCreateError(res.error);
        setPreviewData(null);
      } else {
        setPreviewData(res);
      }
    } catch (err) {
      setCreateError(String(err));
      setPreviewData(null);
    } finally {
      setPreviewLoading(false);
    }
  }, []);

  const handlePreview = () => {
    hasPreviewRef.current = true;
    runPreview(localPath, stagePath, previewSource, cfg.fileFormat);
  };

  const runAiSuggest = async () => {
    if (!localPath) return;
    setAiSuggesting(true);
    setCreateError(null);
    setAiExplanation(null);
    try {
      const head = await ReadFileHead(localPath, 65536);
      if (!head) throw new Error("Could not read file head");

      const raw = await SuggestImportOptions(cfg.fileFormat.type, head);
      if (!raw) throw new Error("No suggestion returned");

      const obj = JSON.parse(raw);
      const apply = (key: keyof fileformat.FileFormatConfig, val: any) => {
        if (val !== undefined) setFormatField(key, val);
      };

      if (cfg.fileFormat.type === "CSV") {
        if (obj.fieldDelimiter !== undefined) apply("fieldDelimiter", obj.fieldDelimiter);
        if (obj.parseHeader !== undefined) apply("parseHeader", obj.parseHeader);
        if (obj.fieldOptionallyEnclosedBy !== undefined) apply("fieldOptionallyEnclosedBy", obj.fieldOptionallyEnclosedBy);
        if (obj.encoding !== undefined) apply("encoding", obj.encoding);
        if (obj.compression !== undefined) apply("compression", obj.compression);
        if (obj.recordDelimiter !== undefined) apply("recordDelimiter", obj.recordDelimiter);
      } else if (cfg.fileFormat.type === "JSON") {
        if (obj.multiLine !== undefined) apply("multiLine", obj.multiLine);
        if (obj.stripOuterArray !== undefined) apply("stripOuterArray", obj.stripOuterArray);
      }

      if (obj.explanation) setAiExplanation(obj.explanation);
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setAiSuggesting(false);
    }
  };

  useEffect(() => {
    if (!hasPreviewRef.current) return;
    const hasTarget = previewSource === "LOCAL" ? !!localPath : !!stagePath.trim();
    if (!hasTarget) return;
    const timer = setTimeout(() => {
      runPreview(localPath, stagePath, previewSource, cfg.fileFormat);
    }, 500);
    return () => clearTimeout(timer);
  }, [cfg.fileFormat, previewSource, localPath, stagePath, runPreview]);

  const renderPreviewTable = () => {
    if (!previewData) return null;
    if (!previewData.columns || previewData.columns.length === 0) {
      return (
        <div style={{ padding: "12px 0", textAlign: "center", color: "var(--text-muted)", fontSize: 12 }}>
          No data to preview
        </div>
      );
    }
    return (
      <div style={{
        marginTop: 10,
        border: "1px solid var(--border)",
        borderRadius: 6,
        overflow: "auto",
        maxHeight: 280,
      }}>
        <table style={{ borderCollapse: "separate", borderSpacing: 0, width: "100%", fontSize: 11, fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace" }}>
          <thead style={{ position: "sticky", top: 0, zIndex: 1, background: "var(--bg-secondary)" }}>
            <tr>
              {previewData.columns.map((c, i) => (
                <th key={i} style={{ 
                  padding: "6px 8px", 
                  borderBottom: "1px solid var(--border)", 
                  borderRight: i < previewData.columns!.length - 1 ? "1px solid var(--border)" : "none", 
                  textAlign: "left", 
                  whiteSpace: "nowrap",
                  fontWeight: 600,
                }}>
                  {c || <em style={{ color: "var(--text-muted)", fontWeight: 400 }}>(empty)</em>}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {(previewData.rows ?? []).map((row, ri) => (
              <tr key={ri}>
                {previewData.columns.map((col, ci) => (
                  <td key={ci} style={{ 
                    padding: "4px 8px", 
                    borderBottom: ri < (previewData.rows?.length ?? 0) - 1 ? "1px solid var(--border)" : "none",
                    borderRight: ci < previewData.columns!.length - 1 ? "1px solid var(--border)" : "none", 
                    whiteSpace: "pre", 
                    maxWidth: 200, 
                    overflow: "hidden", 
                    textOverflow: "ellipsis" 
                  }}>
                    <Tooltip title={row[col]} placement="topLeft">
                      {row[col] === "" ? <em style={{ color: "var(--text-muted)", fontSize: 10 }}>(empty)</em> : row[col]}
                    </Tooltip>
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    );
  };

  const set = <K extends keyof stage.StageConfig>(key: K, value: stage.StageConfig[K]) =>
    setCfg((prev: any) => ({ ...prev, [key]: value }));

  const setFormatField = <K extends keyof fileformat.FileFormatConfig>(key: K, value: fileformat.FileFormatConfig[K]) =>
    setCfg((prev: any) => ({ ...prev, fileFormat: { ...prev.fileFormat, [key]: value } }));

  const canSubmit = cfg.name.trim() !== "" && (cfg.type === "INTERNAL" || cfg.url.trim() !== "");

  const handleCreate = async () => {
    if (!canSubmit) return;
    setCreating(true);
    setCreateError(null);
    try {
      const sql = await BuildCreateStageSql(cfg as stage.StageConfig);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const divider = (label: string) => (
    <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "16px 0 8px" }}>
      {label}
    </Divider>
  );

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <InboxOutlined style={{ color: "var(--link)" }} />
          <span>Create stage</span>
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
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={formatSource === "inline" ? 1040 : 600}
      styles={{ body: { paddingTop: 16, maxHeight: "85vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Stage creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <div style={formatSource === "inline" ? { display: "grid", gridTemplateColumns: "380px minmax(0, 1fr)", gap: 24 } : {}}>
          {/* ── Left Column: Configuration ─────────────────────────────────── */}
          <div>
            <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
              <Form.Item label="Stage name" required style={{ marginBottom: 4 }}>
                <Input
                  value={cfg.name}
                  onChange={(e) => set("name", e.target.value)}
                  placeholder="MY_STAGE"
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

            <Form.Item label="Stage type" style={{ marginBottom: 12 }}>
              <Radio.Group
                value={cfg.type}
                onChange={(e) => set("type", e.target.value)}
                size="small"
              >
                <Radio value="INTERNAL">Internal</Radio>
                <Radio value="EXTERNAL">External</Radio>
              </Radio.Group>
            </Form.Item>

            {cfg.type === "EXTERNAL" && (
              <>
                {divider("External Location")}
                <Form.Item label="URL" required style={{ marginBottom: 12 }} help="e.g. s3://bucket/path/ or gcs://bucket/path/">
                  <Input 
                    value={cfg.url} 
                    onChange={(e) => set("url", e.target.value)} 
                    placeholder="s3://my-bucket/data/" 
                  />
                </Form.Item>
                <Form.Item label="Storage Integration" style={{ marginBottom: 12 }}>
                  <Select
                    value={cfg.storageIntegration}
                    onChange={(v) => set("storageIntegration", v)}
                    placeholder="Select an integration"
                    allowClear
                    options={(integrations || []).map(i => ({ value: i.name, label: i.name }))}
                  />
                </Form.Item>
              </>
            )}

            {divider("Encryption")}
            <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
              <Form.Item label="Type" style={{ marginBottom: 12 }}>
                <Select
                  value={cfg.encryptionType}
                  onChange={(v) => set("encryptionType", v)}
                  options={[
                    { value: "SNOWFLAKE_FULL", label: "Snowflake Full" },
                    { value: "SNOWFLAKE_SSE", label: "Snowflake SSE" },
                    { value: "AWS_SSE_S3", label: "AWS SSE-S3" },
                    { value: "AWS_SSE_KMS", label: "AWS SSE-KMS" },
                    { value: "GCS_SSE_KMS", label: "GCS SSE-KMS" },
                    { value: "AZURE_SSE_S3", label: "Azure SSE-S3" },
                    { value: "NONE", label: "None" },
                  ]}
                />
              </Form.Item>
              <Form.Item label="KMS Key ID" style={{ marginBottom: 12 }}>
                <Input 
                  value={cfg.kmsKeyId} 
                  onChange={(e) => set("kmsKeyId", e.target.value)} 
                  placeholder="Optional KMS key ID"
                  disabled={cfg.encryptionType === "NONE"}
                />
              </Form.Item>
            </div>

            {divider("Directory Settings")}
            <Space size={24} style={{ marginBottom: 12 }}>
              <Checkbox checked={cfg.directoryEnabled} onChange={e => set("directoryEnabled", e.target.checked)}>
                Enable directory
              </Checkbox>
              <Checkbox 
                checked={cfg.directoryAutoRefresh} 
                onChange={e => set("directoryAutoRefresh", e.target.checked)}
                disabled={!cfg.directoryEnabled || cfg.type === "INTERNAL"}
              >
                Auto refresh
              </Checkbox>
              <Checkbox 
                checked={cfg.directoryRefreshOnCreate} 
                onChange={e => set("directoryRefreshOnCreate", e.target.checked)}
                disabled={!cfg.directoryEnabled || cfg.type === "EXTERNAL"}
              >
                Refresh on create
              </Checkbox>
            </Space>

            {divider("File Format")}
            <Form.Item style={{ marginBottom: 12 }}>
              <Radio.Group
                value={formatSource}
                onChange={(e) => {
                  setFormatSource(e.target.value);
                  if (e.target.value === "none") {
                    set("fileFormatName", "");
                    set("fileFormat", BASE_DEFAULTS);
                  } else if (e.target.value === "named") {
                    set("fileFormat", BASE_DEFAULTS);
                  } else {
                    set("fileFormatName", "");
                  }
                  setPreviewData(null);
                  hasPreviewRef.current = false;
                }}
                size="small"
              >
                <Radio value="none">None</Radio>
                <Radio value="named">Named Format</Radio>
                {featureFlags.fileFormatBuilder && <Radio value="inline">Inline Format</Radio>}
              </Radio.Group>
            </Form.Item>

            {formatSource === "named" && (
              <Form.Item style={{ marginBottom: 12 }}>
                <Select
                  showSearch
                  placeholder="Select a format in this schema"
                  value={cfg.fileFormatName || undefined}
                  onChange={(v) => set("fileFormatName", v)}
                  options={(fileFormats || []).map((f) => ({ value: f, label: f }))}
                  allowClear
                />
              </Form.Item>
            )}

            {formatSource === "inline" && (
              <div style={{ paddingLeft: 12, borderLeft: "2px solid var(--border)", marginBottom: 12 }}>
                <FileFormatFields cfg={cfg.fileFormat} set={setFormatField} hideNameFields />
              </div>
            )}

            <Form.Item label="Comment" style={{ marginBottom: 12 }}>
              <Input value={cfg.comment} onChange={e => set("comment", e.target.value)} placeholder="Stage comment" />
            </Form.Item>

            {formatSource !== "inline" && (
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
            )}
          </div>

          {/* ── Right Column: Preview & SQL (Inline Mode Only) ──────────────── */}
          {formatSource === "inline" && (
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
                  {previewSource === "LOCAL" && featureFlags.aiImportSuggest && (
                    <Tooltip title="AI Suggest format options from file content">
                      <Button
                        size="small"
                        onClick={runAiSuggest}
                        disabled={!localPath || aiSuggesting}
                        loading={aiSuggesting}
                      >
                        {!aiSuggesting && "✨"}
                      </Button>
                    </Tooltip>
                  )}
                </div>

                {previewSource === "STAGE" && (
                  <div style={{ marginTop: 6, fontSize: 11, color: "var(--text-muted)" }}>
                    <InfoCircleOutlined style={{ marginRight: 4 }} />
                    Stage preview requires an active warehouse and consumes compute credits.
                  </div>
                )}

                {aiExplanation && (
                  <Alert
                    type="info"
                    message="AI Suggestion"
                    description={aiExplanation}
                    showIcon
                    style={{ marginTop: 10 }}
                    closable
                    onClose={() => setAiExplanation(null)}
                  />
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
                  {preview}
                </pre>
              </div>
            </div>
          )}
        </div>
      </Form>
    </Modal>
  );
}

// @thaw-domain: Object Browser & Administration
