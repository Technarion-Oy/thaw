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
  source: TableInfo | null;
  modalOpen: boolean;

  setTarget: (target: TableInfo) => void;
  setSource: (source: TableInfo) => void;
  setModalOpen: (open: boolean) => void;
  reset: () => void;
}

export const useInsertMappingStore = create<InsertMappingState>((set) => ({
  target: null,
  source: null,
  modalOpen: false,

  setTarget: (target) => set({ target }),
  setSource: (source) => set({ source, modalOpen: true }),
  setModalOpen: (modalOpen) => set({ modalOpen }),
  reset: () => set({ target: null, source: null, modalOpen: false }),
}));
