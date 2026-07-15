// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import {
  Form, Input, AutoComplete, Button, Checkbox, Collapse, Typography, Space,
} from "antd";
import { EyeInvisibleOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateMaskingPolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { maskingpolicy as mpModels } from "../../../wailsjs/go/models";
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

// Common Snowflake data types offered as type suggestions; AutoComplete still
// accepts any free-form text (e.g. NUMBER(38,0), VARCHAR(100)) so the list is a
// convenience, not a constraint.
const TYPE_OPTIONS = [
  "VARCHAR", "STRING", "NUMBER", "NUMBER(38,0)", "FLOAT", "BOOLEAN",
  "DATE", "TIME", "TIMESTAMP_NTZ", "TIMESTAMP_LTZ", "TIMESTAMP_TZ",
  "VARIANT", "OBJECT", "ARRAY",
].map((v) => ({ value: v }));

const DEFAULT_BODY =
  "CASE\n  WHEN CURRENT_ROLE() IN ('ADMIN') THEN val\n  ELSE '***MASKED***'\nEND";

// Plain data shapes for form state. The Wails-generated config class carries a
// `convertValues` method (it has a nested `args` array) which a plain object
// literal can't satisfy; we cast to the generated type only at the IPC boundary
// (`cfg as any`).
type MaskingArg = { name: string; type: string };
type MaskingCfg = Omit<mpModels.MaskingPolicyConfig, "convertValues" | "args"> & {
  args: MaskingArg[];
};

export default function CreateMaskingPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<MaskingCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    args: [{ name: "val", type: "VARCHAR" }],
    returnType: "VARCHAR",
    body: DEFAULT_BODY,
    comment: "",
    exemptOtherPolicies: false,
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateMaskingPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof MaskingCfg>(key: K, value: MaskingCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // The return type must match the first emitted argument's type, so keep it
  // pinned to the first *valid* row (name + type both set) as the signature
  // changes rather than asking the user to re-enter (and risk mismatching) it.
  // The Go builder skips blank rows, so pinning to the first valid row — not the
  // literal args[0] — keeps RETURNS aligned with the argument it actually emits.
  const firstValidType = (args: MaskingArg[]) =>
    args.find((a) => a.name.trim() !== "" && a.type.trim() !== "")?.type ?? "";

  const setArgs = (args: MaskingArg[]) =>
    setCfg((prev) => ({ ...prev, args, returnType: firstValidType(args) }));

  const updateArg = (i: number, patch: Partial<MaskingArg>) =>
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
      icon={<EyeInvisibleOutlined />}
      title="Create Masking Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Masking policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="MASK_EMAIL"
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
          help="The first argument is the column being masked; its type sets the return type. Add more arguments to reference other columns as conditions."
        >
          <Space direction="vertical" size={6} style={{ width: "100%" }}>
            {cfg.args.map((a, i) => (
              <Space key={i} align="baseline" style={{ width: "100%" }}>
                <Input
                  placeholder={i === 0 ? "val (masked column)" : "arg name"}
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
                {i === 0 && (
                  <Text type="secondary" style={{ fontSize: 11 }}>masked column</Text>
                )}
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
              Add conditional column
            </Button>
          </Space>
        </Form.Item>

        <Form.Item label="Returns" style={itemStyle} help="Must match the first column's type — pinned automatically.">
          <Input value={cfg.returnType} disabled />
        </Form.Item>

        <Form.Item label="Body" required style={itemStyle} help="The masking expression — returns the value unchanged or a masked substitute (e.g. based on CURRENT_ROLE()).">
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

        <Collapse
          ghost
          size="small"
          style={{ marginBottom: 8 }}
          items={[{
            key: "advanced",
            label: "Advanced options",
            children: (
              <Form.Item style={{ marginBottom: 0 }}>
                <Checkbox
                  checked={cfg.exemptOtherPolicies}
                  onChange={(e) => set("exemptOtherPolicies", e.target.checked)}
                >
                  Exempt from other policies
                </Checkbox>
                <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 4 }}>
                  Allow a column protected by this policy to be used as a conditional column in another masking policy.
                </Text>
              </Form.Item>
            ),
          }]}
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
