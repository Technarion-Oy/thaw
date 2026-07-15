// SPDX-License-Identifier: GPL-3.0-or-later
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
import TagPropagationFields from "./TagPropagationFields";
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
              <TagPropagationFields
                propagate={cfg.propagate}
                onConflict={cfg.onConflict}
                onChange={({ propagate, onConflict }) =>
                  setCfg((prev) => ({ ...prev, propagate, onConflict }))}
                itemStyle={itemStyle}
              />
            ),
          }]}
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
