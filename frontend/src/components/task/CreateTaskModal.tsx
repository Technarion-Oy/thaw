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
  Modal, Form, Input, Select, Checkbox, Radio, Space,
  Typography, Divider, InputNumber, Button,
} from "antd";
import { ClockCircleOutlined } from "@ant-design/icons";
import { ListWarehouses, ListNotificationIntegrations } from "../../../wailsjs/go/main/App";
import { useQueryStore } from "../../store/queryStore";

const { Text } = Typography;
const { TextArea } = Input;

const SERVERLESS_SIZES = ["XSMALL", "SMALL", "MEDIUM", "LARGE", "XLARGE", "XXLARGE"];
const NO_INTEGRATION = "";

interface TaskConfig {
  name: string;
  computeType: "warehouse" | "serverless";
  warehouse: string;
  serverlessSize: string;
  scheduleType: "none" | "interval" | "cron";
  intervalNum: string;
  intervalUnit: "HOURS" | "MINUTES" | "SECONDS";
  cronExpr: string;
  cronTimezone: string;
  after: string;
  when: string;
  allowOverlapping: boolean;
  timeoutMs: string;
  suspendAfterFailures: string;
  autoRetryAttempts: string;
  errorIntegration: string;
  successIntegration: string;
  comment: string;
  finalize: string;
  sql: string;
}

const DEFAULTS: TaskConfig = {
  name: "",
  computeType: "warehouse",
  warehouse: "",
  serverlessSize: "SMALL",
  scheduleType: "none",
  intervalNum: "",
  intervalUnit: "MINUTES",
  cronExpr: "",
  cronTimezone: "UTC",
  after: "",
  when: "",
  allowOverlapping: false,
  timeoutMs: "",
  suspendAfterFailures: "",
  autoRetryAttempts: "",
  errorIntegration: NO_INTEGRATION,
  successIntegration: NO_INTEGRATION,
  comment: "",
  finalize: "",
  sql: "",
};

function buildSql(db: string, schema: string, cfg: TaskConfig): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const lines: string[] = [
    `CREATE OR REPLACE TASK "${esc(db)}"."${esc(schema)}"."${esc(cfg.name || "task_name")}"`,
  ];

  if (cfg.computeType === "warehouse" && cfg.warehouse.trim()) {
    lines.push(`    WAREHOUSE = ${cfg.warehouse.trim()}`);
  } else if (cfg.computeType === "serverless") {
    lines.push(`    USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = '${cfg.serverlessSize}'`);
  }

  if (cfg.scheduleType === "interval" && cfg.intervalNum.trim()) {
    lines.push(`    SCHEDULE = '${cfg.intervalNum.trim()} ${cfg.intervalUnit}'`);
  } else if (cfg.scheduleType === "cron" && cfg.cronExpr.trim()) {
    lines.push(`    SCHEDULE = 'USING CRON ${cfg.cronExpr.trim()} ${cfg.cronTimezone.trim() || "UTC"}'`);
  }

  if (cfg.allowOverlapping) {
    lines.push(`    ALLOW_OVERLAPPING_EXECUTION = TRUE`);
  }
  if (cfg.timeoutMs.trim()) {
    lines.push(`    USER_TASK_TIMEOUT_MS = ${cfg.timeoutMs.trim()}`);
  }
  if (cfg.suspendAfterFailures.trim()) {
    lines.push(`    SUSPEND_TASK_AFTER_NUM_FAILURES = ${cfg.suspendAfterFailures.trim()}`);
  }
  if (cfg.autoRetryAttempts.trim()) {
    lines.push(`    TASK_AUTO_RETRY_ATTEMPTS = ${cfg.autoRetryAttempts.trim()}`);
  }
  if (cfg.errorIntegration) {
    lines.push(`    ERROR_INTEGRATION = ${cfg.errorIntegration}`);
  }
  if (cfg.successIntegration) {
    lines.push(`    SUCCESS_INTEGRATION = ${cfg.successIntegration}`);
  }
  if (cfg.comment.trim()) {
    lines.push(`    COMMENT = '${cfg.comment.trim().replace(/'/g, "''")}'`);
  }
  if (cfg.finalize.trim()) {
    lines.push(`    FINALIZE = ${cfg.finalize.trim()}`);
  }

  const afterTasks = cfg.after.split(",").map((s) => s.trim()).filter(Boolean);
  if (afterTasks.length > 0) {
    lines.push(`AFTER ${afterTasks.join(", ")}`);
  }
  if (cfg.when.trim()) {
    lines.push(`WHEN ${cfg.when.trim()}`);
  }

  lines.push(`AS`);
  lines.push(cfg.sql.trim() || "-- your SQL here");

  return lines.join("\n") + ";";
}

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
}

export default function CreateTaskModal({ db, schema, onClose }: Props) {
  const [cfg, setCfg] = useState<TaskConfig>(DEFAULTS);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [integrations, setIntegrations] = useState<string[]>([]);
  const executeInNewTab = useQueryStore((s) => s.executeInNewTab);

  useEffect(() => {
    ListWarehouses().then((whs) => setWarehouses(whs ?? [])).catch(() => {});
    ListNotificationIntegrations().then((ints) => setIntegrations(ints ?? [])).catch(() => {});
  }, []);

  const set = <K extends keyof TaskConfig>(key: K, value: TaskConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const canSubmit = cfg.name.trim() !== "" && cfg.sql.trim() !== "";

  const handleRun = () => {
    const sql = buildSql(db, schema, cfg);
    onClose();
    executeInNewTab(sql);
  };

  const preview = buildSql(db, schema, cfg);

  const labelStyle: React.CSSProperties = { fontSize: 12, fontWeight: 600, color: "var(--text-muted)", marginBottom: 4 };
  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const integrationOptions = [
    { value: NO_INTEGRATION, label: "— None —" },
    ...integrations.map((i) => ({ value: i, label: i })),
  ];

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ClockCircleOutlined style={{ color: "var(--link)" }} />
          <span>Create task</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" icon={<ClockCircleOutlined />} onClick={handleRun} disabled={!canSubmit}>
            Create
          </Button>
        </Space>
      }
      width={700}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      <Form layout="vertical" size="small">

        {/* Task name */}
        <Form.Item label="Task name" required style={itemStyle}>
          <Input
            value={cfg.name}
            onChange={(e) => set("name", e.target.value)}
            placeholder="MY_TASK"
          />
        </Form.Item>

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 12px" }}>
          Compute
        </Divider>

        <Form.Item style={itemStyle}>
          <Radio.Group
            value={cfg.computeType}
            onChange={(e) => set("computeType", e.target.value)}
            size="small"
          >
            <Radio value="warehouse">Warehouse</Radio>
            <Radio value="serverless">Serverless (managed)</Radio>
          </Radio.Group>
        </Form.Item>

        {cfg.computeType === "warehouse" ? (
          <Form.Item label="Warehouse" style={itemStyle}>
            <Select
              value={cfg.warehouse || undefined}
              onChange={(v) => set("warehouse", v ?? "")}
              placeholder="Select warehouse"
              showSearch
              allowClear
              options={warehouses.map((w) => ({ value: w, label: w }))}
              style={{ width: "100%" }}
            />
          </Form.Item>
        ) : (
          <Form.Item label="Initial warehouse size" style={itemStyle}>
            <Select
              value={cfg.serverlessSize}
              onChange={(v) => set("serverlessSize", v)}
              options={SERVERLESS_SIZES.map((s) => ({ value: s, label: s }))}
              style={{ width: 160 }}
            />
          </Form.Item>
        )}

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 12px" }}>
          Schedule
        </Divider>

        <Form.Item style={itemStyle}>
          <Radio.Group
            value={cfg.scheduleType}
            onChange={(e) => set("scheduleType", e.target.value)}
            size="small"
          >
            <Radio value="none">None (triggered / dependent)</Radio>
            <Radio value="interval">Interval</Radio>
            <Radio value="cron">Cron</Radio>
          </Radio.Group>
        </Form.Item>

        {cfg.scheduleType === "interval" && (
          <Form.Item label="Interval" style={itemStyle}>
            <Space>
              <InputNumber
                value={cfg.intervalNum === "" ? undefined : Number(cfg.intervalNum)}
                onChange={(v) => set("intervalNum", v === null ? "" : String(v))}
                min={1}
                placeholder="5"
                style={{ width: 90 }}
              />
              <Select
                value={cfg.intervalUnit}
                onChange={(v) => set("intervalUnit", v)}
                options={[
                  { value: "SECONDS", label: "Seconds" },
                  { value: "MINUTES", label: "Minutes" },
                  { value: "HOURS",   label: "Hours" },
                ]}
                style={{ width: 110 }}
              />
            </Space>
          </Form.Item>
        )}

        {cfg.scheduleType === "cron" && (
          <Form.Item label="Cron expression &amp; timezone" style={itemStyle}>
            <Space>
              <Input
                value={cfg.cronExpr}
                onChange={(e) => set("cronExpr", e.target.value)}
                placeholder="0 9 * * *"
                style={{ width: 200 }}
              />
              <Input
                value={cfg.cronTimezone}
                onChange={(e) => set("cronTimezone", e.target.value)}
                placeholder="UTC"
                style={{ width: 120 }}
              />
            </Space>
          </Form.Item>
        )}

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 12px" }}>
          Dependencies
        </Divider>

        <Form.Item
          label={<span style={labelStyle}>Predecessor tasks (AFTER)</span>}
          style={itemStyle}
          help={<span style={{ fontSize: 11 }}>Comma-separated fully-qualified task names</span>}
        >
          <Input
            value={cfg.after}
            onChange={(e) => set("after", e.target.value)}
            placeholder={`"${db}"."${schema}"."PARENT_TASK"`}
          />
        </Form.Item>

        <Form.Item
          label={<span style={labelStyle}>Condition (WHEN)</span>}
          style={itemStyle}
        >
          <Input
            value={cfg.when}
            onChange={(e) => set("when", e.target.value)}
            placeholder="SYSTEM$STREAM_HAS_DATA('MY_STREAM')"
          />
        </Form.Item>

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 12px" }}>
          Execution
        </Divider>

        <Form.Item style={{ marginBottom: 12 }}>
          <Checkbox
            checked={cfg.allowOverlapping}
            onChange={(e) => set("allowOverlapping", e.target.checked)}
          >
            Allow overlapping execution
          </Checkbox>
        </Form.Item>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Timeout (ms)" style={itemStyle}>
            <InputNumber
              value={cfg.timeoutMs === "" ? undefined : Number(cfg.timeoutMs)}
              onChange={(v) => set("timeoutMs", v === null ? "" : String(v))}
              min={0}
              placeholder="3600000"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Suspend after N failures" style={itemStyle}>
            <InputNumber
              value={cfg.suspendAfterFailures === "" ? undefined : Number(cfg.suspendAfterFailures)}
              onChange={(v) => set("suspendAfterFailures", v === null ? "" : String(v))}
              min={0}
              placeholder="10"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Auto-retry attempts" style={itemStyle}>
            <InputNumber
              value={cfg.autoRetryAttempts === "" ? undefined : Number(cfg.autoRetryAttempts)}
              onChange={(v) => set("autoRetryAttempts", v === null ? "" : String(v))}
              min={0}
              placeholder="0"
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Error integration" style={itemStyle}>
            <Select
              value={cfg.errorIntegration}
              onChange={(v) => set("errorIntegration", v ?? NO_INTEGRATION)}
              showSearch
              options={integrationOptions}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Success integration" style={itemStyle}>
            <Select
              value={cfg.successIntegration}
              onChange={(v) => set("successIntegration", v ?? NO_INTEGRATION)}
              showSearch
              options={integrationOptions}
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 12px" }}>
          Other
        </Divider>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Comment" style={itemStyle}>
            <Input
              value={cfg.comment}
              onChange={(e) => set("comment", e.target.value)}
              placeholder="optional comment"
            />
          </Form.Item>
          <Form.Item
            label="Finalize task"
            style={itemStyle}
            help={<span style={{ fontSize: 11 }}>Runs after all tasks in the DAG complete</span>}
          >
            <Input
              value={cfg.finalize}
              onChange={(e) => set("finalize", e.target.value)}
              placeholder="FINALIZE_TASK_NAME"
            />
          </Form.Item>
        </div>

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 12px" }}>
          SQL (AS)
        </Divider>

        <Form.Item required style={itemStyle}>
          <TextArea
            value={cfg.sql}
            onChange={(e) => set("sql", e.target.value)}
            placeholder="INSERT INTO my_table SELECT * FROM my_stream;"
            autoSize={{ minRows: 3, maxRows: 8 }}
            style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}
          />
        </Form.Item>

        {/* Live preview */}
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
            Preview
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
