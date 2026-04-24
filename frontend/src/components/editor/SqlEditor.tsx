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
import * as monacoLib from "monaco-editor";
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore – internal Monaco paths; no public type declarations
import { MenuRegistry, MenuId } from "monaco-editor/esm/vs/platform/actions/common/actions.js";
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { CommandsRegistry } from "monaco-editor/esm/vs/platform/commands/common/commands.js";
// eslint-disable-next-line @typescript-eslint/ban-ts-comment
// @ts-ignore
import { ContextKeyExpr } from "monaco-editor/esm/vs/platform/contextkey/common/contextkey.js";
import { ensureMonacoSetup } from "./monacoSetup";
import { setEditorInstance } from "./editorRef";
import { useQueryStore } from "../../store/queryStore";
import { useObjectStore } from "../../store/objectStore";
import { useSessionStore } from "../../store/sessionStore";
import { useThemeStore } from "../../store/themeStore";
import { ClipboardGetText, ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { GetObjectDDL, ListObjects, ListSchemas, GetTableColumns, GetTableForeignKeys, GetTableColumnsWithTypes, GetSchemaForeignKeys, GetUserDDL, GetAISuggestion, GetFunctionSuggestions, GetFunctionTooltip, GetAllFunctionNames, GetEditorPrefs, AnalyzeSqlSyntax, ParseJoinTableRefs, ComputeJoinOnConditions, AnalyzeSqlSemantics, GetScriptingCompletions, GetSqlStatementRanges, GetIdentifierAtColumn, GetActiveFunctionCall, ParseSignatureParams, GetAllDataTypes, ValidateSnowflakePatterns, ValidateDataTypes, ValidateTablesExist, ValidateBareColumnRefs, GetExplainDiagnostics } from "../../../wailsjs/go/main/App";
import type { queryprofile } from "../../../wailsjs/go/models";
import { getSnowflakeSnippets, SNIPPET_CATEGORIES } from "./snowflakeSnippets";
import { DEFAULT_EDITOR_PREFS, EditorPrefs, formatSQL } from "../../utils/sqlFormatter";

// ── Types migrated from sqlDiagnostics.ts ────────────────────────────────────
export interface DiagMarker {
  startLineNumber: number;
  startColumn: number;
  endLineNumber: number;
  endColumn: number;
  message: string;
  severity: number;
}

export interface ColInfo {
  name: string;
  dataType: string;
}

export interface ResolvedRef {
  alias: string;
  db: string;
  schema: string;
  name: string;
}
// ─────────────────────────────────────────────────────────────────────────────

// Module-level DDL cache and hover provider handle so we only register once
// and don't accumulate duplicate providers on editor remounts.
const DDL_CACHE_TTL = 60_000; // ms — stale entries are re-fetched after this
const hoverDDLCache = new Map<string, { ddl: string; ts: number }>();
let hoverProviderDisposable: { dispose(): void } | null = null;
let inlineCompletionsDisposable: { dispose(): void } | null = null;
let signatureHelpDisposable: { dispose(): void } | null = null;

// Module-level editor preferences — updated whenever the user saves new prefs.
let editorPrefsRef: EditorPrefs = { ...DEFAULT_EDITOR_PREFS };

const builtinFns = new Set<string>();
const udfFns     = new Set<string>();
let fnNamesLoaded = false;

const fetchedSchemaObjects   = new Set<string>();
const fetchedDatabaseSchemas = new Set<string>();

// Module-level store for the latest EXPLAIN diagnostic markers. Kept here so
// the custom hover handler (onMouseMove) can look up ExplainData without going
// through Monaco's stripped IMarkerData (which drops unknown fields).
let lastExplainMarkers: queryprofile.ExplainMarker[] = [];

// ── Datatype completion cache ──────────────────────────────────────────────────
// Fetched once from the Go registry (snowflake.AllDataTypes) so the editor and
// the backend validator always share the same type list.
type DataTypeEntry = { Name: string; Kind: number; ParamHint: string };
let cachedDataTypes: DataTypeEntry[] | null = null;
let dataTypesFetchPromise: Promise<void> | null = null;
function ensureDataTypesLoaded(): Promise<void> {
  if (cachedDataTypes !== null) return Promise.resolve();
  if (dataTypesFetchPromise) return dataTypesFetchPromise;
  dataTypesFetchPromise = GetAllDataTypes()
    .then((dts) => { cachedDataTypes = (dts as DataTypeEntry[]) ?? []; })
    .catch(() => { cachedDataTypes = []; dataTypesFetchPromise = null; });
  return dataTypesFetchPromise;
}

const UC = (s: string) => s.toUpperCase();

// ── Column-level completion cache ─────────────────────────────────────────────
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
const fetchedFKSchemas = new Set<string>(); 

async function warmUpFKsForSchema(db: string, schema: string): Promise<void> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}`;
  if (fetchedFKSchemas.has(key)) return;
  fetchedFKSchemas.add(key);
  try {
    const rows = await GetSchemaForeignKeys(db, schema);
    if (!rows) return;
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
    fetchedFKSchemas.delete(key); 
  }
}

function mkColSuggestions(cols: string[], range: any, monaco: any) {
  return cols.map((col) => ({
    label:      col,
    kind:       monaco.languages.CompletionItemKind.Field,
    insertText: col,
    sortText:   "02_" + col,
    detail:     "COLUMN",
    range,
  }));
}

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

function resolveRefs(
  refs: Array<{ db: string; schema: string; name: string; alias: string }>,
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
  activeStmtIdx?: number | null;
}

function applyPrefsToSnippet(text: string, prefs: EditorPrefs): string {
  const indentUnit = prefs.indentStyle === "tabs" ? "\t" : " ".repeat(prefs.indentSize);
  let result = text.replace(/^( {2})+/gm, (m) => indentUnit.repeat(m.length / 2));

  if (prefs.keywordCase !== "Preserve") {
    result = result.replace(/\b([A-Z][A-Z_0-9]*)\b/g, (kw) => {
      switch (prefs.keywordCase) {
        case "lower": return kw.toLowerCase();
        case "Title": return kw.charAt(0) + kw.slice(1).toLowerCase();
        default:      return kw; 
      }
    });
  }

  return result;
}

let _activeSnippetEditor: monacoLib.editor.ICodeEditor | null = null;

/** Shared — called by any Monaco editor (SQL or notebook cell) on context menu open. */
export function setActiveSnippetEditor(editor: monacoLib.editor.ICodeEditor | null): void {
  _activeSnippetEditor = editor;
}

let _snippetMenuRegistered = false;
(() => {
  if (_snippetMenuRegistered) return;
  _snippetMenuRegistered = true;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  let snippetSubMenuId: any;
  try {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    snippetSubMenuId = new (MenuId as any)("thaw.snippets.submenu");
  } catch {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    snippetSubMenuId = (MenuId as any)._instances?.get("thaw.snippets.submenu");
  }
  if (!snippetSubMenuId) return;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (MenuRegistry as any).appendMenuItem((MenuId as any).EditorContext, {
    submenu: snippetSubMenuId,
    title: "SQL Snippets",
    group: "9_snippets",
    order: 0,
    when: ContextKeyExpr.equals("editorLangId", "sql"),
  });

  const snippetItems = getSnowflakeSnippets(monacoLib);
  const snippetMap   = new Map(snippetItems.map((s) => [String(s.label), s]));

  SNIPPET_CATEGORIES.forEach((cat, gi) => {
    cat.labels.forEach((lbl, li) => {
      const s = snippetMap.get(lbl);
      if (!s) return;
      const cmdId = `thaw.snippet.${lbl}`;

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (CommandsRegistry as any).registerCommand(cmdId, () => {
        if (_activeSnippetEditor) {
          // eslint-disable-next-line @typescript-eslint/no-explicit-any
          const ctrl = (_activeSnippetEditor as any).getContribution("snippetController2");
          if (ctrl) ctrl.insert(applyPrefsToSnippet(s.insertText as string, editorPrefsRef));
          _activeSnippetEditor.focus();
        }
      });

      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      (MenuRegistry as any).appendMenuItem(snippetSubMenuId, {
        command: { id: cmdId, title: cat.titles?.[lbl] ?? lbl },
        group: `${gi + 1}`,
        order: li,
      });
    });
  });
})();

export default function SqlEditor({ tabId, activeStmtIdx }: SqlEditorProps = {}) {
  const activeSql       = useQueryStore((s) => s.sql);
  const activeSqlSetter = useQueryStore((s) => s.setSql);
  const tabs            = useQueryStore((s) => s.tabs);
  const setSqlForTab    = useQueryStore((s) => s.setSqlForTab);
  const setSelectedSql  = useQueryStore((s) => s.setSelectedSql);

  const activeTabId = useQueryStore((s) => s.activeTabId);
  const sql    = tabId ? (tabs.find((t) => t.id === tabId)?.sql ?? "") : activeSql;
  const setSql = tabId ? (newSql: string) => setSqlForTab(tabId, newSql) : activeSqlSetter;

  const activeTab      = tabs.find((t) => t.id === (tabId ?? activeTabId));
  const activeKind     = activeTab?.kind;
  const editorLanguage = activeKind === "python" ? "python"
    : activeKind === "yaml"   ? "yaml"
    : "sql";

  const yamlModelPath = editorLanguage === "yaml"
    ? (activeTab?.path
        ? monacoLib.Uri.file(activeTab.path).toString()
        : `file:///untitled-${tabId ?? activeTabId}.yml`)
    : undefined;
  const resolved          = useThemeStore((s) => s.resolved);
  const editorFont        = useThemeStore((s) => s.editorFont);
  const editorFontSize    = useThemeStore((s) => s.editorFontSize);
  const setEditorFontSize = useThemeStore((s) => s.setEditorFontSize);

  const fontSizeRef = useRef(editorFontSize);
  useEffect(() => { fontSizeRef.current = editorFontSize; }, [editorFontSize]);

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const activeStmtDecRef = useRef<any>(null);

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const fnDecRef       = useRef<any>(null);
  const fnDecTimerRef  = useRef<ReturnType<typeof setTimeout> | null>(null);
  const diagTimerRef   = useRef<ReturnType<typeof setTimeout> | null>(null);
  const explainTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [ddlHover, setDdlHover] = useState<DdlHover | null>(null);
  const [tooltipCtxMenu, setTooltipCtxMenu] = useState<{ x: number; y: number; sel: string } | null>(null);
  const hoverTimerRef          = useRef<ReturnType<typeof setTimeout> | null>(null);
  const hoverHideTimerRef      = useRef<ReturnType<typeof setTimeout> | null>(null);
  const yamlHoverAdjustTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const yamlHoverSetTopRef = useRef<number | null>(null);
  const lastHoverWordRef  = useRef<string | null>(null);
  const currentHoverPosRef = useRef<any>(null);
  const currentMouseYRef   = useRef<number>(0);
  const isOnTooltipRef    = useRef(false);
  const isMouseDownRef    = useRef(false);
  const isCtxMenuOpenRef  = useRef(false);
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

  useEffect(() => {
    const handleMouseUp = () => {
      isMouseDownRef.current = false;
      if (!isOnTooltipRef.current && !isCtxMenuOpenRef.current) setDdlHover(null);
    };
    document.addEventListener("mouseup", handleMouseUp);
    return () => document.removeEventListener("mouseup", handleMouseUp);
  }, []);

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

  useEffect(() => {
    if (!tooltipCtxMenu) return;
    const dismiss = () => {
      setTooltipCtxMenu(null);
      setTimeout(() => { isCtxMenuOpenRef.current = false; }, 50);
    };
    document.addEventListener("click", dismiss);
    return () => document.removeEventListener("click", dismiss);
  }, [tooltipCtxMenu]);


  // ── EXPLAIN hover helpers ────────────────────────────────────────────────

  function formatBytes(bytes: number): string {
    if (bytes < 1024) return `${bytes} B`;
    if (bytes < 1_048_576) return `${(bytes / 1024).toFixed(1)} KB`;
    if (bytes < 1_073_741_824) return `${(bytes / 1_048_576).toFixed(1)} MB`;
    return `${(bytes / 1_073_741_824).toFixed(2)} GB`;
  }

  function formatExplainTitle(marker: queryprofile.ExplainMarker): string {
    const d = marker.explainData;
    if (!d) return marker.message.split("\n")[0];
    const shortName = d.objectName ? d.objectName.split(".").pop() ?? d.objectName : "";
    if (shortName) return `${d.operation}: ${shortName}`;
    return d.operation;
  }

  function formatExplainDetails(marker: queryprofile.ExplainMarker): string {
    const d = marker.explainData;
    if (!d) return marker.message;
    const lines: string[] = [];
    if (d.partitionsTotal && d.partitionsTotal > 0) {
      const pct = Math.round(((d.partitionsScanned ?? 0) / d.partitionsTotal) * 100);
      const icon = pct >= 90 ? "⚠️" : "ℹ️";
      lines.push(`${icon} Partitions Scanned: ${d.partitionsScanned} / ${d.partitionsTotal} (${pct}%)`);
    }
    if (d.bytesAssigned && d.bytesAssigned > 0) {
      lines.push(`   Estimated Bytes:   ${formatBytes(d.bytesAssigned)}`);
    }
    if (d.joinType) {
      lines.push(`   Join Type:         ${d.joinType}`);
    }
    if (d.estimatedRows && d.estimatedRows > 0) {
      lines.push(`   Estimated Rows:    ${d.estimatedRows.toLocaleString()}`);
    }
    // Append the tip line from the message (everything after the first line)
    const tip = marker.message.split("\n").slice(1).join("\n").trim();
    if (tip) {
      lines.push("", tip);
    }
    return lines.join("\n");
  }

  // ────────────────────────────────────────────────────────────────────────────

  const handleBeforeMount: BeforeMount = (monaco) => {
    ensureMonacoSetup(monaco);
  };

  const handleMount: OnMount = (editor, monaco) => {
    if (!tabId) {
      setEditorInstance(editor);
      editor.onDidDispose(() => setEditorInstance(null));
    }

    activeStmtDecRef.current = editor.createDecorationsCollection([]);
    fnDecRef.current = editor.createDecorationsCollection([]);

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

    GetEditorPrefs().then((p) => {
      editorPrefsRef = p as EditorPrefs;
    }).catch(() => { /* best-effort */ });

    const handlePrefsChanged = (e: Event) => {
      editorPrefsRef = (e as CustomEvent<EditorPrefs>).detail;
    };
    window.addEventListener("thaw:editor-prefs-changed", handlePrefsChanged);

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

    editor.onDidChangeModelContent(() => {
      if (fnDecTimerRef.current) clearTimeout(fnDecTimerRef.current);
      fnDecTimerRef.current = setTimeout(refreshFnDecorations, 200);
    });

    const runDiagnostics = async () => {
      const model = editor.getModel();
      if (!model) return;
      if (model.getLanguageId() !== "sql") {
        monaco.editor.setModelMarkers(model, "thaw-sql", []);
        return;
      }
      const diagVersion = model.getVersionId();
      const diagSql = model.getValue();
      const diagMarkers: DiagMarker[] = [];

      try {
        // ADD || [] to prevent spreading null from Go's nil slices!
        const syntaxErrors = await AnalyzeSqlSyntax(diagSql);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((syntaxErrors || []) as DiagMarker[]));

        const stmtRanges = (await GetSqlStatementRanges(diagSql)) || [];
        if (model.getVersionId() !== diagVersion) return;

        const patternMarkers = await ValidateSnowflakePatterns(diagSql, stmtRanges);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((patternMarkers || []) as DiagMarker[]));

        const dataTypeMarkers = await ValidateDataTypes(diagSql, stmtRanges);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((dataTypeMarkers || []) as DiagMarker[]));

        const rawRefs = await ParseJoinTableRefs(diagSql);
        if (model.getVersionId() !== diagVersion) return;
        const storeObjs = useObjectStore.getState().objects;

        const storeDbs = useObjectStore.getState().databases;
        const storeSchemas = useObjectStore.getState().schemas;

        const resolved: ResolvedRef[] = (rawRefs || [])
          .map((ref) => {
            if (ref.db && ref.schema) {
              // Stop blindly trusting the AST! Verify against the global cache first.
              if (storeDbs.length > 0 && !storeDbs.some(d => UC(d) === UC(ref.db!))) {
                return null; // The DB is a typo! Drop it so validation catches it.
              }
              // Only check the schema if we have actively fetched schemas for this DB
              if (fetchedDatabaseSchemas.has(UC(ref.db!))) {
                const schemaExists = storeSchemas.some(s => UC(s.db) === UC(ref.db!) && UC(s.name) === UC(ref.schema!));
                if (!schemaExists) return null; // The Schema is a typo!
              }
              // Only check the table if we have actively fetched objects for this Schema
              const schemaKey = `${UC(ref.db!)}\0${UC(ref.schema!)}`;
              if (fetchedSchemaObjects.has(schemaKey)) {
                const tableExists = storeObjs.some(o => UC(o.db) === UC(ref.db!) && UC(o.schema) === UC(ref.schema!) && UC(o.name) === UC(ref.name));
                if (!tableExists) return null; // The Table is a typo!
              }

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
          })
          .filter(Boolean) as ResolvedRef[];

        const tableMarkers = await ValidateTablesExist({
          sql: diagSql,
          stmtRanges,
          resolvedRefs: resolved,
          knownDatabases: storeDbs,
          knownSchemas: storeSchemas,
          quotedIdentifiersIgnoreCase: false,
          droppedDatabases: [],
          droppedSchemas: [],
          droppedTables: [],
        } as any);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((tableMarkers || []) as DiagMarker[]));

        // Build column entries for resolved refs (also used by ValidateBareColumnRefs).
        const colEntries = resolved.map((ref) => {
          const key = `${UC(ref.db)}\0${UC(ref.schema)}\0${UC(ref.name)}`;
          return { db: ref.db, schema: ref.schema, name: ref.name, cols: colInfoCache.get(key) ?? [] };
        });

        for (const ref of resolved) {
          const warmKey = `${UC(ref.db)}\0${UC(ref.schema)}\0${UC(ref.name)}`;
          if (!colInfoCache.has(warmKey) && !fetchingColInfos.has(warmKey)) {
            void getColInfos(ref.db, ref.schema, ref.name).then(() => {
              if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
              diagTimerRef.current = setTimeout(runDiagnostics, 0);
            });
          }
        }

        if (resolved.length > 0) {
          const semanticMarkers = await AnalyzeSqlSemantics(diagSql, resolved as any, colEntries as any);
          if (model.getVersionId() !== diagVersion) return;
          diagMarkers.push(...((semanticMarkers || []) as DiagMarker[]));
        }

        const bareColMarkers = await ValidateBareColumnRefs({
          sql: diagSql,
          stmtRanges,
          resolvedRefs: resolved,
          colEntries,
          quotedIdentifiersIgnoreCase: false,
        } as any);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((bareColMarkers || []) as DiagMarker[]));
        
      } catch (err) {
        console.warn("[thaw] SQL diagnostics aborted:", err);
      } finally {
        if (model.getVersionId() === diagVersion) {
          monaco.editor.setModelMarkers(model, "thaw-sql", diagMarkers);
        }
      }
    };

    // Runs EXPLAIN USING JSON and emits performance markers (full table scans,
    // cartesian joins). Only runs when there are no syntax errors and the SQL
    // is stable (debounced at 2 s to avoid hammering Snowflake Cloud Services).
    const runExplainDiagnostics = async (model: any) => {
      if (!model || model.getLanguageId() !== "sql") return;
      const explainVersion = model.getVersionId();
      const sql = model.getValue().trim();
      if (!sql) return;

      // Skip if syntax checker already found errors.
      const syntaxMarkers = monaco.editor.getModelMarkers({ owner: "thaw-sql", resource: model.uri });
      if (syntaxMarkers.some((m: any) => m.severity === 8)) return;

      try {
        const results = await GetExplainDiagnostics(sql);
        if (model.getVersionId() !== explainVersion) return;
        lastExplainMarkers = results ?? [];
        monaco.editor.setModelMarkers(model, "thaw-explain", lastExplainMarkers.map((m) => ({
          startLineNumber: m.startLineNumber,
          startColumn: m.startColumn,
          endLineNumber: m.endLineNumber,
          endColumn: m.endColumn,
          message: m.message,
          severity: m.severity,
        })));
      } catch {
        // Best-effort — not connected, or SQL is not explainable. Stay silent.
      }
    };

    editor.onDidChangeModelContent(() => {
      if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
      diagTimerRef.current = setTimeout(runDiagnostics, 400);

      // Clear explain markers immediately so stale highlights don't persist.
      const model = editor.getModel();
      if (model) monaco.editor.setModelMarkers(model, "thaw-explain", []);
      lastExplainMarkers = [];
      if (explainTimerRef.current) clearTimeout(explainTimerRef.current);
      explainTimerRef.current = setTimeout(() => runExplainDiagnostics(editor.getModel()), 2000);
    });

    editor.onDidChangeModelLanguage(() => {
      const model = editor.getModel();
      if (model) {
        monaco.editor.setModelMarkers(model, "thaw-sql", []);
        monaco.editor.setModelMarkers(model, "thaw-explain", []);
        lastExplainMarkers = [];
      }
    });

    runDiagnostics();

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

    const trigger = (id: string) => editor.trigger("keyboard", id, null);
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Slash,                      () => trigger("editor.action.commentLine"));
    editor.addCommand(monaco.KeyMod.Shift   | monaco.KeyMod.Alt | monaco.KeyCode.KeyA,   () => trigger("editor.action.blockComment"));
    editor.addCommand(monaco.KeyMod.Shift   | monaco.KeyMod.Alt | monaco.KeyCode.KeyF,   () => trigger("editor.action.formatDocument"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyF,                       () => trigger("actions.find"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyD,                       () => trigger("editor.action.addSelectionToNextFindMatch"));
    editor.addCommand(monaco.KeyMod.WinCtrl | monaco.KeyCode.KeyG,                       () => trigger("editor.action.gotoLine"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyL,                       () => { window.dispatchEvent(new Event("thaw:focus-ai-chat")); });
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.DownArrow,                  () => { window.dispatchEvent(new Event("thaw:focus-results")); });
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyMod.Alt | monaco.KeyCode.UpArrow,   () => trigger("editor.action.insertCursorAbove"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyMod.Alt | monaco.KeyCode.DownArrow, () => trigger("editor.action.insertCursorBelow"));

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

        const lineUpToWord = model
          .getLineContent(position.lineNumber)
          .substring(0, word.startColumn - 1);

        // ── Datatype context ──────────────────────────────────────────────
        // Offer Snowflake type names when the cursor is in a position that
        // syntactically expects a data type:
        //   • x::| — type-cast shorthand
        //   • CAST(x AS |  /  TRY_CAST(x AS |
        //   • DECLARE varname | — Snowflake Scripting variable declaration
        //   • CREATE/ALTER TABLE (..., col_name | — DDL column type
        const isDatatypeContext = (
          /::$/.test(lineUpToWord) ||
          /\b(?:TRY_)?CAST\s*\([^)]*\bAS\s*$/i.test(lineUpToWord) ||
          /\bDECLARE\b[^;]*\b\w+\s*$/i.test(model.getValue().slice(0, model.getOffsetAt(position))) ||
          /\b(?:CREATE|ALTER)\b[^;]*\(\s*(?:.*,\s*)?\w+\s*$/is.test(model.getValue().slice(0, model.getOffsetAt(position)))
        );
        if (isDatatypeContext) {
          await ensureDataTypesLoaded();
          if (cachedDataTypes && cachedDataTypes.length > 0) {
            return {
              suggestions: cachedDataTypes.map((dt, i) => {
                const hasParams = dt.ParamHint !== "";
                return {
                  label:      dt.Name,
                  kind:       monaco.languages.CompletionItemKind.TypeParameter,
                  insertText: hasParams ? `${dt.Name}($1)` : dt.Name,
                  insertTextRules: hasParams
                    ? monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet
                    : 0,
                  detail:    hasParams ? `Type ${dt.ParamHint}` : "Type",
                  sortText:  "00_dt_" + String(i).padStart(3, "0"),
                  range,
                };
              }),
            };
          }
        }
        // ─────────────────────────────────────────────────────────────────

        const threePartMatch = lineUpToWord.match(/\b(\w+)\.(\w+)\.(\w+)\.\s*$/i);
        if (threePartMatch) {
          const [, db, schema, table] = threePartMatch;
          return { suggestions: mkColSuggestions(await getColumns(db, schema, table), range, monaco) };
        }

        const twoPartMatch = lineUpToWord.match(/\b(\w+)\.(\w+)\.\s*$/i);
        if (twoPartMatch) {
          const [, db, schema] = twoPartMatch;
          const schemaKey = `${UC(db)}\0${UC(schema)}`;

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
              fetchedSchemaObjects.delete(schemaKey); 
            }
          }

          return {
            suggestions: useObjectStore.getState().objects
              .filter((o) => UC(o.db) === UC(db) && UC(o.schema) === UC(schema))
              .map((o) => ({
                label:      o.name,
                kind:       monacoKind(monaco, o.kind),
                insertText: o.name,
                sortText:   "03_" + o.name,
                detail:     o.kind,
                range,
              })),
          };
        }

        const onePartMatch = lineUpToWord.match(/\b(\w+)\.\s*$/i);
        if (onePartMatch) {
          const [, qualifier] = onePartMatch;
          const { databases, schemas, objects } = useObjectStore.getState();

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
                  sortText:   "04_" + s.name,
                  detail:     "SCHEMA",
                  range,
                })),
            };
          }

          const schemaObjs = objects.filter((o) => UC(o.schema) === UC(qualifier));
          if (schemaObjs.length > 0) {
            return {
              suggestions: schemaObjs.map((o) => ({
                label:      o.name,
                kind:       monacoKind(monaco, o.kind),
                insertText: o.name,
                sortText:   "03_" + o.name,
                detail:     o.kind,
                range,
              })),
            };
          }

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

        const cursorOffset = model.getOffsetAt(position);
        const textToCursor = model.getValue().slice(0, cursorOffset);

        const isInJoinOnClause = (() => {
          const joinMatches = [...textToCursor.matchAll(/\bJOIN\b/gi)];
          if (joinMatches.length === 0) return false;
          const lastJoin = joinMatches[joinMatches.length - 1];
          const afterLastJoin = textToCursor.slice(lastJoin.index! + lastJoin[0].length);
          const onMatch = afterLastJoin.match(/\bON\b/i);
          if (!onMatch) return false;
          const afterOn = afterLastJoin.slice(onMatch.index! + onMatch[0].length);
          return !/\b(?:JOIN|WHERE|GROUP|ORDER|HAVING|UNION|INTERSECT|EXCEPT)\b/i.test(afterOn);
        })();

        const wordIsOn = word.word.toUpperCase() === "ON";
        if (wordIsOn || isInJoinOnClause) {
          const rawRefs = await ParseJoinTableRefs(textToCursor);
          if (rawRefs && (rawRefs as any[]).length >= 2) {
            const resolvedRefs = resolveRefs(rawRefs as any[], useObjectStore.getState().objects);
            if (resolvedRefs && resolvedRefs.length >= 2) {
              for (const ref of resolvedRefs) {
                warmUpFKsForSchema(ref.db, ref.schema).catch(() => {});
              }
              const fkEntries: any[] = [];
              const colEntries: any[] = [];
              for (const ref of resolvedRefs) {
                fkEntries.push({ db: ref.db, schema: ref.schema, name: ref.name,
                  fks: await getFKs(ref.db, ref.schema, ref.name) });
                colEntries.push({ db: ref.db, schema: ref.schema, name: ref.name,
                  cols: await getColInfos(ref.db, ref.schema, ref.name) });
              }
              const conditions = await ComputeJoinOnConditions({
                resolvedRefs: resolvedRefs as any, fkEntries, colEntries, prefix: "",
              } as any);
              if ((conditions as any[]).length > 0) {
                return {
                  suggestions: (conditions as any[]).map((c: any) =>
                    makeSugg(c.condition, c.detail, c.sortText, range, monaco)),
                };
              }
            }
          }
        }

        {
          const lastJoinSegment = (textToCursor.split(/\bJOIN\b/i).pop() ?? "").trim();
          const rawRefsC = await ParseJoinTableRefs(textToCursor);
          const hasTriggerC =
            lastJoinSegment.length > 0 &&
            !/\b(?:ON|USING)\b/i.test(lastJoinSegment) &&
            rawRefsC && (rawRefsC as any[]).length >= 2;

          if (hasTriggerC) {
            const resolvedC = resolveRefs(rawRefsC as any[], useObjectStore.getState().objects);
            if (resolvedC && resolvedC.length >= 2) {
              const fkEntriesC: any[] = [];
              const colEntriesC: any[] = [];
              for (const ref of resolvedC) {
                fkEntriesC.push({ db: ref.db, schema: ref.schema, name: ref.name,
                  fks: await getFKs(ref.db, ref.schema, ref.name) });
                colEntriesC.push({ db: ref.db, schema: ref.schema, name: ref.name,
                  cols: await getColInfos(ref.db, ref.schema, ref.name) });
              }
              const conditionsC = await ComputeJoinOnConditions({
                resolvedRefs: resolvedC as any, fkEntries: fkEntriesC,
                colEntries: colEntriesC, prefix: "ON ",
              } as any);
              if ((conditionsC as any[]).length > 0) {
                return {
                  suggestions: (conditionsC as any[]).map((c: any) =>
                    makeSugg(c.condition, c.detail, c.sortText, range, monaco)),
                };
              }
            }
          }
        }

        const { databases, schemas, objects } = useObjectStore.getState();
        const fullContent = model.getValue();
        const offset = model.getOffsetAt(position);
        
        const scriptingResult = await GetScriptingCompletions(fullContent, offset);
        const declaredVars: string[] = scriptingResult?.variables ?? [];
        const needsColon: boolean = scriptingResult?.needsColon ?? false;

        const keywordSuggestions = SNOWFLAKE_KEYWORDS.map((kw) => ({
          label:      kw,
          kind:       monaco.languages.CompletionItemKind.Keyword,
          insertText: kw,
          sortText:   "08_" + kw,
          range,
        }));

        const variableSuggestions = declaredVars.map((v) => ({
          label:      needsColon ? ":" + v : v,
          kind:       monaco.languages.CompletionItemKind.Variable,
          insertText: needsColon ? ":" + v : v,
          filterText: needsColon ? ":" + v : v, 
          sortText:   "01_" + v,
          detail:     "SCRIPT VARIABLE",
          range,
        }));

        const dbSuggestions = databases.map((db) => ({
          label:      db,
          kind:       monaco.languages.CompletionItemKind.Module,
          insertText: db,
          sortText:   "05_" + db,
          detail:     "DATABASE",
          range,
        }));

        const schemaSuggestions = schemas.map((s) => ({
          label:      s.name,
          kind:       monaco.languages.CompletionItemKind.Module,
          insertText: s.name,
          sortText:   "04_" + s.name,
          detail:     `SCHEMA · ${s.db}`,
          range,
        }));

        const objectSuggestions = objects.map((o) => ({
          label:      o.name,
          kind:       monacoKind(monaco, o.kind),
          insertText: o.name,
          sortText:   "03_" + o.name,
          detail:     `${o.kind} · ${o.db}.${o.schema}`,
          range,
        }));

        // Isolate the current statement for context columns so tables from other queries don't bleed in
        const ranges = await GetSqlStatementRanges(fullContent);
        let currentStmtText = fullContent;
        for (const r of ranges) {
          if (position.lineNumber >= r.startLine && position.lineNumber <= r.endLine) {
            const lines = model.getLinesContent().slice(r.startLine - 1, r.endLine);
            currentStmtText = lines.join("\n");
            break;
          }
        }

        const seenColKeys = new Set<string>();
        const contextColSuggestions: any[] = [];
        let fetchPending = false;
        
        const rawRefs = await ParseJoinTableRefs(currentStmtText);
        const refsToFetch: {db: string, schema: string, name: string}[] = [];

        for (const ref of (rawRefs || [])) {
          if (ref.db && ref.schema && ref.name) {
            // Fully qualified: trust the name and attempt fetch directly without requiring it in objectStore
            refsToFetch.push({ db: ref.db, schema: ref.schema, name: ref.name });
          } else {
            // Partial: Try to resolve via objects store
            const matchedObjs = objects.filter((o) => {
              if (o.kind !== "TABLE" && o.kind !== "VIEW") return false;
              if (UC(o.name) !== UC(ref.name)) return false;
              if (ref.db && UC(o.db) !== UC(ref.db)) return false;
              if (ref.schema && UC(o.schema) !== UC(ref.schema)) return false;
              return true;
            });
            for (const obj of matchedObjs) {
              refsToFetch.push({ db: obj.db, schema: obj.schema, name: obj.name });
            }
            
            // Fallback to session store defaults if not found in cache
            if (matchedObjs.length === 0) {
              const sess = useSessionStore.getState();
              if (sess.database && sess.schema && ref.name && !ref.db && !ref.schema) {
                refsToFetch.push({ db: sess.database, schema: sess.schema, name: ref.name });
              } else if (sess.database && ref.schema && ref.name && !ref.db) {
                refsToFetch.push({ db: sess.database, schema: ref.schema, name: ref.name });
              }
            }
          }
        }

        for (const ref of refsToFetch) {
          const cacheKey = `${UC(ref.db)}\0${UC(ref.schema)}\0${UC(ref.name)}`;
          if (columnCache.has(cacheKey)) {
            for (const col of columnCache.get(cacheKey)!) {
              if (!seenColKeys.has(UC(col))) {
                seenColKeys.add(UC(col));
                contextColSuggestions.push({
                  label:      col,
                  kind:       monaco.languages.CompletionItemKind.Field,
                  insertText: col,
                  sortText:   "02_" + col,
                  detail:     `COLUMN · ${ref.name}`,
                  range,
                });
              }
            }
          } else {
            getColumns(ref.db, ref.schema, ref.name);
            fetchPending = true;
          }
        }

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
                sortText:         fn.functionType === "UDF" ? "06_" + fn.functionName : "07_" + fn.functionName,
                range,
              }));
            }
          } catch {
            // best-effort
          }
        }

        return {
          suggestions: [
            ...variableSuggestions, 
            ...contextColSuggestions, 
            ...keywordSuggestions, 
            ...dbSuggestions, 
            ...schemaSuggestions, 
            ...objectSuggestions, 
            ...fnSuggestions
          ],
          incomplete: fetchPending,
        };
      },
    });

    if (hoverProviderDisposable) {
      hoverProviderDisposable.dispose();
      hoverProviderDisposable = null;
    }

    editor.onMouseMove((e: any) => {
      currentMouseYRef.current = (e.event as any).posy ?? 0;

      const model = editor.getModel();
      if (model?.getLanguageId() === "yaml") {
        if (yamlHoverSetTopRef.current !== null) {
          const dom = editor.getDomNode();
          const hoverEl = (
            dom?.parentElement?.querySelector(".monaco-resizable-hover") ??
            dom?.querySelector(".monaco-resizable-hover") ??
            document.querySelector(".monaco-resizable-hover")
          ) as HTMLElement | null;

          const currentStyleTop = hoverEl ? (parseFloat(hoverEl.style.top) || 0) : null;
          if (currentStyleTop !== null && Math.abs(currentStyleTop - yamlHoverSetTopRef.current) < 5) {
            return;
          }
          yamlHoverSetTopRef.current = null;
        }

        if (yamlHoverAdjustTimerRef.current) clearTimeout(yamlHoverAdjustTimerRef.current);

        const tryAdjust = (attemptsLeft: number) => {
          const dom = editor.getDomNode();
          const hoverEl = (
            dom?.parentElement?.querySelector(".monaco-resizable-hover") ??
            dom?.querySelector(".monaco-resizable-hover") ??
            document.querySelector(".monaco-resizable-hover")
          ) as HTMLElement | null;

          const isVisible = hoverEl
            && hoverEl.style.display !== "none"
            && parseFloat(hoverEl.style.top) >= -500;

          if (!isVisible || !hoverEl) {
            if (attemptsLeft > 0)
              yamlHoverAdjustTimerRef.current = setTimeout(() => tryAdjust(attemptsLeft - 1), 50);
            return;
          }

          const rect = hoverEl.getBoundingClientRect();
          if (rect.width === 0 || rect.height === 0) {
            if (attemptsLeft > 0)
              yamlHoverAdjustTimerRef.current = setTimeout(() => tryAdjust(attemptsLeft - 1), 50);
            return;
          }

          const mouseY = currentMouseYRef.current;
          const CLEAR = 24;
          const desiredTop = mouseY + CLEAR + rect.height <= window.innerHeight
            ? mouseY + CLEAR
            : Math.max(0, mouseY - CLEAR - rect.height);
          if (Math.abs(rect.top - desiredTop) < 2) {
            yamlHoverSetTopRef.current = parseFloat(hoverEl.style.top) || 0;
            return;
          }
          const styleTop = parseFloat(hoverEl.style.top) || 0;
          const newStyleTop = styleTop + (desiredTop - rect.top);
          hoverEl.style.top = `${newStyleTop}px`;
          yamlHoverSetTopRef.current = newStyleTop;
        };

        yamlHoverAdjustTimerRef.current = setTimeout(() => tryAdjust(12), 50);
        return;
      }

      if (model && model.getLanguageId() !== "sql") return;

      void (async () => {
      const pos = e.target?.position;
      const partsRaw = (pos && model)
        ? await GetIdentifierAtColumn(model.getLineContent(pos.lineNumber), pos.column - 1)
        : null;
      const parts = (partsRaw && partsRaw.length > 0) ? partsRaw : null;

      const diagMarkerAtPos = (pos && model)
        ? (monaco.editor.getModelMarkers({ owner: "thaw-sql", resource: model.uri }).find((m: any) =>
            pos.lineNumber >= m.startLineNumber && pos.lineNumber <= m.endLineNumber &&
            pos.column    >= m.startColumn      && pos.column    <= m.endColumn,
          ) ?? null)
        : null;

      const explainMarkerAtPos = pos
        ? (lastExplainMarkers.find((m) =>
            pos.lineNumber >= m.startLineNumber && pos.lineNumber <= m.endLineNumber &&
            pos.column    >= m.startColumn      && pos.column    <= m.endColumn,
          ) ?? null)
        : null;

      if ((!parts || parts.length === 0) && !diagMarkerAtPos && !explainMarkerAtPos) {
        lastHoverWordRef.current = null;
        if (hoverTimerRef.current) { clearTimeout(hoverTimerRef.current); hoverTimerRef.current = null; }
        if (!isOnTooltipRef.current) scheduleHide();
        return;
      }

      cancelHide();
      currentHoverPosRef.current = pos;

      const wordKey = (parts && parts.length > 0)
        ? parts.join("\0")
        : diagMarkerAtPos
          ? `marker:${diagMarkerAtPos.startLineNumber}:${diagMarkerAtPos.startColumn}`
          : `explain:${explainMarkerAtPos!.startLineNumber}:${explainMarkerAtPos!.startColumn}`;
      if (wordKey === lastHoverWordRef.current) return;
      lastHoverWordRef.current = wordKey;

      if (hoverTimerRef.current) clearTimeout(hoverTimerRef.current);

      hoverTimerRef.current = setTimeout(async () => {
        if (lastHoverWordRef.current !== wordKey) return;
        const pos = currentHoverPosRef.current;
        if (!pos) return;

        {
          const mModel = editor.getModel();
          if (mModel) {
            // ── EXPLAIN markers (richer tooltip with structured data) ───────
            const explainMarker = lastExplainMarkers.find((m) =>
              pos.lineNumber >= m.startLineNumber && pos.lineNumber <= m.endLineNumber &&
              pos.column    >= m.startColumn      && pos.column    <= m.endColumn,
            );
            if (explainMarker) {
              const editorDom = editor.getDomNode();
              const editorRect = editorDom?.getBoundingClientRect();
              const scrolledPos = editor.getScrolledVisiblePosition(pos);
              if (scrolledPos && editorRect) {
                const rawX = editorRect.left + scrolledPos.left;
                const mouseY = currentMouseYRef.current;
                const details = formatExplainDetails(explainMarker);
                const fitsBelow = mouseY + 24 + (details ? 120 : 60) <= window.innerHeight;
                const x = Math.min(rawX, window.innerWidth - 570);
                const y = fitsBelow ? mouseY + 24 : Math.max(0, mouseY - 24 - (details ? 120 : 60));
                if (hoverHideTimerRef.current) { clearTimeout(hoverHideTimerRef.current); hoverHideTimerRef.current = null; }
                setDdlHover({
                  kind: explainMarker.severity === 8 ? "⚠️ Performance Issue" : "💡 Performance Tip",
                  db: "", schema: "",
                  name: formatExplainTitle(explainMarker),
                  ddl: details,
                  x, y,
                });
              }
              return;
            }

            // ── Syntax / validation markers (plain message) ─────────────────
            const diagMarker = monaco.editor.getModelMarkers({ owner: "thaw-sql", resource: mModel.uri }).find((m: any) =>
              pos.lineNumber >= m.startLineNumber && pos.lineNumber <= m.endLineNumber &&
              pos.column    >= m.startColumn      && pos.column    <= m.endColumn,
            );
            if (diagMarker) {
              const editorDom = editor.getDomNode();
              const editorRect = editorDom?.getBoundingClientRect();
              const scrolledPos = editor.getScrolledVisiblePosition(pos);
              if (scrolledPos && editorRect) {
                const rawX = editorRect.left + scrolledPos.left;
                const mouseY = currentMouseYRef.current;
                const fitsBelow = mouseY + 24 + 80 <= window.innerHeight;
                const x = Math.min(rawX, window.innerWidth - 570);
                const y = fitsBelow ? mouseY + 24 : Math.max(0, mouseY - 24 - 80);
                if (hoverHideTimerRef.current) { clearTimeout(hoverHideTimerRef.current); hoverHideTimerRef.current = null; }
                setDdlHover({
                  kind: diagMarker.severity === 8 ? "ERROR" : "WARNING",
                  db: "", schema: "",
                  name: diagMarker.message,
                  ddl: "",
                  x, y,
                });
              }
              return;
            }
          }
        }

        if (!parts || parts.length === 0) return;

        const { objects } = useObjectStore.getState();
        let db = "", schema = "", kind = "", name = "", ddl = "";

        if (parts.length === 2) {
          const rawRefs = await ParseJoinTableRefs(editor.getModel()?.getValue() ?? "");
          const resolved = resolveRefs((rawRefs || []) as any[], useObjectStore.getState().objects);
          const matchedTable = resolved?.find(
            (r) => r.alias.toUpperCase() === parts[0].toUpperCase(),
          );
          if (matchedTable) {
            const cacheKey = `${UC(matchedTable.db)}\0${UC(matchedTable.schema)}\0${UC(matchedTable.name)}`;
            const cols = colInfoCache.get(cacheKey);
            const col = cols?.find((c) => c.name.toUpperCase() === parts[1].toUpperCase());
            if (col) {
              const editorDom = editor.getDomNode();
              const editorRect = editorDom?.getBoundingClientRect();
              const scrolledPos = editor.getScrolledVisiblePosition(pos);
              if (scrolledPos && editorRect) {
                const rawX = editorRect.left + scrolledPos.left;
                const mouseY = currentMouseYRef.current;
                const fitsBelow = mouseY + 24 + 320 <= window.innerHeight;
                const x = Math.min(rawX, window.innerWidth - 570);
                const y = fitsBelow ? mouseY + 24 : Math.max(0, mouseY - 24 - 320);
                if (hoverHideTimerRef.current) { clearTimeout(hoverHideTimerRef.current); hoverHideTimerRef.current = null; }
                setDdlHover({
                  kind: "COLUMN",
                  db: matchedTable.db,
                  schema: matchedTable.schema,
                  name: `${matchedTable.name}.${col.name}`,
                  ddl: col.dataType,
                  x, y,
                });
              }
              return; 
            }
          }
        }

        if (parts.length >= 3) {
          const [pDb, pSchema, pName] = [
            parts[parts.length - 3],
            parts[parts.length - 2],
            parts[parts.length - 1],
          ];
          const schemaKey = `${UC(pDb)}\0${UC(pSchema)}`;
          const hasSchemaInStore = useObjectStore.getState().objects
            .some((o) => UC(o.db) === UC(pDb) && UC(o.schema) === UC(pSchema));
          if (!hasSchemaInStore && !fetchedSchemaObjects.has(schemaKey)) {
            fetchedSchemaObjects.add(schemaKey);
            try {
              const fetched = await ListObjects(UC(pDb), UC(pSchema));
              useObjectStore.getState().addObjects(
                UC(pDb), UC(pSchema),
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
          const [qualifier, pName] = [parts[0], parts[1]];
          let inStore = objects.find(
            (o) => UC(o.schema) === UC(qualifier) && UC(o.name) === UC(pName) &&
                   (o.kind === "TABLE" || o.kind === "VIEW"),
          );
          if (!inStore) {
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
            try {
              // Prevent keywords (REPLACE, LEFT, RIGHT) from showing function tooltips 
              // when they are being used as SQL keywords rather than function calls.
              const wordInfo = model?.getWordAtPosition(pos);
              if (wordInfo) {
                const textAfterWord = model?.getLineContent(pos.lineNumber).substring(wordInfo.endColumn - 1);
                const isFollowedByParen = /^\s*\(/.test(textAfterWord || "");
                
                // Snowflake has a few parameterless context functions that don't need parentheses
                const upperWord = parts[0].toUpperCase();
                const isContextFn = upperWord.startsWith("CURRENT_") || upperWord === "LOCALTIMESTAMP";
                
                if (!isFollowedByParen && !isContextFn) {
                  setDdlHover(null);
                  return;
                }
              }
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
                  const rawX2 = editorRect2.left + scrolledPos2.left;
                  const mouseY2 = currentMouseYRef.current;
                  const fitsBelow2 = mouseY2 + 24 + 320 <= window.innerHeight;
                  const fnX = Math.min(rawX2, window.innerWidth - 570);
                  const fnY = fitsBelow2 ? mouseY2 + 24 : Math.max(0, mouseY2 - 24 - 320);
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

        const rawX    = editorRect.left + scrolledPos.left;
        const mouseY  = currentMouseYRef.current;
        const fitsBelow = mouseY + 24 + 320 <= window.innerHeight;
        const x = Math.min(rawX, window.innerWidth - 570);
        const y = fitsBelow ? mouseY + 24 : Math.max(0, mouseY - 24 - 320);

        if (hoverHideTimerRef.current) { clearTimeout(hoverHideTimerRef.current); hoverHideTimerRef.current = null; }
        setDdlHover({ ddl, kind, db, schema, name, x, y });
      }, 200);
      })();
    });

    editor.onMouseLeave(() => {
      lastHoverWordRef.current = null;
      yamlHoverSetTopRef.current = null; 
      if (hoverTimerRef.current) clearTimeout(hoverTimerRef.current);
      if (!isOnTooltipRef.current) scheduleHide();
    });

    if (!inlineCompletionsDisposable) {
      inlineCompletionsDisposable = monaco.languages.registerInlineCompletionsProvider("sql", {
        provideInlineCompletions: async (model: any, position: any, _ctx: any, token: any) => {
          const prefixFull = model.getValue().slice(0, model.getOffsetAt(position));
          const lastJoinSeg = (prefixFull.split(/\bJOIN\b/i).pop() ?? "").trim();
          if (lastJoinSeg.length > 0 && !/\b(?:ON|USING)\b/i.test(lastJoinSeg)) {
            const ghostRefs = await ParseJoinTableRefs(prefixFull);
            if (ghostRefs && (ghostRefs as any[]).length >= 2) {
              const resolved = resolveRefs(ghostRefs as any[], useObjectStore.getState().objects);
              if (resolved && resolved.length >= 2) {
                const fkEntries = resolved.map((ref) => ({
                  db: ref.db, schema: ref.schema, name: ref.name,
                  fks: fkCache.get(`${UC(ref.db)}\0${UC(ref.schema)}\0${UC(ref.name)}`) ?? [],
                }));
                const colEntries = resolved.map((ref) => ({
                  db: ref.db, schema: ref.schema, name: ref.name,
                  cols: colInfoCache.get(`${UC(ref.db)}\0${UC(ref.schema)}\0${UC(ref.name)}`) ?? [],
                }));
                const conds = await ComputeJoinOnConditions({
                  resolvedRefs: resolved as any, fkEntries: fkEntries as any,
                  colEntries: colEntries as any, prefix: "ON ",
                } as any);
                if ((conds as any[]).length > 0 && !token.isCancellationRequested) {
                  return { items: [{ insertText: (conds as any[])[0].condition }] };
                }
              }
            }
          }

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

    if (!signatureHelpDisposable) {
      signatureHelpDisposable = monaco.languages.registerSignatureHelpProvider("sql", {
        signatureHelpTriggerCharacters:   ["(", ","],
        signatureHelpRetriggerCharacters: [","],
        provideSignatureHelp: async (model: any, position: any, _token: any, context: any) => {
          const prefix = model.getValueInRange({
            startLineNumber: 1, startColumn: 1,
            endLineNumber:   position.lineNumber, endColumn: position.column,
          });

          const call = await GetActiveFunctionCall(prefix);
          if (!call) return null;

          let overloads: any[] | null = null;
          try { overloads = await GetFunctionTooltip(call.name); } catch { return null; }
          if (!overloads || overloads.length === 0) return null;

          const sigParamsList = await Promise.all(
            overloads.map((fn: any) => ParseSignatureParams(fn.functionSignature))
          );
          const signatures = overloads.map((fn: any, idx: number) => ({
            label:         fn.functionSignature,
            documentation: fn.description ? { value: fn.description } : undefined,
            parameters:    (sigParamsList[idx] ?? []).map((p) => ({ label: [p.start, p.end] as [number, number] })),
          }));

          let activeSignature = context?.activeSignatureHelp?.activeSignature ?? 0;
          if (!context?.activeSignatureHelp) {
            let best = Infinity;
            signatures.forEach((sig: any, i: number) => {
              const n = sig.parameters.length;
              if (n >= call.paramIndex && n < best) { best = n; activeSignature = i; }
            });
          }

          return {
            value:   { signatures, activeSignature, activeParameter: call.paramIndex },
            dispose: () => {},
          };
        },
      });
    }

    const occurrences = editor.createDecorationsCollection([]);

    const refreshOccurrences = () => {
      const selection = editor.getSelection();
      const model     = editor.getModel();

      if (!model || !selection || selection.isEmpty()) {
        occurrences.clear();
        return;
      }

      const selectedText = model.getValueInRange(selection);

      if (selectedText.trim().length < 2) {
        occurrences.clear();
        return;
      }

      const matches = model.findMatches(
        selectedText,
        true,   
        false,  
        true,   
        null,   
        false,  
      );

      occurrences.set(
        matches
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

    editor.onDidChangeCursorSelection(() => {
      const selection = editor.getSelection();
      const selected  = selection && !selection.isEmpty()
        ? editor.getModel()?.getValueInRange(selection) ?? ""
        : "";
      setSelectedSql(selected);
      refreshOccurrences();
    });

    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyCode.Enter,
      () => window.dispatchEvent(new CustomEvent("run-query"))
    );

    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyS,
      () => window.dispatchEvent(new CustomEvent("save-file"))
    );

    editor.onContextMenu(() => { _activeSnippetEditor = editor; });
    editor.onDidDispose(() => {
      if (_activeSnippetEditor === editor) _activeSnippetEditor = null;
    });

    editor.addAction({
      id: "thaw.toggleLineComment",
      label: "Toggle Line Comment",
      contextMenuGroupId: "1_modification",
      contextMenuOrder: 1,
      run: (ed) => ed.trigger("keyboard", "editor.action.commentLine", null),
    });

    editor.addAction({
      id: "thaw.formatSQL",
      label: "Format SQL",
      contextMenuGroupId: "1_modification",
      contextMenuOrder: 2,
      keybindings: [monacoLib.KeyMod.Shift | monacoLib.KeyMod.Alt | monacoLib.KeyCode.KeyF],
      run: async (ed) => {
        const model = ed.getModel();
        if (!model) return;
        const selection = ed.getSelection();
        const hasSelection = selection && !selection.isEmpty();

        if (hasSelection && selection) {
          const original = model.getValueInRange(selection);
          const formatted = await formatSQL(original, editorPrefsRef);
          if (formatted !== original) {
            ed.executeEdits("thaw.formatSQL", [{
              range: selection,
              text: formatted,
              forceMoveMarkers: true,
            }]);
          }
        } else {
          const original = model.getValue();
          const formatted = await formatSQL(original, editorPrefsRef);
          if (formatted !== original) {
            const fullRange = model.getFullModelRange();
            ed.executeEdits("thaw.formatSQL", [{
              range: fullRange,
              text: formatted,
              forceMoveMarkers: true,
            }]);
          }
        }
      },
    });

    const handleScrollToLine = (e: Event) => {
      const { line, matchStart, matchEnd } =
        (e as CustomEvent<{ line: number; matchStart?: number; matchEnd?: number }>).detail;
      if (typeof line !== "number") return;
      editor.revealLineInCenter(line);
      if (typeof matchStart === "number" && typeof matchEnd === "number") {
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

    const editorDom = editor.getDomNode();
    if (editorDom) {
      editorDom.addEventListener("keydown", (e: KeyboardEvent) => {
        if (!(e.metaKey || e.ctrlKey)) return;
        if (e.key === "+" || e.key === "=") {
          e.preventDefault();
          setEditorFontSize(Math.min(fontSizeRef.current + 1, 32));
          return;
        }
        if (e.key === "-") {
          e.preventDefault();
          setEditorFontSize(Math.max(fontSizeRef.current - 1, 8));
          return;
        }
        if (e.key === "0") {
          e.preventDefault();
          setEditorFontSize(14);
        }
      });

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

  useEffect(() => {
    const dec = activeStmtDecRef.current;
    if (!dec) return;
    if (activeStmtIdx == null) {
      dec.clear();
      return;
    }
    void (async () => {
      const ranges = await GetSqlStatementRanges(sql);
      const range  = ranges[activeStmtIdx];
      if (!range) { dec.clear(); return; }
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
            position: 4,
          },
        },
      }]);
    })();
  }, [activeStmtIdx, sql]);

  return (
  <>
    <Editor
      height="100%"
      language={editorLanguage}
      theme={resolved === "dark" ? "thaw-dark" : "thaw-light"}
      path={yamlModelPath}
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
        selectionHighlight: false,
        occurrencesHighlight: "singleFile",
        hover: { enabled: editorLanguage === "yaml" },
        quickSuggestions: editorLanguage === "yaml"
          ? { other: true, comments: false, strings: true }
          : { other: true, comments: false, strings: false },
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
          const sel = window.getSelection()?.toString() ?? "";
          if (sel) savedSelRef.current = sel;
        }}
        onMouseLeave={() => {
          isOnTooltipRef.current = false;
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
          padding: "5px 10px", borderBottom: ddlHover.ddl ? "1px solid var(--border)" : "none",
          flexShrink: 0, gap: 8,
        }}>
          <span style={{ fontWeight: 600, color: "var(--text-primary)" }}>
            {ddlHover.db
              ? `${ddlHover.kind} — ${ddlHover.db}.${ddlHover.schema}.${ddlHover.name}`
              : `${ddlHover.kind} — ${ddlHover.name}`}
          </span>
          {ddlHover.ddl && <Button size="small" onClick={() => ClipboardSetText(ddlHover.ddl)}>Copy</Button>}
        </div>
        {ddlHover.ddl && (
          <pre style={{
            margin: 0, padding: "8px 10px", fontSize: 12, overflow: "auto",
            flex: 1, minWidth: 0, fontFamily: "monospace", whiteSpace: "pre", userSelect: "text",
            color: "var(--text-primary)",
          }}>
            {ddlHover.ddl}
          </pre>
        )}
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