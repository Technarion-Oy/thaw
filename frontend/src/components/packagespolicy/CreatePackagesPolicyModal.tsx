// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Select, Typography } from "antd";
import { CodeSandboxOutlined } from "@ant-design/icons";
import { BuildCreatePackagesPolicySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import NameWithReplaceOptions from "../shared/NameWithReplaceOptions";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

const { Text } = Typography;

// Plain form state. The three list parameters are string[] of package
// specifications (each empty leaves the parameter unset → the builder omits it,
// inheriting Snowflake's default: ALLOWLIST ('*'), BLOCKLIST () and
// ADDITIONAL_CREATION_BLOCKLIST ()). The Wails-generated config class carries a
// `convertValues` method which a plain object literal can't satisfy, so we cast
// at the IPC boundary (`cfg as any`).
type PackagesCfg = {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
  allowlist: string[];
  blocklist: string[];
  additionalCreationBlocklist: string[];
  comment: string;
};

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreatePackagesPolicyModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<PackagesCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    allowlist: [],
    blocklist: [],
    additionalCreationBlocklist: [],
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreatePackagesPolicySql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof PackagesCfg>(key: K, value: PackagesCfg[K]) =>
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
      icon={<CodeSandboxOutlined />}
      title="Create Packages Policy"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Packages policy creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <NameWithReplaceOptions
          label="Policy name"
          placeholder="PYTHON_PACKAGES_POLICY"
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

        <Form.Item label="Language" style={itemStyle} help="PYTHON is the only language Snowflake currently supports for packages policies.">
          <Select value="PYTHON" disabled options={[{ value: "PYTHON", label: "PYTHON" }]} style={{ width: "100%" }} />
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 12 }}>
          Each entry is a package specification — a bare name (<code>numpy</code>), a name with a
          version specifier (<code>numpy==1.26.4</code>, <code>pandas&gt;=2.0</code>), or the wildcard{" "}
          <code>*</code>. Leave a list empty to inherit Snowflake's default (allowlist <code>('*')</code>,
          blocklists empty). The blocklist takes precedence over the allowlist.
        </Text>

        <Form.Item label="Allowlist" style={itemStyle} help="Packages a UDF / procedure may import.">
          <Select
            mode="tags"
            value={cfg.allowlist}
            onChange={(v) => set("allowlist", v)}
            placeholder="default (* — all allowed)"
            tokenSeparators={[",", " "]}
            options={[{ value: "*", label: "* (all)" }]}
            style={{ width: "100%" }}
          />
        </Form.Item>

        <Form.Item label="Blocklist" style={itemStyle} help="Packages that are forbidden (takes precedence over the allowlist).">
          <Select
            mode="tags"
            value={cfg.blocklist}
            onChange={(v) => set("blocklist", v)}
            placeholder="default (none blocked)"
            tokenSeparators={[",", " "]}
            style={{ width: "100%" }}
          />
        </Form.Item>

        <Form.Item
          label="Additional creation blocklist"
          style={itemStyle}
          help="Packages blocked only when an object is created (not when it runs)."
        >
          <Select
            mode="tags"
            value={cfg.additionalCreationBlocklist}
            onChange={(v) => set("additionalCreationBlocklist", v)}
            placeholder="default (none blocked)"
            tokenSeparators={[",", " "]}
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
