// SPDX-License-Identifier: GPL-3.0-or-later

// ── Predecessor parsing ───────────────────────────────────────────────────────
// SHOW TASKS returns predecessors as a string that may look like:
//   ""                                              (no predecessors)
//   "DB"."SCHEMA"."TASK1"                           (single, qualified)
//   ["DB"."SCHEMA"."TASK1","DB"."SCHEMA"."TASK2"]   (array-like, not valid JSON)
//   ["DB.SCHEMA.TASK1"]                             (valid JSON with dotted names)
export function parsePredecessors(raw: string): string[] {
  if (!raw || raw === "[]") return [];
  // Try JSON parse first (handles valid JSON arrays)
  try {
    const arr = JSON.parse(raw);
    if (Array.isArray(arr)) return arr.map(String);
  } catch { /* fall through */ }
  // Strip surrounding brackets if present, then split on commas
  const stripped = raw.replace(/^\[|\]$/g, "");
  return stripped.split(",").map((s) => s.trim()).filter(Boolean);
}

// Extract the bare task name from a fully-qualified reference like "DB"."SCH"."TASK".
export function extractName(ref: string): string {
  const parts = ref.split(".");
  return parts[parts.length - 1].replace(/^"|"$/g, "");
}
