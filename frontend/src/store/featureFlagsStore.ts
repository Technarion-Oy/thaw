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
import { GetFeatureFlags } from "../../wailsjs/go/main/App";
import type { config } from "../../wailsjs/go/models";

interface FeatureFlagsState {
  flags: config.FeatureFlags;
  /** Reload flags from the backend (call after SaveFeatureFlags). */
  load: () => Promise<void>;
}

export const useFeatureFlagsStore = create<FeatureFlagsState>((set) => ({
  // Optimistic default: every feature enabled until the backend responds.
  flags: { initialized: true, exportTableData: true },
  load: async () => {
    const flags = await GetFeatureFlags();
    set({ flags });
  },
}));
