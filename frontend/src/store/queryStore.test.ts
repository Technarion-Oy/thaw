// SPDX-License-Identifier: GPL-3.0-or-later

import { describe, it, expect, beforeEach, vi } from "vitest";

// The store persists to localStorage and wires window/document listeners at
// module load. The vitest environment is "node", so stub the browser globals
// the store touches *before* importing it (via the dynamic import in loadStore).
function stubBrowserGlobals(): void {
  const store = new Map<string, string>();
  const localStorage = {
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    setItem: (k: string, v: string) => void store.set(k, v),
    removeItem: (k: string) => void store.delete(k),
    clear: () => store.clear(),
  };
  const noopTarget = { addEventListener: () => {}, removeEventListener: () => {}, dispatchEvent: () => true };
  Object.assign(globalThis, {
    localStorage,
    window: { ...noopTarget, localStorage },
    document: { ...noopTarget, visibilityState: "visible" },
  });
}

// Fresh module instance per test so each starts from the store's initial tab.
async function loadStore() {
  vi.resetModules();
  stubBrowserGlobals();
  const mod = await import("./queryStore");
  return mod.useQueryStore;
}

describe("queryStore.loadInNewTab", () => {
  beforeEach(() => stubBrowserGlobals());

  it("opens a new active tab without overwriting the previously-active tab (#830)", async () => {
    const useQueryStore = await loadStore();

    // Simulate an active tab holding unsaved SQL (sql !== savedSql).
    const original = useQueryStore.getState().activeTabId;
    useQueryStore.getState().setSql("SELECT unsaved_work();");
    expect(useQueryStore.getState().tabs.find((t) => t.id === original)?.sql).toBe("SELECT unsaved_work();");

    const before = useQueryStore.getState().tabs.length;
    useQueryStore.getState().loadInNewTab("SELECT * FROM history_query;");
    const state = useQueryStore.getState();

    // A new tab was appended and activated — the original tab is not the active one.
    expect(state.tabs.length).toBe(before + 1);
    expect(state.activeTabId).not.toBe(original);
    expect(state.tabs[state.tabs.length - 1].id).toBe(state.activeTabId);

    // The new tab carries the history SQL...
    expect(state.tabs.find((t) => t.id === state.activeTabId)?.sql).toBe("SELECT * FROM history_query;");
    // ...and the previously-active tab's unsaved SQL is untouched.
    expect(state.tabs.find((t) => t.id === original)?.sql).toBe("SELECT unsaved_work();");
  });
});
