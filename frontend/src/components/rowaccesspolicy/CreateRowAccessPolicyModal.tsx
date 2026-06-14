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
import {
  Form, Input, AutoComplete, Button, Typography, Space,
} from "antd";
import { SafetyOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateRowAccessPolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { rowaccesspolicy as rapModels } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";

const { Text } = Typography;

// Common Snowflake data types offered as type suggestions; AutoComplete still
// accepts any free-form text (e.g. NUMBER(38,0), VARCHAR(100)) so the list is a
// convenience, not a constraint.
const TYPE_OPTIONS = [
  "VARCHAR", "STRING", "NUMBER", "NUMBER(38,0)", "FLOAT", "BOOLEAN",
  "DATE", "TIME", "TIMESTAMP_NTZ", "TIMESTAMP_LTZ", "TIMESTAMP_TZ",
  "VARIANT", "OBJECT", "ARRAY",
].map((v) => ({ value: v }));

const DEFAULT_BODY =
  "CASE\n  WHEN CURRENT_ROLE() IN ('ADMIN') THEN TRUE\n  ELSE FALSE\nEND";

// Plain data shapes for form state. The Wails-generated config class carries a
// `convertValues` method (it has a nested `args` array) which a plain object
// literal can't satisfy; we cast to the generated type only at the IPC boundary
// (`cfg as any`).
type RowAccessArg = { name: string; type: string };
type RowAccessCfg = Omit<rapModels.RowAccessPolicyConfig, "convertValues" | "args"> & {
  args: RowAccessArg[];
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateRowAccessPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<RowAccessCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    args: [{ name: "val", type: "VARCHAR" }],
    body: DEFAULT_BODY,
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateRowAccessPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof RowAccessCfg>(key: K, value: RowAccessCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const setArgs = (args: RowAccessArg[]) =>
    setCfg((prev) => ({ ...prev, args }));

  const updateArg = (i: number, patch: Partial<RowAccessArg>) =>
    setArgs(cfg.args.map((a, idx) => (idx === i ? { ...a, ...patch } : a)));

  const addArg = () => setArgs([...cfg.args, { name: "", type: "VARCHAR" }]);

  const removeArg = (i: number) => setArgs(cfg.args.filter((_, idx) => idx !== i));

  const validArgs = cfg.args.filter((a) => a.name.trim() !== "" && a.type.trim() !== "");
  const canSubmit =
    cfg.name.trim().length > 0 &&
    validArgs.length > 0 &&
    cfg.body.trim().length > 0;

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
      icon={<SafetyOutlined />}
      title="Create Row Access Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Row access policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="SALES_REGION_RAP"
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

        <Form.Item
          label="Signature"
          required
          style={itemStyle}
          help="Each argument is a column the policy evaluates. When the policy is attached to a table or view, each argument is mapped to one of that object's columns."
        >
          <Space direction="vertical" size={6} style={{ width: "100%" }}>
            {cfg.args.map((a, i) => (
              <Space key={i} align="baseline" style={{ width: "100%" }}>
                <Input
                  placeholder="arg name"
                  value={a.name}
                  onChange={(e) => updateArg(i, { name: e.target.value })}
                  style={{ width: 200 }}
                />
                <AutoComplete
                  placeholder="VARCHAR"
                  value={a.type}
                  options={TYPE_OPTIONS}
                  onChange={(v) => updateArg(i, { type: v })}
                  filterOption={(input, option) =>
                    (option?.value ?? "").toUpperCase().includes(input.toUpperCase())
                  }
                  style={{ width: 200 }}
                />
                {cfg.args.length > 1 && (
                  <Button
                    type="text"
                    size="small"
                    icon={<DeleteOutlined />}
                    onClick={() => removeArg(i)}
                    danger
                  />
                )}
              </Space>
            ))}
            <Button size="small" icon={<PlusOutlined />} onClick={addArg}>
              Add column argument
            </Button>
          </Space>
        </Form.Item>

        <Form.Item label="Returns" style={itemStyle} help="Row access policies always return BOOLEAN — TRUE keeps the row visible.">
          <Input value="BOOLEAN" disabled />
        </Form.Item>

        <Form.Item label="Body" required style={itemStyle} help="A boolean expression deciding whether a row is visible (e.g. based on CURRENT_ROLE()).">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={140}
              language="sql"
              theme={editorTheme}
              value={cfg.body}
              onChange={(v) => set("body", v ?? "")}
              onMount={(editor) => { patchMonacoClipboard(editor); }}
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
          <code>ALTER TABLE … ADD ROW ACCESS POLICY … ON (col, …)</code> once created.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
