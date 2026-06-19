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
import { Form, Input, Typography } from "antd";
import { GroupOutlined } from "@ant-design/icons";
import { BuildCreateAggregationPolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { aggregationpolicy as apModels } from "../../../wailsjs/go/models";
import Editor from "@monaco-editor/react";
import { useThemeStore } from "../../store/themeStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { setActiveSnippetEditor } from "../editor/SqlEditor";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_BODY = "AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)";

// Plain data shape for form state. The Wails-generated config class carries a
// `convertValues` method which a plain object literal can't satisfy; we cast to
// the generated type only at the IPC boundary (`cfg as any`).
type AggregationCfg = Omit<apModels.AggregationPolicyConfig, "convertValues">;

export default function CreateAggregationPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<AggregationCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    body: DEFAULT_BODY,
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateAggregationPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof AggregationCfg>(key: K, value: AggregationCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

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
      icon={<GroupOutlined />}
      title="Create Aggregation Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Aggregation policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="MIN_GROUP_5"
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
          label="Body"
          required
          style={itemStyle}
          help="An expression returning AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => n) or NO_AGGREGATION_CONSTRAINT(), optionally wrapped in conditional logic (e.g. on CURRENT_ROLE())."
        >
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={140}
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
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginTop: 4 }}>
            The signature is always <code>()</code> and the return type is always{" "}
            <code>AGGREGATION_CONSTRAINT</code> — only the body is authored.
          </Text>
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
