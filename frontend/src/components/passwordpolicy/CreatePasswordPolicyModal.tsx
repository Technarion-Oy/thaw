// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, InputNumber, Typography, Row, Col } from "antd";
import { SafetyCertificateOutlined } from "@ant-design/icons";
import { BuildCreatePasswordPolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { passwordpolicy as ppModels } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Plain form state. Each numeric parameter is `number | null`: null means
// "leave at Snowflake's default" (the builder omits it); the placeholder shows
// what that default is so the field reads as a deviation from a known baseline.
// The Wails-generated config class carries a `convertValues` method which a
// plain object literal can't satisfy, so we cast at the IPC boundary
// (`cfg as any`).
type PwdParam = keyof Pick<
  ppModels.PasswordPolicyConfig,
  | "minLength" | "maxLength" | "minUpperCaseChars" | "minLowerCaseChars"
  | "minNumericChars" | "minSpecialChars" | "minAgeDays" | "maxAgeDays"
  | "maxRetries" | "lockoutTimeMins" | "history"
>;

type PwdCfg = {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  comment: string;
} & Record<PwdParam, number | null>;

// Per-parameter UI metadata: the column label, the Snowflake default (shown as
// placeholder), and the documented valid range used for InputNumber bounds.
interface ParamMeta { key: PwdParam; label: string; def: number; min: number; max: number; }

const COMPLEXITY: ParamMeta[] = [
  { key: "minLength", label: "Min length", def: 14, min: 8, max: 256 },
  { key: "maxLength", label: "Max length", def: 256, min: 8, max: 256 },
  { key: "minUpperCaseChars", label: "Min uppercase", def: 1, min: 0, max: 256 },
  { key: "minLowerCaseChars", label: "Min lowercase", def: 1, min: 0, max: 256 },
  { key: "minNumericChars", label: "Min numeric", def: 1, min: 0, max: 256 },
  { key: "minSpecialChars", label: "Min special", def: 0, min: 0, max: 256 },
];

const AGE_HISTORY: ParamMeta[] = [
  { key: "minAgeDays", label: "Min age (days)", def: 0, min: 0, max: 999 },
  { key: "maxAgeDays", label: "Max age (days)", def: 90, min: 0, max: 999 },
  { key: "history", label: "History (reuse)", def: 5, min: 0, max: 24 },
];

const RETRY_LOCKOUT: ParamMeta[] = [
  { key: "maxRetries", label: "Max retries", def: 5, min: 1, max: 10 },
  { key: "lockoutTimeMins", label: "Lockout time (mins)", def: 15, min: 1, max: 999 },
];

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase", margin: "4px 0 8px",
};

export default function CreatePasswordPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<PwdCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    comment: "",
    minLength: null,
    maxLength: null,
    minUpperCaseChars: null,
    minLowerCaseChars: null,
    minNumericChars: null,
    minSpecialChars: null,
    minAgeDays: null,
    maxAgeDays: null,
    maxRetries: null,
    lockoutTimeMins: null,
    history: null,
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreatePasswordPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof PwdCfg>(key: K, value: PwdCfg[K]) =>
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

  const renderParams = (metas: ParamMeta[]) => (
    <Row gutter={12}>
      {metas.map((m) => (
        <Col span={8} key={m.key} style={{ marginBottom: 12 }}>
          <Text style={{ fontSize: 12, display: "block", marginBottom: 2 }}>{m.label}</Text>
          <InputNumber
            size="small"
            value={cfg[m.key]}
            min={m.min}
            max={m.max}
            placeholder={`default ${m.def}`}
            onChange={(v) => set(m.key, v ?? null)}
            style={{ width: "100%" }}
          />
        </Col>
      ))}
    </Row>
  );

  return (
    <CreateModalShell
      icon={<SafetyCertificateOutlined />}
      title="Create Password Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Password policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="STRICT_PASSWORD_POLICY"
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

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 12 }}>
          Leave a field empty to inherit Snowflake's default (shown as the placeholder).
          Only the parameters you set are written into the policy.
        </Text>

        <div style={SECTION_HEAD}>Complexity</div>
        {renderParams(COMPLEXITY)}

        <div style={SECTION_HEAD}>Age &amp; history</div>
        {renderParams(AGE_HISTORY)}

        <div style={SECTION_HEAD}>Retry &amp; lockout</div>
        {renderParams(RETRY_LOCKOUT)}

        <Form.Item label="Comment" style={{ ...itemStyle, marginTop: 8 }}>
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
