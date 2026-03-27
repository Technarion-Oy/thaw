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
  Typography, Divider, InputNumber, Button, Tag, Alert,
} from "antd";
import { ClockCircleOutlined, PlusOutlined, InfoCircleOutlined } from "@ant-design/icons";
import { ListWarehouses, ListNotificationIntegrations, ListObjects, ExecDDL } from "../../../wailsjs/go/main/App";
import ScheduleEditor from "./ScheduleEditor";
import WhenConditionBuilder from "./WhenConditionBuilder";

const { Text } = Typography;
const { TextArea } = Input;

const SERVERLESS_SIZES = ["XSMALL", "SMALL", "MEDIUM", "LARGE", "XLARGE", "XXLARGE"];
const LOG_LEVELS = ["TRACE", "DEBUG", "INFO", "WARN", "ERROR", "FATAL", "OFF"];
const NO_INTEGRATION = "";

interface TaskConfig {
  name: string;
  orReplace: boolean;
  ifNotExists: boolean;
  computeType: "warehouse" | "serverless";
  warehouse: string;
  serverlessSize: string;
  serverlessMinSize: string;
  serverlessMaxSize: string;
  schedule: string;
  config: string;
  overlapPolicy: "" | "NO_OVERLAP" | "ALLOW_CHILD_OVERLAP" | "ALLOW_ALL_OVERLAP";
  timeoutMs: string;
  suspendAfterFailures: string;
  autoRetryAttempts: string;
  minTriggerIntervalSecs: string;
  targetCompletionNum: string;
  targetCompletionUnit: "HOURS" | "MINUTES" | "SECONDS";
  errorIntegration: string;
  successIntegration: string;
  logLevel: string;
  comment: string;
  finalize: string;
  executeAsType: "default" | "caller" | "user";
  executeAsUser: string;
  after: string[];
  when: string;
  sql: string;
}

const DEFAULTS: TaskConfig = {
  name: "",
  orReplace: false,
  ifNotExists: false,
  computeType: "warehouse",
  warehouse: "",
  serverlessSize: "SMALL",
  serverlessMinSize: "",
  serverlessMaxSize: "",
  schedule: "",
  config: "",
  overlapPolicy: "",
  timeoutMs: "",
  suspendAfterFailures: "",
  autoRetryAttempts: "",
  minTriggerIntervalSecs: "",
  targetCompletionNum: "",
  targetCompletionUnit: "MINUTES",
  errorIntegration: NO_INTEGRATION,
  successIntegration: NO_INTEGRATION,
  logLevel: "",
  comment: "",
  finalize: "",
  executeAsType: "default",
  executeAsUser: "",
  after: [],
  when: "",
  sql: "",
};

function buildSql(db: string, schema: string, cfg: TaskConfig): string {
  const esc = (s: string) => s.replace(/"/g, '""');

  let createClause = "CREATE";
  if (cfg.orReplace) createClause += " OR REPLACE";
  createClause += " TASK";
  if (cfg.ifNotExists && !cfg.orReplace) createClause += " IF NOT EXISTS";

  const lines: string[] = [
    `${createClause} "${esc(db)}"."${esc(schema)}"."${esc(cfg.name || "task_name")}"`,
  ];

  // Compute
  if (cfg.computeType === "warehouse" && cfg.warehouse.trim()) {
    lines.push(`    WAREHOUSE = ${cfg.warehouse.trim()}`);
  } else if (cfg.computeType === "serverless") {
    lines.push(`    USER_TASK_MANAGED_INITIAL_WAREHOUSE_SIZE = '${cfg.serverlessSize}'`);
    if (cfg.serverlessMinSize) lines.push(`    SERVERLESS_TASK_MIN_STATEMENT_SIZE = '${cfg.serverlessMinSize}'`);
    if (cfg.serverlessMaxSize) lines.push(`    SERVERLESS_TASK_MAX_STATEMENT_SIZE = '${cfg.serverlessMaxSize}'`);
  }

  // Schedule
  if (cfg.schedule.trim()) {
    lines.push(`    SCHEDULE = '${cfg.schedule.trim()}'`);
  }

  // Config
  if (cfg.config.trim()) {
    lines.push(`    CONFIG = $$${cfg.config.trim()}$$`);
  }

  // Overlap policy
  if (cfg.overlapPolicy) {
    lines.push(`    OVERLAP_POLICY = ${cfg.overlapPolicy}`);
  }

  // Limits
  if (cfg.timeoutMs.trim()) lines.push(`    USER_TASK_TIMEOUT_MS = ${cfg.timeoutMs.trim()}`);
  if (cfg.suspendAfterFailures.trim()) lines.push(`    SUSPEND_TASK_AFTER_NUM_FAILURES = ${cfg.suspendAfterFailures.trim()}`);
  if (cfg.autoRetryAttempts.trim()) lines.push(`    TASK_AUTO_RETRY_ATTEMPTS = ${cfg.autoRetryAttempts.trim()}`);
  if (cfg.minTriggerIntervalSecs.trim()) lines.push(`    USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS = ${cfg.minTriggerIntervalSecs.trim()}`);
  if (cfg.targetCompletionNum.trim()) lines.push(`    TARGET_COMPLETION_INTERVAL = '${cfg.targetCompletionNum.trim()} ${cfg.targetCompletionUnit}'`);

  // Notifications
  if (cfg.errorIntegration) lines.push(`    ERROR_INTEGRATION = ${cfg.errorIntegration}`);
  if (cfg.successIntegration) lines.push(`    SUCCESS_INTEGRATION = ${cfg.successIntegration}`);

  // Other
  if (cfg.logLevel) lines.push(`    LOG_LEVEL = '${cfg.logLevel}'`);
  if (cfg.comment.trim()) lines.push(`    COMMENT = '${cfg.comment.trim().replace(/'/g, "''")}'`);
  if (cfg.finalize.trim()) lines.push(`    FINALIZE = "${esc(cfg.finalize.trim())}"`);

  // AFTER — each entry is a bare task name in this db/schema; emit fully-qualified
  if (cfg.after.length > 0) {
    const qn = (s: string) => `"${s.replace(/"/g, '""')}"`;
    lines.push(`AFTER ${cfg.after.map((n) => `${qn(db)}.${qn(schema)}.${qn(n)}`).join(", ")}`);
  }

  // EXECUTE AS
  if (cfg.executeAsType === "caller") {
    lines.push(`EXECUTE AS CALLER`);
  } else if (cfg.executeAsType === "user" && cfg.executeAsUser.trim()) {
    lines.push(`EXECUTE AS USER ${cfg.executeAsUser.trim()}`);
  }

  // WHEN
  if (cfg.when.trim()) lines.push(`WHEN ${cfg.when.trim()}`);

  lines.push(`AS`);
  lines.push(cfg.sql.trim() || "-- your SQL here");

  return lines.join("\n") + ";";
}

/** Determines how the dialog is pre-configured when opened from the task graph. */
export type CreateTaskMode = "create" | "child" | "finalizer";

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  /** Called after the CREATE TASK statement executes successfully. */
  onSuccess?: () => void;
  /** Context for the dialog. Defaults to "create" (no pre-population). */
  mode?: CreateTaskMode;
  /** "child" mode: this task name is pre-added to the AFTER (predecessor) list. */
  predecessorTask?: string;
  /** "finalizer" mode: the FINALIZE clause is pre-filled with this root task name. */
  finalizerForTask?: string;
}

export default function CreateTaskModal({
  db, schema, onClose, onSuccess,
  mode = "create", predecessorTask, finalizerForTask,
}: Props) {
  const [cfg, setCfg] = useState<TaskConfig>(() => {
    if (mode === "child" && predecessorTask) return { ...DEFAULTS, after: [predecessorTask] };
    if (mode === "finalizer" && finalizerForTask) return { ...DEFAULTS, finalize: finalizerForTask };
    return { ...DEFAULTS };
  });
  const [warehouses,    setWarehouses]    = useState<string[]>([]);
  const [integrations,  setIntegrations]  = useState<string[]>([]);
  const [availableTasks, setAvailableTasks] = useState<string[]>([]);
  const [taskSearchVal, setTaskSearchVal] = useState<string | undefined>(undefined);
  const [creating,        setCreating]        = useState(false);
  const [createError,     setCreateError]     = useState<string | null>(null);
  // Show warehouse validation error only after the user has touched the field
  // or attempted to submit — avoids red border on fresh dialog open.
  const [warehouseTouched, setWarehouseTouched] = useState(false);

  useEffect(() => {
    ListWarehouses().then((whs) => setWarehouses(whs ?? [])).catch(() => {});
    ListNotificationIntegrations().then((ints) => setIntegrations(ints ?? [])).catch(() => {});
    ListObjects(db, schema)
      .then((objs) => {
        const tasks = (objs ?? [])
          .filter((o) => (o.kind || "").toUpperCase() === "TASK")
          .map((o) => o.name);
        setAvailableTasks(tasks);
      })
      .catch(() => {});
  }, [db, schema]);

  const set = <K extends keyof TaskConfig>(key: K, value: TaskConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  // Warehouse is mandatory when using the warehouse compute type.
  const warehouseMissing = cfg.computeType === "warehouse" && cfg.warehouse.trim() === "";
  const canSubmit = cfg.name.trim() !== "" && cfg.sql.trim() !== "" && !warehouseMissing;

  const handleRun = async () => {
    // Mark warehouse as touched so validation UI appears if it's missing.
    setWarehouseTouched(true);
    if (!canSubmit) return;
    const sql = buildSql(db, schema, cfg);
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const preview = buildSql(db, schema, cfg);

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  const integrationOptions = [
    { value: NO_INTEGRATION, label: "— None —" },
    ...integrations.map((i) => ({ value: i, label: i })),
  ];

  const divider = (label: string) => (
    <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 12px" }}>
      {label}
    </Divider>
  );

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <ClockCircleOutlined style={{ color: "var(--link)" }} />
          <span>
            {mode === "child" ? "Create child task" :
             mode === "finalizer" ? "Create finalizer task" :
             "Create task"}
          </span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={creating}>Cancel</Button>
          <Button
            type="primary"
            icon={<ClockCircleOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={700}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Task creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form layout="vertical" size="small">

        {/* Task name + create options */}
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Task name" required style={itemStyle}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_TASK"
            />
          </Form.Item>
          <Form.Item style={itemStyle}>
            <Space direction="vertical" size={4}>
              <Checkbox
                checked={cfg.orReplace}
                onChange={(e) => {
                  set("orReplace", e.target.checked);
                  if (e.target.checked) set("ifNotExists", false);
                }}
              >
                OR REPLACE
              </Checkbox>
              <Checkbox
                checked={cfg.ifNotExists}
                disabled={cfg.orReplace}
                onChange={(e) => set("ifNotExists", e.target.checked)}
              >
                IF NOT EXISTS
              </Checkbox>
            </Space>
          </Form.Item>
        </div>

        {divider("Compute")}

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
          <Form.Item
            label="Warehouse"
            required
            style={itemStyle}
            validateStatus={warehouseMissing && warehouseTouched ? "error" : ""}
            help={warehouseMissing && warehouseTouched ? "A warehouse is required" : undefined}
          >
            <Select
              value={cfg.warehouse || undefined}
              onChange={(v) => { set("warehouse", v ?? ""); setWarehouseTouched(true); }}
              onClear={() => setWarehouseTouched(true)}
              placeholder="Select warehouse"
              showSearch
              allowClear
              options={warehouses.map((w) => ({ value: w, label: w }))}
              style={{ width: "100%" }}
              status={warehouseMissing && warehouseTouched ? "error" : undefined}
            />
          </Form.Item>
        ) : (
          <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr 1fr", gap: "0 16px" }}>
            <Form.Item label="Initial size" style={itemStyle}>
              <Select
                value={cfg.serverlessSize}
                onChange={(v) => set("serverlessSize", v)}
                options={SERVERLESS_SIZES.map((s) => ({ value: s, label: s }))}
                style={{ width: "100%" }}
              />
            </Form.Item>
            <Form.Item label="Min statement size" style={itemStyle}>
              <Select
                value={cfg.serverlessMinSize || undefined}
                onChange={(v) => set("serverlessMinSize", v ?? "")}
                allowClear
                placeholder="Default"
                options={SERVERLESS_SIZES.map((s) => ({ value: s, label: s }))}
                style={{ width: "100%" }}
              />
            </Form.Item>
            <Form.Item label="Max statement size" style={itemStyle}>
              <Select
                value={cfg.serverlessMaxSize || undefined}
                onChange={(v) => set("serverlessMaxSize", v ?? "")}
                allowClear
                placeholder="Default"
                options={SERVERLESS_SIZES.map((s) => ({ value: s, label: s }))}
                style={{ width: "100%" }}
              />
            </Form.Item>
          </div>
        )}

        {divider("Schedule")}

        {mode === "create" ? (
          <Form.Item style={itemStyle}>
            <ScheduleEditor value={cfg.schedule} onChange={(v) => set("schedule", v)} />
          </Form.Item>
        ) : (
          <Form.Item style={itemStyle}>
            <div style={{
              display: "flex", alignItems: "flex-start", gap: 8, fontSize: 12,
              color: "var(--text-secondary, #888)",
              background: "var(--bg)", border: "1px solid var(--border)",
              borderRadius: 6, padding: "8px 12px",
            }}>
              <InfoCircleOutlined style={{ marginTop: 2, flexShrink: 0 }} />
              <span>
                {mode === "child"
                  ? "Child tasks cannot have their own schedule — they are triggered automatically when their predecessor(s) complete successfully."
                  : "Finalizer tasks have no schedule — they run when the root task's DAG completes, including after failures or cancellations."}
              </span>
            </div>
          </Form.Item>
        )}

        {divider("Configuration")}

        <Form.Item
          label="CONFIG"
          style={itemStyle}
          help={<span style={{ fontSize: 11 }}>JSON string passed to the task at runtime (dollar-quoted)</span>}
        >
          <TextArea
            value={cfg.config}
            onChange={(e) => set("config", e.target.value)}
            placeholder={'{"learning_rate": 0.2, "environment": "production"}'}
            autoSize={{ minRows: 2, maxRows: 5 }}
            style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}
          />
        </Form.Item>

        {divider("Dependencies")}

        {/* Finalizer tasks use FINALIZE (not AFTER) — show it prominently here */}
        {mode === "finalizer" && (
          <Form.Item
            label="Finalizes root task (FINALIZE)"
            style={itemStyle}
            help="This task runs after the named root task's DAG completes — even if upstream tasks fail or are cancelled"
          >
            <Input value={cfg.finalize} disabled />
          </Form.Item>
        )}

        {/* AFTER / predecessors — not applicable for finalizer tasks */}
        {mode !== "finalizer" && (
          <Form.Item label="Predecessor tasks (AFTER)" style={itemStyle}>
            {mode === "child" && predecessorTask && (
              <div style={{
                display: "flex", alignItems: "center", gap: 6, fontSize: 11,
                color: "var(--text-secondary, #888)", marginBottom: 6,
              }}>
                <InfoCircleOutlined />
                <span>
                  <code style={{ fontFamily: "monospace" }}>{predecessorTask}</code> is pre-selected.
                  Add more predecessors if needed (max 100 per task).
                </span>
              </div>
            )}
            <Space.Compact style={{ width: "100%" }}>
              <Select
                showSearch
                value={taskSearchVal}
                onChange={(v) => setTaskSearchVal(v)}
                onClear={() => setTaskSearchVal(undefined)}
                placeholder="Search tasks…"
                allowClear
                style={{ flex: 1 }}
                filterOption={(input, option) =>
                  (option?.value as string ?? "").toLowerCase().includes(input.toLowerCase())
                }
                options={availableTasks
                  .filter((t) => !cfg.after.includes(t))
                  .map((t) => ({ value: t, label: t }))}
                notFoundContent={
                  <span style={{ fontSize: 12, color: "var(--text-muted)" }}>No tasks found</span>
                }
              />
              <Button
                icon={<PlusOutlined />}
                disabled={!taskSearchVal}
                onClick={() => {
                  if (!taskSearchVal) return;
                  set("after", [...cfg.after, taskSearchVal]);
                  setTaskSearchVal(undefined);
                }}
              />
            </Space.Compact>
            {cfg.after.length > 0 && (
              <div style={{ marginTop: 8, display: "flex", flexWrap: "wrap", gap: 4 }}>
                {cfg.after.map((t) => (
                  <Tag
                    key={t}
                    closable
                    onClose={() => set("after", cfg.after.filter((x) => x !== t))}
                    style={{ fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", fontSize: 12 }}
                  >
                    {t}
                  </Tag>
                ))}
              </div>
            )}
          </Form.Item>
        )}

        {/* WHEN condition — finalizer tasks always run; condition not applicable */}
        {mode !== "finalizer" && (
          <Form.Item label="Condition (WHEN)" style={itemStyle}>
            <WhenConditionBuilder
              db={db} schema={schema}
              value={cfg.when}
              onChange={(v) => set("when", v)}
            />
          </Form.Item>
        )}

        {divider("Execution")}

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Overlap policy" style={itemStyle}>
            <Select
              value={cfg.overlapPolicy || undefined}
              onChange={(v) => set("overlapPolicy", v ?? "")}
              allowClear
              placeholder="Default"
              options={[
                { value: "NO_OVERLAP",          label: "NO_OVERLAP" },
                { value: "ALLOW_CHILD_OVERLAP",  label: "ALLOW_CHILD_OVERLAP" },
                { value: "ALLOW_ALL_OVERLAP",    label: "ALLOW_ALL_OVERLAP" },
              ]}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Execute as" style={itemStyle}>
            <Radio.Group
              value={cfg.executeAsType}
              onChange={(e) => set("executeAsType", e.target.value)}
              size="small"
            >
              <Radio value="default">Default</Radio>
              <Radio value="caller">Caller</Radio>
              <Radio value="user">User</Radio>
            </Radio.Group>
            {cfg.executeAsType === "user" && (
              <Input
                value={cfg.executeAsUser}
                onChange={(e) => set("executeAsUser", e.target.value)}
                placeholder="USERNAME"
                style={{ marginTop: 6 }}
              />
            )}
          </Form.Item>
        </div>

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
          <Form.Item
            label="Min trigger interval (s)"
            style={itemStyle}
            help={<span style={{ fontSize: 11 }}>USER_TASK_MINIMUM_TRIGGER_INTERVAL_IN_SECONDS</span>}
          >
            <InputNumber
              value={cfg.minTriggerIntervalSecs === "" ? undefined : Number(cfg.minTriggerIntervalSecs)}
              onChange={(v) => set("minTriggerIntervalSecs", v === null ? "" : String(v))}
              min={0}
              placeholder="30"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Target completion interval" style={itemStyle}>
            <Space>
              <InputNumber
                value={cfg.targetCompletionNum === "" ? undefined : Number(cfg.targetCompletionNum)}
                onChange={(v) => set("targetCompletionNum", v === null ? "" : String(v))}
                min={1}
                placeholder="—"
                style={{ width: 80 }}
              />
              <Select
                value={cfg.targetCompletionUnit}
                onChange={(v) => set("targetCompletionUnit", v)}
                options={[
                  { value: "SECONDS", label: "Seconds" },
                  { value: "MINUTES", label: "Minutes" },
                  { value: "HOURS",   label: "Hours" },
                ]}
                style={{ width: 110 }}
              />
            </Space>
          </Form.Item>
        </div>

        {divider("Notifications")}

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

        {divider("Other")}

        <div style={{
          display: "grid",
          gridTemplateColumns: mode === "create" ? "1fr 1fr 1fr" : "1fr 1fr",
          gap: "0 16px",
        }}>
          <Form.Item label="Log level" style={itemStyle}>
            <Select
              value={cfg.logLevel || undefined}
              onChange={(v) => set("logLevel", v ?? "")}
              allowClear
              placeholder="Default"
              options={LOG_LEVELS.map((l) => ({ value: l, label: l }))}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Comment" style={itemStyle}>
            <Input
              value={cfg.comment}
              onChange={(e) => set("comment", e.target.value)}
              placeholder="optional comment"
            />
          </Form.Item>
          {/* Finalize field: only relevant for standalone root tasks in create mode.
              Finalizer mode shows it in the Dependencies section; child mode hides it. */}
          {mode === "create" && (
            <Form.Item
              label="Finalize task"
              style={itemStyle}
              help={<span style={{ fontSize: 11 }}>Runs after the full DAG completes</span>}
            >
              <Input
                value={cfg.finalize}
                onChange={(e) => set("finalize", e.target.value)}
                placeholder="FINALIZE_TASK_NAME"
              />
            </Form.Item>
          )}
        </div>

        {divider("SQL (AS)")}

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
