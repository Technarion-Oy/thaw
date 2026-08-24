// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import type { table } from "../../../wailsjs/go/models";

/**
 * Every TABLE_TYPE INFORMATION_SCHEMA.TABLES can return, plus TRANSIENT, which
 * the backend folds in from the separate IS_TRANSIENT flag. The set is fixed by
 * Snowflake's documented TABLE_TYPE domain, not by our object-kind registry, and
 * the raw value doubles as the option label so it always reads exactly like the
 * Type tag in the same row — there is no second label to drift.
 */
export const KIND_FILTERS = [
  "BASE TABLE",
  "TRANSIENT",
  "TEMPORARY TABLE",
  "EXTERNAL TABLE",
  "EVENT TABLE",
  "VIEW",
  "MATERIALIZED VIEW",
].map((k) => ({ text: k, value: k }));

/**
 * The canonical object-kind name (`frontend/src/generated/objectKinds.ts`, from
 * `internal/objectkind`) for a TABLE_TYPE, so the Type tag can take its colour
 * from the sidebar's `KIND_VAR` palette instead of a second hand-rolled map.
 * The three table variants have no registry entry of their own — the registry
 * models them all as TABLE — so they share the table colour and are told apart
 * by the tag text.
 */
export function registryKind(tableType: string) {
  switch (tableType) {
    case "VIEW":
    case "MATERIALIZED VIEW":
    case "EXTERNAL TABLE":
    case "EVENT TABLE":
      return tableType;
    default:
      return "TABLE";
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
 * `rows` is `table.UnknownCount` (-1) when Snowflake reports no row count at all
 * — it does not for views — which is neither empty nor non-empty, so such a row
 * matches neither option rather than being counted as empty.
 */
export function matchesRowFilter(value: string, rows: number) {
  if (rows < 0) return false;
  return value === "empty" ? rows === 0 : rows > 0;
}

/** Selected filter values per column key, as antd's Table `onChange` reports them. */
export type SummaryFilters = Record<string, readonly (string | number | bigint | boolean)[] | null | undefined>;

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
