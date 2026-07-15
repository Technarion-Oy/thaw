// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: SQL Editor & Diagnostics

import { GetTableForeignKeys } from "../../../wailsjs/go/app/App";

// ── UC helper ────────────────────────────────────────────────────────────────
export const UC = (s: string) => s.toUpperCase();

// ── normId ────────────────────────────────────────────────────────────────────
// Mirror of the backend `normID` (internal/sqleditor/sqleditor.go): normalise a
// raw identifier the way ResolveTableRefs already stored its refs, so a captured
// qualifier can be matched with an exact `===`. A quoted "..." keeps its
// (case-sensitive) inner text with doubled "" unescaped; a bare identifier is
// upper-cased (Snowflake folds unquoted names). Keeps `"Foo"` and `"foo"`
// distinct — a lower-cased compare would silently expand the wrong table.
export const normId = (s: string): string =>
  !s ? s : s.startsWith('"') ? s.slice(1, -1).replace(/""/g, '"') : s.toUpperCase();

// ── colCacheKey ───────────────────────────────────────────────────────────────
// Case-insensitive, NUL-delimited key for a fully-qualified table, shared by
// every per-table cache (column list, column types, wildcard expansion) so the
// format lives in exactly one place.
export const colCacheKey = (db: string, schema: string, table: string) =>
  `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;

// ── byteColToUtf16Col ──────────────────────────────────────────────────────────
// Backend diagnostics validators emit 1-based UTF-8 *byte* columns (sqltok.Token.Col),
// but Monaco columns are 1-based UTF-16 code units. Any non-ASCII char earlier on a
// line shifts every later marker (emoji shift by 2) and corrupts the "Qualify as …"
// quick fix. Convert byte → UTF-16 against the model's own line text. See issue #702.
// A byte column past the line length clamps to end+1, matching Monaco's space.
export function byteColToUtf16Col(lineText: string, byteCol: number): number {
  if (byteCol <= 1) return 1;
  const targetBytes = byteCol - 1;
  let bytes = 0;
  let utf16 = 0;
  for (const ch of lineText) {
    if (bytes >= targetBytes) break;
    const cp = ch.codePointAt(0)!;
    bytes += cp < 0x80 ? 1 : cp < 0x800 ? 2 : cp < 0x10000 ? 3 : 4;
    utf16 += ch.length; // 1 for BMP, 2 for astral (surrogate pair)
  }
  return utf16 + 1;
}

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
      let closed = false;
      while (i < line.length) {
        if (line[i] === '"') { if (line[i + 1] === '"') { i += 2; continue; } i++; closed = true; break; }
        i++;
      }
      // Only a *closed* quote is a real identifier segment. An unterminated quote
      // (mid-typing, e.g. DB.SCHEMA."MY) must not swallow the rest of the line.
      if (closed) for (let k = q; k < i; k++) inIdent[k] = true;
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

// ── starMenuEligible ──────────────────────────────────────────────────────────
// Display gate for the editor's "Expand *" context-menu item: true when the
// cursor sits on a literal `*` that is NOT part of an object name. A `*` in an
// object name is always inside a quoted identifier ("Testin*table") — reuse
// identifierRangeAt (the DDL-hover span logic) instead of a bespoke parser to
// tell them apart: it returns a range that *contains* the star only in that
// case. A genuine `alias.*` star falls just past the `alias.` range (identifier
// chars never include `*`), and a bare `*` gets no range at all — both eligible.
// A `*` inside a single-quoted string literal ('x*y') is likewise not a wildcard
// (identifierRangeAt only tracks double-quoted identifiers, so that's checked
// separately). Cheap + synchronous so it can fill the Monaco context key before
// the menu renders; the command still re-checks the token authoritatively.
export function starMenuEligible(line: string, column: number): boolean {
  let idx = -1;
  if (line[column - 1] === "*") idx = column - 1;
  else if (line[column - 2] === "*") idx = column - 2; // cursor on the star's right edge
  if (idx < 0) return false;
  // Inside a double-quoted identifier (object name)?
  const r = identifierRangeAt(line, idx);
  const col = idx + 1; // 1-based Monaco column of the star
  if (r !== null && col >= r.start && col < r.end) return false;
  // Inside a single-quoted string literal? Count unescaped `'` before the star
  // ('' is an escaped quote); an odd count means the star is inside a string. Skip
  // over double-quoted identifiers so an apostrophe in a column name (e.g. "it's")
  // doesn't flip the parity.
  let quotes = 0;
  let inDouble = false;
  for (let i = 0; i < idx; i++) {
    const ch = line[i];
    if (inDouble) {
      if (ch === '"') { if (line[i + 1] === '"') { i++; continue; } inDouble = false; }
      continue;
    }
    if (ch === '"') { inDouble = true; continue; }
    if (ch === "'") { if (line[i + 1] === "'") { i++; continue; } quotes++; }
  }
  return quotes % 2 === 0;
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
