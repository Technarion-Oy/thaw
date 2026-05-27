// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

import { create } from "zustand";

export interface KnownObject {
  db: string;
  schema: string;
  name: string;
  kind: string;
}

interface ObjectState {
  databases: string[];
  schemas: { db: string; name: string }[];
  objects: KnownObject[];

  setDatabases: (dbs: string[]) => void;
  // Replaces all schemas for the given db (idempotent on re-load).
  addSchemas: (db: string, schemas: string[]) => void;
  // Replaces all objects for the given db.schema (idempotent on re-load).
  addObjects: (db: string, schema: string, objects: { name: string; kind: string }[]) => void;
  // Merges additional objects into the given db.schema (deduplicates by kind|name).
  mergeObjects: (db: string, schema: string, objects: { name: string; kind: string }[]) => void;
  // Removes everything under a database (used on sidebar refresh).
  clearDatabase: (db: string) => void;
  // Removes a database and all its schemas/objects (used after DROP DATABASE).
  removeDatabase: (db: string) => void;
  // Removes a schema and all its objects (used after DROP SCHEMA).
  removeSchema: (db: string, schema: string) => void;
}

export const useObjectStore = create<ObjectState>((set) => ({
  databases: [],
  schemas: [],
  objects: [],

  setDatabases: (dbs) =>
    set({ databases: dbs }),

  addSchemas: (db, schemaNames) =>
    set((s) => ({
      schemas: [
        ...s.schemas.filter((x) => x.db !== db),
        ...schemaNames.map((name) => ({ db, name })),
      ],
    })),

  addObjects: (db, schema, objs) =>
    set((s) => ({
      objects: [
        ...s.objects.filter((x) => !(x.db === db && x.schema === schema)),
        ...objs.map((o) => ({ db, schema, name: o.name, kind: o.kind })),
      ],
    })),

  mergeObjects: (db, schema, objs) =>
    set((s) => {
      const existing = new Set(
        s.objects
          .filter((x) => x.db === db && x.schema === schema)
          .map((x) => `${x.kind}|${x.name}`),
      );
      const newObjs = objs
        .filter((o) => !existing.has(`${o.kind}|${o.name}`))
        .map((o) => ({ db, schema, name: o.name, kind: o.kind }));
      return { objects: [...s.objects, ...newObjs] };
    }),

  clearDatabase: (db) =>
    set((s) => ({
      schemas: s.schemas.filter((x) => x.db !== db),
      objects: s.objects.filter((x) => x.db !== db),
    })),

  removeDatabase: (db) =>
    set((s) => ({
      databases: s.databases.filter((d) => d !== db),
      schemas: s.schemas.filter((x) => x.db !== db),
      objects: s.objects.filter((x) => x.db !== db),
    })),

  removeSchema: (db, schema) =>
    set((s) => ({
      schemas: s.schemas.filter((x) => !(x.db === db && x.name === schema)),
      objects: s.objects.filter((x) => !(x.db === db && x.schema === schema)),
    })),
}));
