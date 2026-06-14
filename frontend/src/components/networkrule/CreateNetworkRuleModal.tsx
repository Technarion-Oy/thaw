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
import {
  Form, Input, Select, Checkbox, Button, Space, Typography,
} from "antd";
import { GlobalOutlined, PlusOutlined, DeleteOutlined } from "@ant-design/icons";
import { BuildCreateNetworkRuleSql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import type { networkrule as nrModels } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

// TYPE selects which kind of network identifier the rule groups. Each option's
// help text mirrors the Snowflake docs; TYPE is fixed at creation.
const TYPE_OPTIONS = [
  { value: "IPV4", label: "IPV4 — IPv4 addresses / CIDR ranges" },
  { value: "IPV6", label: "IPV6 — IPv6 addresses (AWS, INGRESS only)" },
  { value: "AWSVPCEID", label: "AWSVPCEID — AWS PrivateLink VPC endpoint IDs" },
  { value: "AZURELINKID", label: "AZURELINKID — Azure Private Link IDs" },
  { value: "GCPPSCID", label: "GCPPSCID — Google Cloud PSC connection IDs" },
  { value: "HOST_PORT", label: "HOST_PORT — outbound host:port destinations" },
  { value: "PRIVATE_HOST_PORT", label: "PRIVATE_HOST_PORT — private host:port" },
  { value: "COMPUTE_POOL", label: "COMPUTE_POOL — Snowpark Container Services pools" },
];

const MODE_OPTIONS = [
  { value: "INGRESS", label: "INGRESS — restrict inbound access (default)" },
  { value: "EGRESS", label: "EGRESS — permit outbound requests" },
  { value: "INTERNAL_STAGE", label: "INTERNAL_STAGE — AWS internal stages (AWSVPCEID)" },
  { value: "SNOWFLAKE_MANAGED_STORAGE_VOLUME", label: "SNOWFLAKE_MANAGED_STORAGE_VOLUME (AWSVPCEID)" },
];

// Per-TYPE placeholder for the value rows, nudging the right identifier format.
const VALUE_PLACEHOLDER: Record<string, string> = {
  IPV4: "192.168.1.0/24",
  IPV6: "2001:db8::/32",
  AWSVPCEID: "vpce-0123456789abcdef0",
  AZURELINKID: "/subscriptions/…/privateEndpoints/…",
  GCPPSCID: "1234567890",
  HOST_PORT: "example.com:443",
  PRIVATE_HOST_PORT: "example.internal:443",
  COMPUTE_POOL: "MY_POOL",
};

// The Wails-generated config class carries a `convertValues` method (it has a
// nested `valueList` array) which a plain object literal can't satisfy; we cast
// to the generated type only at the IPC boundary (`cfg as any`).
type NetworkRuleCfg = Omit<nrModels.NetworkRuleConfig, "convertValues">;

export default function CreateNetworkRuleModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<NetworkRuleCfg>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    type: "IPV4",
    mode: "INGRESS",
    valueList: [""],
    comment: "",
  });

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () => BuildCreateNetworkRuleSql(db, schema, cfg as any),
    [db, schema, cfg],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const set = <K extends keyof NetworkRuleCfg>(key: K, value: NetworkRuleCfg[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const updateValue = (i: number, v: string) =>
    set("valueList", cfg.valueList.map((x, idx) => (idx === i ? v : x)));

  const addValue = () => set("valueList", [...cfg.valueList, ""]);

  const removeValue = (i: number) => set("valueList", cfg.valueList.filter((_, idx) => idx !== i));

  const canSubmit =
    cfg.name.trim().length > 0 &&
    cfg.type.trim().length > 0 &&
    cfg.mode.trim().length > 0;

  const handleRun = () => {
    if (!canSubmit) return;
    submit(async () => {
      await ExecDDL(preview);
      onSuccess?.();
      onClose();
    });
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };
  const placeholder = VALUE_PLACEHOLDER[cfg.type] ?? "value";

  return (
    <CreateModalShell
      icon={<GlobalOutlined />}
      title="Create Network Rule"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Network rule creation failed"
      onErrorClose={() => setError(null)}
      creating={creating}
      canSubmit={canSubmit}
      onClose={onClose}
      onSubmit={handleRun}
    >
      <Form layout="vertical" size="small">
        {/* NETWORK RULE has no IF NOT EXISTS form, so only OR REPLACE is offered. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Rule name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="ALLOW_OFFICE_IPS"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox checked={cfg.orReplace} onChange={(e) => set("orReplace", e.target.checked)}>
              OR REPLACE
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

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Type" required style={itemStyle} help="The kind of network identifier. Fixed at creation.">
            <Select
              value={cfg.type}
              options={TYPE_OPTIONS}
              onChange={(v) => set("type", v)}
              optionLabelProp="value"
            />
          </Form.Item>
          <Form.Item label="Mode" required style={itemStyle} help="How the rule is used. Fixed at creation.">
            <Select
              value={cfg.mode}
              options={MODE_OPTIONS}
              onChange={(v) => set("mode", v)}
              optionLabelProp="value"
            />
          </Form.Item>
        </div>

        <Form.Item
          label="Value list"
          style={itemStyle}
          help="Network identifiers for this rule. The expected format depends on the chosen type."
        >
          <Space direction="vertical" size={6} style={{ width: "100%" }}>
            {cfg.valueList.map((v, i) => (
              <Space key={i} align="baseline" style={{ width: "100%" }}>
                <Input
                  placeholder={placeholder}
                  value={v}
                  onChange={(e) => updateValue(i, e.target.value)}
                  style={{ width: 360 }}
                />
                {cfg.valueList.length > 1 && (
                  <Button
                    type="text"
                    size="small"
                    icon={<DeleteOutlined />}
                    onClick={() => removeValue(i)}
                    danger
                  />
                )}
              </Space>
            ))}
            <Button size="small" icon={<PlusOutlined />} onClick={addValue}>
              Add value
            </Button>
            <Text type="secondary" style={{ fontSize: 11 }}>
              An empty list is allowed — you can add identifiers later.
            </Text>
          </Space>
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
