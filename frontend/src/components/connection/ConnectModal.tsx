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
import { Form, Input, Button, Alert, Space, Typography, Select, Divider, Tooltip } from "antd";
import { CloudServerOutlined, FolderOpenOutlined } from "@ant-design/icons";
import {
  Connect, CancelConnect, LoadSnowflakeCLIConfig,
  GetSnowflakeCLIConfigPath, PickSnowflakeCLIConfigPath,
} from "../../../wailsjs/go/main/App";
import { useConnectionStore, type ConnectionParams } from "../../store/connectionStore";
import type { sfconfig } from "../../../wailsjs/go/models";

const { Title, Text } = Typography;

const AUTH_OPTIONS = [
  {
    value: "username_password_mfa",
    label: "Password + MFA push",
    description: "Approve a push notification on your MFA device",
  },
  {
    value: "externalbrowser",
    label: "Browser SSO",
    description: "Opens a browser window for SSO / MFA",
  },
  {
    value: "snowflake",
    label: "Password only",
    description: "Classic username + password (optionally with a TOTP code)",
  },
  {
    value: "okta",
    label: "Okta native SSO",
    description: "Authenticates directly against your Okta tenant",
  },
  {
    value: "snowflake_jwt",
    label: "Key pair (JWT)",
    description: "RSA private key — no password needed",
  },
];

const needsPassword = (auth: string) =>
  auth !== "externalbrowser" && auth !== "snowflake_jwt";

export default function ConnectModal() {
  const [form] = Form.useForm<ConnectionParams>();
  const [loading, setLoading]   = useState(false);
  const [error, setError]       = useState<string | null>(null);
  const [auth, setAuth]         = useState("username_password_mfa");
  const setConnected            = useConnectionStore((s) => s.setConnected);

  const [cliConfig, setCliConfig] = useState<sfconfig.Config | null>(null);
  const [cliConfigPath, setCliConfigPath] = useState<string>("");

  const refreshCliConfig = () => {
    LoadSnowflakeCLIConfig()
      .then((cfg) => {
        setCliConfig(cfg.connections?.length ? cfg : null);
      })
      .catch(() => {
        setCliConfig(null);
      });
  };

  // Load Snowflake CLI config and path once on mount.
  useEffect(() => {
    GetSnowflakeCLIConfigPath().then(setCliConfigPath);
    refreshCliConfig();
  }, []);

  const changeCliConfigPath = async () => {
    try {
      const path = await PickSnowflakeCLIConfigPath();
      if (path) {
        setCliConfigPath(path);
        refreshCliConfig();
      }
    } catch (e) {
      console.error("Failed to pick config path", e);
    }
  };

  const applyCliConnection = (name: string) => {
    const conn = cliConfig?.connections?.find((c) => c.name === name);
    if (!conn) return;

    const authValue = (conn.authenticator || "username_password_mfa").toLowerCase();
    setAuth(authValue);

    form.setFieldsValue({
      account:              conn.account,
      user:                 conn.user,
      password:             conn.password,
      role:                 conn.role,
      warehouse:            conn.warehouse,
      database:             conn.database,
      schema:               conn.schema,
      authenticator:        authValue,
      passcode:             conn.passcode,
      oktaUrl:              conn.oktaUrl,
      privateKeyPath:       conn.privateKeyPath,
      privateKeyPassphrase: conn.privateKeyPassphrase,
    });
  };

  const onFinish = async (values: ConnectionParams) => {
    setLoading(true);
    setError(null);
    try {
      await Connect(values);
      setConnected(values);
    } catch (e) {
      setError(String(e));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        height: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "var(--bg)",
      }}
    >
      <div style={{ width: 460 }}>
        <Space direction="vertical" size={24} style={{ width: "100%" }}>
          <Space align="center">
            <CloudServerOutlined style={{ fontSize: 28, color: "#29B6F6" }} />
            <Title level={3} style={{ margin: 0, color: "var(--text)" }}>
              Connect to Snowflake
            </Title>
          </Space>

          {/* ── Snowflake CLI profiles ──────────────────────────────────── */}
          <div>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-end", marginBottom: 6 }}>
              <Text type="secondary" style={{ fontSize: 12 }}>
                Snowflake CLI profiles
              </Text>
              <Tooltip title={cliConfigPath}>
                <Button
                  type="link"
                  size="small"
                  icon={<FolderOpenOutlined />}
                  onClick={changeCliConfigPath}
                  style={{ fontSize: 11, padding: 0, height: "auto" }}
                >
                  Change config…
                </Button>
              </Tooltip>
            </div>

            {cliConfig && cliConfig.connections?.length > 0 ? (
              <Select
                style={{ width: "100%" }}
                placeholder="Select a connection profile…"
                onChange={applyCliConnection}
                defaultValue={cliConfig.defaultConnection || undefined}
                options={cliConfig.connections.map((c) => ({
                  value: c.name,
                  label: cliConfig.defaultConnection === c.name ? `${c.name} (default)` : c.name,
                }))}
              />
            ) : (
              <div style={{ 
                padding: "8px 12px", 
                background: "var(--bg-faint)", 
                border: "1px dashed var(--border)",
                borderRadius: 6,
                textAlign: "center"
              }}>
                <Text type="secondary" style={{ fontSize: 12 }}>
                  No profiles found in {cliConfigPath.split("/").pop() || "config.toml"}
                </Text>
              </div>
            )}
            <Divider style={{ borderColor: "var(--border)", margin: "16px 0 4px" }} />
          </div>

          {error && <Alert type="error" message={error} showIcon />}

          <Form
            form={form}
            layout="vertical"
            onFinish={onFinish}
            requiredMark={false}
            initialValues={{ authenticator: "username_password_mfa" }}
          >
            {/* ── Connection details ─────────────────────────────────── */}
            <Form.Item name="account" label="Account" rules={[{ required: true }]}>
              <Input placeholder="myorg-account  or  locator.region (e.g. xy12345.eu-north-1)" />
            </Form.Item>

            <Space.Compact style={{ width: "100%", gap: 8, display: "flex" }}>
              <Form.Item name="role" label="Role" style={{ flex: 1 }}>
                <Input placeholder="SYSADMIN" />
              </Form.Item>
              <Form.Item name="warehouse" label="Warehouse" style={{ flex: 1 }}>
                <Input placeholder="COMPUTE_WH" />
              </Form.Item>
            </Space.Compact>

            <Space.Compact style={{ width: "100%", gap: 8, display: "flex" }}>
              <Form.Item name="database" label="Database" style={{ flex: 1 }}>
                <Input placeholder="optional" />
              </Form.Item>
              <Form.Item name="schema" label="Schema" style={{ flex: 1 }}>
                <Input placeholder="optional" />
              </Form.Item>
            </Space.Compact>

            <Divider style={{ borderColor: "var(--border)", margin: "4px 0 16px" }} />

            {/* ── Authentication ─────────────────────────────────────── */}
            <Form.Item name="authenticator" label="Authentication method">
              <Select
                onChange={(v) => {
                  setAuth(v);
                  form.resetFields(["passcode", "oktaUrl", "privateKeyPath", "privateKeyPassphrase"]);
                }}
                options={AUTH_OPTIONS.map((o) => ({ value: o.value, label: o.label }))}
                optionRender={(option) => {
                  const o = AUTH_OPTIONS.find((x) => x.value === option.value)!;
                  return (
                    <div>
                      <div>{o.label}</div>
                      <div style={{ fontSize: 11, color: "var(--text-muted)", marginTop: 2 }}>
                        {o.description}
                      </div>
                    </div>
                  );
                }}
              />
            </Form.Item>

            {/* Username */}
            {auth !== "externalbrowser" && (
              <Form.Item name="user" label="Username" rules={[{ required: true }]}>
                <Input autoComplete="username" />
              </Form.Item>
            )}

            {/* Password */}
            {needsPassword(auth) && (
              <Form.Item name="password" label="Password" rules={[{ required: true }]}>
                <Input.Password autoComplete="current-password" />
              </Form.Item>
            )}

            {/* TOTP passcode (snowflake authenticator only) */}
            {auth === "snowflake" && (
              <Form.Item name="passcode" label="TOTP passcode (optional)">
                <Input placeholder="6-digit code" maxLength={8} />
              </Form.Item>
            )}

            {/* Okta URL */}
            {auth === "okta" && (
              <Form.Item
                name="oktaUrl"
                label="Okta account URL"
                rules={[{ required: true, type: "url" }]}
              >
                <Input placeholder="https://mycompany.okta.com" />
              </Form.Item>
            )}

            {/* Key pair */}
            {auth === "snowflake_jwt" && (
              <>
                <Form.Item
                  name="privateKeyPath"
                  label="Private key path"
                  rules={[{ required: true }]}
                >
                  <Input placeholder="/path/to/rsa_key.p8" />
                </Form.Item>
                <Form.Item name="privateKeyPassphrase" label="Key passphrase (if encrypted)">
                  <Input.Password />
                </Form.Item>
              </>
            )}

            <Form.Item style={{ marginBottom: 0, marginTop: 8 }}>
              {loading ? (
                <Button danger block onClick={() => CancelConnect()}>
                  Cancel
                </Button>
              ) : (
                <Button type="primary" htmlType="submit" block>
                  {auth === "externalbrowser" ? "Connect (opens browser)" : "Connect"}
                </Button>
              )}
            </Form.Item>
          </Form>
        </Space>
      </div>
    </div>
  );
}
