// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Checkbox, Alert, Typography } from "antd";
import { GlobalOutlined } from "@ant-design/icons";
import { BuildCreateExternalAgentSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateExternalAgentModal({ db, schema, onClose, onSuccess }: Props) {
  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [versionName, setVersionName] = useState("");
  const [comment, setComment] = useState("");

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () =>
      BuildCreateExternalAgentSql(db, schema, {
        name,
        caseSensitive,
        orReplace,
        ifNotExists,
        versionName,
        comment,
      } as any),
    [db, schema, name, caseSensitive, orReplace, ifNotExists, versionName, comment],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const canSubmit = name.trim().length > 0;

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
      icon={<GlobalOutlined />}
      title="Create External Agent"
      subtitle={`${db}.${schema}`}
      width={640}
      error={error}
      errorTitle="External agent creation failed"
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
          message="An external agent registers a third-party / generative-AI application for use with AI Observability. It is version-based — each version is a different implementation (retriever, prompt, LLM, or inference configuration)."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="External agent name" required style={{ marginBottom: 4 }}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_EXTERNAL_AGENT" />
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

        <Form.Item
          label="Initial version name (WITH VERSION)"
          style={itemStyle}
          help="Optional. Name of the first version of the external agent."
        >
          <Input value={versionName} onChange={(e) => setVersionName(e.target.value)} placeholder="V1" />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="optional comment" />
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          External agents share their namespace with model objects.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
