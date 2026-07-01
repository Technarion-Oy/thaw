// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.
//
// @thaw-domain: SQL Editor & Diagnostics

import { useState, useRef, useCallback, useEffect } from "react";
import { Button } from "antd";
import Editor, { type BeforeMount, type OnMount } from "@monaco-editor/react";
// Slim editor API only (no language services) — see monacoSetup.ts for why.
import * as monacoLib from "monaco-editor/esm/vs/editor/editor.api.js";
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
import { useFeatureFlagsStore } from "../../store/featureFlagsStore";
import { patchMonacoClipboard } from "../../utils/monacoClipboard";
import { ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { GetObjectDDL, ListObjects, ListSchemas, GetTableColumns, GetTableColumnsWithTypes, GetSchemaForeignKeys, GetUserDDL, GetAISuggestion, GetFunctionSuggestions, GetFunctionTooltip, GetAllFunctionNames, GetEditorPrefs, GitGetHeadFileContent } from "../../../wailsjs/go/app/App";
import { SNOWFLAKE_DATA_TYPES } from "../../generated/snowflakeDataTypes";
import { AnalyzeSqlSyntax, ParseJoinTableRefs, ComputeJoinOnConditions, AnalyzeSqlSemantics, GetSqlStatementRanges, GetIdentifierAtColumn, GetActiveFunctionCall, ParseSignatureParams, ValidateDataTypes, ValidateGrammar, ValidateAntiPatterns, ValidateTablesExist, ValidateBareColumnRefs, GetSnowflakeKeywords, GetAutocompleteContextFull, ResolveTableRefs, ComputeGitLineDiff } from "../../../wailsjs/go/sqleditor/Service";
import { getSnowflakeSnippets, SNIPPET_CATEGORIES } from "./snowflakeSnippets";
import { UC, quoteIfNecessary, getFKs, getFKsCached, setFKCache, clearFKCache, currentCacheGeneration, bumpCacheGeneration, FKEntry, buildVariableSuggestions } from "./sqlEditorUtils";
import ExplainModal from "../results/ExplainModal";
import { DEFAULT_EDITOR_PREFS, EditorPrefs, formatSQL } from "../../utils/sqlFormatter";

// ── Types migrated from sqlDiagnostics.ts ────────────────────────────────────
export interface DiagMarker {
  startLineNumber: number;
  startColumn: number;
  endLineNumber: number;
  endColumn: number;
  message: string;
  severity: number;
  code?: string;
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
// Module-level map used to pass pre-computed diagnostics from MCP open_sql_tab
// to the editor without going through React state. QueryPage writes into it
// when the Wails event fires; onDidChangeModelContent reads and clears.
//
// Timing: the write (EventsOn callback) and read (onDidChangeModelContent) run
// in different React lifecycles, but Monaco fires onDidChangeModelContent when
// the new tab's model receives its initial SQL content (setValue during mount),
// so the markers are always consumed. Cleanup on tab close (requestClose in
// QueryPage) prevents leaks for tabs closed before their editor mounts.
export const pendingMcpMarkers = new Map<string, DiagMarker[]>();

// ─────────────────────────────────────────────────────────────────────────────

// Module-level DDL cache and hover provider handle so we only register once
// and don't accumulate duplicate providers on editor remounts.
const DDL_CACHE_TTL = 60_000; // ms — stale entries are re-fetched after this
const hoverDDLCache = new Map<string, { ddl: string; ts: number }>();
let hoverProviderDisposable: { dispose(): void } | null = null;
let inlineCompletionsDisposable: { dispose(): void } | null = null;
let signatureHelpDisposable: { dispose(): void } | null = null;
let codeActionProviderDisposable: { dispose(): void } | null = null;

// Module-level editor preferences — updated whenever the user saves new prefs.
let editorPrefsRef: EditorPrefs = { ...DEFAULT_EDITOR_PREFS };

const builtinFns = new Set<string>();
const udfFns     = new Set<string>();
let fnNamesLoaded = false;

const fetchedSchemaObjects   = new Set<string>();
const fetchedDatabaseSchemas = new Set<string>();

// ── Git gutter: HEAD content cache ────────────────────────────────────────────
// Maps absolute file path → HEAD content string (empty = new file).
const headContentCache = new Map<string, string>();

// Maximum lines to diff — beyond this, gutter decorators are skipped to
// avoid O(H×C) DP memory / time blowup on very large files.
const MAX_DIFF_LINES = 3000;

// computeGitLineDiff — moved to Go backend (sqleditor.ComputeGitLineDiff).

// ── Datatype completion source ──────────────────────────────────────────────────
// The type list is static, so it is imported synchronously from the generated
// artifact (SNOWFLAKE_DATA_TYPES) whose single source of truth is the Go
// registry (snowflake.AllDataTypes).  No IPC call is needed — the editor and the
// backend validator share the same list at build time.

let snowflakeKeywords: Set<string> | null = null;
let snowflakeKeywordsArray: string[] = [];
let keywordsFetchPromise: Promise<void> | null = null;
function ensureKeywordsLoaded(): Promise<void> {
  if (snowflakeKeywords !== null) return Promise.resolve();
  if (keywordsFetchPromise) return keywordsFetchPromise;
  keywordsFetchPromise = (async () => {
    try {
      const kws = await GetSnowflakeKeywords();
      snowflakeKeywordsArray = (kws as string[]) ?? [];
      snowflakeKeywords = new Set(snowflakeKeywordsArray.map(k => k.toUpperCase()));
    } catch {
      snowflakeKeywords = new Set();
      snowflakeKeywordsArray = [];
    } finally {
      keywordsFetchPromise = null;
    }
  })();
  return keywordsFetchPromise;
}

// quoteIfNecessary is imported from sqlEditorUtils.ts — local wrapper for convenience.
const quoteIfNec = (name: string) => quoteIfNecessary(name, snowflakeKeywords);

// ── Column-level completion cache ─────────────────────────────────────────────
const columnCache  = new Map<string, string[]>();
const fetchingCols = new Set<string>();

async function getColumns(db: string, schema: string, table: string): Promise<string[]> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  if (columnCache.has(key)) return columnCache.get(key)!;
  if (fetchingCols.has(key)) return [];
  fetchingCols.add(key);
  const gen = currentCacheGeneration();
  try {
    const cols = await GetTableColumns(db, schema, table);
    if (gen === currentCacheGeneration()) columnCache.set(key, cols ?? []);
    return cols ?? [];
  } catch {
    if (gen === currentCacheGeneration()) columnCache.set(key, []);
    return [];
  } finally {
    fetchingCols.delete(key);
  }
}

// FK cache for JOIN ON autocomplete — imported from sqlEditorUtils.ts

// ── ColInfo cache for type-compatible JOIN ON suggestions ─────────────────────
const colInfoCache   = new Map<string, ColInfo[]>();
const fetchingColInfos = new Set<string>();

async function getColInfos(db: string, schema: string, table: string): Promise<ColInfo[]> {
  const key = `${db.toUpperCase()}\0${schema.toUpperCase()}\0${table.toUpperCase()}`;
  if (colInfoCache.has(key)) return colInfoCache.get(key)!;
  if (fetchingColInfos.has(key)) return [];
  fetchingColInfos.add(key);
  const gen = currentCacheGeneration();
  try {
    const cols = await GetTableColumnsWithTypes(db, schema, table);
    const entries: ColInfo[] = (cols ?? []).map((c: any) => ({
      name:     c.name     ?? "",
      dataType: c.dataType ?? "",
    }));
    if (gen === currentCacheGeneration()) colInfoCache.set(key, entries);
    return entries;
  } catch {
    if (gen === currentCacheGeneration()) colInfoCache.set(key, []);
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
      setFKCache(k, entries);
    }
  } catch {
    fetchedFKSchemas.delete(key); 
  }
}

// clearMetadataCaches drops every cached Snowflake catalog lookup the editor uses
// for autocomplete and diagnostics — table columns, column types, foreign keys,
// and the "already fetched" markers for schema object lists / database schemas /
// per-schema FK warm-ups / hover DDL. Call it whenever the catalog may have
// changed underneath us: after a statement is executed (DDL can add/drop/alter
// objects and columns) or when the object store is explicitly refreshed. The next
// autocomplete/diagnostics pass then re-fetches fresh metadata on demand.
//
// Function-name, keyword, and git-HEAD caches are intentionally left alone — they
// are not part of the live catalog and are not affected by running SQL.
export function clearMetadataCaches(): void {
  // Bump the generation first so any fetch already in flight discards its
  // now-stale result instead of repopulating the just-cleared cache.
  bumpCacheGeneration();
  columnCache.clear();
  fetchingCols.clear();
  colInfoCache.clear();
  fetchingColInfos.clear();
  fetchedSchemaObjects.clear();
  fetchedDatabaseSchemas.clear();
  fetchedFKSchemas.clear();
  hoverDDLCache.clear();
  clearFKCache();
}

function mkColSuggestions(cols: string[], range: any, monaco: any) {
  return cols.map((col) => ({
    label:      col,
    kind:       monaco.languages.CompletionItemKind.Field,
    insertText: quoteIfNec(col),
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

// resolveRefs has been moved to the backend (sqleditor.ResolveTableRefs).
// Use the `ResolveTableRefs` IPC method from wailsjs/go/sqleditor/Service.

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

// ── "Explain SQL" context menu item ──────────────────────────────────────────
// Registered once at module load. The command dispatches a custom event so the
// React component can handle async statement detection and show the modal.
let _explainMenuRegistered = false;
(() => {
  if (_explainMenuRegistered) return;
  _explainMenuRegistered = true;

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (CommandsRegistry as any).registerCommand("thaw.explain.sql", () => {
    if (!_activeSnippetEditor) return;
    const editor = _activeSnippetEditor;
    const selection = editor.getSelection();
    const model = editor.getModel();
    const selectedText =
      selection && !selection.isEmpty() ? (model?.getValueInRange(selection) ?? null) : null;
    window.dispatchEvent(
      new CustomEvent("thaw:explain-sql", {
        detail: {
          selectedText,
          fullSql: model?.getValue() ?? "",
          cursorLine: editor.getPosition()?.lineNumber ?? 1,
        },
      })
    );
  });

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  (MenuRegistry as any).appendMenuItem((MenuId as any).EditorContext, {
    command: { id: "thaw.explain.sql", title: "Explain SQL" },
    group: "z_thaw_explain",
    order: 0,
    when: ContextKeyExpr.equals("editorLangId", "sql"),
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
    : activeKind === "plaintext" ? "plaintext"
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
  const fnDecRef          = useRef<any>(null);
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const gitGutterDecRef   = useRef<any>(null);
  const gitGutterTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  // Ref holding the current file path — updated synchronously on every render
  // so that the stable handleMount closure always reads the latest value.
  const activeFilePathRef    = useRef<string | null>(null);
  const refreshGitGutterRef  = useRef<(() => Promise<void>) | null>(null);
  activeFilePathRef.current  = activeTab?.path ?? null;
  const fnDecTimerRef  = useRef<ReturnType<typeof setTimeout> | null>(null);
  const diagTimerRef   = useRef<ReturnType<typeof setTimeout> | null>(null);

  const [explainSql, setExplainSql] = useState<string | null>(null);

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

  // ── Git gutter: clear HEAD cache and re-run when active tab changes ───────
  const activeFilePath = activeTab?.path ?? null;
  useEffect(() => {
    if (activeFilePath) {
      // Evict cached HEAD content so the next refresh re-fetches from go-git.
      headContentCache.delete(activeFilePath);
    }
    // Clear immediately, then schedule a fresh run for the new tab's content.
    gitGutterDecRef.current?.set([]);
    if (gitGutterTimerRef.current) clearTimeout(gitGutterTimerRef.current);
    gitGutterTimerRef.current = setTimeout(() => { refreshGitGutterRef.current?.(); }, 0);
  }, [tabId ?? activeTabId]);

  // ── "Explain SQL" context menu handler ───────────────────────────────────
  useEffect(() => {
    const handleExplain = async (e: Event) => {
      if (!useFeatureFlagsStore.getState().flags.explainSql) return;
      const { selectedText, fullSql, cursorLine } = (e as CustomEvent).detail as {
        selectedText: string | null;
        fullSql: string;
        cursorLine: number;
      };

      // Priority 1: use the selection verbatim.
      if (selectedText?.trim()) {
        setExplainSql(selectedText.trim());
        return;
      }

      // Priority 2: resolve the statement the cursor sits inside.
      try {
        const ranges = await GetSqlStatementRanges(fullSql);
        for (const r of (ranges || [])) {
          if (cursorLine >= r.startLine && cursorLine <= r.endLine) {
            const lines = fullSql.split("\n").slice(r.startLine - 1, r.endLine);
            const stmt = lines.join("\n").trim();
            if (stmt) { setExplainSql(stmt); return; }
          }
        }
      } catch { /* fall through */ }

      // Fallback: full editor content.
      if (fullSql.trim()) setExplainSql(fullSql.trim());
    };

    window.addEventListener("thaw:explain-sql", handleExplain);
    return () => window.removeEventListener("thaw:explain-sql", handleExplain);
  }, [tabId, activeTabId]);

  const handleBeforeMount: BeforeMount = (monaco) => {
    ensureMonacoSetup(monaco);
  };

  const handleMount: OnMount = (editor, monaco) => {
    if (!tabId) {
      setEditorInstance(editor);
      editor.onDidDispose(() => setEditorInstance(null));
      // Signal that the editor is mounted and ready for external commands
      // (used by CrossTabSearch to scroll to a match after a tab switch).
      window.dispatchEvent(new Event("thaw:editor-ready"));
    }

    activeStmtDecRef.current = editor.createDecorationsCollection([]);
    fnDecRef.current         = editor.createDecorationsCollection([]);
    gitGutterDecRef.current  = editor.createDecorationsCollection([]);

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
      if (!useFeatureFlagsStore.getState().flags.sqlDiagnostics) {
        monaco.editor.setModelMarkers(model, "thaw-sql", []);
        return;
      }
      if (model.getLanguageId() !== "sql") {
        monaco.editor.setModelMarkers(model, "thaw-sql", []);
        return;
      }
      const diagVersion = model.getVersionId();
      const diagSql = model.getValue();
      const diagMarkers: DiagMarker[] = [];

      try {
        // NOTE: This diagnostics pipeline is mirrored server-side in
        // internal/mcp/diag_tools.go (validateSQL). Changes to validation
        // ordering or request assembly should be reflected there too (#336).

        // ADD || [] to prevent spreading null from Go's nil slices!
        const syntaxErrors = await AnalyzeSqlSyntax(diagSql);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((syntaxErrors || []) as DiagMarker[]));

        const stmtRanges = (await GetSqlStatementRanges(diagSql)) || [];
        if (model.getVersionId() !== diagVersion) return;

        const dataTypeMarkers = await ValidateDataTypes(diagSql, stmtRanges);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((dataTypeMarkers || []) as DiagMarker[]));

        // Grammar check: recursive-descent Snowflake grammar (internal/sqlgrammar).
        // Flags recognized-but-malformed statements (missing names, dangling
        // keywords, …) as Warnings; skips unmodelled statements entirely.
        const grammarMarkers = await ValidateGrammar(diagSql, stmtRanges);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((grammarMarkers || []) as DiagMarker[]));

        // Semantic anti-patterns the grammar can't see (MERGE clause actions,
        // QUALIFY placement, FLATTEN/LATERAL, variant paths, Cortex names).
        const antiPatternMarkers = await ValidateAntiPatterns(diagSql, stmtRanges);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((antiPatternMarkers || []) as DiagMarker[]));

        const rawRefs = await ParseJoinTableRefs(diagSql);
        if (model.getVersionId() !== diagVersion) return;
        const storeObjs = useObjectStore.getState().objects;

        const storeDbs = useObjectStore.getState().databases;
        const storeSchemas = useObjectStore.getState().schemas;

        // Warm up databases/schemas from rawRefs (including USE statements)
        for (const ref of rawRefs || []) {
          if (ref.db) {
            const db = ref.db;
            const dbSchemas = storeSchemas.filter((s) => UC(s.db) === UC(db));
            if (dbSchemas.length === 0 && !fetchedDatabaseSchemas.has(UC(db))) {
              fetchedDatabaseSchemas.add(UC(db));
              void ListSchemas(db).then((fetched) => {
                useObjectStore.getState().addSchemas(db, fetched ?? []);
                if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
                diagTimerRef.current = setTimeout(runDiagnostics, 0);
              }).catch(() => { fetchedDatabaseSchemas.delete(UC(db)); });
            }
          }
          if (ref.db && ref.schema) {
            const db = ref.db;
            const schema = ref.schema;
            const schemaKey = `${UC(db)}\0${UC(schema)}`;
            const hasObjects = storeObjs.some((o) => UC(o.db) === UC(db) && UC(o.schema) === UC(schema));
            if (!hasObjects && !fetchedSchemaObjects.has(schemaKey)) {
              fetchedSchemaObjects.add(schemaKey);
              void ListObjects(db, schema).then((fetched) => {
                useObjectStore.getState().addObjects(
                  db, schema,
                  (fetched ?? []).map((o) => ({ name: o.name, kind: (o.kind || "OTHER").toUpperCase() })),
                );
                if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
                diagTimerRef.current = setTimeout(runDiagnostics, 0);
              }).catch(() => { fetchedSchemaObjects.delete(schemaKey); });
            }
          }
        }

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

        // Build allKnownTables from objectStore for quick-fix qualification suggestions
        const allKnownTables: ResolvedRef[] = storeObjs
          .filter((o) => o.kind === "TABLE" || o.kind === "VIEW")
          .map((o) => ({ alias: "", db: o.db, schema: o.schema, name: o.name }));

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
          allKnownTables,
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

        const semanticMarkers = await AnalyzeSqlSemantics(diagSql, resolved as any, colEntries as any);
        if (model.getVersionId() !== diagVersion) return;
        diagMarkers.push(...((semanticMarkers || []) as DiagMarker[]));

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

    editor.onDidChangeModelContent(() => {
      // Apply any pending MCP markers immediately (before the debounced diagnostics run).
      const curTabId = tabId ?? useQueryStore.getState().activeTabId;
      const pending = pendingMcpMarkers.get(curTabId);
      if (pending) {
        const m = editor.getModel();
        if (m) {
          pendingMcpMarkers.delete(curTabId);
          monaco.editor.setModelMarkers(m, "thaw-sql", pending);
        }
      }
      if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
      diagTimerRef.current = setTimeout(runDiagnostics, 400);
    });

    editor.onDidChangeModelLanguage(() => {
      const model = editor.getModel();
      if (model) {
        monaco.editor.setModelMarkers(model, "thaw-sql", []);
      }
    });

    runDiagnostics();

    // Re-run diagnostics when the object store is refreshed after a new
    // connection (e.g. offline-first startup: databases load post-connect).
    // The catalog may have changed, so drop the cached column/object metadata
    // first — autocomplete and the re-run diagnostics then re-fetch it fresh.
    const refreshDiagnosticsHandler = () => {
      clearMetadataCaches();
      if (diagTimerRef.current) clearTimeout(diagTimerRef.current);
      diagTimerRef.current = setTimeout(runDiagnostics, 0);
    };
    window.addEventListener("thaw:refresh-diagnostics", refreshDiagnosticsHandler);
    editor.onDidDispose(() => window.removeEventListener("thaw:refresh-diagnostics", refreshDiagnosticsHandler));

    // ── Git gutter decorators ────────────────────────────────────────────────
    const refreshGitGutter = async () => {
      const model = editor.getModel();
      if (!model) return;
      // Read from the ref so we always get the current tab's path, not the
      // path that was active when handleMount first ran (stale closure fix).
      const filePath = activeFilePathRef.current;
      if (!filePath) {
        // Scratch tab — clear any stale decorations.
        gitGutterDecRef.current?.set([]);
        return;
      }

      let headContent = headContentCache.get(filePath);
      if (headContent === undefined) {
        try {
          headContent = await GitGetHeadFileContent(filePath);
          headContentCache.set(filePath, headContent ?? "");
        } catch {
          headContentCache.set(filePath, "");
          headContent = "";
        }
      }

      const currentText = model.getValue();
      const headLines    = (headContent ?? "").split("\n");
      const currentLines = currentText.split("\n");

      const { added, modified, deleted } = await ComputeGitLineDiff(headLines, currentLines, MAX_DIFF_LINES);

      const decorations: any[] = [];
      for (const lineNum of added) {
        decorations.push({
          range: new monaco.Range(lineNum, 1, lineNum, 1),
          options: { linesDecorationsClassName: "git-gutter-added" },
        });
      }
      for (const lineNum of modified) {
        decorations.push({
          range: new monaco.Range(lineNum, 1, lineNum, 1),
          options: { linesDecorationsClassName: "git-gutter-modified" },
        });
      }
      for (const lineNum of deleted) {
        decorations.push({
          range: new monaco.Range(lineNum, 1, lineNum, 1),
          options: { linesDecorationsClassName: "git-gutter-deleted" },
        });
      }
      gitGutterDecRef.current?.set(decorations);
    };
    // Store so the tab-switch effect can trigger a refresh outside handleMount.
    refreshGitGutterRef.current = refreshGitGutter;

    // Initial run and debounced re-run on content change.
    refreshGitGutter();
    editor.onDidChangeModelContent(() => {
      if (gitGutterTimerRef.current) clearTimeout(gitGutterTimerRef.current);
      gitGutterTimerRef.current = setTimeout(refreshGitGutter, 400);
    });

    // Every editor instance — including the split-view secondary pane
    // (`tabId=splitTabId`) — needs its own clipboard patch (WKWebView blocks
    // navigator.clipboard).
    patchMonacoClipboard(editor);

    const trigger = (id: string) => editor.trigger("keyboard", id, null);
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.Slash,                      () => trigger("editor.action.commentLine"));
    editor.addCommand(monaco.KeyMod.Shift   | monaco.KeyMod.Alt | monaco.KeyCode.KeyA,   () => trigger("editor.action.blockComment"));
    editor.addCommand(monaco.KeyMod.Shift   | monaco.KeyMod.Alt | monaco.KeyCode.KeyF,   () => trigger("editor.action.formatDocument"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyF,                       () => trigger("actions.find"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyD,                       () => trigger("editor.action.addSelectionToNextFindMatch"));
    editor.addCommand(monaco.KeyMod.WinCtrl | monaco.KeyCode.KeyG,                       () => trigger("editor.action.gotoLine"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.DownArrow,                  () => { window.dispatchEvent(new Event("thaw:focus-results")); });
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyMod.Alt | monaco.KeyCode.UpArrow,   () => trigger("editor.action.insertCursorAbove"));
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyMod.Alt | monaco.KeyCode.DownArrow, () => trigger("editor.action.insertCursorBelow"));

    monaco.languages.registerCompletionItemProvider("sql", {
      triggerCharacters: ["."],
      provideCompletionItems: async (model: any, position: any) => {
        await ensureKeywordsLoaded();
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

        const schemaAutocompleteEnabled = useFeatureFlagsStore.getState().flags.schemaAutocomplete;

        const fullLine = model.getLineContent(position.lineNumber);
        const cursorOffset = model.getOffsetAt(position);
        const textToCursor = model.getValue().slice(0, cursorOffset);

        // Scan backwards from word start to see if we're triggered by a dot
        let charBefore = "";
        for (let i = word.startColumn - 2; i >= 0; i--) {
          if (fullLine[i] !== " " && fullLine[i] !== "\t") {
            charBefore = fullLine[i];
            break;
          }
        }

        if (charBefore === "." && schemaAutocompleteEnabled) {
          const idParts = await GetIdentifierAtColumn(fullLine, word.startColumn - 1);
          // GetIdentifierAtColumn returns the whole dotted chain, which includes the
          // segment currently being typed (word.word) as its last element. Drop it so
          // we complete the children of the *qualifier* and never DESCRIBE/SHOW the
          // half-typed name itself — doing so fired a failing Snowflake query on every
          // keystroke while typing an object name (DB.SCH.MY_N, MY_NO, MY_NOT, …).
          const contextParts = word.word ? idParts.slice(0, -1) : idParts;
          if (contextParts && contextParts.length > 0) {
            // Case: db.schema.table.
            if (contextParts.length === 3) {
              const [db, schema, table] = contextParts;
              return { suggestions: mkColSuggestions(await getColumns(db, schema, table), range, monaco) };
            }

            // Case: db.schema.
            if (contextParts.length === 2) {
              const [db, schema] = contextParts;
              const schemaKey = `${UC(db)}\0${UC(schema)}`;

              // If db/schema matches an existing table/view in objectStore, offer columns (alias-like behavior)
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
                    insertText: quoteIfNec(o.name),
                    sortText:   "03_" + o.name,
                    detail:     o.kind,
                    range,
                  })),
              };
            }

            // Case: qualifier.
            if (contextParts.length === 1) {
              const [qualifier] = contextParts;
              const { databases, schemas, objects } = useObjectStore.getState();

              // 1. Is it a known database? -> suggest schemas
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
                      insertText: quoteIfNec(s.name),
                      sortText:   "04_" + s.name,
                      detail:     "SCHEMA",
                      range,
                    })),
                };
              }

              // 1b + 2. Use full-statement context for CTE columns AND alias resolution
              // (ParseJoinTableRefs(textToCursor) misses FROM clauses after cursor)
              {
                const dotCtx = await GetAutocompleteContextFull({
                  sql: model.getValue(),
                  cursorOffset: model.getOffsetAt(position),
                  storeObjects: objects.map(o => ({ db: o.db, schema: o.schema, name: o.name, kind: o.kind })),
                  session: { database: useSessionStore.getState().database, schema: useSessionStore.getState().schema },
                  lineUpToWord,
                } as any);

                // 1b. CTE name → suggest CTE columns
                const dotCTECols = (dotCtx?.cteColumns ?? []).find((c: any) => UC(c.name) === UC(qualifier));
                if (dotCTECols && dotCTECols.cols && dotCTECols.cols.length > 0) {
                  return {
                    suggestions: dotCTECols.cols.map((col: any, i: number) => ({
                      label:      col.name,
                      kind:       monaco.languages.CompletionItemKind.Field,
                      insertText: quoteIfNec(col.name),
                      sortText:   "02_" + String(i).padStart(3, "0"),
                      detail:     `COLUMN (CTE) · ${dotCTECols.name}`,
                      range,
                    })),
                  };
                }

                // 2. Alias in current query → suggest columns (uses full-statement table refs)
                const dotRefs = dotCtx?.tableRefs ?? [];
                const aliasMatch = dotRefs.find((r: any) => UC(r.alias) === UC(qualifier));
                if (aliasMatch) {
                  // Find the resolved ref for this alias
                  const resolvedMatch = (dotCtx?.resolvedRefs ?? []).find(
                    (r: any) => UC(r.alias) === UC(qualifier)
                  );
                  if (resolvedMatch && resolvedMatch.db && resolvedMatch.schema) {
                    // Check in-editor tables first (tables defined but not yet executed)
                    const inEditorMatch = (dotCtx?.inEditorTables || []).find(
                      (tbl: any) => UC(tbl.name) === UC(resolvedMatch.name) &&
                                   UC(tbl.db) === UC(resolvedMatch.db) &&
                                   UC(tbl.schema) === UC(resolvedMatch.schema)
                    );
                    if (inEditorMatch && inEditorMatch.cols.length > 0) {
                      return {
                        suggestions: inEditorMatch.cols.map((col: any, i: number) => ({
                          label:      col.name,
                          kind:       monaco.languages.CompletionItemKind.Field,
                          insertText: quoteIfNec(col.name),
                          sortText:   "02_" + String(i).padStart(3, "0") + "_" + col.name,
                          detail:     `COLUMN (in-editor) · ${inEditorMatch.name}`,
                          range,
                        })),
                      };
                    }
                    // Fall back to Snowflake metadata
                    return { suggestions: mkColSuggestions(await getColumns(resolvedMatch.db, resolvedMatch.schema, resolvedMatch.name), range, monaco) };
                  }
                }
              }

              // 3. Is it a schema name (in current context)? -> suggest objects
              const schemaObjs = objects.filter((o) => UC(o.schema) === UC(qualifier));
              if (schemaObjs.length > 0) {
                return {
                  suggestions: schemaObjs.map((o) => ({
                    label:      o.name,
                    kind:       monacoKind(monaco, o.kind),
                    insertText: quoteIfNec(o.name),
                    sortText:   "03_" + o.name,
                    detail:     o.kind,
                    range,
                  })),
                };
              }

              // 4. Is it a table/view name? -> suggest columns
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
          }
        }

        // ── Unified autocomplete context (single IPC round-trip) ─────────
        const { databases, schemas, objects } = useObjectStore.getState();

        // Kick off the function-name lookup concurrently with the context fetch:
        // it depends only on the typed word, so there's no reason to wait for the
        // context round-trip before starting it. Awaited near the end.
        //
        // Skip it in JOIN-ON / JOIN-USING contexts: those branches return their own
        // suggestions before this promise is awaited, so firing it there is a wasted
        // IPC — and neither position wants function names ("ON" is never a function;
        // a JOIN USING(…) list holds column names). The USING check requires a
        // preceding JOIN in the same statement so it does NOT match
        // `MERGE INTO t USING (SELECT …)`, whose subquery DOES want function names.
        const inJoinOnOrUsing =
          word.word.toUpperCase() === "ON" ||
          /\bJOIN\b[^;]*\bUSING\s*\([^)]*$/i.test(textToCursor);
        const fnSuggestionsPromise =
          (word.word.length >= 2 && !lineUpToWord.trim().endsWith(".") && !inJoinOnOrUsing)
            ? GetFunctionSuggestions(word.word).catch(() => null)
            : Promise.resolve(null);

        const ctx = await GetAutocompleteContextFull({
          sql: model.getValue(),
          cursorOffset,
          storeObjects: objects.map(o => ({ db: o.db, schema: o.schema, name: o.name, kind: o.kind })),
          session: { database: useSessionStore.getState().database, schema: useSessionStore.getState().schema },
          lineUpToWord,
        } as any);
        const declaredVars: string[] = ctx?.scripting?.variables ?? [];
        const needsColon: boolean = ctx?.scripting?.needsColon ?? false;
        const ctxTableRefs = ctx?.tableRefs ?? [];
        const ctxCTEColumns = ctx?.cteColumns ?? [];

        // Build a CTE column lookup map for quick access
        const cteColMap = new Map<string, {name: string, dataType: string}[]>();
        for (const cte of ctxCTEColumns) {
          cteColMap.set(UC(cte.name), cte.cols ?? []);
        }

        // ── Datatype context (computed by backend) ────────────────────────
        if (ctx?.isDatatypeContext) {
          return {
            suggestions: SNOWFLAKE_DATA_TYPES.map((dt, i) => {
              const hasParams = dt.paramHint !== "";
              return {
                label:      dt.name,
                kind:       monaco.languages.CompletionItemKind.TypeParameter,
                insertText: hasParams ? `${dt.name}($1)` : dt.name,
                insertTextRules: hasParams
                  ? monaco.languages.CompletionItemInsertTextRule.InsertAsSnippet
                  : 0,
                detail:    hasParams ? `Type ${dt.paramHint}` : "Type",
                sortText:  "00_dt_" + String(i).padStart(3, "0"),
                range,
              };
            }),
          };
        }

        // ── JOIN ON clause completion (computed by backend) ────────────────
        const isInJoinOnClause = ctx?.isInJoinOnClause ?? false;
        const wordIsOn = word.word.toUpperCase() === "ON";
        // Needs ≥2 table refs; gate the IPC on the already-fetched ctxTableRefs.
        if ((wordIsOn || isInJoinOnClause) && schemaAutocompleteEnabled && ctxTableRefs.length >= 2) {
          const rawRefs = await ParseJoinTableRefs(textToCursor);
          if (rawRefs && (rawRefs as any[]).length >= 2) {
            const resolvedRefs = await ResolveTableRefs(rawRefs as any[], useObjectStore.getState().objects.map(o => ({ db: o.db, schema: o.schema, name: o.name, kind: o.kind })) as any, { database: "", schema: "" } as any, { database: useSessionStore.getState().database, schema: useSessionStore.getState().schema } as any);
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

        // JOIN/comma-join "ON <condition>" completion needs ≥2 table refs. ctxTableRefs
        // (already fetched, parsed from the whole statement) is a superset of the refs
        // up to the cursor, so when it has <2 we skip the ParseJoinTableRefs IPC
        // entirely — the common single-table keystroke no longer pays for it. The cheap
        // ON/USING text check is likewise done before the round-trip.
        if (schemaAutocompleteEnabled && ctxTableRefs.length >= 2) {
          const lastJoinSegment = (textToCursor.split(/\bJOIN\b/i).pop() ?? "").trim();
          const segmentOpenForJoin =
            lastJoinSegment.length > 0 && !/\b(?:ON|USING)\b/i.test(lastJoinSegment);
          const rawRefsC = segmentOpenForJoin ? await ParseJoinTableRefs(textToCursor) : null;
          const hasTriggerC = !!rawRefsC && (rawRefsC as any[]).length >= 2;

          if (hasTriggerC) {
            const resolvedC = await ResolveTableRefs(rawRefsC as any[], useObjectStore.getState().objects.map(o => ({ db: o.db, schema: o.schema, name: o.name, kind: o.kind })) as any, { database: "", schema: "" } as any, { database: useSessionStore.getState().database, schema: useSessionStore.getState().schema } as any);
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

        // ── USING clause completion (computed by backend) ──────────────
        const usingInCtx = ctx?.usingClause?.inUsing ?? false;
        const usingPartialInCtx = ctx?.usingClause?.isPartial ?? false;
        if ((usingInCtx || usingPartialInCtx) && schemaAutocompleteEnabled) {
          const usingRefs = ctxTableRefs.length >= 2 ? ctxTableRefs : (await ParseJoinTableRefs(textToCursor) || []);
          if (usingRefs.length >= 2) {
            const resolvedUsing = await ResolveTableRefs(usingRefs as any[], objects.map(o => ({ db: o.db, schema: o.schema, name: o.name, kind: o.kind })) as any, (ctx?.useContext ?? { database: "", schema: "" }) as any, { database: useSessionStore.getState().database, schema: useSessionStore.getState().schema } as any);
            if (resolvedUsing && resolvedUsing.length >= 2) {
              // Get columns for the last two refs (the JOIN pair)
              const lastTwo = resolvedUsing.slice(-2);
              const colSets: Set<string>[] = [];
              for (const ref of lastTwo) {
                const cols = await getColInfos(ref.db, ref.schema, ref.name);
                colSets.push(new Set((cols || []).map((c: any) => UC(c.name))));
              }
              if (colSets.length === 2) {
                const shared = [...colSets[0]].filter((c) => colSets[1].has(c));
                // Filter out already-listed columns in partial USING
                const alreadyListed = new Set<string>();
                if (usingPartialInCtx) {
                  const insideParen = textToCursor.slice(textToCursor.lastIndexOf("(") + 1);
                  for (const part of insideParen.split(",")) {
                    const trimmed = part.trim().toUpperCase();
                    if (trimmed) alreadyListed.add(trimmed);
                  }
                }
                const filtered = shared.filter((c) => !alreadyListed.has(c));
                if (filtered.length > 0) {
                  return {
                    suggestions: filtered.map((col, i) => ({
                      label:      col,
                      kind:       monaco.languages.CompletionItemKind.Field,
                      insertText: col,
                      sortText:   "00_" + String(i).padStart(3, "0"),
                      detail:     "SHARED COLUMN",
                      range,
                    })),
                  };
                }
              }
            }
          }
        }

        // ── Grammar-driven keyword expectations ───────────────────────────
        // The recursive-descent grammar (internal/sqlgrammar, via ExpectedAt)
        // reports the keywords valid right after the cursor for modelled
        // statements — e.g. FROM after `COPY INTO <table>`, the object types
        // after CREATE/DROP, the alter verbs after `ALTER TABLE <name>`. Offer
        // them first (sortText "00_grm_") and drop them from the generic keyword
        // dump below so they aren't listed twice. Empty for unmodelled leaders,
        // so completion stays leading-keyword-gated with no behavior change.
        const grammarKeywords: string[] = ctx?.grammarExpected?.keywords ?? [];
        const grammarKwSet = new Set(grammarKeywords.map((k) => k.toUpperCase()));
        const grammarKeywordSuggestions = grammarKeywords.map((kw) => ({
          label:      kw,
          kind:       monaco.languages.CompletionItemKind.Keyword,
          insertText: kw,
          sortText:   "00_grm_" + kw,
          detail:     "Expected here",
          range,
        }));

        const keywordSuggestions = snowflakeKeywordsArray
          .filter((kw) => !grammarKwSet.has(kw.toUpperCase()))
          .map((kw) => ({
            label:      kw,
            kind:       monaco.languages.CompletionItemKind.Keyword,
            insertText: kw,
            sortText:   "08_" + kw,
            range,
          }));

        const variableSuggestions = buildVariableSuggestions(declaredVars, needsColon, range, monaco);

        const dbSuggestions = databases.map((db) => ({
          label:      db,
          kind:       monaco.languages.CompletionItemKind.Module,
          insertText: quoteIfNec(db),
          sortText:   "05_" + db,
          detail:     "DATABASE",
          range,
        }));

        const schemaSuggestions = schemas.map((s) => ({
          label:      s.name,
          kind:       monaco.languages.CompletionItemKind.Module,
          insertText: quoteIfNec(s.name),
          sortText:   "04_" + s.name,
          detail:     `SCHEMA · ${s.db}`,
          range,
        }));

        const objectSuggestions = objects.map((o) => ({
          label:      o.name,
          kind:       monacoKind(monaco, o.kind),
          insertText: quoteIfNec(o.name),
          sortText:   "03_" + o.name,
          detail:     `${o.kind} · ${o.db}.${o.schema}`,
          range,
        }));

        const contextColSuggestions: any[] = [];
        let fetchPending = false;

        if (schemaAutocompleteEnabled) {
        const seenColKeys = new Set<string>();

        // Use backend-resolved refs directly (already qualified via store/UseContext/session)
        const refsToFetch: {db: string, schema: string, name: string}[] = [];
        for (const ref of (ctx?.resolvedRefs || [])) {
          // Skip CTE names — their columns are added below
          if (cteColMap.has(UC(ref.name)) && !ref.db && !ref.schema) continue;
          // Skip the ref whose name is the token currently being typed: it's a
          // half-finished table name (FROM MY_T…), so DESCRIBE-ing it fires a failing
          // Snowflake query on every keystroke. Its columns get fetched once the name
          // is complete and the cursor moves off it.
          if (word.word && UC(ref.name) === UC(word.word)) continue;
          refsToFetch.push({ db: ref.db, schema: ref.schema, name: ref.name });
        }

        // Add in-editor CREATE TABLE column suggestions (only for tables referenced in current stmt)
        const referencedTableNames = new Set((ctx?.resolvedRefs || []).map((r: any) => UC(r.name)));
        for (const tbl of (ctx?.inEditorTables || [])) {
          if (!referencedTableNames.has(UC(tbl.name))) continue;
          for (const col of tbl.cols) {
            const colName = col.name || (col as any);
            if (!seenColKeys.has(UC(colName))) {
              seenColKeys.add(UC(colName));
              contextColSuggestions.push({
                label:      colName,
                kind:       monaco.languages.CompletionItemKind.Field,
                insertText: quoteIfNec(colName),
                sortText:   "02_" + colName,
                detail:     `COLUMN (in-editor) · ${tbl.name}`,
                range,
              });
            }
          }
        }

        // Add CTE column suggestions
        for (const [cteName, cols] of cteColMap) {
          for (const col of cols) {
            const colName = col.name || (col as any);
            if (!seenColKeys.has(UC(colName))) {
              seenColKeys.add(UC(colName));
              contextColSuggestions.push({
                label:      colName,
                kind:       monaco.languages.CompletionItemKind.Field,
                insertText: quoteIfNec(colName),
                sortText:   "02_" + colName,
                detail:     `COLUMN (CTE) · ${cteName}`,
                range,
              });
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
                  insertText: quoteIfNec(col),
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
        } // end schemaAutocompleteEnabled (context columns)

        let fnSuggestions: any[] = [];
        {
          const fns = await fnSuggestionsPromise;
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
        }

        return {
          suggestions: [
            ...grammarKeywordSuggestions,
            ...variableSuggestions,
            ...contextColSuggestions,
            ...keywordSuggestions,
            ...(schemaAutocompleteEnabled ? dbSuggestions      : []),
            ...(schemaAutocompleteEnabled ? schemaSuggestions  : []),
            ...(schemaAutocompleteEnabled ? objectSuggestions  : []),
            ...fnSuggestions,
          ],
          incomplete: fetchPending,
        };
      },
    });

    // ── Quick-Fix CodeActionProvider ──────────────────────────────────────────
    if (codeActionProviderDisposable) {
      codeActionProviderDisposable.dispose();
      codeActionProviderDisposable = null;
    }
    codeActionProviderDisposable = monaco.languages.registerCodeActionProvider("sql", {
      provideCodeActions(_model: any, _range: any, context: any) {
        const actions: any[] = [];
        for (const marker of context.markers ?? []) {
          if (!marker.code) continue;
          let payload: any;
          try {
            payload = typeof marker.code === "string" ? JSON.parse(marker.code) : marker.code;
          } catch { continue; }
          if (payload.kind !== "qualify-table" || !payload.suggestions) continue;
          for (const suggestion of payload.suggestions) {
            actions.push({
              title: `Qualify as ${suggestion}`,
              kind: "quickfix",
              diagnostics: [marker],
              edit: {
                edits: [{
                  resource: _model.uri,
                  textEdit: {
                    range: {
                      startLineNumber: marker.startLineNumber,
                      startColumn: marker.startColumn,
                      endLineNumber: marker.endLineNumber,
                      endColumn: marker.endColumn,
                    },
                    text: suggestion,
                  },
                  versionId: undefined,
                }],
              },
            });
          }
        }
        return { actions, dispose() {} };
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

      if (!useFeatureFlagsStore.getState().flags.ddlHoverTooltips) {
        if (hoverTimerRef.current) { clearTimeout(hoverTimerRef.current); hoverTimerRef.current = null; }
        scheduleHide();
        return;
      }

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

      if ((!parts || parts.length === 0) && !diagMarkerAtPos) {
        lastHoverWordRef.current = null;
        if (hoverTimerRef.current) { clearTimeout(hoverTimerRef.current); hoverTimerRef.current = null; }
        if (!isOnTooltipRef.current) scheduleHide();
        return;
      }

      cancelHide();
      currentHoverPosRef.current = pos;

      const wordKey = (parts && parts.length > 0)
        ? parts.join("\0")
        : `marker:${diagMarkerAtPos!.startLineNumber}:${diagMarkerAtPos!.startColumn}`;
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
          const resolved = await ResolveTableRefs((rawRefs || []) as any[], objects.map(o => ({ db: o.db, schema: o.schema, name: o.name, kind: o.kind })) as any, { database: "", schema: "" } as any, { database: useSessionStore.getState().database, schema: useSessionStore.getState().schema } as any);
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
              const resolved = await ResolveTableRefs(ghostRefs as any[], useObjectStore.getState().objects.map(o => ({ db: o.db, schema: o.schema, name: o.name, kind: o.kind })) as any, { database: "", schema: "" } as any, { database: useSessionStore.getState().database, schema: useSessionStore.getState().schema } as any);
              if (resolved && resolved.length >= 2) {
                const fkEntries = resolved.map((ref) => ({
                  db: ref.db, schema: ref.schema, name: ref.name,
                  fks: getFKsCached(ref.db, ref.schema, ref.name),
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
          if (!useFeatureFlagsStore.getState().flags.aiInlineCompletions) return { items: [] };

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

    // ponytail: defer the setSelectedSql store write (a React re-render) past
    // Monaco's keystroke tick — running it synchronously here drops the first
    // keystroke when typing over a selection (issue #575). refreshOccurrences
    // stays synchronous so occurrence highlights still update live during a drag.
    let selectionTimer: ReturnType<typeof setTimeout> | undefined;
    editor.onDidChangeCursorSelection(() => {
      refreshOccurrences();
      clearTimeout(selectionTimer);
      selectionTimer = setTimeout(() => {
        const selection = editor.getSelection();
        const selected  = selection && !selection.isEmpty()
          ? editor.getModel()?.getValueInRange(selection) ?? ""
          : "";
        setSelectedSql(selected);
      }, 0);
    });
    editor.onDidDispose(() => clearTimeout(selectionTimer));

    // After a mouse drag-select, WKWebView wedges Monaco's hidden-textarea input
    // deduction: the first printable key typed over the selection produces no
    // model edit (you had to press twice). Keyboard selections are unaffected.
    // Rather than fight Monaco's textarea sync, intercept that first key and
    // re-issue it through Monaco's type command. (#575)
    const dragDom = editor.getDomNode();
    let pendingDragReplace = false;
    const onDragMouseUp = (e: Event) => {
      const me = e as MouseEvent;
      if (me.button !== 0) { pendingDragReplace = false; return; } // leave right-click alone
      const sel = editor.getSelection();
      pendingDragReplace = !!(sel && !sel.isEmpty());
    };
    const onDragKeyDown = (e: Event) => {
      const ke = e as KeyboardEvent;
      if (!pendingDragReplace) return;
      // If focus moved out of the code text input — e.g. into the find/replace
      // box, which is part of the editor widget and so does NOT fire
      // onDidBlurEditorWidget — this keystroke isn't for the editor. Drop the
      // pending state and let it reach the focused field untouched.
      if (!editor.hasTextFocus()) { pendingDragReplace = false; return; }
      // A lone modifier press precedes the real char (Shift before 'A'); let it
      // through without consuming the pending state.
      if (ke.key === "Shift" || ke.key === "Control" || ke.key === "Alt" || ke.key === "Meta") return;
      pendingDragReplace = false;                                   // first real key consumes it
      if (ke.metaKey || ke.ctrlKey || ke.altKey || ke.isComposing) return;
      if (ke.key.length !== 1) return;                              // printable chars only
      const sel = editor.getSelection();
      if (!sel || sel.isEmpty()) return;
      ke.preventDefault();
      ke.stopPropagation();
      // type command (not executeEdits) so auto-surround/auto-close, cursor
      // placement and undo coalescing match a real keystroke.
      editor.trigger("keyboard", "type", { text: ke.key });
    };
    dragDom?.addEventListener("mouseup", onDragMouseUp);
    dragDom?.addEventListener("keydown", onDragKeyDown, true);      // capture: beat Monaco's handler
    // Drop the pending state when the editor loses focus (e.g. Alt+Tab) so we
    // don't intercept a keystroke against a stale selection on return. Uses
    // Monaco's widget-blur event, not a DOM blur, to ignore internal textarea churn.
    editor.onDidBlurEditorWidget(() => { pendingDragReplace = false; });
    editor.onDidDispose(() => {
      dragDom?.removeEventListener("mouseup", onDragMouseUp);
      dragDom?.removeEventListener("keydown", onDragKeyDown, true);
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

    if (useFeatureFlagsStore.getState().flags.crossTabSearch) {
      // No keybindings — ⌘⇧H is handled by QueryPage's global keydown handler
      // to avoid a double-toggle when Monaco doesn't preventDefault on the event.
      editor.addAction({
        id: "thaw.crossTabSearch",
        label: "Find & Replace in Tabs",
        contextMenuGroupId: "3_find",
        contextMenuOrder: 1,
        run: () => {
          window.dispatchEvent(new Event("thaw:toggle-cross-tab-search"));
        },
      });
    }

    // Only the primary editor (no tabId) should respond to scroll-to-line
    // events — split/secondary editors have different content and the line
    // number could be invalid or misleading.
    if (!tabId) {
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
      editor.onDidDispose(() => window.removeEventListener("thaw:scroll-to-line", handleScrollToLine));
    }

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
        fixedOverflowWidgets: true,
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
    {explainSql && (
      <ExplainModal sql={explainSql} tabId={tabId ?? activeTabId} onClose={() => setExplainSql(null)} />
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