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
  Button,
  Table,
  Space,
  Tag,
  Spin,
  Alert,
  Popconfirm,
  Form,
  Input,
  InputNumber,
  Select,
  Checkbox,
  Modal,
  Typography,
  message,
} from "antd";
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  ReloadOutlined,
  SafetyCertificateOutlined,
} from "@ant-design/icons";
import type { ColumnsType } from "antd/es/table";
import {
  ListBackupPolicies,
  CreateBackupPolicy,
  DropBackupPolicy,
  AlterBackupPolicy,
  GetQuotedIdentifiersIgnoreCase,
} from "../../../wailsjs/go/app/App";
import type { app } from "../../../wailsjs/go/models";
import ObjectNameCaseControl, { identToken } from "../shared/ObjectNameCaseControl";
import dayjs from "dayjs";

const { Text } = Typography;

type AlterAction =
  | "rename"
  | "set-schedule"
  | "set-expire"
  | "set-comment"
  | "unset-comment"
  | "set-retention-lock"
  | "unset-retention-lock";

interface AlterState {
  name: string;
  action: AlterAction;
  value: string;
  caseSensitive: boolean; // only relevant for rename action
}

interface CreateForm {
  name: string;
  caseSensitive: boolean;
  schedule: string;
  expireAfterDays: number | null;
  retentionLock: boolean;
  comment: string;
  tags: string;
  orReplace: boolean;
  ifNotExists: boolean;
}

const DEFAULT_CREATE: CreateForm = {
  name: "",
  caseSensitive: false,
  schedule: "",
  expireAfterDays: null,
  retentionLock: false,
  comment: "",
  tags: "",
  orReplace: false,
  ifNotExists: false,
};

export default function BackupPoliciesPanel() {
  const [rows,    setRows]    = useState<app.BackupPolicyRow[] | null>(null);
  const [loading, setLoading] = useState(false);
  const [error,   setError]   = useState<string | null>(null);
  const [loaded,  setLoaded]  = useState(false);

  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);

  const [createOpen,    setCreateOpen]    = useState(false);
  const [createForm,    setCreateForm]    = useState<CreateForm>(DEFAULT_CREATE);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError,   setCreateError]   = useState<string | null>(null);

  const [alterState,   setAlterState]   = useState<AlterState | null>(null);
  const [alterLoading, setAlterLoading] = useState(false);

  useEffect(() => {
    GetQuotedIdentifiersIgnoreCase().then((v) => setQuotedIdentifiersIgnoreCase(v ?? false)).catch(() => {});
  }, []);

  const load = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await ListBackupPolicies();
      setRows(data ?? []);
      setLoaded(true);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  const handleDrop = async (name: string) => {
    try {
      await DropBackupPolicy(name);
      message.success(`Backup policy "${name}" dropped.`);
      load();
    } catch (e) {
      message.error(String(e));
    }
  };

  const handleCreate = async () => {
    if (!createForm.name.trim()) {
      setCreateError("Policy name is required.");
      return;
    }
    setCreateLoading(true);
    setCreateError(null);
    try {
      await CreateBackupPolicy(
        createForm.name.trim(),
        createForm.schedule.trim(),
        createForm.expireAfterDays ?? 0,
        createForm.retentionLock,
        createForm.comment.trim(),
        createForm.tags.trim(),
        createForm.orReplace,
        createForm.ifNotExists,
        createForm.caseSensitive,
      );
      message.success(`Backup policy "${createForm.name}" created.`);
      setCreateOpen(false);
      setCreateForm(DEFAULT_CREATE);
      load();
    } catch (e) {
      setCreateError(String(e));
    } finally {
      setCreateLoading(false);
    }
  };

  const handleAlterSubmit = async () => {
    if (!alterState) return;
    setAlterLoading(true);
    try {
      const esc = (s: string) => s.replace(/'/g, "''");
      let alteration = "";
      switch (alterState.action) {
        case "rename":
          alteration = `RENAME TO ${identToken(alterState.value, alterState.caseSensitive)}`;
          break;
        case "set-schedule":
          alteration = `SET SCHEDULE = '${esc(alterState.value)}'`;
          break;
        case "set-expire":
          alteration = `SET EXPIRE_AFTER_DAYS = ${parseInt(alterState.value, 10) || 0}`;
          break;
        case "set-comment":
          alteration = `SET COMMENT = '${esc(alterState.value)}'`;
          break;
        case "unset-comment":
          alteration = "UNSET COMMENT";
          break;
        case "set-retention-lock":
          alteration = "SET RETENTION LOCK = TRUE";
          break;
        case "unset-retention-lock":
          alteration = "SET RETENTION LOCK = FALSE";
          break;
      }
      await AlterBackupPolicy(alterState.name, alteration);
      message.success("Backup policy updated.");
      setAlterState(null);
      load();
    } catch (e) {
      message.error(String(e));
    } finally {
      setAlterLoading(false);
    }
  };

  const columns: ColumnsType<app.BackupPolicyRow> = [
    {
      key: "name",
      title: "Name",
      dataIndex: "name",
      render: (v: string) => <Text code style={{ fontSize: 12 }}>{v}</Text>,
    },
    {
      key: "schedule",
      title: "Schedule",
      dataIndex: "schedule",
      render: (v: string) => v || <Text type="secondary">—</Text>,
    },
    {
      key: "expireAfterDays",
      title: "Expires",
      dataIndex: "expireAfterDays",
      width: 80,
      render: (v: number) => v > 0 ? `${v}d` : <Text type="secondary">—</Text>,
    },
    {
      key: "retentionLock",
      title: "Lock",
      dataIndex: "retentionLock",
      width: 60,
      render: (v: boolean) => v ? <Tag color="blue">Locked</Tag> : null,
    },
    {
      key: "owner",
      title: "Owner",
      dataIndex: "owner",
      width: 100,
      render: (v: string) => v || <Text type="secondary">—</Text>,
    },
    {
      key: "createdOn",
      title: "Created",
      dataIndex: "createdOn",
      width: 120,
      render: (v: string) => v ? dayjs(v).format("DD MMM YYYY") : "—",
    },
    {
      key: "comment",
      title: "Comment",
      dataIndex: "comment",
      ellipsis: true,
      render: (v: string) => v || <Text type="secondary">—</Text>,
    },
    {
      key: "actions",
      title: "",
      width: 70,
      render: (_: unknown, row: app.BackupPolicyRow) => (
        <Space size={4}>
          <Button
            size="small"
            type="text"
            icon={<EditOutlined />}
            title="Alter…"
            onClick={() => setAlterState({ name: row.name, action: "rename", value: row.name, caseSensitive: false })}
          />
          <Popconfirm
            title={`Drop backup policy "${row.name}"?`}
            onConfirm={() => handleDrop(row.name)}
            okText="Drop"
            okButtonProps={{ danger: true }}
          >
            <Button size="small" type="text" danger icon={<DeleteOutlined />} title="Drop" />
          </Popconfirm>
        </Space>
      ),
    },
  ];

  const needsValue =
    alterState &&
    ["rename", "set-schedule", "set-expire", "set-comment"].includes(alterState.action);

  const valueLabel: Record<AlterAction, string> = {
    "rename":              "New name",
    "set-schedule":        "Schedule",
    "set-expire":          "Expire after days",
    "set-comment":         "Comment",
    "unset-comment":       "",
    "set-retention-lock":  "",
    "unset-retention-lock":"",
  };

  const valuePlaceholder: Partial<Record<AlterAction, string>> = {
    "rename":       "new_policy_name",
    "set-schedule": "60 MINUTE  /  6 HOUR  /  USING CRON 0 2 * * * UTC",
    "set-expire":   "30",
    "set-comment":  "Your comment…",
  };

  return (
    <div style={{ marginTop: 12 }}>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          padding: "4px 0 6px",
          borderBottom: "1px solid var(--border)",
          marginBottom: 8,
        }}
      >
        <Space size={6}>
          <SafetyCertificateOutlined style={{ fontSize: 12, color: "var(--text-muted)" }} />
          <Text style={{ fontSize: 11, color: "var(--text)", textTransform: "uppercase", letterSpacing: "0.06em" }}>
            Backup Policies
          </Text>
        </Space>
        <Space size={4}>
          <Button
            size="small"
            type="text"
            icon={<ReloadOutlined style={{ fontSize: 11 }} />}
            onClick={load}
            loading={loading}
            style={{ height: 18, padding: "0 4px" }}
          />
          <Button
            size="small"
            type="text"
            icon={<PlusOutlined style={{ fontSize: 11 }} />}
            title="Create backup policy"
            onClick={() => { setCreateOpen(true); if (!loaded) load(); }}
            style={{ height: 18, padding: "0 4px" }}
          />
        </Space>
      </div>

      {!loaded && !loading && (
        <Button size="small" type="link" onClick={load} style={{ padding: 0, fontSize: 11 }}>
          Load backup policies…
        </Button>
      )}

      {loading && <Spin size="small" style={{ display: "block", margin: "8px auto" }} />}
      {error && <Alert type="error" message={error} style={{ marginBottom: 8 }} showIcon />}

      {loaded && !loading && rows !== null && (
        <Table<app.BackupPolicyRow>
          dataSource={rows}
          columns={columns}
          rowKey="name"
          size="small"
          pagination={{ pageSize: 10, showSizeChanger: false, hideOnSinglePage: true }}
          locale={{ emptyText: "No backup policies found." }}
          scroll={{ x: true }}
        />
      )}

      {/* Create modal */}
      <Modal
        open={createOpen}
        title="Create Backup Policy"
        onCancel={() => { setCreateOpen(false); setCreateForm(DEFAULT_CREATE); setCreateError(null); }}
        onOk={handleCreate}
        confirmLoading={createLoading}
        okText="Create"
        width={540}
      >
        <Form layout="vertical" style={{ marginTop: 8 }}>
          <Form.Item label="Policy name" required style={{ marginBottom: 4 }}>
            <Input
              value={createForm.name}
              onChange={(e) => setCreateForm((f) => ({ ...f, name: e.target.value }))}
              placeholder="my_backup_policy"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 12 }}>
            <ObjectNameCaseControl
              name={createForm.name}
              caseSensitive={createForm.caseSensitive}
              onCaseSensitiveChange={(v) => setCreateForm((f) => ({ ...f, caseSensitive: v }))}
              quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
            />
          </Form.Item>
          <Form.Item
            label="Schedule"
            help={<span style={{ fontSize: 11 }}>e.g. <code>60 MINUTE</code>, <code>6 HOUR</code>, <code>USING CRON 0 2 * * * UTC</code></span>}
          >
            <Input
              value={createForm.schedule}
              onChange={(e) => setCreateForm((f) => ({ ...f, schedule: e.target.value }))}
              placeholder="60 MINUTE"
            />
          </Form.Item>
          <Form.Item label="Expire after days">
            <InputNumber
              value={createForm.expireAfterDays}
              onChange={(v) => setCreateForm((f) => ({ ...f, expireAfterDays: v }))}
              min={1}
              style={{ width: 120 }}
              placeholder="e.g. 30"
            />
          </Form.Item>
          <Form.Item label="Tags" help={<span style={{ fontSize: 11 }}>e.g. <code>"MY_TAG" = 'value', "OTHER_TAG" = 'val'</code></span>}>
            <Input
              value={createForm.tags}
              onChange={(e) => setCreateForm((f) => ({ ...f, tags: e.target.value }))}
              placeholder={`"MY_TAG" = 'value'`}
            />
          </Form.Item>
          <Form.Item label="Comment">
            <Input.TextArea
              value={createForm.comment}
              onChange={(e) => setCreateForm((f) => ({ ...f, comment: e.target.value }))}
              rows={2}
              placeholder="Optional description…"
            />
          </Form.Item>
          <Form.Item>
            <Space direction="vertical" size={4}>
              <Checkbox
                checked={createForm.retentionLock}
                onChange={(e) => setCreateForm((f) => ({ ...f, retentionLock: e.target.checked }))}
              >
                WITH RETENTION LOCK
              </Checkbox>
              <Checkbox
                checked={createForm.orReplace}
                onChange={(e) => setCreateForm((f) => ({ ...f, orReplace: e.target.checked, ifNotExists: false }))}
              >
                OR REPLACE
              </Checkbox>
              <Checkbox
                checked={createForm.ifNotExists}
                disabled={createForm.orReplace}
                onChange={(e) => setCreateForm((f) => ({ ...f, ifNotExists: e.target.checked }))}
              >
                IF NOT EXISTS
              </Checkbox>
            </Space>
          </Form.Item>
          {createError && <Alert type="error" message={createError} />}
        </Form>
      </Modal>

      {/* Alter modal */}
      {alterState && (
        <Modal
          open
          title={`Alter Backup Policy: ${alterState.name}`}
          onCancel={() => setAlterState(null)}
          onOk={handleAlterSubmit}
          confirmLoading={alterLoading}
          okText="Apply"
          width={460}
        >
          <Form layout="vertical" style={{ marginTop: 8 }}>
            <Form.Item label="Action">
              <Select
                value={alterState.action}
                onChange={(v) =>
                  setAlterState((s) =>
                    s ? { ...s, action: v, value: v === "rename" ? s.name : "", caseSensitive: false } : s
                  )
                }
                options={[
                  { value: "rename",               label: "Rename To" },
                  { value: "set-schedule",         label: "Set Schedule" },
                  { value: "set-expire",           label: "Set Expire After Days" },
                  { value: "set-comment",          label: "Set Comment" },
                  { value: "unset-comment",        label: "Unset Comment" },
                  { value: "set-retention-lock",   label: "Enable Retention Lock" },
                  { value: "unset-retention-lock", label: "Disable Retention Lock" },
                ]}
              />
            </Form.Item>
            {needsValue && (
              <Form.Item label={valueLabel[alterState.action]} style={{ marginBottom: alterState.action === "rename" ? 4 : undefined }}>
                <Input
                  value={alterState.value}
                  onChange={(e) => setAlterState((s) => s ? { ...s, value: e.target.value } : s)}
                  placeholder={valuePlaceholder[alterState.action]}
                />
              </Form.Item>
            )}
            {alterState.action === "rename" && (
              <Form.Item style={{ marginBottom: 12 }}>
                <ObjectNameCaseControl
                  name={alterState.value}
                  caseSensitive={alterState.caseSensitive}
                  onCaseSensitiveChange={(v) => setAlterState((s) => s ? { ...s, caseSensitive: v } : s)}
                  quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
                />
              </Form.Item>
            )}
          </Form>
        </Modal>
      )}
    </div>
  );
}
