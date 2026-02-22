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
import { ExecuteQuery } from "../../wailsjs/go/main/App";

export interface QueryResult {
  columns: string[];
  rows: unknown[][];
  rowsAffected: number;
}

export interface Tab {
  id: string;
  path: string | null;   // null = unsaved scratch tab
  title: string;
  sql: string;
  result: QueryResult | null;
  error: string | null;
}

// ── helpers ───────────────────────────────────────────────────────────────────

function makeTab(overrides?: Partial<Tab>): Tab {
  return {
    id: crypto.randomUUID(),
    path: null,
    title: "SQL",
    sql: "",
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

  // Tab management
  activateTab: (id: string) => void;
  openFile: (path: string, content: string) => void;
  openScratch: () => void;
  closeTab: (id: string) => void;

  // Active-tab mutations (also kept in the tabs array for restoration on switch)
  setSql: (sql: string) => void;
  setSelectedSql: (selected: string) => void;
  setResult: (result: QueryResult) => void;
  setRunning: (isRunning: boolean) => void;
  setError: (error: string | null) => void;
  executeWith: (sql: string) => Promise<void>;
}

// ── store ─────────────────────────────────────────────────────────────────────

const initialTab = makeTab({
  sql: "SELECT CURRENT_USER(), CURRENT_WAREHOUSE(), CURRENT_DATABASE();",
});

export const useQueryStore = create<QueryState>((set) => ({
  tabs: [initialTab],
  activeTabId: initialTab.id,

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
        title: path.split("/").pop() ?? path,
        sql: content,
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

  closeTab: (id) =>
    set((state) => {
      if (state.tabs.length <= 1) return {};
      const idx = state.tabs.findIndex((t) => t.id === id);
      const newTabs = state.tabs.filter((t) => t.id !== id);

      if (id !== state.activeTabId) {
        return { tabs: newTabs };
      }

      // Closing the active tab — move to the nearest neighbour
      const next = newTabs[Math.min(idx, newTabs.length - 1)];
      return {
        tabs: newTabs,
        activeTabId: next.id,
        sql: next.sql,
        selectedSql: "",
        currentFile: next.path,
        result: next.result,
        error: next.error,
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
}));
