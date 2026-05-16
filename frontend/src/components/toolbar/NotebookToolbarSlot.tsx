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

import type { ReactNode } from "react";
import { Button, Spin, Tooltip, Typography } from "antd";
import {
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
  onRestartKernel: () => void;
  onAddCell: () => void;
  onDeploy: () => void;
}

/**
 * Returns the 3 notebook action buttons (bare elements, no wrapper) to be
 * placed as the second row of the toolbar's 3-column button grid.
 */
export function notebookButtons(props: NotebookToolbarSlotProps): ReactNode {
  const { onRestartKernel, onAddCell, onDeploy } = props;
  return (
    <>
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
      <Tooltip title="Deploy notebook to Snowflake">
        <Button
          icon={<CloudUploadOutlined />}
          size="small"
          onClick={onDeploy}
          style={{ width: 28, padding: 0 }}
        />
      </Tooltip>
    </>
  );
}

/** Returns a compact kernel status indicator (icon or spinner). */
export function notebookStatus(props: NotebookToolbarSlotProps): ReactNode {
  const { kernelReady, kernelStarting, kernelError } = props;
  return (
    <div style={{ display: "flex", alignItems: "center", gap: 4, marginLeft: 4 }}>
      {kernelStarting && <Spin size="small" />}
      {kernelStarting && <Text style={{ fontSize: 10, color: "var(--text-muted)" }}>Starting&hellip;</Text>}
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
  );
}
