// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: AI Tooling

import { create } from "zustand";
import { GetAISchemaContextEnabled } from "../../wailsjs/go/app/App";

interface AIPrefsState {
  /**
   * Whether inline completions may send schema context (#924). Starts false and
   * stays false until the backend answers: the editor skips building the context
   * while it is off, and guessing "on" would mean sending column names to a
   * hosted provider on the strength of an optimistic default.
   */
  schemaContext: boolean;
  /** Re-read from the backend. Call after saving AI settings. */
  load: () => Promise<void>;
  /** Load once, then hand back the cached value. Safe to call per keystroke. */
  ensureLoaded: () => Promise<void>;
}

// Dedupes concurrent first reads; cleared on an explicit reload.
let inFlight: Promise<void> | null = null;
let loaded = false;

export const useAIPrefsStore = create<AIPrefsState>((set) => ({
  schemaContext: false,

  load: async () => {
    inFlight = null;
    try {
      set({ schemaContext: await GetAISchemaContextEnabled() });
      loaded = true;
    } catch {
      set({ schemaContext: false });
    }
  },

  ensureLoaded: async () => {
    if (loaded) return;
    inFlight ??= useAIPrefsStore.getState().load();
    await inFlight;
  },
}));
