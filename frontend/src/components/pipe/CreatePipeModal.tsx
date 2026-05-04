// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to patents holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Switch, Checkbox, Select, Space,
  Typography, Button, Alert,
} from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { BuildCreatePipeSql, ExecDDL, GetQuotedIdentifiersIgnoreCase, ListNotificationIntegrations } from "../../../wailsjs/go/main/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import type { pipe } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_COPY = "COPY INTO my_table\n  FROM @my_stage";

export default function CreatePipeModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<pipe.PipeConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    autoIngest: false,
    errorIntegration: "",
    awsSnsTopic: "",
    integration: "",
    comment: "",
    copyStatement: DEFAULT_COPY,
  });
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");
  const [notifIntegrations, setNotifIntegrations] = useState<string[]>([]);
  const [loadingIntegrations, setLoadingIntegrations] = useState(false);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});

    setLoadingIntegrations(true);
    ListNotificationIntegrations()
      .then((names) => setNotifIntegrations(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingIntegrations(false));
  }, []);

  useEffect(() => {
    BuildCreatePipeSql(db, schema, cfg as any).then(setPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = <K extends keyof pipe.PipeConfig>(key: K, value: pipe.PipeConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim().length > 0;

  const handleRun = async () => {
    if (!canSubmit) return;
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const integrationOptions = (notifIntegrations || []).map((n) => ({ value: n, label: n }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ApiOutlined style={{ color: "var(--link)" }} />
          <span>Create Pipe</span>
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
            icon={<ApiOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={720}
      styles={{ body: { paddingTop: 16, maxHeight: "80vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Pipe creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Pipe name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_PIPE"
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

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Auto Ingest" style={itemStyle} help="Enable automatic ingestion when files arrive in a stage (requires S3 event notification or similar)">
          <Switch
            checked={cfg.autoIngest}
            onChange={(v) => set("autoIngest", v)}
            checkedChildren="TRUE"
            unCheckedChildren="FALSE"
          />
        </Form.Item>

        <Form.Item label="Error Integration" style={itemStyle} help="Notification integration used for error notifications">
          <Select
            allowClear
            showSearch
            loading={loadingIntegrations}
            value={cfg.errorIntegration || undefined}
            onChange={(v) => set("errorIntegration", v ?? "")}
            placeholder="Select notification integration…"
            options={integrationOptions}
            style={{ width: "100%" }}
            notFoundContent={loadingIntegrations ? "Loading…" : "No notification integrations found"}
          />
        </Form.Item>

        <Form.Item label="AWS SNS Topic" style={itemStyle} help="AWS SNS topic ARN for auto-ingest notifications">
          <Input
            value={cfg.awsSnsTopic}
            onChange={(e) => set("awsSnsTopic", e.target.value)}
            placeholder="arn:aws:sns:..."
          />
        </Form.Item>

        <Form.Item label="Integration" style={itemStyle} help="Notification integration for pipe event notifications">
          <Select
            allowClear
            showSearch
            loading={loadingIntegrations}
            value={cfg.integration || undefined}
            onChange={(v) => set("integration", v ?? "")}
            placeholder="Select notification integration…"
            options={integrationOptions}
            style={{ width: "100%" }}
            notFoundContent={loadingIntegrations ? "Loading…" : "No notification integrations found"}
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Form.Item label="COPY INTO Statement (AS)" required style={itemStyle}>
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={120}
              language="sql"
              theme={editorTheme}
              value={cfg.copyStatement}
              onChange={(v) => set("copyStatement", v ?? "")}
              onMount={(editor) => patchMonacoClipboard(editor)}
              options={{
                minimap: { enabled: false },
                lineNumbers: "off",
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
              }}
            />
          </div>
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
