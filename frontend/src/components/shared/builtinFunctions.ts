// SPDX-License-Identifier: GPL-3.0-or-later

// Curated Snowflake built-in functions that are valid inside a column DEFAULT
// clause at table-create time. This is a hand-picked subset of the full
// ~321-entry catalog in internal/fnmeta: only context/session and date-time
// functions that Snowflake accepts as a CREATE TABLE column DEFAULT are listed
// here (arbitrary scalar functions that need arguments or reference other
// columns are rejected, so they are deliberately excluded). Surfaced by the
// DefaultFunctionPicker — the "insert as DEFAULT" pickers in Create Table and
// the ER Designer, and the per-field value picker in the Insert Row form.

// Display order for the grouped picker. This tuple is the single source of
// truth: BuiltinFnCategory is derived from it, so adding/renaming a category
// here is enough — a `category` value that isn't in this list won't type-check.
export const DEFAULT_FUNCTION_CATEGORIES = [
  "Date & Time",
  "Session & Context",
  "Identifiers & Misc",
] as const;

export type BuiltinFnCategory = (typeof DEFAULT_FUNCTION_CATEGORIES)[number];

export interface BuiltinFn {
  name: string;
  sql: string;
  desc: string;
  category: BuiltinFnCategory;
}

export const DEFAULT_FUNCTIONS: BuiltinFn[] = [
  // ── Date & Time ────────────────────────────────────────────────────────
  { name: "CURRENT_TIMESTAMP", sql: "CURRENT_TIMESTAMP()", desc: "Current date & time (TIMESTAMP_LTZ)", category: "Date & Time" },
  { name: "CURRENT_DATE", sql: "CURRENT_DATE()", desc: "Current date", category: "Date & Time" },
  { name: "CURRENT_TIME", sql: "CURRENT_TIME()", desc: "Current time", category: "Date & Time" },
  { name: "LOCALTIME", sql: "LOCALTIME()", desc: "Current local time", category: "Date & Time" },
  { name: "LOCALTIMESTAMP", sql: "LOCALTIMESTAMP()", desc: "Current local timestamp", category: "Date & Time" },
  { name: "SYSDATE", sql: "SYSDATE()", desc: "Current timestamp in UTC", category: "Date & Time" },
  { name: "GETDATE", sql: "GETDATE()", desc: "Current date & time (alias of CURRENT_TIMESTAMP)", category: "Date & Time" },
  { name: "UNIX_TIMESTAMP", sql: "DATE_PART(EPOCH_SECOND, CURRENT_TIMESTAMP())", desc: "Current Unix time (seconds since epoch)", category: "Date & Time" },

  // ── Session & Context ──────────────────────────────────────────────────
  { name: "CURRENT_USER", sql: "CURRENT_USER()", desc: "Name of the current user", category: "Session & Context" },
  { name: "CURRENT_ROLE", sql: "CURRENT_ROLE()", desc: "Name of the current role", category: "Session & Context" },
  { name: "CURRENT_AVAILABLE_ROLES", sql: "CURRENT_AVAILABLE_ROLES()", desc: "Roles available to the current user (JSON array)", category: "Session & Context" },
  { name: "CURRENT_DATABASE", sql: "CURRENT_DATABASE()", desc: "Name of the current database", category: "Session & Context" },
  { name: "CURRENT_SCHEMA", sql: "CURRENT_SCHEMA()", desc: "Name of the current schema", category: "Session & Context" },
  { name: "CURRENT_WAREHOUSE", sql: "CURRENT_WAREHOUSE()", desc: "Name of the current warehouse", category: "Session & Context" },
  { name: "CURRENT_ACCOUNT", sql: "CURRENT_ACCOUNT()", desc: "Current account locator", category: "Session & Context" },
  { name: "CURRENT_REGION", sql: "CURRENT_REGION()", desc: "Region of the current account", category: "Session & Context" },
  { name: "CURRENT_SESSION", sql: "CURRENT_SESSION()", desc: "ID of the current session", category: "Session & Context" },
  { name: "CURRENT_VERSION", sql: "CURRENT_VERSION()", desc: "Snowflake version of the current system", category: "Session & Context" },
  { name: "CURRENT_CLIENT", sql: "CURRENT_CLIENT()", desc: "Version of the client driver", category: "Session & Context" },

  // ── Identifiers & Misc ─────────────────────────────────────────────────
  { name: "UUID_STRING", sql: "UUID_STRING()", desc: "Random UUID v4 string", category: "Identifiers & Misc" },
  { name: "RANDOM", sql: "RANDOM()", desc: "Pseudo-random 64-bit signed integer", category: "Identifiers & Misc" },
];

export interface BuiltinFnGroup {
  category: BuiltinFnCategory;
  fns: BuiltinFn[];
}

/**
 * Filters DEFAULT_FUNCTIONS by a free-text query (matched case-insensitively
 * against name, SQL and description) and returns them grouped by category in
 * DEFAULT_FUNCTION_CATEGORIES order. Empty groups are omitted. A blank query
 * returns every function. Pure — shared by the picker UI and unit tests.
 */
export function filterDefaultFunctions(query: string): BuiltinFnGroup[] {
  const q = query.trim().toLowerCase();
  const match = (f: BuiltinFn) =>
    q === "" ||
    f.name.toLowerCase().includes(q) ||
    f.sql.toLowerCase().includes(q) ||
    f.desc.toLowerCase().includes(q);
  return DEFAULT_FUNCTION_CATEGORIES.map((category) => ({
    category,
    fns: DEFAULT_FUNCTIONS.filter((f) => f.category === category && match(f)),
  })).filter((g) => g.fns.length > 0);
}
