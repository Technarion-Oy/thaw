// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect, useState } from "react";
import { Modal, Button, Spin, Space, Typography, Tag } from "antd";
import {
  CheckCircleOutlined,
  CloseCircleOutlined,
  ReloadOutlined,
} from "@ant-design/icons";
import { CheckSnowparkEnv } from "../../../wailsjs/go/main/App";
import type { main } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  onClose: () => void;
  onSetup: () => void;
}

type CheckResult = main.SnowparkCheckResult;

interface CheckRow {
  label: string;
  ok: boolean | null;
}

export default function SnowparkCheckModal({ onClose, onSetup }: Props) {
  const [loading, setLoading] = useState(true);
  const [result, setResult] = useState<CheckResult | null>(null);

  const run = async () => {
    setLoading(true);
    try {
      const r = await CheckSnowparkEnv();
      setResult(r);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { run(); }, []);

  const rows: CheckRow[] = result
    ? [
        {
          label: `Python${result.systemPythonVersion ? " " + result.systemPythonVersion : ""} (system)`,
          ok: result.systemPythonVersion !== "",
        },
        ...(result.backend !== "venv"
          ? [
              { label: "conda",               ok: result.hasConda },
              { label: "env 'thaw_snowpark'", ok: result.hasEnv },
              {
                label: `Python${result.version ? " " + result.version : ""} (conda env)`,
                ok: result.hasEnv && result.version !== "",
              },
            ]
          : [
              {
                label: `venv${result.venvPath ? "  " + result.venvPath : ""}`,
                ok: result.hasVenv,
              },
              {
                label: `Python${result.version ? " " + result.version : ""} (venv)`,
                ok: result.hasVenv && result.version !== "",
              },
            ]),
        { label: "snowflake-snowpark-python", ok: result.hasSnowpark },
        { label: "notebook",                  ok: result.hasNotebook },
      ]
    : [];

  return (
    <Modal
      title="Snowpark Environment Check"
      open
      onCancel={onClose}
      width={440}
      footer={[
        <Button key="refresh" icon={<ReloadOutlined />} onClick={run} loading={loading}>
          Re-check
        </Button>,
        ...(!result?.isReady
          ? [<Button key="setup" type="primary" onClick={() => { onClose(); onSetup(); }}>
              Setup Environment…
            </Button>]
          : []),
        <Button key="close" onClick={onClose}>Close</Button>,
      ]}
    >
      {loading && !result ? (
        <div style={{ textAlign: "center", padding: "24px 0" }}>
          <Spin tip="Checking environment…" />
        </div>
      ) : result ? (
        <Space direction="vertical" style={{ width: "100%" }} size={12}>
          <Space direction="vertical" size={6} style={{ width: "100%" }}>
            {rows.map((row) => (
              <div key={row.label} style={{ display: "flex", alignItems: "center", gap: 8 }}>
                {row.ok
                  ? <CheckCircleOutlined style={{ color: "#52c41a", fontSize: 15 }} />
                  : <CloseCircleOutlined style={{ color: "#ff4d4f", fontSize: 15 }} />
                }
                <Text style={{ fontSize: 13 }}>{row.label}</Text>
              </div>
            ))}
          </Space>

          <div style={{ marginTop: 4 }}>
            {result.isReady ? (
              <Tag color="success" style={{ fontSize: 12 }}>
                {result.details}
              </Tag>
            ) : (
              <Text type="secondary" style={{ fontSize: 12 }}>
                {result.details}
              </Text>
            )}
          </div>

          {result.isReady && result.pythonPath && (
            <Text type="secondary" style={{ fontSize: 11, fontFamily: "monospace" }}>
              {result.pythonPath}
            </Text>
          )}
        </Space>
      ) : null}
    </Modal>
  );
}
