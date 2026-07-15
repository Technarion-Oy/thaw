// SPDX-License-Identifier: GPL-3.0-or-later

import { create } from "zustand";
import { persist } from "zustand/middleware";

export type ThemePreference = "system" | "light" | "dark";
export type ResolvedTheme   = "light" | "dark";
export type UIDensity       = "compact" | "default" | "comfortable";

export const UI_FONTS = [
  { label: "Inter",         value: "'Inter', 'SF Pro Text', system-ui, sans-serif" },
  { label: "System UI",     value: "system-ui, -apple-system, sans-serif" },
  { label: "SF Pro",        value: "'SF Pro Text', 'SF Pro Display', system-ui, sans-serif" },
  { label: "Roboto",        value: "'Roboto', 'Helvetica Neue', Arial, sans-serif" },
  { label: "Open Sans",     value: "'Open Sans', 'Helvetica Neue', Arial, sans-serif" },
  { label: "IBM Plex Sans", value: "'IBM Plex Sans', 'Helvetica Neue', Arial, sans-serif" },
  { label: "Courier New",   value: "'Courier New', Courier, monospace" },
] as const;

export const EDITOR_FONTS = [
  { label: "JetBrains Mono", value: "'JetBrains Mono', 'Fira Code', monospace" },
  { label: "Fira Code",      value: "'Fira Code', 'Cascadia Code', monospace" },
  { label: "Cascadia Code",  value: "'Cascadia Code', Consolas, monospace" },
  { label: "Menlo",          value: "Menlo, Monaco, monospace" },
  { label: "Courier New",    value: "'Courier New', Courier, monospace" },
  { label: "System Mono",    value: "ui-monospace, SFMono-Regular, Consolas, monospace" },
] as const;

export const EDITOR_FONT_SIZES = [11, 12, 13, 14, 15, 16, 18, 20] as const;

function resolveTheme(pref: ThemePreference): ResolvedTheme {
  if (pref === "light") return "light";
  if (pref === "dark")  return "dark";
  // "system" — check media query
  return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
}

interface ThemeState {
  preference:     ThemePreference;
  resolved:       ResolvedTheme;
  uiFont:         string;
  editorFont:     string;
  editorFontSize: number;
  uiDensity:      UIDensity;
  setPreference:    (pref: ThemePreference) => void;
  syncSystem:       () => void;
  setUiFont:        (font: string) => void;
  setEditorFont:    (font: string) => void;
  setEditorFontSize:(size: number) => void;
  setUIDensity:     (density: UIDensity) => void;
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set, get) => ({
      preference:     "system",
      resolved:       resolveTheme("system"),
      uiFont:         UI_FONTS[0].value,
      editorFont:     EDITOR_FONTS[0].value,
      editorFontSize: 14,
      uiDensity:      "default",

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

      setUiFont: (font) => {
        set({ uiFont: font });
        applyUiFont(font);
      },

      setEditorFont: (font) => {
        set({ editorFont: font });
      },

      setEditorFontSize: (size) => {
        set({ editorFontSize: size });
      },

      setUIDensity: (density) => {
        set({ uiDensity: density });
        applyDensity(density);
      },
    }),
    {
      name: "thaw-theme",
      partialize: (state) => ({
        preference:     state.preference,
        uiFont:         state.uiFont,
        editorFont:     state.editorFont,
        editorFontSize: state.editorFontSize,
        uiDensity:      state.uiDensity,
      }),
      onRehydrateStorage: () => (state) => {
        if (state) {
          const resolved = resolveTheme(state.preference);
          state.resolved = resolved;
          applyTheme(resolved);
          applyUiFont(state.uiFont);
          applyDensity(state.uiDensity);
        }
      },
    }
  )
);

function applyTheme(resolved: ResolvedTheme) {
  document.documentElement.setAttribute("data-theme", resolved);
}

function applyUiFont(font: string) {
  document.documentElement.style.setProperty("--ui-font", font);
}

function applyDensity(density: UIDensity) {
  document.documentElement.setAttribute("data-density", density);
}

// Initialise on module load so settings are applied before the first render.
applyTheme(resolveTheme(useThemeStore.getState().preference));
applyUiFont(useThemeStore.getState().uiFont);
applyDensity(useThemeStore.getState().uiDensity);
