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
} from "../../wailsjs/go/main/App";

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

  loadContext: () => Promise<void>;
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

  loadContext: async () => {
    set({ loadingContext: true });
    try {
      const ctx = await GetSessionContext();
      const prev = get();
      const dbChanged = ctx.database && ctx.database !== prev.database;
      set({
        role: ctx.role,
        warehouse: ctx.warehouse,
        database: ctx.database ?? "",
        schema: ctx.schema ?? "",
        ...(dbChanged ? { schemas: [], schemasForDatabase: "" } : {}),
      });
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

  switchDatabase: async (database) => {
    set({ switchingDatabase: true, error: null });
    try {
      await UseDatabase(database);
      set({ database, schemas: [], schemasForDatabase: "", schema: "" });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingDatabase: false });
    }
  },

  switchSchema: async (schema) => {
    set({ switchingSchema: true, error: null });
    try {
      await UseSchema(schema);
      set({ schema });
    } catch (e) {
      set({ error: String(e) });
    } finally {
      set({ switchingSchema: false });
    }
  },

  clearError: () => set({ error: null }),
}));
