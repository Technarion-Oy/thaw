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
// @thaw-domain: Object Browser & Administration
//
// Parsing / serialization for a session policy's ALLOWED_SECONDARY_ROLES and
// BLOCKED_SECONDARY_ROLES lists, shared by the properties modal and unit-tested
// in secondaryRoles.test.ts. Mirrors the backend sessionpolicy.FormatSecondaryRoles.

// A valid Snowflake unquoted identifier: a letter/underscore followed by
// letters, digits, underscores, or dollar signs. Such names can be emitted bare
// (Snowflake uppercases them on resolution); anything else must be double-quoted.
const SIMPLE_IDENT = /^[A-Za-z_][A-Za-z0-9_$]*$/;

// formatRoles renders a SECONDARY_ROLES list value for an ALTER SESSION POLICY
// clause, mirroring the backend sessionpolicy.FormatSecondaryRoles: the special
// token "ALL" (case-insensitive) becomes the quoted literal 'ALL'; every other
// entry is a role identifier emitted bare when it is a valid unquoted identifier
// (so "analyst" resolves to role ANALYST) and double-quoted only when it needs
// quoting. Blank entries are skipped. The result is parenthesized, e.g.
// ('ALL') or (R1, R2) or ("my role") or ().
export function formatRoles(roles: string[]): string {
  const parts = roles
    .map((r) => r.trim())
    .filter((r) => r !== "")
    .map((r) => {
      if (r.toUpperCase() === "ALL") return "'ALL'";
      return SIMPLE_IDENT.test(r) ? r : `"${r.replace(/"/g, '""')}"`;
    });
  return "(" + parts.join(", ") + ")";
}

// parseRoles parses a secondary-roles cell from DESCRIBE SESSION POLICY into a
// list of role tokens.
//
// Snowflake does not document this column's exact format, so two shapes are
// handled so the parse → edit → re-serialize round-trip can never corrupt the
// list:
//   - a SQL tuple, e.g. ('ALL') or (R1, "my role"); and
//   - a JSON-style array, e.g. ["ALL"] or ["R1","R2"].
// The outer (...) / [...] wrapper is stripped, then each entry is comma-split
// and any surrounding single/double quotes are removed. An empty / null cell
// yields an empty list.
export function parseRoles(raw: string): string[] {
  let s = (raw ?? "").trim();
  if (s === "" || s.toLowerCase() === "null") return [];
  if ((s.startsWith("(") && s.endsWith(")")) || (s.startsWith("[") && s.endsWith("]"))) {
    s = s.slice(1, -1);
  }
  if (s.trim() === "") return [];
  return s
    .split(",")
    .map((part) => {
      let p = part.trim();
      if (p.length >= 2 && ((p.startsWith("'") && p.endsWith("'")) || (p.startsWith('"') && p.endsWith('"')))) {
        p = p.slice(1, -1);
      }
      return p.trim();
    })
    .filter((p) => p !== "");
}
