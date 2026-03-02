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
import { UserOutlined } from "@ant-design/icons";
import { ListWarehouses, ListRoles, ExecuteQuery } from "../../../wailsjs/go/main/App";
import type { snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface FormState {
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
}

function buildAlterSql(user: snowflake.SnowflakeUser, form: FormState): string {
  const esc = (s: string) => s.replace(/"/g, '""');
  const sq  = (s: string) => `'${s.replace(/'/g, "''")}'`;

  const setLines: string[] = [];
  const unsetProps: string[] = [];

  const str = (key: string, newVal: string, origVal: string) => {
    if (newVal.trim()) {
      setLines.push(`    ${key} = ${sq(newVal.trim())}`);
    } else if (origVal.trim()) {
      unsetProps.push(key);
    }
  };

  const ident = (key: string, newVal: string, origVal: string) => {
    // warehouse / role / namespace — unquoted identifiers
    if (newVal.trim()) {
      setLines.push(`    ${key} = ${newVal.trim()}`);
    } else if (origVal.trim()) {
      unsetProps.push(key);
    }
  };

  str("LOGIN_NAME",         form.loginName,         user.loginName);
  str("DISPLAY_NAME",       form.displayName,        user.displayName);
  str("FIRST_NAME",         form.firstName,          user.firstName);
  str("LAST_NAME",          form.lastName,           user.lastName);
  str("EMAIL",              form.email,              user.email);
  ident("DEFAULT_WAREHOUSE", form.defaultWarehouse,  user.defaultWarehouse);
  ident("DEFAULT_ROLE",      form.defaultRole,       user.defaultRole);
  ident("DEFAULT_NAMESPACE", form.defaultNamespace,  user.defaultNamespace);
  str("COMMENT",            form.comment,            user.comment);
  setLines.push(`    MUST_CHANGE_PASSWORD = ${form.mustChangePassword ? "TRUE" : "FALSE"}`);

  if (form.daysToExpiry.trim()) {
    setLines.push(`    DAYS_TO_EXPIRY = ${form.daysToExpiry.trim()}`);
  } else if (user.daysToExpiry.trim()) {
    unsetProps.push("DAYS_TO_EXPIRY");
  }

  if (setLines.length === 0 && unsetProps.length === 0) {
    return `-- no changes`;
  }

  let sql = `ALTER USER "${esc(user.name)}"`;
  if (setLines.length > 0) {
    sql += `\nSET\n${setLines.join("\n")}`;
  }
  if (unsetProps.length > 0) {
    sql += `\nUNSET ${unsetProps.join(", ")}`;
  }
  return sql + ";";
}

interface Props {
  user: snowflake.SnowflakeUser;
  onClose: () => void;
  onSuccess: () => void;
}

export default function EditUserModal({ user, onClose, onSuccess }: Props) {
  const [form, setForm] = useState<FormState>({
    loginName:          user.loginName,
    displayName:        user.displayName,
    firstName:          user.firstName,
    lastName:           user.lastName,
    email:              user.email,
    defaultWarehouse:   user.defaultWarehouse,
    defaultRole:        user.defaultRole,
    defaultNamespace:   user.defaultNamespace,
    comment:            user.comment,
    mustChangePassword: user.mustChangePassword,
    daysToExpiry:       user.daysToExpiry,
  });
  const [warehouses, setWarehouses] = useState<string[]>([]);
  const [roles, setRoles]           = useState<string[]>([]);
  const [saving, setSaving]         = useState(false);

  useEffect(() => {
    ListWarehouses().then((w) => setWarehouses(w ?? [])).catch(() => {});
    ListRoles().then((r) => setRoles(r ?? [])).catch(() => {});
  }, []);

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }));

  const handleSave = async () => {
    const sql = buildAlterSql(user, form);
    if (sql === "-- no changes") {
      onClose();
      return;
    }
    setSaving(true);
    try {
      await ExecuteQuery(sql);
      message.success(`User ${user.name} updated`);
      onSuccess();
    } catch (e) {
      message.error(String(e));
    } finally {
      setSaving(false);
    }
  };

  const itemStyle: React.CSSProperties = { marginBottom: 10 };
  const sql = buildAlterSql(user, form);

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <UserOutlined style={{ color: "var(--link)" }} />
          <span>Edit user</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {user.name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose}>Cancel</Button>
          <Button type="primary" icon={<UserOutlined />} onClick={handleSave} loading={saving}>
            Save
          </Button>
        </Space>
      }
      width={640}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      <Form layout="vertical" size="small">

        <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: "0 16px" }}>
          <Form.Item label="Login name" style={itemStyle}>
            <Input value={form.loginName} onChange={(e) => set("loginName", e.target.value)} />
          </Form.Item>
          <Form.Item label="Display name" style={itemStyle}>
            <Input value={form.displayName} onChange={(e) => set("displayName", e.target.value)} />
          </Form.Item>
          <Form.Item label="First name" style={itemStyle}>
            <Input value={form.firstName} onChange={(e) => set("firstName", e.target.value)} />
          </Form.Item>
          <Form.Item label="Last name" style={itemStyle}>
            <Input value={form.lastName} onChange={(e) => set("lastName", e.target.value)} />
          </Form.Item>
        </div>

        <Form.Item label="Email" style={itemStyle}>
          <Input value={form.email} onChange={(e) => set("email", e.target.value)} />
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
            help={<span style={{ fontSize: 11 }}>Clear to remove expiry</span>}
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
            <Checkbox
              checked={form.mustChangePassword}
              onChange={(e) => set("mustChangePassword", e.target.checked)}
            >
              Must change password
            </Checkbox>
          </Form.Item>
        </div>

        <Form.Item label="Comment" style={itemStyle}>
          <Input value={form.comment} onChange={(e) => set("comment", e.target.value)} />
        </Form.Item>

        {/* Live preview */}
        <div style={{ padding: "10px 12px", background: "var(--bg)", borderRadius: 6, border: "1px solid var(--border)", marginTop: 4 }}>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>Preview</Text>
          <pre style={{ margin: 0, color: "var(--text)", fontSize: 11, fontFamily: "'JetBrains Mono', 'Cascadia Code', monospace", whiteSpace: "pre-wrap", wordBreak: "break-all" }}>
            {sql}
          </pre>
        </div>

      </Form>
    </Modal>
  );
}
