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
import { Modal, Select, Typography, Space, Alert, message } from "antd";
import { BranchesOutlined } from "@ant-design/icons";
import { AlterTask } from "../../../wailsjs/go/app/App";
import type { tasks } from "../../../wailsjs/go/models";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  /** The task that will become the parent (predecessor). */
  parentTaskName: string;
  taskRows: tasks.StatusRow[];
  onClose: () => void;
  onSuccess?: () => void;
}

export default function AddExistingChildModal({
  db, schema, parentTaskName, taskRows, onClose, onSuccess,
}: Props) {
  const [selectedTask, setSelectedTask] = useState<string | undefined>(undefined);
  const [submitting,   setSubmitting]   = useState(false);
  const [submitError,  setSubmitError]  = useState<string | null>(null);

  const parentUpper = parentTaskName.toUpperCase();

  // Tasks that are already direct children of parentTaskName.
  const existingChildrenUpper = new Set(
    taskRows
      .filter((t) =>
        parsePredecessors(t.predecessors ?? "").some(
          (p) => extractName(p).toUpperCase() === parentUpper,
        ),
      )
      .map((t) => t.name.toUpperCase()),
  );

  // Eligible: all tasks except the parent itself, its existing children, and finalizer tasks.
  const eligible = taskRows.filter((t) => {
    const u = t.name.toUpperCase();
    return u !== parentUpper && !existingChildrenUpper.has(u) && !t.finalize;
  });

  const handleSubmit = async () => {
    if (!selectedTask) return;
    setSubmitting(true);
    setSubmitError(null);
    try {
      // Snowflake requires the task to be suspended before ALTER TASK … ADD AFTER.
      const row = taskRows.find((t) => t.name.toUpperCase() === selectedTask.toUpperCase());
      if (row?.taskState?.toUpperCase() === "STARTED") {
        await AlterTask(db, schema, selectedTask, "SUSPEND");
      }
      const esc = (s: string) => s.replace(/"/g, '""');
      await AlterTask(
        db, schema, selectedTask,
        `ADD AFTER "${esc(db)}"."${esc(schema)}"."${esc(parentTaskName)}"`,
      );
      message.success(`"${selectedTask}" is now a child of "${parentTaskName}"`);
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
          <BranchesOutlined style={{ color: "var(--link)" }} />
          <span>Add Existing Task as Child</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            of {parentTaskName}
          </Text>
        </Space>
      }
      onCancel={() => !submitting && onClose()}
      okText="Add as Child"
      okButtonProps={{ disabled: !selectedTask, loading: submitting }}
      cancelButtonProps={{ disabled: submitting }}
      onOk={handleSubmit}
      width={440}
      styles={{ body: { paddingTop: 16 } }}
    >
      {submitError && (
        <Alert
          type="error"
          message="Operation failed"
          description={submitError}
          showIcon
          closable
          onClose={() => setSubmitError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      <div style={{ marginBottom: 12 }}>
        <div style={{ fontSize: 12, color: "var(--text-muted)", marginBottom: 8 }}>
          Select a task to become a child of{" "}
          <code style={{ fontFamily: "monospace" }}>{parentTaskName}</code>:
        </div>
        <Select
          showSearch
          value={selectedTask}
          onChange={(v) => setSelectedTask(v)}
          placeholder="Search tasks…"
          style={{ width: "100%" }}
          autoFocus
          filterOption={(input, option) =>
            (option?.value as string ?? "").toLowerCase().includes(input.toLowerCase())
          }
          options={eligible.map((t) => ({ value: t.name, label: t.name }))}
          notFoundContent={
            <span style={{ fontSize: 12, color: "var(--text-muted)" }}>
              No eligible tasks — all graph tasks are already children of this task
            </span>
          }
        />
      </div>
      <Text type="secondary" style={{ fontSize: 11 }}>
        The selected task will be suspended if running, then linked via{" "}
        <code style={{ fontFamily: "monospace" }}>ALTER TASK … ADD AFTER</code>.
      </Text>
    </Modal>
  );
}
