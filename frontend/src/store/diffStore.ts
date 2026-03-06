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
import { GetObjectDDL, GetRoleDDL, GetWarehouseDDL, ReadFile } from "../../wailsjs/go/main/App";

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
  pending:    PendingDiffItem | null;
  isOpen:     boolean;
  leftText:   string;  rightText:  string;
  leftLabel:  string;  rightLabel: string;
  loading:    boolean;  error: string | null;

  selectForComparison: (item: PendingDiffItem) => void;
  compareWith:         (item: PendingDiffItem) => Promise<void>;
  close:               () => void;
}

async function fetchText(item: PendingDiffItem): Promise<string> {
  switch (item.category) {
    case "obj":
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
  pending:    null,
  isOpen:     false,
  leftText:   "",
  rightText:  "",
  leftLabel:  "",
  rightLabel: "",
  loading:    false,
  error:      null,

  selectForComparison: (item) => {
    set({ pending: item });
  },

  compareWith: async (item) => {
    const left = get().pending;
    if (!left) return;

    set({
      isOpen:     true,
      loading:    true,
      error:      null,
      leftLabel:  left.label,
      rightLabel: item.label,
      leftText:   "",
      rightText:  "",
    });

    try {
      const [leftText, rightText] = (await Promise.all([fetchText(left), fetchText(item)])).map((t) => t.trim());
      set({ leftText, rightText, loading: false, pending: null });
    } catch (e) {
      set({ error: String(e), loading: false });
    }
  },

  close: () => {
    set({ isOpen: false, leftText: "", rightText: "", leftLabel: "", rightLabel: "", error: null });
  },
}));
