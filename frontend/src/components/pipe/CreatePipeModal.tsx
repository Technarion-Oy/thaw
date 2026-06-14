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
import { Form, Input, Switch, Select } from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { BuildCreatePipeSql, ExecDDL, ListNotificationIntegrations } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { pipe } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

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
  const [notifIntegrations, setNotifIntegrations] = useState<string[]>([]);
  const [loadingIntegrations, setLoadingIntegrations] = useState(false);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(() => BuildCreatePipeSql(db, schema, cfg as any), [db, schema, cfg]);
  const { creating, error, setError, submit } = useCreateSubmit();

  useEffect(() => {
    setLoadingIntegrations(true);
    ListNotificationIntegrations()
      .then((names) => setNotifIntegrations(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingIntegrations(false));
  }, []);

  const set = <K extends keyof pipe.PipeConfig>(key: K, value: pipe.PipeConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const integrationOptions = (notifIntegrations || []).map((n) => ({ value: n, label: n }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<ApiOutlined />}
      title="Create Pipe"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Pipe creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Pipe name"
          placeholder="MY_PIPE"
          name={cfg.name}
          onNameChange={(v) => set("name", v)}
          orReplace={cfg.orReplace}
          ifNotExists={cfg.ifNotExists}
          onOrReplaceChange={(v) => set("orReplace", v)}
          onIfNotExistsChange={(v) => set("ifNotExists", v)}
        />

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

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
