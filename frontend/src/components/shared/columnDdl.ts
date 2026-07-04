// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

/**
 * Build the ordered inline column modifiers for a `CREATE TABLE` / `ADD COLUMN`
 * column definition. Snowflake's column grammar requires `DEFAULT` before
 * `NOT NULL`, which precedes the inline `PRIMARY KEY` / `UNIQUE` constraints —
 * centralized here so the ordering rule lives in one place (it regressed twice
 * when each caller re-implemented it). Returns a string beginning with a space
 * when non-empty, e.g. `" DEFAULT CURRENT_TIMESTAMP() NOT NULL"`.
 */
export function columnConstraints(opts: {
  defaultValue?: string;
  notNull?: boolean;
  primaryKey?: boolean;
  unique?: boolean;
}): string {
  let s = "";
  const dv = opts.defaultValue?.trim();
  if (dv) s += ` DEFAULT ${dv}`;
  if (opts.notNull) s += " NOT NULL";
  if (opts.primaryKey) s += " PRIMARY KEY";
  if (opts.unique && !opts.primaryKey) s += " UNIQUE";
  return s;
}
