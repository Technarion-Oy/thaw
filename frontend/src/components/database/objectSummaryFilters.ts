// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: Object Browser & Administration

import type { table } from "../../../wailsjs/go/models";

/** Table types INFORMATION_SCHEMA.TABLES can return for the summary report. */
export const KIND_FILTERS = [
  { text: "Base Table", value: "BASE TABLE" },
  { text: "Transient", value: "TRANSIENT" },
  { text: "Temporary", value: "TEMPORARY" },
];

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
