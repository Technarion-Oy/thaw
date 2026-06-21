// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Typography, Switch, Space } from "antd";
import { DisconnectOutlined } from "@ant-design/icons";
import { BuildCreateJoinPolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import Editor from "@monaco-editor/react";
import { setActiveSnippetEditor } from "../editor/SqlEditor";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

const REQUIRED_BODY = "JOIN_CONSTRAINT(JOIN_REQUIRED => TRUE)";
const NOT_REQUIRED_BODY = "JOIN_CONSTRAINT(JOIN_REQUIRED => FALSE)";

// Plain data shape for form state. JoinPolicyConfig has no nested arrays, but we
// keep a local type and cast to the generated config only at the IPC boundary
// (`cfg as any`) so a literal needn't carry the Wails `convertValues` method.
type JoinPolicyCfg = {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  body: string;
  comment: string;
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateJoinPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<JoinPolicyCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    body: REQUIRED_BODY,
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateJoinPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof JoinPolicyCfg>(key: K, value: JoinPolicyCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // The body is almost always one of the two JOIN_CONSTRAINT forms; offer a quick
  // toggle that rewrites the body, while still allowing free-form edits below.
  const joinRequired = cfg.body.replace(/\s+/g, "").toUpperCase().includes("JOIN_REQUIRED=>TRUE");

  const canSubmit = cfg.name.trim().length > 0 && cfg.body.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<DisconnectOutlined />}
      title="Create Join Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Join policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="REQUIRE_JOIN_POLICY"
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

        <Form.Item label="Signature" style={itemStyle} help="Join policies have a fixed, argument-less signature returning JOIN_CONSTRAINT.">
          <Input value="() RETURNS JOIN_CONSTRAINT" disabled />
        </Form.Item>

        <Form.Item label="Join required" style={itemStyle} help="When on, queries that read from objects this policy is attached to must join on an allowed key; otherwise the join is rejected.">
          <Space>
            <Switch
              checked={joinRequired}
              onChange={(v) => set("body", v ? REQUIRED_BODY : NOT_REQUIRED_BODY)}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>{joinRequired ? "JOIN_REQUIRED => TRUE" : "JOIN_REQUIRED => FALSE"}</Text>
          </Space>
        </Form.Item>

        <Form.Item label="Body" required style={itemStyle} help="A JOIN_CONSTRAINT(...) expression. The body cannot reference user-defined functions, tables, or views.">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={120}
              language="sql"
              theme={editorTheme}
              value={cfg.body}
              onChange={(v) => set("body", v ?? "")}
              onMount={(editor) => {
                patchMonacoClipboard(editor);
                // The "SQL Snippets" context-menu submenu is registered globally
                // for any SQL editor, but its commands insert into the shared
                // _activeSnippetEditor. Register this editor on right-click (and
                // clear on dispose) so picking a snippet lands here, not in the
                // main SQL editor behind the modal.
                editor.onContextMenu(() => setActiveSnippetEditor(editor));
                editor.onDidDispose(() => setActiveSnippetEditor(null));
              }}
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

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          Apply the policy to a table or view with{" "}
          <code>ALTER TABLE … ADD JOIN POLICY … ON (col, …)</code> once created.
          Join policies require Enterprise Edition.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
