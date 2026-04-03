// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { Parser as SnowflakeParser } from "node-sql-parser/build/snowflake";

// ── Helpers ───────────────────────────────────────────────────────────────────

const UC = (s: string) => s.toUpperCase();

// ── Types ─────────────────────────────────────────────────────────────────────

export interface ColInfo { name: string; dataType: string; }

/** Subset of monaco.editor.IMarkerData used for SQL diagnostics. */
export interface DiagMarker {
  startLineNumber: number; // 1-indexed (Monaco convention)
  startColumn:     number;
  endLineNumber:   number;
  endColumn:       number;
  message:         string;
  severity:        8 | 4;  // 8 = Error (red), 4 = Warning (yellow)
}

// ── SQL statement-starting keywords ──────────────────────────────────────────
// Used by validateSyntax to catch garbage tokens that appear after a semicolon.
// A token immediately after ';' that isn't in this set is flagged as an error.
const SQL_STMT_KEYWORDS = new Set([
  // DML
  "SELECT", "WITH", "INSERT", "UPDATE", "DELETE", "MERGE",
  // DDL
  "CREATE", "ALTER", "DROP", "TRUNCATE", "UNDROP", "COMMENT",
  // DCL / session
  "GRANT", "REVOKE", "USE", "SET", "UNSET",
  // Info
  "SHOW", "DESCRIBE", "DESC", "EXPLAIN",
  // TCL
  "BEGIN", "COMMIT", "ROLLBACK", "SAVEPOINT",
  // Execution
  "CALL", "EXECUTE", "RETURN",
  // Data loading
  "COPY", "PUT", "GET", "LIST", "REMOVE",
  // Snowflake scripting
  "DECLARE", "LET", "FOR", "WHILE", "IF", "CASE", "RAISE",
  "END", "LOOP",
  // Misc
  "ANALYZE",
]);

// ── validateSyntax ────────────────────────────────────────────────────────────

/**
 * Character-by-character tokenizer that catches structural errors:
 *   - Unclosed single-quoted strings
 *   - Unclosed double-quoted identifiers
 *   - Unclosed dollar-quoted strings
 *   - Unclosed block comments
 *   - Unmatched / extra closing parens and brackets
 *   - Unclosed opening parens and brackets
 *
 * Designed to have no false positives on well-formed Snowflake SQL.
 */
export function validateSyntax(sql: string): DiagMarker[] {
  const markers: DiagMarker[] = [];
  let line = 1, col = 1;
  // true at the start of the SQL and after each ';' — the next non-whitespace
  // bare-word token must be a recognised SQL statement-starting keyword.
  let atStmtStart = true;
  const parenStack: Array<{ char: string; line: number; col: number }> = [];

  const addError = (msg: string, sl: number, sc: number, el: number, ec: number): void => {
    markers.push({ startLineNumber: sl, startColumn: sc, endLineNumber: el, endColumn: ec, message: msg, severity: 8 });
  };

  let i = 0;
  while (i < sql.length) {
    const ch = sql[i];

    // Newline
    if (ch === "\n") {
      line++; col = 1; i++;
      continue;
    }

    // Line comment --
    if (ch === "-" && sql[i + 1] === "-") {
      i += 2; col += 2;
      while (i < sql.length && sql[i] !== "\n") { i++; col++; }
      continue;
    }

    // Block comment /* */
    if (ch === "/" && sql[i + 1] === "*") {
      const openLine = line, openCol = col;
      i += 2; col += 2;
      let closed = false;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "*" && sql[i + 1] === "/") {
          i += 2; col += 2; closed = true; break;
        } else { i++; col++; }
      }
      if (!closed) addError("Unclosed block comment", openLine, openCol, openLine, openCol + 2);
      continue;
    }

    // Single-quoted string '...'  ('' is the escape for a literal ')
    if (ch === "'") {
      const openLine = line, openCol = col;
      i++; col++;
      let closed = false;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "'" && sql[i + 1] === "'") { i += 2; col += 2; }
        else if (sql[i] === "'") { i++; col++; closed = true; break; }
        else { i++; col++; }
      }
      if (!closed) addError("Unclosed string literal", openLine, openCol, openLine, openCol + 1);
      continue;
    }

    // Double-quoted identifier "..."  ("" is the escape for a literal ")
    if (ch === '"') {
      const openLine = line, openCol = col;
      i++; col++;
      let closed = false;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === '"' && sql[i + 1] === '"') { i += 2; col += 2; }
        else if (sql[i] === '"') { i++; col++; closed = true; break; }
        else { i++; col++; }
      }
      if (!closed) addError("Unclosed quoted identifier", openLine, openCol, openLine, openCol + 1);
      continue;
    }

    // Dollar-quoted marker  $tag$...$tag$  (tag may be empty: $$)
    // We treat these as delimiters and DO NOT skip the content, so the character-by-character
    // scanner can see structural errors (unclosed parens, etc) inside the scripting block.
    if (ch === "$") {
      const dollarMatch = sql.slice(i).match(/^\$([a-zA-Z0-9_]*)\$/);
      if (dollarMatch) {
        const tag = dollarMatch[0];
        i += tag.length; col += tag.length;
        continue;
      }
    }

    // Opening paren / bracket
    if (ch === "(" || ch === "[") {
      parenStack.push({ char: ch, line, col });
      i++; col++;
      continue;
    }

    // Closing paren / bracket
    if (ch === ")" || ch === "]") {
      const expected = ch === ")" ? "(" : "[";
      if (parenStack.length === 0 || parenStack[parenStack.length - 1].char !== expected) {
        addError(`Unmatched '${ch}'`, line, col, line, col + 1);
      } else {
        parenStack.pop();
      }
      i++; col++;
      continue;
    }

    // Semicolon: marks end of one statement; next bare word must be a keyword
    if (ch === ";") {
      atStmtStart = true;
      i++; col++;
      continue;
    }

    // Post-semicolon (or start-of-SQL) word must be a known SQL statement keyword
    if (atStmtStart && /[a-zA-Z_]/.test(ch)) {
      const wordLine = line, wordCol = col;
      const wordStart = i;
      while (i < sql.length && /\w/.test(sql[i])) { i++; col++; }
      const word = sql.slice(wordStart, i);
      atStmtStart = false;
      if (!SQL_STMT_KEYWORDS.has(word.toUpperCase())) {
        addError(`Unexpected token '${word}'`, wordLine, wordCol, wordLine, wordCol + word.length);
      }
      continue;
    }

    // Any other non-whitespace character resets the statement-start flag silently
    if (atStmtStart && ch !== " " && ch !== "\t" && ch !== "\r") {
      atStmtStart = false;
    }
    i++; col++;
  }

  // Unclosed opening parens / brackets
  for (const open of parenStack) {
    addError(`Unclosed '${open.char}'`, open.line, open.col, open.line, open.col + 1);
  }

  return markers;
}

// ── validateWithParser ────────────────────────────────────────────────────────

// Statement-starting keywords whose SQL the Snowflake PEG parser handles
// correctly (no false positives on valid Snowflake syntax).
// Keywords NOT in this set — DELETE, MERGE, GRANT, REVOKE, EXPLAIN, BEGIN,
// COMMIT, ROLLBACK, USE, COPY, PUT, GET, UNSET, DESCRIBE, DECLARE, DROP, etc.
// all produce parser errors on valid Snowflake SQL and are therefore skipped.
//
// DROP is intentionally absent: the parser only handles `DROP TABLE` and fails
// on DROP VIEW, DROP TASK, DROP STREAM, DROP STAGE, DROP IF EXISTS, etc.
const PARSEABLE_STMT_KEYWORDS = new Set([
  "SELECT", "WITH", "INSERT", "UPDATE", "CREATE", "ALTER",
  "TRUNCATE", "CALL", "SHOW", "SET",
]);

// Within otherwise-parseable statements, these Snowflake-specific constructs
// also trip up the parser — skip the whole statement if any is detected.
//
// CREATE / ALTER: the parser only handles a subset of object types. Skip any
// statement that uses a Snowflake-specific object kind it doesn't know about.
const SNOWFLAKE_FP_RE = new RegExp(
  // Snowflake SELECT-clause constructs the parser can't handle
  "\\bTABLESAMPLE\\b|\\bSAMPLE\\s*\\(|\\bWITHIN\\s+GROUP\\b|\\bCONNECT\\s+BY\\b" +
  "|\\bAT\\s*\\(|\\bBEFORE\\s*\\(|\\bIN\\s+TABLE\\b" +
  // Snowflake-specific CREATE object types (parser only knows TABLE/VIEW/DATABASE/SCHEMA/SEQUENCE/INDEX)
  "|CREATE\\s+(?:OR\\s+REPLACE\\s+)?(?:TASK|STREAM|STAGE|PIPE|FUNCTION|PROCEDURE|AGGREGATE" +
  "|WAREHOUSE|ROLE|FILE\\s+FORMAT|USER|ALERT|SHARE|EXTERNAL|DYNAMIC|MATERIALIZED" +
  "|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK|RESOURCE|ROW\\s+ACCESS" +
  "|SESSION|PASSWORD|REPLICATION|FAILOVER|APPLICATION)\\b" +
  // Snowflake-specific ALTER object types (parser handles TABLE and SCHEMA/FUNCTION only)
  "|ALTER\\s+(?:VIEW|TASK|STREAM|WAREHOUSE|DATABASE|SEQUENCE|STAGE|PIPE" +
  "|USER|ALERT|SHARE|EXTERNAL|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK" +
  "|RESOURCE|REPLICATION|FAILOVER)\\b" +
  // Snowflake-specific clauses/keywords within otherwise-parseable statements
  "|\\bCLUSTER\\s+(?:BY|KEY)\\b" +   // ALTER TABLE ... CLUSTER BY/KEY
  "|\\bCLONE\\b" +                    // CREATE TABLE t CLONE src
  "|INSERT\\s+OVERWRITE\\b" +         // INSERT OVERWRITE INTO
  "|TRUNCATE\\s+\\S+\\s+IF\\b",       // TRUNCATE TABLE IF EXISTS
  "i",
);

/** Subset of a statement extracted from the full SQL text. */
interface SplitStmt {
  /** Raw slice of `sql` — NOT trimmed, so parser line numbers are accurate. */
  text:        string;
  /** Byte offset of `text[0]` within the original `sql` string. */
  startOffset: number;
}

/**
 * Splits `sql` into per-statement chunks (on `;`) while correctly skipping
 * string literals, quoted identifiers, dollar-quoted strings, and comments.
 * Because we only call this after `validateSyntax` has confirmed no structural
 * errors, we don't need to handle malformed input.
 */
function splitSqlStatements(sql: string): SplitStmt[] {
  const stmts: SplitStmt[] = [];
  let start = 0;
  let i = 0;

  while (i < sql.length) {
    const ch = sql[i];

    if (ch === "-" && sql[i + 1] === "-") {
      i += 2;
      while (i < sql.length && sql[i] !== "\n") i++;
      continue;
    }
    if (ch === "/" && sql[i + 1] === "*") {
      i += 2;
      while (i < sql.length) {
        if (sql[i] === "*" && sql[i + 1] === "/") { i += 2; break; }
        else i++;
      }
      continue;
    }
    if (ch === "'") {
      i++;
      while (i < sql.length) {
        if (sql[i] === "'" && sql[i + 1] === "'") i += 2;
        else if (sql[i] === "'") { i++; break; }
        else i++;
      }
      continue;
    }
    if (ch === '"') {
      i++;
      while (i < sql.length) {
        if (sql[i] === '"' && sql[i + 1] === '"') i += 2;
        else if (sql[i] === '"') { i++; break; }
        else i++;
      }
      continue;
    }
    if (ch === "$") {
      const m = sql.slice(i).match(/^\$([a-zA-Z0-9_]*)\$/);
      if (m) {
        const tag = m[0];
        i += tag.length;
        continue;
      }
    }
    if (ch === ";") {
      if (sql.slice(start, i).trim()) {
        const rawText = sql.slice(start, i);
        const trimmedText = rawText.trimStart();
        stmts.push({ text: trimmedText, startOffset: start + (rawText.length - trimmedText.length) });
      }
      start = i + 1;
      i++;
      continue;
    }
    i++;
  }
  if (sql.slice(start).trim()) {
    const rawText = sql.slice(start);
    const trimmedText = rawText.trimStart();
    stmts.push({ text: trimmedText, startOffset: start + (rawText.length - trimmedText.length) });
  }
  return stmts;
}

/** Counts how many newlines appear before character offset `offset` in `sql`. */
function newlinesBefore(sql: string, offset: number): number {
  let n = 0;
  for (let i = 0; i < offset && i < sql.length; i++) {
    if (sql[i] === "\n") n++;
  }
  return n;
}

function cleanParserMessage(raw: string): string {
  // PEG.js messages are very verbose ("Expected ... but 'X' found.")
  const m = raw.match(/but\s+"([^"]+)"\s+found/i) ?? raw.match(/but\s+([^\s.]+)\s+found/i);
  if (m) return `Unexpected: '${m[1]}'`;
  if (/end of input/i.test(raw)) return "Unexpected end of statement";
  return raw.length > 100 ? raw.slice(0, 97) + "…" : raw;
}

/**
 * Uses the node-sql-parser Snowflake grammar to detect grammatical errors.
 * Errors are emitted as **Warnings** (severity 4) because the parser may
 * produce false positives on valid Snowflake-specific syntax.
 *
 * Only statements whose first keyword is in `PARSEABLE_STMT_KEYWORDS` are
 * checked; all others are silently skipped.
 */
export function validateWithParser(sql: string): DiagMarker[] {
  const markers: DiagMarker[] = [];
  const parser = new SnowflakeParser();

  for (const stmt of splitSqlStatements(sql)) {
    // Only attempt statements the parser can handle without false positives.
    const firstToken = stmt.text.trimStart().match(/^[a-zA-Z_]\w*/)?.[0]?.toUpperCase();
    if (!firstToken || !PARSEABLE_STMT_KEYWORDS.has(firstToken)) continue;

    // Skip statements containing Snowflake syntax the parser chokes on.
    if (SNOWFLAKE_FP_RE.test(stmt.text)) continue;

    try {
      parser.parse(stmt.text);
    } catch (err: unknown) {
      const e = err as {
        location?: { start: { line: number; column: number } };
        message?: string;
      };
      if (e?.location?.start) {
        // stmt.text starts at startOffset in the original sql; translate line.
        const stmtBaseLine = newlinesBefore(sql, stmt.startOffset) + 1;
        const errLine = stmtBaseLine + e.location.start.line - 1;
        const errCol  = e.location.start.column; // 1-indexed (PEG.js convention)

        // Extend the squiggly to cover the full word at the error position so
        // a single mis-placed keyword like 'not' gets a 3-char span rather than
        // a barely-visible 1-char underline.
        const errLineText = stmt.text.split("\n")[(e.location.start.line ?? 1) - 1] ?? "";
        const errColIdx   = errCol - 1; // 0-indexed
        let wordEndIdx    = errColIdx;
        while (wordEndIdx < errLineText.length && /\w/.test(errLineText[wordEndIdx])) wordEndIdx++;
        const wordAtError = errLineText.slice(errColIdx, wordEndIdx);
        const endCol      = wordEndIdx > errColIdx ? wordEndIdx + 1 : errCol + 1; // 1-indexed
        const message     = wordAtError.length > 1
          ? `Unexpected: '${wordAtError}'`
          : cleanParserMessage(e.message ?? "Syntax error");

        markers.push({
          startLineNumber: errLine,
          startColumn:     errCol,
          endLineNumber:   errLine,
          endColumn:       endCol,
          message,
          severity:        4, // Warning — some false positives may remain
        });
      }
    }
  }

  return markers;
}

// ── validateBareColumnRefs ────────────────────────────────────────────────────

interface WordToken { name: string; line: number; col: number; endCol: number; }

/**
 * Walks `sql`, skipping strings/comments/quoted-idents, and returns every
 * unquoted bare-word token (not preceded by `.`, not followed by `(`) whose
 * upper-cased form is in `targets`.
 */
function findBareWordPositions(sql: string, targets: Set<string>): WordToken[] {
  const results: WordToken[] = [];
  let line = 1, col = 1, i = 0;

  while (i < sql.length) {
    const ch = sql[i];
    if (ch === "\n") { line++; col = 1; i++; continue; }
    if (ch === "-" && sql[i + 1] === "-") {
      i += 2; col += 2;
      while (i < sql.length && sql[i] !== "\n") { i++; col++; }
      continue;
    }
    if (ch === "/" && sql[i + 1] === "*") {
      i += 2; col += 2;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "*" && sql[i + 1] === "/") { i += 2; col += 2; break; }
        else { i++; col++; }
      }
      continue;
    }
    if (ch === "'") {
      i++; col++;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "'" && sql[i + 1] === "'") { i += 2; col += 2; }
        else if (sql[i] === "'") { i++; col++; break; }
        else { i++; col++; }
      }
      continue;
    }
    if (ch === '"') {
      i++; col++;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === '"' && sql[i + 1] === '"') { i += 2; col += 2; }
        else if (sql[i] === '"') { i++; col++; break; }
        else { i++; col++; }
      }
      continue;
    }
    if (ch === "$") {
      const m = sql.slice(i).match(/^\$([a-zA-Z0-9_]*)\$/);
      if (m) {
        const tag = m[0]; i += tag.length; col += tag.length;
        continue;
      }
    }
    if (/[a-zA-Z_]/.test(ch)) {
      const wLine = line, wCol = col, wStart = i;
      while (i < sql.length && /\w/.test(sql[i])) { i++; col++; }
      const word = sql.slice(wStart, i);
      const prevCh = wStart > 0 ? sql[wStart - 1] : null;
      const nextCh = i < sql.length ? sql[i] : null;
      if (prevCh !== "." && nextCh !== "(" && targets.has(word.toUpperCase())) {
        results.push({ name: word, line: wLine, col: wCol, endCol: wCol + word.length });
      }
      continue;
    }
    i++; col++;
  }
  return results;
}

/**
 * Walks `sql` and returns every double-quoted identifier `"name"` whose
 * inner name's upper-cased form is in `targets`.
 * `col` / `endCol` include the surrounding quotes in the span.
 */
function findQuotedWordPositions(sql: string, targets: Set<string>): WordToken[] {
  const results: WordToken[] = [];
  let line = 1, col = 1, i = 0;

  while (i < sql.length) {
    const ch = sql[i];
    if (ch === "\n") { line++; col = 1; i++; continue; }
    if (ch === "-" && sql[i + 1] === "-") {
      i += 2; col += 2;
      while (i < sql.length && sql[i] !== "\n") { i++; col++; }
      continue;
    }
    if (ch === "/" && sql[i + 1] === "*") {
      i += 2; col += 2;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "*" && sql[i + 1] === "/") { i += 2; col += 2; break; }
        else { i++; col++; }
      }
      continue;
    }
    if (ch === "'") {
      i++; col++;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "'" && sql[i + 1] === "'") { i += 2; col += 2; }
        else if (sql[i] === "'") { i++; col++; break; }
        else { i++; col++; }
      }
      continue;
    }
    if (ch === '"') {
      const startLine = line, startCol = col;
      i++; col++;
      let name = ""; let closed = false;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; name += "\n"; }
        else if (sql[i] === '"' && sql[i + 1] === '"') { name += '"'; i += 2; col += 2; }
        else if (sql[i] === '"') { i++; col++; closed = true; break; }
        else { name += sql[i]; i++; col++; }
      }
      if (closed && targets.has(name.toUpperCase())) {
        results.push({ name, line: startLine, col: startCol, endCol: col });
      }
      continue;
    }
    i++; col++;
  }
  return results;
}

/**
 * Uses the node-sql-parser AST to find bare and double-quoted column names in
 * SELECT lists, then cross-references them against `colInfoCache`.
 *
 * Rules:
 * - Only runs when the statement is parseable (SELECT / WITH, no Snowflake-FP patterns).
 * - Only validates when **all** FROM tables have warm cache entries; if any is cold
 *   (or is a subquery / CTE), the statement is silently skipped (no false positives).
 * - Covers both unquoted `column_name` and double-quoted `"column_name"` in the
 *   top-level SELECT list (Snowflake always treats `"..."` as quoted identifiers).
 */
export function validateBareColumnRefs(
  sql:          string,
  resolvedRefs: ResolvedRef[],
  colInfoCache: Map<string, ColInfo[]>,
): DiagMarker[] {
  const markers: DiagMarker[] = [];
  const parser = new SnowflakeParser();

  for (const stmt of splitSqlStatements(sql)) {
    const firstToken = stmt.text.trimStart().match(/^[a-zA-Z_]\w*/)?.[0]?.toUpperCase();
    if (firstToken !== "SELECT" && firstToken !== "WITH") continue;
    if (SNOWFLAKE_FP_RE.test(stmt.text)) continue;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let ast: any;
    try { ast = parser.parse(stmt.text); } catch { continue; }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const stmtAsts: any[] = Array.isArray(ast.ast) ? ast.ast : [ast.ast];

    for (const node of stmtAsts) {
      if (!node || node.type !== "select") continue;

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const fromTables: any[] = node.from ?? [];

      interface TableCheck { cacheKey: string; tableName: string; }
      const tableChecks: TableCheck[] = [];
      let skip = false;

      for (const ft of fromTables) {
        if (!ft.table) { skip = true; break; } // subquery or lateral in FROM → skip

        const ftDb    = (ft.db ?? ft.catalog) as string | null;
        const ftSchema = ft.schema as string | null;
        const ftTable  = ft.table as string;

        let cacheKey: string | undefined;
        if (ftDb && ftSchema) {
          // Fully qualified reference — build key directly from AST
          cacheKey = `${UC(ftDb)}\0${UC(ftSchema)}\0${UC(ftTable)}`;
        } else {
          // Unqualified — look up the resolved ref for the db/schema context
          const ref = resolvedRefs.find((r) =>
            UC(r.name) === UC(ftTable) &&
            (!ftDb     || UC(r.db)     === UC(ftDb))     &&
            (!ftSchema || UC(r.schema) === UC(ftSchema))
          );
          if (!ref) { skip = true; break; } // CTE, subquery alias, or unknown table
          cacheKey = `${UC(ref.db)}\0${UC(ref.schema)}\0${UC(ref.name)}`;
        }

        if (!colInfoCache.has(cacheKey)) { skip = true; break; } // cold cache → skip
        tableChecks.push({ cacheKey, tableName: ftTable });
      }

      if (skip || tableChecks.length === 0) continue;

      // Union of all column names across every FROM table
      const knownCols = new Set<string>();
      for (const tc of tableChecks) {
        for (const c of colInfoCache.get(tc.cacheKey)!) knownCols.add(UC(c.name));
      }

      const unknownBare   = new Set<string>(); // unquoted  column_ref
      const unknownQuoted = new Set<string>(); // "double_quote_string"

      for (const col of (node.columns ?? [])) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        const expr = (col as any)?.expr;
        if (!expr) continue;
        if (expr.type === "column_ref" && expr.table === null && expr.column !== "*") {
          if (!knownCols.has(UC(expr.column as string))) unknownBare.add(expr.column as string);
        } else if (expr.type === "double_quote_string") {
          if (!knownCols.has(UC(expr.value as string))) unknownQuoted.add(expr.value as string);
        }
      }

      if (unknownBare.size === 0 && unknownQuoted.size === 0) continue;

      const stmtBaseLine = newlinesBefore(sql, stmt.startOffset) + 1;
      const tableLabel = tableChecks.length === 1 ? tableChecks[0].tableName : "query tables";

      // findBareWordPositions / findQuotedWordPositions do targets.has(word.toUpperCase())
      // so the targets set must be uppercase to match.
      const unknownBareUC   = new Set([...unknownBare].map(UC));
      const unknownQuotedUC = new Set([...unknownQuoted].map(UC));

      for (const t of findBareWordPositions(stmt.text, unknownBareUC)) {
        markers.push({
          startLineNumber: stmtBaseLine + t.line - 1,
          startColumn:     t.col,
          endLineNumber:   stmtBaseLine + t.line - 1,
          endColumn:       t.endCol,
          message:         `Column '${t.name}' not found in ${tableLabel}`,
          severity:        4,
        });
      }
      for (const t of findQuotedWordPositions(stmt.text, unknownQuotedUC)) {
        markers.push({
          startLineNumber: stmtBaseLine + t.line - 1,
          startColumn:     t.col,
          endLineNumber:   stmtBaseLine + t.line - 1,
          endColumn:       t.endCol,
          message:         `Column '"${t.name}"' not found in ${tableLabel}`,
          severity:        4,
        });
      }
    }
  }
  return markers;
}

// ── validateSemantics ─────────────────────────────────────────────────────────

export interface ResolvedRef {
  alias:  string;
  db:     string;
  schema: string;
  name:   string;
}

/**
 * Walks the SQL text (skipping strings/comments/quoted-identifiers) and for
 * every `word1.word2` two-part reference where `word1` matches a known alias:
 *   - If the alias's table columns ARE in `colInfoCache` and `word2` is not
 *     among them → emit a Warning marker on the `word2` span.
 *   - If the columns are NOT yet cached → silent (no false positives).
 *
 * Three-part references (w1.w2.w3) are not checked.
 */
export function validateSemantics(
  sql:          string,
  resolvedRefs: ResolvedRef[],
  colInfoCache: Map<string, ColInfo[]>,
): DiagMarker[] {
  const markers: DiagMarker[] = [];

  // Build alias → cache key map  ("DB\0SCHEMA\0TABLE" keyed by UPPER(alias))
  const aliasMap = new Map<string, string>();
  for (const ref of resolvedRefs) {
    const cacheKey = `${ref.db.toUpperCase()}\0${ref.schema.toUpperCase()}\0${ref.name.toUpperCase()}`;
    aliasMap.set(ref.alias.toUpperCase(), cacheKey);
  }

  let line = 1, col = 1;
  let i = 0;

  while (i < sql.length) {
    const ch = sql[i];

    // ── Skip constructs that contain arbitrary text ───────────────────────
    if (ch === "\n") { line++; col = 1; i++; continue; }

    if (ch === "-" && sql[i + 1] === "-") {
      i += 2; col += 2;
      while (i < sql.length && sql[i] !== "\n") { i++; col++; }
      continue;
    }

    if (ch === "/" && sql[i + 1] === "*") {
      i += 2; col += 2;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "*" && sql[i + 1] === "/") { i += 2; col += 2; break; }
        else { i++; col++; }
      }
      continue;
    }

    if (ch === "'") {
      i++; col++;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === "'" && sql[i + 1] === "'") { i += 2; col += 2; }
        else if (sql[i] === "'") { i++; col++; break; }
        else { i++; col++; }
      }
      continue;
    }

    // Double-quoted identifier — skip contents; don't treat as an alias word
    if (ch === '"') {
      i++; col++;
      while (i < sql.length) {
        if (sql[i] === "\n") { line++; col = 1; i++; }
        else if (sql[i] === '"' && sql[i + 1] === '"') { i += 2; col += 2; }
        else if (sql[i] === '"') { i++; col++; break; }
        else { i++; col++; }
      }
      continue;
    }

    if (ch === "$") {
      const dollarMatch = sql.slice(i).match(/^\$([a-zA-Z0-9_]*)\$/);
      if (dollarMatch) {
        const tag = dollarMatch[0];
        i += tag.length; col += tag.length;
        continue;
      }
    }

    // ── Bare word token ───────────────────────────────────────────────────
    if (/[a-zA-Z_]/.test(ch)) {
      const word1Start = i;
      while (i < sql.length && /\w/.test(sql[i])) { i++; col++; }
      const word1 = sql.slice(word1Start, i);

      // Peek ahead: is the next character a dot followed by another bare word?
      const j    = i;
      const jCol = col;
      if (j < sql.length && sql[j] === ".") {
        const afterDot    = j + 1;
        const afterDotCol = jCol + 1;
        if (afterDot < sql.length && /[a-zA-Z_]/.test(sql[afterDot])) {
          // Scan word2
          const word2Col  = afterDotCol;
          const word2Line = line; // same line (no newline between word1 and .word2)
          let k    = afterDot;
          let kCol = afterDotCol;
          while (k < sql.length && /\w/.test(sql[k])) { k++; kCol++; }
          const word2 = sql.slice(afterDot, k);

          // If followed by yet another dot → three-part reference → skip
          if (!(k < sql.length && sql[k] === ".")) {
            const cacheKey = aliasMap.get(word1.toUpperCase());
            if (cacheKey !== undefined) {
              const cols = colInfoCache.get(cacheKey);
              if (cols !== undefined) {
                // Cache is warm — validate existence of word2
                const found = cols.some((c) => c.name.toUpperCase() === word2.toUpperCase());
                if (!found) {
                  const tableName = cacheKey.split("\0")[2] ?? word1;
                  markers.push({
                    startLineNumber: word2Line,
                    startColumn:     word2Col,
                    endLineNumber:   word2Line,
                    endColumn:       word2Col + word2.length,
                    message:         `Column '${word2}' does not exist in ${tableName}`,
                    severity:        4,
                  });
                }
              }
            }
          }
        }
      }

      continue;
    }

    i++; col++;
  }

  return markers;
}
