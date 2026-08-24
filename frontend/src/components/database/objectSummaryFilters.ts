// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import type { table } from "../../../wailsjs/go/models";

/**
 * Every TABLE_TYPE INFORMATION_SCHEMA.TABLES can return, plus TRANSIENT, which
 * the backend folds in from the separate IS_TRANSIENT flag.
 */
export const KIND_FILTERS = [
  { text: "Base Table", value: "BASE TABLE" },
  { text: "Transient", value: "TRANSIENT" },
  { text: "Temporary Table", value: "TEMPORARY TABLE" },
  { text: "External Table", value: "EXTERNAL TABLE" },
  { text: "Event Table", value: "EVENT TABLE" },
  { text: "View", value: "VIEW" },
  { text: "Materialized View", value: "MATERIALIZED VIEW" },
];

/** Tag colour per kind; anything unlisted falls back to blue. */
export const KIND_COLORS: Record<string, string> = {
  TRANSIENT: "orange",
  "TEMPORARY TABLE": "purple",
  "EXTERNAL TABLE": "cyan",
  "EVENT TABLE": "gold",
  VIEW: "green",
  "MATERIALIZED VIEW": "geekblue",
};

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

export function matchesRowFilter(value: string, rows: number) {
  return value === "empty" ? rows === 0 : rows > 0;
}
