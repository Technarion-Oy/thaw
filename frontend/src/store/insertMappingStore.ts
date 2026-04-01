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

export interface TableInfo {
  db: string;
  schema: string;
  name: string;
}

interface InsertMappingState {
  target: TableInfo | null;
  sources: TableInfo[];
  modalOpen: boolean;

  setTarget: (target: TableInfo) => void;
  addSource: (source: TableInfo) => void;
  removeSource: (index: number) => void;
  setModalOpen: (open: boolean) => void;
  reset: () => void;
}

export const useInsertMappingStore = create<InsertMappingState>((set) => ({
  target: null,
  sources: [],
  modalOpen: false,

  setTarget: (target) => set({ target }),
  addSource: (source) =>
    set((s) => ({ sources: [...s.sources, source], modalOpen: true })),
  removeSource: (index) =>
    set((s) => ({ sources: s.sources.filter((_, i) => i !== index) })),
  setModalOpen: (modalOpen) => set({ modalOpen }),
  reset: () => set({ target: null, sources: [], modalOpen: false }),
}));
