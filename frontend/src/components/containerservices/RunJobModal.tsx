// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Snowpark & Developer Workflows

import { useEffect, useState } from "react";
import { Checkbox, Form, Input, InputNumber, Radio, Segmented, Select } from "antd";
import { PlayCircleOutlined } from "@ant-design/icons";
import {
  BuildExecuteInferenceJobServiceSql, BuildExecuteJobServiceSql, ListComputePools, ListWarehouses,
} from "../../../wailsjs/go/app/App";
import { useQueryStore } from "../../store/queryStore";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import MonacoSqlField from "../shared/MonacoSqlField";
import StageFilePicker from "../shared/StageFilePicker";
import { useSqlPreview } from "../shared/createModalHooks";
import ServiceSpecFields, { EMPTY_SPEC, specReady, type SpecFieldsValue } from "../service/ServiceSpecFields";
import { loadModelsCached } from "../model/ModelSourcePicker";
import type { service as svcModels } from "../../../wailsjs/go/models";

interface Props {
  onClose: () => void;
  /** Called after the statement is handed to a new query tab. */
  onRan: (async: boolean) => void;
}

const JOB_SPEC_PLACEHOLDER = `spec:
  containers:
  - name: main
    image: /db/schema/repo/image:latest
    args: ["--run-once"]`;

const INFERENCE_SPEC_PLACEHOLDER = `output:
  stage_location: "@db.schema.stage/results/"`;

const DEFAULT_QUERY = "SELECT * FROM ";

/** The inference-only fields of InferenceJobConfig (pool/name/async/replicas are shared). */
type InferenceFields = Pick<svcModels.InferenceJobConfig,
  "spec" | "inputSource" | "query" | "stagePath" | "model" | "version" | "function">;

/**
 * Run Job dialog (issue #942): EXECUTE JOB SERVICE or EXECUTE INFERENCE JOB
 * SERVICE, built live in the backend and run in a new query tab, where a
 * synchronous job's output shows and it can be cancelled.
 */
export default function RunJobModal({ onClose, onRan }: Props) {
  const [kind, setKind] = useState<"job" | "inference">("job");
  const [pool, setPool] = useState("");
  const [name, setName] = useState("");
  const [async, setAsync] = useState(false);
  const [replicas, setReplicas] = useState("");
  // EXECUTE JOB SERVICE
  const [spec, setSpec] = useState<SpecFieldsValue>(EMPTY_SPEC);
  const [warehouse, setWarehouse] = useState("");
  const [eai, setEai] = useState("");
  const [comment, setComment] = useState("");
  // EXECUTE INFERENCE JOB SERVICE
  const [inf, setInf] = useState<InferenceFields>({
    spec: "", inputSource: "query", query: DEFAULT_QUERY, stagePath: "", model: "", version: "", function: "",
  });
  const setI = (patch: Partial<InferenceFields>) => setInf((prev) => ({ ...prev, ...patch }));

  const [pools, setPools] = useState<string[]>([]);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [models, setModels] = useState<string[] | null>(null);
  useEffect(() => {
    ListComputePools().then((n) => setPools(n ?? [])).catch(() => {});
    ListWarehouses().then((n) => setWarehouses(n ?? [])).catch(() => {});
  }, []);
  useEffect(() => {
    if (kind === "inference" && models === null) loadModelsCached().then(setModels).catch(() => setModels([]));
  }, [kind, models]);

  const executeInNewTab = useQueryStore((s) => s.executeInNewTab);

  const preview = useSqlPreview(
    () => kind === "job"
      ? BuildExecuteJobServiceSql({
        ...spec, name, computePool: pool, async, replicas,
        queryWarehouse: warehouse, externalAccessIntegrations: eai, comment,
      } as any)
      : BuildExecuteInferenceJobServiceSql({ ...inf, name, computePool: pool, async, replicas } as any),
    [kind, pool, name, async, replicas, spec, warehouse, eai, comment, inf],
    { blankOnError: true },
  );

  const canSubmit = preview !== "" && pool !== "" && (kind === "job"
    ? specReady(spec)
    : inf.spec.trim() !== "" && inf.model !== ""
      && (inf.inputSource === "stage" ? inf.stagePath.trim() !== "" : inf.query.trim() !== "" && inf.query !== DEFAULT_QUERY));

  const run = () => {
    if (!canSubmit) return;
    executeInNewTab(preview);
    onRan(async);
    onClose();
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };
  const grid2: React.CSSProperties = { display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" };
  const opts = (names: string[]) => names.map((n) => ({ value: n, label: n }));

  return (
    <CreateModalShell
      icon={<PlayCircleOutlined />}
      title="Run Job"
      width={760}
      creating={false}
      canSubmit={canSubmit}
      okText="Run"
      okIcon={<PlayCircleOutlined />}
      onClose={onClose}
      onSubmit={run}
    >
      <Form layout="vertical" size="small">
        <Segmented
          block
          style={{ marginBottom: 12 }}
          value={kind}
          onChange={(v) => setKind(v as "job" | "inference")}
          options={[{ value: "job", label: "Job" }, { value: "inference", label: "Inference job" }]}
        />

        <div style={grid2}>
          <Form.Item label="Compute pool" required style={itemStyle}>
            <Select
              showSearch value={pool || undefined} onChange={(v) => setPool(v ?? "")}
              options={opts(pools)} placeholder="Select a compute pool"
            />
          </Form.Item>
          <Form.Item label="Name" style={itemStyle} help="Optional [db.schema.]name; Snowflake generates JOB_<uuid> when blank.">
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_DB.MY_SCHEMA.MY_JOB" />
          </Form.Item>
        </div>

        {kind === "job" ? (
          <ServiceSpecFields
            db="" schema="" value={spec} noun="Job" placeholder={JOB_SPEC_PLACEHOLDER}
            onChange={(patch) => setSpec((prev) => ({ ...prev, ...patch }))}
          />
        ) : (
          <>
            <Form.Item label="Specification (YAML)" required style={itemStyle}>
              <Input.TextArea
                value={inf.spec} onChange={(e) => setI({ spec: e.target.value })}
                placeholder={INFERENCE_SPEC_PLACEHOLDER}
                autoSize={{ minRows: 4, maxRows: 12 }}
                style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}
              />
            </Form.Item>
            <Form.Item label="Input" style={{ marginBottom: 6 }}>
              <Radio.Group
                value={inf.inputSource} onChange={(e) => setI({ inputSource: e.target.value })}
                optionType="button" buttonStyle="solid" size="small"
                options={[{ value: "query", label: "Query" }, { value: "stage", label: "Stage path" }]}
              />
            </Form.Item>
            {inf.inputSource === "query" ? (
              <MonacoSqlField
                label="Input query" required value={inf.query} onChange={(v) => setI({ query: v })}
                placeholder={DEFAULT_QUERY} height={110}
                objectKinds={["TABLE", "VIEW"]} defaultDb="" defaultSchema=""
                notFoundText="No tables or views" itemStyle={itemStyle}
              />
            ) : (
              <>
                <Form.Item style={itemStyle}>
                  <StageFilePicker
                    db="" schema="" label="Browse internal stage — select an input file"
                    onPick={(stage, file) => setI({ stagePath: `@${stage}/${file}` })}
                  />
                </Form.Item>
                <Form.Item label="Stage path" required style={itemStyle}>
                  <Input value={inf.stagePath} onChange={(e) => setI({ stagePath: e.target.value })} placeholder="@my_stage/input/" />
                </Form.Item>
              </>
            )}
            <Form.Item label="Model" required style={itemStyle}>
              <Select
                showSearch value={inf.model || undefined} onChange={(v) => setI({ model: v ?? "" })}
                options={opts(models ?? [])} loading={models === null} placeholder="Select a model"
                notFoundContent={models === null ? "Loading…" : "No models found"}
              />
            </Form.Item>
            <div style={grid2}>
              <Form.Item label="Version" style={itemStyle} help="Version or alias; the default version when blank.">
                <Input value={inf.version} onChange={(e) => setI({ version: e.target.value })} placeholder="V1" />
              </Form.Item>
              <Form.Item label="Function" style={itemStyle} help="Model method to call.">
                <Input value={inf.function} onChange={(e) => setI({ function: e.target.value })} placeholder="predict" />
              </Form.Item>
            </div>
          </>
        )}

        <div style={grid2}>
          <Form.Item label="Replicas" style={itemStyle}>
            <InputNumber
              min={1} style={{ width: "100%" }} placeholder="1"
              value={replicas ? Number(replicas) : undefined}
              onChange={(v) => setReplicas(v == null ? "" : String(v))}
            />
          </Form.Item>
          <Form.Item label=" " style={itemStyle} help="Return immediately; the job keeps running and shows in the list.">
            <Checkbox checked={async} onChange={(e) => setAsync(e.target.checked)}>ASYNC</Checkbox>
          </Form.Item>
        </div>

        {kind === "job" && (
          <>
            <div style={grid2}>
              <Form.Item label="Query warehouse" style={itemStyle}>
                <Select
                  showSearch allowClear value={warehouse || undefined} onChange={(v) => setWarehouse(v ?? "")}
                  options={opts(warehouses)} placeholder="(optional)"
                />
              </Form.Item>
              <Form.Item label="External access integrations" style={itemStyle}>
                <Input value={eai} onChange={(e) => setEai(e.target.value)} placeholder="EAI_ONE, EAI_TWO" />
              </Form.Item>
            </div>
            <Form.Item label="Comment" style={itemStyle}>
              <Input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="optional comment" />
            </Form.Item>
          </>
        )}

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
