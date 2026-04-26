// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useMemo, useEffect } from "react";
import {
  Modal, Tabs, Button, Input, Checkbox, Space, Tag, Typography, Divider, Alert,
  List, Badge, Tooltip, Radio, message,
} from "antd";
import {
  CloudUploadOutlined, CloudDownloadOutlined, CheckSquareOutlined,
  CloseSquareOutlined, FolderOpenOutlined, ReloadOutlined,
  BranchesOutlined, PlusOutlined, SearchOutlined, GithubOutlined,
} from "@ant-design/icons";
import { useGitStore } from "../../store/gitStore";
import type { CredentialResult } from "../../store/gitStore";
import { PickDirectory } from "../../../wailsjs/go/main/App";

type AuthMethod = "pat" | "bearer" | "stored" | "oauth";

const { Text } = Typography;
const { TextArea } = Input;

const CLR_ADDED    = "#3fb950";
const CLR_MODIFIED = "#d29922";
const CLR_DELETED  = "#f85149";

function extOf(path: string): string {
  const dot = path.lastIndexOf(".");
  return dot >= 0 ? path.slice(dot).toLowerCase() : "(no ext)";
}

// ── Auth method selector ───────────────────────────────────────────────────────
// Renders a Radio group + credential input / credential probe panel.

function AuthSelector({
  authMethod,
  onAuthMethodChange,
  token,
  onTokenChange,
  remoteURL,
}: {
  authMethod: AuthMethod;
  onAuthMethodChange: (m: AuthMethod) => void;
  token: string;
  onTokenChange: (v: string) => void;
  remoteURL?: string;
}) {
  const { lookupCredentials, loginWithOAuth } = useGitStore();
  const [checking, setChecking] = useState(false);
  const [probeResult, setProbeResult] = useState<CredentialResult | null>(null);
  const [oauthLoading, setOauthLoading] = useState(false);

  const handleProbe = async () => {
    if (!remoteURL) return;
    setChecking(true);
    setProbeResult(null);
    const r = await lookupCredentials(remoteURL);
    setProbeResult(r);
    setChecking(false);
  };

  const handleOAuthLogin = async () => {
    setOauthLoading(true);
    try {
      // For now, default to github. We could parse remoteURL to detect gitlab vs github.
      const provider = remoteURL?.includes("gitlab.com") ? "gitlab" : "github";
      const obtainedToken = await loginWithOAuth(provider);
      onTokenChange(obtainedToken);
      message.success(`Successfully authenticated with ${provider === "gitlab" ? "GitLab" : "GitHub"}`);
    } catch (e) {
      // Error is handled in the store, just clear loading
    } finally {
      setOauthLoading(false);
    }
  };

  // Reset probe result when switching auth methods or remoteURL changes
  useEffect(() => { setProbeResult(null); }, [authMethod, remoteURL]);

  const placeholder =
    authMethod === "bearer"
      ? "Bearer token (ey…)"
      : "Personal Access Token (ghp_…, glpat-…) — not saved";

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      <Radio.Group
        size="small"
        value={authMethod}
        onChange={(e) => onAuthMethodChange(e.target.value as AuthMethod)}
        style={{ display: "flex", gap: 4, flexWrap: "wrap" }}
      >
        <Radio.Button value="pat">PAT / Password</Radio.Button>
        <Radio.Button value="oauth">OAuth Browser Flow</Radio.Button>
        <Radio.Button value="bearer">Bearer Token</Radio.Button>
        <Radio.Button value="stored">Stored Credentials</Radio.Button>
      </Radio.Group>

      {authMethod !== "stored" && authMethod !== "oauth" && (
        <Input.Password
          size="small"
          placeholder={placeholder}
          value={token}
          onChange={(e) => onTokenChange(e.target.value)}
          style={{ fontSize: 12 }}
          visibilityToggle
        />
      )}

      {authMethod === "oauth" && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          {token ? (
            <Alert
              type="success"
              showIcon
              style={{ fontSize: 12 }}
              message="Authenticated! Token securely captured in memory."
              action={
                <Button size="small" type="link" onClick={() => onTokenChange("")}>
                  Clear
                </Button>
              }
            />
          ) : (
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <Button
                icon={<GithubOutlined />}
                loading={oauthLoading}
                onClick={handleOAuthLogin}
                size="small"
              >
                Connect to Provider
              </Button>
              <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>
                Opens your browser. Token is kept in memory.
              </Text>
            </div>
          )}
        </div>
      )}

      {authMethod === "stored" && (
        <div style={{ display: "flex", flexDirection: "column", gap: 6 }}>
          <div style={{ display: "flex", gap: 6, alignItems: "center" }}>
            <Text style={{ fontSize: 12, color: "var(--text-muted)", flex: 1 }}>
              Reads OS keychain, ~/.git-credentials, or ~/.netrc
            </Text>
            <Button
              size="small"
              icon={<SearchOutlined />}
              loading={checking}
              disabled={!remoteURL}
              onClick={handleProbe}
            >
              Check
            </Button>
          </div>
          {probeResult !== null && (
            probeResult.found ? (
              <Alert
                type="success"
                showIcon
                style={{ fontSize: 12 }}
                message={
                  `Found ${probeResult.source}` +
                  (probeResult.username ? ` · ${probeResult.username}` : "")
                }
              />
            ) : (
              <Alert
                type="warning"
                showIcon
                style={{ fontSize: 12 }}
                message="No stored credentials found for this remote URL"
              />
            )
          )}
        </div>
      )}
    </div>
  );
}

// ── Virtual file list ─────────────────────────────────────────────────────────
// Renders only the rows visible in the scroll viewport (+ a small buffer).
// Avoids creating thousands of DOM nodes for large repos.

const ROW_HEIGHT   = 24; // px — must match the row's rendered height
const LIST_HEIGHT  = 240; // px — visible container height
const SCROLL_BUFFER = 8;  // extra rows rendered above / below visible area

type FileEntry = { path: string; change: "added" | "modified" | "deleted" };

function VirtualFileList({
  files,
  selected,
  onToggle,
}: {
  files: FileEntry[];
  selected: Set<string>;
  onToggle: (path: string) => void;
}) {
  const [scrollTop, setScrollTop] = useState(0);

  const startIndex  = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - SCROLL_BUFFER);
  const visibleRows = Math.ceil(LIST_HEIGHT / ROW_HEIGHT) + SCROLL_BUFFER * 2;
  const endIndex    = Math.min(files.length, startIndex + visibleRows);

  const topPad    = startIndex * ROW_HEIGHT;
  const bottomPad = Math.max(0, (files.length - endIndex) * ROW_HEIGHT);

  const clrOf = (c: FileEntry["change"]) =>
    c === "added" ? CLR_ADDED : c === "modified" ? CLR_MODIFIED : CLR_DELETED;
  const prefixOf = (c: FileEntry["change"]) =>
    c === "added" ? "+" : c === "modified" ? "~" : "-";

  if (files.length === 0) return null;

  return (
    <div
      style={{
        height:     LIST_HEIGHT,
        overflowY:  "auto",
        border:     "1px solid var(--border)",
        borderRadius: 6,
        background: "var(--bg)",
      }}
      onScroll={(e) => setScrollTop((e.currentTarget).scrollTop)}
    >
      {/* Top spacer maintains total scroll height */}
      {topPad > 0 && <div style={{ height: topPad }} />}

      {files.slice(startIndex, endIndex).map(({ path, change }) => (
        <div
          key={path}
          style={{
            display: "flex", alignItems: "center", gap: 8,
            padding: "3px 10px", height: ROW_HEIGHT,
            cursor: "pointer",
          }}
          onClick={() => onToggle(path)}
        >
          <Checkbox
            checked={selected.has(path)}
            onChange={() => onToggle(path)}
            onClick={(e) => e.stopPropagation()}
          />
          <Text
            style={{
              fontFamily: "monospace", fontSize: 12,
              color: clrOf(change), flex: 1,
              overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap",
            }}
          >
            {prefixOf(change)} {path}
          </Text>
        </div>
      ))}

      {/* Bottom spacer */}
      {bottomPad > 0 && <div style={{ height: bottomPad }} />}
    </div>
  );
}

// ── Tab 1: Commit & Push ──────────────────────────────────────────────────────

function CommitPushTab() {
  const { status, pushing, push, clearError, error, exportDir, remoteURL } = useGitStore();

  // allFiles is built from the capped arrays (≤ 500 per category from backend).
  // selected tracks the full set of paths chosen for commit — operations like
  // selectAll use the capped list, which is all the paths we actually know about.
  const allFiles = useMemo<FileEntry[]>(() => {
    if (!status) return [];
    const files: FileEntry[] = [];
    for (const f of (status.added    ?? [])) files.push({ path: f, change: "added" });
    for (const f of (status.modified ?? [])) files.push({ path: f, change: "modified" });
    for (const f of (status.deleted  ?? [])) files.push({ path: f, change: "deleted" });
    return files;
  }, [status]);

  const [selected, setSelected] = useState<Set<string>>(() => new Set(allFiles.map((f) => f.path)));
  const [message, setMessage] = useState("");
  const [authMethod, setAuthMethod] = useState<AuthMethod>("pat");
  const [token, setToken] = useState("");

  // Re-select all when status refreshes.
  useEffect(() => {
    setSelected(new Set(allFiles.map((f) => f.path)));
  }, [allFiles]);

  const extensions = useMemo(() => [...new Set(allFiles.map((f) => extOf(f.path)))].sort(), [allFiles]);

  const toggle = (path: string) =>
    setSelected((prev) => {
      const n = new Set(prev);
      n.has(path) ? n.delete(path) : n.add(path);
      return n;
    });

  const selectAll  = () => setSelected(new Set(allFiles.map((f) => f.path)));
  const selectNone = () => setSelected(new Set());
  const selectByExt = (ext: string) =>
    setSelected(new Set(allFiles.filter((f) => extOf(f.path) === ext).map((f) => f.path)));

  const handleCommit = async () => {
    await push({ authMethod, token, message, files: [...selected] });
    if (!useGitStore.getState().error) {
      setMessage("");
      setToken("");
    }
  };

  if (!exportDir) {
    return (
      <Text style={{ color: "var(--text-muted)", fontSize: 13 }}>
        No export directory configured. Set one in the Git panel.
      </Text>
    );
  }

  const totalChanged = status?.totalChanged ?? 0;
  const showing = allFiles.length;
  const truncated = totalChanged > showing;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      {error && (
        <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />
      )}

      {/* File selection toolbar */}
      <Space wrap size={4}>
        <Button size="small" icon={<CheckSquareOutlined />} onClick={selectAll}>Select All</Button>
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

      {/* File count + truncation notice */}
      {totalChanged > 0 && (
        <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>
          {truncated
            ? `Showing ${showing.toLocaleString()} of ${totalChanged.toLocaleString()} changed files`
            : `${totalChanged.toLocaleString()} changed file${totalChanged !== 1 ? "s" : ""}`}
        </Text>
      )}

      {/* Virtual file list */}
      {allFiles.length === 0 ? (
        <div style={{ height: 48, display: "flex", alignItems: "center", border: "1px solid var(--border)", borderRadius: 6, padding: "0 16px", background: "var(--bg)" }}>
          <Text style={{ color: "var(--text-muted)", fontSize: 12 }}>
            {status?.isRepo ? "Working tree clean." : "Not a git repository — will be initialised on push."}
          </Text>
        </div>
      ) : (
        <VirtualFileList files={allFiles} selected={selected} onToggle={toggle} />
      )}

      <Divider style={{ borderColor: "var(--border)", margin: "0" }} />

      <TextArea
        size="small"
        rows={2}
        placeholder="Commit message (default: chore: export Snowflake DDL)"
        value={message}
        onChange={(e) => setMessage(e.target.value)}
        style={{ fontSize: 12, resize: "none" }}
      />

      <AuthSelector
        authMethod={authMethod}
        onAuthMethodChange={setAuthMethod}
        token={token}
        onTokenChange={setToken}
        remoteURL={remoteURL || status?.remoteURL}
      />

      <Button
        type="primary"
        icon={<CloudUploadOutlined />}
        loading={pushing}
        disabled={selected.size === 0}
        onClick={handleCommit}
      >
        {pushing ? "Pushing…" : `Commit & Push (${selected.size.toLocaleString()} file${selected.size !== 1 ? "s" : ""})`}
      </Button>
    </div>
  );
}

// ── Tab 2: Pull ───────────────────────────────────────────────────────────────

function PullTab() {
  const { status, pulling, pull, clearError, error, remoteURL, branch, exportDir } = useGitStore();
  const [authMethod, setAuthMethod] = useState<AuthMethod>("pat");
  const [token, setToken] = useState("");

  const handlePull = async () => {
    await pull({ authMethod, token });
    if (!useGitStore.getState().error) setToken("");
  };

  if (!exportDir) {
    return (
      <Text style={{ color: "var(--text-muted)", fontSize: 13 }}>
        No export directory configured. Set one in the Git panel.
      </Text>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      {error && (
        <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />
      )}

      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>Remote URL</Text>
        <Text style={{ fontSize: 12, fontFamily: "monospace", color: remoteURL ? "var(--text)" : "var(--text-muted)" }}>
          {remoteURL || "(none configured)"}
        </Text>
      </div>

      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>Branch</Text>
        <Text style={{ fontSize: 12, fontFamily: "monospace" }}>{branch || "main"}</Text>
      </div>

      {status?.isRepo && (status.ahead ?? 0) > 0 && (
        <Alert
          type="info"
          message={`${status.ahead} commit(s) ahead of remote — push before pulling to avoid conflicts.`}
          showIcon
          style={{ fontSize: 12 }}
        />
      )}

      <AuthSelector
        authMethod={authMethod}
        onAuthMethodChange={setAuthMethod}
        token={token}
        onTokenChange={setToken}
        remoteURL={remoteURL || status?.remoteURL}
      />

      <Button
        type="primary"
        icon={<CloudDownloadOutlined />}
        loading={pulling}
        disabled={!status?.isRepo}
        onClick={handlePull}
      >
        {pulling ? "Pulling…" : "Pull"}
      </Button>
    </div>
  );
}

// ── Tab 3: Clone ──────────────────────────────────────────────────────────────

function CloneTab() {
  const { clone, cloning, error, clearError } = useGitStore();
  const [url, setUrl] = useState("");
  const [path, setPath] = useState("");
  const [authMethod, setAuthMethod] = useState<AuthMethod>("pat");
  const [token, setToken] = useState("");
  const [success, setSuccess] = useState(false);

  const handlePickPath = async () => {
    const dir = await PickDirectory();
    if (dir) setPath(dir);
  };

  const handleClone = async () => {
    setSuccess(false);
    try {
      await clone({ url, path, authMethod, token } as any);
      if (!useGitStore.getState().error) {
        setSuccess(true);
        setUrl("");
        setPath("");
        setToken("");
      }
    } catch {
      // error already in store
    }
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      {error && (
        <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />
      )}
      {success && (
        <Alert type="success" message="Repository cloned successfully." showIcon style={{ fontSize: 12 }} />
      )}

      <Input
        size="small"
        placeholder="https://github.com/org/repo.git"
        value={url}
        onChange={(e) => setUrl(e.target.value)}
        style={{ fontSize: 12 }}
        addonBefore={<Text style={{ fontSize: 11, color: "var(--text-muted)" }}>Remote URL</Text>}
      />

      <div style={{ display: "flex", gap: 6 }}>
        <Input
          size="small"
          placeholder="/path/to/local/directory"
          value={path}
          onChange={(e) => setPath(e.target.value)}
          style={{ fontSize: 12, flex: 1 }}
          addonBefore={<Text style={{ fontSize: 11, color: "var(--text-muted)" }}>Local path</Text>}
        />
        <Tooltip title="Pick directory">
          <Button size="small" icon={<FolderOpenOutlined />} onClick={handlePickPath} />
        </Tooltip>
      </div>

      <AuthSelector
        authMethod={authMethod}
        onAuthMethodChange={setAuthMethod}
        token={token}
        onTokenChange={setToken}
        remoteURL={url || undefined}
      />

      <Button
        type="primary"
        loading={cloning}
        disabled={!url || !path}
        onClick={handleClone}
      >
        {cloning ? "Cloning…" : "Clone"}
      </Button>
    </div>
  );
}

// ── Tab 4: Branches ───────────────────────────────────────────────────────────

function BranchesTab() {
  const { branches, listBranches, checkoutBranch, createBranch, status, exportDir, error, clearError } = useGitStore();
  const [newBranch, setNewBranch] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    if (exportDir) listBranches();
  }, [exportDir]);

  const handleCheckout = async (name: string) => {
    setLoading(true);
    await checkoutBranch(name);
    setLoading(false);
  };

  const handleCreate = async () => {
    if (!newBranch.trim()) return;
    setLoading(true);
    await createBranch(newBranch.trim());
    if (!useGitStore.getState().error) setNewBranch("");
    setLoading(false);
  };

  const currentBranch = status?.branch;

  if (!exportDir) {
    return (
      <Text style={{ color: "var(--text-muted)", fontSize: 13 }}>
        No export directory configured. Set one in the Git panel.
      </Text>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
      {error && (
        <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />
      )}

      <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
        <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>
          {branches.length} branch{branches.length !== 1 ? "es" : ""}
        </Text>
        <Tooltip title="Refresh">
          <Button size="small" icon={<ReloadOutlined />} onClick={listBranches} />
        </Tooltip>
      </div>

      <div style={{ maxHeight: 280, overflowY: "auto", border: "1px solid var(--border)", borderRadius: 6, background: "var(--bg)" }}>
        {branches.length === 0 && (
          <Text style={{ display: "block", padding: "12px 16px", color: "var(--text-muted)", fontSize: 12 }}>
            No branches found.
          </Text>
        )}
        <List
          size="small"
          dataSource={branches}
          renderItem={(b) => (
            <List.Item
              style={{ padding: "4px 10px" }}
              actions={[
                !b.isRemote && b.name !== currentBranch ? (
                  <Button
                    key="switch"
                    size="small"
                    loading={loading}
                    onClick={() => handleCheckout(b.name)}
                  >
                    Switch
                  </Button>
                ) : null,
              ].filter(Boolean) as React.ReactNode[]}
            >
              <Space size={6}>
                <BranchesOutlined style={{ color: b.isCurrent ? "var(--accent)" : "var(--text-muted)", fontSize: 12 }} />
                <Text
                  style={{
                    fontSize: 12,
                    fontFamily: "monospace",
                    color: b.isCurrent ? "var(--accent)" : "var(--text)",
                    fontWeight: b.isCurrent ? 600 : 400,
                  }}
                >
                  {b.name}
                </Text>
                {b.isRemote && <Badge color="blue" text={<Text style={{ fontSize: 10, color: "var(--text-muted)" }}>remote</Text>} />}
                {b.isCurrent && <Badge color="green" text={<Text style={{ fontSize: 10, color: "var(--text-muted)" }}>current</Text>} />}
              </Space>
            </List.Item>
          )}
        />
      </div>

      <Divider style={{ borderColor: "var(--border)", margin: "0" }} />

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
          disabled={!newBranch.trim() || loading}
          onClick={handleCreate}
        >
          Create Branch
        </Button>
      </div>
    </div>
  );
}

// Need React import for JSX in filter cast
import React from "react";

// ── Main Dialog ───────────────────────────────────────────────────────────────

export default function GitOperationsDialog() {
  const { gitOpsOpen, closeGitOps } = useGitStore();

  return (
    <Modal
      open={gitOpsOpen}
      onCancel={closeGitOps}
      title="Git Operations"
      width={620}
      footer={null}
      styles={{ body: { padding: "12px 16px 16px" } }}
      destroyOnClose={false}
    >
      <Tabs
        defaultActiveKey="commit"
        size="small"
        items={[
          { key: "commit",   label: "Commit & Push", children: <CommitPushTab /> },
          { key: "pull",     label: "Pull",          children: <PullTab /> },
          { key: "clone",    label: "Clone",         children: <CloneTab /> },
          { key: "branches", label: "Branches",      children: <BranchesTab /> },
        ]}
      />
    </Modal>
  );
}
