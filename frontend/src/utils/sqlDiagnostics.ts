// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.

import { Parser as SnowflakeParser } from "node-sql-parser/build/snowflake";
import type { sqleditor } from "../../wailsjs/go/models";

/** StatementRange as returned by GetSqlStatementRanges IPC. */
export type StatementRange = sqleditor.StatementRange;

// ── Helpers ───────────────────────────────────────────────────────────────────

const UC = (s: string) => s.toUpperCase();

// Helper to safely extract the first SQL keyword, completely ignoring
// leading newlines, spaces, and SQL comments.
function getFirstToken(sql: string): string | null {
  const stripped = sql.replace(/--.*$/gm, "").replace(/\/\*[\s\S]*?\*\//g, "").trimStart();
  return stripped.match(/^[a-zA-Z_]\w*/)?.[0]?.toUpperCase() ?? null;
}

// Local token finder that guarantees accurate line/col offsets without relying
// on the backend, completely immune to Go EOF tokenizer bugs.
function findTokensLocally(stmtText: string, targets: string[], baseLine: number) {
  const tokens: Array<{ name: string; line: number; col: number; endCol: number; quoted: boolean }> = [];
  const lines = stmtText.split("\n");
  const targetSet = new Set(targets.map(UC));
  
  for (let i = 0; i < lines.length; i++) {
    const lineStr = lines[i];
    // Match valid Snowflake identifiers: bare words or double-quoted strings
    const regex = /[a-zA-Z0-9_$]+|"[^"]+"/g;
    let match;
    while ((match = regex.exec(lineStr)) !== null) {
      let word = match[0];
      let quoted = false;
      if (word.startsWith('"') && word.endsWith('"')) {
        word = word.slice(1, -1);
        quoted = true;
      }
      if (targetSet.has(UC(word))) {
        tokens.push({
          name: word,
          line: baseLine + i,
          col: match.index + 1,
          endCol: match.index + 1 + match[0].length,
          quoted
        });
      }
    }
  }
  return tokens;
}

// ── Types ─────────────────────────────────────────────────────────────────────

export interface ColInfo { name: string; dataType: string; }

/** Subset of monaco.editor.IMarkerData used for SQL diagnostics. */
export interface DiagMarker {
  startLineNumber: number; 
  startColumn:     number;
  endLineNumber:   number;
  endColumn:       number;
  message:         string;
  severity:        8 | 4;  
}

// ── validateWithParser ────────────────────────────────────────────────────────

const PARSEABLE_STMT_KEYWORDS = new Set([
  "SELECT", "WITH", "INSERT", "UPDATE", "CREATE", "ALTER",
  "TRUNCATE", "CALL", "SHOW", "SET",
]);

const SNOWFLAKE_FP_RE = new RegExp(
  "\\bTABLESAMPLE\\b|\\bSAMPLE\\s*\\(|\\bWITHIN\\s+GROUP\\b|\\bCONNECT\\s+BY\\b" +
  "|\\bAT\\s*\\(|\\bBEFORE\\s*\\(|\\bIN\\s+TABLE\\b" +
  "|CREATE\\s+(?:OR\\s+REPLACE\\s+)?(?:TASK|STREAM|STAGE|PIPE|FUNCTION|PROCEDURE|AGGREGATE" +
  "|WAREHOUSE|ROLE|FILE\\s+FORMAT|USER|ALERT|SHARE|EXTERNAL|DYNAMIC|MATERIALIZED" +
  "|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK|RESOURCE|ROW\\s+ACCESS" +
  "|SESSION|PASSWORD|REPLICATION|FAILOVER|APPLICATION)\\b" +
  "|ALTER\\s+(?:VIEW|TASK|STREAM|WAREHOUSE|DATABASE|SEQUENCE|STAGE|PIPE" +
  "|USER|ALERT|SHARE|EXTERNAL|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK" +
  "|RESOURCE|REPLICATION|FAILOVER)\\b" +
  "|\\bCLUSTER\\s+(?:BY|KEY)\\b" +   
  "|\\bCLONE\\b" +                    
  "|INSERT\\s+OVERWRITE\\b" +         
  "|TRUNCATE\\s+\\S+\\s+IF\\b",       
  "i",
);

function cleanParserMessage(raw: string): string {
  // PEG.js messages are very verbose ("Expected ... but 'X' found.")
  const m = raw.match(/but\s+"([^"]+)"\s+found/i) ?? raw.match(/but\s+([^\s.]+)\s+found/i);
  if (m) return `Unexpected: '${m[1]}'`;
  if (/end of input/i.test(raw)) return "Unexpected end of statement";
  return raw.length > 100 ? raw.slice(0, 97) + "…" : raw;
}

export function validateWithParser(sql: string, stmtRanges: StatementRange[]): DiagMarker[] {
  const markers: DiagMarker[] = [];

  for (const r of stmtRanges) {
    const parser = new SnowflakeParser();
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    
    const firstToken = getFirstToken(rawStmtText);
    if (!firstToken || !PARSEABLE_STMT_KEYWORDS.has(firstToken)) continue;
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

    const parseText = rawStmtText.replace(/;+\s*$/, "");

    try {
      parser.parse(parseText);
    } catch (err: unknown) {
      const e = err as {
        location?: { start: { line: number; column: number } };
        message?: string;
      };
      if (e?.location?.start) {
        const stmtBaseLine = r.startLine;
        const errLine = stmtBaseLine + e.location.start.line - 1;
        const errCol  = e.location.start.column;

        const errLineText = rawStmtText.split("\n")[(e.location.start.line ?? 1) - 1] ?? "";
        const errColIdx   = errCol - 1;
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

  for (const r of stmtRanges) {
    const parser = new SnowflakeParser();
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    const firstToken = getFirstToken(rawStmtText);
    if (firstToken !== "SELECT" && firstToken !== "WITH") continue;
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

    const parseText = rawStmtText.replace(/;+\s*$/, "");

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let ast: any;
    try { ast = parser.parse(parseText); } catch { continue; }

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
      const allUnknown = [...new Set([...unknownBare, ...unknownQuoted])].map(UC);

      const tokens = findTokensLocally(rawStmtText, allUnknown, r.startLine);
      for (const t of tokens) {
        const message = t.quoted
          ? `Column '"${t.name}"' not found in ${tableLabel}`
          : `Column '${t.name}' not found in ${tableLabel}`;
        markers.push({
          startLineNumber: t.line,
          startColumn:     t.col,
          endLineNumber:   t.line,
          endColumn:       t.endCol,
          message,
          severity:        4,
        });
      }
    }
  }
  return markers;
}

// ── validateTablesExist ───────────────────────────────────────────────────────

export async function validateTablesExist(
  sql: string,
  stmtRanges: StatementRange[],
  resolvedRefs: ResolvedRef[],
): Promise<DiagMarker[]> {
  const markers: DiagMarker[] = [];
  const scriptCreatedTables = new Set<string>();

  // 1. PRE-PASS: Collect locally created tables
  for (const r of stmtRanges) {
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    const createMatch = rawStmtText.match(/^CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+|TEMPORARY\s+)?(?:TABLE|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    
    if (createMatch) {
      const parts = [...createMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);
      if (parts.length > 0) {
        const newTableName = parts[parts.length - 1].replace(/^"|"$/g, "").toUpperCase();
        scriptCreatedTables.add(newTableName);
      }
    }
  }

  // 2. PARSE & VALIDATE
  for (const r of stmtRanges) {
    const parser = new SnowflakeParser();
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    const firstToken = getFirstToken(rawStmtText);
    if (firstToken !== "SELECT" && firstToken !== "WITH") continue;
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

    const parseText = rawStmtText.replace(/;+\s*$/, "");

    let ast: any;
    try { ast = parser.parse(parseText); } catch { continue; }

    const stmtAsts: any[] = Array.isArray(ast.ast) ? ast.ast : [ast.ast];

    for (const node of stmtAsts) {
      if (!node || node.type !== "select") continue;

      const currentCTEs = new Set<string>();
      
      if (firstToken === "WITH" && node.with && Array.isArray(node.with)) {
        for (const cte of node.with) {
          const cteName = typeof cte.name === "string" ? cte.name : cte.name?.value;
          if (cteName) currentCTEs.add(UC(String(cteName)));
        }
      }

      const fromTables: any[] = node.from ?? [];
      const missingBare = new Set<string>();
      const missingQuoted = new Set<string>();

      for (const ft of fromTables) {
        if (!ft.table) continue; 
        
        const ftTable = String(ft.table);
        const ftTableUC = UC(ftTable);

        if (currentCTEs.has(ftTableUC)) continue;
        if (scriptCreatedTables.has(ftTableUC)) continue;
        
        const isLive = resolvedRefs.some(ref => UC(ref.name) === ftTableUC);
        if (isLive) continue;

        missingBare.add(ftTable);
        missingQuoted.add(ftTable);
      }

      if (missingBare.size === 0 && missingQuoted.size === 0) continue;

      const allUnknown = [...new Set([...missingBare, ...missingQuoted])].map(UC);
      
      // Use the local typescript token finder!
      const tokens = findTokensLocally(rawStmtText, allUnknown, r.startLine);
      
      for (const t of tokens) {
        markers.push({
          startLineNumber: t.line,
          startColumn:     t.col,
          endLineNumber:   t.line,
          endColumn:       t.endCol,
          message:         `Object '${t.name}' does not exist or is not authorized.`,
          severity:        8, 
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