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
import { Form, Input, Checkbox, Alert, Typography } from "antd";
import { ApiOutlined } from "@ant-design/icons";
import { BuildCreateAgentSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { buildProfileJson } from "./profile";

const { Text } = Typography;

// A skeleton of the full agent specification (models / orchestration /
// instructions / tools / tool_resources) with placeholder values, mirroring the
// Snowflake reference so users can fill in each section rather than recall its
// shape from scratch.
const DEFAULT_SPEC = `models:
  orchestration: <model_name>

orchestration:
  budget:
      seconds: <number_of_seconds>
      tokens: <number_of_tokens>

instructions:
  response: '<response_instructions>'
  orchestration: '<orchestration_instructions>'
  sample_questions:
      - question: '<sample_question>'
      ...

tools:
  - tool_spec:
      type: '<tool_type>'
      name: '<tool_name>'
      description: '<tool_description>'
      input_schema:
          type: 'object'
          properties:
            <property_name>:
              type: '<property_type>'
              description: '<property_description>'
          required: <required_property_names>
  ...

tool_resources:
  <tool_name>:
    <resource_key>: '<resource_value>'
    ...
  ...
`;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateAgentModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [comment, setComment] = useState("");
  const [specification, setSpecification] = useState(DEFAULT_SPEC);
  // Profile (optional display metadata) → assembled into a JSON object literal.
  const [displayName, setDisplayName] = useState("");
  const [avatar, setAvatar] = useState("");
  const [color, setColor] = useState("");

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const profile = buildProfileJson({ display_name: displayName, avatar, color });
  const preview = useSqlPreview(
    () =>
      BuildCreateAgentSql(db, schema, {
        name,
        caseSensitive,
        orReplace,
        ifNotExists,
        comment,
        profile,
        specification,
      } as any),
    [db, schema, name, caseSensitive, orReplace, ifNotExists, comment, profile, specification],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const canSubmit = name.trim().length > 0 && specification.trim().length > 0;

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
      icon={<ApiOutlined />}
      title="Create Agent"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Agent creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="A Cortex AI agent pairs an orchestration model with a set of tools. Its behaviour is defined by the YAML/JSON specification below (models, orchestration, instructions, tools, tool_resources)."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Agent name" required style={{ marginBottom: 4 }}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_AGENT" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={orReplace}
              onChange={(e) => { setOrReplace(e.target.checked); if (e.target.checked) setIfNotExists(false); }}
            >
              OR REPLACE
            </Checkbox>
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={ifNotExists}
              onChange={(e) => { setIfNotExists(e.target.checked); if (e.target.checked) setOrReplace(false); }}
            >
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={name}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="optional comment" />
        </Form.Item>

        <Form.Item label="Profile (optional display metadata)" style={itemStyle} help="Assembled into the PROFILE JSON object. Leave blank to omit.">
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: 8 }}>
            <Input value={displayName} onChange={(e) => setDisplayName(e.target.value)} placeholder="display_name" />
            <Input value={avatar} onChange={(e) => setAvatar(e.target.value)} placeholder="avatar" />
            <Input value={color} onChange={(e) => setColor(e.target.value)} placeholder="color (e.g. #2563EB)" />
          </div>
        </Form.Item>

        <Form.Item label="Specification" required style={itemStyle} help="YAML or JSON (max 100,000 bytes). Sent via FROM SPECIFICATION $$ … $$.">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={260}
              language="yaml"
              theme={editorTheme}
              value={specification}
              onChange={(v) => setSpecification(v ?? "")}
              onMount={(editor) => { patchMonacoClipboard(editor); }}
              options={{
                minimap: { enabled: false },
                scrollBeyondLastLine: false,
                fontSize: 12,
                wordWrap: "on",
                automaticLayout: true,
              }}
            />
          </div>
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          Agents require Cortex AI to be enabled in your account.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
