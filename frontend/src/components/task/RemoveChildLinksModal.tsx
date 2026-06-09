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
import { ScissorOutlined } from "@ant-design/icons";
import { AlterTask } from "../../../wailsjs/go/app/App";
import type { tasks } from "../../../wailsjs/go/models";
import { parsePredecessors, extractName } from "../../utils/taskHierarchy";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  /** The task whose child links will be removed (the parent side of the edge). */
  parentTaskName: string;
  taskRows: tasks.StatusRow[];
  onClose: () => void;
  onSuccess?: () => void;
}

export default function RemoveChildLinksModal({
  db, schema, parentTaskName, taskRows, onClose, onSuccess,
}: Props) {
  const [selected,    setSelected]    = useState<string[]>([]);
  const [submitting,  setSubmitting]  = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);

  const parentUpper = parentTaskName.toUpperCase();

  // Tasks that currently list parentTaskName as a direct predecessor.
  const children = taskRows.filter((t) =>
    parsePredecessors(t.predecessors ?? "").some(
      (p) => extractName(p).toUpperCase() === parentUpper,
    ),
  );

  const handleSubmit = async () => {
    if (selected.length === 0) return;
    setSubmitting(true);
    setSubmitError(null);
    const esc = (s: string) => s.replace(/"/g, '""');
    const removeClause = `REMOVE AFTER "${esc(db)}"."${esc(schema)}"."${esc(parentTaskName)}"`;
    try {
      for (const childName of selected) {
        // Snowflake requires the task to be suspended before ALTER TASK … REMOVE AFTER.
        const row = taskRows.find((t) => t.name.toUpperCase() === childName.toUpperCase());
        if (row?.taskState?.toUpperCase() === "STARTED") {
          await AlterTask(db, schema, childName, "SUSPEND");
        }
        await AlterTask(db, schema, childName, removeClause);
      }
      message.success(
        `Removed ${selected.length} child link${selected.length !== 1 ? "s" : ""} from "${parentTaskName}"`,
      );
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
          <ScissorOutlined style={{ color: "var(--link)" }} />
          <span>Remove Child Task Links</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            from {parentTaskName}
          </Text>
        </Space>
      }
      onCancel={() => !submitting && onClose()}
      okText={`Remove Link${selected.length !== 1 ? "s" : ""}`}
      okButtonProps={{ danger: true, disabled: selected.length === 0, loading: submitting }}
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
          Select child tasks to unlink from{" "}
          <code style={{ fontFamily: "monospace" }}>{parentTaskName}</code>:
        </div>
        <Select
          mode="multiple"
          showSearch
          value={selected}
          onChange={setSelected}
          placeholder="Select child tasks…"
          style={{ width: "100%" }}
          autoFocus
          filterOption={(input, option) =>
            (option?.value as string ?? "").toLowerCase().includes(input.toLowerCase())
          }
          options={children.map((t) => ({ value: t.name, label: t.name }))}
          notFoundContent={
            <span style={{ fontSize: 12, color: "var(--text-muted)" }}>No children found</span>
          }
        />
      </div>
      <Text type="secondary" style={{ fontSize: 11 }}>
        Each selected task will be suspended if running, then unlinked via{" "}
        <code style={{ fontFamily: "monospace" }}>ALTER TASK … REMOVE AFTER</code>.
      </Text>
    </Modal>
  );
}
