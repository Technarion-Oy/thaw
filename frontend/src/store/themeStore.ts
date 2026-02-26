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
import { persist } from "zustand/middleware";

export type ThemePreference = "system" | "light" | "dark";
export type ResolvedTheme   = "light" | "dark";

function resolveTheme(pref: ThemePreference): ResolvedTheme {
  if (pref === "light") return "light";
  if (pref === "dark")  return "dark";
  // "system" — check media query
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

interface ThemeState {
  preference: ThemePreference;
  resolved:   ResolvedTheme;
  setPreference: (pref: ThemePreference) => void;
  syncSystem: () => void;
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      preference: "system",
      resolved:   resolveTheme("system"),

      setPreference: (pref) => {
        const resolved = resolveTheme(pref);
        set({ preference: pref, resolved });
        applyTheme(resolved);
      },

      syncSystem: () => {
        const { preference } = get();
        if (preference === "system") {
          const resolved = resolveTheme("system");
          set({ resolved });
          applyTheme(resolved);
        }
      },
    }),
    {
      name: "thaw-theme",
      // Only persist the preference; resolved is always recomputed on load.
      partialize: (state) => ({ preference: state.preference }),
      onRehydrateStorage: () => (state) => {
        if (state) {
          const resolved = resolveTheme(state.preference);
          state.resolved = resolved;
          applyTheme(resolved);
        }
      },
    }
  )
);

function applyTheme(resolved: ResolvedTheme) {
  document.documentElement.setAttribute("data-theme", resolved);
}

// Initialise on module load so the theme is applied before the first render.
applyTheme(resolveTheme(useThemeStore.getState().preference));
