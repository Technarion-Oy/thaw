// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import { useState, useRef, useCallback, useEffect } from "react";
import { Button } from "antd";
import Editor, { type BeforeMount, type OnMount } from "@monaco-editor/react";
import { ensureMonacoSetup } from "./monacoSetup";
import { setEditorInstance } from "./editorRef";
import { useQueryStore } from "../../store/queryStore";
import { useObjectStore } from "../../store/objectStore";
import { useSessionStore } from "../../store/sessionStore";
import { useThemeStore } from "../../store/themeStore";
import { ClipboardGetText, ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { GetObjectDDL, ListObjects, ListSchemas, GetTableColumns, GetTableForeignKeys, GetTableColumnsWithTypes, GetSchemaForeignKeys, GetUserDDL, GetAISuggestion, GetFunctionSuggestions, GetFunctionTooltip, GetAllFunctionNames } from "../../../wailsjs/go/main/App";

// Module-level DDL cache and hover provider handle so we only register once
// and don't accumulate duplicate providers on editor remounts.
const DDL_CACHE_TTL = 60_000; // ms — stale entries are re-fetched after this
const hoverDDLCache = new Map<string, { ddl: string; ts: number }>();
let hoverProviderDisposable: { dispose(): void } | null = null;
let inlineCompletionsDisposable: { dispose(): void } | null = null;

// Function name sets for decoration-based highlighting.
// Populated once from the local SQLite cache; shared across all editor instances.
const builtinFns = new Set<string>();
const udfFns     = new Set<string>();
let fnNamesLoaded = false;

// Track which db/schema pairs and databases have already been lazy-fetched by
// the completion provider so we don't fire duplicate requests.
const fetchedSchemaObjects   = new Set<string>(); // "DB\0SCHEMA"
const fetchedDatabaseSchemas = new Set<string>(); // "DB"

// Shared case-fold helper used by module-level cache functions.
const UC = (s: string) => s.toUpperCase();

// ── Column-level completion cache ─────────────────────────────────────────────
// Keyed "DB\0SCHEMA\0TABLE". Populated lazily on first dot-trigger.
const columnCache  = new Map<string, string[]>();
const fetchingCols = new Set<string>();

async function getColumns(db: string, schema: string, table: string): Promise<string[]> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  if (columnCache.has(key)) return columnCache.get(key)!;
  if (fetchingCols.has(key)) return [];
  fetchingCols.add(key);
  try {
    const cols = await GetTableColumns(db, schema, table);
    columnCache.set(key, cols ?? []);
    return cols ?? [];
  } catch {
    columnCache.set(key, []);
    return [];
  } finally {
    fetchingCols.delete(key);
  }
}

// ── FK cache for JOIN ON autocomplete ─────────────────────────────────────────
// Keyed "DB\0SCHEMA\0TABLE" → imported FKs (where the table is the child side).
interface FKEntry {
  pkDatabase: string; pkSchema: string; pkTable: string; pkColumn: string;
  fkColumn: string;
  constraintName: string;
  keySequence: number;
}
const fkCache    = new Map<string, FKEntry[]>();
const fetchingFKs = new Set<string>();

async function getFKs(db: string, schema: string, table: string): Promise<FKEntry[]> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  if (fkCache.has(key)) return fkCache.get(key)!;
  if (fetchingFKs.has(key)) return [];
  fetchingFKs.add(key);
  try {
    const fks = await GetTableForeignKeys(db, schema, table);
    const entries: FKEntry[] = (fks ?? []).map((fk: any) => ({
      pkDatabase:     fk.pkDatabase     ?? "",
      pkSchema:       fk.pkSchema       ?? "",
      pkTable:        fk.pkTable        ?? "",
      pkColumn:       fk.pkColumn       ?? "",
      fkColumn:       fk.fkColumn       ?? "",
      constraintName: fk.constraintName ?? "",
      keySequence:    fk.keySequence    ?? 0,
    }));
    fkCache.set(key, entries);
    return entries;
  } catch {
    fkCache.set(key, []);
    return [];
  } finally {
    fetchingFKs.delete(key);
  }
}

// ── ColInfo cache for type-compatible JOIN ON suggestions ─────────────────────
// Keyed "DB\0SCHEMA\0TABLE" → column info with data types.
interface ColInfo { name: string; dataType: string; }
const colInfoCache   = new Map<string, ColInfo[]>();
const fetchingColInfos = new Set<string>();

async function getColInfos(db: string, schema: string, table: string): Promise<ColInfo[]> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  if (colInfoCache.has(key)) return colInfoCache.get(key)!;
  if (fetchingColInfos.has(key)) return [];
  fetchingColInfos.add(key);
  try {
    const cols = await GetTableColumnsWithTypes(db, schema, table);
    const entries: ColInfo[] = (cols ?? []).map((c: any) => ({
      name:     c.name     ?? "",
      dataType: c.dataType ?? "",
    }));
    colInfoCache.set(key, entries);
    return entries;
  } catch {
    colInfoCache.set(key, []);
    return [];
  } finally {
    fetchingColInfos.delete(key);
  }
}

// ── Schema-level FK warm-up ────────────────────────────────────────────────────
// Bulk-fetch all FKs in a schema from INFORMATION_SCHEMA and populate fkCache.
const fetchedFKSchemas = new Set<string>(); // "DB\0SCHEMA"

async function warmUpFKsForSchema(db: string, schema: string): Promise<void> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}`;
  if (fetchedFKSchemas.has(key)) return;
  fetchedFKSchemas.add(key);
  try {
    const rows = await GetSchemaForeignKeys(db, schema);
    if (!rows) return;
    // Group by FK table and populate fkCache (don't overwrite existing per-table entries)
    const grouped = new Map<string, FKEntry[]>();
    for (const r of rows as any[]) {
      const k = `${UC(r.fkDatabase)}\0${UC(r.fkSchema)}\0${UC(r.fkTable)}`;
      if (!grouped.has(k)) grouped.set(k, []);
      grouped.get(k)!.push({
        pkDatabase:     r.pkDatabase     ?? "",
        pkSchema:       r.pkSchema       ?? "",
        pkTable:        r.pkTable        ?? "",
        pkColumn:       r.pkColumn       ?? "",
        fkColumn:       r.fkColumn       ?? "",
        constraintName: r.constraintName ?? "",
        keySequence:    r.keySequence    ?? 0,
      });
    }
    for (const [k, entries] of grouped) {
      if (!fkCache.has(k)) fkCache.set(k, entries);
    }
  } catch {
    fetchedFKSchemas.delete(key); // allow retry
  }
}

// ── JOIN table ref parser ──────────────────────────────────────────────────────
// Extracts all FROM/JOIN table references (with aliases) from the given SQL text.
interface JoinTableRef { db?: string; schema?: string; name: string; alias: string; }

const JOIN_STOP_KW = new Set([
  "ON","WHERE","SET","GROUP","ORDER","HAVING","LIMIT","UNION","EXCEPT",
  "INTERSECT","CROSS","INNER","LEFT","RIGHT","FULL","OUTER","NATURAL","JOIN",
  "SELECT","WITH","FROM",
]);

function parseJoinTables(sql: string): JoinTableRef[] {
  const ID_PAT = `(?:"[^"]+"|\\w+)`;
  // Use [ \t]+ (NOT \s+) for the alias separator so the alias group never crosses
  // a newline and accidentally consumes the JOIN keyword of the next clause.
  const tableRefRe = new RegExp(
    `(?:FROM|JOIN)\\s+(?:(${ID_PAT})\\.(${ID_PAT})\\.(${ID_PAT})|(${ID_PAT})\\.(${ID_PAT})|(${ID_PAT}))` +
    `(?:[ \\t]+(?:AS[ \\t]+)?(${ID_PAT}))?`,
    "gi",
  );
  // Snowflake normalises unquoted identifiers to UPPERCASE; quoted ones preserve case.
  // normId applies that rule to db/schema/table parts so API calls use the right case.
  // Aliases are user-defined tokens kept exactly as typed (stripQ only strips quotes).
  const normId = (s?: string) => {
    if (!s) return s;
    return s.startsWith('"') ? s.slice(1, -1) : s.toUpperCase();
  };
  const stripQ = (s?: string) => (s && s.startsWith('"') ? s.slice(1, -1) : s);
  const result: JoinTableRef[] = [];
  let m: RegExpExecArray | null;
  while ((m = tableRefRe.exec(sql)) !== null) {
    let db: string | undefined, schema: string | undefined, name: string;
    if (m[1] && m[2] && m[3]) {
      db = normId(m[1])!; schema = normId(m[2])!; name = normId(m[3])!;
    } else if (m[4] && m[5]) {
      schema = normId(m[4])!; name = normId(m[5])!;
    } else {
      name = normId(m[6])!;
    }
    const rawAlias = stripQ(m[7]);
    const alias = rawAlias && !JOIN_STOP_KW.has(rawAlias.toUpperCase()) ? rawAlias : name;
    result.push({ db, schema, name, alias });
  }
  return result;
}

function mkColSuggestions(cols: string[], range: any, monaco: any) {
  return cols.map((col) => ({
    label:      col,
    kind:       monaco.languages.CompletionItemKind.Field,
    insertText: col,
    detail:     "COLUMN",
    range,
  }));
}

// ── JOIN ON autocomplete helpers ──────────────────────────────────────────────

/** Build one Monaco completion item for a JOIN ON condition. */
function makeSugg(label: string, detail: string, sortText: string, range: any, monaco: any) {
  return {
    label,
    kind:       monaco.languages.CompletionItemKind.Operator,
    insertText: label,
    detail,
    sortText,
    range,
  };
}

/**
 * Group FKs by constraintName, sort each group by keySequence, and return one
 * condition string per constraint (multi-column → ANDed pairs).
 */
function buildCompositeConditions(
  fks: FKEntry[],
  fkAlias: string,
  pkAlias: string,
): string[] {
  const groups = new Map<string, FKEntry[]>();
  for (const fk of fks) {
    const k = fk.constraintName || fk.fkColumn;
    if (!groups.has(k)) groups.set(k, []);
    groups.get(k)!.push(fk);
  }
  return [...groups.values()].map((cols) => {
    cols.sort((a, b) => a.keySequence - b.keySequence);
    return cols
      .map((fk) => `${fkAlias}.${fk.fkColumn} = ${pkAlias}.${fk.pkColumn}`)
      .join(" AND ");
  });
}

/**
 * When no FK constraints exist, suggest join conditions using the naming
 * convention TABLE_B.TABLE_A_ID ↔ TABLE_A.ID (or TABLE_A.TABLE_BID).
 */
function pkHeuristicConditions(
  lastRef:  { alias: string; name: string },
  otherRef: { alias: string; name: string },
  lastCols: string[],
  otherCols: string[],
): string[] {
  const results: string[] = [];
  const ln = lastRef.name.toUpperCase();
  const on = otherRef.name.toUpperCase();

  for (const col of lastCols) {
    const uc = col.toUpperCase();
    if (uc === `${on}_ID` || uc === `${on}ID`) {
      const pkCol = otherCols.find((c) => c.toUpperCase() === "ID");
      if (pkCol) results.push(`${lastRef.alias}.${col} = ${otherRef.alias}.${pkCol}`);
    }
  }
  for (const col of otherCols) {
    const uc = col.toUpperCase();
    if (uc === `${ln}_ID` || uc === `${ln}ID`) {
      const pkCol = lastCols.find((c) => c.toUpperCase() === "ID");
      if (pkCol) results.push(`${otherRef.alias}.${col} = ${lastRef.alias}.${pkCol}`);
    }
  }
  return results;
}

/** Map a Snowflake data-type string to a broad category for compatibility checks. */
function typeCategory(dt: string): string {
  const t = dt.toUpperCase().replace(/\s*\(.*/, ""); // strip params
  if (/^(NUMBER|INT|INTEGER|FLOAT|DECIMAL|NUMERIC|BIGINT|SMALLINT|TINYINT|BYTEINT|DOUBLE|REAL)$/.test(t)) return "numeric";
  if (/^(VARCHAR|CHAR|STRING|TEXT|NCHAR|NVARCHAR|CHARACTER VARYING)$/.test(t)) return "text";
  if (/^(DATE|TIME|TIMESTAMP|DATETIME|TIMESTAMP_NTZ|TIMESTAMP_LTZ|TIMESTAMP_TZ)$/.test(t)) return "datetime";
  if (t === "BOOLEAN") return "boolean";
  if (/^(VARIANT|OBJECT|ARRAY)$/.test(t)) return "semi";
  return "other";
}

/** Resolve raw JoinTableRef list to fully-qualified refs via the object store. */
function resolveRefs(
  refs: JoinTableRef[],
  storeObjs: Array<{ db: string; schema: string; name: string; kind: string }>,
): Array<{ db: string; schema: string; name: string; alias: string }> | null {
  const resolved = refs.map((ref) => {
    if (ref.db && ref.schema) {
      return { db: ref.db, schema: ref.schema, name: ref.name, alias: ref.alias };
    }
    const obj = storeObjs.find((o) => {
      if (o.kind !== "TABLE" && o.kind !== "VIEW") return false;
      if (UC(o.name) !== UC(ref.name)) return false;
      if (ref.db     && UC(o.db)     !== UC(ref.db))     return false;
      if (ref.schema && UC(o.schema) !== UC(ref.schema)) return false;
      return true;
    });
    return obj ? { db: obj.db, schema: obj.schema, name: obj.name, alias: ref.alias } : null;
  }).filter(Boolean) as Array<{ db: string; schema: string; name: string; alias: string }>;
  return resolved.length >= 2 ? resolved : null;
}

const SNOWFLAKE_KEYWORDS = [
  "SELECT", "FROM", "WHERE", "JOIN", "LEFT", "RIGHT", "INNER", "OUTER",
  "GROUP BY", "ORDER BY", "HAVING", "LIMIT", "INSERT", "UPDATE", "DELETE",
  "CREATE", "ALTER", "DROP", "TABLE", "VIEW", "SCHEMA", "DATABASE",
  "WAREHOUSE", "ROLE", "GRANT", "REVOKE", "SHOW", "DESCRIBE", "USE",
  "WITH", "AS", "ON", "AND", "OR", "NOT", "IN", "IS", "NULL", "LIKE",
  "ILIKE", "BETWEEN", "CASE", "WHEN", "THEN", "ELSE", "END", "DISTINCT",
  "QUALIFY", "OVER", "PARTITION BY", "ROWS", "RANGE", "UNBOUNDED",
  "PRECEDING", "FOLLOWING", "CURRENT ROW", "FLATTEN", "LATERAL",
];

// Map Snowflake object kinds to Monaco completion item kinds.
function monacoKind(monaco: any, kind: string): number {
  const K = monaco.languages.CompletionItemKind;
  switch (kind) {
    case "TABLE":     return K.Class;
    case "VIEW":      return K.Interface;
    case "FUNCTION":  return K.Function;
    case "PROCEDURE": return K.Function;
    case "SEQUENCE":  return K.Constant;
    default:          return K.Value;
  }
}

interface DdlHover {
  ddl: string; kind: string; db: string; schema: string; name: string;
  x: number; y: number;
}

interface SqlEditorProps {
  tabId?: string;
  /** Zero-based index of the statement currently executing; null when idle. */
  activeStmtIdx?: number | null;
}

// ── Qualified identifier extractor ────────────────────────────────────────
// Given a Monaco model and cursor position, finds the full dot-separated
// identifier that contains the cursor (e.g. "DB.SCHEMA.TABLE" when the cursor
// is over any of the three parts) and returns its unquoted parts.
function getQualifiedIdent(model: any, pos: any): string[] | null {
  const line: string = model.getLineContent(pos.lineNumber);
  const col = pos.column - 1; // 0-based

  // Scan the line left-to-right, building each dot-separated qualified
  // identifier (which may contain quoted parts like "MY_TABLE"), and return
  // the one whose character span contains `col`.
  //
  // This forward-scanning approach correctly handles all combinations:
  //   MY_TABLE        "MY_TABLE"        DB.SCHEMA.TABLE
  //   "DB"."SCHEMA"."TABLE"     SCHEMA."TABLE"
  // It avoids the ambiguity of bidirectional expansion, which struggled to
  // distinguish opening vs closing double-quotes for cursors inside a quoted
  // identifier.
  let i = 0;
  while (i < line.length) {
    // Skip characters that cannot begin an identifier part.
    if (line[i] !== '"' && !/\w/.test(line[i])) { i++; continue; }

    const parts: string[] = [];
    let containsCol = false;

    // Parse one dot-separated qualified identifier.
    while (i < line.length) {
      const partStart = i;
      let partName = '';

      if (line[i] === '"') {
        // Quoted identifier: consume everything between the double-quotes.
        i++; // past opening '"'
        while (i < line.length && line[i] !== '"') { partName += line[i]; i++; }
        if (i < line.length) i++; // past closing '"'
      } else if (/\w/.test(line[i])) {
        // Bare (unquoted) identifier.
        while (i < line.length && /\w/.test(line[i])) { partName += line[i]; i++; }
      } else {
        break;
      }

      parts.push(partName);

      // `col` falls inside this part (including any surrounding quote chars).
      if (col >= partStart && col < i) containsCol = true;

      // Continue only if followed by '.' and another identifier part.
      if (i < line.length && line[i] === '.') {
        const next = line[i + 1];
        if (next !== undefined && (next === '"' || /\w/.test(next))) {
          if (col === i) containsCol = true; // cursor is on the '.'
          i++; // past '.'
          continue;
        }
      }
      break;
    }

    if (containsCol && parts.length > 0) return parts;
  }

  return null;
}

// ── Statement range parser ─────────────────────────────────────────────────
// Returns [{startLine, endLine}] (1-indexed Monaco line numbers) for each
// semicolon-separated statement in the SQL.  Mirrors the backend's
// splitStatements logic for consistent statement counting.
// Exported so QueryPage can compute statement offsets for selection runs.
export function getStatementLineRanges(sql: string): Array<{ startLine: number; endLine: number }> {
  const ranges: Array<{ startLine: number; endLine: number }> = [];
  let line = 1;
  let stmtStartLine = -1; // -1 = not yet started (waiting for first non-ws char)
  let inSingleQuote = false;
  let inDoubleQuote = false;
  let inLineComment = false;
  let inBlockComment = false;
  let dollarTag = "";

  const finishStmt = (endLine: number) => {
    if (stmtStartLine > 0) {
      ranges.push({ startLine: stmtStartLine, endLine });
      stmtStartLine = -1;
    }
  };

  for (let i = 0; i < sql.length; i++) {
    const ch = sql[i];

    if (ch === "\n") {
      if (inLineComment) inLineComment = false;
      line++;
      continue;
    }

    if (inLineComment) continue;

    if (inBlockComment) {
      if (ch === "*" && sql[i + 1] === "/") { inBlockComment = false; i++; }
      continue;
    }

    if (inSingleQuote) {
      if (ch === "'" && sql[i + 1] === "'") { i++; } // '' escape
      else if (ch === "'") { inSingleQuote = false; }
      continue;
    }

    if (inDoubleQuote) {
      if (ch === '"') inDoubleQuote = false;
      continue;
    }

    if (dollarTag) {
      if (sql.startsWith(dollarTag, i)) { i += dollarTag.length - 1; dollarTag = ""; }
      continue;
    }

    // Mark the start of a new statement on the first real (non-ws, non-comment-open) character.
    if (stmtStartLine < 0) {
      const ws  = ch === " " || ch === "\t" || ch === "\r";
      const cmt = (ch === "-" && sql[i + 1] === "-") || (ch === "/" && sql[i + 1] === "*");
      if (!ws && !cmt) stmtStartLine = line;
    }

    if (ch === "-" && sql[i + 1] === "-") { inLineComment = true; i++; continue; }
    if (ch === "/" && sql[i + 1] === "*") { inBlockComment = true; i++; continue; }
    if (ch === "'") { inSingleQuote = true; continue; }
    if (ch === '"') { inDoubleQuote = true; continue; }

    if (ch === "$") {
      const m = sql.slice(i).match(/^\$([a-zA-Z0-9_]*)\$/);
      if (m) { dollarTag = m[0]; i += dollarTag.length - 1; continue; }
    }

    if (ch === ";") { finishStmt(line); continue; }
  }

  finishStmt(line); // last statement with no trailing semicolon
  return ranges;
}

export default function SqlEditor({ tabId, activeStmtIdx }: SqlEditorProps = {}) {
  const activeSql       = useQueryStore((s) => s.sql);
  const activeSqlSetter = useQueryStore((s) => s.setSql);
  const tabs            = useQueryStore((s) => s.tabs);
  const setSqlForTab    = useQueryStore((s) => s.setSqlForTab);
  const setSelectedSql  = useQueryStore((s) => s.setSelectedSql);

  const sql    = tabId ? (tabs.find((t) => t.id === tabId)?.sql ?? "") : activeSql;
  const setSql = tabId ? (newSql: string) => setSqlForTab(tabId, newSql) : activeSqlSetter;
  const resolved          = useThemeStore((s) => s.resolved);
  const editorFont        = useThemeStore((s) => s.editorFont);
  const editorFontSize    = useThemeStore((s) => s.editorFontSize);
  const setEditorFontSize = useThemeStore((s) => s.setEditorFontSize);
  // Ref so the native keydown listener always sees the current font size
  // without being re-registered on every render.
  const fontSizeRef = useRef(editorFontSize);
  useEffect(() => { fontSizeRef.current = editorFontSize; }, [editorFontSize]);

  // Decoration collection for the currently-running statement highlight.
  // Set inside handleMount; read by the useEffect below.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const activeStmtDecRef = useRef<any>(null);

  // Decoration collection for function-call token highlighting.
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const fnDecRef      = useRef<any>(null);
  const fnDecTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [ddlHover, setDdlHover] = useState<DdlHover | null>(null);
  const [tooltipCtxMenu, setTooltipCtxMenu] = useState<{ x: number; y: number; sel: string } | null>(null);
  const hoverTimerRef     = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hoverHideTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Tracks the word key ("db.schema.table") the hover timer is currently running
  // for, and the latest cursor position under the mouse for that word.
  const lastHoverWordRef  = useRef<string | null>(null);
  const currentHoverPosRef = useRef<any>(null);
  // True while the cursor is physically inside the tooltip overlay.
  const isOnTooltipRef    = useRef(false);
  // True while a mouse button is held down (e.g. text selection drag).
  const isMouseDownRef    = useRef(false);
  // True while the right-click context menu is open (prevents tooltip hiding).
  const isCtxMenuOpenRef  = useRef(false);
  // Last text selection made inside the tooltip (saved on mouseup so right-click
  // can't clear it before onContextMenu fires).
  const savedSelRef       = useRef("");

  const scheduleHide = useCallback(() => {
    if (hoverHideTimerRef.current) clearTimeout(hoverHideTimerRef.current);
    hoverHideTimerRef.current = setTimeout(() => {
      setDdlHover(null);
      lastHoverWordRef.current = null;
    }, 400);
  }, []);
  const cancelHide = useCallback(() => {
    if (hoverHideTimerRef.current) clearTimeout(hoverHideTimerRef.current);
  }, []);

  // Hide tooltip on mouseup if cursor has left the overlay (handles text-selection drags
  // that temporarily move the cursor outside the tooltip bounds).
  useEffect(() => {
    const handleMouseUp = () => {
      isMouseDownRef.current = false;
      if (!isOnTooltipRef.current && !isCtxMenuOpenRef.current) setDdlHover(null);
    };
    document.addEventListener("mouseup", handleMouseUp);
    return () => document.removeEventListener("mouseup", handleMouseUp);
  }, []);

  // While the tooltip is open, intercept Cmd+C / Ctrl+C at capture phase so it
  // fires before Monaco's global key handler. Copies the current text selection
  // (if any) via the Wails clipboard API which works reliably in WKWebView.
  useEffect(() => {
    if (!ddlHover) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key === "c") {
        const sel = window.getSelection()?.toString();
        if (sel) {
          e.stopPropagation();
          ClipboardSetText(sel);
        }
      }
    };
    document.addEventListener("keydown", handleKeyDown, true);
    return () => document.removeEventListener("keydown", handleKeyDown, true);
  }, [ddlHover]);

  // Dismiss the right-click context menu on the next left-click anywhere.
  // Using a document click listener avoids needing a backdrop div, which would
  // cause mouseleave to fire on the tooltip and hide it.
  useEffect(() => {
    if (!tooltipCtxMenu) return;
    const dismiss = () => {
      setTooltipCtxMenu(null);
      setTimeout(() => { isCtxMenuOpenRef.current = false; }, 50);
    };
    document.addEventListener("click", dismiss);
    return () => document.removeEventListener("click", dismiss);
  }, [tooltipCtxMenu]);

  // Register the custom Snowflake SQL tokenizer and themes exactly once,
  // before the editor instance is created.
  const handleBeforeMount: BeforeMount = (monaco) => {
    ensureMonacoSetup(monaco);
  };

  const handleMount: OnMount = (editor, monaco) => {
    if (!tabId) {
      setEditorInstance(editor);
      editor.onDidDispose(() => setEditorInstance(null));
    }

    // Create the decoration collection used to highlight the active statement.
    activeStmtDecRef.current = editor.createDecorationsCollection([]);

    // ── Function-call token highlighting ──────────────────────────────────
    fnDecRef.current = editor.createDecorationsCollection([]);

    // Regex: matches identifiers immediately followed by '(' — i.e. function calls.
    const fnCallRe = /\b([A-Za-z_][A-Za-z0-9_$]*)\s*(?=\()/g;

    const refreshFnDecorations = () => {
      const model = editor.getModel();
      if (!model || (builtinFns.size === 0 && udfFns.size === 0)) return;
      const text = model.getValue();
      const decorations: any[] = [];
      fnCallRe.lastIndex = 0;
      let m: RegExpExecArray | null;
      while ((m = fnCallRe.exec(text)) !== null) {
        const word = m[1].toUpperCase();
        const cls = builtinFns.has(word) ? "sql-token-builtin"
                  : udfFns.has(word)     ? "sql-token-udf"
                  : null;
        if (!cls) continue;
        const start = model.getPositionAt(m.index);
        const end   = model.getPositionAt(m.index + m[1].length);
        decorations.push({
          range: new monaco.Range(start.lineNumber, start.column, end.lineNumber, end.column),
          options: { inlineClassName: cls },
        });
      }
      fnDecRef.current?.set(decorations);
    };

    // Populate function name sets on first mount, then decorate immediately.
    if (!fnNamesLoaded) {
      GetAllFunctionNames().then((fns) => {
        if (!fns) return;
        for (const fn of fns) {
          if (fn.functionType === "UDF") udfFns.add(fn.functionName);
          else builtinFns.add(fn.functionName);
        }
        fnNamesLoaded = true;
        refreshFnDecorations();
      }).catch(() => { /* best-effort */ });
    } else {
      refreshFnDecorations();
    }

    // Re-decorate on every content change (debounced).
    editor.getModel()?.onDidChangeContent(() => {
      if (fnDecTimerRef.current) clearTimeout(fnDecTimerRef.current);
      fnDecTimerRef.current = setTimeout(refreshFnDecorations, 200);
    });

    // ── Clipboard (WKWebView fix) ─────────────────────────────────────────
    // WKWebView blocks navigator.clipboard.readText/writeText (async Clipboard
    // API), so Monaco's built-in copy/paste silently fails.
    // Override the three clipboard keybindings inside Monaco's own command
    // system so Monaco never reaches its async clipboard code.

    // Shared implementations used by both keyboard and context-menu paths.
    const doPaste = async () => {
      const text = await ClipboardGetText();
      if (!text) return;
      const selection = editor.getSelection();
      if (!selection) return;
      editor.executeEdits("clipboard-paste", [{ range: selection, text, forceMoveMarkers: true }]);
      editor.pushUndoStop();
    };

    const doCopy = async () => {
      const selection = editor.getSelection();
      const model = editor.getModel();
      if (!selection || !model) return;
      const text = model.getValueInRange(selection);
      if (text) await ClipboardSetText(text);
    };

    const doCut = async () => {
      const selection = editor.getSelection();
      const model = editor.getModel();
      if (!selection || !model) return;
      const text = model.getValueInRange(selection);
      if (!text) return;
      await ClipboardSetText(text);
      editor.executeEdits("clipboard-cut", [{ range: selection, text: "", forceMoveMarkers: true }]);
      editor.pushUndoStop();
    };

    // Context-menu paste: Monaco's context menu calls commandService.executeCommand()
    // directly. Patch the editor's internal command service so paste/copy/cut from
    // the context menu use the Wails native clipboard.
    // Only patch for the primary editor: _commandService is shared across all Monaco
    // editor instances in the same window. The keyboard shortcuts (Cmd+V/C/X) are
    // handled via a capture-phase DOM listener below, which is per-editor and does
    // not have this sharing problem.
    if (!tabId) {
      const cs = (editor as any)._commandService;
      if (cs && typeof cs.executeCommand === "function") {
        const origExec = cs.executeCommand.bind(cs);
        cs.executeCommand = (commandId: string, ...args: any[]): Promise<any> => {
          switch (commandId) {
            case "editor.action.clipboardPasteAction": doPaste(); return Promise.resolve();
            case "editor.action.clipboardCopyAction":  doCopy();  return Promise.resolve();
            case "editor.action.clipboardCutAction":   doCut();   return Promise.resolve();
            default: return origExec(commandId, ...args);
          }
        };
      }
    }

    monaco.languages.registerCompletionItemProvider("sql", {
      triggerCharacters: ["."],
      provideCompletionItems: async (model: any, position: any) => {
        const word = model.getWordUntilPosition(position);
        const range = {
          startLineNumber: position.lineNumber,
          endLineNumber:   position.lineNumber,
          startColumn:     word.startColumn,
          endColumn:       word.endColumn,
        };

        // Text on the current line up to (but not including) the current word —
        // used to detect whether the user is typing after a dot qualifier.
        const lineUpToWord = model
          .getLineContent(position.lineNumber)
          .substring(0, word.startColumn - 1);

        // ── db.schema.table. → suggest columns ──────────────────────────
        const threePartMatch = lineUpToWord.match(/\b(\w+)\.(\w+)\.(\w+)\.\s*$/i);
        if (threePartMatch) {
          const [, db, schema, table] = threePartMatch;
          return { suggestions: mkColSuggestions(await getColumns(db, schema, table), range, monaco) };
        }

        // ── db.schema. → suggest objects in that schema ──────────────────
        const twoPartMatch = lineUpToWord.match(/\b(\w+)\.(\w+)\.\s*$/i);
        if (twoPartMatch) {
          const [, db, schema] = twoPartMatch;
          const schemaKey = `${UC(db)}\0${UC(schema)}`;

          // If `db` is not a known database, treat this as schema.table. → columns
          if (!useObjectStore.getState().databases.some((d) => UC(d) === UC(db))) {
            const colObj = useObjectStore.getState().objects.find(
              (o) => UC(o.schema) === UC(db) && UC(o.name) === UC(schema) &&
                     (o.kind === "TABLE" || o.kind === "VIEW")
            );
            if (colObj) {
              return { suggestions: mkColSuggestions(await getColumns(colObj.db, colObj.schema, colObj.name), range, monaco) };
            }
          }

          const hasObjects = useObjectStore.getState().objects
            .some((o) => UC(o.db) === UC(db) && UC(o.schema) === UC(schema));

          if (!hasObjects && !fetchedSchemaObjects.has(schemaKey)) {
            fetchedSchemaObjects.add(schemaKey);
            try {
              const fetched = await ListObjects(db, schema);
              useObjectStore.getState().addObjects(
                db, schema,
                (fetched ?? []).map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })),
              );
            } catch {
              fetchedSchemaObjects.delete(schemaKey); // allow retry on next keystroke
            }
          }

          return {
            suggestions: useObjectStore.getState().objects
              .filter((o) => UC(o.db) === UC(db) && UC(o.schema) === UC(schema))
              .map((o) => ({
                label:      o.name,
                kind:       monacoKind(monaco, o.kind),
                insertText: o.name,
                detail:     o.kind,
                range,
              })),
          };
        }

        // ── db. → suggest schemas of that database ────────────────────────
        const onePartMatch = lineUpToWord.match(/\b(\w+)\.\s*$/i);
        if (onePartMatch) {
          const [, qualifier] = onePartMatch;
          const { databases, schemas, objects } = useObjectStore.getState();

          // Is the qualifier a known database?
          const isKnownDb = databases.some((db) => UC(db) === UC(qualifier));
          if (isKnownDb) {
            const dbSchemas = schemas.filter((s) => UC(s.db) === UC(qualifier));
            if (dbSchemas.length === 0 && !fetchedDatabaseSchemas.has(UC(qualifier))) {
              fetchedDatabaseSchemas.add(UC(qualifier));
              try {
                const fetched = await ListSchemas(qualifier);
                useObjectStore.getState().addSchemas(qualifier, fetched ?? []);
              } catch {
                fetchedDatabaseSchemas.delete(UC(qualifier));
              }
            }
            return {
              suggestions: useObjectStore.getState().schemas
                .filter((s) => UC(s.db) === UC(qualifier))
                .map((s) => ({
                  label:      s.name,
                  kind:       monaco.languages.CompletionItemKind.Module,
                  insertText: s.name,
                  detail:     "SCHEMA",
                  range,
                })),
            };
          }

          // Is the qualifier a known schema? → suggest its objects
          const schemaObjs = objects.filter((o) => UC(o.schema) === UC(qualifier));
          if (schemaObjs.length > 0) {
            return {
              suggestions: schemaObjs.map((o) => ({
                label:      o.name,
                kind:       monacoKind(monaco, o.kind),
                insertText: o.name,
                detail:     o.kind,
                range,
              })),
            };
          }

          // Is the qualifier a known table/view? → suggest its columns
          const colObjs = objects.filter(
            (o) => UC(o.name) === UC(qualifier) && (o.kind === "TABLE" || o.kind === "VIEW")
          );
          if (colObjs.length > 0) {
            const allCols = new Set<string>();
            await Promise.all(
              colObjs.map(async (o) => {
                (await getColumns(o.db, o.schema, o.name)).forEach((c) => allCols.add(c));
              })
            );
            if (allCols.size > 0) {
              return { suggestions: mkColSuggestions(Array.from(allCols), range, monaco) };
            }
          }
        }

        // ── JOIN ON → suggest FK / same-name-column conditions ──────────────
        // Detect whether the cursor is inside a JOIN … ON clause.
        // Strategy: scan full text-to-cursor rather than relying on
        // getWordUntilPosition, which behaves unpredictably after `"` (quoted
        // identifier start) and varies across Monaco versions.
        // When no alias is used, ref.alias defaults to the table name, so
        // suggestions appear as  TABLE1.col = TABLE2.col  rather than a.col = b.col.
        const cursorOffset = model.getOffsetAt(position);
        const textToCursor = model.getValue().slice(0, cursorOffset);

        // Find the last JOIN keyword in the text, then check whether ON appears
        // after it with no intervening clause keyword (WHERE, GROUP, ORDER, …).
        const isInJoinOnClause = (() => {
          const joinMatches = [...textToCursor.matchAll(/\bJOIN\b/gi)];
          if (joinMatches.length === 0) return false;
          const lastJoin = joinMatches[joinMatches.length - 1];
          const afterLastJoin = textToCursor.slice(lastJoin.index! + lastJoin[0].length);
          const onMatch = afterLastJoin.match(/\bON\b/i);
          if (!onMatch) return false;
          const afterOn = afterLastJoin.slice(onMatch.index! + onMatch[0].length);
          // Still in ON clause if no JOIN/WHERE/GROUP/ORDER/HAVING/UNION/… follows
          return !/\b(?:JOIN|WHERE|GROUP|ORDER|HAVING|UNION|INTERSECT|EXCEPT)\b/i.test(afterOn);
        })();

        const wordIsOn = word.word.toUpperCase() === "ON";
        if (wordIsOn || isInJoinOnClause) {
          const refs = parseJoinTables(textToCursor);
          if (refs.length >= 2) {
            const storeObjs = useObjectStore.getState().objects;
            const resolvedRefs = resolveRefs(refs, storeObjs);

            if (resolvedRefs && resolvedRefs.length >= 2) {
              const onSuggestions: any[] = [];
              const seen = new Set<string>();
              const lastRef   = resolvedRefs[resolvedRefs.length - 1];
              const otherRefs = resolvedRefs.slice(0, -1);

              // Warm up FKs for all involved schemas (background, non-blocking)
              for (const ref of resolvedRefs) {
                warmUpFKsForSchema(ref.db, ref.schema).catch(() => {});
              }

              // ── Tier 1a: Explicit FK constraints (composite-aware) ────────
              const lastFKs = await getFKs(lastRef.db, lastRef.schema, lastRef.name);
              for (const otherRef of otherRefs) {
                // lastRef is FK child → otherRef is PK parent
                const fksForPk = lastFKs.filter((fk) =>
                  UC(fk.pkTable) === UC(otherRef.name) &&
                  (!fk.pkSchema   || UC(fk.pkSchema)   === UC(otherRef.schema)) &&
                  (!fk.pkDatabase || UC(fk.pkDatabase) === UC(otherRef.db)),
                );
                for (const cond of buildCompositeConditions(fksForPk, lastRef.alias, otherRef.alias)) {
                  if (!seen.has(cond)) {
                    seen.add(cond);
                    onSuggestions.push(makeSugg(cond, "FK RELATION", `0a${cond}`, range, monaco));
                  }
                }
                // otherRef is FK child → lastRef is PK parent
                const otherFKs = await getFKs(otherRef.db, otherRef.schema, otherRef.name);
                const fksForLast = otherFKs.filter((fk) =>
                  UC(fk.pkTable) === UC(lastRef.name) &&
                  (!fk.pkSchema   || UC(fk.pkSchema)   === UC(lastRef.schema)) &&
                  (!fk.pkDatabase || UC(fk.pkDatabase) === UC(lastRef.db)),
                );
                for (const cond of buildCompositeConditions(fksForLast, otherRef.alias, lastRef.alias)) {
                  if (!seen.has(cond)) {
                    seen.add(cond);
                    onSuggestions.push(makeSugg(cond, "FK RELATION", `0b${cond}`, range, monaco));
                  }
                }
              }

              // ── Tier 1b: PK name heuristic (only when no FK suggestions) ─
              if (onSuggestions.length === 0) {
                const lastColNames = await getColumns(lastRef.db, lastRef.schema, lastRef.name);
                for (const otherRef of otherRefs) {
                  const otherColNames = await getColumns(otherRef.db, otherRef.schema, otherRef.name);
                  for (const cond of pkHeuristicConditions(lastRef, otherRef, lastColNames, otherColNames)) {
                    if (!seen.has(cond)) {
                      seen.add(cond);
                      onSuggestions.push(makeSugg(cond, "PK HEURISTIC", `0c${cond}`, range, monaco));
                    }
                  }
                }
              }

              // ── Tier 2: Same-name columns (type-compatible) + USING ───────
              const lastColInfos = await getColInfos(lastRef.db, lastRef.schema, lastRef.name);
              const lastColInfoMap = new Map(lastColInfos.map((c) => [UC(c.name), c.dataType]));
              for (const otherRef of otherRefs) {
                const otherColInfos = await getColInfos(otherRef.db, otherRef.schema, otherRef.name);
                const sharedCompatible: string[] = [];
                for (const info of otherColInfos) {
                  const dt1 = lastColInfoMap.get(UC(info.name));
                  if (!dt1) continue;
                  const cat1 = typeCategory(dt1);
                  const cat2 = typeCategory(info.dataType);
                  // Allow if same category or either is "other" (unknown → permissive)
                  if (cat1 !== "other" && cat2 !== "other" && cat1 !== cat2) continue;
                  sharedCompatible.push(info.name);
                  const cond = `${lastRef.alias}.${info.name} = ${otherRef.alias}.${info.name}`;
                  if (!seen.has(cond)) {
                    seen.add(cond);
                    onSuggestions.push(makeSugg(cond, "SAME-NAME COLUMN", `1${cond}`, range, monaco));
                  }
                }
                // USING syntax for type-compatible same-name columns
                if (sharedCompatible.length > 0) {
                  const usingCond = `USING (${sharedCompatible.join(", ")})`;
                  if (!seen.has(usingCond)) {
                    seen.add(usingCond);
                    onSuggestions.push(makeSugg(usingCond, "USING", `1.5${usingCond}`, range, monaco));
                  }
                }
              }

              if (onSuggestions.length > 0) {
                return { suggestions: onSuggestions };
              }
            }
          }
        }

        // ── Trigger C: Ctrl+Space after JOIN table (before ON is typed) ─────
        // Detect: last JOIN clause in text-to-cursor has no ON / USING yet.
        // Reuses textToCursor computed above.
        {
          const lastJoinSegment = (textToCursor.split(/\bJOIN\b/i).pop() ?? "").trim();
          const hasTriggerC =
            lastJoinSegment.length > 0 &&
            !/\b(?:ON|USING)\b/i.test(lastJoinSegment) &&
            parseJoinTables(textToCursor).length >= 2;

          if (hasTriggerC) {
            const refsC = parseJoinTables(textToCursor);
            const resolvedC = resolveRefs(refsC, useObjectStore.getState().objects);
            if (resolvedC && resolvedC.length >= 2) {
              const lastR  = resolvedC[resolvedC.length - 1];
              const others = resolvedC.slice(0, -1);
              const cSugg: any[] = [];
              const seenC = new Set<string>();

              // Tier 1a: FK constraints
              const lastFKsC = await getFKs(lastR.db, lastR.schema, lastR.name);
              for (const otherR of others) {
                const fksC = lastFKsC.filter((fk) => UC(fk.pkTable) === UC(otherR.name));
                for (const cond of buildCompositeConditions(fksC, lastR.alias, otherR.alias)) {
                  if (!seenC.has(cond)) {
                    seenC.add(cond);
                    cSugg.push(makeSugg(`ON ${cond}`, "FK RELATION", `0a${cond}`, range, monaco));
                  }
                }
                const otherFKsC = await getFKs(otherR.db, otherR.schema, otherR.name);
                const fksForLastC = otherFKsC.filter((fk) => UC(fk.pkTable) === UC(lastR.name));
                for (const cond of buildCompositeConditions(fksForLastC, otherR.alias, lastR.alias)) {
                  if (!seenC.has(cond)) {
                    seenC.add(cond);
                    cSugg.push(makeSugg(`ON ${cond}`, "FK RELATION", `0b${cond}`, range, monaco));
                  }
                }
              }

              // Tier 1b: PK name heuristic (only when no FK suggestions)
              if (cSugg.length === 0) {
                const lastColsC = await getColumns(lastR.db, lastR.schema, lastR.name);
                for (const otherR of others) {
                  const otherColsC = await getColumns(otherR.db, otherR.schema, otherR.name);
                  for (const cond of pkHeuristicConditions(lastR, otherR, lastColsC, otherColsC)) {
                    if (!seenC.has(cond)) {
                      seenC.add(cond);
                      cSugg.push(makeSugg(`ON ${cond}`, "PK HEURISTIC", `0c${cond}`, range, monaco));
                    }
                  }
                }
              }

              // Tier 2: same-name type-compatible columns + USING
              const lastInfosC = await getColInfos(lastR.db, lastR.schema, lastR.name);
              const lastInfoMapC = new Map(lastInfosC.map((c) => [UC(c.name), c.dataType]));
              for (const otherR of others) {
                const otherInfosC = await getColInfos(otherR.db, otherR.schema, otherR.name);
                const sharedC: string[] = [];
                for (const info of otherInfosC) {
                  const dt1 = lastInfoMapC.get(UC(info.name));
                  if (!dt1) continue;
                  const cat1 = typeCategory(dt1), cat2 = typeCategory(info.dataType);
                  if (cat1 !== "other" && cat2 !== "other" && cat1 !== cat2) continue;
                  sharedC.push(info.name);
                  const cond = `${lastR.alias}.${info.name} = ${otherR.alias}.${info.name}`;
                  if (!seenC.has(cond)) {
                    seenC.add(cond);
                    cSugg.push(makeSugg(`ON ${cond}`, "SAME-NAME COLUMN", `1${cond}`, range, monaco));
                  }
                }
                if (sharedC.length > 0) {
                  const usingC = `USING (${sharedC.join(", ")})`;
                  if (!seenC.has(usingC)) {
                    seenC.add(usingC);
                    cSugg.push(makeSugg(usingC, "USING", `1.5${usingC}`, range, monaco));
                  }
                }
              }

              if (cSugg.length > 0) return { suggestions: cSugg };
            }
          }
        }

        // ── No qualifier → keywords + databases + all object names ────────
        const { databases, schemas, objects } = useObjectStore.getState();

        const keywordSuggestions = SNOWFLAKE_KEYWORDS.map((kw) => ({
          label:      kw,
          kind:       monaco.languages.CompletionItemKind.Keyword,
          insertText: kw,
          range,
        }));

        const dbSuggestions = databases.map((db) => ({
          label:      db,
          kind:       monaco.languages.CompletionItemKind.Module,
          insertText: db,
          detail:     "DATABASE",
          range,
        }));

        const schemaSuggestions = schemas.map((s) => ({
          label:      s.name,
          kind:       monaco.languages.CompletionItemKind.Module,
          insertText: s.name,
          detail:     `SCHEMA · ${s.db}`,
          range,
        }));

        const objectSuggestions = objects.map((o) => ({
          label:      o.name,
          kind:       monacoKind(monaco, o.kind),
          insertText: o.name,
          detail:     `${o.kind} · ${o.db}.${o.schema}`,
          range,
        }));

        // ── Context columns: scan FROM/JOIN refs in the current query ────
        // Use the column cache SYNCHRONOUSLY so Monaco sees results immediately.
        // If a table's columns are not yet cached, fire a background fetch so
        // the NEXT Ctrl+Space press will find them in the cache.
        // Scan the full editor text so FROM/JOIN refs below the cursor
        // (e.g. when completing inside a SELECT list) are also found.
        const fullTextToCursor = model.getValue();

        // Matches quoted ("IDENT") and unquoted identifiers in FROM/JOIN clauses.
        // Handles: db.schema.table | schema.table | table (each part quoted or unquoted)
        const ID_PAT = `(?:"[^"]+"|\\w+)`;
        const tableRefRe = new RegExp(
          `(?:FROM|JOIN)\\s+(?:(${ID_PAT})\\.(${ID_PAT})\\.(${ID_PAT})|(${ID_PAT})\\.(${ID_PAT})|(${ID_PAT}))`,
          "gi"
        );
        // Strip surrounding double-quotes from a captured identifier group.
        const stripQ = (s: string | undefined) =>
          s ? (s.startsWith('"') ? s.slice(1, -1) : s) : undefined;

        const seenColKeys = new Set<string>();
        const contextColSuggestions: any[] = [];
        let fetchPending = false;
        let tm: RegExpExecArray | null;
        while ((tm = tableRefRe.exec(fullTextToCursor)) !== null) {
          let refDb: string | undefined, refSchema: string | undefined, refName: string;
          if (tm[1] && tm[2] && tm[3]) {
            [refDb, refSchema, refName] = [stripQ(tm[1])!, stripQ(tm[2])!, stripQ(tm[3])!];
          } else if (tm[4] && tm[5]) {
            [refSchema, refName] = [stripQ(tm[4])!, stripQ(tm[5])!];
          } else {
            refName = stripQ(tm[6])!;
          }

          const matchedObjs = objects.filter((o) => {
            if (o.kind !== "TABLE" && o.kind !== "VIEW") return false;
            if (UC(o.name) !== UC(refName)) return false;
            if (refDb && UC(o.db) !== UC(refDb)) return false;
            if (refSchema && UC(o.schema) !== UC(refSchema)) return false;
            return true;
          });

          for (const obj of matchedObjs) {
            const cacheKey = `${UC(obj.db)}\0${UC(obj.schema)}\0${UC(obj.name)}`;
            if (columnCache.has(cacheKey)) {
              // Columns already cached — add synchronously
              for (const col of columnCache.get(cacheKey)!) {
                if (!seenColKeys.has(UC(col))) {
                  seenColKeys.add(UC(col));
                  contextColSuggestions.push({
                    label:      col,
                    kind:       monaco.languages.CompletionItemKind.Field,
                    insertText: col,
                    detail:     `COLUMN · ${obj.name}`,
                    range,
                  });
                }
              }
            } else {
              // Not cached yet — fire background fetch; columns appear on next Ctrl+Space
              getColumns(obj.db, obj.schema, obj.name);
              fetchPending = true;
            }
          }
        }

        // ── Function completions (only when not inside a dotted context) ────
        let fnSuggestions: any[] = [];
        if (word.word.length >= 2 && !lineUpToWord.trim().endsWith(".")) {
          try {
            const fns = await GetFunctionSuggestions(word.word);
            if (fns) {
              fnSuggestions = fns.map((fn) => ({
                label:            fn.functionName,
                kind:             monaco.languages.CompletionItemKind.Function,
                detail:           fn.functionType === "UDF" ? "User-defined function" : "Built-in function",
                documentation:    fn.description || fn.functionSignature,
                insertText:       fn.functionName,
                filterText:       fn.functionName,
                sortText:         fn.functionType === "UDF" ? "0" + fn.functionName : "1" + fn.functionName,
                range,
              }));
            }
          } catch {
            // best-effort — silently ignore if store is not ready
          }
        }

        return {
          suggestions: [...contextColSuggestions, ...keywordSuggestions, ...dbSuggestions, ...schemaSuggestions, ...objectSuggestions, ...fnSuggestions],
          // Tell Monaco these results may be incomplete so it re-queries on next invocation
          incomplete: fetchPending,
        };
      },
    });

    // ── Object definition hover (custom React overlay) ───────────────────
    // Dispose any previously registered Monaco hover provider from a prior mount.
    if (hoverProviderDisposable) {
      hoverProviderDisposable.dispose();
      hoverProviderDisposable = null;
    }

    editor.onMouseMove((e: any) => {
      const pos = e.target?.position;
      const model = editor.getModel();
      const parts = (pos && model) ? getQualifiedIdent(model, pos) : null;
      if (!parts || parts.length === 0) {
        // Mouse moved off any recognisable identifier — cancel the pending show
        // timer so it doesn't fire after the mouse has already left the word.
        lastHoverWordRef.current = null;
        if (hoverTimerRef.current) { clearTimeout(hoverTimerRef.current); hoverTimerRef.current = null; }
        if (!isOnTooltipRef.current) scheduleHide();
        return;
      }

      cancelHide();

      // Always update the latest position so the tooltip appears where the
      // mouse currently is, even if it moved within the same word.
      currentHoverPosRef.current = pos;

      // Only restart the timer when the hovered word changes.  Moving
      // within the same word should NOT reset the clock — otherwise the tooltip
      // never fires while the mouse is crossing the token.
      const wordKey = parts.join("\0");
      if (wordKey === lastHoverWordRef.current) return;
      lastHoverWordRef.current = wordKey;

      if (hoverTimerRef.current) clearTimeout(hoverTimerRef.current);

      hoverTimerRef.current = setTimeout(async () => {
        // Bail if the mouse has already moved to a different word (or off-word)
        // since this timer was scheduled.
        if (lastHoverWordRef.current !== wordKey) return;

        // Use the most recent position so the tooltip is anchored to wherever
        // the mouse is now (may have moved within the same word).
        const pos = currentHoverPosRef.current;
        if (!pos) return;
        const { objects } = useObjectStore.getState();

        let db = "", schema = "", kind = "", name = "", ddl = "";

        if (parts.length >= 3) {
          // 3-part: DB.SCHEMA.TABLE — use last three parts
          const [pDb, pSchema, pName] = [
            parts[parts.length - 3],
            parts[parts.length - 2],
            parts[parts.length - 1],
          ];
          // If this schema's objects aren't loaded yet, fetch them now so we
          // know the kind before calling GET_DDL. This avoids a failed
          // GET_DDL('TABLE',...) attempt on a VIEW (and vice versa) which the
          // gosnowflake driver logs at ERROR level even when caught gracefully.
          const schemaKey = `${UC(pDb)}\0${UC(pSchema)}`;
          const hasSchemaInStore = useObjectStore.getState().objects
            .some((o) => UC(o.db) === UC(pDb) && UC(o.schema) === UC(pSchema));
          if (!hasSchemaInStore && !fetchedSchemaObjects.has(schemaKey)) {
            fetchedSchemaObjects.add(schemaKey);
            try {
              const fetched = await ListObjects(pDb, pSchema);
              useObjectStore.getState().addObjects(
                pDb, pSchema,
                (fetched ?? []).map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })),
              );
            } catch {
              fetchedSchemaObjects.delete(schemaKey);
            }
          }
          const inStore = useObjectStore.getState().objects.find(
            (o) => UC(o.db) === UC(pDb) && UC(o.schema) === UC(pSchema) &&
                   UC(o.name) === UC(pName) && (o.kind === "TABLE" || o.kind === "VIEW"),
          );
          if (inStore) {
            db = inStore.db; schema = inStore.schema; kind = inStore.kind; name = inStore.name;
          } else {
            setDdlHover(null); return;
          }
        } else if (parts.length === 2) {
          // 2-part: SCHEMA.TABLE
          const [qualifier, pName] = [parts[0], parts[1]];
          let inStore = objects.find(
            (o) => UC(o.schema) === UC(qualifier) && UC(o.name) === UC(pName) &&
                   (o.kind === "TABLE" || o.kind === "VIEW"),
          );
          if (!inStore) {
            // Objects for this schema may not be loaded yet — try auto-loading
            // using the current session database.
            const sessDb = useSessionStore.getState().database;
            if (sessDb) {
              const schemaKey = `${UC(sessDb)}\0${UC(qualifier)}`;
              if (!fetchedSchemaObjects.has(schemaKey)) {
                fetchedSchemaObjects.add(schemaKey);
                try {
                  const fetched = await ListObjects(sessDb, qualifier);
                  useObjectStore.getState().addObjects(
                    sessDb, qualifier,
                    (fetched ?? []).map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })),
                  );
                } catch { fetchedSchemaObjects.delete(schemaKey); }
              }
              inStore = useObjectStore.getState().objects.find(
                (o) => UC(o.db) === UC(sessDb) && UC(o.schema) === UC(qualifier) &&
                       UC(o.name) === UC(pName) && (o.kind === "TABLE" || o.kind === "VIEW"),
              );
            }
          }
          if (!inStore) { setDdlHover(null); return; }
          db = inStore.db; schema = inStore.schema; kind = inStore.kind; name = inStore.name;
        } else {
          // 1-part: name only — look in any loaded schema first, then auto-load
          // from the current session's database+schema if needed.
          let inStore = objects.find(
            (o) => UC(o.name) === UC(parts[0]) && (o.kind === "TABLE" || o.kind === "VIEW"),
          );
          if (!inStore) {
            const sess = useSessionStore.getState();
            if (sess.database && sess.schema) {
              const schemaKey = `${UC(sess.database)}\0${UC(sess.schema)}`;
              if (!fetchedSchemaObjects.has(schemaKey)) {
                fetchedSchemaObjects.add(schemaKey);
                try {
                  const fetched = await ListObjects(sess.database, sess.schema);
                  useObjectStore.getState().addObjects(
                    sess.database, sess.schema,
                    (fetched ?? []).map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })),
                  );
                } catch { fetchedSchemaObjects.delete(schemaKey); }
              }
              inStore = useObjectStore.getState().objects.find(
                (o) => UC(o.db) === UC(sess.database) && UC(o.schema) === UC(sess.schema) &&
                       UC(o.name) === UC(parts[0]) && (o.kind === "TABLE" || o.kind === "VIEW"),
              );
            }
          }
          if (!inStore) {
            // Not a TABLE/VIEW — check if it's a known Snowflake function.
            try {
              const fns = await GetFunctionTooltip(parts[0]);
              if (fns && fns.length > 0) {
                const sigs = fns.map((fn: any) => fn.functionSignature).join("\n");
                const desc = fns.find((fn: any) => fn.description)?.description ?? "";
                const fnDdl = desc ? `${sigs}\n\n${desc}` : sigs;
                const fnKind = fns[0].functionType === "UDF" ? "UDF" : "FUNCTION";
                const editorDom2 = editor.getDomNode();
                const editorRect2 = editorDom2?.getBoundingClientRect();
                const scrolledPos2 = editor.getScrolledVisiblePosition(pos);
                if (scrolledPos2 && editorRect2) {
                  const lineH2 = scrolledPos2.height ?? 20;
                  const rawX2 = editorRect2.left + scrolledPos2.left;
                  const belowY2 = editorRect2.top + scrolledPos2.top + lineH2 + 4;
                  const aboveY2 = editorRect2.top + scrolledPos2.top - 4;
                  const fitsBelow2 = belowY2 + 320 <= window.innerHeight;
                  const fnX = Math.min(rawX2, window.innerWidth - 570);
                  const fnY = fitsBelow2 ? belowY2 : Math.max(0, aboveY2 - 320);
                  if (hoverHideTimerRef.current) { clearTimeout(hoverHideTimerRef.current); hoverHideTimerRef.current = null; }
                  setDdlHover({ ddl: fnDdl, kind: fnKind, db: "", schema: "", name: parts[0].toUpperCase(), x: fnX, y: fnY });
                }
              } else {
                setDdlHover(null);
              }
            } catch { setDdlHover(null); }
            return;
          }
          db = inStore.db; schema = inStore.schema; kind = inStore.kind; name = inStore.name;
        }

        // Fetch DDL from cache or API (skip if already resolved by direct 3-part fetch above)
        if (!ddl) {
          const cacheKey = `${db}\0${schema}\0${kind}\0${name}`;
          const cached = hoverDDLCache.get(cacheKey);
          if (cached && Date.now() - cached.ts < DDL_CACHE_TTL) {
            ddl = cached.ddl;
          } else {
            try {
              ddl = await GetObjectDDL(db, schema, kind, name, "");
              hoverDDLCache.set(cacheKey, { ddl, ts: Date.now() });
            } catch {
              return;
            }
          }
        }
        if (!ddl) return;

        const editorDom = editor.getDomNode();
        const editorRect = editorDom?.getBoundingClientRect();
        const scrolledPos = editor.getScrolledVisiblePosition(pos);
        if (!scrolledPos || !editorRect) return;

        const lineH   = scrolledPos.height ?? 20;
        const rawX    = editorRect.left + scrolledPos.left;
        const belowY  = editorRect.top + scrolledPos.top + lineH + 4;
        const aboveY  = editorRect.top + scrolledPos.top - 4;
        const fitsBelow = belowY + 320 <= window.innerHeight;
        const x = Math.min(rawX, window.innerWidth - 570);
        const y = fitsBelow ? belowY : Math.max(0, aboveY - 320);

        // Cancel any pending hide before showing — prevents a race where the
        // mouse crossed the word quickly, scheduleHide() was called, and its
        // 400 ms timer would dismiss the tooltip right after it appears.
        if (hoverHideTimerRef.current) { clearTimeout(hoverHideTimerRef.current); hoverHideTimerRef.current = null; }
        setDdlHover({ ddl, kind, db, schema, name, x, y });
      }, 200);
    });

    editor.onMouseLeave(() => {
      lastHoverWordRef.current = null;
      if (hoverTimerRef.current) clearTimeout(hoverTimerRef.current);
      if (!isOnTooltipRef.current) scheduleHide();
    });

    // ── AI inline completions ─────────────────────────────────────────────
    if (!inlineCompletionsDisposable) {
      inlineCompletionsDisposable = monaco.languages.registerInlineCompletionsProvider("sql", {
        provideInlineCompletions: async (model: any, position: any, _ctx: any, token: any) => {
          // ── Trigger A: ghost text after JOIN table (before ON is typed) ───
          const prefixFull = model.getValue().slice(0, model.getOffsetAt(position));
          const lastJoinSeg = (prefixFull.split(/\bJOIN\b/i).pop() ?? "").trim();
          if (lastJoinSeg.length > 0 && !/\b(?:ON|USING)\b/i.test(lastJoinSeg)) {
            const ghostRefs = parseJoinTables(prefixFull);
            if (ghostRefs.length >= 2) {
              const resolved = resolveRefs(ghostRefs, useObjectStore.getState().objects);
              if (resolved && resolved.length >= 2) {
                const lr = resolved[resolved.length - 1];
                const or = resolved[resolved.length - 2];
                // Use cache only — getFKs returns [] if not yet fetched (non-blocking)
                const lFKs = fkCache.get(
                  `${UC(lr.db)}\0${UC(lr.schema)}\0${UC(lr.name)}`,
                ) ?? [];
                const relevant = lFKs.filter((fk) => UC(fk.pkTable) === UC(or.name));
                const conds = buildCompositeConditions(relevant, lr.alias, or.alias);
                if (conds.length > 0 && !token.isCancellationRequested) {
                  return { items: [{ insertText: `ON ${conds[0]}` }] };
                }
              }
            }
          }
          // fall through to AI suggestion ─────────────────────────────────────

          const prefix = model.getValueInRange({
            startLineNumber: Math.max(1, position.lineNumber - 30),
            startColumn:     1,
            endLineNumber:   position.lineNumber,
            endColumn:       position.column,
          });
          const trimmed = prefix.length > 800 ? prefix.slice(-800) : prefix;
          if (trimmed.trim().length < 3) return { items: [] };

          const suggestion = await GetAISuggestion(trimmed);
          if (token.isCancellationRequested || !suggestion) return { items: [] };

          return { items: [{ insertText: suggestion }] };
        },
        freeInlineCompletions: () => {},
      });
    }

    // ── Selection highlight ───────────────────────────────────────────────
    // When text is selected, find every other occurrence in the document and
    // decorate it with a coloured background so they are easy to spot.
    const occurrences = editor.createDecorationsCollection([]);

    const refreshOccurrences = () => {
      const selection = editor.getSelection();
      const model     = editor.getModel();

      if (!model || !selection || selection.isEmpty()) {
        occurrences.clear();
        return;
      }

      const selectedText = model.getValueInRange(selection);

      // Ignore whitespace-only or single-character selections.
      if (selectedText.trim().length < 2) {
        occurrences.clear();
        return;
      }

      const matches = model.findMatches(
        selectedText,
        true,   // searchOnlyEditableRange
        false,  // isRegex
        true,   // matchCase
        null,   // wordSeparators (null = substring, no word boundary)
        false,  // captureMatches
      );

      occurrences.set(
        matches
          // Exclude the range the user has actively selected.
          .filter((m) => !selection.equalsRange(m.range))
          .map((m) => ({
            range: m.range,
            options: {
              inlineClassName: "sql-occurrence-highlight",
              overviewRuler: {
                color: "rgba(173, 214, 255, 0.5)",
                position: monaco.editor.OverviewRulerLane.Center,
              },
            },
          })),
      );
    };

    // Track selection so QueryPage knows what to run, and refresh highlights.
    editor.onDidChangeCursorSelection(() => {
      const selection = editor.getSelection();
      const selected  = selection && !selection.isEmpty()
        ? editor.getModel()?.getValueInRange(selection) ?? ""
        : "";
      setSelectedSql(selected);
      refreshOccurrences();
    });

    // Cmd+Enter / Ctrl+Enter → run query
    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter,
      () => window.dispatchEvent(new CustomEvent("run-query"))
    );

    // Cmd+S / Ctrl+S → save file
    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS,
      () => window.dispatchEvent(new CustomEvent("save-file"))
    );

    // Toggle Line Comment → right-click context menu entry only (no keybinding here;
    // the shortcut is handled via a native keydown listener below to avoid WKWebView capture).
    editor.addAction({
      id: "thaw.toggleLineComment",
      label: "Toggle Line Comment",
      contextMenuGroupId: "1_modification",
      contextMenuOrder: 1,
      run: (ed) => ed.trigger("keyboard", "editor.action.commentLine", null),
    });

    // thaw:scroll-to-line → jump to a specific line and highlight the match (used by file search)
    const handleScrollToLine = (e: Event) => {
      const { line, matchStart, matchEnd } =
        (e as CustomEvent<{ line: number; matchStart?: number; matchEnd?: number }>).detail;
      if (typeof line !== "number") return;
      editor.revealLineInCenter(line);
      if (typeof matchStart === "number" && typeof matchEnd === "number") {
        // Monaco columns are 1-based; matchStart/matchEnd are 0-based byte offsets.
        editor.setSelection({
          startLineNumber: line,
          startColumn:     matchStart + 1,
          endLineNumber:   line,
          endColumn:       matchEnd + 1,
        });
      } else {
        editor.setPosition({ lineNumber: line, column: 1 });
      }
    };
    window.addEventListener("thaw:scroll-to-line", handleScrollToLine);

    // ── Drag-and-drop from sidebar / panels ──────────────────────────────
    // TABLE/VIEW nodes set "thaw/table"; user rows set "thaw/user".
    const editorDom = editor.getDomNode();
    if (editorDom) {
      // Native keydown listener — handles shortcuts that WKWebView intercepts
      // before Monaco's own keybinding layer sees them.
      // Font-size shortcuts use e.key (the printed character) so they work
      // correctly on non-US keyboard layouts such as Finnish, where e.code
      // positions differ from the US layout (e.g. Finnish "+" is e.code=Minus).
      editorDom.addEventListener("keydown", (e: KeyboardEvent) => {
        if (!(e.metaKey || e.ctrlKey)) return;
        // Cmd++ / Cmd+= → increase font size
        if (e.key === "+" || e.key === "=") {
          e.preventDefault();
          setEditorFontSize(Math.min(fontSizeRef.current + 1, 32));
          return;
        }
        // Cmd+- → decrease font size
        if (e.key === "-") {
          e.preventDefault();
          setEditorFontSize(Math.max(fontSizeRef.current - 1, 8));
          return;
        }
        // Cmd+0 → reset font size to default
        if (e.key === "0") {
          e.preventDefault();
          setEditorFontSize(14);
        }
      });

      // Clipboard shortcuts — use a capture-phase listener so this fires before
      // Monaco's internal keyboard handler (which runs on the textarea inside
      // editorDom). stopPropagation() prevents the event from reaching the
      // textarea, so Monaco never sees Cmd+V/C/X and doesn't call the shared
      // _commandService. Each editor's editorDom is a separate DOM node, so
      // capture listeners here are definitively per-editor instance.
      editorDom.addEventListener("keydown", (e: KeyboardEvent) => {
        if (!(e.metaKey || e.ctrlKey)) return;
        switch (e.key.toLowerCase()) {
          case "v": e.preventDefault(); e.stopPropagation(); doPaste(); break;
          case "c": e.preventDefault(); e.stopPropagation(); doCopy(); break;
          case "x": e.preventDefault(); e.stopPropagation(); doCut(); break;
        }
      }, true /* capture */);

      editorDom.addEventListener("dragover", (e: DragEvent) => {
        const types = e.dataTransfer?.types ?? [];
        if (types.includes("thaw/table") || types.includes("thaw/user")) {
          e.preventDefault();
          e.dataTransfer!.dropEffect = "copy";
        }
      });

      // ── user DDL drop ─────────────────────────────────────────────────
      editorDom.addEventListener("drop", async (e: DragEvent) => {
        const rawUser = e.dataTransfer?.getData("thaw/user");
        if (rawUser) {
          e.preventDefault();
          let info: { name: string };
          try { info = JSON.parse(rawUser); } catch { return; }

          const target = editor.getTargetAtClientPoint(e.clientX, e.clientY);
          const pos = target?.position ?? editor.getPosition() ?? { lineNumber: 1, column: 1 };

          let ddl: string;
          try {
            ddl = await GetUserDDL(info.name);
          } catch {
            ddl = `-- Could not fetch DDL for user "${info.name}"`;
          }

          const range = {
            startLineNumber: pos.lineNumber,
            endLineNumber:   pos.lineNumber,
            startColumn:     pos.column,
            endColumn:       pos.column,
          };
          editor.executeEdits("drag-drop-user", [{ range, text: ddl, forceMoveMarkers: true }]);
          editor.pushUndoStop();
          editor.focus();
          return;
        }
      });

      // ── table/view SELECT drop ────────────────────────────────────────
      editorDom.addEventListener("drop", async (e: DragEvent) => {
        const raw = e.dataTransfer?.getData("thaw/table");
        if (!raw) return;
        e.preventDefault();
        let info: { db: string; schema: string; name: string };
        try { info = JSON.parse(raw); } catch { return; }

        const target = editor.getTargetAtClientPoint(e.clientX, e.clientY);
        const pos = target?.position ?? editor.getPosition() ?? { lineNumber: 1, column: 1 };

        const esc = (s: string) => s.replace(/"/g, '""');
        let sql: string;
        try {
          const columns = await GetTableColumns(info.db, info.schema, info.name);
          const colList = columns.map((c) => `    "${esc(c)}"`).join(",\n");
          sql = `SELECT\n${colList}\nFROM "${esc(info.db)}"."${esc(info.schema)}"."${esc(info.name)}";`;
        } catch {
          sql = `SELECT *\nFROM "${esc(info.db)}"."${esc(info.schema)}"."${esc(info.name)}";`;
        }

        const range = {
          startLineNumber: pos.lineNumber,
          endLineNumber:   pos.lineNumber,
          startColumn:     pos.column,
          endColumn:       pos.column,
        };
        editor.executeEdits("drag-drop", [{ range, text: sql, forceMoveMarkers: true }]);
        editor.pushUndoStop();
        editor.focus();
      });
    }
  };

  // ── Active-statement decoration ──────────────────────────────────────────
  // Highlights the statement currently being executed in a multi-statement run.
  useEffect(() => {
    const dec = activeStmtDecRef.current;
    if (!dec) return;
    if (activeStmtIdx == null) {
      dec.clear();
      return;
    }
    const ranges = getStatementLineRanges(sql);
    const range  = ranges[activeStmtIdx];
    if (!range) {
      dec.clear();
      return;
    }
    dec.set([{
      range: {
        startLineNumber: range.startLine,
        startColumn:     1,
        endLineNumber:   range.endLine,
        endColumn:       1,
      },
      options: {
        isWholeLine:              true,
        className:                "sql-active-stmt-bg",
        linesDecorationsClassName: "sql-active-stmt-indicator",
        overviewRuler: {
          color:    "rgba(210, 153, 34, 0.8)",
          position: 4, // monaco.editor.OverviewRulerLane.Full
        },
      },
    }]);
  }, [activeStmtIdx, sql]);

  return (
  <>
    <Editor
      height="100%"
      defaultLanguage="sql"
      theme={resolved === "dark" ? "thaw-dark" : "thaw-light"}
      value={sql}
      onChange={(v) => setSql(v ?? "")}
      beforeMount={handleBeforeMount}
      onMount={handleMount}
      options={{
        fontSize: editorFontSize,
        fontFamily: editorFont,
        minimap: { enabled: false },
        scrollBeyondLastLine: false,
        lineNumbers: "on",
        renderLineHighlight: "line",
        padding: { top: 12, bottom: 12 },
        wordWrap: "on",
        tabSize: 2,
        automaticLayout: true,
        // Disable Monaco's built-in outline-only selection highlight; we use
        // our own filled-background decorations via refreshOccurrences().
        selectionHighlight: false,
        // Keep Monaco's word-under-cursor highlight for single clicks.
        occurrencesHighlight: "singleFile",
        // Disable Monaco's built-in hover widget; we render our own overlay
        // so we can support scrolling and a copy button.
        hover: { enabled: false },
        folding: true,
        showFoldingControls: "always",
      }}
    />
    {ddlHover && (
      <div
        className="ddl-tooltip"
        tabIndex={0}
        onMouseEnter={() => { isOnTooltipRef.current = true; cancelHide(); }}
        onMouseDown={() => { isMouseDownRef.current = true; }}
        onMouseUp={() => {
          // Save selection now, before a right-click can clear window.getSelection().
          const sel = window.getSelection()?.toString() ?? "";
          if (sel) savedSelRef.current = sel;
        }}
        onMouseLeave={() => {
          isOnTooltipRef.current = false;
          // Don't hide while selecting text or context menu is open.
          if (!isMouseDownRef.current && !isCtxMenuOpenRef.current) setDdlHover(null);
        }}
        onContextMenu={(e) => {
          e.preventDefault();
          isCtxMenuOpenRef.current = true;
          setTooltipCtxMenu({ x: e.clientX, y: e.clientY, sel: savedSelRef.current });
        }}
        style={{
          position: "fixed",
          left: ddlHover.x,
          top: ddlHover.y,
          zIndex: 9999,
          background: "var(--bg-overlay)",
          border: "1px solid var(--border)",
          borderRadius: 6,
          maxWidth: 560,
          maxHeight: 320,
          overflow: "hidden",
          display: "flex",
          flexDirection: "column",
          boxShadow: "0 4px 16px rgba(0,0,0,0.45)",
          fontSize: 12,
        }}
      >
        <div style={{
          display: "flex", justifyContent: "space-between", alignItems: "center",
          padding: "5px 10px", borderBottom: "1px solid var(--border)", flexShrink: 0, gap: 8,
        }}>
          <span style={{ fontWeight: 600, color: "var(--text-primary)" }}>
            {ddlHover.db
              ? `${ddlHover.kind} — ${ddlHover.db}.${ddlHover.schema}.${ddlHover.name}`
              : `${ddlHover.kind} — ${ddlHover.name}`}
          </span>
          <Button size="small" onClick={() => ClipboardSetText(ddlHover.ddl)}>Copy</Button>
        </div>
        <pre style={{
          margin: 0, padding: "8px 10px", fontSize: 12, overflow: "auto",
          flex: 1, minWidth: 0, fontFamily: "monospace", whiteSpace: "pre", userSelect: "text",
          color: "var(--text-primary)",
        }}>
          {ddlHover.ddl}
        </pre>
      </div>
    )}
    {tooltipCtxMenu && (
      <>
        <div
          style={{
            position: "fixed",
            left: tooltipCtxMenu.x,
            top: tooltipCtxMenu.y,
            zIndex: 10001,
            background: "var(--bg-overlay)",
            border: "1px solid var(--border)",
            borderRadius: 4,
            boxShadow: "0 2px 8px rgba(0,0,0,0.35)",
            minWidth: 120,
            padding: "2px 0",
            fontSize: 12,
          }}
        >
          <div
            style={{
              padding: "5px 14px", cursor: "pointer",
              color: tooltipCtxMenu.sel ? "var(--text-primary)" : "var(--text-faint)",
            }}
            onMouseEnter={(e) => { if (tooltipCtxMenu.sel) (e.currentTarget as HTMLElement).style.background = "var(--bg-raised)"; }}
            onMouseLeave={(e) => { (e.currentTarget as HTMLElement).style.background = ""; }}
            onClick={(e) => {
              e.stopPropagation();
              if (tooltipCtxMenu.sel) ClipboardSetText(tooltipCtxMenu.sel);
              savedSelRef.current = "";
              setTooltipCtxMenu(null);
              // Defer so the mouseleave fired when the menu div disappears is
              // still guarded by isCtxMenuOpenRef before we clear it.
              setTimeout(() => { isCtxMenuOpenRef.current = false; }, 50);
            }}
          >
            Copy{tooltipCtxMenu.sel ? "" : " (no selection)"}
          </div>
        </div>
      </>
    )}
  </>
  );
}
