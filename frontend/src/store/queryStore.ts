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
import { persist, createJSONStorage } from "zustand/middleware";
import { ExecuteQuery } from "../../wailsjs/go/main/App";

// Custom event name used by executeInNewTab to ask QueryPage to run a query
// through its own StartQuery/WaitForQueryResult path (the only path that
// populates resultHistory and makes results visible in the UI).
export const EXECUTE_IN_TAB_EVENT = "thaw:execute-in-tab";

export interface QueryResult {
  columns: string[];
  rows: unknown[][];
  rowsAffected: number;
  queryID?: string;
  truncated?: boolean;
}

export interface TabDiff {
  leftLabel: string;
  rightLabel: string;
  leftText: string;
  rightText: string;
}

export interface Tab {
  id: string;
  kind?: "sql" | "notebook" | "yaml" | "python"; // defaults to "sql" when absent (backward compat)
  path: string | null;   // null = unsaved scratch tab
  title: string;
  sql: string;
  savedSql: string;      // content at last open/save; compare to sql to derive isDirty
  result: QueryResult | null;
  error: string | null;
  diff?: TabDiff | null; // populated for diff tabs; absent for regular SQL tabs
  isRunning?: boolean;   // per-tab running state; never persisted
}

// ── helpers ───────────────────────────────────────────────────────────────────

/** Infer the tab kind from a file path's extension. */
function kindFromPath(path: string): Tab["kind"] {
  const ext = path.split(".").pop()?.toLowerCase();
  if (ext === "py") return "python";
  if (ext === "yml" || ext === "yaml") return "yaml";
  return undefined; // treated as "sql"
}

function makeTab(overrides?: Partial<Tab>): Tab {
  return {
    id: crypto.randomUUID(),
    path: null,
    title: "SQL",
    sql: "",
    savedSql: "",
    result: null,
    error: null,
    ...overrides,
  };
}

function patchTab(tabs: Tab[], id: string, patch: Partial<Tab>): Tab[] {
  return tabs.map((t) => (t.id === id ? { ...t, ...patch } : t));
}

// ── state ─────────────────────────────────────────────────────────────────────

interface QueryState {
  tabs: Tab[];
  activeTabId: string;

  // Flat aliases — always mirror the active tab so existing components
  // (SqlEditor, QueryPage) don't need to change their selectors.
  sql: string;
  selectedSql: string;
  currentFile: string | null;
  result: QueryResult | null;
  isRunning: boolean;
  error: string | null;

  splitTabId: string | null;

  // Tab management
  activateTab: (id: string) => void;
  openFile: (path: string, content: string) => void;
  openScratch: () => void;
  openDiff: (leftLabel: string, leftText: string, rightLabel: string, rightText: string) => void;
  openNotebook: (path: string, content: string) => void;
  openNotebookUnsaved: (title: string, content: string) => void;
  closeTab: (id: string) => void;
  moveTab: (draggedId: string, targetId: string, before: boolean) => void;
  // Called after a successful save to update the tab's path/title and clear dirty state.
  markSaved: (id: string, path: string, title: string) => void;
  setSplitTab: (id: string | null) => void;
  setSqlForTab: (tabId: string, sql: string) => void;
  // Called on startup: update a file tab with fresh disk content.
  // Preserves dirty sql but always updates savedSql.
  refreshFileTab: (id: string, diskContent: string) => void;
  // Called when a file no longer exists on disk: make it a scratch tab.
  orphanFileTab: (id: string) => void;

  // Active-tab mutations (also kept in the tabs array for restoration on switch)
  setSql: (sql: string) => void;
  setSelectedSql: (selected: string) => void;
  setResult: (result: QueryResult) => void;
  setRunning: (isRunning: boolean) => void;
  setTabRunning: (tabId: string, running: boolean) => void;
  setError: (error: string | null) => void;
  executeWith: (sql: string) => Promise<void>;
  executeInNewTab: (sql: string) => void;
  loadInNewTab: (sql: string) => void;
}

// ── store ─────────────────────────────────────────────────────────────────────

const INITIAL_SQL = "SELECT CURRENT_USER(), CURRENT_WAREHOUSE(), CURRENT_DATABASE();";

const initialTab = makeTab({ sql: INITIAL_SQL, savedSql: INITIAL_SQL });

export const useQueryStore = create<QueryState>()(
  persist(
    (set) => ({
  tabs: [initialTab],
  activeTabId: initialTab.id,
  splitTabId: null,

  sql: initialTab.sql,
  selectedSql: "",
  currentFile: null,
  result: null,
  isRunning: false,
  error: null,

  // ── tab management ────────────────────────────────────────────────────────

  activateTab: (id) =>
    set((state) => {
      if (id === state.activeTabId) return {};
      const target = state.tabs.find((t) => t.id === id);
      if (!target) return {};
      return {
        activeTabId: id,
        sql: target.sql,
        selectedSql: "",
        currentFile: target.path,
        result: target.result,
        error: target.error,
        isRunning: target.isRunning ?? false,
      };
    }),

  openFile: (path, content) =>
    set((state) => {
      // Re-activate an existing tab for this path
      const existing = state.tabs.find((t) => t.path === path);
      if (existing) {
        if (existing.id === state.activeTabId) return {};
        return {
          activeTabId: existing.id,
          sql: existing.sql,
          selectedSql: "",
          currentFile: existing.path,
          result: existing.result,
          error: existing.error,
        };
      }

      // Open a new tab
      const newTab = makeTab({
        path,
        kind: kindFromPath(path),
        title: path.split("/").pop() ?? path,
        sql: content,
        savedSql: content,
      });
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
        sql: content,
        selectedSql: "",
        currentFile: path,
        result: null,
        error: null,
      };
    }),

  openScratch: () =>
    set((state) => {
      const newTab = makeTab();
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
        sql: "",
        selectedSql: "",
        currentFile: null,
        result: null,
        error: null,
      };
    }),

  openDiff: (leftLabel, leftText, rightLabel, rightText) =>
    set((state) => {
      // Derive short names for the tab title, e.g. "TABLE: DB.SCHEMA.ORDERS" → "ORDERS"
      const short = (label: string) => {
        const part = label.includes(":") ? label.split(":").slice(1).join(":").trim() : label;
        return part.split(/[./\\]/).filter(Boolean).pop() ?? part;
      };
      const newTab = makeTab({
        title: `${short(leftLabel)} ↔ ${short(rightLabel)}`,
        diff: { leftLabel, leftText, rightLabel, rightText },
      });
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
        sql: "",
        selectedSql: "",
        currentFile: null,
        result: null,
        error: null,
      };
    }),

  openNotebook: (path, content) =>
    set((state) => {
      const existing = state.tabs.find((t) => t.path === path);
      if (existing) {
        if (existing.id === state.activeTabId) return {};
        return {
          activeTabId: existing.id,
          sql: existing.sql,
          selectedSql: "",
          currentFile: existing.path,
          result: null,
          error: null,
        };
      }
      const newTab = makeTab({
        kind: "notebook",
        path,
        title: path.split("/").pop() ?? path,
        sql: content,
        savedSql: content,
      });
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
        sql: content,
        selectedSql: "",
        currentFile: path,
        result: null,
        error: null,
      };
    }),

  openNotebookUnsaved: (title, content) =>
    set((state) => {
      const newTab = makeTab({
        kind: "notebook",
        path: null,
        title,
        sql: content,
        savedSql: "",  // dirty from the start — no saved version
      });
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
        sql: content,
        selectedSql: "",
        currentFile: null,
        result: null,
        error: null,
      };
    }),

  moveTab: (draggedId, targetId, before) =>
    set((state) => {
      const without = state.tabs.filter((t) => t.id !== draggedId);
      const idx = without.findIndex((t) => t.id === targetId);
      if (idx === -1) return {};
      without.splice(before ? idx : idx + 1, 0, state.tabs.find((t) => t.id === draggedId)!);
      return { tabs: without };
    }),

  setSplitTab: (id) => set({ splitTabId: id }),

  setSqlForTab: (tabId, sql) =>
    set((s) => ({
      tabs: s.tabs.map((t) => (t.id === tabId ? { ...t, sql } : t)),
    })),

  refreshFileTab: (id, diskContent) =>
    set((state) => {
      const tab = state.tabs.find((t) => t.id === id);
      if (!tab) return {};
      // If clean (no unsaved changes), update both sql and savedSql.
      // If dirty, only update savedSql so the user's edits are preserved.
      const isClean = tab.sql === tab.savedSql;
      const updatedTabs = state.tabs.map((t) =>
        t.id === id
          ? { ...t, savedSql: diskContent, ...(isClean ? { sql: diskContent } : {}) }
          : t
      );
      const isActive = state.activeTabId === id;
      return {
        tabs: updatedTabs,
        ...(isActive && isClean ? { sql: diskContent } : {}),
      };
    }),

  orphanFileTab: (id) =>
    set((state) => {
      const tab = state.tabs.find((t) => t.id === id);
      if (!tab) return {};
      const updatedTabs = state.tabs.map((t) =>
        t.id === id
          ? { ...t, path: null, title: `↺ ${t.title}`, savedSql: "" }
          : t
      );
      const isActive = state.activeTabId === id;
      return {
        tabs: updatedTabs,
        ...(isActive ? { currentFile: null } : {}),
      };
    }),

  closeTab: (id) =>
    set((state) => {
      const idx = state.tabs.findIndex((t) => t.id === id);
      if (idx === -1) return {};
      const newTabs = state.tabs.filter((t) => t.id !== id);

      // Closing the last tab — replace with a fresh scratch tab.
      if (newTabs.length === 0) {
        const freshTab = makeTab();
        return {
          tabs: [freshTab],
          activeTabId: freshTab.id,
          sql: freshTab.sql,
          selectedSql: "",
          currentFile: null,
          result: null,
          error: null,
          splitTabId: null,
        };
      }

      let next: Partial<QueryState>;
      if (id !== state.activeTabId) {
        next = { tabs: newTabs };
      } else {
        // Closing the active tab — move to the nearest neighbour
        const nextTab = newTabs[Math.min(idx, newTabs.length - 1)];
        next = {
          tabs: newTabs,
          activeTabId: nextTab.id,
          sql: nextTab.sql,
          selectedSql: "",
          currentFile: nextTab.path,
          result: nextTab.result,
          error: nextTab.error,
          isRunning: nextTab.isRunning ?? false,
        };
      }

      if (state.splitTabId === id) {
        return { ...next, splitTabId: null };
      }
      return next;
    }),

  markSaved: (id, path, title) =>
    set((state) => {
      const tab = state.tabs.find((t) => t.id === id);
      const savedSql = tab?.sql ?? "";
      const updatedTabs = patchTab(state.tabs, id, { path, title, savedSql });
      const isActive = state.activeTabId === id;
      return {
        tabs: updatedTabs,
        ...(isActive ? { currentFile: path } : {}),
      };
    }),

  // ── active-tab mutations ──────────────────────────────────────────────────

  setSql: (sql) =>
    set((state) => ({
      sql,
      tabs: patchTab(state.tabs, state.activeTabId, { sql }),
    })),

  setSelectedSql: (selectedSql) => set({ selectedSql }),

  setResult: (result) =>
    set((state) => ({
      result,
      error: null,
      tabs: patchTab(state.tabs, state.activeTabId, { result, error: null }),
    })),

  setRunning: (isRunning) => set({ isRunning }),

  setTabRunning: (tabId, running) =>
    set((state) => ({
      // Update flat isRunning only when the tab being updated is the active one.
      isRunning: state.activeTabId === tabId ? running : state.isRunning,
      tabs: patchTab(state.tabs, tabId, { isRunning: running }),
    })),

  setError: (error) =>
    set((state) => ({
      error,
      isRunning: false,
      tabs: patchTab(state.tabs, state.activeTabId, { error }),
    })),

  executeWith: async (sql) => {
    set((state) => ({
      sql,
      selectedSql: "",
      isRunning: true,
      error: null,
      result: null,
      tabs: patchTab(state.tabs, state.activeTabId, { sql, result: null, error: null }),
    }));
    try {
      const res = await ExecuteQuery(sql);
      set((state) => ({
        result: res,
        error: null,
        tabs: patchTab(state.tabs, state.activeTabId, { result: res, error: null }),
      }));
    } catch (e) {
      set((state) => ({
        error: String(e),
        isRunning: false,
        tabs: patchTab(state.tabs, state.activeTabId, { error: String(e) }),
      }));
    } finally {
      set({ isRunning: false });
    }
  },

  executeInNewTab: (sql) => {
    const newTab = makeTab({ sql });
    set((state) => ({
      tabs: [...state.tabs, newTab],
      activeTabId: newTab.id,
      sql,
      selectedSql: "",
      currentFile: null,
      result: null,
      error: null,
      isRunning: false,
    }));
    // Ask QueryPage to run via its StartQuery/WaitForQueryResult path, which
    // is the only path that populates resultHistory and shows results in the UI.
    // The SQL is passed in the event detail to avoid stale-closure issues.
    window.dispatchEvent(new CustomEvent(EXECUTE_IN_TAB_EVENT, { detail: { sql } }));
  },

  loadInNewTab: (sql) => {
    const newTab = makeTab({ sql });
    set((state) => ({
      tabs: [...state.tabs, newTab],
      activeTabId: newTab.id,
      sql,
      selectedSql: "",
      currentFile: null,
      result: null,
      error: null,
      isRunning: false,
    }));
  },
}),
{
  name: "thaw-query-store",
  storage: createJSONStorage(() => localStorage),
  // Persist the canonical tab state and the flat active-tab aliases.
  // isRunning and selectedSql are intentionally excluded so they always
  // reset to safe defaults (false / "") after a page reload.
  // result is intentionally excluded from persistence — large result sets
  // (e.g. account_usage.query_history) exceed the storage quota and throw a
  // QuotaExceededError. Results are kept in memory during the session so
  // tab-switching still works; they are simply not restored after a reload.
  // For file-backed notebook tabs, sql/savedSql are cleared before persisting
  // (content can be large) and re-read from disk on startup by QueryPage.
  partialize: (state) => ({
    tabs: state.tabs.map((t) => ({
      ...t,
      isRunning: false,  // never persist running state
      result: null,
      diff: null,
      sql:      (t.kind === "notebook" && t.path) ? "" : t.sql,
      savedSql: (t.kind === "notebook" && t.path) ? "" : t.savedSql,
    })),
    activeTabId: state.activeTabId,
    sql: state.sql,
    result: null,
    error: state.error,
    currentFile: state.currentFile,
  }),
}
));
