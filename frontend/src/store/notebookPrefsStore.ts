// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Snowpark & Developer Workflows

import { create } from "zustand";
import { GetNotebookPrefs } from "../../wailsjs/go/app/App";
import type { config } from "../../wailsjs/go/models";

interface NotebookPrefsState {
  prefs: config.NotebookPrefs;
  /** Reload prefs from the backend (call after SaveNotebookPrefs). */
  load: () => Promise<void>;
}

export const useNotebookPrefsStore = create<NotebookPrefsState>((set) => ({
  // Optimistic default: kernel-aware mode until the backend responds.
  prefs: { syntaxMode: "kernel" },
  load: async () => {
    const prefs = await GetNotebookPrefs();
    set({ prefs });
  },
}));
