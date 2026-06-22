// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { create } from "zustand";

// Shared open/close state for the centralized Tag Management view. The view is
// a single modal rendered once (in QueryPage) but reachable from several places
// — the Tools menu and the Tags group in the object browser — so its visibility
// lives in a store rather than in any one component.
interface TagManagementState {
  open: boolean;
  openView: () => void;
  closeView: () => void;
}

export const useTagManagementStore = create<TagManagementState>((set) => ({
  open: false,
  openView: () => set({ open: true }),
  closeView: () => set({ open: false }),
}));
