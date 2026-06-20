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
import { Form, Input, Checkbox, Alert } from "antd";
import { RobotOutlined } from "@ant-design/icons";
import { BuildCreateModelSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import ModelSourcePicker, { invalidateModelsCache } from "./ModelSourcePicker";
import type { model as modelModels } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// The Wails-generated config class carries a `convertValues` method that a plain
// object literal can't satisfy; we cast to the generated type only at the IPC
// boundary (`cfg as any`).
type ModelCfg = Omit<modelModels.ModelConfig, "convertValues">;

export default function CreateModelModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<ModelCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    versionName: "",
    sourceType: "model",
    sourceModel: "",
    sourceVersion: "",
    stageLocation: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateModelSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof ModelCfg>(key: K, value: ModelCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const isStage = cfg.sourceType === "stage";
  const canSubmit =
    cfg.name.trim().length > 0 &&
    (isStage ? cfg.stageLocation.trim().length > 0 : cfg.sourceModel.trim().length > 0);

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      invalidateModelsCache(); // a new model is now a possible copy source
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<RobotOutlined />}
      title="Create Model"
      subtitle={`${db}.${schema}`}
      width={680}
      error={error}
      errorTitle="Model creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 12 }}
          message="Models are usually registered via the Snowpark ML Python API. CREATE MODEL here copies an existing model or loads serialized artifacts from an internal stage."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Model name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_MODEL"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={cfg.orReplace}
              onChange={(e) => setCfg((prev) => ({ ...prev, orReplace: e.target.checked, ifNotExists: e.target.checked ? false : prev.ifNotExists }))}
            >
              OR REPLACE
            </Checkbox>
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={cfg.ifNotExists}
              onChange={(e) => setCfg((prev) => ({ ...prev, ifNotExists: e.target.checked, orReplace: e.target.checked ? false : prev.orReplace }))}
            >
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item
          label="Version name (WITH VERSION)"
          style={itemStyle}
          help="Optional. Name of the version created in the new model."
        >
          <Input
            value={cfg.versionName}
            onChange={(e) => set("versionName", e.target.value)}
            placeholder="V1"
          />
        </Form.Item>

        <ModelSourcePicker
          db={db}
          schema={schema}
          value={cfg}
          onChange={(patch) => setCfg((prev) => ({ ...prev, ...patch }))}
        />

        <div style={{ height: 12 }} />
        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
