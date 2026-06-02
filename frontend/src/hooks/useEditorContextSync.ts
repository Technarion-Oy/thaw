// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: MCP Server

import { useEffect, useRef } from "react";
import { useQueryStore, type QueryResult, type Tab } from "../store/queryStore";
import {
  UpdateEditorContext,
  UpdateEditorTabSQL,
  UpdateQueryResult,
  ClearQueryResult,
  RemoveEditorTab,
} from "../../wailsjs/go/app/App";

/**
 * Syncs the editor's active tab, SQL content, and query results to the
 * backend EditorContextStore so MCP tool handlers can read them.
 *
 * Mount once in QueryPage. All calls are fire-and-forget — failures are
 * silently ignored since the MCP bridge is best-effort.
 */
export function useEditorContextSync(): void {
  const debounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const prevActiveTabRef = useRef<string>("");
  const prevSqlRef = useRef<string>("");
  const prevResultRef = useRef<QueryResult | null>(null);
  const prevTabIdsRef = useRef<Set<string>>(new Set());

  useEffect(() => {
    // Fire immediately with current state.
    const initial = useQueryStore.getState();
    const initialTab = initial.tabs.find((t: Tab) => t.id === initial.activeTabId);
    prevActiveTabRef.current = initial.activeTabId;
    prevSqlRef.current = initialTab?.sql ?? "";
    prevTabIdsRef.current = new Set(initial.tabs.map((t: Tab) => t.id));
    UpdateEditorContext(initial.activeTabId, initialTab?.sql ?? "").catch(() => {});

    const unsub = useQueryStore.subscribe((state) => {
      const activeTab = state.tabs.find((t: Tab) => t.id === state.activeTabId);
      const sql = activeTab?.sql ?? "";
      const result = activeTab?.result ?? null;

      // Active tab changed — cancel any pending debounce from the old tab
      // since UpdateEditorContext already sends the new tab's SQL.
      if (state.activeTabId !== prevActiveTabRef.current) {
        if (debounceRef.current) clearTimeout(debounceRef.current);
        prevActiveTabRef.current = state.activeTabId;
        prevSqlRef.current = sql;
        UpdateEditorContext(state.activeTabId, sql).catch(() => {});
      }

      // SQL changed (debounced). Capture tabId at schedule time so a
      // tab switch within the 300ms window doesn't send stale SQL to
      // the wrong tab.
      if (sql !== prevSqlRef.current) {
        prevSqlRef.current = sql;
        if (debounceRef.current) clearTimeout(debounceRef.current);
        const currentTabId = state.activeTabId;
        debounceRef.current = setTimeout(() => {
          UpdateEditorTabSQL(currentTabId, sql).catch(() => {});
        }, 300);
      }

      // Result changed.
      if (result !== prevResultRef.current) {
        prevResultRef.current = result;
        if (result) {
          const sampleRows = (result.rows ?? []).slice(0, 5);
          UpdateQueryResult(
            state.activeTabId,
            result.columns ?? [],
            result.rows?.length ?? 0,
            result.truncated ?? false,
            sampleRows,
            result.queryID ?? "",
          ).catch(() => {});
        } else {
          // Result cleared (new query started) — tell the backend so
          // MCP clients don't see stale results from a previous run.
          ClearQueryResult(state.activeTabId).catch(() => {});
        }
      }

      // Tab removals.
      const currentIds = new Set(state.tabs.map((t: Tab) => t.id));
      prevTabIdsRef.current.forEach((id) => {
        if (!currentIds.has(id)) {
          RemoveEditorTab(id).catch(() => {});
        }
      });
      prevTabIdsRef.current = currentIds;
    });

    return () => {
      unsub();
      if (debounceRef.current) clearTimeout(debounceRef.current);
    };
  }, []);
}
