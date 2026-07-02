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

import { create } from "zustand";
import { GitStatus, GitCommitAndPush, GitPull, GitFetch, PickDirectory, GetGitConfig, SaveGitConfig, GitClone, GitListBranches, GitCheckoutBranch, GitCheckoutRemoteBranch, GitCreateBranch, GitDeleteBranch, GitDeleteRemoteBranch, GitMergeBranch, GitResetHard, GitUpdateRemoteURL, GitPushBranch, GitLoginWithOAuth, GitStageFile, GitUnstageFile, GitStageAll, GitUnstageAll, GitDiscardFile, OpenFolderInNewInstance, AddRecentDir, ClearRecentDirs, SaveGitAuthor, SaveGitExportPathTemplate } from "../../wailsjs/go/app/App";
import type { gitrepo } from "../../wailsjs/go/models";

export type RepoStatus = gitrepo.RepoStatus;
export type PushParams = gitrepo.PushParams;
export type BranchInfo = gitrepo.BranchInfo;
export type CloneParams = gitrepo.CloneParams;
export type CredentialResult = gitrepo.CredentialResult;
export type FileChange = gitrepo.FileChange;

interface GitState {
  // Persistent config (saved to disk, excluding token)
  exportDir: string;
  remoteURL: string;
  branch: string;
  authorName: string;
  authorEmail: string;
  exportPathTemplate: string;
  recentDirs: string[];
  configLoaded: boolean;

  // Runtime state
  status: RepoStatus | null;
  loading: boolean;
  pulling: boolean;
  cloning: boolean;
  resetting: boolean;
  error: string | null;

  // OAuth token (in-memory only, never persisted)
  oauthToken: string;

  // Dialog state
  gitOpsOpen: boolean;

  // Branch state
  branches: BranchInfo[];

  // Actions
  loadConfig: () => Promise<void>;
  // saveConfig persists only the per-repo/instance git fields. The shared identity/
  // pref fields are owned by dedicated atomic actions (saveAuthor,
  // saveExportPathTemplate) so a whole-struct write can't revert another window's edit.
  saveConfig: (patch: Partial<{
    exportDir: string;
    remoteURL: string;
    branch: string;
  }>) => Promise<void>;
  saveAuthor: (name: string, email: string) => Promise<void>;
  saveExportPathTemplate: (tmpl: string) => Promise<void>;
  pickExportDir: () => Promise<void>;
  // openFolder switches the working directory to `dir` (VS Code "Open Folder"),
  // recording it in the recent-folders list. Used by the File menu, the File
  // Browser header dropdown, and pickExportDir.
  openFolder: (dir: string) => Promise<void>;
  clearRecentDirs: () => Promise<void>;
  // openInNewWindow picks a folder and launches a second Thaw instance rooted
  // there, leaving this window's working directory unchanged.
  openInNewWindow: () => Promise<void>;
  refreshStatus: (silent?: boolean) => Promise<void>;
  pull: () => Promise<void>;

  // Staging (git index) actions — operate on the real index, then refresh status.
  staging: boolean;
  committing: boolean;
  stageFile: (file: string) => Promise<void>;
  unstageFile: (file: string) => Promise<void>;
  stageAll: () => Promise<void>;
  unstageAll: () => Promise<void>;
  discardFile: (file: string) => Promise<void>;
  commitStaged: (message: string, push?: boolean) => Promise<boolean>;

  loginWithOAuth: (provider: string) => Promise<string>;
  setOAuthToken: (token: string) => void;
  clearError: () => void;

  // Dialog actions
  openGitOps: () => void;
  closeGitOps: () => void;

  // Branch actions
  listBranches: () => Promise<void>;
  checkoutBranch: (name: string) => Promise<void>;
  checkoutRemoteBranch: (remoteName: string) => Promise<void>;
  createBranch: (name: string) => Promise<void>;
  deleteBranch: (name: string) => Promise<void>;
  mergeBranch: (name: string) => Promise<void>;
  deleteRemoteBranch: (remoteName: string) => Promise<void>;
  pushBranch: (branch: string) => Promise<void>;
  pullBranch: (branch: string) => Promise<void>;

  // Reset / remote actions
  resetHard: () => Promise<void>;
  updateRemoteURL: (remoteURL: string) => Promise<void>;

  // Clone action
  clone: (params: CloneParams) => Promise<void>;
}

export const useGitStore = create<GitState>((set, get) => ({
  exportDir: "",
  remoteURL: "",
  branch: "main",
  authorName: "",
  authorEmail: "",
  exportPathTemplate: "",
  recentDirs: [],
  configLoaded: false,

  status: null,
  loading: false,
  pulling: false,
  cloning: false,
  resetting: false,
  staging: false,
  committing: false,
  error: null,

  oauthToken: "",

  gitOpsOpen: false,
  branches: [],

  loadConfig: async () => {
    try {
      const cfg = await GetGitConfig();
      set({
        exportDir:          cfg.exportDir          || "",
        remoteURL:          cfg.remoteURL          || "",
        branch:             cfg.branch             || "main",
        authorName:         cfg.authorName         || "",
        authorEmail:        cfg.authorEmail        || "",
        exportPathTemplate: cfg.exportPathTemplate || "",
        recentDirs:         cfg.recentDirs         || [],
        configLoaded: true,
      });
      // Auto-refresh git status if we have a saved directory
      if (cfg.exportDir) {
        await get().refreshStatus();
      }
    } catch {
      set({ configLoaded: true });
    }
  },

  saveConfig: async (patch) => {
    const { exportDir, remoteURL, branch } = get();
    // Only the per-repo/instance fields ride SaveGitConfig. Shared fields (author,
    // export-path template, recentDirs) are owned by dedicated atomic backend methods
    // and preserved on disk, so they can't be clobbered by this whole-struct write.
    const merged = { exportDir, remoteURL, branch, ...patch };
    set(merged);
    try {
      await SaveGitConfig(merged as any);
    } catch {
      // non-fatal — in-memory state is still updated
    }
  },

  saveAuthor: async (name, email) => {
    set({ authorName: name, authorEmail: email });
    try {
      await SaveGitAuthor(name, email); // atomic, field-scoped — safe across windows
    } catch {
      // non-fatal — in-memory state is still updated
    }
  },

  saveExportPathTemplate: async (tmpl) => {
    set({ exportPathTemplate: tmpl });
    try {
      await SaveGitExportPathTemplate(tmpl);
    } catch {
      // non-fatal — in-memory state is still updated
    }
  },

  openFolder: async (dir: string) => {
    if (!dir) return;
    try {
      if (dir !== get().exportDir) {
        // Actually switching repos. Clear the previous folder's live status AND stored
        // remote override up front so any git op during the async refresh window falls
        // back to the NEW folder's own repo — an empty remoteURL makes the Go side use
        // the actual origin at exportDir, and a null status disables Commit/Push (gated
        // on stagedTotal) until the refresh lands, so B's changes can't push to A's
        // remote. branch is NOT blanked — refreshStatus derives it from the new folder's
        // live HEAD. Re-selecting the current folder skips this so a manual remote
        // override isn't wiped.
        set({ status: null });
        await get().saveConfig({ exportDir: dir, remoteURL: "" });
      }
      // Atomic add returns the authoritative merged list (no stale-snapshot overwrite).
      const recentDirs = await AddRecentDir(dir);
      set({ recentDirs: recentDirs ?? [] });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      // Always refresh — otherwise a thrown AddRecentDir would strand `status: null`
      // and leave the new folder showing "no status" until an unrelated action ran.
      await get().refreshStatus();
    }
  },

  clearRecentDirs: async () => {
    await ClearRecentDirs();
    set({ recentDirs: [] });
  },

  openInNewWindow: async () => {
    if (!get().configLoaded) await get().loadConfig();
    const dir = await PickDirectory();
    if (!dir) return;
    // Spawn first so a bad folder (missing/unmounted) surfaces its error before we
    // record it as recent; the new instance opens rooted there without touching
    // this window's exportDir.
    try {
      await OpenFolderInNewInstance(dir);
    } catch (e) {
      set({ error: String(e) });
      return;
    }
    // Record in the shared recents so it's reachable from any window.
    const recentDirs = await AddRecentDir(dir);
    set({ recentDirs: recentDirs ?? [] });
  },

  pickExportDir: async () => {
    // Ensure config is loaded before we save — the File menu's Open Folder (⌘⇧O)
    // can fire before FileBrowser mounts (sidebar hidden), and saving with an
    // unloaded store would clobber the real GitConfig with defaults.
    if (!get().configLoaded) await get().loadConfig();
    const dir = await PickDirectory();
    if (!dir) return;
    await get().openFolder(dir);
  },

  // silent=true is for refreshes that run after another operation (or as
  // background auto-refresh): they must not write `error`, or a status-fetch
  // failure would masquerade as the preceding operation having failed.
  refreshStatus: async (silent = false) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set(silent ? { loading: true } : { loading: true, error: null });
    try {
      const status = await GitStatus(exportDir);
      // Track the live checked-out branch so pull/commit target the real HEAD rather
      // than a stale default (branch has no other fallback, unlike remoteURL). Only
      // when non-empty, so a transient head-read miss doesn't blank it to "main".
      set({ status, loading: false, ...(status.branch ? { branch: status.branch } : {}) });
    } catch (e) {
      set(silent ? { loading: false } : { error: String(e), loading: false });
    }
  },

  pull: async () => {
    const { exportDir, remoteURL: storedURL, branch, oauthToken, status } = get();
    if (!exportDir) return;
    const remoteURL = storedURL || status?.remoteURL || "";
    set({ pulling: true, error: null });
    try {
      await GitPull({
        dir:        exportDir,
        remoteURL,
        branch:     branch || "main",
        authMethod: "oauth",
        token:      oauthToken,
      } as any);
      await get().refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ pulling: false });
    }
  },

  stageFile: async (file: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ staging: true, error: null });
    try {
      await GitStageFile(exportDir, file);
      await get().refreshStatus(true);
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ staging: false });
    }
  },

  unstageFile: async (file: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ staging: true, error: null });
    try {
      await GitUnstageFile(exportDir, file);
      await get().refreshStatus(true);
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ staging: false });
    }
  },

  stageAll: async () => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ staging: true, error: null });
    try {
      await GitStageAll(exportDir);
      await get().refreshStatus(true);
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ staging: false });
    }
  },

  unstageAll: async () => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ staging: true, error: null });
    try {
      await GitUnstageAll(exportDir);
      await get().refreshStatus(true);
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ staging: false });
    }
  },

  discardFile: async (file: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ staging: true, error: null });
    try {
      await GitDiscardFile(exportDir, file);
      await get().refreshStatus(true);
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ staging: false });
    }
  },

  commitStaged: async (message: string, push: boolean = true): Promise<boolean> => {
    const { exportDir, remoteURL: storedURL, branch, authorName, authorEmail, oauthToken, status } = get();
    if (!exportDir) return false;
    const remoteURL = storedURL || status?.remoteURL || "";
    set({ committing: true, error: null });
    try {
      await GitCommitAndPush({
        dir:         exportDir,
        remoteURL,
        branch:      branch || "main",
        authMethod:  "oauth",
        token:       oauthToken,
        message:     message || "chore: export Snowflake DDL",
        authorName,
        authorEmail,
        files:       [],
        stagedOnly:  true,
        noPush:      !push, // commit locally only when push is false
      } as any);
    } catch (e) {
      const msg = String(e);
      // Refresh BEFORE clearing `committing` (mirrors the success path) so the
      // button stays disabled until stagedTotal reflects reality.
      await get().refreshStatus(true);
      if (msg.includes("nothing staged to commit")) {
        // ErrNothingToCommit: the index was empty (e.g. cleared in a terminal
        // between the last refresh and the click). Surface it instead of a silent
        // no-op; keep the message (return false).
        set({ error: "Nothing to commit — there were no staged changes.", committing: false });
        return false;
      }
      // A "git push:" error means the local commit succeeded and only the push
      // failed — the index is drained, so clear the message (return true) to stop
      // the user re-committing into a duplicate, while still showing the push error.
      const committedButPushFailed = msg.includes("git push");
      set({ error: msg, committing: false });
      return committedButPushFailed;
    }
    // Keep `committing` true through the status refresh so the commit button stays
    // disabled until stagedTotal reflects the now-empty index — otherwise a fast
    // second click commits an empty index. Silent refresh keeps a status-fetch
    // failure from masquerading as a commit failure.
    await get().refreshStatus(true);
    set({ committing: false });
    return true;
  },

  loginWithOAuth: async (provider: string) => {
    try {
      const token = await GitLoginWithOAuth(provider);
      set({ oauthToken: token });
      return token;
    } catch (e) {
      const msg = String(e);
      // A user-initiated cancel (dialog closed / Cancel button) isn't an error.
      if (!msg.includes("context canceled")) set({ error: msg });
      throw e;
    }
  },

  setOAuthToken: (token: string) => set({ oauthToken: token }),

  clearError: () => set({ error: null }),

  openGitOps: () => set({ gitOpsOpen: true }),
  closeGitOps: () => set({ gitOpsOpen: false }),

  listBranches: async () => {
    const { exportDir, oauthToken } = get();
    if (!exportDir) return;
    // Fetch from remote first so remote-tracking refs are current.
    // Fetch errors are intentionally swallowed — offline / no remote is fine,
    // we still want to show the local branches.
    try { await GitFetch(exportDir, oauthToken); } catch { /* offline or no remote */ }
    try {
      const branches = await GitListBranches(exportDir);
      set({ branches: branches ?? [] });
    } catch {
      set({ branches: [] });
    }
  },

  checkoutBranch: async (name: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ error: null });
    try {
      await GitCheckoutBranch(exportDir, name);
      await get().refreshStatus();
      await get().listBranches();
    } catch (e) {
      set({ error: String(e) });
    }
  },

  checkoutRemoteBranch: async (remoteName: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ error: null });
    try {
      await GitCheckoutRemoteBranch(exportDir, remoteName);
      await get().refreshStatus();
      await get().listBranches();
    } catch (e) {
      set({ error: String(e) });
    }
  },

  createBranch: async (name: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ error: null });
    try {
      await GitCreateBranch(exportDir, name);
      await get().refreshStatus();
      await get().listBranches();
    } catch (e) {
      set({ error: String(e) });
    }
  },

  deleteBranch: async (name: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ error: null });
    try {
      await GitDeleteBranch(exportDir, name);
      await get().listBranches();
    } catch (e) {
      set({ error: String(e) });
    }
  },

  mergeBranch: async (name: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ error: null });
    try {
      await GitMergeBranch(exportDir, name);
      await get().refreshStatus();
      await get().listBranches();
    } catch (e) {
      set({ error: String(e) });
    }
  },

  deleteRemoteBranch: async (remoteName: string) => {
    const { exportDir, oauthToken } = get();
    if (!exportDir) return;
    set({ error: null });
    // remoteName is "origin/branch-name" — strip the remote prefix for the IPC call
    const idx = remoteName.indexOf("/");
    const branch = idx >= 0 ? remoteName.slice(idx + 1) : remoteName;
    try {
      await GitDeleteRemoteBranch(exportDir, branch, oauthToken);
      await get().listBranches();
    } catch (e) {
      set({ error: String(e) });
    }
  },

  pushBranch: async (branch: string) => {
    const { exportDir, oauthToken } = get();
    if (!exportDir) return;
    set({ error: null });
    try {
      await GitPushBranch(exportDir, branch, oauthToken);
    } catch (e) {
      set({ error: String(e) });
    }
  },

  pullBranch: async (branch: string) => {
    const { exportDir, remoteURL: storedURL, oauthToken, status } = get();
    if (!exportDir) return;
    const remoteURL = storedURL || status?.remoteURL || "";
    set({ pulling: true, error: null });
    try {
      await GitPull({
        dir:        exportDir,
        remoteURL,
        branch,
        authMethod: "oauth",
        token:      oauthToken,
      } as any);
      await get().refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ pulling: false });
    }
  },

  resetHard: async () => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ resetting: true, error: null });
    try {
      await GitResetHard(exportDir);
      await get().refreshStatus(true); // silent: a status-fetch failure isn't a reset failure
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ resetting: false });
    }
  },

  updateRemoteURL: async (remoteURL: string) => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ error: null });
    try {
      await GitUpdateRemoteURL(exportDir, remoteURL);
      await get().saveConfig({ remoteURL });
      await get().refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    }
  },

  clone: async (params: CloneParams) => {
    set({ cloning: true, error: null });
    try {
      await GitClone(params as any);
    } catch (e) {
      set({ error: String(e) });
      throw e;
    } finally {
      set({ cloning: false });
    }
  },
}));
