// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: MCP Server

import { create } from "zustand";
import { ListMCPSessions } from "../../wailsjs/go/app/App";
import type { mcp } from "../../wailsjs/go/models";

interface MCPState {
  sessions: mcp.SessionInfo[];
  /** Reload the running-session list from the backend. */
  refresh: () => Promise<void>;
}

export const useMCPStore = create<MCPState>((set) => ({
  sessions: [],
  refresh: async () => {
    try {
      const sessions = await ListMCPSessions();
      set({ sessions: sessions ?? [] });
    } catch {
      set({ sessions: [] });
    }
  },
}));
