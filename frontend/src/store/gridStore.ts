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
// @thaw-domain: SQL Editor & Diagnostics

import { create } from "zustand";
import type { Row } from "@tanstack/react-table";

// ─── Selection ────────────────────────────────────────────────────────────────

export interface SelectionRange {
  startRow: number;
  endRow: number;
  startCol: number;
  endCol: number;
}

// ─── Search ───────────────────────────────────────────────────────────────────

export interface CellCoord {
  row: number;
  col: number;
}

// ─── Column Formatting ────────────────────────────────────────────────────────

export type FormatType = "number" | "currency" | "percentage" | "datetime";

export interface FormatConfig {
  type: FormatType;
  locale?: string;
  decimals?: number;
  currency?: string;
  timezone?: "utc" | "local";
}

// ─── Conditional Formatting ───────────────────────────────────────────────────

export type ConditionalRuleType = "colorScale" | "dataBar" | "textMatch";

export interface ColorScaleRule {
  type: "colorScale";
  minColor: string;
  maxColor: string;
}

export interface DataBarRule {
  type: "dataBar";
  color: string;
}

export interface TextMatchRule {
  type: "textMatch";
  pattern: string;
  backgroundColor: string;
  textColor: string;
}

export type ConditionalRule = ColorScaleRule | DataBarRule | TextMatchRule;

// ─── Store ────────────────────────────────────────────────────────────────────

interface GridState {
  // Filtered/sorted rows from TanStack table model (set by ResultGrid)
  tableRows: Row<unknown[]>[] | null;
  setTableRows: (rows: Row<unknown[]>[]) => void;

  // Range selection
  selectionRange: SelectionRange | null;
  isSelecting: boolean;
  setSelectionRange: (range: SelectionRange | null) => void;
  setIsSelecting: (v: boolean) => void;

  // Search
  searchTerm: string;
  searchMatches: CellCoord[];
  currentMatchIndex: number;
  setSearchTerm: (term: string) => void;
  setSearchMatches: (matches: CellCoord[]) => void;
  setCurrentMatchIndex: (index: number) => void;
  nextMatch: () => void;
  prevMatch: () => void;

  // Column formatting
  columnFormats: Record<string, FormatConfig>;
  setColumnFormat: (colId: string, format: FormatConfig) => void;
  clearColumnFormat: (colId: string) => void;

  // Conditional formatting
  conditionalRules: Record<string, ConditionalRule[]>;
  setConditionalRules: (colId: string, rules: ConditionalRule[]) => void;
  clearConditionalRules: (colId: string) => void;

  // Reset navigation state only (selection, search) — preserves formatting
  resetNavigation: () => void;
  // Reset all state including formatting (called when column schema changes)
  reset: () => void;
}

const initialState = {
  tableRows: null as Row<unknown[]>[] | null,
  selectionRange: null as SelectionRange | null,
  isSelecting: false,
  searchTerm: "",
  searchMatches: [] as CellCoord[],
  currentMatchIndex: 0,
  columnFormats: {} as Record<string, FormatConfig>,
  conditionalRules: {} as Record<string, ConditionalRule[]>,
};

export const useGridStore = create<GridState>((set, get) => ({
  ...initialState,

  setTableRows: (rows) => set({ tableRows: rows }),
  setSelectionRange: (range) => set({ selectionRange: range }),
  setIsSelecting: (v) => set({ isSelecting: v }),

  setSearchTerm: (term) => set({ searchTerm: term, currentMatchIndex: 0 }),
  setSearchMatches: (matches) => set({ searchMatches: matches }),
  setCurrentMatchIndex: (index) => set({ currentMatchIndex: index }),
  nextMatch: () => {
    const { searchMatches, currentMatchIndex } = get();
    if (searchMatches.length === 0) return;
    set({ currentMatchIndex: (currentMatchIndex + 1) % searchMatches.length });
  },
  prevMatch: () => {
    const { searchMatches, currentMatchIndex } = get();
    if (searchMatches.length === 0) return;
    set({ currentMatchIndex: (currentMatchIndex - 1 + searchMatches.length) % searchMatches.length });
  },

  setColumnFormat: (colId, format) =>
    set((s) => ({ columnFormats: { ...s.columnFormats, [colId]: format } })),
  clearColumnFormat: (colId) =>
    set((s) => {
      const { [colId]: _, ...rest } = s.columnFormats;
      return { columnFormats: rest };
    }),

  setConditionalRules: (colId, rules) =>
    set((s) => ({ conditionalRules: { ...s.conditionalRules, [colId]: rules } })),
  clearConditionalRules: (colId) =>
    set((s) => {
      const { [colId]: _, ...rest } = s.conditionalRules;
      return { conditionalRules: rest };
    }),

  resetNavigation: () => set({
    tableRows: null,
    selectionRange: null,
    isSelecting: false,
    searchTerm: "",
    searchMatches: [],
    currentMatchIndex: 0,
  }),
  reset: () => set(initialState),
}));
