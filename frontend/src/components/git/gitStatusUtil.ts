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
// @thaw-domain: Git Integration

// Shared helpers for rendering git status — the VS Code-style sigil colors and
// the Snowflake object-type label derived from the export path. Used by both the
// Changes view (GitOperationsDialog) and the FileBrowser tree so coloring stays
// consistent across surfaces.

/** Map a single-letter git status to its theme token color (matches VS Code's
 *  gitDecoration mapping so muscle memory transfers). */
export function sigilColor(status: string): string {
  switch (status) {
    case "A": return "var(--success)";   // added / staged-new — full green
    // Untracked is also "new" (green family) but NOT yet staged, so it's a muted
    // green — distinguishable at a glance from a staged-new file in the tree.
    case "U": return "color-mix(in srgb, var(--success) 60%, var(--text-faint))";
    case "M": return "var(--warning)";   // modified
    case "D": return "var(--danger)";    // deleted
    case "R":
    case "C": return "var(--accent)";    // renamed / copied
    default:  return "var(--text-faint)";
  }
}

/** Snowflake object-type directory names (from the default export path template
 *  `{database}/{schema}/{object_type}/{object_name}.sql`) mapped to a singular
 *  label and its --icon-* color token. */
const OBJECT_DIRS: Record<string, { label: string; color: string }> = {
  tables:       { label: "table",       color: "var(--icon-table)" },
  views:        { label: "view",        color: "var(--icon-view)" },
  functions:    { label: "function",    color: "var(--icon-function)" },
  procedures:   { label: "procedure",   color: "var(--icon-procedure)" },
  sequences:    { label: "sequence",    color: "var(--icon-sequence)" },
  stages:       { label: "stage",       color: "var(--icon-stage)" },
  streams:      { label: "stream",      color: "var(--icon-stream)" },
  tasks:        { label: "task",        color: "var(--icon-task)" },
  file_formats: { label: "file format", color: "var(--icon-fileformat)" },
  schemas:      { label: "schema",      color: "var(--icon-schema)" },
};

/** Derive the Snowflake object type from a path by scanning its segments for a
 *  recognized object-type directory. Returns null when none is found (e.g. a
 *  non-DDL file or a custom export layout). */
export function objectTypeFromPath(path: string): { label: string; color: string } | null {
  const segs = path.replace(/\\/g, "/").split("/");
  for (const s of segs) {
    const m = OBJECT_DIRS[s.toLowerCase()];
    if (m) return m;
  }
  return null;
}

/** From the uncapped `changedPaths` map, derive the two path-sets the discard
 *  prompts need: `newFilesRel` (no committed version → discard deletes, not
 *  reverts) and `partiallyStagedRel` (both staged & unstaged → discard also drops
 *  the staged part). Both flags are computed authoritatively on the backend; this
 *  is the single place the frontend reads them, shared by the Changes view and the
 *  FileBrowser tree. Keys are repo-relative forward-slash paths (as the backend
 *  emits them). */
export function deriveNewAndPartial(
  changedPaths: Record<string, { isNew: boolean; partiallyStaged: boolean }> | undefined | null,
): { newFilesRel: Set<string>; partiallyStagedRel: Set<string> } {
  const newFilesRel = new Set<string>();
  const partiallyStagedRel = new Set<string>();
  for (const [p, cf] of Object.entries(changedPaths ?? {})) {
    if (cf.isNew) newFilesRel.add(p);
    if (cf.partiallyStaged) partiallyStagedRel.add(p);
  }
  return { newFilesRel, partiallyStagedRel };
}

/** Split a path into its directory prefix (with trailing slash) and filename.
 *  Trailing separators are stripped first so "a/b/" yields name "b" (matches
 *  FileBrowser's pathBase). */
export function splitPath(path: string): { dir: string; name: string } {
  const norm = path.replace(/\\/g, "/").replace(/\/+$/, "");
  const i = norm.lastIndexOf("/");
  return i >= 0 ? { dir: norm.slice(0, i + 1), name: norm.slice(i + 1) } : { dir: "", name: norm };
}
