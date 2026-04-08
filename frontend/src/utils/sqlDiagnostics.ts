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

// Safe Uppercase: Prevents crashes if the backend sends null/undefined db or schema refs
const UC = (s: any) => (typeof s === "string" ? s.toUpperCase() : "");

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

// Surgically precise AST table path extractor
// Ensures no properties are swallowed or redundantly aliased by the parser
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function extractTablePath(ft: any): { db: string | null; schema: string | null; table: string | null } {
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

  // 2. Re-combine and cleanly extract exactly the valid identifiers
  const combined = parts.join(".");
  const matches = [...combined.matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);
  const clean = (s: string | undefined) => s ? s.replace(/^"|"$/g, "") : null;

  const len = matches.length;
  
  // 3. Extract purely by index position (Right-to-Left) to ignore parser aliasing quirks
  if (len >= 3) {
    return {
      db: clean(matches[len - 3]),
      schema: clean(matches[len - 2]),
      table: clean(matches[len - 1]),
    };
  } else if (len === 2) {
    return {
      db: null,
      schema: clean(matches[0]),
      table: clean(matches[1]),
    };
  } else if (len === 1) {
    return {
      db: null,
      schema: null,
      table: clean(matches[0]),
    };
  }

  return { db: null, schema: null, table: null };
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
  "TRUNCATE", "CALL", "SHOW", "SET", "DROP", // ADDED: "DROP" so the parser handles it!
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
  "|DROP\\s+(?:TABLE|VIEW|TASK|STREAM|STAGE|PIPE|PROCEDURE|FUNCTION|WAREHOUSE|ROLE|SCHEMA)\\b" + // ADDED: Fallback skips for unsupported Drops
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

    const parseText = rawStmtText.replace(/;+\s*$/, "");

    // --- Custom Syntax Validator for CREATE DATABASE / SCHEMA ---
    const createDbSchemaMatch = parseText.match(/^CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(DATABASE|SCHEMA)\b/i);
    if (createDbSchemaMatch) {
      const createDbProps = [
        `CLONE\\s+(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}(?:\\s+(?:AT|BEFORE)\\s*\\(\\s*(?:TIMESTAMP|OFFSET|STATEMENT)\\s*=>\\s*[^)]+\\))?(?:\\s+IGNORE\\s+TABLES\\s+WITH\\s+INSUFFICIENT\\s+DATA\\s+RETENTION)?(?:\\s+IGNORE\\s+HYBRID\\s+TABLES)?`,
        `WITH\\s+MANAGED\\s+ACCESS`, // NEW for SCHEMA
        `(?:DATA_RETENTION_TIME_IN_DAYS|MAX_DATA_EXTENSION_TIME_IN_DAYS|ICEBERG_VERSION_DEFAULT)\\s*=\\s*\\d+`,
        `(?:ENABLE_ICEBERG_MERGE_ON_READ|REPLACE_INVALID_CHARACTERS|ENABLE_DATA_COMPACTION)\\s*=\\s*(?:TRUE|FALSE)`,
        `(?:EXTERNAL_VOLUME|CATALOG)\\s*=\\s*(?:[a-zA-Z0-9_$]+|"[^"]+")`,
        `DEFAULT_DDL_COLLATION\\s*=\\s*'(?:[^']|'')*'`,
        `STORAGE_SERIALIZATION_POLICY\\s*=\\s*(?:COMPATIBLE|OPTIMIZED)`,
        `CLASSIFICATION_PROFILE\\s*=\\s*'(?:[^']|'')*'`, // NEW for SCHEMA
        `COMMENT\\s*=\\s*'(?:[^']|'')*'`,
        `CATALOG_SYNC\\s*=\\s*'(?:[^']|'')*'`,
        `CATALOG_SYNC_NAMESPACE_MODE\\s*=\\s*(?:NEST|FLATTEN)`,
        `CATALOG_SYNC_NAMESPACE_FLATTEN_DELIMITER\\s*=\\s*'(?:[^']|'')*'`,
        `(?:WITH\\s+)?TAG\\s*\\([^)]+\\)`,
        `(?:WITH\\s+)?CONTACT\\s*\\([^)]+\\)`,
        `OBJECT_VISIBILITY\\s*=\\s*(?:PRIVILEGED|[a-zA-Z0-9_$]+|"[^"]+")`
      ].join("|");

      // Validates exactly against Snowflake's strict property spec
      const validCreateDbSchemaRe = new RegExp(
        `^CREATE\\s+(?:OR\\s+REPLACE\\s+)?(?:TRANSIENT\\s+)?(?:DATABASE|SCHEMA)\\s+(?:IF\\s+NOT\\s+EXISTS\\s+)?(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2}(?:\\s+(?:${createDbProps}))*\\s*$`,
        "i"
      );

      if (validCreateDbSchemaRe.test(parseText)) {
        continue; // Valid Snowflake syntax, safely bypass the PEG parser
      } else {
        // Invalid syntax! Throw a diagnostic marker immediately.
        markers.push({
          startLineNumber: r.startLine,
          startColumn: 1,
          endLineNumber: r.endLine,
          endColumn: 100, // Safe default fallback length
          message: `Unexpected syntax in CREATE ${createDbSchemaMatch[1].toUpperCase()} statement.`,
          severity: 4
        });
        continue; 
      }
    }

    // --- NEW: Custom Syntax Validator for DROP DATABASE ---
    const dropDbMatch = parseText.match(/^DROP\s+DATABASE\b/i);
    if (dropDbMatch) {
      // Rigidly matches DROP DATABASE [IF EXISTS] name [CASCADE|RESTRICT]
      const validDropDbRe = /^DROP\s+DATABASE\s+(?:IF\s+EXISTS\s+)?(?:[a-zA-Z0-9_$]+|"[^"]+")(?:\s+(?:CASCADE|RESTRICT))?\s*$/i;
      
      if (validDropDbRe.test(parseText)) {
        continue;
      } else {
        markers.push({
          startLineNumber: r.startLine,
          startColumn: 1,
          endLineNumber: r.endLine,
          endColumn: 100,
          message: `Unexpected syntax in DROP DATABASE statement.`,
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

        const { db: ftDb, schema: ftSchema, table: rawTable } = extractTablePath(ft);
        if (!rawTable) { skip = true; break; }
        const ftTable = rawTable;

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

      // ────────────────────────────────────────────────────────────────────────
      // RECURSIVE AST TRAVERSAL FOR EXPRESSIONS
      // ────────────────────────────────────────────────────────────────────────
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      function extractColumnsFromExpr(expr: any) {
        if (!expr || typeof expr !== 'object') return;

        // Skip traversing into nested subqueries to avoid cross-scope false positives
        if (expr.type === 'select' || expr.type === 'sub_select' || expr.ast !== undefined) return;

        if (expr.type === "column_ref" && expr.table === null && expr.column !== "*") {
          if (!knownCols.has(UC(expr.column as string))) unknownBare.add(expr.column as string);
          return; // No need to recurse inside a column_ref
        } else if (expr.type === "double_quote_string") {
          if (!knownCols.has(UC(expr.value as string))) unknownQuoted.add(expr.value as string);
          return; // No need to recurse inside a string literal
        }

        // Recursively walk Arrays and Objects
        if (Array.isArray(expr)) {
          for (const item of expr) extractColumnsFromExpr(item);
        } else {
          for (const key of Object.keys(expr)) extractColumnsFromExpr(expr[key]);
        }
      }

      for (const col of (node.columns ?? [])) {
        // eslint-disable-next-line @typescript-eslint/no-explicit-any
        extractColumnsFromExpr((col as any)?.expr);
      }
      // ────────────────────────────────────────────────────────────────────────

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
  knownDatabases: string[] = [], 
  knownSchemas: { db: string, name: string }[] = [], 
): Promise<DiagMarker[]> {
  const markers: DiagMarker[] = [];
  const scriptCreatedTables = new Set<string>();
  const scriptCreatedDbsAndSchemas = new Set<string>();

  // 1. PRE-PASS: Collect locally created tables, databases, and schemas!
  for (const r of stmtRanges) {
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    
    // Check for TABLE or VIEW creation
    const createTableViewMatch = rawStmtText.match(/^CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+|TEMPORARY\s+)?(?:TABLE|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createTableViewMatch) {
      const parts = [...createTableViewMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);
      if (parts.length > 0) {
        const newTableName = parts[parts.length - 1].replace(/^"|"$/g, "").toUpperCase();
        scriptCreatedTables.add(newTableName);
      }
    }

    // Robustly check for DATABASE or SCHEMA creation (handles multi-part names)
    const createDbSchemaMatch = rawStmtText.match(/^CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(?:DATABASE|SCHEMA)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createDbSchemaMatch) {
      const parts = [...createDbSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);
      if (parts.length > 0) {
        const newEntityName = parts[parts.length - 1].replace(/^"|"$/g, "").toUpperCase();
        scriptCreatedDbsAndSchemas.add(newEntityName);
      }
    }
  }

  // 2. PARSE & VALIDATE
  let scriptHasActiveDb = false; // Track session state across statements

  for (const r of stmtRanges) {
    const parser = new SnowflakeParser();
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    const firstToken = getFirstToken(rawStmtText);

    // Update active DB state based on script commands
    if (/^USE\s+DATABASE\s+/i.test(rawStmtText)) {
      scriptHasActiveDb = true;
    }
    // Creating a DB implicitly sets it as the active session database
    if (/^CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?DATABASE\b/i.test(rawStmtText)) {
      scriptHasActiveDb = true;
    }

    // --- Context-aware CREATE SCHEMA validation ---
    const createSchemaMatch = rawStmtText.match(/^CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?SCHEMA\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createSchemaMatch) {
      const parts = [...createSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);
      
      if (parts.length === 1) {
        // 1-part identifier requires a database context
        const hasGlobalDb = knownDatabases.length > 0 || resolvedRefs.some(ref => !!ref.db);
        
        if (!hasGlobalDb && !scriptHasActiveDb) {
          const tokens = findTokensLocally(rawStmtText, [parts[0]], r.startLine);
          for (const t of tokens) {
            markers.push({
              startLineNumber: t.line,
              startColumn:     t.col,
              endLineNumber:   t.line,
              endColumn:       t.endCol,
              message:         `No database selected. Cannot create schema '${t.name}'.`,
              severity:        8, // Fatal error
            });
          }
        }
      }
    }
    // ----------------------------------------------------

    // --- Context-aware DROP DATABASE validation ---
    const dropDbMatch = rawStmtText.match(/^DROP\s+DATABASE\s+(IF\s+EXISTS\s+)?([a-zA-Z0-9_$]+|"[^"]+")/i);
    if (dropDbMatch) {
      const ifExists = !!dropDbMatch[1];
      const dbName = dropDbMatch[2].replace(/^"|"$/g, "");
      const dbNameUC = UC(dbName);

      if (!ifExists) {
        const dbExists = scriptCreatedDbsAndSchemas.has(dbNameUC) || 
                         (knownDatabases.length > 0 
                           ? knownDatabases.some(d => UC(d) === dbNameUC) 
                           : resolvedRefs.some(ref => UC(ref.db) === dbNameUC));
                           
        if (!dbExists) {
          const tokens = findTokensLocally(rawStmtText, [dbName], r.startLine);
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

      const currentCTEs = new Set<string>();
      
      if (firstToken === "WITH" && node.with && Array.isArray(node.with)) {
        for (const cte of node.with) {
          const cteName = typeof cte.name === "string" ? cte.name : cte.name?.value;
          if (cteName) currentCTEs.add(UC(String(cteName)));
        }
      }

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      const fromTables: any[] = node.from ?? [];
      const missingTokens = new Map<string, string>(); 

      for (const ft of fromTables) {
        if (!ft.table) continue; 
        
        const { db: ftDb, schema: ftSchema, table: rawTable } = extractTablePath(ft);
        if (!rawTable) continue;
        
        const ftTable = rawTable;
        const ftTableUC = UC(ftTable);

        if (currentCTEs.has(ftTableUC)) continue;
        if (scriptCreatedTables.has(ftTableUC)) continue;
        
        // Exact Match
        const isLive = resolvedRefs.some(ref => 
          UC(ref.name) === ftTableUC &&
          (!ftDb || UC(ref.db) === UC(ftDb)) &&
          (!ftSchema || UC(ref.schema) === UC(ftSchema))
        );
        
        if (isLive) continue;

        // Hierarchy Check (DB -> Schema -> Table)
        let badToken = ftTable;
        let msg = `Table or View '${ftTable}' does not exist or is not authorized.`;

        if (ftDb) {
          // Verify if it exists, treating script-created DBs as instantly valid
          const dbExists = scriptCreatedDbsAndSchemas.has(UC(ftDb)) || (knownDatabases.length > 0 
            ? knownDatabases.some(d => UC(d) === UC(ftDb))
            : resolvedRefs.some(ref => UC(ref.db) === UC(ftDb)));

          if (!dbExists) {
            badToken = ftDb;
            msg = `Database '${ftDb}' does not exist or is not authorized.`;
          } else if (ftSchema) {
            // Verify if schema exists, treating script-created schemas as instantly valid
            const dbSchemas = knownSchemas.filter(s => UC(s.db) === UC(ftDb));
            const schemaExists = scriptCreatedDbsAndSchemas.has(UC(ftSchema)) || (dbSchemas.length > 0
              ? dbSchemas.some(s => UC(s.name) === UC(ftSchema))
              : resolvedRefs.some(ref => UC(ref.db) === UC(ftDb) && UC(ref.schema) === UC(ftSchema)));

            if (!schemaExists) {
              badToken = ftSchema;
              msg = `Schema '${ftSchema}' does not exist or is not authorized.`;
            }
          }
        } else if (ftSchema) {
          // Handle schema verification when DB is omitted
          const schemaExists = scriptCreatedDbsAndSchemas.has(UC(ftSchema)) || (knownSchemas.length > 0
            ? knownSchemas.some(s => UC(s.name) === UC(ftSchema))
            : resolvedRefs.some(ref => UC(ref.schema) === UC(ftSchema)));
            
          if (!schemaExists) {
            badToken = ftSchema;
            msg = `Schema '${ftSchema}' does not exist or is not authorized.`;
          }
        }

        missingTokens.set(UC(badToken), msg);
      }

      if (missingTokens.size === 0) continue;

      const allUnknown = Array.from(missingTokens.keys());
      const tokens = findTokensLocally(rawStmtText, allUnknown, r.startLine);
      
      for (const t of tokens) {
        markers.push({
          startLineNumber: t.line,
          startColumn:     t.col,
          endLineNumber:   t.line,
          endColumn:       t.endCol,
          message:         missingTokens.get(UC(t.name)) || `Object '${t.name}' does not exist or is not authorized.`,
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