// SPDX-License-Identifier: GPL-3.0-or-later

// TanStack Table v9 feature sets. v9 registers only the features a table
// lists (tree-shakeable), so each grid types its columns/rows against the
// set it actually uses.

import {
  tableFeatures,
  rowSortingFeature,
  columnFilteringFeature,
  columnPinningFeature,
  columnSizingFeature,
  columnResizingFeature,
  columnOrderingFeature,
  createSortedRowModel,
  createFilteredRowModel,
  sortFns,
} from "@tanstack/react-table";

/** Core + sorting + column resizing — the simple virtualised grids. */
export const sortableGridFeatures = tableFeatures({
  rowSortingFeature,
  columnSizingFeature,
  columnResizingFeature,
  sortedRowModel: createSortedRowModel(),
  sortFns,
});
export type SortableGridFeatures = typeof sortableGridFeatures;

/** The result grid: adds per-column filtering, pinning and reordering. */
export const resultGridFeatures = tableFeatures({
  rowSortingFeature,
  columnFilteringFeature,
  columnPinningFeature,
  columnSizingFeature,
  columnResizingFeature,
  columnOrderingFeature,
  sortedRowModel: createSortedRowModel(),
  filteredRowModel: createFilteredRowModel(),
  sortFns,
});
export type ResultGridFeatures = typeof resultGridFeatures;
