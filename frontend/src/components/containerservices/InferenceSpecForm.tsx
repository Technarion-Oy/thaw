// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useEffect, useState } from "react";
import { ListImageRepositories } from "../../../wailsjs/go/app/App";
import StageFilePicker from "../shared/StageFilePicker";
import { Bool, Grid, Group, KV, Num, Sel, Str, Tags, numberish, useSpec } from "../service/specFields";
import { rowsOf } from "./computePools";

/**
 * EXECUTE INFERENCE JOB SERVICE … WITH SPECIFICATION form (issue #958),
 * rendered inside SpecEditor. A flat schema unrelated to the service spec.
 */
export default function InferenceSpecForm() {
  const { set } = useSpec();
  const [repos, setRepos] = useState<string[]>([]);
  useEffect(() => {
    ListImageRepositories()
      .then((r) => setRepos(rowsOf(r).map((x) => `${x.database_name}.${x.schema_name}.${x.name}`)))
      .catch(() => {});
  }, []);

  return (
    <Grid>
      <Group title="Output" open>
        <Str p={["output", "stage_location"]} label="Stage location" span={2} placeholder="@db.schema.stage/results/" />
        <Sel p={["output", "mode"]} label="Mode" options={["error", "overwrite"]} placeholder="error" />
        <details style={{ gridColumn: "1 / -1", marginBottom: 8 }}>
          <summary style={{ cursor: "pointer", fontSize: 11 }}>Browse stage — pick any file in the target folder</summary>
          {/* ponytail: the picker returns files; the file's folder becomes the output location. */}
          <StageFilePicker db="" schema="" label="Select a file inside the output folder"
            onPick={(stage, file) => set(["output", "stage_location"], `@${stage}/${file.replace(/[^/]*$/, "")}`)} />
        </details>
      </Group>
      <Group title="Resources" p={["resources"]}>
        <Str p={["resources", "cpu_requests"]} label="CPU requests" numeric placeholder="4" />
        <Str p={["resources", "memory_requests"]} label="Memory requests" placeholder="8GiB" />
        <Num p={["resources", "gpu_requests"]} label="GPU requests" min={0} />
      </Group>
      <Group title="Inference" p={["inference"]}>
        <Num p={["inference", "num_workers"]} label="Workers" min={1} />
        <Num p={["inference", "max_batch_rows"]} label="Max batch rows" min={1} />
        <Sel p={["inference", "engine_options", "engine"]} label="Engine" options={["DEFAULT", "VLLM", "PYTHON_GENERIC"]} placeholder="DEFAULT" />
        <Tags p={["inference", "engine_options", "engine_args_override"]} label="Engine args override" span={3} placeholder="--max-model-len=4096" />
      </Group>
      <Group title="Input" p={["input"]}>
        <KV p={["input", "params"]} label="Params (keyword arguments)" keyPh="name" wrap={numberish} />
        <KV p={["input", "column_handling"]} label="Column handling (stage-path columns)" keyPh="COLUMN" valPh="convert to"
          valueOptions={["raw_bytes", "base64", "base64_data_url"]}
          wrap={(convert_to) => ({ input_format: "full_stage_path", convert_to })}
          unwrap={(h) => String((h as { convert_to?: string })?.convert_to ?? "")} />
        <Str p={["input", "partition_column"]} label="Partition column" />
      </Group>
      <Group title="Image build" p={["image_build"]}>
        <Str p={["image_build", "image_repo"]} label="Image repository" span={2} options={repos} placeholder="db.schema.repo" />
        <Bool p={["image_build", "force_rebuild"]} label="Force rebuild" />
      </Group>
    </Grid>
  );
}
