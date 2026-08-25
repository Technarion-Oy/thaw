// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import type { table } from "../../../wailsjs/go/models";
import type { FilterValue, SortOrder } from "antd/es/table/interface";

/**
 * Every TABLE_TYPE INFORMATION_SCHEMA.TABLES can return, plus the four kinds the
 * backend folds in from the separate IS_TRANSIENT / IS_DYNAMIC / IS_ICEBERG /
 * IS_HYBRID flags (all of which report TABLE_TYPE = BASE TABLE). The set is fixed by
 * Snowflake's documented TABLE_TYPE domain, not by our object-kind registry, and
 * the raw value doubles as the option label so it always reads exactly like the
 * Type tag in the same row — there is no second label to drift.
 */
export const KIND_FILTERS = [
  "BASE TABLE",
  "TRANSIENT",
  "TEMPORARY TABLE",
  "DYNAMIC TABLE",
  "ICEBERG TABLE",
  "HYBRID TABLE",
  "EXTERNAL TABLE",
  "EVENT TABLE",
  "VIEW",
  "MATERIALIZED VIEW",
].map((k) => ({ text: k, value: k }));

/**
 * The canonical object-kind name (`frontend/src/generated/objectKinds.ts`, from
 * `internal/objectkind`) for a TABLE_TYPE, so the Type tag can take its colour
 * from the sidebar's `KIND_VAR` palette instead of a second hand-rolled map.
 * Only the three plain table variants have no registry entry of their own — the
 * registry models them all as TABLE — so they share the table colour and are told
 * apart by the tag text; every other kind maps to itself.
 */
export function registryKind(tableType: string) {
  switch (tableType) {
    case "BASE TABLE":
    case "TRANSIENT":
    case "TEMPORARY TABLE":
      return "TABLE";
    default:
      return tableType;
  }
}

export const ROW_FILTERS = [
  { text: "Empty", value: "empty" },
  { text: "Non-empty", value: "nonempty" },
];

/** Distinct schemas present in the loaded rows, sorted, as antd filter options. */
export function schemaFilters(data: table.TableSummary[]) {
  return [...new Set(data.map((t) => t.schema))]
    .sort((a, b) => a.localeCompare(b))
    .map((s) => ({ text: s, value: s }));
}

/**
 * `rows` is undefined when Snowflake reports no row count — for a view, and for a
 * table whose statistics have not been recomputed yet (freshly created, just
 * truncated). That is neither empty nor non-empty, so such a row matches neither
 * option rather than being counted as empty.
 */
export function matchesRowFilter(value: string, rows?: number) {
  if (rows === undefined) return false;
  return value === "empty" ? rows === 0 : rows > 0;
}

/**
 * Sorts the Rows / Size columns, keeping rows with no count (undefined) last in
 * both directions: antd negates the comparator for a descending sort, so the
 * unknown-vs-known verdict is pre-negated to survive it.
 */
export function compareCounts(a: number | undefined, b: number | undefined, order?: SortOrder) {
  if (a === undefined || b === undefined) {
    const last = (a === undefined ? 1 : 0) - (b === undefined ? 1 : 0);
    return order === "descend" ? -last : last;
  }
  return a - b;
}

/** Selected filter values per column key, exactly as antd's Table `onChange` reports them. */
export type SummaryFilters = Record<string, FilterValue | null>;

/**
 * Applies the column filters ourselves rather than via antd's `onFilter`, so the
 * rendered rows and the "Found N" caption always come from the same array — an
 * antd-internal filter selection survives a `dataSource` swap on Reload, which
 * a count captured in `onChange` would not (issue #908 review).
 *
 * Values OR within a column, columns AND with each other.
 */
export function applyFilters(data: table.TableSummary[], filters: SummaryFilters) {
  const matches = (key: string, predicate: (value: string) => boolean) => {
    const selected = filters[key] ?? [];
    return selected.length === 0 || selected.some((v) => predicate(String(v)));
  };
  return data.filter(
    (t) =>
      matches("name", (v) => v === t.schema) &&
      matches("kind", (v) => v === t.kind) &&
      matches("rows", (v) => matchesRowFilter(v, t.rows)),
  );
}
