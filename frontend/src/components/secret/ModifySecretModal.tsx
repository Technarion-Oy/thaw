// SPDX-License-Identifier: GPL-3.0-or-later

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Select, Space,
  Typography, Button, Alert, DatePicker,
} from "antd";
import { KeyOutlined } from "@ant-design/icons";
import { ListSecurityIntegrations, ExecDDL, GetObjectProperties, BuildModifySecretSql } from "../../../wailsjs/go/app/App";
import type { secret } from "../../../wailsjs/go/models";
import dayjs from "dayjs";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function ModifySecretModal({ db, schema, name, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<secret.SecretConfig | null>(null);
  const [originalComment, setOriginalComment] = useState("");
  const [integrations, setIntegrations] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [modifying, setModifying] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState("");

  useEffect(() => {
    const init = async () => {
      try {
        const [props, ints] = await Promise.all([
          GetObjectProperties(db, schema, "SECRET", name),
          ListSecurityIntegrations(),
        ]);
        setIntegrations(ints ?? []);

        const pMap = new Map(props.map(p => [p.key.toUpperCase(), p.value]));
        const type = pMap.get("TYPE") || "PASSWORD";
        const comment = pMap.get("COMMENT") || "";
        setOriginalComment(comment);

        setCfg({
          name,
          caseSensitive: true,
          orReplace: false,
          ifNotExists: false,
          type: type as any,
          oauthFlow: pMap.has("OAUTH_REFRESH_TOKEN") ? "AUTHORIZATION_CODE" : "CLIENT_CREDENTIALS",
          apiAuthentication: pMap.get("API_AUTHENTICATION") || "",
          oauthScopes: pMap.get("OAUTH_SCOPES") || "",
          oauthRefreshToken: "", // masked in DESCRIBE
          oauthRefreshTokenExpiry: pMap.has("OAUTH_REFRESH_TOKEN_EXPIRY_TIME") ? dayjs(pMap.get("OAUTH_REFRESH_TOKEN_EXPIRY_TIME")).toISOString() : "",
          enabled: pMap.get("ENABLED") === "true",
          username: pMap.get("USERNAME") || "",
          password: "", // masked
          secretString: "", // masked
          comment,
        });
      } catch (err) {
        setError(String(err));
      } finally {
        setLoading(false);
      }
    };
    init();
  }, [db, schema, name]);

  useEffect(() => {
    if (cfg) {
      BuildModifySecretSql(db, schema, name, cfg, originalComment).then((sqls) => setPreview(sqls.join("\n\n"))).catch(() => {});
    }
  }, [db, schema, name, cfg, originalComment]);

  const set = <K extends keyof secret.SecretConfig>(key: K, value: secret.SecretConfig[K]) =>
    setCfg((prev) => prev ? ({ ...prev, [key]: value }) : null);

  const handleRun = async () => {
    if (!cfg) return;
    const sqls = preview.split("\n\n").filter(s => s.trim() !== "");
    if (sqls.length === 0) {
      onClose();
      return;
    }
    setModifying(true);
    setError(null);
    try {
      for (const sql of sqls) {
        await ExecDDL(sql);
      }
      onSuccess?.();
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setModifying(false);
    }
  };

  if (loading) {
    return (
      <Modal open title="Modify Secret" onCancel={onClose} footer={null}>
        <div style={{ padding: 20, textAlign: "center" }}>Loading properties…</div>
      </Modal>
    );
  }

  if (!cfg) return null;

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <KeyOutlined style={{ color: "var(--link)" }} />
          <span>Modify Secret: {name}</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {cfg.type}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={modifying}>Cancel</Button>
          <Button
            type="primary"
            onClick={handleRun}
            loading={modifying}
          >
            Apply Changes
          </Button>
        </Space>
      }
      width={600}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      {error && (
        <Alert
          type="error"
          message="Modification failed"
          description={error}
          showIcon
          closable
          onClose={() => setError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form layout="vertical" size="small">
        {cfg.type === "OAUTH2" && (originalComment === "AUTHORIZATION_CODE" || cfg.oauthFlow === "AUTHORIZATION_CODE") && (
          <>
            <Form.Item label="OAuth Refresh Token" style={itemStyle} help="Leave empty to keep existing value">
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

        {cfg.type === "OAUTH2" && cfg.oauthFlow === "CLIENT_CREDENTIALS" && (
          <Form.Item label="OAuth Scopes" style={itemStyle} help="Comma-separated list of scopes">
            <Input
              value={cfg.oauthScopes}
              onChange={(e) => set("oauthScopes", e.target.value)}
              placeholder="scope1, scope2"
            />
          </Form.Item>
        )}

        {cfg.type === "CLOUD_PROVIDER_TOKEN" && (
          <Form.Item label="API Authentication" style={itemStyle}>
            <Select
              value={cfg.apiAuthentication || undefined}
              onChange={(v) => set("apiAuthentication", v)}
              placeholder="Select security integration"
              options={integrations.map(i => ({ value: i.name, label: i.name }))}
            />
          </Form.Item>
        )}

        {cfg.type === "PASSWORD" && (
          <>
            <Form.Item label="Username" style={itemStyle}>
              <Input
                value={cfg.username}
                onChange={(e) => set("username", e.target.value)}
              />
            </Form.Item>
            <Form.Item label="Password" style={itemStyle} help="Leave empty to keep existing value">
              <Input.Password
                value={cfg.password}
                onChange={(e) => set("password", e.target.value)}
              />
            </Form.Item>
          </>
        )}

        {cfg.type === "GENERIC_STRING" && (
          <Form.Item label="Secret String" style={itemStyle} help="Leave empty to keep existing value">
            <Input.Password
              value={cfg.secretString}
              onChange={(e) => set("secretString", e.target.value)}
            />
          </Form.Item>
        )}

        <Form.Item label="Comment" style={itemStyle}>
          <Input
            value={cfg.comment}
            onChange={(e) => set("comment", e.target.value)}
            placeholder="optional comment"
            allowClear
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
            {preview || "-- No changes"}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
