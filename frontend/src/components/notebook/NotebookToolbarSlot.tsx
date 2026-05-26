// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import type { ReactNode } from "react";
import { Button, Spin, Tooltip } from "antd";
import { ReloadOutlined, CloudUploadOutlined, WarningOutlined } from "@ant-design/icons";

export interface NotebookToolbarSlotProps {
  kernelReady: boolean;
  kernelStarting: boolean;
  kernelError: string | null;
  /** Optional kernel name shown in the title tooltip (e.g. "Python 3.11"). */
  kernelName?: string;
  onRestartKernel: () => void;
  onDeploy: () => void;
}

/**
 * Full notebook section for the app toolbar.
 *
 * Layout: [kernel dot] [Restart icon-btn] [Deploy primary-btn]
 *
 * The "+ Cell" action moved inline (hover-reveal bars between cells in
 * NotebookTab.tsx), so it's NOT included here.
 */
export function NotebookToolbarSlot(props: NotebookToolbarSlotProps): ReactNode {
  const { kernelReady, kernelStarting, kernelError, kernelName, onRestartKernel, onDeploy } = props;
  return (
    <>
      <KernelDot
        ready={kernelReady}
        starting={kernelStarting}
        error={kernelError}
        name={kernelName ?? "Python 3.11"}
      />

      <Tooltip title="Restart kernel">
        <Button className="thaw-tb-icon-btn" aria-label="Restart kernel"
          icon={<ReloadOutlined />} onClick={onRestartKernel} />
      </Tooltip>

      <Tooltip title="Deploy notebook to Snowflake">
        <Button className="thaw-tb-primary-btn" aria-label="Deploy notebook"
          icon={<CloudUploadOutlined />} onClick={onDeploy}>Deploy</Button>
      </Tooltip>
    </>
  );
}

function KernelDot({ ready, starting, error, name }: {
  ready: boolean; starting: boolean; error: string | null; name: string;
}) {
  if (starting) return (
    <span className="thaw-kernel-dot starting"
          title={`Starting ${name} kernel…`}
          aria-label="Kernel starting">
      <Spin size="small" />
    </span>
  );
  if (error) return (
    <span className="thaw-kernel-dot error"
          title={`Kernel error — ${error}`}
          aria-label="Kernel error">
      <WarningOutlined />
    </span>
  );
  if (ready) return (
    <span className="thaw-kernel-dot ready"
          title={`${name} · kernel ready`}
          aria-label="Kernel ready">
      <span className="dot" />
    </span>
  );
  return null;
}
