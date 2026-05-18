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

interface FocusedCell {
  rowIndex: number;
  columnId: string;
}

interface SelectionRange {
  startRow: number;
  endRow: number;
  startCol: number;
  endCol: number;
}

interface PivotConfig {
  rowGroups: string[];
  values: string[];
}

interface GridState {
  focusedCell: FocusedCell | null;
  selectionRange: SelectionRange | null;
  pivotConfig: PivotConfig;
  setFocusedCell: (cell: FocusedCell | null) => void;
  setSelectionRange: (range: SelectionRange | null) => void;
  setPivotConfig: (config: PivotConfig) => void;
}

export const useGridStore = create<GridState>()((set) => ({
  focusedCell: null,
  selectionRange: null,
  pivotConfig: { rowGroups: [], values: [] },
  setFocusedCell: (cell) => set({ focusedCell: cell }),
  setSelectionRange: (range) => set({ selectionRange: range }),
  setPivotConfig: (config) => set({ pivotConfig: config }),
}));
