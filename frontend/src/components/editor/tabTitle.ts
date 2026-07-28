// SPDX-License-Identifier: GPL-3.0-or-later
// @thaw-domain: SQL Editor & Diagnostics

import { OBJECT_KINDS } from "../../generated/objectKinds";
import type { Tab } from "../../store/queryStore";

/**
 * Provisional tab titles (issue #881).
 *
 * A strip of `SQL 1 … SQL 5` differs by one digit; a strip of `SELECT · ORDERS`,
 * `CREATE TABLE · CUSTOMERS`, `SHOW WAREHOUSES` tells you what each tab holds.
 * So a scratch tab that still carries its auto-generated title (`isDefaultTitle`)
 * shows a title *derived from its first statement* instead, falling back to the
 * counter while the tab is empty.
 *
 * This is display-only: `Tab.title` in the store is untouched, so rename, save
 * defaults and the `SQL n` numbering all keep working off the real title, and
 * the derived title disappears the moment the user renames or saves the tab.
 *
 * The parse is a deliberate heuristic — regex over the first statement, not the
 * real grammar in `internal/sqleditor` — because it runs on the store's hot path
 * (see `tabStripSignature`) and IPC per keystroke is out of the question. It is
 * allowed to be wrong: the cost of a mislabelled scratch tab is a slightly odd
 * word, and the user can always rename. Known simplifications: `--` and comment
 * delimiters are stripped even inside string literals, and only the first 2 000
 * characters are scanned.
 */

// A Snowflake identifier: bare (letters/digits/_/$) or double-quoted with ""
// escapes. QNAME adds dotted qualification and the `@` of a stage reference.
const IDENT = String.raw`(?:"(?:[^"]|"")+"|[A-Za-z_][\w$]*)`;
const QNAME = String.raw`@?${IDENT}(?:\.${IDENT})*`;

// Object kinds usable after CREATE/ALTER/DROP, sourced from the canonical
// registry (`internal/objectkind`) so a new object kind is understood here for
// free. The registry only covers what the object browser shows, so account- and
// session-level kinds are added on top.
const EXTRA_KINDS = [
  "DATABASE", "SCHEMA", "WAREHOUSE", "ROLE", "DATABASE ROLE", "USER", "SHARE",
  "INTEGRATION", "API INTEGRATION", "CATALOG INTEGRATION", "NOTIFICATION INTEGRATION",
  "SECURITY INTEGRATION", "STORAGE INTEGRATION", "EXTERNAL VOLUME", "COMPUTE POOL",
  "RESOURCE MONITOR", "CONNECTION", "REPLICATION GROUP", "FAILOVER GROUP",
  "APPLICATION", "APPLICATION PACKAGE", "LISTING", "SESSION", "ACCOUNT",
];

// Longest first, so "DYNAMIC TABLE" wins over "TABLE" and the kind capture never
// swallows half a two-word kind.
const KIND = `(?:${[...new Set([...OBJECT_KINDS.map((k) => k.name), ...EXTRA_KINDS])]
  .sort((a, b) => b.length - a.length)
  .map((k) => k.replace(/ /g, String.raw`\s+`))
  .join("|")})`;

// Modifiers that sit between CREATE and the object kind.
const CREATE_MODS = String.raw`(?:(?:LOCAL|GLOBAL|TEMP|TEMPORARY|TRANSIENT|VOLATILE|SECURE|RECURSIVE|MANAGED)\s+)*`;

const SEP = " · ";
/** Longest object name kept in a title; the strip ellipsizes anything longer anyway. */
const MAX_NAME = 22;

/**
 * The last segment of a qualified name, unquoted. Bare identifiers are
 * upper-cased (Snowflake resolves them that way); quoted ones keep their case,
 * since that is exactly what the quotes were for.
 */
function shortName(raw: string): string {
  const parts = raw.match(new RegExp(IDENT, "g")) ?? [];
  let last = parts[parts.length - 1] ?? raw;
  if (last.startsWith('"')) last = last.slice(1, -1).replace(/""/g, '"');
  else last = last.toUpperCase();
  return last.length > MAX_NAME ? `${last.slice(0, MAX_NAME - 1)}…` : last;
}

const withName = (verb: string) => (m: RegExpExecArray) => verb + SEP + shortName(m[1]);
/** `<verb> <kind>` + the name when the statement names one (`ALTER SESSION` does not). */
const withKind = (verb: string) => (m: RegExpExecArray) => {
  const head = `${verb} ${m[1].toUpperCase().replace(/\s+/g, " ")}`;
  return m[2] ? head + SEP + shortName(m[2]) : head;
};

// Words that can follow `ALTER <kind>` in place of a name — without this guard,
// `ALTER SESSION SET TIMEZONE` reads as an object called SET.
const NOT_A_NAME = String.raw`(?!(?:SET|UNSET|ADD|DROP|RENAME|SWAP|SUSPEND|RESUME|REFRESH|CLUSTER|MODIFY|ALTER|IF)\b)`;

/** First matching rule wins, so more specific patterns come first. */
const RULES: Array<{ re: RegExp; label: (m: RegExpExecArray) => string }> = [
  // Queries — the object is what you are reading from, not the projection.
  // `FROM TABLE(FLATTEN(…))` is skipped so the regex backtracks to a real table.
  { re: new RegExp(String.raw`^SELECT\b[\s\S]*?\bFROM\s+(?!TABLE\s*\()(${QNAME})`, "i"), label: withName("SELECT") },
  { re: /^SELECT\b/i,                                                      label: () => "SELECT" },
  { re: new RegExp(String.raw`^WITH\s+(${IDENT})`, "i"),                   label: withName("WITH") },

  // DML — the object is the write target.
  { re: new RegExp(String.raw`^INSERT\s+(?:OVERWRITE\s+)?INTO\s+(${QNAME})`, "i"), label: withName("INSERT") },
  { re: new RegExp(String.raw`^UPDATE\s+(${QNAME})`, "i"),                 label: withName("UPDATE") },
  { re: new RegExp(String.raw`^DELETE\s+FROM\s+(${QNAME})`, "i"),          label: withName("DELETE") },
  { re: new RegExp(String.raw`^MERGE\s+INTO\s+(${QNAME})`, "i"),           label: withName("MERGE") },
  { re: new RegExp(String.raw`^COPY\s+INTO\s+(${QNAME})`, "i"),            label: withName("COPY") },
  { re: new RegExp(String.raw`^TRUNCATE\s+(?:TABLE\s+)?(${QNAME})`, "i"),  label: withName("TRUNCATE") },

  // DDL — the kind is part of the verb ("CREATE TABLE · ORDERS").
  { re: new RegExp(String.raw`^CREATE\s+(?:OR\s+REPLACE\s+)?${CREATE_MODS}(${KIND})\s+(?:IF\s+NOT\s+EXISTS\s+)?(${QNAME})`, "i"), label: withKind("CREATE") },
  { re: new RegExp(String.raw`^ALTER\s+(${KIND})(?:\s+(?:IF\s+EXISTS\s+)?${NOT_A_NAME}(${QNAME}))?`, "i"),                       label: withKind("ALTER") },
  { re: new RegExp(String.raw`^DROP\s+(${KIND})\s+(?:IF\s+EXISTS\s+)?(${QNAME})`, "i"),                                          label: withKind("DROP") },
  { re: new RegExp(String.raw`^UNDROP\s+(${KIND})\s+(${QNAME})`, "i"),                                                           label: withKind("UNDROP") },

  // Introspection & session.
  { re: new RegExp(String.raw`^DESC(?:RIBE)?\s+(?:${KIND}\s+)?(${QNAME})`, "i"),   label: withName("DESCRIBE") },
  { re: /^SHOW\s+(?:TERSE\s+)?([A-Za-z]+(?:\s+[A-Za-z]+)?)/i,                      label: (m) => `SHOW ${showSubject(m[1])}` },
  { re: new RegExp(String.raw`^USE\s+(?:(${KIND})\s+)?(${QNAME})`, "i"),
    label: (m) => (m[1] ? withKind("USE")(m) : `USE${SEP}${shortName(m[2])}`) },
  { re: new RegExp(String.raw`^CALL\s+(${QNAME})`, "i"),                           label: withName("CALL") },

  // Privileges & stage files — the object after ON / the stage reference.
  { re: new RegExp(String.raw`^(GRANT|REVOKE)\b[\s\S]*?\bON\s+(?:${KIND}\s+)?(${QNAME})`, "i"),
    label: (m) => `${m[1].toUpperCase()}${SEP}${shortName(m[2])}` },
  { re: new RegExp(String.raw`^(PUT|GET|LIST|LS|REMOVE|RM)\b[\s\S]*?(@${IDENT}(?:\.${IDENT})*)`, "i"),
    label: (m) => `${m[1].toUpperCase()}${SEP}${shortName(m[2])}` },
];

/**
 * The word(s) after SHOW, minus the qualifier that starts the `IN <scope>` /
 * `LIKE '…'` tail — `SHOW TABLES IN SCHEMA X` is about TABLES.
 */
function showSubject(words: string): string {
  const [first, second] = words.toUpperCase().split(/\s+/);
  return second && !/^(IN|LIKE|STARTS|LIMIT|FROM|HISTORY)$/.test(second) ? `${first} ${second}` : first;
}

/**
 * The first statement, with comments and redundant whitespace collapsed away.
 * Only the head of the buffer is scanned — a title never depends on statement
 * 40 of a long script.
 */
function firstStatement(sql: string): string {
  const head = sql
    .slice(0, 2000)
    .replace(/\/\*[\s\S]*?(?:\*\/|$)/g, " ")
    .replace(/--[^\n]*/g, " ")
    .replace(/\s+/g, " ")
    .trim();
  const semi = head.indexOf(";");
  return (semi >= 0 ? head.slice(0, semi) : head).trim();
}

/**
 * A short, human-readable label for what a SQL buffer does — `SELECT · ORDERS`,
 * `CREATE TABLE · CUSTOMERS`, `SHOW WAREHOUSES` — or null when there is nothing
 * to describe yet (empty buffer, or comments only).
 */
export function provisionalTitle(sql: string): string | null {
  const stmt = firstStatement(sql);
  if (!stmt) return null;
  for (const rule of RULES) {
    const m = rule.re.exec(stmt);
    if (m) return rule.label(m);
  }
  // Unrecognised statement: the leading keyword alone (BEGIN, DECLARE, EXPLAIN…)
  // still beats "SQL 3".
  const word = /^[A-Za-z_]+/.exec(stmt);
  return word ? word[0].toUpperCase() : null;
}

/**
 * The title to render for a tab. Only tabs still carrying an auto-generated
 * title get a derived one; a renamed tab, a file-backed tab, and every non-SQL
 * kind keep the title the store holds.
 */
export function tabDisplayTitle(t: Tab): string {
  if (!t.isDefaultTitle || t.path) return t.title;
  if (t.kind && t.kind !== "sql") return t.title;
  return provisionalTitle(t.sql) ?? t.title;
}
