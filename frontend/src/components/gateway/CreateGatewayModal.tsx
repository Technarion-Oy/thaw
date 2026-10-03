// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { Form, Input, Checkbox, Alert, Typography } from "antd";
import { NodeIndexOutlined } from "@ant-design/icons";
import { BuildCreateGatewaySql, ExecDDL } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import CreateModalShell from "../shared/CreateModalShell";
import SqlPreview from "../shared/SqlPreview";
import { useQuotedIdentifiers, useSqlPreview, useCreateSubmit } from "../shared/createModalHooks";
import SpecEditor from "../service/SpecEditor";
import { specProblems } from "../service/specDoc";
import GatewaySpecForm from "./GatewaySpecForm";

const { Text } = Typography;

// A skeleton traffic-split specification: a single endpoint target taking 100%
// of the traffic, so the form opens with one row to pick the endpoint for.
const DEFAULT_SPEC = `spec:
  type: traffic_split
  split_type: custom
  targets:
    - type: endpoint
      weight: 100
`;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CreateGatewayModal({ db, schema, onClose, onSuccess }: Props) {
  const [name, setName] = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [orReplace, setOrReplace] = useState(false);
  const [ifNotExists, setIfNotExists] = useState(false);
  const [specification, setSpecification] = useState(DEFAULT_SPEC);

  const quotedIdentifiersIgnoreCase = useQuotedIdentifiers();
  const preview = useSqlPreview(
    () =>
      BuildCreateGatewaySql(db, schema, {
        name,
        caseSensitive,
        orReplace,
        ifNotExists,
        specification,
      } as any),
    [db, schema, name, caseSensitive, orReplace, ifNotExists, specification],
  );
  const { creating, error, setError, submit } = useCreateSubmit();

  const canSubmit = name.trim().length > 0 && specification.trim().length > 0
    && specProblems("gateway", specification).length === 0;

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
      icon={<NodeIndexOutlined />}
      title="Create Gateway"
      subtitle={`${db}.${schema}`}
      width={720}
      error={error}
      errorTitle="Gateway creation failed"
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
          message="A gateway fronts Snowpark Container Services endpoints: a traffic split routes ingress HTTP traffic across up to five service endpoints by weight (summing to 100); shadow traffic serves from one primary endpoint and mirrors a share of requests to shadow endpoints."
        />

        {/* OR REPLACE and IF NOT EXISTS are mutually exclusive in Snowflake;
            selecting one clears the other. */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Gateway name" required style={{ marginBottom: 4 }}>
            <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="MY_GATEWAY" />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={orReplace}
              onChange={(e) => { setOrReplace(e.target.checked); if (e.target.checked) setIfNotExists(false); }}
            >
              OR REPLACE
            </Checkbox>
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
            <Checkbox
              checked={ifNotExists}
              onChange={(e) => { setIfNotExists(e.target.checked); if (e.target.checked) setOrReplace(false); }}
            >
              IF NOT EXISTS
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={name}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Specification" required style={itemStyle} help="Sent via FROM SPECIFICATION $$ … $$.">
          <SpecEditor kind="gateway" value={specification} onChange={setSpecification} height={300}>
            <GatewaySpecForm defaultDb={db} defaultSchema={schema} />
          </SpecEditor>
        </Form.Item>

        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 8 }}>
          Each target is a fully-qualified endpoint <code>db.schema.service!endpoint</code> that must already exist. The specification can be changed later via the gateway’s Properties panel (ALTER GATEWAY … FROM SPECIFICATION).
        </Text>

        <SqlPreview sql={preview} />
      </Form>
    </CreateModalShell>
  );
}
