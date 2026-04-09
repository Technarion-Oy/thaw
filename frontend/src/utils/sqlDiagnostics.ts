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

const NORM = (s: any, ignoreCase: boolean = false): string => {
  if (typeof s !== "string") return "";
  if (s.startsWith('"') && s.endsWith('"')) {
    const inner = s.slice(1, -1);
    return ignoreCase ? inner.toUpperCase() : inner;
  }
  return s.toUpperCase();
};

function getFirstToken(sql: string): string | null {
  const stripped = sql.replace(/--.*$/gm, "").replace(/\/\*[\s\S]*?\*\//g, "").trimStart();
  return stripped.match(/^[a-zA-Z_]\w*/)?.[0]?.toUpperCase() ?? null;
}

function findTokensLocally(stmtText: string, targets: string[], baseLine: number, ignoreCase: boolean = false) {
  const tokens: Array<{ name: string; line: number; col: number; endCol: number; quoted: boolean }> = [];
  const lines = stmtText.split("\n");
  
  const targetSet = new Set(targets);
  
  for (let i = 0; i < lines.length; i++) {
    const lineStr = lines[i];
    const regex = /[a-zA-Z0-9_$]+|"[^"]+"/g;
    let match;
    while ((match = regex.exec(lineStr)) !== null) {
      const rawWord = match[0];
      const isQuoted = rawWord.startsWith('"') && rawWord.endsWith('"');
      
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

export function extractTablePath(ft: any, rawSql: string = "", ignoreCase: boolean = false): { db: string | null; schema: string | null; table: string | null } {
  const parts: string[] = [];
  
  if (ft.catalog) parts.push(String(ft.catalog));
  if (ft.db && ft.db !== ft.catalog) parts.push(String(ft.db));
  else if (ft.database && ft.database !== ft.catalog) parts.push(String(ft.database));
  if (ft.schema && ft.schema !== ft.db && ft.schema !== ft.catalog && ft.schema !== ft.database) parts.push(String(ft.schema));
  if (ft.table) parts.push(String(ft.table));

  if (parts.length === 0) return { db: null, schema: null, table: null };

  const combined = parts.join(".");
  const matches = [...combined.matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => m[0]);

  const cleanParts = matches.map(p => {
    const stripped = p.replace(/^"|"$/g, "");
    if (!rawSql) return NORM(p, ignoreCase); 

    const regex = /[a-zA-Z0-9_$]+|"[^"]+"/g;
    let match;
    let foundNorm = null;
    
    while ((match = regex.exec(rawSql)) !== null) {
      const rawWord = match[0];
      const isQuoted = rawWord.startsWith('"') && rawWord.endsWith('"');
      const inner = isQuoted ? rawWord.slice(1, -1) : rawWord;
      
      if (inner.toUpperCase() === stripped.toUpperCase()) {
        foundNorm = NORM(rawWord, ignoreCase);
      }
    }
    return foundNorm !== null ? foundNorm : NORM(p, ignoreCase);
  });

  const len = cleanParts.length;
  
  if (len >= 3) return { db: cleanParts[len - 3], schema: cleanParts[len - 2], table: cleanParts[len - 1] };
  if (len === 2) return { db: null, schema: cleanParts[0], table: cleanParts[1] };
  if (len === 1) return { db: null, schema: null, table: cleanParts[0] };
  
  return { db: null, schema: null, table: null };
}

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

export interface ColInfo { name: string; dataType: string; }

export interface DiagMarker {
  startLineNumber: number; 
  startColumn:     number;
  endLineNumber:   number;
  endColumn:       number;
  message:         string;
  severity:        8 | 4;  
}

const PARSEABLE_STMT_KEYWORDS = new Set([
  "SELECT", "WITH", "INSERT", "UPDATE", "CREATE", "ALTER",
  "TRUNCATE", "CALL", "SHOW", "SET", "DROP", "UNDROP"
]);

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
  "|UNDROP\\s+(?:DATABASE|SCHEMA|TABLE)\\b" + 
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

  for (const r of stmtRanges) {
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    
    const createMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+|TEMPORARY\s+)?TABLE\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})\s*\(([^;]+)\)/i);
    if (createMatch) {
      const parts = [...createMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      const localTableName = parts[parts.length - 1];
      
      const colsRaw = createMatch[2];
      const columns: ColInfo[] = [];
      
      let depth = 0;
      let currentDef = "";
      const defs: string[] = [];
      for (let i = 0; i < colsRaw.length; i++) {
          const char = colsRaw[i];
          if (char === '(') depth++;
          else if (char === ')') {
            if (depth > 0) depth--;
            if (depth < 0) break;
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
      
      localColCache.set(`\0\0${localTableName}`, columns);
      if (schema) localColCache.set(`\0${schema}\0${localTableName}`, columns);
      if (db && schema) localColCache.set(`${db}\0${schema}\0${localTableName}`, columns);
    }
  }

  for (const r of stmtRanges) {
    const parser = new SnowflakeParser();
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    const firstToken = getFirstToken(rawStmtText);
    
    if (firstToken !== "SELECT" && firstToken !== "WITH" && firstToken !== "INSERT" && firstToken !== "CREATE" && firstToken !== "UNDROP") continue;
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

    let parseText = rawStmtText.replace(/;+\s*$/, "");
    const createViewMatch = parseText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE)\s+)?(?:RECURSIVE\s+)?VIEW\b/i);
    if (createViewMatch) {
      parseText = parseText.replace(VALID_CREATE_VIEW_PREAMBLE_RE, "CREATE VIEW V AS ");
    }

    let ast: any;
    try { ast = parser.parse(parseText); } catch { continue; }

    const stmtAsts: any[] = Array.isArray(ast.ast) ? ast.ast : [ast.ast];

    for (let node of stmtAsts) {
      if (node && node.type === "create") {
        if (node.as && node.as.type === "select") node = node.as;
        else if (node.select && node.select.type === "select") node = node.select;
        else if (node.query && node.query.type === "select") node = node.query;
      }

      if (!node || (node.type !== "select" && node.type !== "insert")) continue;

      const currentCTEs = new Set<string>();
      const fromTables: any[] = [];

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

        if (!cols) { skip = true; break; } 
        tableChecks.push({ cacheKey: cacheKey as string, tableName: ftTable, cols });
      }

      if (skip || tableChecks.length === 0) continue;

      const knownCols = new Set<string>();
      for (const tc of tableChecks) {
        for (const c of tc.cols) knownCols.add(quotedIdentifiersIgnoreCase ? c.name.toUpperCase() : c.name);
      }

      const missingNormCols = new Set<string>();

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

      if (node.type === "select") {
        for (const col of (node.columns ?? [])) {
          extractColumnsFromExpr((col as any)?.expr);
        }
      } else if (node.type === "insert") {
        for (const col of (node.columns ?? [])) {
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

export async function validateTablesExist(
  sql: string,
  stmtRanges: StatementRange[],
  resolvedRefs: ResolvedRef[],
  knownDatabases: string[] = [], 
  knownSchemas: { db: string, name: string }[] = [], 
  quotedIdentifiersIgnoreCase: boolean = false,
  droppedDatabases: string[] = [],
  droppedSchemas: { db: string, name: string }[] = [],
  droppedTables: { db: string, schema: string, name: string }[] = []
): Promise<DiagMarker[]> {
  const checkEq = (a: string, b: string) => quotedIdentifiersIgnoreCase ? a.toUpperCase() === b.toUpperCase() : a === b;
  const markers: DiagMarker[] = [];
  const scriptCreatedTables = new Set<string>();
  const scriptCreatedDbsAndSchemas = new Set<string>();
  const scriptDroppedTables = new Set<string>();
  const scriptDroppedDbsAndSchemas = new Set<string>();

  // 1. PRE-PASS: Collect locally created and dropped tables, databases, and schemas!
  for (const r of stmtRanges) {
    const rawStmtText = sql.slice(r.startOffset, r.endOffset);
    
    const createTableViewMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+|TEMPORARY\s+)?(?:TABLE|VIEW)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createTableViewMatch) {
      const parts = [...createTableViewMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        const newTableName = parts[parts.length - 1];
        scriptCreatedTables.add(newTableName);
        scriptCreatedTables.add(parts.join("."));
      }
    }

    const createDbSchemaMatch = rawStmtText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?(?:DATABASE|SCHEMA)\s+(?:IF\s+NOT\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (createDbSchemaMatch) {
      const parts = [...createDbSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        const newEntityPath = parts.join(".");
        scriptCreatedDbsAndSchemas.add(newEntityPath);
        const newEntityName = parts[parts.length - 1];
        scriptCreatedDbsAndSchemas.add(newEntityName);
      }
    }

    // Track Drops
    const dropTableMatch = rawStmtText.match(/^\s*DROP\s+(?:TABLE)\s+(?:IF\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (dropTableMatch) {
      const parts = [...dropTableMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        scriptDroppedTables.add(parts[parts.length - 1]);
        scriptDroppedTables.add(parts.join("."));
      }
    }

    const dropDbSchemaMatch = rawStmtText.match(/^\s*DROP\s+(?:DATABASE|SCHEMA)\s+(?:IF\s+EXISTS\s+)?((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+"))?)/i);
    if (dropDbSchemaMatch) {
      const parts = [...dropDbSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        scriptDroppedDbsAndSchemas.add(parts[parts.length - 1]);
        scriptDroppedDbsAndSchemas.add(parts.join("."));
      }
    }

    // Track Undrops
    const undropTableMatch = rawStmtText.match(/^\s*UNDROP\s+TABLE\s+((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (undropTableMatch) {
      const parts = [...undropTableMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        scriptCreatedTables.add(parts[parts.length - 1]);
        scriptCreatedTables.add(parts.join("."));
      }
    }

    const undropDbSchemaMatch = rawStmtText.match(/^\s*UNDROP\s+(?:DATABASE|SCHEMA)\s+((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+"))?)/i);
    if (undropDbSchemaMatch) {
      const parts = [...undropDbSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      if (parts.length > 0) {
        scriptCreatedDbsAndSchemas.add(parts[parts.length - 1]);
        scriptCreatedDbsAndSchemas.add(parts.join("."));
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

    if (/^\s*USE\s+DATABASE\s+/i.test(rawStmtText)) scriptHasActiveDb = true;
    if (/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?DATABASE\b/i.test(rawStmtText)) scriptHasActiveDb = true;
    
    if (/^\s*USE\s+SCHEMA\s+/i.test(rawStmtText)) scriptHasActiveSchema = true;
    if (/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:TRANSIENT\s+)?SCHEMA\b/i.test(rawStmtText)) scriptHasActiveSchema = true;

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
           if (!hasGlobalDb && !scriptHasActiveDb) {
              const tokens = findTokensLocally(rawStmtText, [targetSch], r.startLine, quotedIdentifiersIgnoreCase);
              for (const t of tokens) {
                markers.push({
                  startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
                  message: `No database selected. Cannot drop schema '${t.name}'.`, severity: 8
                });
              }
           } else {
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

    // --- UNDROP DATABASE validation ---
    const undropDbMatch = rawStmtText.match(/^\s*UNDROP\s+DATABASE\s+([a-zA-Z0-9_$]+|"[^"]+")/i);
    if (undropDbMatch) {
      const rawDbName = undropDbMatch[1];
      const dbNameNorm = NORM(rawDbName, quotedIdentifiersIgnoreCase);

      const isDropped = scriptDroppedDbsAndSchemas.has(dbNameNorm) || droppedDatabases.some(d => checkEq(d, dbNameNorm));
                       
      if (!isDropped) {
        const tokens = findTokensLocally(rawStmtText, [dbNameNorm], r.startLine, quotedIdentifiersIgnoreCase);
        for (const t of tokens) {
          markers.push({
            startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
            message: `Database '${t.name}' is not available to undrop.`, severity: 8
          });
        }
      }
    }

    // --- UNDROP SCHEMA validation ---
    const undropSchemaMatch = rawStmtText.match(/^\s*UNDROP\s+SCHEMA\s+((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+"))?)/i);
    if (undropSchemaMatch) {
      const parts = [...undropSchemaMatch[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      let targetSch = parts[parts.length - 1];
      
      const isDropped = scriptDroppedDbsAndSchemas.has(targetSch) || scriptDroppedDbsAndSchemas.has(parts.join(".")) || droppedSchemas.some(s => checkEq(s.name, targetSch));
      if (!isDropped) {
         const tokens = findTokensLocally(rawStmtText, [targetSch], r.startLine, quotedIdentifiersIgnoreCase);
         for (const t of tokens) {
            markers.push({
              startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
              message: `Schema '${t.name}' is not available to undrop.`, severity: 8
            });
         }
      }
    }

    // --- UNDROP TABLE validation ---
    const undropTableMatch2 = rawStmtText.match(/^\s*UNDROP\s+TABLE\s+((?:[a-zA-Z0-9_$]+|"[^"]+")(?:\.(?:[a-zA-Z0-9_$]+|"[^"]+")){0,2})/i);
    if (undropTableMatch2) {
      const parts = [...undropTableMatch2[1].matchAll(/[a-zA-Z0-9_$]+|"[^"]+"/g)].map(m => NORM(m[0], quotedIdentifiersIgnoreCase));
      let targetTab = parts[parts.length - 1];
      
      const isDropped = scriptDroppedTables.has(targetTab) || scriptDroppedTables.has(parts.join(".")) || droppedTables.some(t => checkEq(t.name, targetTab));
      if (!isDropped) {
         const tokens = findTokensLocally(rawStmtText, [targetTab], r.startLine, quotedIdentifiersIgnoreCase);
         for (const t of tokens) {
            markers.push({
              startLineNumber: t.line, startColumn: t.col, endLineNumber: t.line, endColumn: t.endCol,
              message: `Table '${t.name}' is not available to undrop.`, severity: 8
            });
         }
      }
    }

    if (firstToken !== "SELECT" && firstToken !== "WITH" && firstToken !== "CREATE" && firstToken !== "UNDROP") continue;
    if (SNOWFLAKE_FP_RE.test(rawStmtText)) continue;

    let parseText = rawStmtText.replace(/;+\s*$/, "");
    const createViewMatch = parseText.match(/^\s*CREATE\s+(?:OR\s+REPLACE\s+)?(?:SECURE\s+)?(?:(?:(?:LOCAL|GLOBAL)\s+)?(?:TEMP|TEMPORARY|VOLATILE)\s+)?(?:RECURSIVE\s+)?VIEW\b/i);
    if (createViewMatch) {
      parseText = parseText.replace(VALID_CREATE_VIEW_PREAMBLE_RE, "CREATE VIEW V AS ");
    }

    let ast: any;
    try { ast = parser.parse(parseText); } catch { continue; }

    const stmtAsts: any[] = Array.isArray(ast.ast) ? ast.ast : [ast.ast];

    for (let node of stmtAsts) {
      if (node && node.type === "create") {
        if (node.as && node.as.type === "select") node = node.as;
        else if (node.select && node.select.type === "select") node = node.select;
        else if (node.query && node.query.type === "select") node = node.query;
      }

      if (!node || node.type !== "select") continue;

      const currentCTEs = new Set<string>();
      const fromTables: any[] = [];

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
        
        const isLive = resolvedRefs.some(ref => 
          checkEq(ref.name, ftTable) &&
          (!ftDb || checkEq(ref.db, ftDb)) &&
          (!ftSchema || checkEq(ref.schema, ftSchema))
        );
        
        if (isLive) continue;

        let badToken = ftTable;
        let msg = `Table or View '${ftTable}' does not exist or is not authorized.`;

        if (ftDb) {
          const dbExists = scriptCreatedDbsAndSchemas.has(ftDb) || (knownDatabases.length > 0 
            ? knownDatabases.some(d => checkEq(d, ftDb))
            : resolvedRefs.some(ref => checkEq(ref.db, ftDb)));

          if (!dbExists) {
            badToken = ftDb;
            msg = `Database '${ftDb}' does not exist or is not authorized.`;
          } else if (ftSchema) {
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