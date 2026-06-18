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
// Serialization + selection helpers for a session policy's
// ALLOWED_SECONDARY_ROLES and BLOCKED_SECONDARY_ROLES lists, shared by the
// properties / create modals and unit-tested in secondaryRoles.test.ts.
// `formatRoles` mirrors the backend snowflake.FormatSecondaryRoles. The inverse
// (parsing a DESCRIBE cell back into role tokens) lives in Go as
// snowflake.ParseSecondaryRoles and is reached via the App.ParseSecondaryRoles
// IPC binding, so the parse/serialize round-trip has a single source of truth.
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
