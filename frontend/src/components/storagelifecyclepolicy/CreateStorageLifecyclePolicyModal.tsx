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
  Form, Input, AutoComplete, Button, Typography, Space, Select, InputNumber,
} from "antd";
import { HddOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateStorageLifecyclePolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
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

// Common Snowflake data types offered as type suggestions; AutoComplete still
// accepts any free-form text so the list is a convenience, not a constraint. The
// lifecycle body usually keys off a timestamp/date column, so those lead.
const TYPE_OPTIONS = [
  "TIMESTAMP_NTZ", "TIMESTAMP_LTZ", "TIMESTAMP_TZ", "DATE", "TIME",
  "VARCHAR", "STRING", "NUMBER", "NUMBER(38,0)", "FLOAT", "BOOLEAN",
  "VARIANT", "OBJECT", "ARRAY",
].map((v) => ({ value: v }));

const DEFAULT_BODY = "created_at < DATEADD('day', -365, CURRENT_TIMESTAMP())";

// Plain data shapes for form state. The Wails-generated config class carries a
// `convertValues` method (it has a nested `args` array) which a plain object
// literal can't satisfy; we cast to the generated type only at the IPC boundary
// (`cfg as any`).
type StorageLifecycleArg = { name: string; type: string };
type StorageLifecycleCfg = {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  args: StorageLifecycleArg[];
  body: string;
  archiveTier: string; // "" | "COOL" | "COLD"
  archiveForDays: number | null;
  comment: string;
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateStorageLifecyclePolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const resolved = useThemeStore((s) => s.resolved);
  const editorTheme = resolved === "dark" ? "vs-dark" : "vs";

  const [cfg, setCfg] = useState<StorageLifecycleCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    args: [{ name: "created_at", type: "TIMESTAMP_NTZ" }],
    body: DEFAULT_BODY,
    archiveTier: "",
    archiveForDays: null,
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    // The generated config expects archiveForDays as a number; send 0 for "unset"
    // (the builder omits ARCHIVE_FOR_DAYS when <= 0).
    () => BuildCreateStorageLifecyclePolicySql(db, schema, {
      ...cfg,
      archiveForDays: cfg.archiveForDays ?? 0,
    } as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof StorageLifecycleCfg>(key: K, value: StorageLifecycleCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const setArgs = (args: StorageLifecycleArg[]) =>
    setCfg((prev) => ({ ...prev, args }));

  const updateArg = (i: number, patch: Partial<StorageLifecycleArg>) =>
    setArgs(cfg.args.map((a, idx) => (idx === i ? { ...a, ...patch } : a)));

  const addArg = () => setArgs([...cfg.args, { name: "", type: "TIMESTAMP_NTZ" }]);

  const removeArg = (i: number) => setArgs(cfg.args.filter((_, idx) => idx !== i));

  const validArgs = cfg.args.filter((a) => a.name.trim() !== "" && a.type.trim() !== "");
  // ARCHIVE_TIER and ARCHIVE_FOR_DAYS must be set together — a tier with no days
  // is rejected by Snowflake, so require the days whenever a tier is chosen.
  const archiveOk = cfg.archiveTier === "" || (cfg.archiveForDays !== null && cfg.archiveForDays > 0);
  const canSubmit =
    cfg.name.trim().length > 0 &&
    validArgs.length > 0 &&
    cfg.body.trim().length > 0 &&
    archiveOk;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  // ARCHIVE_FOR_DAYS only makes sense when a tier is set (rows are archived
  // before expiring); the documented minimums are 90 days for COOL and 180 for
  // COLD. Disable the field — and clear it — when no tier is chosen.
  const archiveEnabled = cfg.archiveTier !== "";
  const minDays = cfg.archiveTier === "COLD" ? 180 : 90;

  return (
    <CreateModalShell
      icon={<HddOutlined />}
      title="Create Storage Lifecycle Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Storage lifecycle policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="RETAIN_365_DAYS"
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
          help="Each argument is a column the policy evaluates. When the policy is attached to a table, each argument is mapped to one of that table's columns. At least one argument is required."
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
                  placeholder="TIMESTAMP_NTZ"
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

        <Form.Item label="Returns" style={itemStyle} help="Storage lifecycle policies always return BOOLEAN — TRUE marks the row as eligible for the lifecycle action (archival, then expiration).">
          <Input value="BOOLEAN" disabled />
        </Form.Item>

        <Form.Item label="Body" required style={itemStyle} help="A boolean expression deciding whether a row is eligible for the lifecycle action (e.g. based on an age threshold over a timestamp column).">
          <div style={{ border: "1px solid var(--border)", borderRadius: 6, overflow: "hidden" }}>
            <Editor
              height={120}
              language="sql"
              theme={editorTheme}
              value={cfg.body}
              onChange={(v) => set("body", v ?? "")}
              onMount={(editor) => {
                patchMonacoClipboard(editor);
                // Register this editor as the active snippet target so the global
                // "SQL Snippets" context-menu commands insert here, not into the
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

        <Form.Item label="Archive tier" style={itemStyle} help="When set, eligible rows are first moved to the chosen archive tier before expiring; when left as None, rows expire directly. Immutable once set.">
          <Select
            value={cfg.archiveTier === "" ? undefined : cfg.archiveTier}
            placeholder="None (expire without archiving)"
            allowClear
            style={{ width: 320 }}
            onChange={(v) => setCfg((prev) => ({
              ...prev,
              archiveTier: v ?? "",
              // Tier and days must be set together: default the days to the
              // per-tier minimum when enabling, and clear them when disabling.
              archiveForDays: v
                ? (prev.archiveForDays ?? (v === "COLD" ? 180 : 90))
                : null,
            }))}
            options={[
              { value: "COOL", label: "COOL (min 90 days)" },
              { value: "COLD", label: "COLD (min 180 days)" },
            ]}
          />
        </Form.Item>

        <Form.Item label="Archive for days" style={itemStyle} help={archiveEnabled ? `Days rows remain in the archive tier (minimum ${minDays} for ${cfg.archiveTier}).` : "Select an archive tier first."}>
          <InputNumber
            value={cfg.archiveForDays}
            min={minDays}
            placeholder={String(minDays)}
            disabled={!archiveEnabled}
            style={{ width: 200 }}
            onChange={(v) => set("archiveForDays", v === null || v === undefined ? null : Number(v))}
          />
        </Form.Item>

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          Apply the policy to a table with{" "}
          <code>ALTER TABLE … ADD STORAGE LIFECYCLE POLICY … ON (col, …)</code> once created.
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
