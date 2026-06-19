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
import { Form, Input, Select, Typography } from "antd";
import { LoginOutlined } from "@ant-design/icons";
import { BuildCreateAuthenticationPolicySql, ExecDDL, ReconcileAllExclusiveList } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

const { Text } = Typography;

// Allowed tokens for the list/enum parameters (Snowflake's documented values).
// AUTHENTICATION_METHODS / CLIENT_TYPES are fixed enumerations; SECURITY_INTEGRATIONS
// is free-form (integration names) plus the special ALL token.
const AUTH_METHOD_OPTIONS = ["ALL", "SAML", "PASSWORD", "OAUTH", "KEYPAIR", "PROGRAMMATIC_ACCESS_TOKEN", "WORKLOAD_IDENTITY"]
  .map((v) => ({ value: v, label: v }));
const CLIENT_TYPE_OPTIONS = ["ALL", "SNOWFLAKE_UI", "DRIVERS", "SNOWFLAKE_CLI", "SNOWSQL"]
  .map((v) => ({ value: v, label: v }));
const MFA_ENROLLMENT_OPTIONS = ["REQUIRED", "REQUIRED_PASSWORD_ONLY", "OPTIONAL"]
  .map((v) => ({ value: v, label: v }));

// Plain form state. The list parameters are string[] (each empty leaves the
// parameter unset → the builder omits it, inheriting Snowflake's ('ALL')
// default). The Wails-generated config class carries a `convertValues` method
// which a plain object literal can't satisfy, so we cast at the IPC boundary
// (`cfg as any`).
type AuthCfg = {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  authenticationMethods: string[];
  clientTypes: string[];
  securityIntegrations: string[];
  mfaEnrollment: string;
  comment: string;
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateAuthenticationPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<AuthCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    authenticationMethods: [],
    clientTypes: [],
    securityIntegrations: [],
    mfaEnrollment: "",
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateAuthenticationPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof AuthCfg>(key: K, value: AuthCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // ALL is mutually exclusive with specific values — reconcile the list params in
  // the backend (keeps whichever kind was chosen last) so CREATE can't emit an
  // invalid ('ALL', <specific>) list, matching the Properties modal.
  const setList = async (
    key: "authenticationMethods" | "clientTypes" | "securityIntegrations",
    v: string[],
  ) => set(key, (await ReconcileAllExclusiveList(v)) ?? []);

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
      icon={<LoginOutlined />}
      title="Create Authentication Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Authentication policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="STRICT_AUTH_POLICY"
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
          Leave a field empty to inherit Snowflake's default (the list parameters default to
          <code> ALL</code>, MFA enrollment to <code>OPTIONAL</code>). Only the parameters you set are
          written into the policy.
        </Text>

        <Form.Item label="Authentication methods" style={itemStyle}>
          <Select
            mode="multiple"
            value={cfg.authenticationMethods}
            onChange={(v) => setList("authenticationMethods", v)}
            placeholder="default (ALL)"
            options={AUTH_METHOD_OPTIONS}
            style={{ width: "100%" }}
          />
        </Form.Item>

        <Form.Item label="Client types" style={itemStyle}>
          <Select
            mode="multiple"
            value={cfg.clientTypes}
            onChange={(v) => setList("clientTypes", v)}
            placeholder="default (ALL)"
            options={CLIENT_TYPE_OPTIONS}
            style={{ width: "100%" }}
          />
        </Form.Item>

        <Form.Item
          label="Security integrations"
          style={itemStyle}
          help="Enter ALL or one or more security-integration names (e.g. SAML/OAuth integrations)."
        >
          <Select
            mode="tags"
            value={cfg.securityIntegrations}
            onChange={(v) => setList("securityIntegrations", v)}
            placeholder="default (ALL)"
            tokenSeparators={[","]}
            options={[{ value: "ALL", label: "ALL" }]}
            style={{ width: "100%" }}
          />
        </Form.Item>

        <Form.Item label="MFA enrollment" style={itemStyle}>
          <Select
            allowClear
            value={cfg.mfaEnrollment || undefined}
            onChange={(v) => set("mfaEnrollment", v ?? "")}
            placeholder="default (OPTIONAL)"
            options={MFA_ENROLLMENT_OPTIONS}
            style={{ width: "100%" }}
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
