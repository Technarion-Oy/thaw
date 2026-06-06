// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

const KEY_PREFIX = "thaw-er-layout:";
const DEBOUNCE_MS = 500;

type PositionMap = Record<string, { x: number; y: number }>;

const saveTimers = new Map<string, ReturnType<typeof setTimeout>>();

function storageKey(database: string): string {
  return KEY_PREFIX + database.toUpperCase();
}

/** Position key for a table: "SCHEMA.TABLE" (uppercase, stable across sessions). */
export function positionKey(schema: string, table: string): string {
  return `${schema.toUpperCase()}.${table.trim().toUpperCase()}`;
}

/** Save node positions to localStorage (debounced per database). */
export function saveERLayout(database: string, positions: PositionMap): void {
  const key = storageKey(database);
  const existing = saveTimers.get(key);
  if (existing) clearTimeout(existing);
  saveTimers.set(
    key,
    setTimeout(() => {
      saveTimers.delete(key);
      try {
        localStorage.setItem(key, JSON.stringify(positions));
      } catch {
        // localStorage full or unavailable — silently ignore
      }
    }, DEBOUNCE_MS),
  );
}

/** Load saved positions, or null if none exist. */
export function loadERLayout(database: string): PositionMap | null {
  try {
    const raw = localStorage.getItem(storageKey(database));
    if (!raw) return null;
    return JSON.parse(raw) as PositionMap;
  } catch {
    return null;
  }
}

/** Remove saved layout for a database. */
export function clearERLayout(database: string): void {
  try {
    localStorage.removeItem(storageKey(database));
  } catch {
    // ignore
  }
}
