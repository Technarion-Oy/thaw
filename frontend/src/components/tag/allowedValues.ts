// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// @thaw-domain: Object Browser & Administration

// SHOW TAGS reports allowed_values as a JSON array string (e.g. ["a","b"]) or an
// empty/null value when the tag accepts any string. Parsed defensively so a
// format change degrades to "no restriction" rather than throwing. An empty
// result means the tag value field should stay free-text; a non-empty list backs
// a value dropdown.
export function parseAllowedValues(raw: string): string[] {
  const s = (raw ?? "").trim();
  if (s === "" || s.toLowerCase() === "null" || s === "[]") return [];
  try {
    const parsed = JSON.parse(s);
    if (Array.isArray(parsed)) return parsed.map((v) => String(v));
  } catch {
    /* fall through */
  }
  return [];
}
