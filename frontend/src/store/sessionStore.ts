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
import {
  GetSessionContext,
  ListRoles,
  ListWarehouses,
  UseRole,
  UseWarehouse,
} from "../../wailsjs/go/main/App";

interface SessionState {
  role: string;
  warehouse: string;
  roles: string[];
  warehouses: string[];
  loadingContext: boolean;
  loadingRoles: boolean;
  loadingWarehouses: boolean;
  switchingRole: boolean;
  switchingWarehouse: boolean;
  error: string | null;

  loadContext: () => Promise<void>;
  loadRoles: () => Promise<void>;
  loadWarehouses: () => Promise<void>;
  switchRole: (role: string) => Promise<void>;
  switchWarehouse: (warehouse: string) => Promise<void>;
  clearError: () => void;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  role: "",
  warehouse: "",
  roles: [],
  warehouses: [],
  loadingContext: false,
  loadingRoles: false,
  loadingWarehouses: false,
  switchingRole: false,
  switchingWarehouse: false,
  error: null,

  loadContext: async () => {
    set({ loadingContext: true });
    try {
      const ctx = await GetSessionContext();
      set({ role: ctx.role, warehouse: ctx.warehouse });
    } catch {
      // silently ignore — might not be connected yet
    } finally {
      set({ loadingContext: false });
    }
  },

  loadRoles: async () => {
    if (get().roles.length > 0) return;
    set({ loadingRoles: true });
    try {
      const roles = await ListRoles();
      set({ roles });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ loadingRoles: false });
    }
  },

  loadWarehouses: async () => {
    if (get().warehouses.length > 0) return;
    set({ loadingWarehouses: true });
    try {
      const warehouses = await ListWarehouses();
      set({ warehouses });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ loadingWarehouses: false });
    }
  },

  switchRole: async (role) => {
    set({ switchingRole: true, error: null });
    try {
      await UseRole(role);
      set({ role });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingRole: false });
    }
  },

  switchWarehouse: async (warehouse) => {
    set({ switchingWarehouse: true, error: null });
    try {
      await UseWarehouse(warehouse);
      set({ warehouse });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingWarehouse: false });
    }
  },

  clearError: () => set({ error: null }),
}));
