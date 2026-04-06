// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { Parser as SnowflakeParser } from "node-sql-parser/build/snowflake";
import type { sqleditor } from "../../wailsjs/go/models";
import { FindSqlTokenPositions } from "../../wailsjs/go/main/App";

/** StatementRange as returned by GetSqlStatementRanges IPC. */
export type StatementRange = sqleditor.StatementRange;

/** TokenMatch as returned by FindSqlTokenPositions IPC. */
export type TokenMatch = sqleditor.TokenMatch;

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
 *
 * `stmtRanges` must be the result of `GetSqlStatementRanges(sql)` — one
 * range per statement with pre-computed start lines and byte offsets.
 */
export function validateWithParser(sql: string, stmtRanges: StatementRange[]): DiagMarker[] {
  const markers: DiagMarker[] = [];
  const parser = new SnowflakeParser();

  for (const r of stmtRanges) {
    const stmtText = sql.slice(r.startOffset, r.endOffset).trimStart();
    // Only attempt statements the parser can handle without false positives.
    const firstToken = stmtText.match(/^[a-zA-Z_]\w*/)?.[0]?.toUpperCase();
    if (!firstToken || !PARSEABLE_STMT_KEYWORDS.has(firstToken)) continue;

    // Skip statements containing Snowflake syntax the parser chokes on.
    if (SNOWFLAKE_FP_RE.test(stmtText)) continue;

    try {
      parser.parse(stmtText);
    } catch (err: unknown) {
      const e = err as {
        location?: { start: { line: number; column: number } };
        message?: string;
      };
      if (e?.location?.start) {
        const stmtBaseLine = r.startLine;
        const errLine = stmtBaseLine + e.location.start.line - 1;
        const errCol  = e.location.start.column; // 1-indexed (PEG.js convention)

        // Extend the squiggly to cover the full word at the error position so
        // a single mis-placed keyword like 'not' gets a 3-char span rather than
        // a barely-visible 1-char underline.
        const errLineText = stmtText.split("\n")[(e.location.start.line ?? 1) - 1] ?? "";
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

/**
 * Uses the node-sql-parser AST to find bare and double-quoted column names in
 * SELECT lists, then cross-references them against `colInfoCache`.
 *
 * Rules:
 * - Only runs when the statement is parseable (SELECT / WITH, no Snowflake-FP patterns).
 * - Only validates when **all** FROM tables have warm cache entries; if any is cold
 * (or is a subquery / CTE), the statement is silently skipped (no false positives).
 * - Covers both unquoted `column_name` and double-quoted `"column_name"` in the
 * top-level SELECT list (Snowflake always treats `"..."` as quoted identifiers).
 *
 * `stmtRanges` must be the result of `GetSqlStatementRanges(sql)` — one
 * range per statement with pre-computed start lines and byte offsets.
 */
export async function validateBareColumnRefs(
  sql:          string,
  stmtRanges:   StatementRange[],
  resolvedRefs: ResolvedRef[],
  colInfoCache: Map<string, ColInfo[]>,
): Promise<DiagMarker[]> {
  const markers: DiagMarker[] = [];
  const parser = new SnowflakeParser();

  for (const r of stmtRanges) {
    const stmtText = sql.slice(r.startOffset, r.endOffset).trimStart();
    const firstToken = stmtText.match(/^[a-zA-Z_]\w*/)?.[0]?.toUpperCase();
    if (firstToken !== "SELECT" && firstToken !== "WITH") continue;
    if (SNOWFLAKE_FP_RE.test(stmtText)) continue;

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let ast: any;
    try { ast = parser.parse(stmtText); } catch { continue; }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const stmtAsts: any[] = Array.isArray(ast.ast) ? ast.ast : [ast.ast];

    const stmtBaseLine = r.startLine;

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
          const ref = resolvedRefs.find((rr) =>
            UC(rr.name) === UC(ftTable) &&
            (!ftDb     || UC(rr.db)     === UC(ftDb))     &&
            (!ftSchema || UC(rr.schema) === UC(ftSchema))
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

      const tableLabel = tableChecks.length === 1 ? tableChecks[0].tableName : "query tables";

      // FindSqlTokenPositions matches targets case-insensitively in Go, so pass
      // uppercase values to match the UC-normalised knownCols set used above.
      // We pass ALL unknowns to BOTH bare and quoted targets, because node-sql-parser
      // sometimes categorizes double-quoted columns ("MY_COL") as standard column_refs.
      const allUnknown = [...new Set([...unknownBare, ...unknownQuoted])].map(UC);

      // eslint-disable-next-line no-await-in-loop
      const tokens = await FindSqlTokenPositions(stmtText, allUnknown, allUnknown);
      for (const t of (tokens ?? [])) {
        const message = t.quoted
          ? `Column '"${t.name}"' not found in ${tableLabel}`
          : `Column '${t.name}' not found in ${tableLabel}`;
        markers.push({
          startLineNumber: stmtBaseLine + t.line - 1,
          startColumn:     t.col,
          endLineNumber:   stmtBaseLine + t.line - 1,
          endColumn:       t.endCol,
          message,
          severity:        4,
        });
      }
    }
  }
  return markers;
}

export interface ResolvedRef {
  alias:  string;
  db:     string;
  schema: string;
  name:   string;
}