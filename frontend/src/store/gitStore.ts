import { create } from "zustand";
import { GitStatus, GitCommitAndPush, PickDirectory } from "../../wailsjs/go/main/App";
import type { gitrepo } from "../../wailsjs/go/models";

export type RepoStatus = gitrepo.RepoStatus;
export type PushParams = gitrepo.PushParams;

interface GitState {
  dir: string;
  status: RepoStatus | null;
  loading: boolean;
  pushing: boolean;
  error: string | null;

  setDir: (dir: string) => void;
  pickDir: () => Promise<void>;
  refreshStatus: () => Promise<void>;
  push: (params: Omit<PushParams, "dir">) => Promise<void>;
  clearError: () => void;
}

export const useGitStore = create<GitState>((set, get) => ({
  dir: "",
  status: null,
  loading: false,
  pushing: false,
  error: null,

  setDir: (dir) => set({ dir, status: null, error: null }),

  pickDir: async () => {
    const dir = await PickDirectory();
    if (!dir) return;
    set({ dir, status: null, error: null });
    await get().refreshStatus();
  },

  refreshStatus: async () => {
    const { dir } = get();
    if (!dir) return;
    set({ loading: true, error: null });
    try {
      const status = await GitStatus(dir);
      set({ status, loading: false });
    } catch (e) {
      set({ error: String(e), loading: false });
    }
  },

  push: async (params) => {
    const { dir, refreshStatus } = get();
    if (!dir) return;
    set({ pushing: true, error: null });
    try {
      await GitCommitAndPush({ ...params, dir });
      await refreshStatus();
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ pushing: false });
    }
  },

  clearError: () => set({ error: null }),
}));
