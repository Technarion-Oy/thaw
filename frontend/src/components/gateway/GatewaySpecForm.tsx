// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { Form, InputNumber, Segmented, Typography } from "antd";
import { getIn, type Doc } from "../service/specDoc";
import { List, useSpec } from "../service/specFields";
import EndpointTargetPicker from "./EndpointTargetPicker";

const { Text } = Typography;

interface Props {
  defaultDb: string;
  defaultSchema: string;
}

const endpoint = (value?: string, weight?: number) => ({ type: "endpoint", ...(value ? { value } : {}), ...(weight != null ? { weight } : {}) });

/**
 * Gateway specification form (issue #958), rendered inside SpecEditor:
 * traffic split (≤ 5 weighted targets summing to 100) or shadow traffic (one
 * primary endpoint, weighted shadow endpoints). Switching type carries the
 * first endpoint over.
 */
export default function GatewaySpecForm({ defaultDb, defaultSchema }: Props) {
  const { doc, set } = useSpec();
  const spec = doc?.spec ?? {};
  const type = spec.type === "shadow_traffic" ? "shadow_traffic" : "traffic_split";
  const targets: Doc[] = Array.isArray(spec.targets) ? spec.targets : [];
  const sum = targets.reduce((s, t) => s + (Number(t?.weight) || 0), 0);

  const switchType = (t: string) => {
    const first: string | undefined = (type === "traffic_split" ? targets[0] : spec.primary?.[0])?.value;
    set(["spec"], t === "traffic_split"
      ? { type: t, split_type: "custom", targets: [endpoint(first, 100)] }
      : { type: t, primary: [endpoint(first)], shadow: [endpoint(undefined, 10)] });
  };

  const row = (base: (string | number)[], weighted: boolean) => (
    <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
      <div style={{ flex: 1 }}>
        <EndpointTargetPicker defaultDb={defaultDb} defaultSchema={defaultSchema}
          value={getIn(doc, [...base, "value"]) ?? ""} onChange={(v) => set([...base, "value"], v)} />
      </div>
      {weighted && (
        <InputNumber size="small" min={0} max={100} addonBefore="wt" style={{ width: 100 }}
          value={getIn(doc, [...base, "weight"]) ?? null} onChange={(n) => set([...base, "weight"], n ?? undefined)} />
      )}
    </div>
  );

  return (
    <div>
      <Segmented size="small" style={{ marginBottom: 8 }} value={type} onChange={(v) => switchType(String(v))}
        options={[{ value: "traffic_split", label: "Traffic split" }, { value: "shadow_traffic", label: "Shadow traffic" }]} />
      {type === "traffic_split" ? (
        <>
          <List p={["spec", "targets"]} noun="target" max={5} item={() => endpoint(undefined, Math.max(0, 100 - sum))}>
            {(base) => row(base, true)}
          </List>
          <Text type={sum === 100 ? "secondary" : "warning"} style={{ fontSize: 11 }}>
            Weights sum to {sum} / 100 · up to 5 targets
          </Text>
        </>
      ) : (
        <>
          <Form.Item label="Primary endpoint" style={{ marginBottom: 8 }} help="Serves the responses.">
            {row(["spec", "primary", 0], false)}
          </Form.Item>
          <List p={["spec", "shadow"]} label="Shadow endpoints" noun="shadow endpoint" item={() => endpoint(undefined, 10)}>
            {(base) => row(base, true)}
          </List>
          <Text type="secondary" style={{ fontSize: 11 }}>
            Each shadow endpoint receives a copy of that percentage of requests; its responses are discarded.
          </Text>
        </>
      )}
    </div>
  );
}
