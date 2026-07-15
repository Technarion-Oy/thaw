// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Checkbox, Alert, Typography } from "antd";
import { ApartmentOutlined } from "@ant-design/icons";
import { BuildCreateSemanticViewSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import Editor from "@monaco-editor/react";
import { setActiveSnippetEditor } from "../editor/SqlEditor";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

// A skeleton of the semantic-view definition: logical TABLES, the
// RELATIONSHIPS between them, and the FACTS / DIMENSIONS / METRICS that describe
// the data. The clause order matters to Snowflake (FACTS before DIMENSIONS,
// etc.), so the template lays them out in the required order for the user to
// fill in rather than recall from scratch.
const DEFAULT_BODY = `TABLES (
    orders AS <database>.<schema>.orders
      PRIMARY KEY (order_id)
      WITH SYNONYMS = ('sales')
      COMMENT = 'Order facts',
    customers AS <database>.<schema>.customers
      PRIMARY KEY (customer_id)
  )
  RELATIONSHIPS (
    orders (customer_id) REFERENCES customers (customer_id)
  )
  FACTS (
    orders.amount AS orders.amount
  )
  DIMENSIONS (
    customers.region AS customers.region
      WITH SYNONYMS = ('area'),
    orders.order_date AS orders.order_date
  )
  METRICS (
    orders.total_revenue AS SUM(orders.amount)
      COMMENT = 'Total order revenue'
  )`;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateSemanticViewModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [body, setBody] = useState(DEFAULT_BODY);
  const [comment, setComment] = useState("");
  const [copyGrants, setCopyGrants] = useState(false);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () =>
      BuildCreateSemanticViewSql(db, schema, {
        name,
        caseSensitive,
        orReplace,
        ifNotExists,
        body,
        comment,
        copyGrants,
      } as any),
    [db, schema, name, caseSensitive, orReplace, ifNotExists, body, comment, copyGrants],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  // Block the untouched placeholder body (it contains literal <database>.<schema>
  // tokens that would fail server-side), mirroring CreateViewModal's DEFAULT_QUERY
  // guard.
  const canSubmit =
    name.trim().length > 0 &&
    body.trim().length > 0 &&
    body.trim() !== DEFAULT_BODY.trim();

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
      icon={<ApartmentOutlined />}
      title="Create Semantic View"
      subtitle={`${db}.${schema}`}
      width={760}
      error={error}
      errorTitle="Semantic view creation failed"
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
          message="A semantic view defines a semantic layer over physical tables for natural-language querying with Cortex Analyst. Define the logical TABLES, the RELATIONSHIPS between them, and the FACTS / DIMENSIONS / METRICS below — the clause order matters (FACTS before DIMENSIONS, etc.)."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Semantic view name" required style={{ marginBottom: 4 }}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_SEMANTIC_VIEW" />
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

        <Form.Item label="Definition" required style={itemStyle} help="TABLES → RELATIONSHIPS → FACTS → DIMENSIONS → METRICS (emitted verbatim, in this order).">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={320}
              language="sql"
              theme={editorTheme}
              value={body}
              onChange={(v) => setBody(v ?? "")}
              onMount={(editor) => {
                patchMonacoClipboard(editor);
                editor.onContextMenu(() => setActiveSnippetEditor(editor));
                editor.onDidDispose(() => setActiveSnippetEditor(null));
              }}
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

        <Form.Item label="Comment" style={itemStyle}>
          <Input.TextArea
            value={comment}
            onChange={(e) => setComment(e.target.value)}
            placeholder="Optional description of this semantic view"
            autoSize={{ minRows: 1, maxRows: 3 }}
          />
        </Form.Item>

        <Form.Item style={itemStyle}>
          <Checkbox checked={copyGrants} onChange={(e) => setCopyGrants(e.target.checked)}>
            COPY GRANTS
          </Checkbox>
          <Text type="secondary" style={{ fontSize: 11, marginLeft: 8 }}>
            Retain access grants from a replaced semantic view of the same name.
          </Text>
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          ALTER only changes the comment, tags, or name — change the definition with “OR REPLACE”. Semantic views require Cortex AI to be enabled in your account.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
