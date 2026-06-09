// Copyright (c) 2026 Technarion Oy. All rights reserved.
// ... (license header remains the same)

import { useState, useEffect } from "react";
import { Modal, Form, Input, Select, Button, Space, Typography, Alert } from "antd";
import { CopyOutlined } from "@ant-design/icons";
import { CloneChildTask, GetQuotedIdentifiersIgnoreCase } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  sourceTaskName: string;
  graphTaskNames: string[];
  onClose: () => void;
  onSuccess?: () => void;
}

export default function CopyTaskModal({
  db, schema, sourceTaskName, graphTaskNames, onClose, onSuccess,
}: Props) {
  const [newName,     setNewName]     = useState("");
  const [caseSensitive, setCaseSensitive] = useState(false);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [afterTasks,  setAfterTasks]  = useState<string[]>([]);
  const [submitting,  setSubmitting]  = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase().then((v) => setQuotedIdentifiersIgnoreCase(v ?? false)).catch(() => {});
  }, []);

  // Duplicate-name check against all tasks currently visible in the graph.
  const isDuplicate =
    newName.trim() !== "" &&
    graphTaskNames.some((t) => t.toUpperCase() === newName.trim().toUpperCase());

  const canSubmit = newName.trim() !== "" && !isDuplicate;

  const handleSubmit = async () => {
    if (!canSubmit) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      await CloneChildTask(db, schema, sourceTaskName, newName.trim(), caseSensitive, afterTasks);
      onSuccess?.();
      onClose();
    } catch (err) {
      setSubmitError(String(err));
    } finally {
      setSubmitting(false);
    }
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <CopyOutlined style={{ color: "var(--link)" }} />
          <span>Copy Task</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{sourceTaskName}
          </Text>
        </Space>
      }
      onCancel={() => !submitting && onClose()}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={submitting}>Cancel</Button>
          <Button
            type="primary"
            icon={<CopyOutlined />}
            onClick={handleSubmit}
            disabled={!canSubmit}
            loading={submitting}
          >
            Create Copy
          </Button>
        </Space>
      }
      width={560}
      styles={{ body: { paddingTop: 16 } }}
    >
      {submitError && (
        <Alert
          type="error"
          message="Task creation failed"
          description={submitError}
          showIcon
          closable
          onClose={() => setSubmitError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      
      <Form layout="vertical" size="small">
        <Form.Item
          label="New task name"
          required
          style={{ marginBottom: 4 }}
          validateStatus={isDuplicate ? "error" : ""}
          help={
            isDuplicate
              ? `A task named "${newName.trim()}" already exists in the graph`
              : undefined
          }
        >
          <Input
            value={newName}
            onChange={(e) => setNewName(e.target.value)}
            placeholder={`${sourceTaskName}_COPY`}
            autoFocus
            onPressEnter={handleSubmit}
            status={isDuplicate ? "error" : ""}
          />
        </Form.Item>
        <Form.Item style={{ marginBottom: 12 }}>
          <ObjectNameCaseControl
            name={newName}
            caseSensitive={caseSensitive}
            onCaseSensitiveChange={setCaseSensitive}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item
          label="Predecessor tasks (AFTER)"
          style={{ marginBottom: 12 }}
          help={
            <span style={{ fontSize: 11 }}>
              Leave empty to create the copy as an independent root task.
            </span>
          }
        >
          <Select
            mode="multiple"
            showSearch
            value={afterTasks}
            onChange={(values) => setAfterTasks(values)}
            placeholder="Search and select tasks…"
            allowClear
            style={{ width: "100%" }}
            filterOption={(input, option) =>
              (option?.value as string ?? "").toLowerCase().includes(input.toLowerCase())
            }
            options={graphTaskNames.map((t) => ({ value: t, label: t }))}
            notFoundContent={
              <span style={{ fontSize: 12, color: "var(--text-muted)" }}>No tasks found</span>
            }
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}