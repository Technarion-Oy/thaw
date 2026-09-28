// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { Form, Input, Checkbox, Radio, Button, Space, Typography } from "antd";
import { PlusOutlined, MinusCircleOutlined } from "@ant-design/icons";
import StageFilePicker from "../shared/StageFilePicker";
import type { service as svcModels } from "../../../wailsjs/go/models";

const { Text } = Typography;

export type TemplateVar = Omit<svcModels.TemplateVar, "convertValues">;

/** The spec-source fields shared by ServiceConfig and JobServiceConfig. */
export interface SpecFieldsValue {
  specSource: string; // "inline" | "stage"
  template: boolean;
  specInline: string;
  specStage: string;
  specFile: string;
  templateVars: TemplateVar[];
}

export const EMPTY_SPEC: SpecFieldsValue = {
  specSource: "inline", template: false, specInline: "", specStage: "", specFile: "", templateVars: [],
};

export const specReady = (v: SpecFieldsValue) =>
  v.specSource === "stage"
    ? v.specStage.trim().length > 0 && v.specFile.trim().length > 0
    : v.specInline.trim().length > 0;

interface Props {
  db: string;
  schema: string;
  value: SpecFieldsValue;
  onChange: (patch: Partial<SpecFieldsValue>) => void;
  /** Placeholder YAML for the inline editor. */
  placeholder: string;
  /** "Service" / "Job" — used in the inline editor label. */
  noun: string;
}

/**
 * Specification section shared by CREATE SERVICE and EXECUTE JOB SERVICE: inline
 * YAML or a staged file, optionally a template with USING ( k => v ) variables.
 */
export default function ServiceSpecFields({ db, schema, value: cfg, onChange, placeholder, noun }: Props) {
  const itemStyle: React.CSSProperties = { marginBottom: 12 };
  const setVars = (templateVars: TemplateVar[]) => onChange({ templateVars });
  const updateVar = (i: number, field: "key" | "value", val: string) =>
    setVars(cfg.templateVars.map((v, idx) => (idx === i ? { ...v, [field]: val } : v)));

  return (
    <>
      <Form.Item label="Specification" style={{ marginBottom: 6 }}>
        <Space size={16} wrap>
          <Radio.Group
            value={cfg.specSource}
            onChange={(e) => onChange({ specSource: e.target.value })}
            optionType="button"
            buttonStyle="solid"
            size="small"
          >
            <Radio.Button value="inline">Inline YAML</Radio.Button>
            <Radio.Button value="stage">From stage file</Radio.Button>
          </Radio.Group>
          <Checkbox checked={cfg.template} onChange={(e) => onChange({ template: e.target.checked })}>
            Template (with variables)
          </Checkbox>
        </Space>
      </Form.Item>

      {cfg.specSource === "stage" ? (
        <>
          <Form.Item style={{ marginBottom: 12 }}>
            <StageFilePicker
              db={db}
              schema={schema}
              label="Browse internal stage — select the specification file"
              onPick={(stage, file) => onChange({ specStage: stage, specFile: file })}
            />
          </Form.Item>
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
            <Form.Item label="Stage" required style={itemStyle} help="Internal stage (browse above, or type @stage / db.schema.stage).">
              <Input
                value={cfg.specStage}
                onChange={(e) => onChange({ specStage: e.target.value })}
                placeholder="@my_stage"
              />
            </Form.Item>
            <Form.Item label="Specification file" required style={itemStyle} help="Path to the YAML file within the stage.">
              <Input
                value={cfg.specFile}
                onChange={(e) => onChange({ specFile: e.target.value })}
                placeholder="service/spec.yaml"
              />
            </Form.Item>
          </div>
        </>
      ) : (
        <Form.Item
          label={cfg.template ? `${noun} specification template (YAML, with {{ variables }})` : `${noun} specification (YAML)`}
          required
          style={itemStyle}
        >
          <Input.TextArea
            value={cfg.specInline}
            onChange={(e) => onChange({ specInline: e.target.value })}
            placeholder={placeholder}
            autoSize={{ minRows: 8, maxRows: 18 }}
            style={{ fontFamily: "var(--font-mono)", fontSize: 12 }}
          />
        </Form.Item>
      )}

      {cfg.template && (
        <Form.Item
          label="Template variables (USING)"
          style={itemStyle}
          help="Bound to the template's {{ variables }}. Numbers and TRUE/FALSE/NULL are emitted unquoted; everything else is a string literal."
        >
          <Space direction="vertical" style={{ width: "100%" }} size={6}>
            {cfg.templateVars.map((v, i) => (
              <Space key={i} style={{ width: "100%" }} align="center">
                <Input
                  value={v.key}
                  onChange={(e) => updateVar(i, "key", e.target.value)}
                  placeholder="name"
                  style={{ width: 200 }}
                />
                <Text type="secondary">=&gt;</Text>
                <Input
                  value={v.value}
                  onChange={(e) => updateVar(i, "value", e.target.value)}
                  placeholder="value"
                  style={{ width: 260 }}
                />
                <Button
                  type="text" size="small" icon={<MinusCircleOutlined />}
                  onClick={() => setVars(cfg.templateVars.filter((_, idx) => idx !== i))}
                />
              </Space>
            ))}
            <Button
              type="dashed" size="small" icon={<PlusOutlined />}
              onClick={() => setVars([...cfg.templateVars, { key: "", value: "" }])}
            >
              Add variable
            </Button>
          </Space>
        </Form.Item>
      )}
    </>
  );
}
