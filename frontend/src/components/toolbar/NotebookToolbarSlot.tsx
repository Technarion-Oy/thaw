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
// @thaw-domain: Snowpark & Developer Workflows

import { Button, Space, Spin, Tooltip, Typography, Tag } from "antd";
import {
  PlayCircleOutlined,
  ReloadOutlined,
  PlusOutlined,
  CloudUploadOutlined,
} from "@ant-design/icons";

const { Text } = Typography;

export interface NotebookToolbarSlotProps {
  kernelReady: boolean;
  kernelStarting: boolean;
  kernelError: string | null;
  saving: boolean;
  onRunAll: () => void;
  onRestartKernel: () => void;
  onAddCell: () => void;
  onDeploy: () => void;
}

export default function NotebookToolbarSlot({
  kernelReady,
  kernelStarting,
  kernelError,
  saving: _saving,
  onRunAll,
  onRestartKernel,
  onAddCell,
  onDeploy,
}: NotebookToolbarSlotProps) {
  return (
    <Space size={4}>
      {/* Separator */}
      <div style={{ width: 1, height: 20, background: "var(--border)", margin: "0 4px" }} />

      <Tooltip title="Run all cells">
        <Button
          icon={<PlayCircleOutlined />}
          size="small"
          onClick={onRunAll}
          disabled={!kernelReady}
          style={{ width: 28, padding: 0 }}
        />
      </Tooltip>
      <Tooltip title="Restart kernel">
        <Button
          icon={<ReloadOutlined />}
          size="small"
          onClick={onRestartKernel}
          style={{ width: 28, padding: 0 }}
        />
      </Tooltip>
      <Tooltip title="Add cell">
        <Button
          icon={<PlusOutlined />}
          size="small"
          onClick={onAddCell}
          style={{ width: 28, padding: 0 }}
        />
      </Tooltip>
      <Tooltip title="Deploy this notebook to Snowflake">
        <Button
          icon={<CloudUploadOutlined />}
          size="small"
          onClick={onDeploy}
          style={{ width: 28, padding: 0 }}
        />
      </Tooltip>

      {/* Kernel status */}
      {kernelStarting && <Spin size="small" />}
      {kernelStarting && <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>Starting kernel\u2026</Text>}
      {kernelReady && !kernelStarting && (
        <Tag color="success" style={{ fontSize: 10 }}>Kernel ready</Tag>
      )}
      {kernelError && (
        <Tag color="error" style={{ fontSize: 10 }} title={kernelError}>Kernel error</Tag>
      )}
    </Space>
  );
}
