// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useEffect } from "react";
import { Button, Typography, Alert, Collapse, Space, Badge, Tooltip } from "antd";
import {
  FolderOpenOutlined,
  ReloadOutlined,
  BranchesOutlined,
} from "@ant-design/icons";
import { useGitStore } from "../../store/gitStore";

const { Text } = Typography;

const CLR_BORDER    = "var(--border)";
const CLR_SECONDARY = "var(--text-muted)";
const CLR_MODIFIED  = "#d29922";

export default function GitPanel() {
  const {
    exportDir,
    configLoaded, status, loading, error,
    loadConfig, pickExportDir, refreshStatus, clearError,
    openGitOps,
  } = useGitStore();

  useEffect(() => {
    if (!configLoaded) {
      loadConfig();
    }
  }, []);

  // Use the exact total from the backend (may exceed the capped array lengths).
  const totalChanged = status?.totalChanged ?? 0;

  const headerLabel = (() => {
    if (!status?.isRepo) return "Git";
    const b = status.branch || "main";
    return status.ahead > 0 ? `Git · ${b} (↑${status.ahead})` : `Git · ${b}`;
  })();

  return (
    <div style={{ borderTop: `1px solid ${CLR_BORDER}`, marginTop: 8 }}>
      <Collapse
        ghost
        defaultActiveKey={[]}
        style={{ background: "transparent" }}
        items={[{
          key: "git",
          label: (
            <Space size={6}>
              <BranchesOutlined style={{ color: "var(--text)", fontSize: 13 }} />
              <Text style={{ fontSize: 11, color: "var(--text)", textTransform: "uppercase", letterSpacing: "0.08em" }}>
                {headerLabel}
              </Text>
              {totalChanged > 0 && (
                <Badge
                  count={totalChanged}
                  size="small"
                  style={{ backgroundColor: CLR_MODIFIED, fontSize: 10 }}
                />
              )}
            </Space>
          ),
          style: { border: "none" },
          children: (
            <div style={{ display: "flex", flexDirection: "column", gap: 8, padding: "0 4px 8px" }}>

              {/* ── Export / repository directory ────────────────── */}
              <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Export directory</Text>
              <div style={{ display: "flex", gap: 4, alignItems: "center" }}>
                <Text
                  style={{
                    flex: 1, fontSize: 11, fontFamily: "monospace",
                    color: exportDir ? "var(--text)" : CLR_SECONDARY,
                    overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
                  }}
                  title={exportDir}
                >
                  {exportDir || "No directory selected"}
                </Text>
                <Tooltip title="Pick directory">
                  <Button size="small" icon={<FolderOpenOutlined />} onClick={pickExportDir} />
                </Tooltip>
                {exportDir && (
                  <Tooltip title="Refresh">
                    <Button
                      size="small"
                      icon={<ReloadOutlined spin={loading} />}
                      onClick={refreshStatus}
                      disabled={loading}
                    />
                  </Tooltip>
                )}
              </div>

              {/* ── Status summary ───────────────────────────────── */}
              {exportDir && status && !loading && (
                <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>
                  {!status.isRepo
                    ? "Not a git repository — will be initialised on push."
                    : totalChanged === 0
                      ? `Working tree clean.${status.ahead > 0 ? ` ${status.ahead} commit(s) not yet pushed.` : ""}`
                      : `${totalChanged.toLocaleString()} changed file${totalChanged !== 1 ? "s" : ""}.`}
                </Text>
              )}

              {/* ── Git Operations button ────────────────────────── */}
              {exportDir && (
                <Button
                  size="small"
                  type="default"
                  onClick={openGitOps}
                  style={{ marginTop: 2 }}
                >
                  Git Operations…
                </Button>
              )}

              {/* ── Error ──────────────────────────────────────────── */}
              {error && (
                <Alert
                  type="error"
                  message={error}
                  showIcon
                  closable
                  onClose={clearError}
                  style={{ fontSize: 11 }}
                />
              )}
            </div>
          ),
        }]}
      />
    </div>
  );
}
