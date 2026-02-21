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
import { GitStatus, GitCommitAndPush, GitPull, PickDirectory, GetGitConfig, SaveGitConfig } from "../../wailsjs/go/main/App";
import type { gitrepo } from "../../wailsjs/go/models";

export type RepoStatus = gitrepo.RepoStatus;
export type PushParams = gitrepo.PushParams;

interface GitState {
  // Persistent config (saved to disk, excluding token)
  exportDir: string;
  remoteURL: string;
  branch: string;
  authorName: string;
  authorEmail: string;
  configLoaded: boolean;

  // Runtime state
  status: RepoStatus | null;
  loading: boolean;
  pushing: boolean;
  pulling: boolean;
  error: string | null;

  // Actions
  loadConfig: () => Promise<void>;
  saveConfig: (patch: Partial<{
    exportDir: string;
    remoteURL: string;
    branch: string;
    authorName: string;
    authorEmail: string;
  }>) => Promise<void>;
  pickExportDir: () => Promise<void>;
  refreshStatus: () => Promise<void>;
  push: (params: { token: string; message: string; files?: string[] }) => Promise<void>;
  pull: (params: { token: string }) => Promise<void>;
  clearError: () => void;
}

export const useGitStore = create<GitState>((set, get) => ({
  exportDir: "",
  remoteURL: "",
  branch: "main",
  authorName: "",
  authorEmail: "",
  configLoaded: false,

  status: null,
  loading: false,
  pushing: false,
  pulling: false,
  error: null,

  loadConfig: async () => {
    try {
      const cfg = await GetGitConfig();
      set({
        exportDir:   cfg.exportDir   || "",
        remoteURL:   cfg.remoteURL   || "",
        branch:      cfg.branch      || "main",
        authorName:  cfg.authorName  || "",
        authorEmail: cfg.authorEmail || "",
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
    const { exportDir, remoteURL, branch, authorName, authorEmail } = get();
    const merged = { exportDir, remoteURL, branch, authorName, authorEmail, ...patch };
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

  push: async ({ token, message, files }) => {
    const { exportDir, remoteURL, branch, authorName, authorEmail } = get();
    if (!exportDir) return;
    set({ pushing: true, error: null });
    try {
      await GitCommitAndPush({
        dir:         exportDir,
        remoteURL:   remoteURL,
        branch:      branch || "main",
        token,
        message:     message || "chore: export Snowflake DDL",
        authorName,
        authorEmail,
        files:       files ?? [],
      });
      await get().refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ pushing: false });
    }
  },

  pull: async ({ token }) => {
    const { exportDir, remoteURL, branch } = get();
    if (!exportDir) return;
    set({ pulling: true, error: null });
    try {
      await GitPull({
        dir:       exportDir,
        remoteURL: remoteURL,
        branch:    branch || "main",
        token,
      });
      await get().refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ pulling: false });
    }
  },

  clearError: () => set({ error: null }),
}));
