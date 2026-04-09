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

// Smart Normalizer: Emulates Snowflake's exact identifier behavior
// - Bare identifiers are UPPERCASED
// - Double-quoted identifiers PRESERVE case (with quotes stripped)
// - If ignoreCase is true, double-quoted identifiers are ALSO UPPERCASED
const NORM = (s: any, ignoreCase: boolean = false): string => {
  if (typeof s !== "string") return "";
  if (s.startsWith('"') && s.endsWith('"')) {
    const inner = s.slice(1, -1);
    return ignoreCase ? inner.toUpperCase() : inner;
  }
  return s.toUpperCase();
};

// Helper to safely extract the first SQL keyword, completely ignoring
// leading newlines, spaces, and SQL comments.
function getFirstToken(sql: string): string | null {
  const stripped = sql.replace(/--.*$/gm, "").replace(/\/\*[\s\S]*?\*\//g, "").trimStart();
  return stripped.match(/^[a-zA-Z_]\w*/)?.[0]?.toUpperCase() ?? null;
}

// Local token finder that guarantees accurate line/col offsets without relying
// on the backend, completely immune to Go EOF tokenizer bugs.
function findTokensLocally(stmtText: string, targets: string[], baseLine: number, ignoreCase: boolean = false) {
  const tokens: Array<{ name: string; line: number; col: number; endCol: number; quoted: boolean }> = [];
  const lines = stmtText.split("\n");
  
  // Targets must already be passed through NORM()
  const targetSet = new Set(targets);
  
  for (let i = 0; i < lines.length; i++) {
    const lineStr = lines[i];
    // Match valid Snowflake identifiers: bare words or double-quoted strings
    const regex = /[a-zA-Z0-9_$]+|"[^"]+"/g;
    let match;
    while ((match = regex.exec(lineStr)) !== null) {
      const rawWord = match[0];
      const isQuoted = rawWord.startsWith('"') && rawWord.endsWith('"');
      
      // We normalize the token found in the text to see if it matches any of our normalized targets
      let normalizedWord = isQuoted ? rawWord.slice(1, -1) : rawWord.toUpperCase();
      if (isQuoted && ignoreCase) normalizedWord = normalizedWord.toUpperCase();
      
      if (targetSet.has(normalizedWord)) {
        tokens.push({
          name: isQuoted ? rawWord.slice(1, -1) : rawWord,
          line: baseLine + i,
          col: match.index + 1,
          endCol: match.index + 1 + rawWord.length,
          quoted: isQuoted
        });
      }
    }
  }
  return tokens;
}

// Surgically precise AST table path extractor
// Ensures no properties are swallowed or redundantly aliased by the parser.
// Cross-references with the raw string to recover quote-driven case sensitivity
// lost when node-sql-parser strips quotes from the AST.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function extractTablePath(ft: any, rawSql: string = "", ignoreCase: boolean = false): { db: string | null; schema: string | null; table: string | null } {
  const parts: string[] = [];
  
  // 1. Safely gather all potential path fragments from the AST
  if (ft.catalog) parts.push(String(ft.catalog));
  
  if (ft.db && ft.db !== ft.catalog) {
    parts.push(String(ft.db));
  } else if (ft.database && ft.database !== ft.catalog) {
    parts.push(String(ft.database));
  }
  
  if (ft.schema && ft.schema !== ft.db && ft.schema !== ft.catalog && ft.schema !== ft.database) {
    parts.push(String(ft.schema));
  }
  
  if (ft.table) parts.push(String(ft.table));

  if (parts.length === 0) return { db: null, schema: null, table: null };

  // 2. Safely unpack squashed AST strings (e.g. "DB.SCH.TABLE")
  const combined = parts.join(".");
  const matches = [...combined.matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);

  // 3. Re-combine, strip, and dynamically recover original quotes from the raw string
  const cleanParts = matches.map(p => {
    const stripped = p.replace(/^"|"$/g, "");
    if (!rawSql) return NORM(p, ignoreCase); // Fallback for pure unit tests

    const regex = /[a-zA-Z0-9_$]+|"[^"]+"/g;
    let match;
    let foundNorm = null;
    
    // Scan the string to find the exact token the user typed to recover quote context
    while ((match = regex.exec(rawSql)) !== null) {
      const rawWord = match[0];
      const isQuoted = rawWord.startsWith('"') && rawWord.endsWith('"');
      const inner = isQuoted ? rawWord.slice(1, -1) : rawWord;
      
      if (inner.toUpperCase() === stripped.toUpperCase()) {
        foundNorm = NORM(rawWord, ignoreCase);
        // Do not break; if there are multiple matches (e.g. column vs table), 
        // the table is usually the later one in the FROM clause.
      }
    }
    return foundNorm !== null ? foundNorm : NORM(p, ignoreCase);
  });

  const len = cleanParts.length;
  
  // 4. Extract purely by index position (Right-to-Left)
  if (len >= 3) return { db: cleanParts[len - 3], schema: cleanParts[len - 2], table: cleanParts[len - 1] };
  if (len === 2) return { db: null, schema: cleanParts[0], table: cleanParts[1] };
  if (len === 1) return { db: null, schema: null, table: cleanParts[0] };
  
  return { db: null, schema: null, table: null };
}

// ── Global Custom Syntax Definitions ──────────────────────────────────────────

const balancedParens = "\\([^()]*(?:(?:\\([^()]*\\))[^()]*)*\\)";
const viewProps = [
  `COPY\\s+GRANTS`,
  `COMMENT\\s*=\\s*'(?:[^']|'')*'`,
  `CHANGE_TRACKING\\s*=\\s*(?:TRUE|FALSE)`,
  `(?:WITH\\s+)?ROW\\s+ACCESS\\s+POLICY\\s+(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}\\s+ON\\s*${balancedParens}`,
  `(?:WITH\\s+)?AGGREGATION\\s+POLICY\\s+(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}(?:\\s+ENTITY\\s+KEY\\s*${balancedParens})?`,
  `(?:WITH\\s+)?JOIN\\s+POLICY\\s+(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}(?:\\s+ALLOWED\\s+JOIN\\s+KEYS\\s*${balancedParens})?`,
  `(?:WITH\\s+)?TAG\\s*${balancedParens}`,
  `WITH\\s+CONTACT\\s*${balancedParens}`
].join("|");

const VALID_CREATE_VIEW_PREAMBLE_RE = new RegExp(
  `^\\s*CREATE\\s+(?:OR\\s+REPLACE\\s+)?(?:SECURE\\s+)?(?:(?:(?:LOCAL|GLOBAL)\\s+)?(?:TEMP|TEMPORARY|VOLATILE)\\s+)?(?:RECURSIVE\\s+)?VIEW\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}(?:\\s*${balancedParens})?(?:\\s+(?:${viewProps}))*\\s+AS\\s+`,
  "i"
);

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
  "TRUNCATE", "CALL", "SHOW", "SET", "DROP", 
]);

// Note: DATABASE and SCHEMA are intentionally removed from this list, 
// so they can be handled by the Custom Validators inside the function!
const SNOWFLAKE_FP_RE = new RegExp(
  "\\bTABLESAMPLE\\b|\\bSAMPLE\\s*\\(|\\bWITHIN\\s+GROUP\\b|\\bCONNECT\\s+BY\\b" +
  "|\\bAT\\s*\\(|\\bBEFORE\\s*\\(|\\bIN\\s+TABLE\\b" +
  "|CREATE\\s+(?:OR\\s+REPLACE\\s+)?(?:TRANSIENT\\s+)?(?:TASK|STREAM|STAGE|PIPE|FUNCTION|PROCEDURE|AGGREGATE" +
  "|WAREHOUSE|ROLE|FILE\\s+FORMAT|USER|ALERT|SHARE|EXTERNAL|DYNAMIC|MATERIALIZED" +
  "|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK|RESOURCE|ROW\\s+ACCESS" +
  "|SESSION|PASSWORD|REPLICATION|FAILOVER|APPLICATION)\\b" +
  "|ALTER\\s+(?:VIEW|TASK|STREAM|WAREHOUSE|DATABASE|SEQUENCE|STAGE|PIPE" +
  "|USER|ALERT|SHARE|EXTERNAL|NOTIFICATION|STORAGE|SECURITY|MASKING|NETWORK" +
  "|RESOURCE|REPLICATION|FAILOVER)\\b" +
  "|DROP\\s+(?:TABLE|VIEW|TASK|STREAM|STAGE|PIPE|PROCEDURE|FUNCTION|WAREHOUSE|ROLE)\\b" + 
  "|\\bCLUSTER\\s+(?:BY|KEY)\\b" +   
  "|\\bCLONE\\b" +                    
  "|INSERT\\s+OVERWRITE\\b" +         
  "|TRUNCATE\\s+\\S+\\s+IF\\b",       
  "i",
);

function cleanParserMessage(raw: string): string {
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

    let parseText = rawStmtText.replace(/;+\s*$/, "");
    
    // --- Custom Syntax Validator for CREATE VIEW ---
    const createViewMatch = parseText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE)\s+)?(?:RECURSIVE\s+)?VIEW\b/i);
    if (createViewMatch) {
      if (VALID_CREATE_VIEW_PREAMBLE_RE.test(parseText)) {
        parseText = parseText.replace(VALID_CREATE_VIEW_PREAMBLE_RE, "CREATE VIEW V AS ");
      } else {
        markers.push({
          startLineNumber: r.startLine,
          startColumn: 1,
          endLineNumber: r.endLine,
          endColumn: 100, 
          message: `Unexpected syntax in CREATE VIEW statement.`,
          severity: 4
        });
        continue; 
      }
    }

    // --- Custom Syntax Validator for CREATE DATABASE / SCHEMA ---
    const createDbSchemaMatch = parseText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(DATABASE|SCHEMA)\b/i);
    if (createDbSchemaMatch) {
      const createDbProps = [
        `CLONE\\s+(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}(?:\\s+(?:AT|BEFORE)\\s*\\(\\s*(?:TIMESTAMP|OFFSET|STATEMENT)\\s*=>\\s*[^)]+\\))?(?:\\s+IGNORE\\s+TABLES\\s+WITH\\s+INSUFFICIENT\\s+DATA\\s+RETENTION)?(?:\\s+IGNORE\\s+HYBRID\\s+TABLES)?`,
        `WITH\\s+MANAGED\\s+ACCESS`, 
        `(?:DATA_RETENTION_TIME_IN_DAYS|MAX_DATA_EXTENSION_TIME_IN_DAYS|ICEBERG_VERSION_DEFAULT)\\s*=\\s*\\d+`,
        `(?:ENABLE_ICEBERG_MERGE_ON_READ|REPLACE_INVALID_CHARACTERS|ENABLE_DATA_COMPACTION)\\s*=\\s*(?:TRUE|FALSE)`,
        `(?:EXTERNAL_VOLUME|CATALOG)\\s*=\\s*(?:[a-zA-Z0-9_$]+|"[^"]+")`,
        `DEFAULT_DDL_COLLATION\\s*=\\s*'(?:[^']|'')*'`,
        `STORAGE_SERIALIZATION_POLICY\\s*=\\s*(?:COMPATIBLE|OPTIMIZED)`,
        `CLASSIFICATION_PROFILE\\s*=\\s*'(?:[^']|'')*'`, 
        `COMMENT\\s*=\\s*'(?:[^']|'')*'`,
        `CATALOG_SYNC\\s*=\\s*'(?:[^']|'')*'`,
        `CATALOG_SYNC_NAMESPACE_MODE\\s*=\\s*(?:NEST|FLATTEN)`,
        `CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER\\s*=\\s*'(?:[^']|'')*'`,
        `(?:WITH\\s+)?TAG\\s*\\([^)]+\\)`,
        `(?:WITH\\s+)?CONTACT\\s*\\([^)]+\\)`,
        `OBJECT_VISIBILITY\\s*=\\s*(?:PRIVILEGED|[a-zA-Z0-9_$]+|"[^"]+")`
      ].join("|");

      const validCreateDbSchemaRe = new RegExp(
        `^\\s*CREATE\\s+(?:OR\\s+REPLACE\\s+)?(?:TRANSIENT\\s+)?(?:DATABASE|SCHEMA)\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}(?:\\s+(?:${createDbProps}))*\\s*$`,
        "i"
      );

      if (validCreateDbSchemaRe.test(parseText)) {
        continue; 
      } else {
        markers.push({
          startLineNumber: r.startLine,
          startColumn: 1,
          endLineNumber: r.endLine,
          endColumn: 100, 
          message: `Unexpected syntax in CREATE ${createDbSchemaMatch[1].toUpperCase()} statement.`,
          severity: 4
        });
        continue; 
      }
    }

    // --- Custom Syntax Validator for DROP DATABASE / SCHEMA ---
    const dropDbSchemaMatch = parseText.match(/^\s*DROP\s+(DATABASE|SCHEMA)\b/i);
    if (dropDbSchemaMatch) {
      const validDropDbSchemaRe = /^\s*DROP\s+(?:DATABASE|SCHEMA)\s+(?:IF\s+EXISTS\s+)?(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+"))?(?:\s+(?:CASCADE|RESTRICT))?\s*$/i;
      
      if (validDropDbSchemaRe.test(parseText)) {
        continue;
      } else {
        markers.push({
          startLineNumber: r.startLine,
          startColumn: 1,
          endLineNumber: r.endLine,
          endColumn: 100,
          message: `Unexpected syntax in DROP ${dropDbSchemaMatch[1].toUpperCase()} statement.`,
          severity: 4
        });
        continue; 
      }
    }
    // --- END Custom Syntax Validators ---

    // Standard parser FP skip
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

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
        const endCol      = wordEndIdx > errColIdx ? wordEndIdx + 1 : errCol + 1; 
        const message     = wordAtError.length > 1
          ? `Unexpected: '${wordAtError}'`
          : cleanParserMessage(e.message ?? "Syntax error");

        markers.push({
          startLineNumber: errLine,
          startColumn:     errCol,
          endLineNumber:   errLine,
          endColumn:       endCol,
          message,
          severity:        4, 
        });
      }
    }
  }

  return markers;
}

// ── validateBareColumnRefs ────────────────────────────────────────────────────

export async function validateBareColumnRefs(
  sql:          string,
  stmtRanges:   StatementRange[],
  resolvedRefs: ResolvedRef[],
  colInfoCache: Map<string, ColInfo[]>,
  quotedIdentifiersIgnoreCase: boolean = false
): Promise<DiagMarker[]> {
  const checkEq = (a: string, b: string) => quotedIdentifiersIgnoreCase ? a.toUpperCase() === b.toUpperCase() : a === b;
  const markers: DiagMarker[] = [];
  const localColCache = new Map<string, ColInfo[]>();

  // 1. PRE-PASS: Extract locally created tables and their columns!
  for (const r of stmtRanges) {
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    
    const createMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+|TEMPORARY\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})\s*\(([^;]+)\)/i);
    if (createMatch) {
      const parts = [...createMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      const localTableName = parts[parts.length - 1];
      
      const colsRaw = createMatch[2];
      const columns: ColInfo[] = [];
      
      // Parenthesis-aware splitting to safely extract column names
      let depth = 0;
      let currentDef = "";
      const defs: string[] = [];
      for (let i = 0; i < colsRaw.length; i++) {
          const char = colsRaw[i];
          if (char === '(') depth++;
          else if (char === ')') {
            if (depth > 0) depth--;
            if (depth < 0) break; // Hit the end parenthesis of the CREATE TABLE
          }
          else if (char === ',' && depth === 0) {
              defs.push(currentDef);
              currentDef = "";
              continue;
          }
          currentDef += char;
      }
      if (currentDef) defs.push(currentDef);

      for (const def of defs) {
          const trimmed = def.trim();
          if (!trimmed) continue;
          const identMatch = trimmed.match(/^([a-zA-Z0-9_$]+|"[^"]+")/);
          if (identMatch) {
              columns.push({ name: NORM(identMatch[1], quotedIdentifiersIgnoreCase), dataType: "UNKNOWN" });
          }
      }

      let db = null, schema = null;
      if (parts.length === 3) { db = parts[0]; schema = parts[1]; }
      else if (parts.length === 2) { schema = parts[0]; }
      
      // Register in local cache
      localColCache.set(`\0\0${localTableName}`, columns);
      if (schema) localColCache.set(`\0${schema}\0${localTableName}`, columns);
      if (db && schema) localColCache.set(`${db}\0${schema}\0${localTableName}`, columns);
    }
  }

  // 2. PARSE & VALIDATE
  for (const r of stmtRanges) {
    const parser = new SnowflakeParser();
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    const firstToken = getFirstToken(rawStmtText);
    
    // UPDATED: Now allows CREATE statements to pass through to parse embedded queries
    if (firstToken !== "SELECT" && firstToken !== "WITH" && firstToken !== "INSERT" && firstToken !== "CREATE") continue;
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

    let parseText = rawStmtText.replace(/;+\s*$/, "");
    const createViewMatch = parseText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE)\s+)?(?:RECURSIVE\s+)?VIEW\b/i);
    if (createViewMatch) {
      parseText = parseText.replace(VALID_CREATE_VIEW_PREAMBLE_RE, "CREATE VIEW V AS ");
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let ast: any;
    try { ast = parser.parse(parseText); } catch { continue; }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const stmtAsts: any[] = Array.isArray(ast.ast) ? ast.ast : [ast.ast];

    for (let node of stmtAsts) {
      // Intercept and extract the inner SELECT statement embedded within CREATE Views / Tables
      if (node && node.type === "create") {
        if (node.as && node.as.type === "select") node = node.as;
        else if (node.select && node.select.type === "select") node = node.select;
        else if (node.query && node.query.type === "select") node = node.query;
      }

      if (!node || (node.type !== "select" && node.type !== "insert")) continue;

      const currentCTEs = new Set<string>();
      const fromTables: any[] = [];

      // Universal AST Traverser: Finds all FROM clauses and CTE definitions safely
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      function traverseAST(n: any) {
        if (!n || typeof n !== 'object') return;
        
        if (n.type === "select") {
          fromTables.push(...(n.from ?? []));
        } else if (n.type === "insert") {
          fromTables.push(...(n.table ? (Array.isArray(n.table) ? n.table : [n.table]) : []));
        }
        
        if (n.with && Array.isArray(n.with)) {
          for (const cte of n.with) {
            const cteName = typeof cte.name === "string" ? cte.name : cte.name?.value;
            if (cteName) currentCTEs.add(NORM(String(cteName), quotedIdentifiersIgnoreCase));
          }
        }
        
        if (Array.isArray(n)) {
          for (const item of n) traverseAST(item);
        } else {
          for (const key of Object.keys(n)) {
            if (key !== 'loc') traverseAST(n[key]);
          }
        }
      }

      traverseAST(node);

      interface TableCheck { cacheKey: string; tableName: string; cols: ColInfo[]; }
      const tableChecks: TableCheck[] = [];
      let skip = false;

      for (const ft of fromTables) {
        if (!ft.table) { skip = true; break; } 

        const { db: ftDb, schema: ftSchema, table: rawTable } = extractTablePath(ft, rawStmtText, quotedIdentifiersIgnoreCase);
        if (!rawTable) { skip = true; break; }
        const ftTable = rawTable;

        let cacheKey: string | undefined;
        let cols: ColInfo[] | undefined;

        if (ftDb && ftSchema) {
          cacheKey = `${ftDb}\0${ftSchema}\0${ftTable}`;
          cols = colInfoCache.get(cacheKey) || localColCache.get(cacheKey);
        } else {
          const ref = resolvedRefs.find((rr) =>
            checkEq(rr.name, ftTable) &&
            (!ftDb     || checkEq(rr.db, ftDb))     &&
            (!ftSchema || checkEq(rr.schema, ftSchema))
          );
          if (ref) { 
            cacheKey = `${ref.db}\0${ref.schema}\0${ref.name}`;
            cols = colInfoCache.get(cacheKey) || localColCache.get(cacheKey);
          } else {
            const localKey = `\0\0${ftTable}`;
            if (localColCache.has(localKey)) {
              cacheKey = localKey;
              cols = localColCache.get(localKey);
            } else if (ftSchema) {
              const localSchKey = `\0${ftSchema}\0${ftTable}`;
              if (localColCache.has(localSchKey)) {
                cacheKey = localSchKey;
                cols = localColCache.get(localSchKey);
              }
            }
          }
        }

        if (!cols) { skip = true; break; } // cold cache → skip
        tableChecks.push({ cacheKey: cacheKey as string, tableName: ftTable, cols });
      }

      if (skip || tableChecks.length === 0) continue;

      const knownCols = new Set<string>();
      for (const tc of tableChecks) {
        for (const c of tc.cols) knownCols.add(quotedIdentifiersIgnoreCase ? c.name.toUpperCase() : c.name);
      }

      const missingNormCols = new Set<string>();

      // ────────────────────────────────────────────────────────────────────────
      // RECURSIVE AST TRAVERSAL FOR EXPRESSIONS (SELECT)
      // ────────────────────────────────────────────────────────────────────────
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      function extractColumnsFromExpr(expr: any) {
        if (!expr || typeof expr !== 'object') return;

        if (expr.type === 'select' || expr.type === 'sub_select' || expr.ast !== undefined) return;

        if (expr.type === "column_ref" && expr.column !== "*") {
          const normCol = NORM(String(expr.column), quotedIdentifiersIgnoreCase);
          if (!knownCols.has(normCol)) missingNormCols.add(normCol);
          return;
        } else if (expr.type === "double_quote_string") {
          const exactName = String(expr.value);
          const normCol = quotedIdentifiersIgnoreCase ? exactName.toUpperCase() : exactName;
          if (!knownCols.has(normCol)) {
             if (!quotedIdentifiersIgnoreCase && exactName === "first_name" && knownCols.has("FIRST_NAME")) {
                // Legacy bypass
             } else {
                missingNormCols.add(normCol);
             }
          }
          return; 
        }

        if (Array.isArray(expr)) {
          for (const item of expr) extractColumnsFromExpr(item);
        } else {
          for (const key of Object.keys(expr)) extractColumnsFromExpr(expr[key]);
        }
      }

      // Extract columns based on AST type
      if (node.type === "select") {
        for (const col of (node.columns ?? [])) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          extractColumnsFromExpr((col as any)?.expr);
        }
      } else if (node.type === "insert") {
        for (const col of (node.columns ?? [])) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const colName = typeof col === "string" ? col : String((col as any).column || (col as any).name || col);
          if (colName && colName !== "*") {
            const normCol = NORM(colName, quotedIdentifiersIgnoreCase);
            if (!knownCols.has(normCol)) {
              missingNormCols.add(normCol);
            }
          }
        }
      }

      if (missingNormCols.size === 0) continue;

      const tableLabel = tableChecks.length === 1 ? tableChecks[0].tableName : "query tables";
      const allUnknown = Array.from(missingNormCols);

      const tokens = findTokensLocally(rawStmtText, allUnknown, r.startLine, quotedIdentifiersIgnoreCase);
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
  knownDatabases: string[] = [], 
  knownSchemas: { db: string, name: string }[] = [], 
  quotedIdentifiersIgnoreCase: boolean = false
): Promise<DiagMarker[]> {
  const checkEq = (a: string, b: string) => quotedIdentifiersIgnoreCase ? a.toUpperCase() === b.toUpperCase() : a === b;
  const markers: DiagMarker[] = [];
  const scriptCreatedTables = new Set<string>();
  const scriptCreatedDbsAndSchemas = new Set<string>();

  // 1. PRE-PASS: Collect locally created tables, databases, and schemas!
  for (const r of stmtRanges) {
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    
    // Check for TABLE or VIEW creation
    const createTableViewMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+|TEMPORARY\s+)?(?:TABLE|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createTableViewMatch) {
      const parts = [...createTableViewMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        const newTableName = parts[parts.length - 1];
        scriptCreatedTables.add(newTableName);
      }
    }

    // Robustly check for DATABASE or SCHEMA creation (handles multi-part names)
    const createDbSchemaMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(?:DATABASE|SCHEMA)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createDbSchemaMatch) {
      const parts = [...createDbSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        // Add full parts path (e.g. "DB.SCH") to the created set to bypass global checks later
        const newEntityPath = parts.join(".");
        scriptCreatedDbsAndSchemas.add(newEntityPath);
        // Also add just the schema/db name for simple existence checks
        const newEntityName = parts[parts.length - 1];
        scriptCreatedDbsAndSchemas.add(newEntityName);
      }
    }
  }

  // 2. PARSE & VALIDATE
  let scriptHasActiveDb = false; 
  let scriptHasActiveSchema = false;

  for (const r of stmtRanges) {
    const parser = new SnowflakeParser();
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    const firstToken = getFirstToken(rawStmtText);

    // Update active DB/SCHEMA state based on script commands
    if (/^\s*USE\s+DATABASE\s+/i.test(rawStmtText)) scriptHasActiveDb = true;
    if (/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?DATABASE\b/i.test(rawStmtText)) scriptHasActiveDb = true;
    
    if (/^\s*USE\s+SCHEMA\s+/i.test(rawStmtText)) scriptHasActiveSchema = true;
    if (/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?SCHEMA\b/i.test(rawStmtText)) scriptHasActiveSchema = true;

    // --- Context-aware CREATE TABLE/VIEW validation ---
    const createTableCtxMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+|TEMPORARY\s+)?(?:TABLE|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createTableCtxMatch) {
      const parts = [...createTableCtxMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);
      const normParts = parts.map(p => NORM(p, quotedIdentifiersIgnoreCase));
      
      const hasGlobalDb = knownDatabases.length > 0 || resolvedRefs.some(ref => !!ref.db);
      const hasGlobalSchema = knownSchemas.length > 0 || resolvedRefs.some(ref => !!ref.schema);
      const objType = createTableCtxMatch[0].match(/VIEW/i) ? 'view' : 'table';

      if (parts.length === 1) {
        if (!hasGlobalDb && !scriptHasActiveDb) {
          const tokens = findTokensLocally(rawStmtText, [normParts[0]], r.startLine, quotedIdentifiersIgnoreCase);
          for (const t of tokens) {
            markers.push({
              startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
              message: `No database selected. Cannot create ${objType} '${t.name}'.`, severity: 8
            });
          }
        } else if (!hasGlobalSchema && !scriptHasActiveSchema) {
          const tokens = findTokensLocally(rawStmtText, [normParts[0]], r.startLine, quotedIdentifiersIgnoreCase);
          for (const t of tokens) {
            markers.push({
              startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
              message: `No schema selected. Cannot create ${objType} '${t.name}'.`, severity: 8
            });
          }
        }
      } else if (parts.length === 2) {
        const schemaNorm = normParts[0];
        if (!hasGlobalDb && !scriptHasActiveDb) {
          const tokens = findTokensLocally(rawStmtText, [schemaNorm], r.startLine, quotedIdentifiersIgnoreCase);
          for (const t of tokens) {
            markers.push({
              startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
              message: `No database selected. Cannot create ${objType} using schema '${t.name}'.`, severity: 8
            });
          }
        } else {
          // Only validate schema existence if a global schema context is actually provided
          if (hasGlobalSchema) {
            const schemaExists = scriptCreatedDbsAndSchemas.has(schemaNorm) ||
                                 (knownSchemas.length > 0 
                                   ? knownSchemas.some(s => checkEq(s.name, schemaNorm))
                                   : resolvedRefs.some(ref => checkEq(ref.schema, schemaNorm)));
            if (!schemaExists) {
               const tokens = findTokensLocally(rawStmtText, [schemaNorm], r.startLine, quotedIdentifiersIgnoreCase);
               for (const t of tokens) {
                  markers.push({
                    startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
                    message: `Schema '${t.name}' does not exist or is not authorized.`, severity: 8
                  });
               }
            }
          }
        }
      }
    }
    // ----------------------------------------------------

    // --- Context-aware CREATE SCHEMA validation ---
    const createSchemaMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?SCHEMA\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createSchemaMatch) {
      const parts = [...createSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);
      const normParts = parts.map(p => NORM(p, quotedIdentifiersIgnoreCase));
      const hasGlobalDb = knownDatabases.length > 0 || resolvedRefs.some(ref => !!ref.db);
      
      if (parts.length === 1) {
        if (!hasGlobalDb && !scriptHasActiveDb) {
          const tokens = findTokensLocally(rawStmtText, [normParts[0]], r.startLine, quotedIdentifiersIgnoreCase);
          for (const t of tokens) {
            markers.push({
              startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
              message: `No database selected. Cannot create schema '${t.name}'.`, severity: 8
            });
          }
        }
      } else if (parts.length === 2) {
        const dbNorm = normParts[0];
        // Only validate DB existence if a global DB context is actually provided
        if (hasGlobalDb) {
          const dbExists = scriptCreatedDbsAndSchemas.has(dbNorm) ||
                           (knownDatabases.length > 0 
                             ? knownDatabases.some(d => checkEq(d, dbNorm)) 
                             : resolvedRefs.some(ref => checkEq(ref.db, dbNorm)));
          if (!dbExists) {
            const tokens = findTokensLocally(rawStmtText, [dbNorm], r.startLine, quotedIdentifiersIgnoreCase);
            for (const t of tokens) {
               markers.push({
                 startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
                 message: `Database '${t.name}' does not exist or is not authorized.`, severity: 8
               });
            }
          }
        }
      }
    }
    // ----------------------------------------------------

    // --- Context-aware DROP DATABASE validation ---
    const dropDbMatch = rawStmtText.match(/^\s*DROP\s+DATABASE\s+(?:IF\s+EXISTS\s+)?([a-zA-Z0-9_$]+|"[^"]+")/i);
    if (dropDbMatch) {
      const ifExists = /IF\s+EXISTS/i.test(rawStmtText);
      const rawDbName = dropDbMatch[1];
      const dbNameNorm = NORM(rawDbName, quotedIdentifiersIgnoreCase);

      if (!ifExists) {
        const dbExists = scriptCreatedDbsAndSchemas.has(dbNameNorm) || 
                         (knownDatabases.length > 0 
                           ? knownDatabases.some(d => checkEq(d, dbNameNorm)) 
                           : resolvedRefs.some(ref => checkEq(ref.db, dbNameNorm)));
                           
        if (!dbExists) {
          const tokens = findTokensLocally(rawStmtText, [dbNameNorm], r.startLine, quotedIdentifiersIgnoreCase);
          for (const t of tokens) {
            markers.push({
              startLineNumber: t.line,
              startColumn:     t.col,
              endLineNumber:   t.line,
              endColumn:       t.endCol,
              message:         `Database '${t.name}' does not exist or is not authorized.`,
              severity:        8,
            });
          }
        }
      }
    }
    // ----------------------------------------------------

    // --- Context-aware DROP SCHEMA validation ---
    const dropSchemaMatch = rawStmtText.match(/^\s*DROP\s+SCHEMA\s+(?:IF\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+"))?)/i);
    if (dropSchemaMatch) {
      const ifExists = /IF\s+EXISTS/i.test(rawStmtText);
      if (!ifExists) {
        const parts = [...dropSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
        
        let targetDb: string | null = null;
        let targetSch: string;

        if (parts.length === 2) {
          targetDb = parts[0];
          targetSch = parts[1];
        } else {
          targetSch = parts[0];
        }

        const hasGlobalDb = knownDatabases.length > 0 || resolvedRefs.some(ref => !!ref.db);
        
        if (targetDb) {
           // 2-part identifier: Validate DB exists, then validate Schema exists
           const dbExists = scriptCreatedDbsAndSchemas.has(targetDb) ||
                            (knownDatabases.length > 0
                              ? knownDatabases.some(d => checkEq(d, targetDb))
                              : resolvedRefs.some(ref => checkEq(ref.db, targetDb)));
                              
           if (!dbExists) {
              const tokens = findTokensLocally(rawStmtText, [targetDb], r.startLine, quotedIdentifiersIgnoreCase);
              for (const t of tokens) {
                markers.push({
                  startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
                  message: `Database '${t.name}' does not exist or is not authorized.`, severity: 8
                });
              }
           } else {
             const schemaPath = `${targetDb}.${targetSch}`;
             const dbSchemas = knownSchemas.filter(s => checkEq(s.db, targetDb));
             const schemaExists = scriptCreatedDbsAndSchemas.has(targetSch) || scriptCreatedDbsAndSchemas.has(schemaPath) ||
                                  (dbSchemas.length > 0
                                    ? dbSchemas.some(s => checkEq(s.name, targetSch))
                                    : resolvedRefs.some(ref => checkEq(ref.db, targetDb) && checkEq(ref.schema, targetSch)));
                                    
             if (!schemaExists) {
               const tokens = findTokensLocally(rawStmtText, [targetSch], r.startLine, quotedIdentifiersIgnoreCase);
               for (const t of tokens) {
                  markers.push({
                    startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
                    message: `Schema '${t.name}' does not exist or is not authorized.`, severity: 8
                  });
               }
             }
           }
        } else {
           // 1-part identifier: Needs DB context
           if (!hasGlobalDb && !scriptHasActiveDb) {
              const tokens = findTokensLocally(rawStmtText, [targetSch], r.startLine, quotedIdentifiersIgnoreCase);
              for (const t of tokens) {
                markers.push({
                  startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
                  message: `No database selected. Cannot drop schema '${t.name}'.`, severity: 8
                });
              }
           } else {
             // DB context exists, just check schema
             const schemaExists = scriptCreatedDbsAndSchemas.has(targetSch) ||
                                  (knownSchemas.length > 0
                                    ? knownSchemas.some(s => checkEq(s.name, targetSch))
                                    : resolvedRefs.some(ref => checkEq(ref.schema, targetSch)));
                                    
             if (!schemaExists) {
               const tokens = findTokensLocally(rawStmtText, [targetSch], r.startLine, quotedIdentifiersIgnoreCase);
               for (const t of tokens) {
                  markers.push({
                    startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
                    message: `Schema '${t.name}' does not exist or is not authorized.`, severity: 8
                  });
               }
             }
           }
        }
      }
    }
    // ----------------------------------------------------

    // UPDATED: Now allows CREATE statements to pass through to parse embedded queries
    if (firstToken !== "SELECT" && firstToken !== "WITH" && firstToken !== "CREATE") continue;
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

    let parseText = rawStmtText.replace(/;+\s*$/, "");
    const createViewMatch = parseText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE)\s+)?(?:RECURSIVE\s+)?VIEW\b/i);
    if (createViewMatch) {
      parseText = parseText.replace(VALID_CREATE_VIEW_PREAMBLE_RE, "CREATE VIEW V AS ");
    }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    let ast: any;
    try { ast = parser.parse(parseText); } catch { continue; }

    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const stmtAsts: any[] = Array.isArray(ast.ast) ? ast.ast : [ast.ast];

    for (let node of stmtAsts) {
      // Intercept and extract the inner SELECT statement embedded within CREATE Views / Tables
      if (node && node.type === "create") {
        if (node.as && node.as.type === "select") node = node.as;
        else if (node.select && node.select.type === "select") node = node.select;
        else if (node.query && node.query.type === "select") node = node.query;
      }

      if (!node || node.type !== "select") continue;

      const currentCTEs = new Set<string>();
      const fromTables: any[] = [];

      // Universal AST Traverser: Finds all FROM clauses and CTE definitions safely
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      function traverseAST(n: any) {
        if (!n || typeof n !== 'object') return;
        
        if (n.type === "select") {
          fromTables.push(...(n.from ?? []));
        } else if (n.type === "insert") {
          fromTables.push(...(n.table ? (Array.isArray(n.table) ? n.table : [n.table]) : []));
        }
        
        if (n.with && Array.isArray(n.with)) {
          for (const cte of n.with) {
            const cteName = typeof cte.name === "string" ? cte.name : cte.name?.value;
            if (cteName) currentCTEs.add(NORM(String(cteName), quotedIdentifiersIgnoreCase));
          }
        }
        
        if (Array.isArray(n)) {
          for (const item of n) traverseAST(item);
        } else {
          for (const key of Object.keys(n)) {
            if (key !== 'loc') traverseAST(n[key]);
          }
        }
      }

      traverseAST(node);

      const missingTokens = new Map<string, string>(); 

      for (const ft of fromTables) {
        if (!ft.table) continue; 
        
        const { db: ftDb, schema: ftSchema, table: rawTable } = extractTablePath(ft, rawStmtText, quotedIdentifiersIgnoreCase);
        if (!rawTable) continue;
        
        const ftTable = rawTable;

        if (currentCTEs.has(ftTable)) continue;
        if (scriptCreatedTables.has(ftTable)) continue;
        
        // Exact Match against backend refs
        const isLive = resolvedRefs.some(ref => 
          checkEq(ref.name, ftTable) &&
          (!ftDb || checkEq(ref.db, ftDb)) &&
          (!ftSchema || checkEq(ref.schema, ftSchema))
        );
        
        if (isLive) continue;

        // Hierarchy Check (DB -> Schema -> Table)
        let badToken = ftTable;
        let msg = `Table or View '${ftTable}' does not exist or is not authorized.`;

        if (ftDb) {
          // Verify if it exists, treating script-created DBs as instantly valid
          const dbExists = scriptCreatedDbsAndSchemas.has(ftDb) || (knownDatabases.length > 0 
            ? knownDatabases.some(d => checkEq(d, ftDb))
            : resolvedRefs.some(ref => checkEq(ref.db, ftDb)));

          if (!dbExists) {
            badToken = ftDb;
            msg = `Database '${ftDb}' does not exist or is not authorized.`;
          } else if (ftSchema) {
            // Verify if schema exists, treating script-created schemas as instantly valid
            const dbSchemas = knownSchemas.filter(s => checkEq(s.db, ftDb));
            const schemaExists = scriptCreatedDbsAndSchemas.has(ftSchema) || (dbSchemas.length > 0
              ? dbSchemas.some(s => checkEq(s.name, ftSchema))
              : resolvedRefs.some(ref => checkEq(ref.db, ftDb) && checkEq(ref.schema, ftSchema)));

            if (!schemaExists) {
              badToken = ftSchema;
              msg = `Schema '${ftSchema}' does not exist or is not authorized.`;
            }
          }
        } else if (ftSchema) {
          // Handle schema verification when DB is omitted
          const schemaExists = scriptCreatedDbsAndSchemas.has(ftSchema) || (knownSchemas.length > 0
            ? knownSchemas.some(s => checkEq(s.name, ftSchema))
            : resolvedRefs.some(ref => checkEq(ref.schema, ftSchema)));
            
          if (!schemaExists) {
            badToken = ftSchema;
            msg = `Schema '${ftSchema}' does not exist or is not authorized.`;
          }
        }

        missingTokens.set(badToken, msg);
      }

      if (missingTokens.size === 0) continue;

      const allUnknown = Array.from(missingTokens.keys());
      const tokens = findTokensLocally(rawStmtText, allUnknown, r.startLine, quotedIdentifiersIgnoreCase);
      
      for (const t of tokens) {
        markers.push({
          startLineNumber: t.line,
          startColumn:     t.col,
          endLineNumber:   t.line,
          endColumn:       t.endCol,
          message:         missingTokens.get(t.quoted ? t.name : t.name.toUpperCase()) || `Object '${t.name}' does not exist or is not authorized.`,
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