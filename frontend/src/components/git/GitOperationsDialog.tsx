// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import React, { useState, useMemo, useEffect } from "react";
import {
  Modal, Button, Input, Checkbox, Space, Tag, Typography, Divider, Alert,
  List, Badge, Tooltip, Popconfirm, message as antMessage,
} from "antd";
import {
  CloudUploadOutlined, CloudDownloadOutlined, CheckSquareOutlined,
  CloseSquareOutlined, FolderOpenOutlined, ReloadOutlined,
  BranchesOutlined, PlusOutlined, GithubOutlined, WarningOutlined,
  EditOutlined, CheckOutlined, CloseOutlined, MergeOutlined,
} from "@ant-design/icons";
import { useGitStore } from "../../store/gitStore";
import { PickDirectory, GitInitWithRemote } from "../../../wailsjs/go/main/App";

const { Text } = Typography;
const { TextArea } = Input;

const CLR_ADDED    = "#3fb950";
const CLR_MODIFIED = "#d29922";
const CLR_DELETED  = "#f85149";

// ── Helpers ───────────────────────────────────────────────────────────────────

function isGithubURL(url: string): boolean {
  return url.includes("github.com");
}

function extOf(path: string): string {
  const dot = path.lastIndexOf(".");
  return dot >= 0 ? path.slice(dot).toLowerCase() : "(no ext)";
}

// ── Section heading ───────────────────────────────────────────────────────────

function SectionTitle({ children }: { children: React.ReactNode }) {
  return (
    <Text style={{ fontSize: 10, fontWeight: 600, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
      {children}
    </Text>
  );
}

// ── Virtual file list ─────────────────────────────────────────────────────────

const ROW_HEIGHT    = 24;
const LIST_HEIGHT   = 200;
const SCROLL_BUFFER = 8;

type FileEntry = { path: string; change: "added" | "modified" | "deleted" };

function VirtualFileList({
  files, selected, onToggle,
}: {
  files: FileEntry[];
  selected: Set<string>;
  onToggle: (path: string) => void;
}) {
  const [scrollTop, setScrollTop] = useState(0);

  const startIndex  = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - SCROLL_BUFFER);
  const visibleRows = Math.ceil(LIST_HEIGHT / ROW_HEIGHT) + SCROLL_BUFFER * 2;
  const endIndex    = Math.min(files.length, startIndex + visibleRows);
  const topPad      = startIndex * ROW_HEIGHT;
  const bottomPad   = Math.max(0, (files.length - endIndex) * ROW_HEIGHT);

  const clrOf    = (c: FileEntry["change"]) => c === "added" ? CLR_ADDED : c === "modified" ? CLR_MODIFIED : CLR_DELETED;
  const prefixOf = (c: FileEntry["change"]) => c === "added" ? "+" : c === "modified" ? "~" : "-";

  if (files.length === 0) return null;

  return (
    <div
      style={{ height: LIST_HEIGHT, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 6, background: "var(--bg)" }}
      onScroll={(e) => setScrollTop(e.currentTarget.scrollTop)}
    >
      {topPad > 0 && <div style={{ height: topPad }} />}
      {files.slice(startIndex, endIndex).map(({ path, change }) => (
        <div
          key={path}
          style={{ display: "flex", alignItems: "center", gap: 8, padding: "3px 10px", height: ROW_HEIGHT, cursor: "pointer" }}
          onClick={() => onToggle(path)}
        >
          <Checkbox checked={selected.has(path)} onChange={() => onToggle(path)} onClick={(e) => e.stopPropagation()} />
          <Text style={{ fontFamily: "monospace", fontSize: 12, color: clrOf(change), flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
            {prefixOf(change)} {path}
          </Text>
        </div>
      ))}
      {bottomPad > 0 && <div style={{ height: bottomPad }} />}
    </div>
  );
}

// ── Section 1: Repository ─────────────────────────────────────────────────────

function RepositorySection() {
  const { exportDir, status, loading, pickExportDir, refreshStatus, clone, cloning, updateRemoteURL, error, clearError } = useGitStore();
  const [cloneUrl, setCloneUrl]   = useState("");
  const [clonePath, setClonePath] = useState("");
  const [cloneSuccess, setCloneSuccess] = useState(false);
  const [cloneUrlError, setCloneUrlError] = useState("");

  // Init-instead-of-clone state (shown when remote is an empty repository)
  const [initMode, setInitMode]       = useState(false);
  const [branchName, setBranchName]   = useState("main");
  const [initLoading, setInitLoading] = useState(false);
  const [initSuccess, setInitSuccess] = useState(false);

  // Remote URL editing
  const [editingRemote, setEditingRemote] = useState(false);
  const [remoteInput, setRemoteInput]     = useState(status?.remoteURL ?? "");
  const [remoteUrlError, setRemoteUrlError] = useState("");

  useEffect(() => {
    setRemoteInput(status?.remoteURL ?? "");
  }, [status?.remoteURL]);

  const handlePickClonePath = async () => {
    const dir = await PickDirectory();
    if (dir) setClonePath(dir);
  };

  const handleCloneUrlChange = (v: string) => {
    setCloneUrl(v);
    setCloneUrlError(v && !isGithubURL(v) ? "Only GitHub repositories are supported." : "");
  };

  const handleClone = async () => {
    if (!isGithubURL(cloneUrl)) {
      setCloneUrlError("Only GitHub repositories are supported.");
      return;
    }
    setCloneSuccess(false);
    setInitSuccess(false);
    clearError();
    try {
      const { oauthToken } = useGitStore.getState();
      const targetPath = clonePath || exportDir || "";
      await clone({ url: cloneUrl, path: targetPath, authMethod: "oauth", token: oauthToken } as any);
      if (!useGitStore.getState().error) {
        setCloneSuccess(true);
        setCloneUrl("");
        setClonePath("");
      }
    } catch (e) {
      // If the remote is empty, offer to initialize instead.
      if (String(e).includes("remote repository is empty")) {
        clearError();
        setInitMode(true);
      }
    }
  };

  const handleInit = async () => {
    setInitLoading(true);
    setInitSuccess(false);
    clearError();
    try {
      const targetPath = clonePath || exportDir || "";
      await GitInitWithRemote(targetPath, cloneUrl, branchName || "main");
      setInitSuccess(true);
      setInitMode(false);
      setCloneUrl("");
      setClonePath("");
      await refreshStatus();
    } catch (e) {
      useGitStore.setState({ error: String(e) });
    } finally {
      setInitLoading(false);
    }
  };

  const handleCancelInit = () => {
    setInitMode(false);
    clearError();
  };

  const handleRemoteUrlChange = (v: string) => {
    setRemoteInput(v);
    setRemoteUrlError(v && !isGithubURL(v) ? "Only GitHub repositories are supported." : "");
  };

  const handleSaveRemote = async () => {
    if (!isGithubURL(remoteInput)) {
      setRemoteUrlError("Only GitHub repositories are supported.");
      return;
    }
    await updateRemoteURL(remoteInput);
    if (!useGitStore.getState().error) setEditingRemote(false);
  };

  const isRepo = status?.isRepo;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <SectionTitle>Repository</SectionTitle>

      {/* Export directory */}
      <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
        <Text style={{ fontSize: 12, color: "var(--text-muted)", whiteSpace: "nowrap" }}>Local path</Text>
        <Text
          style={{ flex: 1, fontSize: 12, fontFamily: "monospace", color: exportDir ? "var(--text)" : "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
          title={exportDir}
        >
          {exportDir || "No directory selected"}
        </Text>
        <Tooltip title="Pick directory">
          <Button size="small" icon={<FolderOpenOutlined />} onClick={pickExportDir} />
        </Tooltip>
        {exportDir && (
          <Tooltip title="Refresh status">
            <Button size="small" icon={<ReloadOutlined spin={loading} />} onClick={refreshStatus} disabled={loading} />
          </Tooltip>
        )}
      </div>

      {/* If repo: show remote URL + edit option */}
      {isRepo && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
            <Text style={{ fontSize: 12, color: "var(--text-muted)", whiteSpace: "nowrap" }}>Remote URL</Text>
            {editingRemote ? (
              <>
                <Input
                  size="small"
                  value={remoteInput}
                  onChange={(e) => handleRemoteUrlChange(e.target.value)}
                  style={{ flex: 1, fontSize: 12 }}
                  status={remoteUrlError ? "error" : ""}
                  addonBefore={<GithubOutlined />}
                />
                <Tooltip title="Save">
                  <Button size="small" icon={<CheckOutlined />} type="primary" onClick={handleSaveRemote} />
                </Tooltip>
                <Tooltip title="Cancel">
                  <Button size="small" icon={<CloseOutlined />} onClick={() => { setEditingRemote(false); setRemoteInput(status?.remoteURL ?? ""); setRemoteUrlError(""); }} />
                </Tooltip>
              </>
            ) : (
              <>
                <Text style={{ flex: 1, fontSize: 12, fontFamily: "monospace", color: status?.remoteURL ? "var(--text)" : "var(--text-muted)", overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>
                  {status?.remoteURL || "(none configured)"}
                </Text>
                <Tooltip title="Update remote URL">
                  <Button size="small" icon={<EditOutlined />} onClick={() => setEditingRemote(true)} />
                </Tooltip>
              </>
            )}
          </div>
          {remoteUrlError && <Text style={{ fontSize: 11, color: CLR_DELETED }}>{remoteUrlError}</Text>}
        </div>
      )}

      {/* If no repo or no export dir: show clone / init form */}
      {exportDir && !isRepo && (
        <div style={{ display: "flex", flexDirection: "column", gap: 8, padding: "10px 12px", background: "var(--bg-subtle, var(--bg))", border: "1px solid var(--border)", borderRadius: 8 }}>
          {initMode ? (
            /* ── Initialize empty repository ── */
            <>
              <Alert
                type="warning"
                showIcon
                style={{ fontSize: 12 }}
                message="Remote repository is empty"
                description="The repository has no commits yet. You can initialize a local repository, set this URL as origin, and push your first commit."
              />
              {error && <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />}
              {initSuccess && <Alert type="success" message="Repository initialized successfully." showIcon style={{ fontSize: 12 }} />}

              <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
                <Text style={{ fontSize: 12, color: "var(--text-muted)", whiteSpace: "nowrap" }}>Default branch</Text>
                <Input
                  size="small"
                  value={branchName}
                  onChange={(e) => setBranchName(e.target.value)}
                  placeholder="main"
                  style={{ fontSize: 12, flex: 1 }}
                  addonBefore={<BranchesOutlined />}
                />
              </div>

              <div style={{ display: "flex", gap: 6 }}>
                <Button
                  type="primary"
                  size="small"
                  loading={initLoading}
                  disabled={!cloneUrl}
                  onClick={handleInit}
                  style={{ flex: 1 }}
                >
                  Initialize Repository
                </Button>
                <Button size="small" onClick={handleCancelInit}>Cancel</Button>
              </div>
            </>
          ) : (
            /* ── Clone existing repository ── */
            <>
              <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>No git repository found. Clone one to get started.</Text>

              {cloneSuccess && <Alert type="success" message="Repository cloned successfully." showIcon style={{ fontSize: 12 }} />}
              {error && <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />}

              <div style={{ display: "flex", gap: 6, alignItems: "flex-start", flexDirection: "column" }}>
                <Input
                  size="small"
                  placeholder="https://github.com/org/repo.git"
                  value={cloneUrl}
                  onChange={(e) => handleCloneUrlChange(e.target.value)}
                  style={{ fontSize: 12 }}
                  status={cloneUrlError ? "error" : ""}
                  addonBefore={<GithubOutlined style={{ color: cloneUrlError ? CLR_DELETED : undefined }} />}
                />
                {cloneUrlError && <Text style={{ fontSize: 11, color: CLR_DELETED }}>{cloneUrlError}</Text>}
              </div>

              <div style={{ display: "flex", gap: 6 }}>
                <Input
                  size="small"
                  placeholder={clonePath || "Clone into selected directory above"}
                  value={clonePath}
                  onChange={(e) => setClonePath(e.target.value)}
                  style={{ fontSize: 12, flex: 1 }}
                />
                <Tooltip title="Pick directory">
                  <Button size="small" icon={<FolderOpenOutlined />} onClick={handlePickClonePath} />
                </Tooltip>
              </div>

              <Button
                type="primary"
                size="small"
                loading={cloning}
                disabled={!cloneUrl || (!clonePath && !exportDir) || !!cloneUrlError}
                onClick={handleClone}
              >
                {cloning ? "Cloning…" : "Clone Repository"}
              </Button>
            </>
          )}
        </div>
      )}
    </div>
  );
}

// ── Section 2: GitHub Authentication ─────────────────────────────────────────

function AuthSection() {
  const { oauthToken, loginWithOAuth, setOAuthToken } = useGitStore();
  const [loading, setLoading] = useState(false);

  const handleConnect = async () => {
    setLoading(true);
    try {
      await loginWithOAuth("github");
      antMessage.success("Successfully authenticated with GitHub");
    } catch {
      // error in store
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <SectionTitle>GitHub Authentication</SectionTitle>
        <Tag color="default" style={{ fontSize: 10, lineHeight: "16px", padding: "0 5px" }}>
          <GithubOutlined /> GitHub only
        </Tag>
      </div>

      {oauthToken ? (
        <Alert
          type="success"
          showIcon
          style={{ fontSize: 12 }}
          message="Authenticated with GitHub. Token is held in memory only."
          action={
            <Button size="small" type="link" danger onClick={() => setOAuthToken("")}>
              Disconnect
            </Button>
          }
        />
      ) : (
        <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
          <Button icon={<GithubOutlined />} loading={loading} onClick={handleConnect} size="small">
            Connect to GitHub
          </Button>
          <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>
            Opens your browser. Token is kept in memory only.
          </Text>
        </div>
      )}
    </div>
  );
}

// ── Section 3: Working Tree ───────────────────────────────────────────────────

function WorkingTreeSection() {
  const {
    status, pushing, resetting, push, resetHard, clearError, error, oauthToken,
  } = useGitStore();

  const allFiles = useMemo<FileEntry[]>(() => {
    if (!status) return [];
    const files: FileEntry[] = [];
    for (const f of (status.added    ?? [])) files.push({ path: f, change: "added" });
    for (const f of (status.modified ?? [])) files.push({ path: f, change: "modified" });
    for (const f of (status.deleted  ?? [])) files.push({ path: f, change: "deleted" });
    return files;
  }, [status]);

  const [selected, setSelected]   = useState<Set<string>>(() => new Set(allFiles.map((f) => f.path)));
  const [commitMsg, setCommitMsg] = useState("");

  useEffect(() => {
    setSelected(new Set(allFiles.map((f) => f.path)));
  }, [allFiles]);

  const extensions = useMemo(() => [...new Set(allFiles.map((f) => extOf(f.path)))].sort(), [allFiles]);

  const toggle      = (path: string) => setSelected((prev) => { const n = new Set(prev); n.has(path) ? n.delete(path) : n.add(path); return n; });
  const selectAll   = () => setSelected(new Set(allFiles.map((f) => f.path)));
  const selectNone  = () => setSelected(new Set());
  const selectByExt = (ext: string) => setSelected(new Set(allFiles.filter((f) => extOf(f.path) === ext).map((f) => f.path)));

  const handleCommitPush = async () => {
    await push({ message: commitMsg, files: [...selected] });
    if (!useGitStore.getState().error) {
      setCommitMsg("");
    }
  };

  const totalChanged = status?.totalChanged ?? 0;
  const showing      = allFiles.length;
  const truncated    = totalChanged > showing;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <SectionTitle>Working Tree</SectionTitle>
        {totalChanged > 0 && (
          <Popconfirm
            title="Discard all uncommitted changes?"
            description="This will reset the working tree to the last commit. This cannot be undone."
            onConfirm={resetHard}
            okText="Discard Changes"
            okButtonProps={{ danger: true }}
            cancelText="Cancel"
            icon={<WarningOutlined style={{ color: CLR_DELETED }} />}
          >
            <Button size="small" danger icon={<WarningOutlined />} loading={resetting}>
              Discard Changes
            </Button>
          </Popconfirm>
        )}
      </div>

      {error && (
        <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />
      )}

      {/* File toolbar */}
      {allFiles.length > 0 && (
        <Space wrap size={4}>
          <Button size="small" icon={<CheckSquareOutlined />} onClick={selectAll}>All</Button>
          <Button size="small" icon={<CloseSquareOutlined />} onClick={selectNone} disabled={selected.size === 0}>None</Button>
          {extensions.length > 1 && (
            <>
              <Divider type="vertical" style={{ borderColor: "var(--border)" }} />
              {extensions.map((ext) => (
                <Tag key={ext} style={{ cursor: "pointer", userSelect: "none", fontSize: 11 }} onClick={() => selectByExt(ext)}>
                  {ext}
                </Tag>
              ))}
            </>
          )}
        </Space>
      )}

      {totalChanged > 0 && (
        <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
          {truncated
            ? `Showing ${showing.toLocaleString()} of ${totalChanged.toLocaleString()} changed files`
            : `${totalChanged.toLocaleString()} changed file${totalChanged !== 1 ? "s" : ""}`}
        </Text>
      )}

      {allFiles.length === 0 ? (
        <div style={{ padding: "8px 12px", border: "1px solid var(--border)", borderRadius: 6, background: "var(--bg)" }}>
          <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>
            {status?.isRepo ? "Working tree clean." : "Not a git repository — will be initialised on push."}
          </Text>
        </div>
      ) : (
        <VirtualFileList files={allFiles} selected={selected} onToggle={toggle} />
      )}

      <TextArea
        size="small"
        rows={2}
        placeholder="Commit message (default: chore: export Snowflake DDL)"
        value={commitMsg}
        onChange={(e) => setCommitMsg(e.target.value)}
        style={{ fontSize: 12, resize: "none" }}
      />

      <div style={{ display: "flex", gap: 6 }}>
        <Button
          type="primary"
          icon={<CloudUploadOutlined />}
          loading={pushing}
          disabled={selected.size === 0 || !oauthToken}
          onClick={handleCommitPush}
          style={{ flex: 1 }}
        >
          {pushing ? "Pushing…" : `Commit & Push (${selected.size.toLocaleString()} file${selected.size !== 1 ? "s" : ""})`}
        </Button>
      </div>

      {!oauthToken && (
        <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
          Connect to GitHub above to enable push.
        </Text>
      )}
    </div>
  );
}

// ── Section 4: Branches ───────────────────────────────────────────────────────

function BranchesSection() {
  const {
    branches, listBranches, checkoutBranch, checkoutRemoteBranch, createBranch,
    deleteBranch, deleteRemoteBranch, pushBranch, pullBranch, mergeBranch,
    exportDir, error, clearError, oauthToken,
  } = useGitStore();

  // Set of local branch names — used to grey-out Checkout on remote branches
  // that already have a local counterpart.
  const localBranchNames = useMemo(
    () => new Set(branches.filter((b) => !b.isRemote).map((b) => b.name)),
    [branches],
  );

  const [newBranch, setNewBranch]         = useState("");
  const [actionLoading, setActionLoading] = useState<string | null>(null); // branch name currently acting on

  useEffect(() => {
    if (exportDir) listBranches();
  }, [exportDir]);

  const act = async (key: string, fn: () => Promise<void>) => {
    setActionLoading(key);
    await fn();
    setActionLoading(null);
  };

  const handleCreate = async () => {
    if (!newBranch.trim()) return;
    await act("__create__", () => createBranch(newBranch.trim()));
    if (!useGitStore.getState().error) setNewBranch("");
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <SectionTitle>Branches</SectionTitle>
        <Tooltip title="Refresh">
          <Button size="small" icon={<ReloadOutlined />} onClick={listBranches} />
        </Tooltip>
      </div>

      {error && (
        <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />
      )}

      <div style={{ maxHeight: 240, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 6, background: "var(--bg)" }}>
        {branches.length === 0 && (
          <Text style={{ display: "block", padding: "10px 14px", color: "var(--text-muted)", fontSize: 12 }}>
            No branches found.
          </Text>
        )}
        <List
          size="small"
          dataSource={branches}
          renderItem={(b) => {
            const isLoading = (k: string) => actionLoading === `${b.name}:${k}`;
            return (
              <List.Item style={{ padding: "4px 10px" }}>
                <div style={{ display: "flex", alignItems: "center", gap: 8, width: "100%" }}>
                  <BranchesOutlined style={{ color: b.isCurrent ? "var(--accent)" : "var(--text-muted)", fontSize: 12, flexShrink: 0 }} />
                  <Text
                    style={{ fontSize: 12, fontFamily: "monospace", color: b.isCurrent ? "var(--accent)" : "var(--text)", fontWeight: b.isCurrent ? 600 : 400, flex: 1, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}
                  >
                    {b.name}
                  </Text>
                  {b.isRemote && <Badge color="blue" text={<Text style={{ fontSize: 10, color: "var(--text-muted)" }}>remote</Text>} />}
                  {b.isCurrent && <Badge color="green" text={<Text style={{ fontSize: 10, color: "var(--text-muted)" }}>current</Text>} />}

                  {/* Per-branch actions */}
                  <Space size={4} style={{ flexShrink: 0 }}>
                    {/* Checkout remote → local */}
                    {b.isRemote && (() => {
                      const localName = b.name.slice(b.name.indexOf("/") + 1);
                      const alreadyLocal = localBranchNames.has(localName);
                      return (
                        <Tooltip title={alreadyLocal ? `Local branch "${localName}" already exists` : `Check out ${b.name} as a local branch`}>
                          <Button
                            size="small"
                            loading={isLoading("checkout")}
                            disabled={alreadyLocal}
                            onClick={() => act(`${b.name}:checkout`, () => checkoutRemoteBranch(b.name))}
                          >
                            Checkout
                          </Button>
                        </Tooltip>
                      );
                    })()}

                    {/* Delete remote branch */}
                    {b.isRemote && (
                      <Popconfirm
                        title={`Delete remote branch "${b.name}"?`}
                        description="This permanently deletes the branch on GitHub. Local branches are unaffected."
                        onConfirm={() => act(`${b.name}:deleteRemote`, () => deleteRemoteBranch(b.name))}
                        okText="Delete Remote"
                        okButtonProps={{ danger: true }}
                        cancelText="Cancel"
                        disabled={!oauthToken}
                      >
                        <Tooltip title={oauthToken ? `Delete ${b.name} on remote` : "Connect to GitHub first"}>
                          <Button
                            size="small"
                            danger
                            loading={isLoading("deleteRemote")}
                            disabled={!oauthToken}
                          >
                            Delete
                          </Button>
                        </Tooltip>
                      </Popconfirm>
                    )}

                    {/* Switch — only local, non-current */}
                    {!b.isRemote && !b.isCurrent && (
                      <>
                        <Tooltip title={`Merge ${b.name} into current branch`}>
                          <Button
                            size="small"
                            icon={<MergeOutlined />}
                            loading={isLoading("merge")}
                            onClick={() => act(`${b.name}:merge`, () => mergeBranch(b.name))}
                          />
                        </Tooltip>
                        <Button
                          size="small"
                          loading={isLoading("switch")}
                          onClick={() => act(`${b.name}:switch`, () => checkoutBranch(b.name))}
                        >
                          Switch
                        </Button>
                      </>
                    )}

                    {/* Push */}
                    {!b.isRemote && (
                      <Tooltip title={oauthToken ? `Push ${b.name}` : "Connect to GitHub first"}>
                        <Button
                          size="small"
                          icon={<CloudUploadOutlined />}
                          loading={isLoading("push")}
                          disabled={!oauthToken}
                          onClick={() => act(`${b.name}:push`, () => pushBranch(b.name))}
                        />
                      </Tooltip>
                    )}

                    {/* Pull */}
                    {!b.isRemote && (
                      <Tooltip title={oauthToken ? `Pull ${b.name}` : "Connect to GitHub first"}>
                        <Button
                          size="small"
                          icon={<CloudDownloadOutlined />}
                          loading={isLoading("pull")}
                          disabled={!oauthToken}
                          onClick={() => act(`${b.name}:pull`, () => pullBranch(b.name))}
                        />
                      </Tooltip>
                    )}

                    {/* Delete — only local, non-current */}
                    {!b.isRemote && !b.isCurrent && (
                      <Popconfirm
                        title={`Delete branch "${b.name}"?`}
                        description="This deletes the local branch only. Remote branches are unaffected."
                        onConfirm={() => act(`${b.name}:delete`, () => deleteBranch(b.name))}
                        okText="Delete"
                        okButtonProps={{ danger: true }}
                        cancelText="Cancel"
                      >
                        <Button size="small" danger loading={isLoading("delete")}>
                          Delete
                        </Button>
                      </Popconfirm>
                    )}
                  </Space>
                </div>
              </List.Item>
            );
          }}
        />
      </div>

      {/* Create branch */}
      <div style={{ display: "flex", gap: 6 }}>
        <Input
          size="small"
          placeholder="new-branch-name"
          value={newBranch}
          onChange={(e) => setNewBranch(e.target.value)}
          onPressEnter={handleCreate}
          style={{ fontSize: 12, flex: 1 }}
        />
        <Button
          size="small"
          type="primary"
          icon={<PlusOutlined />}
          disabled={!newBranch.trim() || actionLoading === "__create__"}
          loading={actionLoading === "__create__"}
          onClick={handleCreate}
        >
          Create
        </Button>
      </div>
    </div>
  );
}

// ── Main Dialog ───────────────────────────────────────────────────────────────

export default function GitOperationsDialog() {
  const { gitOpsOpen, closeGitOps, status, exportDir } = useGitStore();
  const isRepo = status?.isRepo ?? false;

  return (
    <Modal
      open={gitOpsOpen}
      onCancel={closeGitOps}
      title={
        <Space size={8}>
          <GithubOutlined />
          <span>Git Operations</span>
          <Tag color="default" style={{ fontSize: 10, lineHeight: "18px", padding: "0 6px", marginLeft: 2 }}>
            GitHub only
          </Tag>
        </Space>
      }
      width={620}
      footer={null}
      styles={{ body: { padding: "12px 16px 16px", maxHeight: "80vh", overflowY: "auto" } }}
      destroyOnClose={false}
    >
      <div style={{ display: "flex", flexDirection: "column", gap: 18 }}>

        {/* Section 1: Repository */}
        <RepositorySection />

        <Divider style={{ borderColor: "var(--border)", margin: "0" }} />

        {/* Section 2: GitHub Authentication */}
        <AuthSection />

        {/* Section 3 & 4 only when we have a repo */}
        {exportDir && isRepo && (
          <>
            <Divider style={{ borderColor: "var(--border)", margin: "0" }} />
            <WorkingTreeSection />
            <Divider style={{ borderColor: "var(--border)", margin: "0" }} />
            <BranchesSection />
          </>
        )}
      </div>
    </Modal>
  );
}
