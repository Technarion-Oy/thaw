// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import Editor, { type BeforeMount, type OnMount } from "@monaco-editor/react";
import { snowflakeMonarchLanguage, thawDarkTheme, thawLightTheme } from "./snowflakeSql";
import { useQueryStore } from "../../store/queryStore";
import { useObjectStore } from "../../store/objectStore";
import { useThemeStore } from "../../store/themeStore";
import { ClipboardGetText, ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { GetObjectDDL, ListObjects, ListSchemas, GetTableColumns, GetUserDDL, GetAISuggestion } from "../../../wailsjs/go/main/App";

// Module-level DDL cache and hover provider handle so we only register once
// and don't accumulate duplicate providers on editor remounts.
const hoverDDLCache = new Map<string, string>();
let hoverProviderDisposable: { dispose(): void } | null = null;
let inlineCompletionsDisposable: { dispose(): void } | null = null;
let languageAndThemesRegistered = false;

// Singleton editor reference — set on mount so external callers (e.g. the
// sidebar) can insert text at the current cursor position without prop drilling.
let _editorInstance: import("monaco-editor").editor.IStandaloneCodeEditor | null = null;

export function insertAtCursor(text: string) {
  if (!_editorInstance) return;
  const selection = _editorInstance.getSelection();
  if (!selection) return;
  _editorInstance.executeEdits("sidebar-insert", [{ range: selection, text, forceMoveMarkers: true }]);
  _editorInstance.pushUndoStop();
  _editorInstance.focus();
}

// Track which db/schema pairs and databases have already been lazy-fetched by
// the completion provider so we don't fire duplicate requests.
const fetchedSchemaObjects   = new Set<string>(); // "DB\0SCHEMA"
const fetchedDatabaseSchemas = new Set<string>(); // "DB"

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

function mkColSuggestions(cols: string[], range: any, monaco: any) {
  return cols.map((col) => ({
    label:      col,
    kind:       monaco.languages.CompletionItemKind.Field,
    insertText: col,
    detail:     "COLUMN",
    range,
  }));
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

export default function SqlEditor() {
  const { sql, setSql, setSelectedSql } = useQueryStore();
  const resolved       = useThemeStore((s) => s.resolved);
  const editorFont     = useThemeStore((s) => s.editorFont);
  const editorFontSize = useThemeStore((s) => s.editorFontSize);

  // Register the custom Snowflake SQL tokenizer and themes exactly once,
  // before the editor instance is created.
  const handleBeforeMount: BeforeMount = (monaco) => {
    if (languageAndThemesRegistered) return;
    languageAndThemesRegistered = true;
    monaco.languages.setMonarchTokensProvider("sql", snowflakeMonarchLanguage as any);
    monaco.editor.defineTheme("thaw-dark",  thawDarkTheme  as any);
    monaco.editor.defineTheme("thaw-light", thawLightTheme as any);
  };

  const handleMount: OnMount = (editor, monaco) => {
    _editorInstance = editor;

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

    // Keyboard shortcut overrides (addCommand).
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyV, doPaste);
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyC, doCopy);
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyX, doCut);

    // Context-menu paste: Monaco's context menu calls commandService.executeCommand()
    // directly, bypassing addCommand keybindings. Patch the editor's internal
    // command service so paste/copy/cut from the context menu also use the
    // Wails native clipboard instead of the blocked navigator.clipboard API.
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

        const UC = (s: string) => s.toUpperCase();

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

        return {
          suggestions: [...contextColSuggestions, ...keywordSuggestions, ...dbSuggestions, ...schemaSuggestions, ...objectSuggestions],
          // Tell Monaco these results may be incomplete so it re-queries on next invocation
          incomplete: fetchPending,
        };
      },
    });

    // ── Object definition hover ───────────────────────────────────────────
    // Dispose any previous registration to avoid stacking on remount.
    if (hoverProviderDisposable) {
      hoverProviderDisposable.dispose();
    }
    hoverProviderDisposable = monaco.languages.registerHoverProvider("sql", {
      provideHover: async (model: any, position: any) => {
        const word = model.getWordAtPosition(position);
        if (!word) return null;

        const { objects } = useObjectStore.getState();
        const match = objects.find(
          (o) => o.name.toUpperCase() === word.word.toUpperCase() &&
                 (o.kind === "TABLE" || o.kind === "VIEW"),
        );
        if (!match) return null;

        const cacheKey = `${match.db}\0${match.schema}\0${match.kind}\0${match.name}`;
        let ddl: string;
        if (hoverDDLCache.has(cacheKey)) {
          ddl = hoverDDLCache.get(cacheKey)!;
        } else {
          try {
            ddl = await GetObjectDDL(match.db, match.schema, match.kind, match.name, "");
            hoverDDLCache.set(cacheKey, ddl);
          } catch {
            return null;
          }
        }
        if (!ddl) return null;

        return {
          range: {
            startLineNumber: position.lineNumber,
            endLineNumber:   position.lineNumber,
            startColumn:     word.startColumn,
            endColumn:       word.endColumn,
          },
          contents: [
            { value: `**${match.kind}** — \`${match.db}.${match.schema}.${match.name}\`` },
            { value: "```sql\n" + ddl + "\n```" },
          ],
        };
      },
    });

    // ── AI inline completions ─────────────────────────────────────────────
    if (!inlineCompletionsDisposable) {
      inlineCompletionsDisposable = monaco.languages.registerInlineCompletionsProvider("sql", {
        provideInlineCompletions: async (model: any, position: any, _ctx: any, token: any) => {
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

  return (
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
      }}
    />
  );
}
