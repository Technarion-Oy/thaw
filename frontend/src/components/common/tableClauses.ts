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

/**
 * `<name> | PRIMARY KEY (cols) | UNIQUE (cols) | FOREIGN KEY (cols)` followed by
 * `[NOT] ENFORCED`, `VALIDATE | NOVALIDATE`, `RELY | NORELY` → ALTER …
 */
export function alterConstraint(v: string): string {
  const m = /^(.*?)((?:\s+(?:NOT\s+ENFORCED|ENFORCED|VALIDATE|NOVALIDATE|RELY|NORELY))+)$/is.exec(v.trim());
  if (!m) throw new Error("Expected: <constraint> [NOT] ENFORCED | VALIDATE | NOVALIDATE | RELY | NORELY");
  const target = m[1].trim();
  const props = m[2].trim().toUpperCase().replace(/\s+/g, " ");
  const key = /^(PRIMARY\s+KEY|UNIQUE|FOREIGN\s+KEY)\s*\(([^)]*)\)$/i.exec(target);
  if (key) return `ALTER ${key[1].toUpperCase().replace(/\s+/g, " ")} (${sqlIdentList(key[2])}) ${props}`;
  return `ALTER CONSTRAINT ${sqlIdent(target, 1)} ${props}`;
}

/** `<old> [TO] <new>` → RENAME CONSTRAINT … */
export function renameConstraint(v: string): string {
  const m = new RegExp(String.raw`^(${PART})\s+(?:TO\s+)?(${PART})$`, "i").exec(v.trim());
  if (!m) throw new Error("Expected: <old name> TO <new name>");
  return `RENAME CONSTRAINT ${m[1]} TO ${m[2]}`;
}

// ─── Data metric functions ───────────────────────────────────────────────────

/**
 * `<function> ON (cols [, TABLE <t>(cols)])` — parsed once, reused by every DMF
 * action. The optional TABLE part is for multi-table DMFs (e.g. referential checks).
 */
function dmfTarget(v: string): { fn: string; on: string; rest: string } {
  const m = new RegExp(
    String.raw`^(${IDENT})\s+ON\s*\(([^(),]*(?:,[^(),]*)*?)(?:,?\s*TABLE\s+(${IDENT})\s*\(([^)]*)\))?\s*\)(.*)$`, "is",
  ).exec(v.trim());
  if (!m) throw new Error("Expected: <function> ON (col, … [, TABLE <table>(col, …)])");
  const tbl = m[3] ? `${m[2].trim() ? ", " : ""}TABLE ${m[3]}(${sqlIdentList(m[4])})` : "";
  return { fn: m[1], on: `ON (${cols(m[2])}${tbl})`, rest: m[5].trim() };
}

// Trailing DMF clauses are passed through (like CHECK / FILTER expressions) but
// must start with a known keyword and contain no statement separator.
function dmfTail(rest: string, allowed: RegExp, expected: string): string {
  if (!rest) return "";
  if (rest.includes(";")) throw new Error("Semicolons are not allowed");
  if (!allowed.test(rest)) throw new Error(`Expected ${expected} after ON (…)`);
  return ` ${rest}`;
}

/**
 * `<function> ON (cols) [WITHIN GROUP (…) [GROUP LIMIT n]] [FILTER (…)]
 * [EXPECTATION name (expr), …] [EXECUTE AS ROLE r] [ANOMALY_DETECTION = …] [SENSITIVITY = …]`
 * → ADD DATA METRIC FUNCTION …
 */
export function addDataMetricFunction(v: string): string {
  const { fn, on, rest } = dmfTarget(v);
  const tail = dmfTail(rest, /^(WITHIN\s+GROUP|FILTER|EXPECTATION|EXECUTE\s+AS\s+ROLE|ANOMALY_DETECTION|SENSITIVITY)\b/i,
    "WITHIN GROUP, FILTER, EXPECTATION, EXECUTE AS ROLE, ANOMALY_DETECTION or SENSITIVITY");
  return `ADD DATA METRIC FUNCTION ${fn} ${on}${tail}`;
}

/**
 * `<function> ON (cols) SUSPEND | RESUME | {ADD|MODIFY} EXPECTATION … | DROP EXPECTATION … | FILTER ([pred])`
 * or `<function> SET <props>` → MODIFY DATA METRIC FUNCTION …
 */
export function modifyDataMetricFunctionAction(v: string): string {
  const set = new RegExp(String.raw`^(${IDENT})\s+SET\s+(.+)$`, "is").exec(v.trim());
  if (set) {
    if (set[2].includes(";")) throw new Error("Semicolons are not allowed");
    return `MODIFY DATA METRIC FUNCTION ${set[1]} SET ${set[2].trim()}`;
  }
  const { fn, on, rest } = dmfTarget(v);
  if (!rest) throw new Error("Add an action: SUSPEND, RESUME, ADD/MODIFY/DROP EXPECTATION or FILTER (…)");
  const tail = dmfTail(rest, /^(SUSPEND|RESUME|ADD\s+EXPECTATION|MODIFY\s+EXPECTATION|DROP\s+EXPECTATION|FILTER)\b/i,
    "SUSPEND, RESUME, ADD/MODIFY/DROP EXPECTATION or FILTER (…)");
  return `MODIFY DATA METRIC FUNCTION ${fn} ${on}${tail}`;
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

/** Atomic swap: DROP ROW ACCESS POLICY <old>, ADD ROW ACCESS POLICY <new> ON (cols). */
export const replaceRowAccessPolicy = (oldPolicy: string, newPolicy: string, cols: string) =>
  `DROP ROW ACCESS POLICY ${sqlIdent(oldPolicy)}, ADD ROW ACCESS POLICY ${sqlIdent(newPolicy)} ON (${sqlIdentList(cols)})`;

export const setJoinPolicy = (policy: string, force: boolean) =>
  `SET JOIN POLICY ${sqlIdent(policy)}${force ? " FORCE" : ""}`;

// ─── Column masking (USING / FORCE suffix on the backend-built statement) ─────

/** Append `USING (col, cond…)` / `FORCE` to a `… SET MASKING POLICY p;` statement. */
export function withMaskingOptions(sql: string, using: string, force: boolean): string {
  const u = using.trim() ? ` USING (${sqlIdentList(using)})` : "";
  return `${sql.trim().replace(/;$/, "")}${u}${force ? " FORCE" : ""};`;
}
