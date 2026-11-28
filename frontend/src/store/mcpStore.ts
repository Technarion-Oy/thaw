// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: MCP Server

import { create } from "zustand";
import { ListMCPSessions } from "../../bindings/thaw/internal/app/app";
import type * as mcp from "../../bindings/thaw/internal/mcp/models";

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
