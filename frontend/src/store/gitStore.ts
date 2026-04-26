// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { create } from "zustand";
import { GitStatus, GitCommitAndPush, GitPull, PickDirectory, GetGitConfig, SaveGitConfig, GitClone, GitListBranches, GitCheckoutBranch, GitCreateBranch, GitLookupCredentials, GitLoginWithOAuth } from "../../wailsjs/go/main/App";
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
  error: string | null;

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
  push: (params: { authMethod?: string; token: string; message: string; files?: string[] }) => Promise<void>;
  pull: (params: { authMethod?: string; token: string }) => Promise<void>;
  lookupCredentials: (remoteURL: string) => Promise<CredentialResult>;
  loginWithOAuth: (provider: string) => Promise<string>;
  clearError: () => void;

  // Dialog actions
  openGitOps: () => void;
  closeGitOps: () => void;

  // Branch actions
  listBranches: () => Promise<void>;
  checkoutBranch: (name: string) => Promise<void>;
  createBranch: (name: string) => Promise<void>;

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
  error: null,

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

  push: async ({ authMethod, token, message, files }) => {
    const { exportDir, remoteURL, branch, authorName, authorEmail } = get();
    if (!exportDir) return;
    set({ pushing: true, error: null });
    try {
      await GitCommitAndPush({
        dir:         exportDir,
        remoteURL:   remoteURL,
        branch:      branch || "main",
        authMethod:  authMethod ?? "pat",
        token,
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

  pull: async ({ authMethod, token }) => {
    const { exportDir, remoteURL, branch } = get();
    if (!exportDir) return;
    set({ pulling: true, error: null });
    try {
      await GitPull({
        dir:        exportDir,
        remoteURL:  remoteURL,
        branch:     branch || "main",
        authMethod: authMethod ?? "pat",
        token,
      } as any);
      await get().refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ pulling: false });
    }
  },

  lookupCredentials: async (remoteURL: string) => {
    try {
      return await GitLookupCredentials(remoteURL);
    } catch {
      return { found: false, username: "", source: "" } as CredentialResult;
    }
  },

  loginWithOAuth: async (provider: string) => {
    try {
      return await GitLoginWithOAuth(provider);
    } catch (e) {
      set({ error: String(e) });
      throw e;
    }
  },

  clearError: () => set({ error: null }),

  openGitOps: () => set({ gitOpsOpen: true }),
  closeGitOps: () => set({ gitOpsOpen: false }),

  listBranches: async () => {
    const { exportDir } = get();
    if (!exportDir) return;
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
