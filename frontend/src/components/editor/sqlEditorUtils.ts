// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: SQL Editor & Diagnostics

import { GetTableForeignKeys } from "../../../wailsjs/go/main/App";

// ── UC helper ────────────────────────────────────────────────────────────────
export const UC = (s: string) => s.toUpperCase();

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

export async function getFKs(db: string, schema: string, table: string): Promise<FKEntry[]> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  if (fkCache.has(key)) return fkCache.get(key)!;
  if (fetchingFKs.has(key)) return [];
  fetchingFKs.add(key);
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
    fkCache.set(key, entries);
    return entries;
  } catch {
    fkCache.set(key, []);
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
