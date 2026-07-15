// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

import { create } from "zustand";
import { persist, createJSONStorage, type StateStorage } from "zustand/middleware";
import { ExecuteQuery } from "../../wailsjs/go/app/App";

// Wraps localStorage to swallow QuotaExceededError on setItem.
// Persistence is best-effort — the in-memory store remains authoritative.
const safeLocalStorage: StateStorage = {
  getItem: (name) => localStorage.getItem(name),
  setItem: (name, value) => {
    try {
      localStorage.setItem(name, value);
    } catch {
      // QuotaExceededError — silently drop; data lives in memory.
    }
  },
  removeItem: (name) => localStorage.removeItem(name),
};

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
  kind?: "sql" | "notebook" | "yaml" | "python" | "markdown" | "plaintext"; // defaults to "sql" when absent (backward compat)
  path: string | null;   // null = unsaved scratch tab
  title: string;
  sql: string;
  savedSql: string;      // content at last open/save; compare to sql to derive isDirty
  result: QueryResult | null;
  error: string | null;
  diff?: TabDiff | null; // populated for diff tabs; absent for regular SQL tabs
  isRunning?: boolean;   // per-tab running state; never persisted
  orphaned?: boolean;    // true when the backing file was deleted from disk
  mcpOrigin?: boolean;   // true when the tab was created by an MCP tool
  isDefaultTitle?: boolean; // true while the title is the auto-generated "SQL (n)";
                            // cleared by renameTab. Drives the "untitled.sql" save default.
}

// ── helpers ───────────────────────────────────────────────────────────────────

/** Infer the tab kind from a file path's extension. */
function kindFromPath(path: string): Tab["kind"] {
  const ext = path.split(".").pop()?.toLowerCase();
  if (ext === "py") return "python";
  if (ext === "yml" || ext === "yaml") return "yaml";
  if (ext === "md" || ext === "markdown") return "markdown";
  if (ext === "sql") return undefined; // treated as "sql"
  // Any other text file: open as plaintext (no SQL highlighting/autocomplete).
  // The slim Monaco build registers only sql/python/yaml/markdown grammars;
  // mapping more extensions needs grammar registration in monacoSetup.ts.
  return "plaintext";
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

/** Next scratch-tab title "SQL (n)", where n is one past the highest existing. */
function nextScratchTitle(tabs: Tab[]): string {
  let max = 0;
  for (const t of tabs) {
    const m = /^SQL \((\d+)\)$/.exec(t.title);
    if (m) max = Math.max(max, parseInt(m[1], 10));
  }
  return `SQL (${max + 1})`;
}

/** A scratch tab with the next auto-generated "SQL (n)" title, flagged as default. */
function makeScratchTab(tabs: Tab[], overrides?: Partial<Tab>): Tab {
  return makeTab({ title: nextScratchTitle(tabs), isDefaultTitle: true, ...overrides });
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
  // Rename a tab's title. Only meaningful for non-file tabs (file tabs derive
  // their title from the path); a blank title is ignored.
  renameTab: (id: string, title: string) => void;
  // Called after a successful save to update the tab's path/title and clear dirty state.
  markSaved: (id: string, path: string, title: string) => void;
  // Called after a rename to update the tab's path/title without clearing dirty state.
  updateTabPath: (id: string, path: string, title: string) => void;
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
  openMcpTab: (title: string, sql: string) => string;
  openMcpNotebookTab: (title: string, content: string) => string;
}

// ── store ─────────────────────────────────────────────────────────────────────

const INITIAL_SQL = "SELECT CURRENT_USER(), CURRENT_WAREHOUSE(), CURRENT_DATABASE();";

const initialTab = makeScratchTab([], { sql: INITIAL_SQL, savedSql: INITIAL_SQL });

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
      const newTab = makeScratchTab(state.tabs);
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

  renameTab: (id, title) =>
    set((state) => {
      const t = title.trim();
      if (!t) return {};
      return { tabs: patchTab(state.tabs, id, { title: t, isDefaultTitle: false }) };
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
      // Disk content already matches the tab's saved baseline — nothing to do.
      // Also short-circuits the app's own saves: the watcher re-fires ~200 ms
      // after we write, and re-applying identical content would churn editor state.
      if (diskContent === tab.savedSql) return {};
      // VSCode-style: a clean tab adopts the new disk content; a dirty tab keeps
      // the user's unsaved edits (`sql`) but still advances `savedSql` to the new
      // disk baseline. Advancing it keeps the tab correctly dirty *vs. the current
      // file* — otherwise undoing back to the original load would make the tab look
      // clean and a save would silently clobber the external change.
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
          ? { ...t, path: null, orphaned: true, savedSql: "" }
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

      // Closing the last tab — replace it with a fresh scratch tab so the
      // editor is never left empty. Pass newTabs (empty) so the "SQL (n)"
      // number resets to 1 instead of climbing on every close (#595).
      if (newTabs.length === 0) {
        const freshTab = makeScratchTab(newTabs);
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

  markSaved: (id, path, title) => {
    set((state) => {
      const tab = state.tabs.find((t) => t.id === id);
      const savedSql = tab?.sql ?? "";
      const updatedTabs = patchTab(state.tabs, id, { path, title, savedSql, isDefaultTitle: false });
      const isActive = state.activeTabId === id;
      return {
        tabs: updatedTabs,
        ...(isActive ? { currentFile: path } : {}),
      };
    });
    // Writing the file may have changed its git status — let the file explorer
    // refresh its status colors without a manual tree refresh. The path lets the
    // explorer suppress the watcher's echo of our own write (no tree re-list).
    if (typeof window !== "undefined") {
      window.dispatchEvent(new CustomEvent("thaw:file-saved", { detail: { path } }));
    }
  },

  updateTabPath: (id, path, title) =>
    set((state) => {
      const updatedTabs = patchTab(state.tabs, id, { path, title, isDefaultTitle: false });
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
    set((state) => {
      const newTab = makeScratchTab(state.tabs, { sql });
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
        sql,
        selectedSql: "",
        currentFile: null,
        result: null,
        error: null,
        isRunning: false,
      };
    });
    // Ask QueryPage to run via its StartQuery/WaitForQueryResult path, which
    // is the only path that populates resultHistory and shows results in the UI.
    // The SQL is passed in the event detail to avoid stale-closure issues.
    window.dispatchEvent(new CustomEvent(EXECUTE_IN_TAB_EVENT, { detail: { sql } }));
  },

  loadInNewTab: (sql) => {
    set((state) => {
      const newTab = makeScratchTab(state.tabs, { sql });
      return {
        tabs: [...state.tabs, newTab],
        activeTabId: newTab.id,
        sql,
        selectedSql: "",
        currentFile: null,
        result: null,
        error: null,
        isRunning: false,
      };
    });
  },

  openMcpTab: (title, sql) => {
    // savedSql: "" makes the tab "dirty" (sql !== savedSql) so the user sees
    // a close-confirmation dialog — intentional because MCP-delivered SQL is
    // not persisted to any file and should not be silently discarded.
    const newTab = makeTab({ title, sql, savedSql: "", mcpOrigin: true });
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
    return newTab.id;
  },

  openMcpNotebookTab: (title, content) => {
    // Notebook tabs store nbformat JSON in the `sql` field — this is the
    // existing convention used by openNotebook/openNotebookUnsaved.
    const newTab = makeTab({ kind: "notebook", title, sql: content, savedSql: "", mcpOrigin: true });
    set((state) => ({
      tabs: [...state.tabs, newTab],
      activeTabId: newTab.id,
      sql: content,
      selectedSql: "",
      currentFile: null,
      result: null,
      error: null,
      isRunning: false,
    }));
    return newTab.id;
  },
}),
{
  name: "thaw-query-store",
  storage: createJSONStorage(() => safeLocalStorage),
  // Persist the canonical tab state and the flat active-tab aliases.
  // isRunning and selectedSql are intentionally excluded so they always
  // reset to safe defaults (false / "") after a page reload.
  // result is intentionally excluded from persistence — large result sets
  // (e.g. account_usage.query_history) exceed the storage quota and throw a
  // QuotaExceededError. Results are kept in memory during the session so
  // tab-switching still works; they are simply not restored after a reload.
  // For file-backed tabs, sql/savedSql are cleared before persisting
  // (content can be large and exceeds localStorage quota) and re-read from
  // disk on startup by QueryPage.
  partialize: (state) => {
    const activeTab = state.tabs.find((t) => t.id === state.activeTabId);
    const activeIsFile = activeTab?.path;
    return {
      tabs: state.tabs.map((t) => ({
        ...t,
        isRunning: false,  // never persist running state
        result: null,
        diff: null,
        mcpOrigin: undefined, // MCP origin is session-only; don't persist
        sql:      t.path ? "" : t.sql,
        savedSql: t.path ? "" : t.savedSql,
      })),
      activeTabId: state.activeTabId,
      sql: activeIsFile ? "" : state.sql,
      result: null,
      error: state.error,
      currentFile: state.currentFile,
    };
  },
}
));
