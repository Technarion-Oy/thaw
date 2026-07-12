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

// Pure helpers for CellDetailPanel and ResultGrid.scrollToCell, kept free of
// imports so they can be unit-tested in vitest's node environment.

/** Characters shown before the panel truncates and offers "Show all". */
export const DISPLAY_CAP = 500_000;

/** Values longer than this skip JSON detection — parse + re-stringify of
 *  multi-MB VARIANT cells (up to 16 MB in Snowflake) would freeze the UI. */
export const JSON_DETECT_CAP = 1_000_000;

/**
 * Pretty-print a JSON value for display. Returns null when the value is not
 * JSON-shaped, fails to parse, is already in the formatted form, or exceeds
 * JSON_DETECT_CAP.
 */
export function prettyPrintJson(raw: string): string | null {
  if (raw.length > JSON_DETECT_CAP) return null;
  const t = raw.trim();
  if (!t.startsWith("{") && !t.startsWith("[")) return null;
  try {
    const formatted = JSON.stringify(JSON.parse(t), null, 2);
    return formatted === t ? null : formatted;
  } catch {
    return null;
  }
}

/** GeoJSON `type` values we recognise as map-renderable (geometry objects plus
 *  Feature/FeatureCollection). Matches what Leaflet's `L.geoJSON` accepts. */
const GEOJSON_TYPES = new Set([
  "Point", "MultiPoint", "LineString", "MultiLineString",
  "Polygon", "MultiPolygon", "GeometryCollection", "Feature", "FeatureCollection",
]);

/**
 * Parse a cell value as GeoJSON, returning the parsed object when it has a
 * recognised GeoJSON `type` (so the Map view can render it) or null otherwise.
 * Snowflake GEOGRAPHY/GEOMETRY cells arrive as GeoJSON strings under the
 * default `GEOGRAPHY_OUTPUT_FORMAT=GEOJSON`; WKT/WKB won't JSON.parse and
 * correctly return null. Skips values above JSON_DETECT_CAP to avoid freezing
 * on multi-MB VARIANT cells.
 */
export function parseGeoJson(raw: string): unknown | null {
  if (raw.length > JSON_DETECT_CAP) return null;
  const t = raw.trim();
  if (!t.startsWith("{")) return null;
  try {
    const obj = JSON.parse(t);
    return obj && typeof obj === "object" && GEOJSON_TYPES.has(obj.type) ? obj : null;
  } catch {
    return null;
  }
}

/** Truncate text to DISPLAY_CAP unless the user asked for the full value. */
export function truncateForDisplay(text: string, showFull: boolean): { text: string; truncated: boolean } {
  if (showFull || text.length <= DISPLAY_CAP) return { text, truncated: false };
  return { text: text.slice(0, DISPLAY_CAP), truncated: true };
}

/**
 * Dismissal state machine: an explicit close (Escape/✕) applies only to the
 * anchor it was set for and clears as soon as the anchor moves — including to
 * null (new result), so a dismissal can never suppress an unrelated cell that
 * happens to land on the same coordinates.
 */
export function reconcileDismissedKey(prev: string | null, anchorKey: string | null): string | null {
  return prev !== anchorKey ? null : prev;
}

export interface CellScrollArgs {
  scrollLeft: number;
  clientWidth: number;
  /** Content-x of the column's left edge (absolute, including sticky gutters). */
  colStart: number;
  colWidth: number;
  /** Width of the sticky leading region (row-number gutter + pinned-left columns). */
  stickyLeadingWidth: number;
  /** Width of the sticky trailing region (pinned-right columns). */
  stickyTrailingWidth: number;
}

/**
 * Minimal horizontal scroll to make a column fully visible, where "visible"
 * excludes the sticky regions that overlay the scrolled content. Returns the
 * new scrollLeft, or null when no scroll is needed. For columns wider than
 * the visible window, keeps the column start in view rather than overshooting.
 */
export function computeCellScrollLeft(a: CellScrollArgs): number | null {
  const contentEnd = a.colStart + a.colWidth;
  const viewStart = a.scrollLeft + a.stickyLeadingWidth;
  const viewEnd = a.scrollLeft + a.clientWidth - a.stickyTrailingWidth;
  if (a.colStart < viewStart) return a.scrollLeft - (viewStart - a.colStart);
  if (contentEnd > viewEnd) return a.scrollLeft + Math.min(contentEnd - viewEnd, a.colStart - viewStart);
  return null;
}
