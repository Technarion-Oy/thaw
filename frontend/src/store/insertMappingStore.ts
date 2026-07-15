// SPDX-License-Identifier: GPL-3.0-or-later

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
