// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, InputNumber, Select } from "antd";
import { NumberOutlined } from "@ant-design/icons";
import {
  BuildCreateSequenceSql, ExecDDL,
} from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { sequence } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain data shape for form state. The Wails-generated `SequenceConfig` class
// carries a `convertValues` method that a plain object literal can't satisfy; we
// cast to the generated type only at the IPC boundary (`cfg as any`).
type SeqConfig = Omit<sequence.SequenceConfig, "convertValues">;

const ORDER_OPTIONS = [
  { value: "", label: "Default (NOORDER)" },
  { value: "ORDER", label: "ORDER" },
  { value: "NOORDER", label: "NOORDER" },
];

export default function CreateSequenceModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<SeqConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    start: 1,
    increment: 1,
    ordered: "",
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateSequenceSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof SeqConfig>(key: K, value: SeqConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // INCREMENT BY must be non-zero (it may be negative for a descending
  // sequence); Snowflake rejects 0, so block submission on it.
  const canSubmit = cfg.name.trim().length > 0 && cfg.increment !== 0;

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
      icon={<NumberOutlined />}
      title="Create Sequence"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Sequence creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Sequence name"
          placeholder="MY_SEQUENCE"
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

        <Form.Item label="Start With" style={itemStyle}>
          <InputNumber
            value={cfg.start}
            onChange={(v) => set("start", (v ?? 1) as number)}
            style={{ width: 200 }}
          />
        </Form.Item>

        <Form.Item
          label="Increment By"
          style={itemStyle}
          validateStatus={cfg.increment === 0 ? "error" : undefined}
          help={cfg.increment === 0 ? "Increment must be non-zero (use a negative value for a descending sequence)." : undefined}
        >
          <InputNumber
            value={cfg.increment}
            onChange={(v) => set("increment", (v ?? 1) as number)}
            style={{ width: 200 }}
          />
        </Form.Item>

        <Form.Item label="Ordering" style={itemStyle}>
          <Select
            value={cfg.ordered}
            onChange={(v) => set("ordered", v)}
            options={ORDER_OPTIONS}
            style={{ width: 280 }}
          />
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
