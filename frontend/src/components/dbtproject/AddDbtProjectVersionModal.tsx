// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState, useEffect } from "react";
import {
  Modal, Form, Input, Space,
  Typography, Button, Alert,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  BuildAddDbtProjectVersionSql,
  ExecDDL,
} from "../../../wailsjs/go/main/App";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  onClose: () => void;
  onSuccess?: () => void;
}

export default function AddDbtProjectVersionModal({ db, schema, name, onClose, onSuccess }: Props) {
  const [versionAlias, setVersionAlias] = useState("");
  const [sourceLocation, setSourceLocation] = useState("");
  const [creating, setCreating] = useState(false);
  const [createError, setCreateError] = useState<string | null>(null);
  const [preview, setPreview] = useState("");

  useEffect(() => {
    if (!sourceLocation.trim()) {
      setPreview("");
      return;
    }
    BuildAddDbtProjectVersionSql(db, schema, name, versionAlias, sourceLocation)
      .then(setPreview)
      .catch(() => setPreview(""));
  }, [db, schema, name, versionAlias, sourceLocation]);

  const canSubmit = sourceLocation.trim() !== "";

  const handleRun = async () => {
    if (!canSubmit || !preview) return;
    setCreating(true);
    setCreateError(null);
    try {
      await ExecDDL(preview);
      onSuccess?.();
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
          <PlusOutlined style={{ color: "var(--link)" }} />
          <span>Add Version: {name}</span>
        </Space>
      }
      onCancel={onClose}
      footer={
        <Space style={{ justifyContent: "flex-end", display: "flex" }}>
          <Button onClick={onClose} disabled={creating}>Cancel</Button>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleRun}
            disabled={!canSubmit}
            loading={creating}
          >
            Add Version
          </Button>
        </Space>
      }
      width={520}
      styles={{ body: { paddingTop: 16, maxHeight: "72vh", overflowY: "auto" } }}
    >
      {createError && (
        <Alert
          type="error"
          message="Failed to add version"
          description={createError}
          showIcon
          closable
          onClose={() => setCreateError(null)}
          style={{ marginBottom: 16 }}
        />
      )}
      <Form layout="vertical" size="small">
        <Form.Item label="Version Alias" style={itemStyle}>
          <Input
            value={versionAlias}
            onChange={(e) => setVersionAlias(e.target.value)}
            placeholder="e.g. v1.0 (optional)"
          />
        </Form.Item>

        <Form.Item label="Source Location" required style={itemStyle}>
          <Input
            value={sourceLocation}
            onChange={(e) => setSourceLocation(e.target.value)}
            placeholder="@stage/path"
          />
        </Form.Item>

        {/* SQL Preview */}
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
            {preview || "-- Fill in source location"}
          </pre>
        </div>
      </Form>
    </Modal>
  );
}
