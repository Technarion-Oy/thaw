// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
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
