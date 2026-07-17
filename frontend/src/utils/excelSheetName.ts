// SPDX-License-Identifier: GPL-3.0-or-later

// @thaw-domain: SQL Editor & Diagnostics

// Excel worksheet names are capped at 31 characters and may not contain any of
// the characters [ ] : * ? / \. They also may not start or end with an
// apostrophe ('), and "History" (any case) is reserved by Excel. Names must be
// non-empty and unique within a workbook (case-insensitively). deriveSheetName
// turns a SQL statement into a name that satisfies all of these rules,
// de-duplicating against `used` (which it also mutates so successive calls stay
// unique across the whole workbook).

const INVALID_CHARS = /[[\]:*?/\\]/g;
export const MAX_SHEET_NAME = 31;

// deriveSheetName builds a valid, unique worksheet name from a SQL statement.
// `index` is the 1-based resultset number used for the "Result N" fallback when
// the SQL is empty (or reduces to nothing after sanitising) or collides with a
// reserved name. `used` holds the lower-cased names already claimed; matching
// entries are suffixed with " (2)", " (3)", … while staying within the 31-char
// limit.
export function deriveSheetName(sql: string, index: number, used: Set<string>): string {
  const fallback = `Result ${index}`;
  // Truncate first, then strip apostrophes the cut may have exposed at either
  // end (a 31-char slice can land mid-string-literal). "History" is reserved.
  const cleaned = sql.replace(INVALID_CHARS, " ").replace(/\s+/g, " ").trim();
  let base = cleaned.slice(0, MAX_SHEET_NAME).replace(/^'+|'+$/g, "").trim();
  if (!base || base.toLowerCase() === "history") base = fallback;

  let name = base;
  let n = 2;
  while (used.has(name.toLowerCase())) {
    const suffix = ` (${n})`;
    name = `${base.slice(0, MAX_SHEET_NAME - suffix.length).trim()}${suffix}`;
    n++;
  }
  used.add(name.toLowerCase());
  return name;
}
