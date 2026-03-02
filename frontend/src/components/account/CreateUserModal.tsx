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
  Modal, Form, Input, Select, Checkbox, Space, Typography,
  Divider, InputNumber, Button, message,
} from "antd";
import { UserAddOutlined } from "@ant-design/icons";
import { ListWarehouses, ListRoles, ExecuteQuery } from "../../../wailsjs/go/main/App";

const { Text } = Typography;

interface FormState {
  name: string;
  password: string;
  loginName: string;
  displayName: string;
  firstName: string;
  lastName: string;
  email: string;
  defaultWarehouse: string;
  defaultRole: string;
  defaultNamespace: string;
  comment: string;
  mustChangePassword: boolean;
  daysToExpiry: string;
  disabled: boolean;
}

const DEFAULTS: FormState = {
  name: "",
  password: "",
  loginName: "",
  displayName: "",
  firstName: "",
  lastName: "",
  email: "",
  defaultWarehouse: "",
  defaultRole: "",
  defaultNamespace: "",
  comment: "",
  mustChangePassword: true,
  daysToExpiry: "",
  disabled: false,
};

function buildCreateSql(form: FormState): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const sq  = (s: string) => `'${s.replace(/'/g, "''")}'`;

  const props: string[] = [];

  if (form.password.trim())        props.push(`    PASSWORD = ${sq(form.password)}`);
  if (form.loginName.trim())       props.push(`    LOGIN_NAME = ${sq(form.loginName.trim())}`);
  if (form.displayName.trim())     props.push(`    DISPLAY_NAME = ${sq(form.displayName.trim())}`);
  if (form.firstName.trim())       props.push(`    FIRST_NAME = ${sq(form.firstName.trim())}`);
  if (form.lastName.trim())        props.push(`    LAST_NAME = ${sq(form.lastName.trim())}`);
  if (form.email.trim())           props.push(`    EMAIL = ${sq(form.email.trim())}`);
  if (form.defaultWarehouse.trim()) props.push(`    DEFAULT_WAREHOUSE = ${form.defaultWarehouse.trim()}`);
  if (form.defaultRole.trim())     props.push(`    DEFAULT_ROLE = ${form.defaultRole.trim()}`);
  if (form.defaultNamespace.trim()) props.push(`    DEFAULT_NAMESPACE = ${form.defaultNamespace.trim()}`);
  if (form.comment.trim())         props.push(`    COMMENT = ${sq(form.comment.trim())}`);
  props.push(`    MUST_CHANGE_PASSWORD = ${form.mustChangePassword ? "TRUE" : "FALSE"}`);
  if (form.daysToExpiry.trim())    props.push(`    DAYS_TO_EXPIRY = ${form.daysToExpiry.trim()}`);
  props.push(`    DISABLED = ${form.disabled ? "TRUE" : "FALSE"}`);

  const name = form.name.trim() || "NEW_USER";
  return `CREATE USER "${esc(name)}"\n${props.join("\n")};`;
}

interface Props {
  onClose: () => void;
  onSuccess: () => void;
}

export default function CreateUserModal({ onClose, onSuccess }: Props) {
  const [form, setForm]         = useState<FormState>(DEFAULTS);
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [roles, setRoles]           = useState<string[]>([]);
  const [saving, setSaving]         = useState(false);

  useEffect(() => {
    ListWarehouses().then((w) => setWarehouses(w ?? [])).catch(() => {});
    ListRoles().then((r)      => setRoles(r ?? [])).catch(() => {});
  }, []);

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const canCreate = form.name.trim() !== "";

  const handleCreate = async () => {
    setSaving(true);
    try {
      await ExecuteQuery(buildCreateSql(form));
      message.success(`Created user ${form.name.trim()}`);
      onSuccess();
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  const itemStyle: React.CSSProperties = { marginBottom: 10 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <UserAddOutlined style={{ color: "var(--link)" }} />
          <span>Create user</span>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" icon={<UserAddOutlined />} onClick={handleCreate} loading={saving} disabled={!canCreate}>
            Create
          </Button>
        </Space>
      }
      width={640}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      <Form layout="vertical" size="small">

        <Form.Item label="Username" required style={itemStyle}>
          <Input
            value={form.name}
            onChange={(e) => set("name", e.target.value)}
            placeholder="JOHN_DOE"
          />
        </Form.Item>

        <Form.Item label="Password" style={itemStyle}>
          <Input.Password
            value={form.password}
            onChange={(e) => set("password", e.target.value)}
            placeholder="Leave blank to create without password"
            autoComplete="new-password"
          />
        </Form.Item>

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 10px" }}>
          Identity
        </Divider>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Login name" style={itemStyle}>
            <Input value={form.loginName} onChange={(e) => set("loginName", e.target.value)} placeholder="john.doe" />
          </Form.Item>
          <Form.Item label="Display name" style={itemStyle}>
            <Input value={form.displayName} onChange={(e) => set("displayName", e.target.value)} placeholder="John Doe" />
          </Form.Item>
          <Form.Item label="First name" style={itemStyle}>
            <Input value={form.firstName} onChange={(e) => set("firstName", e.target.value)} />
          </Form.Item>
          <Form.Item label="Last name" style={itemStyle}>
            <Input value={form.lastName} onChange={(e) => set("lastName", e.target.value)} />
          </Form.Item>
        </div>

        <Form.Item label="Email" style={itemStyle}>
          <Input value={form.email} onChange={(e) => set("email", e.target.value)} placeholder="john@example.com" />
        </Form.Item>

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 10px" }}>
          Defaults
        </Divider>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Default warehouse" style={itemStyle}>
            <Select
              value={form.defaultWarehouse || undefined}
              onChange={(v) => set("defaultWarehouse", v ?? "")}
              placeholder="— none —"
              showSearch
              allowClear
              options={warehouses.map((w) => ({ value: w, label: w }))}
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label="Default role" style={itemStyle}>
            <Select
              value={form.defaultRole || undefined}
              onChange={(v) => set("defaultRole", v ?? "")}
              placeholder="— none —"
              showSearch
              allowClear
              options={roles.map((r) => ({ value: r, label: r }))}
              style={{ width: "100%" }}
            />
          </Form.Item>
        </div>

        <Form.Item label="Default namespace" style={itemStyle}
          help={<span style={{ fontSize: 11 }}>DATABASE or DATABASE.SCHEMA</span>}
        >
          <Input value={form.defaultNamespace} onChange={(e) => set("defaultNamespace", e.target.value)} placeholder="MY_DB.MY_SCHEMA" />
        </Form.Item>

        <Divider orientation="left" orientationMargin={0} style={{ fontSize: 11, color: "var(--text-muted)", margin: "4px 0 10px" }}>
          Security
        </Divider>

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Days to expiry" style={itemStyle}
            help={<span style={{ fontSize: 11 }}>Leave blank for no expiry</span>}
          >
            <InputNumber
              value={form.daysToExpiry === "" ? undefined : Number(form.daysToExpiry)}
              onChange={(v) => set("daysToExpiry", v === null ? "" : String(v))}
              min={0}
              placeholder="never"
              style={{ width: "100%" }}
            />
          </Form.Item>
          <Form.Item label=" " style={itemStyle}>
            <Space direction="vertical" size={4}>
              <Checkbox checked={form.mustChangePassword} onChange={(e) => set("mustChangePassword", e.target.checked)}>
                Must change password
              </Checkbox>
              <Checkbox checked={form.disabled} onChange={(e) => set("disabled", e.target.checked)}>
                Create as disabled
              </Checkbox>
            </Space>
          </Form.Item>
        </div>

        <Form.Item label="Comment" style={itemStyle}>
          <Input value={form.comment} onChange={(e) => set("comment", e.target.value)} />
        </Form.Item>

        {/* Live preview */}
        <div style={{ padding: "10px 12px", background: "var(--bg)", borderRadius: 6, border: "1px solid var(--border)", marginTop: 4 }}>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>Preview</Text>
          <pre style={{ margin: 0, color: "var(--text)", fontSize: 11, fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
            {buildCreateSql(form)}
          </pre>
        </div>

      </Form>
    </Modal>
  );
}
