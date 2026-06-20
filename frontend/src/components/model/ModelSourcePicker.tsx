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
import { Form, Input, Select } from "antd";
import { ListModels } from "../../../wailsjs/go/app/App";
import StageFilePicker from "../shared/StageFilePicker";

// `SHOW MODELS IN ACCOUNT` is an account-wide scan, and this picker remounts on
// every create-modal / add-version open (the latter via destroyOnClose). Cache
// the in-flight/resolved promise at module scope so the scan runs at most once
// per session; consumers that need a fresh list can call invalidateModelsCache().
let modelsCache: Promise<string[]> | null = null;
function loadModelsCached(): Promise<string[]> {
  if (!modelsCache) {
    modelsCache = ListModels()
      .then((names) => names ?? [])
      .catch((e) => { modelsCache = null; throw e; }); // don't cache failures
  }
  return modelsCache;
}
export function invalidateModelsCache() { modelsCache = null; }

// The FROM-clause source shared by CREATE MODEL and ALTER MODEL … ADD VERSION:
// either a copy of an existing model (optionally a specific version) or a load
// from an internal stage path. Both flows reuse this component so the model
// dropdown + stage browser live in one place.
export interface ModelSourceValue {
  // "model" | "stage" — typed as string so the Wails-generated ModelConfig (whose
  // sourceType is a plain string) can be passed straight through as the value.
  sourceType: string;
  sourceModel: string;
  sourceVersion: string;
  stageLocation: string;
}

interface Props {
  db: string;
  schema: string;
  value: ModelSourceValue;
  /** Patch one or more source fields. */
  onChange: (patch: Partial<ModelSourceValue>) => void;
}

export default function ModelSourcePicker({ db, schema, value, onChange }: Props) {
  const [models, setModels] = useState<string[]>([]);
  const [loadingModels, setLoadingModels] = useState(false);

  useEffect(() => {
    let alive = true;
    setLoadingModels(true);
    loadModelsCached()
      .then((names) => { if (alive) setModels(names); })
      .catch(() => { if (alive) setModels([]); })
      .finally(() => { if (alive) setLoadingModels(false); });
    return () => { alive = false; };
  }, []);

  const isStage = value.sourceType === "stage";

  // A user-typed source model (e.g. an FQN not yet listed, or one the role can't
  // SHOW) must still appear as a selectable option so the value isn't dropped.
  const modelOptions = (
    value.sourceModel && !models.includes(value.sourceModel)
      ? [value.sourceModel, ...models]
      : models
  ).map((n) => ({ value: n, label: n }));

  return (
    <>
      <Form.Item label="Source" style={{ marginBottom: 12 }}>
        <Select
          value={value.sourceType}
          onChange={(v) => onChange({ sourceType: v })}
          options={[
            { value: "model", label: "Copy from an existing model" },
            { value: "stage", label: "Load from an internal stage" },
          ]}
        />
      </Form.Item>

      {isStage ? (
        <Form.Item
          label="Stage location"
          required
          style={{ marginBottom: 0 }}
          help="Internal stage path holding the serialized model artifacts. Browse below to fill it in, then adjust to the artifact directory if needed."
        >
          <Input
            value={value.stageLocation}
            onChange={(e) => onChange({ stageLocation: e.target.value })}
            placeholder="@my_stage/model_path"
            style={{ marginBottom: 8 }}
          />
          <StageFilePicker
            db={db}
            schema={schema}
            label="Browse internal stage — select a file in the model artifact directory"
            onPick={(stage, file) => onChange({ stageLocation: `@${stage}/${file}` })}
          />
        </Form.Item>
      ) : (
        <>
          <Form.Item
            label="Source model"
            required
            style={{ marginBottom: 12 }}
            help="The model to copy from (any model your role can access)."
          >
            <Select
              showSearch
              value={value.sourceModel || undefined}
              onChange={(v) => onChange({ sourceModel: v ?? "" })}
              options={modelOptions}
              placeholder="Select a model…"
              loading={loadingModels}
              optionFilterProp="label"
              notFoundContent={loadingModels ? "Loading…" : "No models found"}
            />
          </Form.Item>
          <Form.Item
            label="Source version"
            style={{ marginBottom: 0 }}
            help="Optional. Which source version to copy; defaults to the source model's default version."
          >
            <Input
              value={value.sourceVersion}
              onChange={(e) => onChange({ sourceVersion: e.target.value })}
              placeholder="V1"
            />
          </Form.Item>
        </>
      )}
    </>
  );
}
