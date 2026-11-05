// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration
//
// Clause builders for the ALTER DYNAMIC TABLE actions exposed in the sidebar
// context menu and DynamicTablePropertiesModal. Each returns the text that
// follows the table name and is passed to App.AlterDynamicTable. User-typed
// identifiers are validated (not quoted) so Snowflake's normal case rules apply
// and nothing but an identifier can be spliced in; expressions (CLUSTER BY,
// FROZEN WHERE, SEARCH OPTIMIZATION ON …) are passed through as written.

import { quoteTextLit } from "../shared/sqlEscape";

const PART = String.raw`(?:[A-Za-z_][A-Za-z0-9_$]*|"(?:[^"]|"")+")`;
const IDENT = String.raw`${PART}(?:\.${PART}){0,2}`;

/** Validate a bare or "quoted", optionally qualified identifier and return it trimmed. */
export function sqlIdent(v: string, maxParts = 3): string {
  const t = v.trim();
  if (!new RegExp(String.raw`^${PART}(?:\.${PART}){0,${maxParts - 1}}$`).test(t)) {
    throw new Error(`Not a valid identifier: ${t || "(empty)"}`);
  }
  return t;
}

/** Validate a comma-separated column list. */
// ponytail: splits on every comma, so a quoted column name containing a comma is rejected.
export function sqlIdentList(v: string): string {
  return v.split(",").map((c) => sqlIdent(c, 1)).join(", ");
}

const onList = (on: string) => (on.trim() ? ` ON ${on.trim()}` : "");

export const clusterBy = (expr: string) =>
  expr.trim() ? `CLUSTER BY (${expr.trim()})` : "DROP CLUSTERING KEY";

export const searchOptimization = (verb: "ADD" | "DROP" | "SUSPEND" | "RESUME", on: string) =>
  `${verb} SEARCH OPTIMIZATION${onList(on)}`;

/** `<user> [USE SECONDARY ROLES ALL | NONE | r1, r2]` → SET EXECUTE AS USER … */
export function executeAsUser(v: string): string {
  const m = /^(.+?)(?:\s+USE\s+SECONDARY\s+ROLES\s+(.+))?$/i.exec(v.trim());
  if (!m) throw new Error("A user name is required");
  const user = sqlIdent(m[1], 1);
  if (!m[2]) return `SET EXECUTE AS USER ${user}`;
  const roles = /^(ALL|NONE)$/i.test(m[2].trim()) ? m[2].trim().toUpperCase() : sqlIdentList(m[2]);
  return `SET EXECUTE AS USER ${user} USE SECONDARY ROLES ${roles}`;
}

/** CONTACT purpose is spliced bare, so it must be a plain word. */
export function setContact(purpose: string, contact: string): string {
  return `SET CONTACT ${contactPurpose(purpose)} = ${sqlIdent(contact)}`;
}
export function contactPurpose(purpose: string): string {
  const p = purpose.trim();
  if (!/^[A-Za-z0-9_]+$/.test(p)) throw new Error("Purpose must be letters, digits, or underscores");
  return p;
}

export const addRowAccessPolicy = (policy: string, cols: string) =>
  `ADD ROW ACCESS POLICY ${sqlIdent(policy)} ON (${sqlIdentList(cols)})`;

export function setAggregationPolicy(policy: string, entityKey: string, force: boolean): string {
  const ek = entityKey.trim() ? ` ENTITY KEY (${sqlIdentList(entityKey)})` : "";
  return `SET AGGREGATION POLICY ${sqlIdent(policy)}${ek}${force ? " FORCE" : ""}`;
}

export const addStorageLifecyclePolicy = (policy: string, cols: string) =>
  `ADD STORAGE LIFECYCLE POLICY ${sqlIdent(policy)} ON (${sqlIdentList(cols)})`;

// ─── Column-level ────────────────────────────────────────────────────────────

// Column names come from the sidebar (canonical case), so always quote them.
// (Not importing ObjectNameCaseControl.quoteIdent: that module loads Wails bindings, which breaks vitest.)
const alterColumn = (col: string, action: string) => `ALTER COLUMN "${col.replace(/"/g, '""')}" ${action}`;

export const columnComment = (col: string, comment: string) =>
  alterColumn(col, comment.trim() ? `COMMENT ${quoteTextLit(comment)}` : "UNSET COMMENT");

/** `<policy> [USING (c1, c2)] [FORCE]` → SET MASKING POLICY … */
export function columnMaskingPolicy(col: string, v: string): string {
  const m = new RegExp(String.raw`^(${IDENT})(?:\s+USING\s*\(([^)]*)\))?(\s+FORCE)?$`, "i").exec(v.trim());
  if (!m) throw new Error("Expected: <policy> [USING (col, …)] [FORCE]");
  const using = m[2] !== undefined ? ` USING (${sqlIdentList(m[2])})` : "";
  return alterColumn(col, `SET MASKING POLICY ${m[1]}${using}${m[3] ? " FORCE" : ""}`);
}

/** `<policy> [FORCE]` → SET PROJECTION POLICY … */
export function columnProjectionPolicy(col: string, v: string): string {
  const m = new RegExp(String.raw`^(${IDENT})(\s+FORCE)?$`, "i").exec(v.trim());
  if (!m) throw new Error("Expected: <policy> [FORCE]");
  return alterColumn(col, `SET PROJECTION POLICY ${m[1]}${m[2] ? " FORCE" : ""}`);
}

export const unsetColumnPolicy = (col: string, kind: "MASKING" | "PROJECTION") =>
  alterColumn(col, `UNSET ${kind} POLICY`);

/** `<tag> = <value>` (value optionally single-quoted) → SET TAG … */
export function columnSetTag(col: string, v: string): string {
  const m = new RegExp(String.raw`^(${IDENT})\s*=\s*(.*)$`, "s").exec(v.trim());
  if (!m) throw new Error("Expected: <tag> = <value>");
  const val = m[2].trim().replace(/^'(.*)'$/s, "$1");
  return alterColumn(col, `SET TAG ${m[1]} = ${quoteTextLit(val)}`);
}

export const columnUnsetTag = (col: string, tag: string) => alterColumn(col, `UNSET TAG ${sqlIdent(tag)}`);
