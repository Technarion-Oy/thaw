// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

// Pure data preparation for QuickChartModal: column type detection and row
// reduction. React/Recharts-free so it can be unit-tested.

import type { SelectionRange } from "../../store/gridStore";
import { visualToOriginalIndex } from "./columnOrderUtils";

/** Most rows that ever reach Recharts. The plot area is a few hundred pixels
 *  wide, so more points than this only land on the same pixels. */
export const MAX_CHART_POINTS = 2000;

export interface QuickChartData {
  data: Record<string, unknown>[];
  xKey: string;
  valueKeys: string[];
  /** Rows in the selection; greater than `data.length` when it was sampled. */
  totalRows: number;
}

// Detect if a column is primarily numeric by sampling values.
function isNumericColumn(values: unknown[]): boolean {
  let numericCount = 0;
  let total = 0;
  for (const v of values) {
    if (v === null || v === undefined) continue;
    total++;
    if (!isNaN(Number(v)) && v !== "" && v !== true && v !== false) numericCount++;
  }
  return total > 0 && numericCount / total > 0.7;
}

/**
 * Row positions to chart: every row when the range fits in `maxPoints`,
 * otherwise `maxPoints` evenly spaced positions including the first and last.
 */
export function sampleRowIndices(minRow: number, maxRow: number, maxPoints: number): number[] {
  const total = maxRow - minRow + 1;
  const indices: number[] = [];
  if (total <= maxPoints) {
    for (let r = minRow; r <= maxRow; r++) indices.push(r);
    return indices;
  }
  const step = (total - 1) / (maxPoints - 1);
  for (let i = 0; i < maxPoints; i++) indices.push(minRow + Math.round(i * step));
  return indices;
}

/**
 * Build Recharts data for the selected range. `tableRows` is the filtered/sorted
 * row model, so the chart matches what is on screen. Selections larger than
 * `maxPoints` rows are reduced to evenly spaced rows; type detection runs over
 * the same rows.
 */
export function buildQuickChartData(
  tableRows: { original: unknown[] }[],
  columns: string[],
  selectionRange: SelectionRange,
  visualToOriginal: number[] | null,
  maxPoints = MAX_CHART_POINTS,
): QuickChartData {
  const minRow = Math.min(selectionRange.startRow, selectionRange.endRow);
  const maxRow = Math.min(Math.max(selectionRange.startRow, selectionRange.endRow), tableRows.length - 1);
  const minCol = Math.min(selectionRange.startCol, selectionRange.endCol);
  const maxCol = Math.max(selectionRange.startCol, selectionRange.endCol);

  // selectionRange columns are visual positions; translate each to its
  // original SELECT index (in visual, left-to-right order) for data reads.
  const colIndices: number[] = [];
  for (let c = minCol; c <= maxCol; c++) colIndices.push(visualToOriginalIndex(visualToOriginal, c));

  const names = colIndices.map((c) => columns[c] ?? `Col ${c}`);

  // ponytail: evenly spaced row sampling for every chart type — spikes between
  // sampled rows are dropped. Switch to min/max per bucket if that matters.
  const rowIndices = sampleRowIndices(minRow, maxRow, maxPoints);

  const numericFlags = colIndices.map((c) => isNumericColumn(rowIndices.map((r) => tableRows[r]?.original[c])));

  // Pick x-axis: first non-numeric column, or first column if all numeric
  let xIdx = numericFlags.findIndex((n) => !n);
  if (xIdx < 0) xIdx = 0;

  const xColIndex = colIndices[xIdx];
  const xName = names[xIdx];

  // Value columns: all numeric columns except the x-axis
  const valCols: { index: number; name: string }[] = [];
  for (let i = 0; i < colIndices.length; i++) {
    if (i === xIdx) continue;
    if (numericFlags[i]) {
      valCols.push({ index: colIndices[i], name: names[i] });
    }
  }

  // When all columns are numeric, xIdx is 0 and was excluded from valCols above.
  // Add it back so at least one value column is available for charting.
  if (valCols.length === 0 && numericFlags[xIdx]) {
    valCols.push({ index: colIndices[xIdx], name: names[xIdx] });
  }

  // Build chart data
  const data: Record<string, unknown>[] = [];
  for (const r of rowIndices) {
    const row = tableRows[r]?.original;
    if (!row) continue;
    const entry: Record<string, unknown> = {
      [xName]: row[xColIndex] != null ? String(row[xColIndex]) : `Row ${r}`,
    };
    for (const vc of valCols) {
      const val = row[vc.index];
      entry[vc.name] = val != null ? Number(val) : null;
    }
    data.push(entry);
  }

  return {
    data,
    xKey: xName,
    valueKeys: valCols.map((vc) => vc.name),
    totalRows: Math.max(0, maxRow - minRow + 1),
  };
}
