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
  Typography, Button, Alert, Switch, DatePicker,
} from "antd";
import { KeyOutlined } from "@ant-design/icons";
import { ListSecurityIntegrations, ExecDDL, GetQuotedIdentifiersIgnoreCase, BuildCreateSecretSql } from "../../../wailsjs/go/app/App";
import ObjectNameCaseControl from "../shared/ObjectNameCaseControl";
import type { snowflake, secret } from "../../../wailsjs/go/models";
import dayjs from "dayjs";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  onClose: () => void;
  onSuccess?: (secretFqn: string) => void;
}

export default function CreateSecretModal({ db, schema, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<secret.SecretConfig>({
    name: "",
    caseSensitive: false,
    orReplace: false,
    ifNotExists: false,
    type: "PASSWORD" as any,
    oauthFlow: "CLIENT_CREDENTIALS",
    apiAuthentication: "",
    oauthScopes: "",
    oauthRefreshToken: "",
    oauthRefreshTokenExpiry: "" as any, // backend expects string
    enabled: true,
    username: "",
    password: "",
    secretString: "",
    comment: "",
  });
  const [integrations, setIntegrations] = useState<snowflake.SecurityIntegration[]>([]);
  const [loadingIntegrations, setLoadingIntegrations] = useState(false);
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [quotedIdentifiersIgnoreCase, setQuotedIdentifiersIgnoreCase] = useState(false);
  const [preview, setPreview] = useState("");

  useEffect(() => {
    setLoadingIntegrations(true);
    ListSecurityIntegrations()
      .then((ints) => setIntegrations(ints ?? []))
      .catch(() => {})
      .finally(() => setLoadingIntegrations(false));

    GetQuotedIdentifiersIgnoreCase()
      .then((v) => setQuotedIdentifiersIgnoreCase(v ?? false))
      .catch(() => {});
  }, []);

  useEffect(() => {
    BuildCreateSecretSql(db, schema, cfg).then(setPreview).catch(() => {});
  }, [db, schema, cfg]);

  const set = <K extends keyof secret.SecretConfig>(key: K, value: secret.SecretConfig[K]) =>
    setCfg((prev) => ({ ...prev, [key]: value }));

  const validate = () => {
    if (!cfg.name.trim()) return false;
    switch (cfg.type as any) {
      case "OAUTH2":
        if (!cfg.apiAuthentication) return false;
        if (cfg.oauthFlow === "AUTHORIZATION_CODE" && !cfg.oauthRefreshToken) return false;
        break;
      case "CLOUD_PROVIDER_TOKEN":
        if (!cfg.apiAuthentication) return false;
        break;
      case "PASSWORD":
        if (!cfg.username || !cfg.password) return false;
        break;
      case "GENERIC_STRING":
        if (!cfg.secretString) return false;
        break;
    }
    return true;
  };

  const canSubmit = validate();

  const handleRun = async () => {
    if (!canSubmit) return;
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(preview);
      // Snowflake uppercases unquoted identifiers; match the casing that
      // ListSecretsInAccount will return so the dropdown auto-selects correctly.
      const effectiveName = cfg.caseSensitive ? cfg.name : cfg.name.toUpperCase();
      const fqn = `"${db}"."${schema}"."${effectiveName}"`;
      onSuccess?.(fqn);
      onClose();
    } catch (err) {
      setCreateError(String(err));
    } finally {
      setCreating(false);
    }
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <KeyOutlined style={{ color: "var(--link)" }} />
          <span>Create Secret</span>
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
            icon={<KeyOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Create
          </Button>
        </Space>
      }
      width={600}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Secret creation failed"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form layout="vertical" size="small">
        <div style={{ display: "grid", gridTemplateColumns: "1fr auto", gap: "0 16px", alignItems: "end" }}>
          <Form.Item label="Secret name" required style={{ marginBottom: 4 }}>
            <Input
              value={cfg.name}
              onChange={(e) => set("name", e.target.value)}
              placeholder="MY_SECRET"
            />
          </Form.Item>
          <Form.Item style={{ marginBottom: 4 }}>
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
        <Form.Item style={itemStyle}>
          <ObjectNameCaseControl
            name={cfg.name}
            caseSensitive={cfg.caseSensitive}
            onCaseSensitiveChange={(v) => set("caseSensitive", v)}
            quotedIdentifiersIgnoreCase={quotedIdentifiersIgnoreCase}
          />
        </Form.Item>

        <Form.Item label="Secret Type" required style={itemStyle}>
          <Select
            value={cfg.type}
            onChange={(v) => set("type", v)}
            options={[
              { value: "OAUTH2", label: "OAUTH2" },
              { value: "CLOUD_PROVIDER_TOKEN", label: "CLOUD_PROVIDER_TOKEN" },
              { value: "PASSWORD", label: "PASSWORD" },
              { value: "GENERIC_STRING", label: "GENERIC_STRING" },
              { value: "SYMMETRIC_KEY", label: "SYMMETRIC_KEY" },
            ]}
          />
        </Form.Item>

        {cfg.type === "OAUTH2" && (
          <>
            <Form.Item label="OAuth Flow" required style={itemStyle}>
              <Radio.Group
                value={cfg.oauthFlow}
                onChange={(e) => set("oauthFlow", e.target.value)}
              >
                <Radio value="CLIENT_CREDENTIALS">Client Credentials Flow</Radio>
                <Radio value="AUTHORIZATION_CODE">Authorization Code Grant Flow</Radio>
              </Radio.Group>
            </Form.Item>
            <Form.Item label="API Authentication" required style={itemStyle}>
              <Select
                value={cfg.apiAuthentication || undefined}
                onChange={(v) => set("apiAuthentication", v)}
                placeholder="Select security integration"
                loading={loadingIntegrations}
                options={integrations
                  .filter(i => i.category === "API_AUTHENTICATION")
                  .map(i => ({ value: i.name, label: i.name }))}
              />
            </Form.Item>
            {cfg.oauthFlow === "CLIENT_CREDENTIALS" ? (
              <Form.Item label="OAuth Scopes" style={itemStyle} help="Comma-separated list of scopes">
                <Input
                  value={cfg.oauthScopes}
                  onChange={(e) => set("oauthScopes", e.target.value)}
                  placeholder="scope1, scope2"
                />
              </Form.Item>
            ) : (
              <>
                <Form.Item label="Refresh Token" required style={itemStyle}>
                  <Input.Password
                    value={cfg.oauthRefreshToken}
                    onChange={(e) => set("oauthRefreshToken", e.target.value)}
                  />
                </Form.Item>
                <Form.Item label="Refresh Token Expiry" style={itemStyle}>
                  <DatePicker
                    showTime
                    style={{ width: "100%" }}
                    value={cfg.oauthRefreshTokenExpiry ? dayjs(cfg.oauthRefreshTokenExpiry) : null}
                    onChange={(v) => set("oauthRefreshTokenExpiry", v ? v.toISOString() : "")}
                  />
                </Form.Item>
              </>
            )}
          </>
        )}

        {cfg.type === "CLOUD_PROVIDER_TOKEN" && (
          <>
            <Form.Item label="API Authentication" required style={itemStyle}>
              <Select
                value={cfg.apiAuthentication || undefined}
                onChange={(v) => set("apiAuthentication", v)}
                placeholder="Select security integration"
                loading={loadingIntegrations}
                options={integrations.map(i => ({ value: i.name, label: i.name }))}
              />
            </Form.Item>
            <Form.Item label="Enabled" valuePropName="checked" style={itemStyle}>
              <Switch checked={cfg.enabled} onChange={(v) => set("enabled", v)} />
            </Form.Item>
          </>
        )}

        {cfg.type === "PASSWORD" && (
          <>
            <Form.Item label="Username" required style={itemStyle}>
              <Input
                value={cfg.username}
                onChange={(e) => set("username", e.target.value)}
              />
            </Form.Item>
            <Form.Item label="Password" required style={itemStyle}>
              <Input.Password
                value={cfg.password}
                onChange={(e) => set("password", e.target.value)}
              />
            </Form.Item>
          </>
        )}

        {cfg.type === "GENERIC_STRING" && (
          <Form.Item label="Secret String" required style={itemStyle}>
            <Input.Password
              value={cfg.secretString}
              onChange={(e) => set("secretString", e.target.value)}
            />
          </Form.Item>
        )}

        {cfg.type === "SYMMETRIC_KEY" && (
          <Form.Item label="Algorithm" style={itemStyle}>
            <Input value="GENERIC" disabled />
          </Form.Item>
        )}

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
          />
        </Form.Item>

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
            SQL Preview
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
