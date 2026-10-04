// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: SQL Editor & Diagnostics

// Pure helpers for result-grid column reordering and visual⇄original column
// translation. Import-free so they can be unit-tested without React/TanStack.

/** The TanStack column ID for the column at SELECT position `i` named `name`. */
export function columnIdFor(i: number, name: string): string {
  return `${i}_${name}`;
}

/** The default (SELECT-order) columnOrder for a set of column names. */
export function defaultColumnOrder(columns: string[]): string[] {
  return columns.map((name, i) => columnIdFor(i, name));
}

/**
 * Move `draggedId` to just before/after `targetId` within `order`, returning a
 * new array. `order` is a list of stable column IDs (`{colIndex}_{NAME}`). If
 * either ID is absent, or source == target, the original array is returned
 * unchanged (referential identity preserved so callers can skip a state update).
 */
export function reorderColumnOrder(
  order: string[],
  draggedId: string,
  targetId: string,
  before: boolean,
): string[] {
  if (draggedId === targetId) return order;
  const from = order.indexOf(draggedId);
  if (from < 0) return order;
  const next = order.slice();
  next.splice(from, 1);
  let to = next.indexOf(targetId);
  if (to < 0) return order;
  if (!before) to += 1;
  next.splice(to, 0, draggedId);
  return next;
}

/**
 * Build the clipboard TSV for a cell range. `sel` columns are *visual*
 * positions, translated through `map` so the output follows the on-screen
 * column arrangement (reorder + pinning) left to right. Fields containing a
 * tab, newline, or double-quote are wrapped in double-quotes with internal
 * quotes doubled, so a paste into a spreadsheet keeps the rows × columns shape.
 */
export function selectionToTsv(
  sel: { startRow: number; endRow: number; startCol: number; endCol: number },
  columns: string[],
  rowAt: (rowIndex: number) => unknown[] | undefined,
  map: number[] | null | undefined,
  withHeaders: boolean,
): string {
  const esc = (v: string) => (/[\t\n\r"]/.test(v) ? `"${v.replace(/"/g, '""')}"` : v);
  const origCols: number[] = [];
  for (let c = Math.min(sel.startCol, sel.endCol); c <= Math.max(sel.startCol, sel.endCol); c++) {
    origCols.push(visualToOriginalIndex(map, c));
  }
  const lines: string[] = [];
  if (withHeaders) lines.push(origCols.map((oc) => esc(columns[oc] ?? "")).join("\t"));
  for (let r = Math.min(sel.startRow, sel.endRow); r <= Math.max(sel.startRow, sel.endRow); r++) {
    const row = rowAt(r);
    if (!row) continue;
    lines.push(origCols.map((oc) => esc(row[oc] == null ? "" : String(row[oc]))).join("\t"));
  }
  return lines.join("\n");
}

/**
 * Translate a visual column position to the original SELECT column index using
 * a `visualToOriginal` map (`map[visualPos] = originalIndex`). When `map` is
 * null/undefined (default order, no reorder/pinning) the position is the
 * original index, so it is returned unchanged.
 */
export function visualToOriginalIndex(map: number[] | null | undefined, visualPos: number): number {
  if (!map) return visualPos;
  const orig = map[visualPos];
  return orig === undefined ? visualPos : orig;
}
