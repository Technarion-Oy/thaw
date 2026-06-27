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
// @thaw-domain: Git Integration

import { useEffect, useMemo, useState } from "react";
import { Button, Input, Tooltip, Popconfirm, Typography, Alert } from "antd";
import {
  CloudUploadOutlined, PlusOutlined, MinusOutlined,
  CaretRightFilled, CaretDownFilled, WarningOutlined, ReloadOutlined,
  LeftOutlined, RightOutlined,
} from "@ant-design/icons";
import { useGitStore } from "../../store/gitStore";
import type { FileChange } from "../../store/gitStore";
import ChangeRow from "./ChangeRow";

const { Text } = Typography;
const { TextArea } = Input;

const PAGE_SIZE = 50;
const MONO = 'var(--editor-font, "JetBrains Mono", monospace)';

// ── A collapsible, paginated group of changed files ───────────────────────────

function ChangeGroup({
  title, files, total, action, onAction, onDiscard, busy,
}: {
  title: string;
  files: FileChange[];
  total: number;
  action: "stage" | "unstage";
  onAction: (path: string) => void;
  onDiscard: (path: string) => void;
  busy: boolean;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const [page, setPage] = useState(0);

  const pageCount = Math.max(1, Math.ceil(files.length / PAGE_SIZE));
  useEffect(() => { if (page >= pageCount) setPage(0); }, [pageCount, page]);

  const start = page * PAGE_SIZE;
  const end = Math.min(files.length, start + PAGE_SIZE);
  const visible = files.slice(start, end);
  const capped = total > files.length; // backend caps each list at 500

  if (total === 0) return null;

  return (
    <div style={{ border: "1px solid var(--border)", borderRadius: 6, background: "var(--bg)", overflow: "hidden" }}>
      {/* Group header */}
      <div
        onClick={() => setCollapsed((c) => !c)}
        style={{ display: "flex", alignItems: "center", gap: 6, padding: "5px 8px", cursor: "pointer", background: "var(--bg-raised)" }}
      >
        {collapsed
          ? <CaretRightFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />
          : <CaretDownFilled style={{ fontSize: 9, color: "var(--text-muted)" }} />}
        <Text style={{ fontSize: 10, fontWeight: 600, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.08em", flex: 1 }}>
          {title}
        </Text>
        {/* Count tone encodes state: staged is what's going into the next commit */}
        <span style={{ fontFamily: MONO, fontSize: 11, color: action === "unstage" ? "var(--success)" : "var(--text-muted)", minWidth: 24, textAlign: "right" }}>{total}</span>
      </div>

      {!collapsed && (
        <>
          {visible.map((f) => (
            <ChangeRow key={f.path} file={f} action={action} onAction={onAction} onDiscard={onDiscard} busy={busy} />
          ))}

          {/* Paginator — surfaces the formerly-silent cap honestly */}
          {(pageCount > 1 || capped) && (
            <div style={{ display: "flex", alignItems: "center", gap: 8, padding: "4px 8px", borderTop: "1px solid var(--border)", background: "var(--bg-raised)" }}>
              <Button size="small" type="text" disabled={page === 0} icon={<LeftOutlined />} onClick={() => setPage((p) => Math.max(0, p - 1))} style={{ height: 18, width: 18, minWidth: 0, padding: 0 }} />
              <Text style={{ fontFamily: MONO, fontSize: 11, color: "var(--text-muted)" }}>
                {start + 1}–{end} of {total}
              </Text>
              <Button size="small" type="text" disabled={page >= pageCount - 1} icon={<RightOutlined />} onClick={() => setPage((p) => Math.min(pageCount - 1, p + 1))} style={{ height: 18, width: 18, minWidth: 0, padding: 0 }} />
              {capped && (
                <Text style={{ fontSize: 10, color: "var(--text-faint)", marginLeft: "auto" }}>
                  first {files.length.toLocaleString()} listed
                </Text>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}

// ── The Changes view: staged + unstaged groups with a commit box ──────────────

export default function ChangesView() {
  const {
    status, staging, committing, resetting, oauthToken, error, clearError,
    stageFile, unstageFile, stageAll, unstageAll, discardFile, commitStaged,
    resetHard, refreshStatus, loading,
  } = useGitStore();

  const [commitMsg, setCommitMsg] = useState("");

  const staged   = useMemo(() => status?.staged   ?? [], [status]);
  const unstaged = useMemo(() => status?.unstaged ?? [], [status]);
  const stagedTotal   = status?.stagedTotal   ?? 0;
  const unstagedTotal = status?.unstagedTotal ?? 0;
  const totalChanged  = status?.totalChanged  ?? 0;

  const busy = staging || committing;

  const handleCommit = async () => {
    await commitStaged(commitMsg);
    if (!useGitStore.getState().error) setCommitMsg("");
  };

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 8 }}>
      {/* Header: title + total + global actions */}
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <Text style={{ fontSize: 10, fontWeight: 600, color: "var(--text-muted)", textTransform: "uppercase", letterSpacing: "0.1em" }}>
          Changes{totalChanged > 0 ? ` · ${totalChanged.toLocaleString()}` : ""}
        </Text>
        <div style={{ display: "flex", gap: 4 }}>
          <Tooltip title="Refresh">
            <Button size="small" icon={<ReloadOutlined spin={loading} />} onClick={refreshStatus} disabled={loading} />
          </Tooltip>
          <Tooltip title="Stage every change (git add -A)">
            <Button size="small" icon={<PlusOutlined />} disabled={busy || unstagedTotal === 0} onClick={stageAll}>Stage all</Button>
          </Tooltip>
          <Tooltip title="Remove every file from the staging area. Your edits are kept (git reset).">
            <Button size="small" icon={<MinusOutlined />} disabled={busy || stagedTotal === 0} onClick={unstageAll}>Unstage all</Button>
          </Tooltip>
          {totalChanged > 0 && (
            <Tooltip title="Discard all working-tree changes and reset to the last commit (git reset --hard)">
              <Popconfirm
                title="Reset all changes to the last commit?"
                description="Reverts every staged and unstaged change in the working tree (git reset --hard HEAD). Your edits are permanently lost — this cannot be undone."
                onConfirm={resetHard}
                okText="Reset hard"
                okButtonProps={{ danger: true }}
                icon={<WarningOutlined style={{ color: "var(--danger)" }} />}
              >
                <Button size="small" danger icon={<WarningOutlined />} loading={resetting} disabled={busy}>Reset to commit</Button>
              </Popconfirm>
            </Tooltip>
          )}
        </div>
      </div>

      {error && <Alert type="error" message={error} showIcon closable onClose={clearError} style={{ fontSize: 12 }} />}

      {/* Commit box — operates on the staged set */}
      <TextArea
        rows={2}
        placeholder="Message — what these staged changes do"
        value={commitMsg}
        onChange={(e) => setCommitMsg(e.target.value)}
        style={{ fontSize: 12, resize: "none" }}
      />
      <Button
        type="primary"
        icon={<CloudUploadOutlined />}
        loading={committing}
        disabled={busy || stagedTotal === 0 || !oauthToken}
        onClick={handleCommit}
      >
        {committing ? "Committing & pushing…" : `Commit & Push ${stagedTotal.toLocaleString()} staged`}
      </Button>
      {!oauthToken && (
        <Text style={{ fontSize: 11, color: "var(--text-muted)" }}>Connect to GitHub above to enable commit &amp; push.</Text>
      )}

      {/* Empty state */}
      {totalChanged === 0 ? (
        <div style={{ padding: "10px 12px", border: "1px solid var(--border)", borderRadius: 6, background: "var(--bg)" }}>
          <Text style={{ fontSize: 12, color: "var(--text-muted)" }}>
            {status?.isRepo ? "No changes. Your working tree matches HEAD." : "Not a git repository — will be initialised on push."}
          </Text>
        </div>
      ) : (
        <>
          <ChangeGroup
            title="Staged changes" files={staged} total={stagedTotal} action="unstage"
            onAction={unstageFile} onDiscard={discardFile} busy={busy}
          />
          <ChangeGroup
            title="Changes" files={unstaged} total={unstagedTotal} action="stage"
            onAction={stageFile} onDiscard={discardFile} busy={busy}
          />
        </>
      )}
    </div>
  );
}
