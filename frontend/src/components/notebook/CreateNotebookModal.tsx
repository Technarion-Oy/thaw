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

import { useEffect, useState } from "react";
import { Form, Input, Checkbox, Select } from "antd";
import { ExperimentOutlined } from "@ant-design/icons";
import { BuildCreateNotebookSql, ExecDDL, ListWarehouses } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import StageFilePicker from "../shared/StageFilePicker";
import type { notebook as nbModels } from "../../../wailsjs/go/models";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// The Wails-generated config class carries a `convertValues` method that a plain
// object literal can't satisfy; we cast to the generated type only at the IPC
// boundary (`cfg as any`).
type NotebookCfg = Omit<nbModels.NotebookConfig, "convertValues">;

export default function CreateNotebookModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<NotebookCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    sourceLocation: "",
    mainFile: "",
    queryWarehouse: "",
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateNotebookSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [loadingWarehouses, setLoadingWarehouses] = useState(false);

  useEffect(() => {
    setLoadingWarehouses(true);
    ListWarehouses()
      .then((names) => setWarehouses(names ?? []))
      .catch(() => {})
      .finally(() => setLoadingWarehouses(false));
  }, []);

  const set = <K extends keyof NotebookCfg>(key: K, value: NotebookCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // When a file is picked in the stage browser, split it into the directory
  // (which, with the stage, forms the FROM source location) and the file name
  // (the relative MAIN_FILE). `stage` is the quoted, qualified stage identifier
  // without a leading @; `file` is the path within the stage.
  const onPickFile = (stage: string, file: string) => {
    const slash = file.lastIndexOf("/");
    const dir = slash >= 0 ? file.slice(0, slash) : "";
    const mainFile = slash >= 0 ? file.slice(slash + 1) : file;
    const sourceLocation = `@${stage}${dir ? `/${dir}` : ""}`;
    setCfg((prev) => ({ ...prev, sourceLocation, mainFile }));
  };

  // Only the name is required; an empty notebook (no FROM/MAIN_FILE) is valid —
  // Snowflake provisions an editable notebook on first open. When a source
  // location is supplied a main file is required to locate the .ipynb.
  const canSubmit =
    cfg.name.trim().length > 0 &&
    (cfg.sourceLocation.trim().length === 0 || cfg.mainFile.trim().length > 0);

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const warehouseOptions = (warehouses || []).map((n) => ({ value: n, label: n }));
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<ExperimentOutlined />}
      title="Create Notebook"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Notebook creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Notebook name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_NOTEBOOK"
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

        <Form.Item label="Query warehouse" style={itemStyle} help="Warehouse the notebook runs its cells and SQL queries on.">
          <Select
            showSearch
            allowClear
            value={cfg.queryWarehouse || undefined}
            onChange={(v) => set("queryWarehouse", v ?? "")}
            options={warehouseOptions}
            placeholder="(optional)"
            loading={loadingWarehouses}
            notFoundContent={loadingWarehouses ? "Loading…" : "No warehouses found"}
          />
        </Form.Item>

        <Form.Item
          label="Create from staged file"
          style={{ marginBottom: 8 }}
          help="Optional — browse a stage and select an existing .ipynb file to seed the notebook. Leave blank to create an empty notebook."
        >
          <StageFilePicker db={db} schema={schema} label="Browse internal stage — select an existing notebook file" onPick={onPickFile} />
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Source location (FROM)" style={itemStyle} help="Stage path holding the notebook files, e.g. @db.schema.stage/dir.">
            <Input
              value={cfg.sourceLocation}
              onChange={(e) => set("sourceLocation", e.target.value)}
              placeholder="@my_db.my_schema.my_stage/nb"
            />
          </Form.Item>
          <Form.Item
            label="Main file"
            required={cfg.sourceLocation.trim().length > 0}
            style={itemStyle}
            help="Path to the .ipynb entry point, relative to the source location."
          >
            <Input
              value={cfg.mainFile}
              onChange={(e) => set("mainFile", e.target.value)}
              placeholder="notebook_app.ipynb"
            />
          </Form.Item>
        </div>

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
