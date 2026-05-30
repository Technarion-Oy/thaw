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
  ListAvailableRoles,
  ListWarehouses,
  ListDatabases,
  ListSchemas,
  UseRole,
  UseWarehouse,
  UseDatabase,
  UseSchema,
} from "../../wailsjs/go/app/App";
import { useQueryStore } from "./queryStore";

export interface TabSessionContext {
  role: string;
  warehouse: string;
  database: string;
  schema: string;
}

interface SessionState {
  role: string;
  warehouse: string;
  database: string;
  schema: string;
  roles: string[];
  warehouses: string[];
  databases: string[];
  schemas: string[];
  schemasForDatabase: string; // tracks which db the schemas[] list belongs to
  loadingContext: boolean;
  loadingRoles: boolean;
  loadingWarehouses: boolean;
  loadingDatabases: boolean;
  loadingSchemas: boolean;
  switchingRole: boolean;
  switchingWarehouse: boolean;
  switchingDatabase: boolean;
  switchingSchema: boolean;
  error: string | null;

  // Per-tab session contexts.
  tabContexts: Record<string, TabSessionContext>;

  loadContext: (tabId: string) => Promise<void>;
  setActiveTab: (tabId: string) => void;
  loadRoles: () => Promise<void>;
  loadWarehouses: () => Promise<void>;
  loadDatabases: () => Promise<void>;
  loadSchemas: () => Promise<void>;
  switchRole: (role: string) => Promise<void>;
  switchWarehouse: (warehouse: string) => Promise<void>;
  switchDatabase: (database: string) => Promise<void>;
  switchSchema: (schema: string) => Promise<void>;
  clearError: () => void;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  role: "",
  warehouse: "",
  database: "",
  schema: "",
  roles: [],
  warehouses: [],
  databases: [],
  schemas: [],
  schemasForDatabase: "",
  loadingContext: false,
  loadingRoles: false,
  loadingWarehouses: false,
  loadingDatabases: false,
  loadingSchemas: false,
  switchingRole: false,
  switchingWarehouse: false,
  switchingDatabase: false,
  switchingSchema: false,
  error: null,

  tabContexts: {},

  loadContext: async (tabId: string) => {
    set({ loadingContext: true });
    try {
      const ctx = await GetSessionContext(tabId);
      const tabCtx: TabSessionContext = {
        role: ctx.role ?? "",
        warehouse: ctx.warehouse ?? "",
        database: ctx.database ?? "",
        schema: ctx.schema ?? "",
      };
      const prev = get();
      const isActiveTab = useQueryStore.getState().activeTabId === tabId;
      const dbChanged = isActiveTab && ctx.database !== prev.database;
      set((s) => ({
        tabContexts: { ...s.tabContexts, [tabId]: tabCtx },
        ...(isActiveTab ? {
          role: ctx.role ?? "",
          warehouse: ctx.warehouse ?? "",
          database: ctx.database ?? "",
          schema: ctx.schema ?? "",
          ...(dbChanged ? { schemas: [], schemasForDatabase: "" } : {}),
        } : {}),
      }));
    } catch {
      // silently ignore — might not be connected yet
    } finally {
      set({ loadingContext: false });
    }
  },

  setActiveTab: (tabId: string) => {
    const ctx = get().tabContexts[tabId];
    if (ctx) {
      const dbChanged = ctx.database !== get().database;
      set({
        role: ctx.role,
        warehouse: ctx.warehouse,
        database: ctx.database,
        schema: ctx.schema,
        ...(dbChanged ? { schemas: [], schemasForDatabase: "" } : {}),
      });
    }
  },

  loadRoles: async () => {
    if (get().roles.length > 0) return;
    set({ loadingRoles: true });
    try {
      const roles = await ListAvailableRoles();
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

  loadDatabases: async () => {
    if (get().databases.length > 0) return;
    set({ loadingDatabases: true });
    try {
      const databases = await ListDatabases();
      set({ databases });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ loadingDatabases: false });
    }
  },

  loadSchemas: async () => {
    const { database, schemasForDatabase } = get();
    if (!database) return;
    if (schemasForDatabase === database && get().schemas.length > 0) return;
    set({ loadingSchemas: true });
    try {
      const schemas = await ListSchemas(database);
      set({ schemas, schemasForDatabase: database });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ loadingSchemas: false });
    }
  },

  switchRole: async (role) => {
    const tabId = useQueryStore.getState().activeTabId;
    set({ switchingRole: true, error: null });
    try {
      await UseRole(tabId, role);
      set((s) => ({
        role,
        tabContexts: { ...s.tabContexts, [tabId]: { ...(s.tabContexts[tabId] ?? { role: "", warehouse: "", database: "", schema: "" }), role } },
      }));
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingRole: false });
    }
  },

  switchWarehouse: async (warehouse) => {
    const tabId = useQueryStore.getState().activeTabId;
    set({ switchingWarehouse: true, error: null });
    try {
      await UseWarehouse(tabId, warehouse);
      set((s) => ({
        warehouse,
        tabContexts: { ...s.tabContexts, [tabId]: { ...(s.tabContexts[tabId] ?? { role: "", warehouse: "", database: "", schema: "" }), warehouse } },
      }));
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingWarehouse: false });
    }
  },

  switchDatabase: async (database) => {
    const tabId = useQueryStore.getState().activeTabId;
    set({ switchingDatabase: true, error: null });
    try {
      await UseDatabase(tabId, database);
      set((s) => ({
        database,
        schemas: [],
        schemasForDatabase: "",
        schema: "",
        tabContexts: { ...s.tabContexts, [tabId]: { ...(s.tabContexts[tabId] ?? { role: "", warehouse: "", database: "", schema: "" }), database, schema: "" } },
      }));
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingDatabase: false });
    }
  },

  switchSchema: async (schema) => {
    const tabId = useQueryStore.getState().activeTabId;
    set({ switchingSchema: true, error: null });
    try {
      await UseSchema(tabId, schema);
      set((s) => ({
        schema,
        tabContexts: { ...s.tabContexts, [tabId]: { ...(s.tabContexts[tabId] ?? { role: "", warehouse: "", database: "", schema: "" }), schema } },
      }));
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingSchema: false });
    }
  },

  clearError: () => set({ error: null }),
}));
