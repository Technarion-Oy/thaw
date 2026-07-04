// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

// Curated Snowflake built-in functions that are valid inside a column DEFAULT
// clause at table-create time. Small subset on purpose — the full ~321-entry
// catalog lives in internal/fnmeta; this is the short-list surfaced by the
// "insert as DEFAULT" pickers in Create Table and the ER Designer.
export interface BuiltinFn {
  name: string;
  sql: string;
  desc: string;
}

export const DEFAULT_FUNCTIONS: BuiltinFn[] = [
  { name: "CURRENT_TIMESTAMP", sql: "CURRENT_TIMESTAMP()", desc: "Current date & time (TIMESTAMP_LTZ)" },
  { name: "CURRENT_DATE", sql: "CURRENT_DATE()", desc: "Current date" },
  { name: "CURRENT_TIME", sql: "CURRENT_TIME()", desc: "Current time" },
  { name: "SYSDATE", sql: "SYSDATE()", desc: "Current timestamp in UTC" },
  { name: "LOCALTIMESTAMP", sql: "LOCALTIMESTAMP()", desc: "Current local timestamp" },
  { name: "CURRENT_USER", sql: "CURRENT_USER()", desc: "Name of the current user" },
  { name: "CURRENT_ROLE", sql: "CURRENT_ROLE()", desc: "Name of the current role" },
  { name: "CURRENT_DATABASE", sql: "CURRENT_DATABASE()", desc: "Name of the current database" },
  { name: "CURRENT_SCHEMA", sql: "CURRENT_SCHEMA()", desc: "Name of the current schema" },
  { name: "CURRENT_WAREHOUSE", sql: "CURRENT_WAREHOUSE()", desc: "Name of the current warehouse" },
  { name: "UUID_STRING", sql: "UUID_STRING()", desc: "Random UUID v4 string" },
];
