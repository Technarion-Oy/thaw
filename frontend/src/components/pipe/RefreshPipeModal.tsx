// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, DatePicker, Space, Typography, Button, Alert,
} from "antd";
import { SyncOutlined } from "@ant-design/icons";
import { BuildRefreshPipeSql, ExecDDL } from "../../../wailsjs/go/main/App";
import type { pipe } from "../../../wailsjs/go/models";
import dayjs from "dayjs";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

export default function RefreshPipeModal({ db, schema, name, onClose }: Props) {
  const [cfg, setCfg] = useState<pipe.RefreshPipeConfig>({
    prefix: "",
    modifiedAfter: "",
  });
  const [preview, setPreview] = useState("");
  const [running, setRunning] = useState(false);
  const [runError, setRunError] = useState<string | null>(null);
  const [success, setSuccess] = useState(false);

  useEffect(() => {
    BuildRefreshPipeSql(db, schema, name, cfg as any).then(setPreview).catch(() => {});
  }, [db, schema, name, cfg]);

  const set = <K extends keyof pipe.RefreshPipeConfig>(key: K, value: pipe.RefreshPipeConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const handleRun = async () => {
    setRunning(true);
    setRunError(null);
    setSuccess(false);
    try {
      await ExecDDL(preview);
      setSuccess(true);
    } catch (err) {
      setRunError(String(err));
    } finally {
      setRunning(false);
    }
  };

  const pipeRef = `"${db}"."${schema}"."${name}"`;

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <SyncOutlined style={{ color: "var(--link)" }} />
          <span>Refresh Pipe</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {pipeRef}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Close</Button>
          <Button
            type="primary"
            icon={<SyncOutlined />}
            onClick={handleRun}
            loading={running}
          >
            Refresh
          </Button>
        </Space>
      }
      width={520}
      styles={{ body: { paddingTop: 16 } }}
    >
      {runError && (
        <Alert
          type="error"
          message="Pipe refresh failed"
          description={runError}
          showIcon
          closable
          onClose={() => setRunError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      {success && (
        <Alert
          type="success"
          message="Pipe refresh initiated successfully"
          showIcon
          closable
          onClose={() => setSuccess(false)}
          style={{ marginBottom: 16 }}
        />
      )}

      <Form layout="vertical" size="small">
        <Form.Item label="Prefix" help="Limit refresh to files whose paths begin with this prefix (optional)">
          <Input
            value={cfg.prefix}
            onChange={(e) => set("prefix", e.target.value)}
            placeholder="optional/path/prefix"
          />
        </Form.Item>

        <Form.Item label="Modified After" help="Only include files modified after this time (optional; defaults to all files in stage)">
          <DatePicker
            showTime
            style={{ width: "100%" }}
            value={cfg.modifiedAfter ? dayjs(cfg.modifiedAfter) : null}
            onChange={(v) => set("modifiedAfter", v ? v.toISOString() : "")}
          />
        </Form.Item>

        <div
          style={{
            padding: "10px 12px",
            background: "var(--bg)",
            borderRadius: 6,
            border: "1px solid var(--border)",
            marginTop: 4,
          }}
        >
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
            SQL Preview
          </Text>
          <pre
            style={{
              margin: 0,
              color: "var(--text)",
              fontSize: 11,
              fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
              whiteSpace: "pre-wrap",
              wordBreak: "break-all",
            }}
          >
            {preview}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
