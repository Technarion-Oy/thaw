// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Checkbox, Alert, Typography } from "antd";
import { PartitionOutlined } from "@ant-design/icons";
import { BuildCreateMCPServerSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

// A skeleton of the MCP server specification: a `tools` array where each tool
// has name / type / title / description (plus an `identifier` for Cortex /
// SQL-execution tools, or a `config` block for GENERIC UDF / procedure tools),
// mirroring the Snowflake reference so users fill in each tool rather than
// recall its shape from scratch.
const DEFAULT_SPEC = `tools:
  - name: "product-search"
    type: "CORTEX_SEARCH_SERVICE_QUERY"
    identifier: "<database>.<schema>.<cortex_search_service>"
    title: "Product Search"
    description: "<tool_description>"

  - name: "revenue-analyst"
    type: "CORTEX_ANALYST_MESSAGE"
    identifier: "<database>.<schema>.<semantic_view>"
    title: "Revenue Analyst"
    description: "<tool_description>"
`;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateMCPServerModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [specification, setSpecification] = useState(DEFAULT_SPEC);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () =>
      BuildCreateMCPServerSql(db, schema, {
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
      icon={<PartitionOutlined />}
      title="Create MCP Server"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="MCP server creation failed"
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
          message="An MCP server (Model Context Protocol) exposes Snowflake tools — Cortex Search, Cortex Analyst, SQL execution, Cortex agents, and generic UDFs / procedures — to MCP clients. The tools are defined by the YAML specification below."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="MCP server name" required style={{ marginBottom: 4 }}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_MCP_SERVER" />
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

        <Form.Item label="Specification" required style={itemStyle} help="YAML tools specification. Sent via FROM SPECIFICATION $$ … $$.">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={300}
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
          MCP servers have no ALTER statement — edit one with “OR REPLACE”. They require Cortex AI to be enabled in your account.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
