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

import { useState, useEffect, useRef } from "react";
import {
  Modal, Form, Input, Space, Button, Alert,
} from "antd";
import { PlusOutlined } from "@ant-design/icons";
import {
  BuildAddDbtProjectVersionSql,
  ExecDDL,
} from "../../../wailsjs/go/app/App";
import SourceLocationPicker from "./SourceLocationPicker";
import SqlPreview from "../shared/SqlPreview";

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
  const previewTimer = useRef<ReturnType<typeof setTimeout>>();

  useEffect(() => {
    let stale = false;
    clearTimeout(previewTimer.current);
    if (!sourceLocation.trim()) {
      setPreview("");
      return;
    }
    previewTimer.current = setTimeout(() => {
      BuildAddDbtProjectVersionSql(db, schema, name, versionAlias, sourceLocation)
        .then((sql) => { if (!stale) setPreview(sql); })
        .catch(() => { if (!stale) setPreview(""); });
    }, 200);
    return () => { stale = true; clearTimeout(previewTimer.current); };
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
            disabled={!canSubmit || !preview}
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
          <Space direction="vertical" style={{ width: "100%" }}>
            <Input
              value={sourceLocation}
              onChange={(e) => setSourceLocation(e.target.value)}
              placeholder="@stage/path or use picker below"
            />
            <SourceLocationPicker
              db={db}
              schema={schema}
              value={sourceLocation}
              onChange={setSourceLocation}
              mode="stage-only"
            />
          </Space>
        </Form.Item>

        <SqlPreview sql={preview} placeholder="-- Fill in source location" />
      </Form>
    </Modal>
  );
}
