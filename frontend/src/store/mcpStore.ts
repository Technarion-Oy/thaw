// SPDX-License-Identifier: GPL-3.0-or-later
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
