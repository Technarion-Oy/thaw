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
  // Removes everything under a database (used on sidebar refresh).
  clearDatabase: (db: string) => void;
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

  clearDatabase: (db) =>
    set((s) => ({
      schemas: s.schemas.filter((x) => x.db !== db),
      objects: s.objects.filter((x) => x.db !== db),
    })),
}));
