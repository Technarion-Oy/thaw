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

import { Button, Spin, Tooltip, Typography } from "antd";
import {
  PlayCircleOutlined,
  ReloadOutlined,
  PlusOutlined,
  CloudUploadOutlined,
  WarningOutlined,
  CheckCircleOutlined,
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
    <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
      {/* Separator */}
      <div style={{ width: 1, height: 20, background: "var(--border)" }} />

      {/* Notebook action buttons — vertical column */}
      <div style={{ display: "flex", flexDirection: "column", gap: 2 }}>
        <Tooltip title="Run all cells" placement="right">
          <Button
            icon={<PlayCircleOutlined />}
            size="small"
            onClick={onRunAll}
            disabled={!kernelReady}
            style={{ width: 28, padding: 0 }}
          />
        </Tooltip>
        <Tooltip title="Restart kernel" placement="right">
          <Button
            icon={<ReloadOutlined />}
            size="small"
            onClick={onRestartKernel}
            style={{ width: 28, padding: 0 }}
          />
        </Tooltip>
        <Tooltip title="Add cell" placement="right">
          <Button
            icon={<PlusOutlined />}
            size="small"
            onClick={onAddCell}
            style={{ width: 28, padding: 0 }}
          />
        </Tooltip>
        <Tooltip title="Deploy this notebook to Snowflake" placement="right">
          <Button
            icon={<CloudUploadOutlined />}
            size="small"
            onClick={onDeploy}
            style={{ width: 28, padding: 0 }}
          />
        </Tooltip>
      </div>

      {/* Kernel status */}
      <div style={{ display: "flex", flexDirection: "column", alignItems: "center", gap: 2 }}>
        {kernelStarting && <Spin size="small" />}
        {kernelStarting && <Text style={{ fontSize: 10, color: "var(--text-muted)" }}>Starting\u2026</Text>}
        {kernelReady && !kernelStarting && (
          <Tooltip title="Kernel ready">
            <CheckCircleOutlined style={{ color: "#52c41a", fontSize: 14 }} />
          </Tooltip>
        )}
        {kernelError && (
          <Tooltip title={kernelError}>
            <WarningOutlined style={{ color: "#ff4d4f", fontSize: 14 }} />
          </Tooltip>
        )}
      </div>
    </div>
  );
}
