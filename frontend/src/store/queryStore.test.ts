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

describe("queryStore preview tabs (#849)", () => {
  beforeEach(() => stubBrowserGlobals());

  it("opens a single preview tab and reuses it for the next file", async () => {
    const useQueryStore = await loadStore();
    const before = useQueryStore.getState().tabs.length;

    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", true);
    let state = useQueryStore.getState();
    const previewId = state.activeTabId;
    const previewTab = state.tabs.find((t) => t.id === previewId)!;
    expect(state.tabs.length).toBe(before + 1);
    expect(previewTab.preview).toBe(true);
    expect(previewTab.path).toBe("/tmp/a.sql");

    // Opening another file as preview reuses the same tab (no new tab appended).
    useQueryStore.getState().openFile("/tmp/b.sql", "-- b", true);
    state = useQueryStore.getState();
    expect(state.tabs.length).toBe(before + 1);
    expect(state.activeTabId).toBe(previewId); // same tab, replaced content
    const reused = state.tabs.find((t) => t.id === previewId)!;
    expect(reused.path).toBe("/tmp/b.sql");
    expect(reused.sql).toBe("-- b");
    expect(reused.preview).toBe(true);
  });

  it("promotes the preview tab on edit and keeps the next preview separate", async () => {
    const useQueryStore = await loadStore();

    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", true);
    const previewId = useQueryStore.getState().activeTabId;

    // Editing the active preview tab pins it (clears the preview flag).
    useQueryStore.getState().setSql("-- a edited");
    let state = useQueryStore.getState();
    expect(state.tabs.find((t) => t.id === previewId)?.preview).toBeFalsy();

    // A dirty preview is never replaced: opening a new file makes a fresh preview tab.
    useQueryStore.getState().openFile("/tmp/b.sql", "-- b", true);
    state = useQueryStore.getState();
    const newPreviewId = state.activeTabId;
    expect(newPreviewId).not.toBe(previewId);
    expect(state.tabs.find((t) => t.id === previewId)?.path).toBe("/tmp/a.sql"); // still open
    expect(state.tabs.find((t) => t.id === newPreviewId)?.preview).toBe(true);
    expect(state.tabs.filter((t) => t.preview).length).toBe(1); // only one preview ever
  });

  it("promoteTab pins a preview tab; a permanent open of the same file promotes it", async () => {
    const useQueryStore = await loadStore();

    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", true);
    const previewId = useQueryStore.getState().activeTabId;

    useQueryStore.getState().promoteTab(previewId);
    expect(useQueryStore.getState().tabs.find((t) => t.id === previewId)?.preview).toBeFalsy();

    // Re-open the same file as preview: the existing permanent tab is reused, never
    // demoted back to preview.
    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", true);
    const state = useQueryStore.getState();
    expect(state.activeTabId).toBe(previewId);
    expect(state.tabs.find((t) => t.id === previewId)?.preview).toBeFalsy();
  });

  it("does not recycle a preview tab that has a query running (promotes it instead)", async () => {
    const useQueryStore = await loadStore();
    const before = useQueryStore.getState().tabs.length;

    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", true);
    const runningId = useQueryStore.getState().activeTabId;
    // A running query is bound to this tab id, so browsing away must not swap the
    // tab's file out from under it — the preview is promoted and a fresh one opened.
    useQueryStore.getState().setTabRunning(runningId, true);

    useQueryStore.getState().openFile("/tmp/b.sql", "-- b", true);
    const state = useQueryStore.getState();
    expect(state.tabs.length).toBe(before + 2); // new preview appended, not reused
    expect(state.activeTabId).not.toBe(runningId);
    const running = state.tabs.find((t) => t.id === runningId)!;
    expect(running.path).toBe("/tmp/a.sql"); // still shows its own file
    expect(running.preview).toBeFalsy();     // promoted
    expect(state.tabs.find((t) => t.id === state.activeTabId)?.preview).toBe(true);
  });

  it("does not recycle a preview tab that is the current split target (promotes it)", async () => {
    const useQueryStore = await loadStore();
    const before = useQueryStore.getState().tabs.length;

    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", true);
    const splitId = useQueryStore.getState().activeTabId;
    // Split another editor with the preview tab — recycling it in place would pull
    // the split pane onto the new file (and collapse both panes onto one tab).
    useQueryStore.getState().setSplitTab(splitId);

    useQueryStore.getState().openFile("/tmp/b.sql", "-- b", true);
    const state = useQueryStore.getState();
    expect(state.tabs.length).toBe(before + 2); // fresh preview appended, split tab kept
    expect(state.splitTabId).toBe(splitId);     // split still points at its own tab
    const split = state.tabs.find((t) => t.id === splitId)!;
    expect(split.path).toBe("/tmp/a.sql");      // still shows its own file
    expect(split.preview).toBeFalsy();          // promoted
    expect(state.tabs.find((t) => t.id === state.activeTabId)?.preview).toBe(true);
  });

  it("a permanent open (double-click) of a previewed file promotes it in place", async () => {
    const useQueryStore = await loadStore();
    const before = useQueryStore.getState().tabs.length;

    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", true);
    const previewId = useQueryStore.getState().activeTabId;

    useQueryStore.getState().openFile("/tmp/a.sql", "-- a", false); // permanent open
    const state = useQueryStore.getState();
    expect(state.tabs.length).toBe(before + 1); // reused, not duplicated
    expect(state.tabs.find((t) => t.id === previewId)?.preview).toBeFalsy();
  });
});
