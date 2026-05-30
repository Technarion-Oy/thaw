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
| `connectionStore.ts` | Active connection flag and `ConnectionParams`. Persisted to `sessionStorage`; credentials (password, passcode, privateKeyPassphrase) are stripped before write. |
| `queryStore.ts` | Tab list, active tab, per-tab SQL/result/error/running state, and flat aliases that mirror the active tab. Persisted to localStorage via a `safeLocalStorage` wrapper that swallows `QuotaExceededError`. Results and `isRunning` are never persisted. File-backed tab content is cleared before persist and reloaded from disk on startup. |
| `sessionStore.ts` | Live Snowflake session context (role, warehouse, database, schema) and per-tab context cache. Calls `GetSessionContext`, `UseRole`, `UseWarehouse`, `UseDatabase`, `UseSchema` IPC methods directly. Reads `activeTabId` from `queryStore` to determine which tab to send USE commands to. Not persisted. |
| `objectStore.ts` | In-memory cache of databases, schemas, and objects expanded in the sidebar tree. Used as tier-1 of the search cascade. No persistence. |
| `gridStore.ts` | Results grid state: TanStack `tableRows` reference, range selection, in-grid search matches, column format configs, and conditional formatting rules. **Singleton — shared across all tabs.** Resets navigation state on tab switch; resets formatting only when column schema changes. |
| `themeStore.ts` | Light/dark/system preference, resolved theme, UI font, editor font, editor font size, and UI density. Applies changes to `document.documentElement` immediately on set and on rehydration. Persisted to localStorage. |
| `panelLayoutStore.ts` | Sidebar panel order (left/right), sidebar widths, editor/results split fraction, split-editor width, and left-sidebar hidden toggle. Persisted to localStorage. |
| `diffStore.ts` | Two-step DDL diff workflow: holds the first selected item (`pending`) until the second is chosen, then fetches both DDL texts via IPC and opens a diff tab in `queryStore`. |
| `gitStore.ts` | Git repo config (exportDir, remoteURL, branch, author) and runtime state (status, push/pull/clone loading flags, branches list). OAuth token is in-memory only, never persisted. Calls all `Git*` IPC methods directly. |
| `notebookToolbarStore.ts` | Lightweight bridge between `NotebookTab` (writes kernel state and callbacks) and the unified `Toolbar` via `QueryPage` (reads). Cleared when the notebook tab unmounts or is deactivated. |
| `notebookPrefsStore.ts` | Snowpark notebook preferences (`syntaxMode`). Loaded from backend via `GetNotebookPrefs`. |
| `featureFlagsStore.ts` | Feature flag values and admin-locked flags. Optimistic defaults (all enabled) until `load()` fetches from backend. Reloaded after the user saves flags in the modal. |
| `insertMappingStore.ts` | Transient state for the Insert Mapping feature: target table, source tables, and modal-open flag. No persistence. |

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
- **`connectionStore` credential stripping.** `password`, `passcode`, and
  `privateKeyPassphrase` are zeroed in the persisted slice so they are never
  written to `sessionStorage`.
- **`sessionStore` schema list invalidation.** When `switchDatabase` is called
  (or the database changes after `loadContext`), `schemas` and
  `schemasForDatabase` are reset so the dropdown re-fetches for the new database.
  `loadSchemas` is a no-op when `schemasForDatabase` already matches the current
  database.
