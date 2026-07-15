// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, InputNumber, Select, Typography, Row, Col } from "antd";
import { FieldTimeOutlined } from "@ant-design/icons";
import { BuildCreateSessionPolicySql, ExecDDL, ReconcileSecondaryRoles } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { sessionpolicy as spModels } from "../../../wailsjs/go/models";

const { Text } = Typography;

// Plain form state. Each timeout parameter is `number | null`: null means "leave
// at Snowflake's default" (the builder omits it); the placeholder shows what
// that default is so the field reads as a deviation from a known baseline. The
// secondary-role lists are string[] (each entry is either "ALL" or a role name).
// The Wails-generated config class carries a `convertValues` method which a
// plain object literal can't satisfy, so we cast at the IPC boundary
// (`cfg as any`).
type TimeoutParam = keyof Pick<
  spModels.SessionPolicyConfig,
  "idleTimeoutMins" | "uiIdleTimeoutMins" | "maxLifespanMins" | "uiMaxLifespanMins"
>;

type SessionCfg = {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  allowedSecondaryRoles: string[];
  blockedSecondaryRoles: string[];
  comment: string;
} & Record<TimeoutParam, number | null>;

// Per-parameter UI metadata: the column label, the Snowflake default (shown as
// placeholder), and the documented valid range used for InputNumber bounds.
interface ParamMeta { key: TimeoutParam; label: string; def: number; min: number; max: number; }

const IDLE: ParamMeta[] = [
  { key: "idleTimeoutMins", label: "Idle timeout (mins)", def: 240, min: 5, max: 1440 },
  { key: "uiIdleTimeoutMins", label: "UI idle timeout (mins)", def: 240, min: 5, max: 1440 },
];

const LIFESPAN: ParamMeta[] = [
  { key: "maxLifespanMins", label: "Max lifespan (mins)", def: 0, min: 0, max: 43200 },
  { key: "uiMaxLifespanMins", label: "UI max lifespan (mins)", def: 0, min: 0, max: 43200 },
];

const SECTION_HEAD: React.CSSProperties = {
  fontSize: 11, fontWeight: 600, color: "var(--text-muted)",
  letterSpacing: "0.05em", textTransform: "uppercase", margin: "4px 0 8px",
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateSessionPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<SessionCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    allowedSecondaryRoles: [],
    blockedSecondaryRoles: [],
    comment: "",
    idleTimeoutMins: null,
    uiIdleTimeoutMins: null,
    maxLifespanMins: null,
    uiMaxLifespanMins: null,
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateSessionPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof SessionCfg>(key: K, value: SessionCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Allowed tag-select onChange: run the new selection through the backend
  // ReconcileSecondaryRoles so ALL and named roles can't coexist (the invalid
  // ('ALL', R1) shape) before it reaches the live SQL preview.
  const setAllowedReconciled = async (v: string[]) =>
    set("allowedSecondaryRoles", (await ReconcileSecondaryRoles(v)) ?? []);

  // Blocked tag-select onChange: 'ALL' is not valid for BLOCKED_SECONDARY_ROLES,
  // so drop it even if the user types it free-form (the option isn't offered).
  const setBlocked = (v: string[]) =>
    set("blockedSecondaryRoles", v.filter((r) => r.trim().toUpperCase() !== "ALL"));

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
        <Col span={12} key={m.key} style={{ marginBottom: 12 }}>
          <Text style={{ fontSize: 12, display: "block", marginBottom: 2 }}>{m.label}</Text>
          <InputNumber
            size="small"
            value={cfg[m.key]}
            min={m.min}
            max={m.max}
            step={1}
            precision={0}
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
      icon={<FieldTimeOutlined />}
      title="Create Session Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Session policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="STRICT_SESSION_POLICY"
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
          Only the parameters you set are written into the policy. A lifespan of 0
          means no limit.
        </Text>

        <div style={SECTION_HEAD}>Idle timeout</div>
        {renderParams(IDLE)}

        <div style={SECTION_HEAD}>Maximum lifespan</div>
        {renderParams(LIFESPAN)}

        <div style={SECTION_HEAD}>Secondary roles</div>
        <Row gutter={12}>
          <Col span={12} style={{ marginBottom: 12 }}>
            <Text style={{ fontSize: 12, display: "block", marginBottom: 2 }}>Allowed</Text>
            <Select
              size="small"
              mode="tags"
              value={cfg.allowedSecondaryRoles}
              onChange={setAllowedReconciled}
              placeholder="default ('ALL')"
              tokenSeparators={[","]}
              style={{ width: "100%" }}
              options={[{ value: "ALL", label: "ALL" }]}
            />
          </Col>
          <Col span={12} style={{ marginBottom: 12 }}>
            <Text style={{ fontSize: 12, display: "block", marginBottom: 2 }}>Blocked</Text>
            {/* BLOCKED_SECONDARY_ROLES accepts only a role list — 'ALL' is valid
                solely for the allowed list — so no ALL option and no reconcile. */}
            <Select
              size="small"
              mode="tags"
              value={cfg.blockedSecondaryRoles}
              onChange={setBlocked}
              placeholder="role names"
              tokenSeparators={[","]}
              style={{ width: "100%" }}
            />
          </Col>
        </Row>
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 12 }}>
          For <strong>Allowed</strong>, enter <code>ALL</code> for every secondary
          role or type role names; <strong>Blocked</strong> takes role names only.
          Leave empty to inherit the default (allowed: <code>('ALL')</code>, blocked:
          none).
        </Text>

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
