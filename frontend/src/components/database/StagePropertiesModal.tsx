// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { useState, useEffect } from "react";
import { Modal, Form, Input, Button, Alert, Space, Typography, Checkbox, Select } from "antd";
import { EditOutlined } from "@ant-design/icons";
import { ExecDDL, BuildAlterStageSql, GetObjectProperties, ListIntegrations } from "../../../wailsjs/go/main/App";
import type { stage, snowflake } from "../../../wailsjs/go/models";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function StagePropertiesModal({ db, schema, name, onClose, onSuccess }: Props) {
  const [cfg, setCfg] = useState<any>({
    name,
    database: db,
    schema,
    action: "SET",
    newName: "",
    caseSensitive: false,
    comment: null,
    url: null,
    storageIntegration: null,
    directoryEnabled: null,
  });

  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [preview, setPreview] = useState("");
  const [integrations, setIntegrations] = useState<snowflake.IntegrationRow[]>([]);

  useEffect(() => {
    ListIntegrations("STORAGE").then(setIntegrations).catch(() => {});
    GetObjectProperties(db, schema, name, "STAGE").then((props) => {
      const commentProp = props.find(p => p.key === "COMMENT");
      const dirProp = props.find(p => p.key === "DIRECTORY_ENABLED");
      if (commentProp && commentProp.value && commentProp.value !== "null") {
        set("comment", commentProp.value);
      }
      if (dirProp && dirProp.value) {
        set("directoryEnabled", dirProp.value === "true");
      }
    }).catch(() => {});
  }, [db, schema, name]);

  useEffect(() => {
    BuildAlterStageSql(cfg as stage.AlterStageConfig).then(setPreview).catch(() => {});
  }, [cfg]);

  const set = (key: keyof stage.AlterStageConfig, value: any) =>
    setCfg((prev: any) => ({ ...prev, [key]: value }));

  const handleSave = async () => {
    setSaving(true);
    setError(null);
    try {
      const sql = await BuildAlterStageSql(cfg as stage.AlterStageConfig);
      await ExecDDL(sql);
      onSuccess?.();
      onClose();
    } catch (err) {
      setError(String(err));
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal
      open
      title={
        <Space size={6}>
          <EditOutlined style={{ color: "var(--link)" }} />
          <span>Alter stage</span>
          <Text type="secondary" style={{ fontSize: 12, fontWeight: 400 }}>
            {db}.{schema}.{name}
          </Text>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={saving}>Cancel</Button>
          <Button type="primary" onClick={handleSave} loading={saving}>Apply</Button>
        </Space>
      }
    >
      {error && <Alert type="error" message="Alter failed" description={error} showIcon style={{ marginBottom: 16 }} />}

      <Form layout="vertical" size="small">
        <Form.Item label="Action" style={{ marginBottom: 12 }}>
          <Select
            value={cfg.action}
            onChange={(v) => set("action", v)}
            options={[
              { value: "SET", label: "Set Properties" },
              { value: "UNSET", label: "Unset Properties" },
              { value: "RENAME", label: "Rename" },
              { value: "REFRESH", label: "Refresh Directory Table" },
            ]}
          />
        </Form.Item>

        {cfg.action === "RENAME" && (
          <Form.Item label="New name" style={{ marginBottom: 12 }}>
            <Input value={cfg.newName} onChange={(e) => set("newName", e.target.value)} />
          </Form.Item>
        )}

        {(cfg.action === "SET" || cfg.action === "UNSET") && (
          <>
            <Form.Item label="Comment" style={{ marginBottom: 12 }}>
              <Input
                value={cfg.comment || ""}
                onChange={(e) => set("comment", e.target.value)}
                placeholder="Stage comment"
              />
            </Form.Item>
          </>
        )}

        {cfg.action === "SET" && (
          <>
            <Form.Item label="URL" style={{ marginBottom: 12 }}>
              <Input
                value={cfg.url || ""}
                onChange={(e) => set("url", e.target.value)}
                placeholder="s3://bucket/path"
                allowClear
              />
            </Form.Item>
            <Form.Item label="Storage Integration" style={{ marginBottom: 12 }}>
              <Select
                value={cfg.storageIntegration || ""}
                onChange={(v) => set("storageIntegration", v)}
                options={integrations.map(i => ({ value: i.name, label: i.name }))}
                allowClear
              />
            </Form.Item>
            <Form.Item style={{ marginBottom: 12 }}>
              <Checkbox
                checked={cfg.directoryEnabled || false}
                onChange={(e) => set("directoryEnabled", e.target.checked)}
              >
                Enable Directory
              </Checkbox>
            </Form.Item>
          </>
        )}

        <div style={{ padding: "10px 12px", background: "var(--bg)", borderRadius: 6, border: "1px solid var(--border)", marginTop: 12 }}>
          <Text type="secondary" style={{ fontSize: 11, display: "block", marginBottom: 4 }}>SQL Preview</Text>
          <pre style={{ margin: 0, color: "var(--text)", fontSize: 11, fontFamily: "monospace", whiteSpace: "pre-wrap" }}>
            {preview}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}

// @thaw-domain: Object Browser & Administration
