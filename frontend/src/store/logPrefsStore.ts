// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Core IPC & App Lifecycle

import { create } from "zustand";
import { GetLogPrefs, GetLogPrefsLocked } from "../../wailsjs/go/app/App";
import type { config } from "../../wailsjs/go/models";

// Optimistic defaults: build-default level ("" sentinel), no SQL written to
// disk, nothing admin-locked until the backend responds.
const defaultPrefs: config.LogPrefs = {
  logLevel: "",
  includeQuerySQL: false,
  includeInternalQueries: false,
};

const nothingLocked: config.LogPrefsLocked = {
  logLevel: false,
  includeQuerySQL: false,
  includeInternalQueries: false,
};

interface LogPrefsState {
  prefs: config.LogPrefs;
  /** Which fields are enforced by IT admin and cannot be changed by the user. */
  locked: config.LogPrefsLocked;
  /** Reload prefs and lock mask from the backend (call after UpdateLogPrefs). */
  load: () => Promise<void>;
}

export const useLogPrefsStore = create<LogPrefsState>((set) => ({
  prefs: defaultPrefs,
  locked: nothingLocked,
  load: async () => {
    const [prefs, locked] = await Promise.all([GetLogPrefs(), GetLogPrefsLocked()]);
    set({ prefs, locked });
  },
}));
