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
import { Form, Input, Select, Collapse } from "antd";
import { TagsOutlined } from "@ant-design/icons";
import { BuildCreateTagSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { tag as tagModels } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain data shape for form state. The Wails-generated `TagConfig` class carries
// a `convertValues` method, which a plain object literal can't satisfy; we cast
// to the generated type only at the IPC boundary (`cfg as any`).
type TagCfg = Omit<tagModels.TagConfig, "convertValues">;

const ALLOWED_VALUES_SEQUENCE = "ALLOWED_VALUES_SEQUENCE";

const PROPAGATE_OPTIONS = [
  { value: "", label: "Disabled (no propagation)" },
  { value: "ON_DEPENDENCY_AND_DATA_MOVEMENT", label: "On dependency and data movement" },
  { value: "ON_DEPENDENCY", label: "On dependency only" },
  { value: "ON_DATA_MOVEMENT", label: "On data movement only" },
];

// How ON_CONFLICT is expressed: omit it, use the ALLOWED_VALUES_SEQUENCE
// keyword, or pin a fixed string value.
type ConflictMode = "none" | "sequence" | "value";

export default function CreateTagModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<TagCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    allowedValues: [],
    propagate: "",
    onConflict: "",
    comment: "",
  });

  // ON_CONFLICT mode drives the UI; the resulting cfg.onConflict is "" (none),
  // the ALLOWED_VALUES_SEQUENCE keyword, or the free-form fixed value.
  const [conflictMode, setConflictMode] = useState<ConflictMode>("none");
  const [conflictValue, setConflictValue] = useState("");

  const applyConflict = (mode: ConflictMode, value: string) => {
    setConflictMode(mode);
    setConflictValue(value);
    const oc = mode === "sequence" ? ALLOWED_VALUES_SEQUENCE : mode === "value" ? value : "";
    set("onConflict", oc);
  };

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateTagSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof TagCfg>(key: K, value: TagCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim().length > 0;

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
      icon={<TagsOutlined />}
      title="Create Tag"
      subtitle={`${db}.${schema}`}
      width={680}
      error={error}
      errorTitle="Tag creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Tag name"
          placeholder="COST_CENTER"
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
          label="Allowed values"
          style={itemStyle}
          help="Optional whitelist of values the tag may take. Leave empty to allow any string."
        >
          <Select
            mode="tags"
            value={cfg.allowedValues}
            onChange={(vals) => set("allowedValues", vals)}
            placeholder="Type a value and press Enter (optional)"
            tokenSeparators={[",", "\n"]}
            open={false}
            suffixIcon={null}
          />
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
            key: "propagation",
            label: "Propagation (tag lineage)",
            children: (
              <>
                <Form.Item
                  label="Propagate"
                  style={itemStyle}
                  help="Automatically propagate this tag from source objects to target objects."
                >
                  <Select
                    value={cfg.propagate}
                    onChange={(v) => {
                      set("propagate", v);
                      // ON_CONFLICT only applies alongside PROPAGATE — reset it
                      // when propagation is disabled.
                      if (!v) applyConflict("none", "");
                    }}
                    options={PROPAGATE_OPTIONS}
                  />
                </Form.Item>

                {cfg.propagate && (
                  <Form.Item
                    label="On conflict"
                    style={itemStyle}
                    help="How to resolve conflicts between propagated tag values."
                  >
                    <Select
                      value={conflictMode}
                      onChange={(m) => applyConflict(m, conflictValue)}
                      options={[
                        { value: "none", label: "Default (no override)" },
                        { value: "sequence", label: "By allowed-values order (ALLOWED_VALUES_SEQUENCE)" },
                        { value: "value", label: "Fixed value…" },
                      ]}
                    />
                    {conflictMode === "value" && (
                      <Input
                        style={{ marginTop: 8 }}
                        value={conflictValue}
                        onChange={(e) => applyConflict("value", e.target.value)}
                        placeholder="conflict value"
                      />
                    )}
                  </Form.Item>
                )}
              </>
            ),
          }]}
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
