// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Typography, Switch, Space } from "antd";
import { SecurityScanOutlined } from "@ant-design/icons";
import { BuildCreatePrivacyPolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
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

const BUDGET_BODY = "PRIVACY_BUDGET(BUDGET_NAME => 'privacy_budget')";
const NO_POLICY_BODY = "NO_PRIVACY_POLICY()";

// Plain data shape for form state. PrivacyPolicyConfig has no nested arrays, but
// we keep a local type and cast to the generated config only at the IPC boundary
// (`cfg as any`) so a literal needn't carry the Wails `convertValues` method.
type PrivacyPolicyCfg = {
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

export default function CreatePrivacyPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<PrivacyPolicyCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    body: BUDGET_BODY,
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreatePrivacyPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof PrivacyPolicyCfg>(key: K, value: PrivacyPolicyCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // The body is almost always one of the two standard forms; offer a quick
  // toggle that rewrites the body between an enforced privacy budget and an
  // unrestricted NO_PRIVACY_POLICY(), while still allowing free-form edits below.
  const enforceBudget = cfg.body.replace(/\s+/g, "").toUpperCase().includes("PRIVACY_BUDGET(");

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
      icon={<SecurityScanOutlined />}
      title="Create Privacy Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Privacy policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="ANALYTICS_PRIVACY_POLICY"
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

        <Form.Item label="Signature" style={itemStyle} help="Privacy policies have a fixed, argument-less signature returning PRIVACY_BUDGET.">
          <Input value="() RETURNS PRIVACY_BUDGET" disabled />
        </Form.Item>

        <Form.Item label="Enforce privacy budget" style={itemStyle} help="When on, queries that read from objects this policy is attached to consume a differential-privacy budget; otherwise NO_PRIVACY_POLICY() grants unrestricted access.">
          <Space>
            <Switch
              checked={enforceBudget}
              onChange={(v) => set("body", v ? BUDGET_BODY : NO_POLICY_BODY)}
            />
            <Text type="secondary" style={{ fontSize: 12 }}>{enforceBudget ? "PRIVACY_BUDGET(…)" : "NO_PRIVACY_POLICY()"}</Text>
          </Space>
        </Form.Item>

        <Form.Item label="Body" required style={itemStyle} help="A PRIVACY_BUDGET(...) or NO_PRIVACY_POLICY() expression. Budget parameters: BUDGET_NAME (required), BUDGET_LIMIT, MAX_BUDGET_PER_AGGREGATE, BUDGET_WINDOW.">
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
          <code>ALTER TABLE … ADD PRIVACY POLICY … ENTITY KEY (col, …)</code> once created.
          Privacy policies require Enterprise Edition.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
