// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

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

    // Dollar-quoted string  $tag$...$tag$  (tag may be empty: $$...$$)
    if (ch === "$") {
      const dollarMatch = sql.slice(i).match(/^\$([a-zA-Z0-9_]*)\$/);
      if (dollarMatch) {
        const openLine = line, openCol = col;
        const tag = dollarMatch[0]; // e.g. "$$" or "$body$"
        i += tag.length; col += tag.length;
        let closed = false;
        while (i < sql.length) {
          if (sql[i] === "\n") { line++; col = 1; i++; }
          else if (sql.startsWith(tag, i)) {
            i += tag.length; col += tag.length; closed = true; break;
          } else { i++; col++; }
        }
        if (!closed) addError(`Unclosed dollar-quoted string`, openLine, openCol, openLine, openCol + tag.length);
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
        while (i < sql.length) {
          if (sql[i] === "\n") { line++; col = 1; i++; }
          else if (sql.startsWith(tag, i)) { i += tag.length; col += tag.length; break; }
          else { i++; col++; }
        }
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
