// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

/**
 * Keeps the semantic-view form's alias references honest.
 *
 * `RELATIONSHIPS`, `FACTS`, `DIMENSIONS` and `METRICS` refer to logical tables
 * by alias — a plain string copied out of the `TABLES` list, not a live
 * reference. Renaming or deleting a table row would otherwise leave those rows
 * pointing at an alias that no longer exists in `TABLES ( … )`, which neither
 * the live preview nor the builder can catch: the SQL is well-formed and only
 * fails when Snowflake runs it.
 *
 * So every edit to the table list is diffed here, and the dependent rows are
 * rewritten: a renamed alias is followed, a removed one cleared (an expression
 * left without an alias is then dropped by the builder, which requires one).
 */

/** A before/after change to the set of table aliases. */
export interface AliasDiff {
  /** Old alias → new alias, for rows that were renamed in place. */
  renames: Record<string, string>;
  /** Aliases that no longer exist. */
  removed: string[];
}

/** True when the diff actually changes something (skip the rewrite otherwise). */
export function hasAliasChange(diff: AliasDiff): boolean {
  return Object.keys(diff.renames).length > 0 || diff.removed.length > 0;
}

/**
 * Diffs the alias list before and after an edit to the `TABLES` rows.
 *
 * Only an alias that has *disappeared* is interesting: one still present
 * somewhere in the new list keeps resolving, whichever row now carries it. A
 * same-length list means the rows were edited in place, so the row at the same
 * position holds the replacement — that is a rename; any other length means a
 * row was added or deleted, where position-wise comparison is meaningless, so
 * the alias counts as removed.
 *
 * Because a reference is a bare alias string, an alias used by two rows at once
 * cannot be attributed to either — remapping it would silently repoint rows
 * belonging to the row the user did *not* touch. Duplicated aliases are
 * therefore left alone; the stale value stays visible in its dropdown for the
 * user to correct. Blank aliases (a row whose table isn't picked yet) are never
 * references.
 */
export function diffAliases(prev: string[], next: string[]): AliasDiff {
  const renames: Record<string, string> = {};
  const removed: string[] = [];
  const sameLength = prev.length === next.length;

  prev.forEach((old, i) => {
    if (!old) return;
    if (next.includes(old)) return;
    if (prev.filter((a) => a === old).length > 1) return;
    const replacement = sameLength ? next[i] : "";
    if (replacement) renames[old] = replacement;
    else removed.push(old);
  });
  return { renames, removed };
}

/** A table row reduced to what reference-tracking needs: its alias and its
 * physical table. */
export interface TableIdentity {
  alias: string;
  /** Something that identifies the physical table — a qualified name. */
  table: string;
}

/**
 * The aliases whose row now points at a *different* physical table, named by
 * the alias they carry **after** the edit (references are remapped to the new
 * alias first, so that is what a caller matches against).
 *
 * Deliberately independent of the alias: picking a new table for a row whose
 * alias was auto-seeded from the old table name rewrites both in one edit, so
 * requiring the alias to be unchanged would miss the ordinary case. A swap is
 * not a rename — columns picked against the old table don't carry over, they
 * have to be cleared.
 *
 * Only a same-length edit aligns row-for-row; an add or delete shifts the
 * indices, so nothing can be compared and nothing is reported.
 */
export function swappedAliases(prev: TableIdentity[], next: TableIdentity[]): string[] {
  if (prev.length !== next.length) return [];
  return next
    .filter((r, i) => prev[i].table !== r.table)
    .map((r) => r.alias)
    .filter(Boolean);
}

/** Maps one alias reference through a diff — "" when the alias is gone. */
export function remapAlias(alias: string, diff: AliasDiff): string {
  if (!alias) return "";
  if (diff.renames[alias] !== undefined) return diff.renames[alias];
  return diff.removed.includes(alias) ? "" : alias;
}

/**
 * Maps a dotted `alias.name` reference (a metric's NON ADDITIVE BY dimension)
 * through a diff, returning "" when the alias is gone. A reference with no dot
 * is treated as a bare alias.
 *
 * Splits on the *last* dot: the alias half is free-text with no restriction
 * against containing one (e.g. "ord.v2"), while the name half is a single
 * field — the same reasoning as `qualifiedIdent` in
 * `internal/semanticview/sql.go`, which renders this same reference and must
 * split it the same way to stay consistent with what gets remapped here.
 */
export function remapQualified(ref: string, diff: AliasDiff): string {
  const dot = ref.lastIndexOf(".");
  if (dot < 0) return remapAlias(ref, diff);
  const alias = remapAlias(ref.slice(0, dot), diff);
  return alias ? alias + ref.slice(dot) : "";
}
