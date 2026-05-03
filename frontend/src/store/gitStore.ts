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
import { GitStatus, GitCommitAndPush, GitPull, GitFetch, PickDirectory, GetGitConfig, SaveGitConfig, GitClone, GitListBranches, GitCheckoutBranch, GitCheckoutRemoteBranch, GitCreateBranch, GitDeleteBranch, GitDeleteRemoteBranch, GitMergeBranch, GitResetHard, GitUpdateRemoteURL, GitPushBranch, GitLoginWithOAuth } from "../../wailsjs/go/main/App";
import type { gitrepo } from "../../wailsjs/go/models";

export type RepoStatus = gitrepo.RepoStatus;
export type PushParams = gitrepo.PushParams;
export type BranchInfo = gitrepo.BranchInfo;
export type CloneParams = gitrepo.CloneParams;
export type CredentialResult = gitrepo.CredentialResult;

interface GitState {
  // Persistent config (saved to disk, excluding token)
  exportDir: string;
  remoteURL: string;
  branch: string;
  authorName: string;
  authorEmail: string;
  exportPathTemplate: string;
  configLoaded: boolean;

  // Runtime state
  status: RepoStatus | null;
  loading: boolean;
  pushing: boolean;
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
  saveConfig: (patch: Partial<{
    exportDir: string;
    remoteURL: string;
    branch: string;
    authorName: string;
    authorEmail: string;
    exportPathTemplate: string;
  }>) => Promise<void>;
  pickExportDir: () => Promise<void>;
  refreshStatus: () => Promise<void>;
  push: (params: { message: string; files?: string[] }) => Promise<void>;
  pull: () => Promise<void>;
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
  configLoaded: false,

  status: null,
  loading: false,
  pushing: false,
  pulling: false,
  cloning: false,
  resetting: false,
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
    const { exportDir, remoteURL, branch, authorName, authorEmail, exportPathTemplate } = get();
    const merged = { exportDir, remoteURL, branch, authorName, authorEmail, exportPathTemplate, ...patch };
    set(merged);
    try {
      await SaveGitConfig(merged);
    } catch {
      // non-fatal — in-memory state is still updated
    }
  },

  pickExportDir: async () => {
    const dir = await PickDirectory();
    if (!dir) return;
    await get().saveConfig({ exportDir: dir });
    await get().refreshStatus();
  },

  refreshStatus: async () => {
    const { exportDir } = get();
    if (!exportDir) return;
    set({ loading: true, error: null });
    try {
      const status = await GitStatus(exportDir);
      set({ status, loading: false });
    } catch (e) {
      set({ error: String(e), loading: false });
    }
  },

  push: async ({ message, files }) => {
    const { exportDir, remoteURL: storedURL, branch, authorName, authorEmail, oauthToken, status } = get();
    if (!exportDir) return;
    // Prefer the store's saved URL; fall back to what the repo's git config reports.
    const remoteURL = storedURL || status?.remoteURL || "";
    set({ pushing: true, error: null });
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
        files:       files ?? [],
      } as any);
      await get().refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ pushing: false });
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

  loginWithOAuth: async (provider: string) => {
    try {
      const token = await GitLoginWithOAuth(provider);
      set({ oauthToken: token });
      return token;
    } catch (e) {
      set({ error: String(e) });
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
      await get().refreshStatus();
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
