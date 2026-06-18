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
// in secondaryRoles.test.ts. Mirrors the backend snowflake.FormatSecondaryRoles.
//
// This module is deliberately dependency-free so it stays unit-testable without a
// Wails runtime: the reserved-keyword-aware quoting decision is injected as a
// `needsQuoting` predicate. The properties modal passes the shared
// `needsQuoting` from ../shared/ObjectNameCaseControl (which loads Snowflake's
// reserved-keyword list from the backend, matching the Go snowflake.NeedsQuoting),
// so the ALTER serialization here and the CREATE builder in internal/sessionpolicy
// quote identically — including reserved words such as ORDER.

// quoteIdent wraps name in double-quotes, doubling embedded quotes (Snowflake
// convention). Inlined here to keep the module free of heavier imports.
function quoteIdent(name: string): string {
  return '"' + name.replace(/"/g, '""') + '"';
}

// formatRoles renders a SECONDARY_ROLES list value for an ALTER SESSION POLICY
// clause: the special token "ALL" (case-insensitive) becomes the quoted literal
// 'ALL'; every other entry is a role identifier emitted bare when it is a valid
// unquoted identifier (so "analyst" resolves to role ANALYST) and double-quoted
// only when `needsQuoting` says so. Blank entries are skipped. The result is
// parenthesized, e.g. ('ALL') or (R1, R2) or ("my role") or ().
export function formatRoles(roles: string[], needsQuoting: (name: string) => boolean): string {
  const parts = roles
    .map((r) => r.trim())
    .filter((r) => r !== "")
    .map((r) => {
      if (r.toUpperCase() === "ALL") return "'ALL'";
      return needsQuoting(r) ? quoteIdent(r) : r;
    });
  return "(" + parts.join(", ") + ")";
}

// reconcileAll enforces the grammar's mutual exclusivity for a secondary-role
// list: `( { 'ALL' | <role_name> [, ...] } )` — the `ALL` token cannot be mixed
// with named roles. Given the new tag selection (in selection order, as antd's
// tag Select reports it), it keeps whichever kind was chosen last: if `ALL` was
// just added it collapses to `["ALL"]`; if a named role was added while `ALL`
// was already present it drops `ALL`. Lists without `ALL`, or with one entry,
// pass through unchanged. Prevents the invalid `('ALL', R1)` clause that
// Snowflake would otherwise reject only at execution time.
export function reconcileAll(next: string[]): string[] {
  const hasAll = next.some((r) => r.trim().toUpperCase() === "ALL");
  if (!hasAll || next.length <= 1) return next;
  const lastIsAll = next[next.length - 1].trim().toUpperCase() === "ALL";
  return lastIsAll ? ["ALL"] : next.filter((r) => r.trim().toUpperCase() !== "ALL");
}

// splitTopLevel splits a comma-separated list while ignoring commas that fall
// inside a single- or double-quoted segment, so a quoted role such as "a,b" is
// kept whole. A doubled quote ("" or '') inside a quoted segment is the SQL
// escape for a literal quote and does not end the segment.
function splitTopLevel(s: string): string[] {
  const out: string[] = [];
  let cur = "";
  let quote: '"' | "'" | null = null;
  for (let i = 0; i < s.length; i++) {
    const ch = s[i];
    if (quote) {
      if (ch === quote) {
        if (s[i + 1] === quote) { cur += ch + ch; i++; continue; } // doubled → escaped quote
        quote = null;
        cur += ch;
      } else {
        cur += ch;
      }
    } else if (ch === '"' || ch === "'") {
      quote = ch;
      cur += ch;
    } else if (ch === ",") {
      out.push(cur);
      cur = "";
    } else {
      cur += ch;
    }
  }
  out.push(cur);
  return out;
}

// parseRoles parses a secondary-roles cell from DESCRIBE SESSION POLICY into a
// list of role tokens.
//
// Snowflake does not document this column's exact format, so two shapes are
// handled so the parse → edit → re-serialize round-trip can never corrupt the
// list:
//   - a SQL tuple, e.g. ('ALL') or (R1, "my role"); and
//   - a JSON-style array, e.g. ["ALL"] or ["R1","R2"].
// The outer (...) / [...] wrapper is stripped, then each entry is split on
// top-level commas (commas inside quotes are preserved) and any surrounding
// single/double quotes are removed, un-doubling escaped quotes. An empty / null
// cell yields an empty list.
export function parseRoles(raw: string): string[] {
  let s = (raw ?? "").trim();
  if (s === "" || s.toLowerCase() === "null") return [];
  if ((s.startsWith("(") && s.endsWith(")")) || (s.startsWith("[") && s.endsWith("]"))) {
    s = s.slice(1, -1);
  }
  if (s.trim() === "") return [];
  return splitTopLevel(s)
    .map((part) => {
      let p = part.trim();
      if (p.length >= 2 && p.startsWith("'") && p.endsWith("'")) {
        p = p.slice(1, -1).replace(/''/g, "'");
      } else if (p.length >= 2 && p.startsWith('"') && p.endsWith('"')) {
        p = p.slice(1, -1).replace(/""/g, '"');
      }
      return p.trim();
    })
    .filter((p) => p !== "");
}
