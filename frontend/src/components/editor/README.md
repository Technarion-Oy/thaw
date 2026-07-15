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
| `sqlEditorUtils.ts` | Pure helpers: `UC`, `quoteIfNecessary`, `colCacheKey` (shared NUL-delimited per-table cache key), `normId` (frontend mirror of the backend `normID` identifier normaliser, for matching captured qualifiers against resolved refs), `FKEntry`, `getFKs` (async, deduped), `getFKsCached`, `setFKCache`, `buildVariableSuggestions`, `getQualifiedIdent`, `getStatementLineRanges`, `identifierRangeAt` (quote-aware column span of the dotted identifier under the cursor, for the cmd/ctrl-hover link underline), `starMenuEligible` (reuses `identifierRangeAt` to gate the "Expand \*" menu — hides it when the `*` is inside a quoted object name), `byteColToUtf16Col` (converts a 1-based UTF-8 byte column to a 1-based UTF-16 Monaco column). No React. |
| `sqlEditorUtils.test.ts` | Unit tests for `identifierRangeAt` (bare/quoted/escaped-quote/unterminated-quote spans), `starMenuEligible` (bare/`alias.*` eligible, quoted-object-name and single-quoted-string hidden, apostrophe-in-`"it's"` still eligible), `normId` (bare→upper, quoted case-preserved/distinct, `""` unescape), and `byteColToUtf16Col` (ASCII pass-through, multi-byte/emoji shift, past-end clamp). |
| `editorRef.ts` | Singleton ref to the active `IStandaloneCodeEditor`. Exports `setEditorInstance`, `getEditorInstance`, `insertAtCursor`. Kept separate from `SqlEditor.tsx` so Vite Fast Refresh is not broken by mixing component and non-component exports. |
| `monacoSetup.ts` | One-time Monaco initialisation: Snowflake Monarch language, Python & Markdown Monarch grammars (inlined to avoid side-effect imports), YAML worker wiring, `thawDarkTheme`/`thawLightTheme` registration. Called via `ensureMonacoSetup()` guard. Imports the **slim** Monaco API (`editor.api.js` + `editor.all.js`), never the `monaco-editor` barrel, to keep the TS/HTML/CSS/JSON language workers (~9 MB) and ~80 basic-language grammars out of the binary — see [gotchas](../../../../docs/concepts/gotchas.md). |
| `snowflakeSql.ts` | Snowflake Monarch tokenizer (`snowflakeMonarchLanguage`) and custom Monaco theme definitions (`thawDarkTheme`, `thawLightTheme`). The tokenizer's `datatypes` list is sourced from the generated artifact `src/generated/snowflakeDataTypes.ts` (source of truth: `internal/snowflake/datatypes.go`) rather than hand-maintained. Also the single source for the built-in-function catalogue: `BUILTIN_FUNCTION_CATEGORIES`, `CONTEXT_FUNCTIONS`, and the assembled `FUNCTION_CATEGORIES` consumed by the Code Snippets modal and the editor's Built-in Functions submenu. |
| `monacoMenu.ts` | `getOrCreateMenuId(key)` — one helper for the "get existing Monaco `MenuId` or create it" idiom (reaches into Monaco's unexported `MenuId._instances`), shared by the SQL and notebook context-menu registrations so a Monaco bump breaking that internal is fixed in one place. |
| `snowflakeSnippets.ts` | Snowflake Scripting snippet definitions (`getSnowflakeSnippets`) and `SNIPPET_CATEGORIES` for the cascading context-menu submenu. Snippets are applied through `applyPrefsToSnippet` at insertion time (keyword casing, indent style). |
| `CrossTabSearch.tsx` | Search/replace panel triggered by `⌘⇧H` / `Ctrl+Shift+H`. Searches all tabs (SQL, YAML, Python) and notebook cell sources. Navigates via `thaw:scroll-to-line` / `thaw:editor-ready` events. Supports regex with back-references, case-sensitive toggle, and match counter. Gated behind the `crossTabSearch` feature flag. |
| `CrossTabSearch.test.ts` | Unit tests for `getNotebookCellSources` and related helpers in `CrossTabSearch.tsx`. |
| `TabBar.tsx` | Renders the tab strip above the editor. Supports drag-to-reorder, right-click context menu (close, close others, split, duplicate), bulk-close confirmation, and split-tab mode. MCP-created tabs (`tab.mcpOrigin`) display a `RobotOutlined` icon in the accent color — this check takes priority over the notebook `ExperimentOutlined` icon, so MCP-created notebooks also show the robot badge. The right-click menu is a per-tab AntD `Dropdown` (`trigger={["contextMenu"]}`, built by `buildTabMenuItems`) — same idiom as the query-history context menu in `QueryPage.tsx` — with icons, `danger: true` on Close Others/Close All, an `extra` keybinding hint on Close (`⌘W`), and "Split with" as a native submenu (`children`) instead of a hand-rolled hover panel. A far-right `CaretDownOutlined` button (after the `+` new-scratch button) opens the **Active Files** dropdown: a searchable list of every open tab (icon + dirty `•`/orphan `↺` prefix + title), styled with the same `.ctx-item` class (rounded, inset rows) so it reads as one visual language with the tab context menu; clicking an entry calls `activateTab`. Also toggled by `⌘⇧E` / `Ctrl+Shift+E` via the `thaw:open-active-files` window event; closes on outside click or Escape. Non-file tabs (no backing path, not a diff) can be **renamed** — double-click the title or use the context-menu "Rename" — via `queryStore.renameTab`; file tabs keep the filename-derived title. New scratch tabs are numbered `SQL (n)` (`nextScratchTitle` in `queryStore`). Reads/writes `queryStore`. |
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

Session is **per-tab** on the backend, so `runDiagnostics` and the Expand-`*` command
read THIS editor's own tab session via `sessionForTab(tabId)` (its `tabContexts[tabId]`,
falling back to the global/active-tab context) rather than the global `sessionStore.database/schema`.
Otherwise the split pane (`tabId=splitTabId`) would validate against the active tab's session,
mis-firing the "No database/schema selected" diagnostics (#717).

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

**Object hover + cmd/ctrl modifier (`ddlHoverTooltips` flag):** `resolveStoreObject(parts)`
(module-level) resolves a dotted identifier under the cursor to a store object of **any** kind
(not just TABLE/VIEW), fetching the schema's objects on demand. On a name collision across
namespaces (e.g. a stream named after its source table) it prefers the TABLE/VIEW — a heuristic
tie-break, since hover has no parse context. `editor.onMouseMove` uses it via the shared
`showObjectTooltip(pos, obj, withDdl)`: plain hover shows a lightweight identity tooltip
(`withDdl=false` → header-only `KIND — DB.SCHEMA.NAME`, no DDL fetch); with the platform modifier
(`metaKey`/`ctrlKey`) held, `withDdl=true` fetches `GetObjectDDL(db, schema, kind, name, "")` and
renders the full DDL — no click. Kinds `GET_DDL` can't render (`kindSupportsDdl` from
`utils/objectDdl.ts`) get no underline and fall back to the identity tooltip; a failed/empty fetch
also falls back to identity (not cached, so a re-hover retries). `cmdModHeld` tracks the modifier
from mouse-move events and from **document-level** `keydown`/`keyup` listeners (`onModChange`) —
Monaco's `editor.onKeyDown` only fires while the hidden textarea is focused, which hover never is, so
document listeners (gated on `lastSqlMousePos`) are what make "press the modifier while stationary
over an object" work without clicking in first. That path upgrades identity → DDL via
`showDdlAtLastPos()` — which honours diagnostic-marker precedence (`markerAt`) and bails if the mouse
moved mid-fetch. While held, `evaluateCmdLink` underlines the identifier with a `.cmd-link`
decoration (link affordance); `identifierRangeAt` (in `sqlEditorUtils.ts`, quote-aware so
`DB."MY TABLE".COL` spans as one, unit-tested) computes the dotted-identifier span. All four hover
tooltips (diagnostic, column, object, function) share `positionTooltip(pos, heightPx)` for screen
placement. A resolved
table alias short-circuits to the column path only — never object resolution — so `alias.col` can't
false-match a `schema.object`. The store `kind` is passed straight through — never guessed. Column
and function hovers keep their own dedicated paths in `onMouseMove`.

**Grammar-driven keyword completions:** `GetAutocompleteContextFull` returns `grammarExpected`
— the recursive-descent grammar's "valid next" set at the cursor (see
[`internal/sqlgrammar`](../../../../internal/sqlgrammar/README.md) and `internal/sqleditor.GrammarExpectedAt`).
The completion provider offers `grammarExpected.keywords` first (`sortText "00_grm_"`, detail
"Expected here") and drops them from the generic keyword dump so they aren't listed twice — e.g.
`FROM` after `COPY INTO <table>`, the object types after `CREATE`/`DROP`. It is empty for
unmodelled leading keywords, so completion stays leading-keyword-gated (no behavior change for
SQL the grammar doesn't yet model). `grammarExpected.kinds` (token-kind expectations like
`Identifier`) is reserved for future use; the catalog/column/stage sources still drive those.

**Snippet context menu:** Uses Monaco internal `MenuRegistry` + `CommandsRegistry` (IIFE, runs
once). Per-editor `onContextMenu` sets `_activeSnippetEditor` so commands target the right
instance. **Cascade:** the "SQL Snippets" submenu holds one nested submenu per
`SNIPPET_CATEGORIES` entry (its own `MenuId`, titled by `cat.header`), each holding the snippet
commands, plus a **Built-in Functions** submenu that nests one level deeper — category (Context +
`BUILTIN_FUNCTION_CATEGORIES` from `snowflakeSql.ts`) → `NAME()` command. Function insert text
escapes `$` (e.g. `SYSTEM$CANCEL_QUERY`) so the snippet engine doesn't read it as a variable, and
uses `$0` to drop the cursor inside the parens. Keeping every level short means the menu never
overflows off-screen; `.context-view .monaco-menu-container` in `global.css` also caps menu height
to the viewport and scrolls as a backstop.

**Expand `*` context menu:** A module-level `MenuRegistry` item (`thaw.expandWildcard`, gated on
`editorLangId == sql` **and** the per-editor `thawStarUnderCursor` context key) that replaces a
select-list wildcard with its column list. Detection is **backend/tokenizer-based**: the command calls
`Service.StarSelectAt(sql, line, col)` (`internal/sqleditor`, over `sqltok`), which returns the
wildcard's span + any `alias.` qualifier or nil — a `*` inside a quoted identifier (`"a*b"`) or a
multiplication (`a * b`) is never misread, and quoted aliases (`"my table".*`) are captured whole.
The context key is only a cheap display gate — `starMenuEligible` (in `sqlEditorUtils.ts`) sets it
when the cursor sits on a literal `*` that is **not** part of an object name. It reuses
`identifierRangeAt` (the same span logic behind the DDL-hover underline) rather than a bespoke
parser: a `*` in an object name lives inside a quoted identifier (`"Testin*table"`), which
`identifierRangeAt` returns a range *containing* the star for, whereas a bare `*` gets no range and an
`alias.*` star falls just past the `alias.` range — so both stay eligible. The gate must be set
*before* the menu renders, so it's driven off the events that fire ahead of the context-menu event
rather than `onContextMenu` (whose listener may run after Monaco already showed the menu):
`onDidChangeCursorPosition` (keyboard nav + clicks that move the cursor) and the right-mouse
`onMouseDown` (a right-click *inside a selection*, where Monaco leaves the cursor put so the click
point `e.target.position` is the only truth). The authoritative decision runs in the command against
`_starMenuPos` — the click point for a right-click, or `editor.getPosition()` for a keyboard-invoked
menu (`e.target.position` is null there) — via `Service.StarSelectAt`, and no-ops if the token isn't
really a wildcard. The command then scopes to the statement (`statementTextAtLine`, shared with the
Explain SQL handler), resolves its `FROM`/`JOIN` refs (`ParseJoinTableRefs` + `ResolveTableRefs`,
same as the JOIN/drag-drop paths), matches `alias.*` against them with `normId` — a case-sensitive
mirror of the backend `normID` (quoted `"Foo"`/`"foo"` stay distinct, doubled `"a""b"` unescapes),
so it compares exactly against the already-normID'd refs. For a **bare** `*` it additionally checks
`Service.FromSourceCount(stmt)` (kicked off concurrently with ref resolution) against the resolved-ref
count and refuses (no-op) on any mismatch or `-1` — `ParseJoinTables` isn't depth-aware, so it pulls
tables out of `WHERE (SELECT …)` subqueries and CTEs, drops old-style `FROM a, b` comma joins, and
mis-reads table functions / `PIVOT` clauses; `FromSourceCount` returns `-1` for all of those (and any
non-table source), so the guard prevents writing an incomplete/wrong list. It fetches columns via the shared cached
`getColumns()` wrapper (all target tables concurrently, `Promise.all`, deduped by `colCacheKey`) and
`quoteIfNec`s each column — and the qualifier too, but only for a bare `*` over multiple tables (where
the prefix is a normID'd alias/name that may need re-quoting, e.g. an unaliased `"My Table"`); for
`alias.*` the prefix is the raw source text the user wrote, already valid, so it is emitted verbatim.
It re-checks `model.getVersionId()` after every await (like `runDiagnostics`) and bails if the
document changed, so a stale range is never applied; a cold cache / failed fetch no-ops rather than
leaving a half-edit. (`getColumns` and `getColInfos` share one `cachedTableFetch<T>` helper that caches
the in-flight `Promise`, so a concurrent caller — e.g. the autocomplete cache-warm loop — resolves the
same fetch instead of getting an empty list.) `starMenuEligible` also hides the item for a `*` inside a
single-quoted string literal (`'x*y'`), which `identifierRangeAt` — double-quote-only — doesn't cover;
its `'`-parity scan skips double-quoted identifiers so an apostrophe in a column name (`"it's"`) can't
flip it.

**Clipboard:** `navigator.clipboard` is blocked in WKWebView. All copy operations use
`ClipboardSetText` from `wailsjs/runtime/runtime`. Monaco's built-in **code-buffer** copy/paste is
patched per-editor via `patchMonacoClipboard` (gated on the public `codeEditor.hasTextFocus()`); the
find/replace/rename fields inside `.monaco-editor` are ordinary native fields handled by the global
Cmd/Ctrl+V/C/X handler in `App.tsx` (which skips the code buffer via `monaco.editor.getEditors()`).

## Gotchas

- **Never register completion/hover providers inside render or `handleMount`** — use module-level
  disposable refs; re-registration on remount accumulates duplicates and leaks.
- **`runDiagnostics` must be race-safe and exception-safe**: capture `model.getVersionId()` **and** a
  monotonic run token (`myRun = ++diagRunRef.current`) at the start; after every `await` (and in
  `finally`) bail if either advanced. versionId only detects **text** edits — the run token is what
  supersedes an in-flight run triggered *without* a text change (session switch, `thaw:refresh-diagnostics`,
  mid-run `ListSchemas`/`ListObjects`/`getColInfos` refetch callbacks). Without it, two runs sharing one
  versionId both reach `finally`'s `setModelMarkers` and the last to *finish* wins — re-applying stale
  markers (#718). The `finally` block always calls `setModelMarkers` (when current) so stale markers are
  never left stuck.
- **Backend markers use UTF-8 byte columns; Monaco wants UTF-16** — every diagnostics validator
  emits 1-based byte columns (`sqltok.Token.Col`). `toUtf16Markers` (module-level in `SqlEditor.tsx`,
  via `byteColToUtf16Col`) converts each marker's start/end column against its own line text at the
  single choke point right before `setModelMarkers` (both the debounced run and the MCP-seeded path).
  This is the root-cause fix (issue #702): converting here also fixes the "Qualify as …" quick fix,
  which reads its edit range from the stored marker. Do not re-plumb byte offsets through the Go validators.
- **`editor.onDidChangeModelContent`** must be used, not `editor.getModel()?.onDidChangeContent` —
  the latter silently skips registration if the model is null at mount time.
- **Git gutter is skipped** for files exceeding `MAX_DIFF_LINES` (3 000) to avoid O(H×C) DP
  overhead; `ComputeGitLineDiff` runs on the Go backend.
- **Do not use `instanceof SubmenuAction`** from an external import for the snippet submenu —
  Monaco's `menu.js` checks its own bundled class; external imports are different module instances
  and always fail. Use `MenuRegistry` and let Monaco create `SubmenuAction` internally.
- **Monaco's standalone editor context menu does not render item icons at all**, verified
  empirically (issue #592): the internal `ICommandAction.icon` field on a `MenuRegistry`-registered
  command (tried on Explain SQL / SQL Snippets) never reaches the rendered `.action-label` — the
  standalone context-menu's action resolution doesn't go through the same `MenuItemAction`
  icon-to-class conversion the full VS Code menu bar uses. The codicon font itself loads fine (other
  Monaco chrome uses it), so don't spend more time trying to attach an `icon` — there's no hook for
  it in this render path, for `addAction` items or `MenuRegistry` items alike.
- **Keybinding hints in the Monaco context menu only auto-render when the shown action's own id has
  a keybinding the editor's keybinding service can resolve.** `editor.addAction({keybindings: [...]})`
  registers a real per-editor keybinding under that action's id, and the built-in context menu does
  look it up and right-align it — confirmed working for Format SQL's `⇧⌥F` (and for Monaco's own
  built-ins, e.g. "Change All Occurrences" shows `⌘F2`). Actions with no `keybindings` entry (Toggle
  Line Comment, Find & Replace in Tabs — the latter is deliberately unbound here to avoid a
  double-toggle with `QueryPage`'s global `⌘⇧H` handler) can't get that native hint, so their
  shortcut is appended to the label text instead (`"Toggle Line Comment    ⌘/"`).
- **Find-widget button tooltips clip under the tab bar** unless forced below. Monaco's base-layer
  hover tooltips default to rendering *above* their target and the find widget is pinned to the
  editor's top edge, so "above" lands in the tab-bar band where the editor pane's `overflow: hidden`
  clips it. `utils/monacoTooltipFix.ts`'s `registerFindWidgetTooltipFix()` — called once from
  `ensureMonacoSetup` — registers a global `monaco.editor.onDidCreateEditor` hook that patches the
  hover-service singleton (a private `_createHover` method) to flip the default to below. Because
  it's a global editor-creation hook it covers every Monaco mount (SqlEditor, notebook cells, modals,
  diff views) and is decoupled from the per-editor clipboard patch. If a version bump removes
  `_createHover`, it warns once and no-ops rather than retrying forever.
- **`crossTabSearch` flag**: the panel is conditionally rendered by `QueryPage`; its state (search
  term, toggles) is lost when closed because the component unmounts.
- **Notebook navigation** in `CrossTabSearch`: switching to a notebook tab does not scroll to or
  highlight the match within the cell — `thaw:editor-ready` is only emitted by the primary
  `SqlEditor`, not by per-cell notebook editors.
- **Typing over a selection — two WKWebView workarounds you must not remove (#575):**
  1. In `onDidChangeCursorSelection`, the `setSelectedSql` store write is deferred via
     `setTimeout(0)`; running it synchronously drops the first keystroke typed over a
     keyboard/double-click selection (the Zustand re-render lands mid-keystroke).
     `refreshOccurrences` deliberately stays synchronous so occurrence highlights still
     update live during a drag.
  2. The `onDragMouseUp`/`onDragKeyDown` capture-phase block intercepts the **first printable
     key after a mouse drag-select** and re-issues it via `editor.trigger("keyboard", "type", …)`.
     WKWebView wedges Monaco's hidden-textarea input deduction after a drag, so without this the
     first character is silently dropped. It is **not** dead code — removing it reintroduces #575
     for drag selections.
