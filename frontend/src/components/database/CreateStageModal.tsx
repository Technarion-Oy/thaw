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
import { ExecDDL, GetQuotedIdentifiersIgnoreCase, ListIntegrations } from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl, { identToken } from "../shared/ObjectNameCaseControl";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface StageConfig {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  stageType: "INTERNAL" | "EXTERNAL";
  url: string;
  storageIntegration: string;
  encryptionType: string;
  kmsKeyId: string;
  directoryEnabled: boolean;
  directoryAutoRefresh: boolean;
  comment: string;
}

const DEFAULTS: StageConfig = {
  name: "",
  caseSensitive: false,
  orReplace: false,
  ifNotExists: false,
  stageType: "INTERNAL",
  url: "",
  storageIntegration: "",
  encryptionType: "SNOWFLAKE_FULL",
  kmsKeyId: "",
  directoryEnabled: false,
  directoryAutoRefresh: false,
  comment: "",
};

function buildSql(db: string, schema: string, cfg: StageConfig): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const sq = (s: string) => "'" + s.replace(/'/g, "''") + "'";

  let createClause = "CREATE";
  if (cfg.orReplace) createClause += " OR REPLACE";
  createClause += " STAGE";
  if (cfg.ifNotExists && !cfg.orReplace) createClause += " IF NOT EXISTS";

  const nameToken = identToken(cfg.name || "stage_name", cfg.caseSensitive);
  const lines: string[] = [
    `${createClause} "${esc(db)}"."${esc(schema)}".${nameToken}`,
  ];

  if (cfg.stageType === "EXTERNAL") {
    if (cfg.url.trim()) {
      lines.push(`  URL = ${sq(cfg.url.trim())}`);
    }
    if (cfg.storageIntegration.trim()) {
      lines.push(`  STORAGE_INTEGRATION = ${cfg.storageIntegration.trim()}`);
    }
  }

  // Encryption
  if (cfg.encryptionType !== "NONE") {
    let enc = `  ENCRYPTION = (TYPE = '${cfg.encryptionType}'`;
    if (cfg.kmsKeyId.trim()) {
      enc += ` KMS_KEY_ID = ${sq(cfg.kmsKeyId.trim())}`;
    }
    enc += ")";
    lines.push(enc);
  }

  // Directory
  if (cfg.directoryEnabled) {
    let dir = "  DIRECTORY = (ENABLE = TRUE";
    if (cfg.directoryAutoRefresh && cfg.stageType === "EXTERNAL") {
      dir += " AUTO_REFRESH = TRUE";
    }
    dir += ")";
    lines.push(dir);
  }

  if (cfg.comment.trim()) {
    lines.push(`  COMMENT = ${sq(cfg.comment.trim())}`);
  }

  return lines.join("\n") + ";";
}

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateStageModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<StageConfig>(DEFAULTS);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [integrations, setIntegrations] = useState<snowflake.IntegrationRow[]>([]);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase().then((v) => setQuotedIdentifiersIgnoreCase(v ?? false)).catch(() => {});
    ListIntegrations("STORAGE").then(setIntegrations).catch(() => {});
  }, []);

  const set = <K extends keyof StageConfig>(key: K, value: StageConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim() !== "" && (cfg.stageType === "INTERNAL" || cfg.url.trim() !== "");

  const handleCreate = async () => {
    if (!canSubmit) return;
    const sql = buildSql(db, schema, cfg);
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const preview = buildSql(db, schema, cfg);

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
            value={cfg.stageType}
            onChange={(e) => set("stageType", e.target.value)}
            size="small"
          >
            <Radio value="INTERNAL">Internal</Radio>
            <Radio value="EXTERNAL">External</Radio>
          </Radio.Group>
        </Form.Item>

        {cfg.stageType === "EXTERNAL" && (
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
                options={integrations.map(i => ({ value: i.name, label: i.name }))}
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
            disabled={!cfg.directoryEnabled || cfg.stageType === "INTERNAL"}
          >
            Auto refresh
          </Checkbox>
        </Space>

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
