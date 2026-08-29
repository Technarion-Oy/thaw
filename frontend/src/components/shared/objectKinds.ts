// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration

/**
 * Object kinds that behave like a table — anything a query, a monitor, or a
 * semantic view's logical `TABLES` entry can be pointed at. Shared so the
 * "table-like" pickers can't drift apart when Snowflake adds a kind.
 *
 * The names are the canonical KIND strings from the Go registry
 * (`internal/objectkind/kinds.go`), as they arrive on `SnowflakeObject.kind`.
 */
export const TABLE_LIKE_KINDS: readonly string[] = [
  "TABLE", "VIEW", "MATERIALIZED VIEW", "DYNAMIC TABLE",
  "EXTERNAL TABLE", "ICEBERG TABLE", "HYBRID TABLE", "EVENT TABLE",
];

/** Set form, for `.has()` filtering. */
export const TABLE_LIKE_KIND_SET: ReadonlySet<string> = new Set(TABLE_LIKE_KINDS);
