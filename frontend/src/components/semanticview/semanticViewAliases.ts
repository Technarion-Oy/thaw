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
 * A same-length list means the rows were edited in place, so the aliases are
 * compared position-wise and a changed one is a rename. A different length
 * means a row was added or deleted, where position-wise comparison would be
 * meaningless — there, any alias that is simply gone counts as removed.
 * Blank aliases (a row whose table isn't picked yet) are never references.
 */
export function diffAliases(prev: string[], next: string[]): AliasDiff {
  const renames: Record<string, string> = {};
  const removed: string[] = [];

  if (prev.length === next.length) {
    prev.forEach((old, i) => {
      if (old && next[i] && old !== next[i]) renames[old] = next[i];
      else if (old && !next[i]) removed.push(old);
    });
  } else {
    for (const old of prev) {
      if (old && !next.includes(old)) removed.push(old);
    }
  }
  return { renames, removed };
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
 */
export function remapQualified(ref: string, diff: AliasDiff): string {
  const dot = ref.indexOf(".");
  if (dot < 0) return remapAlias(ref, diff);
  const alias = remapAlias(ref.slice(0, dot), diff);
  return alias ? alias + ref.slice(dot) : "";
}
