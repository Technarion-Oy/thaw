// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Checkbox, Space, Collapse } from "antd";
import { EyeOutlined } from "@ant-design/icons";
import {
  BuildCreateViewSql, ExecDDL,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import TagInput from "../shared/TagInput";
import MonacoSqlField from "../shared/MonacoSqlField";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { view } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

const DEFAULT_QUERY = "SELECT *\n  FROM my_source_table";

// Plain data shape for form state. The Wails-generated `ViewConfig` class carries
// a `convertValues` method (it has a nested `tags` array), which a plain object
// literal can't satisfy; we cast to the generated type only at the IPC boundary
// (`cfg as any`).
type VConfig = Omit<view.ViewConfig, "convertValues" | "tags"> & {
  tags: { name: string; value: string }[];
};

export default function CreateViewModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<VConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    secure: false,
    recursive: false,
    ifNotExists: false,
    copyGrants: false,
    comment: "",
    columns: "",
    tags: [],
    query: DEFAULT_QUERY,
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateViewSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof VConfig>(key: K, value: VConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // The query editor seeds DEFAULT_QUERY as a template; treat the untouched
  // placeholder as "not ready" so Create can't fire a statement that references
  // the obviously-fake `my_source_table` and fails server-side.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.query.trim().length > 0 &&
    cfg.query.trim() !== DEFAULT_QUERY.trim();

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const advancedBody = (
    <>
      <Form.Item label="Columns" style={itemStyle} help="Optional comma-separated column list, inserted verbatim — double-quote any name that needs it (reserved word, mixed case, special chars)">
        <Input
          value={cfg.columns}
          onChange={(e) => set("columns", e.target.value)}
          placeholder="col1, col2"
        />
      </Form.Item>

      <Form.Item style={{ marginBottom: 8 }}>
        <Space size={16} wrap>
          <Checkbox checked={cfg.secure} onChange={(e) => set("secure", e.target.checked)}>
            SECURE
          </Checkbox>
          <Checkbox checked={cfg.recursive} onChange={(e) => set("recursive", e.target.checked)}>
            RECURSIVE
          </Checkbox>
          <Checkbox checked={cfg.copyGrants} onChange={(e) => set("copyGrants", e.target.checked)}>
            COPY GRANTS
          </Checkbox>
        </Space>
      </Form.Item>

      <TagInput
        tags={cfg.tags}
        onChange={(tags) => set("tags", tags)}
        help="View-level tags applied at creation"
        itemStyle={itemStyle}
      />
    </>
  );

  return (
    <CreateModalShell
      icon={<EyeOutlined />}
      title="Create View"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="View creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="View name"
          placeholder="MY_VIEW"
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
          items={[{ key: "advanced", label: "Advanced options", children: advancedBody }]}
        />

        <MonacoSqlField
          label="Defining Query (AS)"
          required
          value={cfg.query}
          onChange={(v) => set("query", v)}
          placeholder={DEFAULT_QUERY}
          objectKinds={["TABLE", "VIEW"]}
          defaultDb={db}
          defaultSchema={schema}
          notFoundText="No tables or views"
        />

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
