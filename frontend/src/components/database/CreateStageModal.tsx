// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Select, Checkbox, Radio, Space,
  Typography, Divider, Button, Alert,
} from "antd";
import { InboxOutlined, PlusOutlined } from "@ant-design/icons";
import { ExecDDL, GetQuotedIdentifiersIgnoreCase, ListIntegrations, BuildCreateStageSql, ListFileFormats } from "../../../wailsjs/go/main/App";

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
  const [cfg, setCfg] = useState<any>({ ...DEFAULTS, database: db, schema });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [integrations, setIntegrations] = useState<snowflake.IntegrationRow[]>([]);
  const [fileFormats, setFileFormats] = useState<string[]>([]);
  const [preview, setPreview] = useState("");
  const [formatSource, setFormatSource] = useState<"named" | "inline" | "none">("none");

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase().then((v) => setQuotedIdentifiersIgnoreCase(v ?? false)).catch(() => {});
    ListIntegrations("STORAGE").then(setIntegrations).catch(() => {});
    ListFileFormats(db, schema).then(setFileFormats).catch(() => {});
  }, [db, schema]);

  useEffect(() => {
    BuildCreateStageSql(cfg as stage.StageConfig).then(setPreview).catch(() => {});
  }, [cfg]);

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
      width={600}
      styles={{ body: { paddingTop: 16, maxHeight: "80vh", overflowY: "auto" } }}
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
            }}
            size="small"
          >
            <Radio value="none">None</Radio>
            <Radio value="named">Named Format</Radio>
            <Radio value="inline">Inline Format</Radio>
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
      </Form>
    </Modal>
  );
}

// @thaw-domain: Object Browser & Administration
