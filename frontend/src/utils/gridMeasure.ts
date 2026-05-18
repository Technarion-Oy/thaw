// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

// Shared canvas-based text measurement and column auto-sizing utilities
// used by ResultGrid and PipeCopyHistoryModal.

const _ctxCache = new Map<string, CanvasRenderingContext2D>();
const MAX_CTX_CACHE = 10;

/** Measure the pixel width of `text` at the given CSS font. */
export function measureText(text: string, font: string): number {
  let ctx = _ctxCache.get(font);
  if (!ctx) {
    if (_ctxCache.size >= MAX_CTX_CACHE) _ctxCache.clear();
    const canvas = document.createElement("canvas");
    ctx = canvas.getContext("2d") ?? undefined;
    if (ctx) {
      ctx.font = font;
      _ctxCache.set(font, ctx);
    }
  }
  if (!ctx) return text.length * 7;
  return ctx.measureText(text).width;
}

/**
 * Compute initial column widths from header text + first N rows of data.
 * Each column width is clamped between `minWidth` and `maxWidth`.
 */
export function computeColumnWidths(
  columns: string[],
  rows: unknown[][],
  opts: {
    font: string;
    minWidth: number;
    maxWidth: number;
    sampleRows?: number;
    nullText?: string;
  },
): number[] {
  const { font, minWidth, maxWidth, sampleRows = 100, nullText = "" } = opts;
  const widths: number[] = [];
  const slicedRows = rows.slice(0, sampleRows);

  for (let colIdx = 0; colIdx < columns.length; colIdx++) {
    let maxW = measureText(columns[colIdx], font) + 32;
    for (const row of slicedRows) {
      const val = row[colIdx];
      const text = val == null ? nullText : String(val);
      const w = measureText(text, font) + 16;
      if (w > maxW) maxW = w;
    }
    widths.push(Math.max(minWidth, Math.min(maxWidth, Math.ceil(maxW))));
  }
  return widths;
}
