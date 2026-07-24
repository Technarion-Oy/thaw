# frontend/src/store

> Zustand stores that hold all global frontend state for the Thaw desktop app.

## Responsibility

Each store owns one coherent slice of application state and exposes typed
actions. Components subscribe to the slice they need; stores call each other
directly (via `getState()`) only where cross-store coupling is unavoidable.
Persistence is handled per-store via Zustand's `persist` middleware.

## Files

| File | Purpose |
|------|---------|
| `connectionStore.ts` | Active connection flag and `ConnectionParams` (including forward-proxy fields). Persisted to `sessionStorage`; credentials (password, passcode, privateKeyPassphrase, token, oauthClientSecret, proxyPassword) are stripped before write. |
| `queryStore.ts` | Tab list, active tab, per-tab SQL/result/error/running state, and flat aliases that mirror the active tab. Persisted to localStorage via a `safeLocalStorage` wrapper that swallows `QuotaExceededError`. Results and `isRunning` are never persisted. File-backed tab content is cleared before persist and reloaded from disk on startup and again whenever the file changes externally (driven by `QueryPage`'s `fs:changed` listener). `refreshFileTab(id, diskContent)` applies a disk re-read: it no-ops when the content already matches `savedSql` or when the tab has unsaved edits (VSCode-style — the external change is ignored and the tab stays dirty against its original baseline), and only updates `sql`/`savedSql` for a clean tab whose disk content actually changed. `Tab.mcpOrigin` marks tabs created by MCP tools; `openMcpTab(title, sql)` creates SQL tabs, `openMcpNotebookTab(title, content)` creates notebook tabs with MCP origin. Closing the last remaining tab (`closeTab`) replaces it with a fresh scratch tab built from the post-removal (empty) list, so the `SQL (n)` number resets to 1 instead of climbing (#595). New scratch tabs are created via `makeScratchTab` — titled `SQL (n)` (`nextScratchTitle`) and flagged `Tab.isDefaultTitle` so callers (e.g. the Save dialog's `untitled.sql` default) can detect an auto-generated title without re-parsing it; `renameTab(id, title)` retitles a tab and clears `isDefaultTitle` (used by the tab bar for non-file tabs), as do `markSaved`/`updateTabPath`. `kindFromPath` infers `Tab.kind` from the extension (`.py`→python, `.yml`/`.yaml`→yaml, `.md`/`.markdown`→markdown, `.sql`→sql); any other extension opens as `"plaintext"` so SQL highlighting/autocomplete stay off for non-SQL text files. **Preview tabs (#849, VS Code style):** `Tab.preview` marks a single reusable "preview" tab (italic title in `TabBar`). `openFile(path, content, preview?)` with `preview=true` reuses the existing clean preview tab in place instead of appending a new one (a *dirty* preview is promoted first, never silently discarded, then a fresh preview opens) — so browsing many files doesn't flood the tab bar. A preview is promoted to permanent (flag cleared) when `promoteTab(id)` is called (double-click in the browser or on the tab), or on first edit (`setSql` clears the flag once `sql` diverges from `savedSql`). A file that already has a permanent tab is never demoted; a permanent open of a currently-previewed file promotes it in place. At most one preview tab exists at a time. A preview is only recycled when it's clean, **idle**, and not the current **split target** — a *running* preview is promoted instead (an in-flight query is bound to the tab id via `runQuery`/`WaitForQueryResult`/`setTabRunning` and must not land on a tab now showing a different file), and a preview that is `splitTabId` is promoted rather than recycled (swapping its file in place would pull the split pane onto the new file and collapse both panes onto one tab). Because recycling keeps the tab **id**, `openFile` dispatches a `thaw:tab-reused` event (same pattern as `markSaved`'s `thaw:file-saved`) so consumers can drop the per-id / per-model state they hold *outside* the store: `QueryPage` drops the result history, history/compare selection, and the file-read sequence guard (all keyed by tab id) — otherwise the previous file's results would leak into the reused tab — and `SqlEditor` resets its Monaco model's undo/redo stack + view state via `model.setValue` (for non-YAML kinds, which share one model per pane; YAML is already model-per-file so its undo is naturally isolated) so an undo can't restore the previous file's text into the reused tab. `preview` is session-only (excluded from `partialize`, like `mcpOrigin`/`isRunning`), so restored tabs come back permanent. Callers gate preview behavior on `editorTabPrefsStore.previewTabsEnabled`. |
| `sessionStore.ts` | Live Snowflake session context (role, warehouse, database, schema) and per-tab context cache. Calls `GetSessionContext`, `UseRole`, `UseWarehouse`, `UseDatabase`, `UseSchema` IPC methods directly. Reads `activeTabId` from `queryStore` to determine which tab to send USE commands to. Not persisted. |
| `objectStore.ts` | In-memory cache of databases, schemas, and objects expanded in the sidebar tree. Used as tier-1 of the search cascade. No persistence. |
| `gridStore.ts` | Results grid state: TanStack `tableRows` reference, range selection (plus its origin — cell click, row gutter, column header, or select-all), in-grid search matches, column format configs, and conditional formatting rules. **Singleton — shared across all tabs.** Resets navigation state on tab switch; resets formatting only when column schema changes. |
| `themeStore.ts` | Light/dark/system preference, resolved theme, UI font, editor font, editor font size, and UI density. Applies changes to `document.documentElement` immediately on set and on rehydration. Persisted to localStorage. |
| `panelLayoutStore.ts` | Sidebar panel order (left/right), sidebar widths, editor/results split fraction, split-editor width, cell detail panel width, and left-sidebar hidden toggle. Persisted to localStorage. |
| `diffStore.ts` | Two-step DDL diff workflow: holds the first selected item (`pending`) until the second is chosen, then fetches both DDL texts via IPC and opens a diff tab in `queryStore`. |
| `gitStore.ts` | Git repo config (exportDir, remoteURL, branch, author, `recentDirs`) and runtime state (status with `staged`/`unstaged` lists, pull/clone/staging/committing loading flags, branches list). **Working directory** = `exportDir`; `openFolder(dir)` sets it (clearing the previous folder's `remoteURL`/`branch` so git ops fall back to the new folder's live status instead of targeting the old repo) then records the folder via the atomic `AddRecentDir` IPC (which returns the authoritative newest-first, deduped, cap-8 list — no stale-snapshot overwrite of another window's entries), `pickExportDir` picks a directory then calls `openFolder`, `clearRecentDirs` calls the `ClearRecentDirs` IPC, `openInNewWindow` picks a folder, spawns a second Thaw instance there (`OpenFolderInNewInstance` — errors surface before it's recorded) without changing this window's `exportDir`. `saveConfig` persists only the per-repo/instance fields (`exportDir`/`remoteURL`/`branch`); the shared identity/pref fields have dedicated atomic actions — `saveAuthor(name,email)` (`SaveGitAuthor`) and `saveExportPathTemplate(tmpl)` (`SaveGitExportPathTemplate`) — and `recentDirs` its own (`AddRecentDir`/`ClearRecentDirs`), so a whole-struct write can't revert another window's edit to a shared field. Staging actions (`stageFile`, `unstageFile`, `stageAll`, `unstageAll`, `discardFile`) operate on the real index; `commitStaged` commits the staged set and pushes. OAuth token is in-memory only, never persisted. Calls all `Git*` IPC methods directly. |
| `notebookToolbarStore.ts` | Lightweight bridge between `NotebookTab` (writes kernel state and callbacks) and the unified `Toolbar` via `QueryPage` (reads). Cleared when the notebook tab unmounts or is deactivated. |
| `notebookPrefsStore.ts` | Snowpark notebook preferences (`syntaxMode`). Loaded from backend via `GetNotebookPrefs`. |
| `featureFlagsStore.ts` | Feature flag values and admin-locked flags. Optimistic defaults (all enabled) until `load()` fetches from backend. Reloaded after the user saves flags in the modal. |
| `editorTabPrefsStore.ts` | Frontend-only editor tab preferences, persisted to localStorage (`thaw-editor-tab-prefs`). Currently `previewTabsEnabled` (default true), mirroring VS Code's `workbench.editor.enablePreview` — when off, file-browser/search opens go straight to permanent tabs. Toggled in `EditorPreferencesModal`; read by `openFileInTab` callers. No backend counterpart (pure UI behavior). |
| `logPrefsStore.ts` | File-logging preferences (`logLevel`, `includeQuerySQL`, `includeInternalQueries`) plus the admin-lock mask. Optimistic defaults (info level, no SQL to disk, nothing locked) until `load()` fetches `GetLogPrefs`/`GetLogPrefsLocked`. Reloaded after `UpdateLogPrefs` in `LoggingPreferencesModal`. |
| `insertMappingStore.ts` | Transient state for the Insert Mapping feature: target table, source tables, and modal-open flag. No persistence. |
| `mcpStore.ts` | Snapshot of running MCP sessions (`SessionInfo[]`). `refresh()` calls `ListMCPSessions` IPC; the Toolbar `MCPIndicator` and `MCPSessionsModal` subscribe. No persistence. |

## Patterns & integration

- All stores use `create<State>()` from Zustand 5. Async actions live directly
  in the store as plain functions (not middleware).
- `persist` middleware is used for `connectionStore` (sessionStorage),
  `queryStore` (localStorage with safe wrapper), `themeStore`, and
  `panelLayoutStore`. The `partialize` option is used to exclude sensitive or
  transient fields.
- Cross-store reads use `useXStore.getState()` (outside React) rather than
  hooks to avoid requiring a component context. Example: `sessionStore` reads
  `queryStore.getState().activeTabId` before firing `UseRole`.
- IPC is called directly from store actions (`import ... from
  "../../wailsjs/go/app/App"`), not proxied through components.
- `themeStore` applies side-effects (`document.documentElement.setAttribute`)
  on module load and in `onRehydrateStorage` so the theme is applied before the
  first render.

## Gotchas

- **`gridStore` is a singleton.** Selection range, search state, column
  formatting, conditional formatting rules, and the `tableRows` reference are
  shared across all tabs. Tab switches reset navigation state but not
  formatting; formatting is only reset when the column schema changes. In
  side-by-side compare mode, both `ResultGrid` instances call `setTableRows()` —
  the last to render wins, so `StatusBar`/`GridSearch` may reflect data from
  the compare grid. Notebook SQL cells use `ResultGrid` with the `standalone`
  prop to suppress `setTableRows()`/`resetGrid()` calls.
- **`queryStore` localStorage quota.** Large result sets can exceed the quota.
  `safeLocalStorage` silently drops writes; the in-memory store remains
  authoritative. Results are never persisted. File-backed tab SQL content is
  cleared before persist to stay within budget.
- **`queryStore` persist is debounced (#762).** `persist` serializes and writes
  on every `set()`, and a keystroke does a `setSql` (plus, on selection change, a
  `setSelectedSql`), so an unthrottled write made letters appear slowly as open
  scratch-tab content grew — a synchronous `localStorage.setItem` in WKWebView
  blocks the main thread. `safeLocalStorage.setItem` now coalesces writes and
  flushes the newest value 500 ms after the last one (and on `pagehide` /
  `visibilitychange`→hidden, so a quit/reload doesn't lose the last burst).
  `getItem` returns any still-pending value so a rehydrate never reads stale data.
- **`connectionStore` credential stripping.** `password`, `passcode`,
  `privateKeyPassphrase`, `token`, `oauthClientSecret`, and `proxyPassword` are
  zeroed in the persisted slice so they are never written to `sessionStorage`.
- **`sessionStore` schema list invalidation.** When `switchDatabase` is called
  (or the database changes after `loadContext`), `schemas` and
  `schemasForDatabase` are reset so the dropdown re-fetches for the new database.
  `loadSchemas` is a no-op when `schemasForDatabase` already matches the current
  database.
