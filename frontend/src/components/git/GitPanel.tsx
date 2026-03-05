// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useEffect } from "react";
import { Button, Input, Typography, Spin, Alert, Badge, Collapse, Space, Tooltip } from "antd";
import {
  FolderOpenOutlined,
  ReloadOutlined,
  BranchesOutlined,
  CloudUploadOutlined,
  CloudDownloadOutlined,
} from "@ant-design/icons";
import { useGitStore } from "../../store/gitStore";
import CommitModal from "./CommitModal";

const { Text } = Typography;

const CLR_BORDER    = "var(--border)";
const CLR_SECONDARY = "var(--text-muted)";
const CLR_ADDED     = "#3fb950";
const CLR_MODIFIED  = "#d29922";
const CLR_DELETED   = "#f85149";

export default function GitPanel() {
  const {
    exportDir, remoteURL, branch, authorName, authorEmail,
    configLoaded, status, loading, pushing, pulling, error,
    loadConfig, saveConfig, pickExportDir, refreshStatus, push, pull, clearError,
  } = useGitStore();

  // Token is ephemeral — shared between pull and the commit modal
  const [token, setToken] = useState("");
  const [commitOpen, setCommitOpen] = useState(false);

  useEffect(() => {
    if (!configLoaded) {
      loadConfig();
    }
  }, []);

  const totalChanged =
    (status?.modified?.length ?? 0) +
    (status?.added?.length   ?? 0) +
    (status?.deleted?.length ?? 0);

  const handlePull = async () => {
    await pull({ token });
  };

  const handleCommit = async (files: string[], message: string, commitToken: string) => {
    await push({ token: commitToken, message, files });
    if (!useGitStore.getState().error) {
      setCommitOpen(false);
    }
  };

  const headerLabel = (() => {
    if (!status?.isRepo) return "Git";
    const b = branch || "main";
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

              {/* ── Status ──────────────────────────────────────── */}
              {exportDir && loading && (
                <Spin size="small" style={{ display: "block", margin: "4px auto" }} />
              )}

              {exportDir && status && !loading && (
                <>
                  {!status.isRepo && (
                    <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>
                      Not a git repository — will be initialised on push.
                    </Text>
                  )}
                  {status.isRepo && totalChanged === 0 && (
                    <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>
                      Working tree clean.
                      {status.ahead > 0 && ` ${status.ahead} commit(s) not yet pushed.`}
                    </Text>
                  )}
                  {totalChanged > 0 && (
                    <div style={{ maxHeight: 140, overflowY: "auto", display: "flex", flexDirection: "column", gap: 2 }}>
                      {status.added?.map((f) => (
                        <Text key={f} style={{ fontSize: 11, fontFamily: "monospace", color: CLR_ADDED }}>+ {f}</Text>
                      ))}
                      {status.modified?.map((f) => (
                        <Text key={f} style={{ fontSize: 11, fontFamily: "monospace", color: CLR_MODIFIED }}>~ {f}</Text>
                      ))}
                      {status.deleted?.map((f) => (
                        <Text key={f} style={{ fontSize: 11, fontFamily: "monospace", color: CLR_DELETED }}>- {f}</Text>
                      ))}
                    </div>
                  )}
                </>
              )}

              {/* ── Remote + Branch + Auth ───────────────────────── */}
              {exportDir && (
                <>
                  <Input
                    size="small"
                    placeholder="https://github.com/org/repo.git"
                    value={remoteURL}
                    onChange={(e) => saveConfig({ remoteURL: e.target.value })}
                    style={{ fontSize: 12 }}
                    addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Remote</Text>}
                  />
                  <Input
                    size="small"
                    placeholder="main"
                    value={branch}
                    onChange={(e) => saveConfig({ branch: e.target.value })}
                    style={{ fontSize: 12 }}
                    addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Branch</Text>}
                  />

                  <Collapse
                    ghost
                    size="small"
                    style={{ background: "transparent", marginLeft: -8 }}
                    items={[{
                      key: "auth",
                      label: <Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Author & token</Text>,
                      style: { border: "none" },
                      children: (
                        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
                          <Input
                            size="small"
                            placeholder="Your Name"
                            value={authorName}
                            onChange={(e) => saveConfig({ authorName: e.target.value })}
                            style={{ fontSize: 12 }}
                            addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Name</Text>}
                          />
                          <Input
                            size="small"
                            placeholder="you@example.com"
                            value={authorEmail}
                            onChange={(e) => saveConfig({ authorEmail: e.target.value })}
                            style={{ fontSize: 12 }}
                            addonBefore={<Text style={{ fontSize: 11, color: CLR_SECONDARY }}>Email</Text>}
                          />
                          <Input.Password
                            size="small"
                            placeholder="GitHub PAT (ghp_…) — not saved"
                            value={token}
                            onChange={(e) => setToken(e.target.value)}
                            style={{ fontSize: 12 }}
                            visibilityToggle
                          />
                        </div>
                      ),
                    }]}
                  />

                  {/* ── Action buttons ──────────────────────────── */}
                  <Space.Compact block>
                    <Button
                      size="small"
                      icon={<CloudDownloadOutlined />}
                      loading={pulling}
                      disabled={loading || pushing || !status?.isRepo}
                      onClick={handlePull}
                      style={{ width: "40%" }}
                    >
                      {pulling ? "Pulling…" : "Pull"}
                    </Button>
                    <Button
                      type="primary"
                      size="small"
                      icon={<CloudUploadOutlined />}
                      loading={pushing}
                      disabled={loading || pulling}
                      onClick={() => setCommitOpen(true)}
                      style={{ width: "60%" }}
                    >
                      {pushing ? "Pushing…" : "Commit & Push"}
                    </Button>
                  </Space.Compact>
                </>
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

      {/* ── Commit modal ───────────────────────────────────────── */}
      {commitOpen && status && (
        <CommitModal
          status={status}
          pushing={pushing}
          onClose={() => setCommitOpen(false)}
          onCommit={handleCommit}
        />
      )}
    </div>
  );
}
