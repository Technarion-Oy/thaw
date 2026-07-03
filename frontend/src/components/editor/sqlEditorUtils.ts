// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: SQL Editor & Diagnostics

import { GetTableForeignKeys } from "../../../wailsjs/go/app/App";

// ── UC helper ────────────────────────────────────────────────────────────────
export const UC = (s: string) => s.toUpperCase();

// ── identifierRangeAt ─────────────────────────────────────────────────────────
// Column span (1-based Monaco columns) of the dotted identifier at a 0-based char
// index, or null if that index isn't on an identifier. A double-quoted segment
// (which may contain spaces, e.g. "MY TABLE") counts as part of the identifier, so
// DB."MY SCHEMA".NAME spans as one unit. Used for the cmd/ctrl-hover link underline.
export function identifierRangeAt(line: string, idx0: number): { start: number; end: number } | null {
  // Mark every column that belongs to an identifier: bare chars, plus whole
  // double-quoted segments ("" is an escaped quote inside one). One pass, quote-aware.
  const isBare = (ch: string) => /[A-Za-z0-9_$.]/.test(ch);
  const inIdent = new Array<boolean>(line.length).fill(false);
  for (let i = 0; i < line.length;) {
    if (line[i] === '"') {
      const q = i++;
      while (i < line.length) {
        if (line[i] === '"') { if (line[i + 1] === '"') { i += 2; continue; } i++; break; }
        i++;
      }
      for (let k = q; k < i; k++) inIdent[k] = true;
    } else if (isBare(line[i])) { inIdent[i] = true; i++; }
    else i++;
  }
  let j = idx0;
  if (!(j >= 0 && j < line.length && inIdent[j])) j = idx0 - 1;   // allow cursor just past the last char
  if (!(j >= 0 && j < line.length && inIdent[j])) return null;
  let s = j, e = j;
  while (s > 0 && inIdent[s - 1]) s--;
  while (e + 1 < line.length && inIdent[e + 1]) e++;
  return { start: s + 1, end: e + 2 };   // Monaco endColumn is exclusive (points after last char)
}

// ── quoteIfNecessary ─────────────────────────────────────────────────────────
// Quotes a Snowflake identifier if it contains characters that require quoting
// or conflicts with a reserved keyword. Accepts the keyword set as a parameter.
export function quoteIfNecessary(name: string, keywords?: Set<string> | null): string {
  if (!name) return name;
  const needsQuoting = !/^[A-Z_][A-Z0-9_$]*$/.test(name) || (keywords?.has(name.toUpperCase()) ?? false);
  return needsQuoting ? `"${name.replace(/"/g, '""')}"` : name;
}

// ── FK cache for JOIN ON autocomplete ────────────────────────────────────────
export interface FKEntry {
  pkDatabase: string; pkSchema: string; pkTable: string; pkColumn: string;
  fkColumn: string;
  constraintName: string;
  keySequence: number;
}

const fkCache    = new Map<string, FKEntry[]>();
const fetchingFKs = new Set<string>();

// cacheGeneration is bumped whenever the metadata caches are invalidated (a query
// ran DDL, or the object store was refreshed). A fetch that started before the
// bump must NOT write its now-stale result into the cache, so every fetch helper
// captures the generation at start and discards its result if it changed.
let cacheGeneration = 0;
export function currentCacheGeneration(): number { return cacheGeneration; }
export function bumpCacheGeneration(): void { cacheGeneration++; }

export async function getFKs(db: string, schema: string, table: string): Promise<FKEntry[]> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  if (fkCache.has(key)) return fkCache.get(key)!;
  if (fetchingFKs.has(key)) return [];
  fetchingFKs.add(key);
  const gen = cacheGeneration;
  try {
    const fks = await GetTableForeignKeys(db, schema, table);
    const entries: FKEntry[] = (fks ?? []).map((fk: any) => ({
      pkDatabase:     fk.pkDatabase     ?? "",
      pkSchema:       fk.pkSchema       ?? "",
      pkTable:        fk.pkTable        ?? "",
      pkColumn:       fk.pkColumn       ?? "",
      fkColumn:       fk.fkColumn       ?? "",
      constraintName: fk.constraintName ?? "",
      keySequence:    fk.keySequence    ?? 0,
    }));
    if (gen === cacheGeneration) fkCache.set(key, entries);
    return entries;
  } catch {
    if (gen === cacheGeneration) fkCache.set(key, []);
    return [];
  } finally {
    fetchingFKs.delete(key);
  }
}

// Expose the cache for schema-level FK warm-up (used from SqlEditor.tsx).
export function setFKCache(key: string, entries: FKEntry[]): void {
  if (!fkCache.has(key)) fkCache.set(key, entries);
}

// Synchronous cache read for inline completions (no IPC call).
export function getFKsCached(db: string, schema: string, table: string): FKEntry[] {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  return fkCache.get(key) ?? [];
}

// Drop all cached foreign keys. Called when query execution or an object-store
// refresh may have changed the catalog, so JOIN-ON suggestions re-fetch.
export function clearFKCache(): void {
  fkCache.clear();
  fetchingFKs.clear();
}

// ── variableSuggestions factory ───────────────────────────────────────────────
export function buildVariableSuggestions(
  declaredVars: string[],
  needsColon: boolean,
  range: any,
  monaco: any,
): any[] {
  return declaredVars.map((v) => ({
    label:      needsColon ? ":" + v : v,
    kind:       monaco.languages.CompletionItemKind.Variable,
    insertText: needsColon ? ":" + v : v,
    filterText: needsColon ? ":" + v : v,
    sortText:   "01_" + v,
    detail:     "SCRIPT VARIABLE",
    range,
  }));
}
