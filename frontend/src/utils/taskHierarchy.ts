// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

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
