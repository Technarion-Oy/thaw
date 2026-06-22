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

import { useState, useRef } from "react";
import { Form, Input, Checkbox, Alert, Typography } from "antd";
import { NodeIndexOutlined } from "@ant-design/icons";
import { BuildCreateGatewaySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import EndpointTargetPicker from "./EndpointTargetPicker";
import { insertSpecTarget } from "./insertSpecTarget";

const { Text } = Typography;

// A skeleton traffic-split specification: a single endpoint target taking 100%
// of the traffic, mirroring the Snowflake reference so users fill in the
// service / endpoint and add additional weighted targets rather than recall the
// YAML shape from scratch. Weights across all targets must sum to 100.
const DEFAULT_SPEC = `spec:
  type: traffic_split
  split_type: custom
  targets:
  - type: endpoint
    value: <database>.<schema>.<service>!<endpoint>
    weight: 100
`;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateGatewayModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [specification, setSpecification] = useState(DEFAULT_SPEC);
  const editorRef = useRef<any>(null);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () =>
      BuildCreateGatewaySql(db, schema, {
        name,
        caseSensitive,
        orReplace,
        ifNotExists,
        specification,
      } as any),
    [db, schema, name, caseSensitive, orReplace, ifNotExists, specification],
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
      icon={<NodeIndexOutlined />}
      title="Create Gateway"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Gateway creation failed"
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
          message="A gateway fronts Snowpark Container Services endpoints, splitting ingress HTTP traffic across up to five service endpoints by weight (weights must sum to 100). The routing is defined by the YAML specification below."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Gateway name" required style={{ marginBottom: 4 }}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_GATEWAY" />
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

        <Form.Item label="Specification" required style={itemStyle} help="YAML traffic-split specification. Sent via FROM SPECIFICATION $$ … $$.">
          <EndpointTargetPicker
            defaultDb={db}
            defaultSchema={schema}
            onInsert={(block) => insertSpecTarget(editorRef.current, block, (b) => setSpecification((s) => s.replace(/\s*$/, "") + "\n" + b))}
          />
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={300}
              language="yaml"
              theme={editorTheme}
              value={specification}
              onChange={(v) => setSpecification(v ?? "")}
              onMount={(editor) => { patchMonacoClipboard(editor); editorRef.current = editor; }}
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
          Each <code>value</code> is a fully-qualified endpoint <code>db.schema.service!endpoint</code> that must already exist. The traffic split can be changed later via the gateway’s Properties panel (ALTER GATEWAY … FROM SPECIFICATION).
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
