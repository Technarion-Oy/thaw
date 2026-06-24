# frontend/src/components/editor

> Monaco-based SQL editor with Snowflake-aware completions, diagnostics, snippets, and cross-tab search.

## Responsibility

Hosts the active query editor. Owns Monaco lifecycle (setup, provider registration, theme),
SQL syntax highlighting, autocomplete, hover DDL, inline AI completions, SQL diagnostics,
git gutter decorations, the tab bar, editor preferences, and the cross-tab search/replace panel.

## Files

| File | Purpose |
|------|---------|
| `SqlEditor.tsx` | Main editor component. Mounts Monaco, registers all completion/hover/code-action/signature providers (module-level, not in render), runs `runDiagnostics` on content change, handles git gutter decoration, clipboard patching, and the snippet context menu via internal Monaco `MenuRegistry`. Exports `DiagMarker`, `ColInfo`, `ResolvedRef`, `pendingMcpMarkers`. |
| `sqlEditorUtils.ts` | Pure helpers: `UC`, `quoteIfNecessary`, `FKEntry`, `getFKs` (async, deduped), `getFKsCached`, `setFKCache`, `buildVariableSuggestions`, `getQualifiedIdent`, `getStatementLineRanges`. No React. |
| `editorRef.ts` | Singleton ref to the active `IStandaloneCodeEditor`. Exports `setEditorInstance`, `getEditorInstance`, `insertAtCursor`. Kept separate from `SqlEditor.tsx` so Vite Fast Refresh is not broken by mixing component and non-component exports. |
| `monacoSetup.ts` | One-time Monaco initialisation: Snowflake Monarch language, Python Monarch grammar (inlined to avoid side-effect imports), YAML worker wiring, `thawDarkTheme`/`thawLightTheme` registration. Called via `ensureMonacoSetup()` guard. Imports the **slim** Monaco API (`editor.api.js` + `editor.all.js`), never the `monaco-editor` barrel, to keep the TS/HTML/CSS/JSON language workers (~9 MB) and ~80 basic-language grammars out of the binary — see [gotchas](../../../../docs/concepts/gotchas.md). |
| `snowflakeSql.ts` | Snowflake Monarch tokenizer (`snowflakeMonarchLanguage`) and custom Monaco theme definitions (`thawDarkTheme`, `thawLightTheme`). The tokenizer's `datatypes` list is sourced from the generated artifact `src/generated/snowflakeDataTypes.ts` (source of truth: `internal/snowflake/datatypes.go`) rather than hand-maintained. |
| `snowflakeSnippets.ts` | Snowflake Scripting snippet definitions (`getSnowflakeSnippets`) and `SNIPPET_CATEGORIES` for the cascading context-menu submenu. Snippets are applied through `applyPrefsToSnippet` at insertion time (keyword casing, indent style). |
| `CrossTabSearch.tsx` | Search/replace panel triggered by `⌘⇧H` / `Ctrl+Shift+H`. Searches all tabs (SQL, YAML, Python) and notebook cell sources. Navigates via `thaw:scroll-to-line` / `thaw:editor-ready` events. Supports regex with back-references, case-sensitive toggle, and match counter. Gated behind the `crossTabSearch` feature flag. |
| `CrossTabSearch.test.ts` | Unit tests for `getNotebookCellSources` and related helpers in `CrossTabSearch.tsx`. |
| `TabBar.tsx` | Renders the tab strip above the editor. Supports drag-to-reorder, right-click context menu (close, close others, split, duplicate), bulk-close confirmation, and split-tab mode. MCP-created tabs (`tab.mcpOrigin`) display a `RobotOutlined` icon in the accent color — this check takes priority over the notebook `ExperimentOutlined` icon, so MCP-created notebooks also show the robot badge. Reads/writes `queryStore`. |
| `EditorPreferencesModal.tsx` | Modal for editor preferences (keyword case, indent style/size, font, font size). Calls `GetEditorPrefs`/`SaveEditorPrefs` IPC. Shows a live SQL preview using `formatSQL`. |
| `yamlWorker.ts` | Vite worker entry point that imports `monaco-yaml/yaml.worker` for YAML language support. |

## Patterns & integration

**IPC calls (from `wailsjs/go/app/App`):**
`GetObjectDDL`, `ListObjects`, `ListSchemas`, `GetTableColumns`, `GetTableColumnsWithTypes`,
`GetSchemaForeignKeys`, `GetUserDDL`, `GetAISuggestion`, `GetFunctionSuggestions`,
`GetFunctionTooltip`, `GetAllFunctionNames`, `GetEditorPrefs`,
`GitGetHeadFileContent`. (Data types are not fetched over IPC — they come from
the bundled artifact `src/generated/snowflakeDataTypes.ts`.)

**IPC calls (from `wailsjs/go/sqleditor/Service`):**
`AnalyzeSqlSyntax`, `ParseJoinTableRefs`, `ComputeJoinOnConditions`, `AnalyzeSqlSemantics`,
`GetSqlStatementRanges`, `GetIdentifierAtColumn`, `GetActiveFunctionCall`,
`ParseSignatureParams`, `ValidateDataTypes`,
`ValidateGrammar` (recursive-descent Snowflake grammar check — Warning markers),
`ValidateAntiPatterns` (semantic anti-patterns: MERGE clause actions, QUALIFY, FLATTEN/LATERAL, variant paths, Cortex names),
`ValidateTablesExist`, `ValidateBareColumnRefs`, `GetSnowflakeKeywords`,
`GetAutocompleteContextFull`, `ResolveTableRefs`, `ComputeGitLineDiff`.

**Stores used:** `queryStore` (SQL content, tab state, selected SQL), `objectStore` (schema cache),
`sessionStore` (session context), `themeStore` (dark/light), `featureFlagsStore` (flag gating).

**Module-level caches:**
- `hoverDDLCache` — `Map<key, {ddl, ts}>`, 60 s TTL.
- `fetchedSchemaObjects` — `Set<string>` to suppress duplicate `ListObjects` calls.
- `fetchedDatabaseSchemas` — `Set<string>` for schema listing dedup.
- `headContentCache` — `Map<filePath, headContent>` for git gutter diffs.
- FK cache lives in `sqlEditorUtils.ts` (`fkCache` Map + `fetchingFKs` Set).
- `pendingMcpMarkers` — `Map<tabId, DiagMarker[]>` for MCP marker seeding. Written by `QueryPage` when the `mcp:open-sql-tab` Wails event fires; read and cleared by `onDidChangeModelContent` in `SqlEditor`. Markers are applied immediately before the 400ms debounced diagnostics run, so the user sees inline errors as soon as the tab opens.

**Provider registration:** All completion, hover, signature-help, and code-action providers
are registered once at module level (disposable refs). Never re-register inside the component
render or `handleMount` — doing so accumulates duplicate providers across editor remounts.

**Snippet context menu:** Uses Monaco internal `MenuRegistry` + `CommandsRegistry` (IIFE, runs
once). Per-editor `onContextMenu` sets `_activeSnippetEditor` so commands target the right
instance.

**Clipboard:** `navigator.clipboard` is blocked in WKWebView. All copy operations use
`ClipboardSetText` from `wailsjs/runtime/runtime`. Monaco's built-in copy/paste is patched via
`patchMonacoClipboard`.

## Gotchas

- **Never register completion/hover providers inside render or `handleMount`** — use module-level
  disposable refs; re-registration on remount accumulates duplicates and leaks.
- **`runDiagnostics` must be race-safe and exception-safe**: capture `model.getVersionId()` before
  every `await`; return early (inside `try/catch/finally`) if the version advanced. The `finally`
  block always calls `setModelMarkers` so stale markers are never left stuck.
- **`editor.onDidChangeModelContent`** must be used, not `editor.getModel()?.onDidChangeContent` —
  the latter silently skips registration if the model is null at mount time.
- **Git gutter is skipped** for files exceeding `MAX_DIFF_LINES` (3 000) to avoid O(H×C) DP
  overhead; `ComputeGitLineDiff` runs on the Go backend.
- **Do not use `instanceof SubmenuAction`** from an external import for the snippet submenu —
  Monaco's `menu.js` checks its own bundled class; external imports are different module instances
  and always fail. Use `MenuRegistry` and let Monaco create `SubmenuAction` internally.
- **`crossTabSearch` flag**: the panel is conditionally rendered by `QueryPage`; its state (search
  term, toggles) is lost when closed because the component unmounts.
- **Notebook navigation** in `CrossTabSearch`: switching to a notebook tab does not scroll to or
  highlight the match within the cell — `thaw:editor-ready` is only emitted by the primary
  `SqlEditor`, not by per-cell notebook editors.
