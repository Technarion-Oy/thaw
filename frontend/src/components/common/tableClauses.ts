// SPDX-License-Identifier: GPL-3.0-or-later
//
// @thaw-domain: Object Browser & Administration
//
// Clause builders for the ALTER TABLE actions exposed in the sidebar context
// menu, PropertiesModal (base tables) and ColumnPropertiesModal that the
// dynamic-table builders don't already cover. Each returns the text that follows
// the table name (passed to App.AlterTable). Same rules as dynamicTableClauses:
// user-typed identifiers are validated, not quoted; expressions (CHECK, FILTER)
// are passed through as written.

import { quoteTextLit } from "../shared/sqlEscape";
import { sqlIdent, sqlIdentList } from "../dynamictable/dynamicTableClauses";

const PART = String.raw`(?:[A-Za-z_][A-Za-z0-9_$]*|"(?:[^"]|"")+")`;
const IDENT = String.raw`${PART}(?:\.${PART}){0,2}`;

// Column names from the sidebar are canonical-case, so always quote them.
const qcol = (c: string) => `"${c.replace(/"/g, '""')}"`;

const cols = (v: string) => (v.trim() ? sqlIdentList(v) : "");

// ─── Columns ─────────────────────────────────────────────────────────────────

/** DROP COLUMN a[, b …]. CASCADE / RESTRICT is only documented for a single column. */
export function dropColumns(columns: string[], mode: "" | "CASCADE" | "RESTRICT" = ""): string {
  if (columns.length === 0) throw new Error("No columns selected");
  if (mode && columns.length > 1) throw new Error(`${mode} applies to a single column only`);
  return `DROP COLUMN ${columns.map(qcol).join(", ")}${mode ? ` ${mode}` : ""}`;
}

// ─── Constraints ─────────────────────────────────────────────────────────────

const CONSTRAINT_PROPS = /^(?:\s+(?:NOT\s+)?(?:ENFORCED|DEFERRABLE)|\s+INITIALLY\s+(?:DEFERRED|IMMEDIATE)|\s+(?:ENABLE|DISABLE|VALIDATE|NOVALIDATE|RELY|NORELY))*$/i;

/**
 * `[CONSTRAINT n] PRIMARY KEY (a) | UNIQUE (a) | FOREIGN KEY (a) REFERENCES t [(b)] | CHECK (expr)`
 * plus optional constraint properties → ADD …
 */
export function addConstraint(v: string): string {
  const t = v.trim();
  const named = new RegExp(String.raw`^CONSTRAINT\s+(${PART})\s+(.*)$`, "is").exec(t);
  const name = named ? `CONSTRAINT ${named[1]} ` : "";
  const body = named ? named[2] : t;

  const chk = /^CHECK\s*\((.*)\)$/is.exec(body);
  if (chk) {
    if (!name) throw new Error("A CHECK constraint needs a name: CONSTRAINT <name> CHECK (…)");
    if (chk[1].includes(";")) throw new Error("Semicolons are not allowed");
    return `ADD ${name}CHECK (${chk[1].trim()})`;
  }

  const m = new RegExp(
    String.raw`^(PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY)\s*\(([^)]*)\)(?:\s+REFERENCES\s+(${IDENT})\s*(?:\(([^)]*)\))?)?(.*)$`,
    "is",
  ).exec(body);
  if (!m) throw new Error("Expected: [CONSTRAINT <name>] PRIMARY KEY | UNIQUE | FOREIGN KEY (cols) [REFERENCES <table> (cols)]");
  const kind = m[1].toUpperCase().replace(/\s+/g, " ");
  if (kind === "FOREIGN KEY" && !m[3]) throw new Error("A foreign key needs REFERENCES <table> (cols)");
  if (kind !== "FOREIGN KEY" && m[3]) throw new Error("REFERENCES only applies to a foreign key");
  if (!CONSTRAINT_PROPS.test(m[5])) throw new Error(`Unrecognised constraint properties: ${m[5].trim()}`);
  const ref = m[3] ? ` REFERENCES ${m[3]}${m[4] !== undefined ? ` (${sqlIdentList(m[4])})` : ""}` : "";
  const props = m[5].trim() ? ` ${m[5].trim().toUpperCase().replace(/\s+/g, " ")}` : "";
  return `ADD ${name}${kind} (${sqlIdentList(m[2])})${ref}${props}`;
}

/** `<name> | PRIMARY KEY | UNIQUE (cols) | FOREIGN KEY (cols)` `[CASCADE | RESTRICT]` → DROP … */
export function dropConstraint(v: string): string {
  const m = /^(.*?)(?:\s+(CASCADE|RESTRICT))?$/is.exec(v.trim())!;
  const target = m[1].trim();
  const mode = m[2] ? ` ${m[2].toUpperCase()}` : "";
  const key = /^(PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY)\s*(?:\(([^)]*)\))?$/i.exec(target);
  if (key) {
    const kind = key[1].toUpperCase().replace(/\s+/g, " ");
    if (kind !== "PRIMARY KEY" && key[2] === undefined) throw new Error(`${kind} needs a column list`);
    return `DROP ${kind}${key[2] !== undefined ? ` (${sqlIdentList(key[2])})` : ""}${mode}`;
  }
  return `DROP CONSTRAINT ${sqlIdent(target, 1)}${mode}`;
}

/** `<old> [TO] <new>` → RENAME CONSTRAINT … */
export function renameConstraint(v: string): string {
  const m = new RegExp(String.raw`^(${PART})\s+(?:TO\s+)?(${PART})$`, "i").exec(v.trim());
  if (!m) throw new Error("Expected: <old name> TO <new name>");
  return `RENAME CONSTRAINT ${m[1]} TO ${m[2]}`;
}

// ─── Data metric functions ───────────────────────────────────────────────────

/** `<function> ON (cols)` — parsed once, reused by every DMF action. */
function dmfTarget(v: string): { fn: string; on: string; rest: string } {
  const m = new RegExp(String.raw`^(${IDENT})\s+ON\s*\(([^)]*)\)(.*)$`, "is").exec(v.trim());
  if (!m) throw new Error("Expected: <function> ON (col, …)");
  return { fn: m[1], on: `ON (${cols(m[2])})`, rest: m[3].trim() };
}

/** `<function> ON (cols) [FILTER (pred)] [EXECUTE AS ROLE r]` → ADD DATA METRIC FUNCTION … */
export function addDataMetricFunction(v: string): string {
  const { fn, on, rest } = dmfTarget(v);
  const m = new RegExp(String.raw`^(?:FILTER\s*\((.*?)\))?\s*(?:EXECUTE\s+AS\s+ROLE\s+(${PART}))?$`, "is").exec(rest);
  if (!m) throw new Error("Only FILTER (…) and EXECUTE AS ROLE <role> may follow ON (…)");
  if (m[1]?.includes(";")) throw new Error("Semicolons are not allowed");
  const filter = m[1] !== undefined ? ` FILTER (${m[1].trim()})` : "";
  const role = m[2] ? ` EXECUTE AS ROLE ${m[2]}` : "";
  return `ADD DATA METRIC FUNCTION ${fn} ${on}${filter}${role}`;
}

export function dropDataMetricFunction(v: string): string {
  const { fn, on, rest } = dmfTarget(v);
  if (rest) throw new Error("Expected: <function> ON (col, …)");
  return `DROP DATA METRIC FUNCTION ${fn} ${on}`;
}

export function modifyDataMetricFunction(v: string, verb: "SUSPEND" | "RESUME"): string {
  const { fn, on, rest } = dmfTarget(v);
  if (rest) throw new Error("Expected: <function> ON (col, …)");
  return `MODIFY DATA METRIC FUNCTION ${fn} ${on} ${verb}`;
}

/** `5 MINUTE` | `USING CRON …` | `TRIGGER_ON_CHANGES`; blank unsets. */
export function dataMetricSchedule(v: string): string {
  const t = v.trim().replace(/^'(.*)'$/s, "$1");
  return t ? `SET DATA_METRIC_SCHEDULE = ${quoteTextLit(t)}` : "UNSET DATA_METRIC_SCHEDULE";
}

// ─── Governance (table-only) ─────────────────────────────────────────────────

export const setJoinPolicy = (policy: string, force: boolean) =>
  `SET JOIN POLICY ${sqlIdent(policy)}${force ? " FORCE" : ""}`;

// ─── Column masking (USING / FORCE suffix on the backend-built statement) ─────

/** Append `USING (col, cond…)` / `FORCE` to a `… SET MASKING POLICY p;` statement. */
export function withMaskingOptions(sql: string, using: string, force: boolean): string {
  const u = using.trim() ? ` USING (${sqlIdentList(using)})` : "";
  return `${sql.trim().replace(/;$/, "")}${u}${force ? " FORCE" : ""};`;
}
