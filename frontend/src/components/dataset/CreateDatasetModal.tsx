// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Checkbox, Alert } from "antd";
import { DotChartOutlined } from "@ant-design/icons";
import { BuildCreateDatasetSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// Local config shape — the Wails-generated DatasetConfig class carries a
// `convertValues` method a plain object literal can't satisfy, so we keep a plain
// interface and cast `cfg as any` only at the IPC boundary.
interface DatasetCfg {
  name: string;
  caseSensitive: boolean;
  orReplace: boolean;
  ifNotExists: boolean;
}

export default function CreateDatasetModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<DatasetCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateDatasetSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof DatasetCfg>(key: K, value: DatasetCfg[K]) =>
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

  return (
    <CreateModalShell
      icon={<DotChartOutlined />}
      title="Create Dataset"
      subtitle={`${db}.${schema}`}
      width={640}
      error={error}
      errorTitle="Dataset creation failed"
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
          message="Datasets hold versioned, immutable snapshots of data for ML training. CREATE DATASET makes an empty dataset; add data one version at a time from the dataset's Properties (Add version…), or via the Snowpark ML Python API."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Dataset name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_DATASET"
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

        <Form.Item style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <div style={{ height: 12 }} />
        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
