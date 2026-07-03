// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { useState } from "react";
import { App as AntApp, Form, Input, Switch, Button, Typography } from "antd";
import { UploadOutlined, FileOutlined } from "@ant-design/icons";
import { PickAnyFile, UploadFileToStage } from "../../../wailsjs/go/app/App";
import CreateModalShell from "../shared/CreateModalShell";
import { quoteIdent } from "../shared/ObjectNameCaseControl";

const { Text } = Typography;

interface Props {
  db: string;
  schema: string;
  name: string;
  /** Destination sub-directory prefill (e.g. the right-clicked dir); "" targets the stage root. */
  initialPath?: string;
  onClose: () => void;
  /**
   * Called after a successful upload with the resolved destination path
   * (normalised, relative to the stage root) so the caller can refresh the
   * actual destination node rather than the right-click-time path.
   */
  onSuccess?: (destPath: string) => void;
}

export default function UploadToStageModal({ db, schema, name, initialPath = "", onClose, onSuccess }: Props) {
  const { message } = AntApp.useApp();
  const [localPath, setLocalPath] = useState("");
  const [destPath, setDestPath] = useState(initialPath);
  const [overwrite, setOverwrite] = useState(true);
  const [autoCompress, setAutoCompress] = useState(true);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const stageRoot = `@${quoteIdent(db)}.${quoteIdent(schema)}.${quoteIdent(name)}`;
  const dest = destPath.trim().replace(/^\/+|\/+$/g, "");
  const stageRef = dest ? `${stageRoot}/${dest}` : stageRoot;
  // dest is spliced unquoted into a PUT statement server-side; reject characters
  // and sequences that could break out of it (statement terminator, quotes,
  // newlines, and the '--' comment that would silently drop the PUT options). The
  // backend enforces this too — this just gives immediate feedback.
  const pathError = /[;'"`\n\r]|--/.test(destPath)
    ? "Path cannot contain ; ' \" ` -- or newlines."
    : null;

  const pickFile = async () => {
    const p = await PickAnyFile();
    if (p) setLocalPath(p);
  };

  const handleUpload = () => {
    if (!localPath || pathError) return;
    setUploading(true);
    setError(null);
    UploadFileToStage(localPath, stageRef, 4, autoCompress, "AUTO_DETECT", overwrite)
      .then(() => {
        message.success("Uploaded successfully.");
        onSuccess?.(dest);
        onClose();
      })
      .catch((e) => setError(String(e)))
      .finally(() => setUploading(false));
  };

  const itemStyle: React.CSSProperties = { marginBottom: 12 };

  return (
    <CreateModalShell
      icon={<UploadOutlined />}
      title="Upload File to Stage"
      subtitle={`${db}.${schema}.${name}`}
      width={560}
      error={error}
      errorTitle="Upload failed"
      onErrorClose={() => setError(null)}
      creating={uploading}
      canSubmit={!!localPath && !pathError}
      okText="Upload"
      onClose={onClose}
      onSubmit={handleUpload}
    >
      <Form layout="vertical" size="small">
        <Form.Item label="File" required style={itemStyle}>
          <Button icon={<FileOutlined />} onClick={pickFile} block style={{ textAlign: "left" }}>
            {localPath || "Choose file…"}
          </Button>
        </Form.Item>

        <Form.Item
          label="Destination path"
          style={itemStyle}
          validateStatus={pathError ? "error" : undefined}
          help={pathError ?? <Text type="secondary" style={{ fontSize: 12 }}>Uploads to {stageRef}</Text>}
        >
          <Input
            value={destPath}
            onChange={(e) => setDestPath(e.target.value)}
            placeholder="e.g. data/2026/ (leave empty for stage root)"
            addonBefore={`${stageRoot}/`}
          />
        </Form.Item>

        <Form.Item label="Overwrite existing file" valuePropName="checked" style={itemStyle}>
          <Switch checked={overwrite} onChange={setOverwrite} />
        </Form.Item>

        <Form.Item
          label="Auto-compress"
          valuePropName="checked"
          style={itemStyle}
          help={<Text type="secondary" style={{ fontSize: 12 }}>Snowflake gzips the file on upload unless it is already compressed</Text>}
        >
          <Switch checked={autoCompress} onChange={setAutoCompress} />
        </Form.Item>
      </Form>
    </CreateModalShell>
  );
}
