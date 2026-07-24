// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { create } from "zustand";
import { persist, createJSONStorage } from "zustand/middleware";

// Frontend-only editor tab preferences, persisted to localStorage. These are pure
// UI behaviors with no backend counterpart, so they live here rather than in the
// backend-driven `featureFlagsStore` / `EditorPrefs`.
interface EditorTabPrefsState {
  // Mirrors VS Code's `workbench.editor.enablePreview`: when true, single-clicking a
  // file in the file browser (or a search result) opens it in a reusable *preview*
  // tab instead of a permanent one. Default true.
  previewTabsEnabled: boolean;
  setPreviewTabsEnabled: (enabled: boolean) => void;
}

export const useEditorTabPrefsStore = create<EditorTabPrefsState>()(
  persist(
    (set) => ({
      previewTabsEnabled: true,
      setPreviewTabsEnabled: (enabled) => set({ previewTabsEnabled: enabled }),
    }),
    {
      name: "thaw-editor-tab-prefs",
      storage: createJSONStorage(() => localStorage),
    },
  ),
);
