// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState } from "react";
import { Modal, Button, Space, Typography, Radio, Input, Alert, message } from "antd";
import { PlayCircleOutlined, RetweetOutlined } from "@ant-design/icons";
import { ExecuteTask } from "../../../wailsjs/go/main/App";

const { Text } = Typography;
const { TextArea } = Input;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
}

type Mode = "execute" | "retry";

function buildSql(db: string, schema: string, name: string, mode: Mode, config: string): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const ref = `"${esc(db)}"."${esc(schema)}"."${esc(name)}"`;
  if (mode === "retry") return `EXECUTE TASK ${ref} RETRY LAST;`;
  if (config.trim()) return `EXECUTE TASK ${ref}\n  USING CONFIG = $$${config.trim()}$$;`;
  return `EXECUTE TASK ${ref};`;
}

function isValidJson(s: string): boolean {
  try { JSON.parse(s); return true; } catch { return false; }
}

export default function ExecuteTaskModal({ db, schema, name, onClose }: Props) {
  const [mode, setMode]       = useState<Mode>("execute");
  const [config, setConfig]   = useState("");
  const [executing, setExecuting] = useState(false);

  const configTrimmed  = config.trim();
  const configInvalid  = mode === "execute" && configTrimmed !== "" && !isValidJson(configTrimmed);
  const canExecute     = !configInvalid;
  const preview        = buildSql(db, schema, name, mode, config);

  const handleExecute = async () => {
    if (!canExecute) return;
    setExecuting(true);
    try {
      await ExecuteTask(db, schema, name, mode === "execute" ? configTrimmed : "", mode === "retry");
      message.success(`Task ${name} triggered`);
      onClose();
    } catch (e) {
      message.error(String(e));
    } finally {
      setExecuting(false);
    }
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <PlayCircleOutlined style={{ color: "var(--link)" }} />
          <span>Execute task</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            type="primary"
            icon={mode === "retry" ? <RetweetOutlined /> : <PlayCircleOutlined />}
            loading={executing}
            disabled={!canExecute}
            onClick={handleExecute}
          >
            {mode === "retry" ? "Retry Last" : "Execute"}
          </Button>
        </Space>
      }
      width={520}
      styles={{ body: { paddingTop: 16 } }}
    >
      {/* Mode selector */}
      <div style={{ marginBottom: 16 }}>
        <Radio.Group value={mode} onChange={(e) => setMode(e.target.value as Mode)}>
          <Radio value="execute">Execute</Radio>
          <Radio value="retry">Retry Last</Radio>
        </Radio.Group>
      </div>

      {mode === "execute" ? (
        <div style={{ marginBottom: 16 }}>
          <Text style={{ fontSize: 13 }}>
            CONFIG{" "}
            <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
              (optional — valid JSON)
            </Text>
          </Text>
          <TextArea
            value={config}
            onChange={(e) => setConfig(e.target.value)}
            placeholder={'{"learning_rate": 0.2, "environment": "testing"}'}
            autoSize={{ minRows: 3, maxRows: 8 }}
            style={{ marginTop: 6, fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}
            status={configInvalid ? "error" : undefined}
          />
          {configInvalid && (
            <Alert
              type="error"
              message="Invalid JSON — fix the CONFIG string before executing"
              style={{ marginTop: 8 }}
              showIcon
            />
          )}
          <Text type="secondary" style={{ fontSize: 12, display: "block", marginTop: 6 }}>
            Merges with the task's default CONFIG at runtime. Leave blank to use the task's default configuration.
          </Text>
        </div>
      ) : (
        <div style={{
          padding: "10px 12px",
          borderRadius: 6,
          border: "1px solid var(--border)",
          background: "var(--bg)",
          marginBottom: 16,
        }}>
          <Text type="secondary" style={{ fontSize: 12 }}>
            Re-executes the last failed or cancelled task graph run from where it failed.
            The last run must be in state <Text code style={{ fontSize: 12 }}>FAILED</Text> or{" "}
            <Text code style={{ fontSize: 12 }}>CANCELED</Text>, the task graph must not have
            been modified since, and the first attempt must have run within the last 14 days.
          </Text>
        </div>
      )}

      {/* Live SQL preview */}
      <div style={{
        padding: "10px 12px",
        background: "var(--bg)",
        borderRadius: 6,
        border: "1px solid var(--border)",
      }}>
        <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>
          Preview
        </Text>
        <pre style={{
          margin: 0,
          color: "var(--text)",
          fontSize: 12,
          fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace",
          whiteSpace: "pre-wrap",
          wordBreak: "break-all",
        }}>
          {preview}
        </pre>
      </div>
    </Modal>
  );
}
