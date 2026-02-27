// Copyright (c) 2026 Technarion Oy. All rights reserved.
//
// This software and its source code are proprietary and confidential.
// Unauthorized copying, distribution, modification, or use of this software,
// in whole or in part, is strictly prohibited without prior written permission
// from Technarion Oy.
//
// Commercial use of this software is restricted to parties holding a valid
// license agreement with Technarion Oy.

import Editor, { type OnMount } from "@monaco-editor/react";
import { useQueryStore } from "../../store/queryStore";
import { useObjectStore } from "../../store/objectStore";
import { useThemeStore } from "../../store/themeStore";
import { ClipboardGetText, ClipboardSetText } from "../../../wailsjs/runtime/runtime";
import { GetObjectDDL, ListObjects, ListSchemas } from "../../../wailsjs/go/main/App";

// Module-level DDL cache and hover provider handle so we only register once
// and don't accumulate duplicate providers on editor remounts.
const hoverDDLCache = new Map<string, string>();
let hoverProviderDisposable: { dispose(): void } | null = null;

// Track which db/schema pairs and databases have already been lazy-fetched by
// the completion provider so we don't fire duplicate requests.
const fetchedSchemaObjects   = new Set<string>(); // "DB\0SCHEMA"
const fetchedDatabaseSchemas = new Set<string>(); // "DB"

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
  const resolved = useThemeStore((s) => s.resolved);

  const handleMount: OnMount = (editor, monaco) => {
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

        // ── db.schema. → suggest objects in that schema ──────────────────
        const twoPartMatch = lineUpToWord.match(/\b(\w+)\.(\w+)\.\s*$/i);
        if (twoPartMatch) {
          const [, db, schema] = twoPartMatch;
          const schemaKey = `${UC(db)}\0${UC(schema)}`;

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

        return { suggestions: [...keywordSuggestions, ...dbSuggestions, ...schemaSuggestions, ...objectSuggestions] };
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
  };

  return (
    <Editor
      height="100%"
      defaultLanguage="sql"
      theme={resolved === "dark" ? "vs-dark" : "vs"}
      value={sql}
      onChange={(v) => setSql(v ?? "")}
      onMount={handleMount}
      options={{
        fontSize: 14,
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
