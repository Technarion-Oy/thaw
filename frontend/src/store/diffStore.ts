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
// @thaw-domain: Schema Migration

import { create } from "zustand";
import { GetObjectDDL, GetRoleDDL, GetWarehouseDDL, ReadFile } from "../../wailsjs/go/app/App";
import { useQueryStore } from "./queryStore";
import { kindSupportsDdl } from "../utils/objectDdl";

export type DiffCategory = "obj" | "file" | "role" | "warehouse";

export interface PendingDiffItem {
  category: DiffCategory;
  label:    string;
  // Per-category fetch params (only relevant fields populated):
  db?:     string; schema?: string; kind?: string; args?: string; // obj
  name?:   string;                                                 // obj | role | warehouse
  path?:   string;                                                 // file
}

interface DiffState {
  pending:  PendingDiffItem | null;
  loading:  boolean;
  error:    string | null;

  selectForComparison: (item: PendingDiffItem) => void;
  compareWith:         (item: PendingDiffItem) => Promise<void>;
  clearError:          () => void;
}

async function fetchText(item: PendingDiffItem): Promise<string> {
  switch (item.category) {
    case "obj":
      // Defence-in-depth: the Sidebar menu already hides Compare for these kinds,
      // but guard here too so a doomed GET_DDL never reaches the driver (log noise).
      if (item.kind && !kindSupportsDdl(item.kind))
        throw new Error(`GET_DDL does not support ${item.kind} objects`);
      return GetObjectDDL(item.db ?? "", item.schema ?? "", item.kind ?? "", item.name ?? "", item.args ?? "");
    case "role":
      return GetRoleDDL(item.name ?? "");
    case "warehouse":
      return GetWarehouseDDL(item.name ?? "");
    case "file":
      return ReadFile(item.path ?? "");
  }
}

export const useDiffStore = create<DiffState>()((set, get) => ({
  pending:  null,
  loading:  false,
  error:    null,

  selectForComparison: (item) => {
    set({ pending: item });
  },

  compareWith: async (item) => {
    const left = get().pending;
    if (!left) return;

    set({ loading: true, error: null });
    try {
      const [leftText, rightText] = (await Promise.all([fetchText(left), fetchText(item)])).map((t) => t.trim());
      set({ loading: false, pending: null });
      useQueryStore.getState().openDiff(left.label, leftText, item.label, rightText);
    } catch (e) {
      set({ error: String(e), loading: false });
    }
  },

  clearError: () => set({ error: null }),
}));
