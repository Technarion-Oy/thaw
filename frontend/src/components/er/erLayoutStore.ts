// Copyright (c) 2026 Technarion Oy. All rights reserved.
// @thaw-domain: ER Designer

const KEY_PREFIX = "thaw-er-layout:";
const DEBOUNCE_MS = 500;

type PositionMap = Record<string, { x: number; y: number }>;

const saveTimers = new Map<string, ReturnType<typeof setTimeout>>();
const pendingData = new Map<string, { key: string; positions: PositionMap }>();

function storageKey(database: string): string {
  return KEY_PREFIX + database.toUpperCase();
}

/** Position key for a table: "SCHEMA.TABLE" (uppercase, stable across sessions). */
export function positionKey(schema: string, table: string): string {
  return `${schema.toUpperCase()}.${table.trim().toUpperCase()}`;
}

function writePending(key: string): void {
  const pending = pendingData.get(key);
  if (!pending) return;
  pendingData.delete(key);
  try {
    localStorage.setItem(pending.key, JSON.stringify(pending.positions));
  } catch {
    // localStorage full or unavailable — silently ignore
  }
}

/** Save node positions to localStorage (debounced per database). */
export function saveERLayout(database: string, positions: PositionMap): void {
  const key = storageKey(database);
  const existing = saveTimers.get(key);
  if (existing) clearTimeout(existing);
  pendingData.set(key, { key, positions });
  saveTimers.set(
    key,
    setTimeout(() => {
      saveTimers.delete(key);
      writePending(key);
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

/** Flush any pending debounced save for a database (e.g. on component unmount). */
export function flushERLayout(database: string): void {
  const key = storageKey(database);
  const timer = saveTimers.get(key);
  if (!timer) return;
  clearTimeout(timer);
  saveTimers.delete(key);
  writePending(key);
}