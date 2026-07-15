// SPDX-License-Identifier: GPL-3.0-or-later
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
