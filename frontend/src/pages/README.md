# frontend/src/pages

> Top-level page components that compose the full application UI from stores, layout primitives, and feature components.

## Responsibility

Pages are the root of the React component tree mounted by the Wails webview.
Each page owns the application-level orchestration for a screen: it wires
stores together, manages per-screen local state, registers Wails event
listeners, drives IPC calls that span multiple domains, and composes the layout
from reusable components under `components/`.

## Files

| File | Purpose |
|------|---------|
| `QueryPage.tsx` | The sole top-level page; renders the complete SQL editor, notebook, diff, and results UI. Owns query execution, tab lifecycle, per-tab result history, cross-tab search, and panel split logic. |

## QueryPage in detail

`QueryPage` is the single page in the application. Every screen the user
interacts with (SQL editor, notebook, DDL diff, results grid, terminal,
modals) is composed here.

### Responsibilities

**Query execution pipeline**

The two-phase async pipeline (`StartQuery` → `WaitForQueryResult`) lives
entirely in `QueryPage`'s `runQuery` function. This is the only path that
populates `resultHistory` and makes results visible in the UI. The store's
`executeWith` is a lower-level escape hatch that bypasses the history. Wails
events `query:statement-start` and `query:statement-qid` drive multi-statement
progress tracking and per-statement editor highlighting.

**Tab session management**

`QueryPage` tracks which tab IDs are new or removed on each render and calls
`InitTabSession`/`CloseTabSession` via IPC accordingly. Session init mode
(`lazy` vs `eager`) is loaded from the backend and re-read whenever the user
saves session config. On tab switch, `useSessionStore.setActiveTab` is called
for instant feedback from cache, followed by `loadContext` for a fresh
round-trip to Go.

**Per-tab result history**

Result history (`HistoryEntry[]`) is local React state keyed by tab ID (a
`Map<string, HistoryEntry[]>`). Up to ten unpinned entries plus all pinned
entries are retained per tab. History state is not persisted — it resets on
page reload.

**Panel layout and split drag**

The vertical editor/results split (fraction 0–1) and the horizontal
primary/compare editor split are both managed with mouse-event handlers that
update local state during drag and flush to `panelLayoutStore` on mouse-up.
Local state drives CSS `flex` directly during drag to avoid React re-render
overhead.

**Wails event listeners**

`QueryPage` subscribes to multiple Wails events on mount:
- `menu:snowpark-open-notebook` — opens a file-picker then loads the notebook.
- `menu:snowpark-new-notebook` — opens a blank notebook tab.
- `query:statement-start` / `query:statement-qid` — multi-statement progress.

Custom DOM events handled here:
- `thaw:execute-in-tab` — emitted by `queryStore.executeInNewTab` to ask
  `QueryPage` to run the query through the full pipeline.
- `load-query` — emitted by `QueryHistoryModal` to set SQL in the active tab.
- `thaw:connect` — triggered when `runQuery` is called while disconnected,
  causing the connect modal to open.
- `thaw:session-config-saved` — re-reads session init mode after config save.

**File-backed tab recovery on startup**

On mount, `QueryPage` iterates all tabs in the store and re-reads file-backed
tabs from disk (SQL files via `ReadFile`, notebooks via `ReadNotebook`),
calling `refreshFileTab` or `orphanFileTab` as appropriate.

**Modal orchestration**

`QueryPage` owns the open/closed state and renders all application-level
modals: `SessionPropertiesModal`, `SnippetsModal`, `ExportPathFormatModal`,
`MigrationModal`, `DbtProjectModal`, `FunctionCatalogModal`,
`KeyboardShortcutsModal`, `AboutModal`, `QueryProfileModal`, and the close-tab
confirmation dialog.

**Recently-closed tab stack**

A `closedTabsRef` captures closed tabs (path, title, SQL, kind) so they can be
reopened with `Cmd+Shift+T` / `Ctrl+Shift+T`.

### Stores consumed

- `queryStore` — tab list, active tab, SQL, results, running state
- `sessionStore` — role/warehouse/database/schema, per-tab context cache
- `connectionStore` — `isConnected`, `disconnect`
- `themeStore` — resolved theme, editor font/size (passed to Monaco)
- `panelLayoutStore` — editor split, sidebar widths
- `featureFlagsStore` — gates terminal, cross-tab search, and other features
- `notebookToolbarStore` — kernel state and callbacks for `NotebookToolbarSlot`
- `gridStore` — `resetNavigation()` called on tab switch

### IPC entry points called directly by QueryPage

All are imported from `wailsjs/go/app/App` or `wailsjs/go/sqleditor/Service`:

`StartQuery`, `WaitForQueryResult`, `CancelQuery`, `GetSqlStatementRanges`,
`GetSessionContext`, `InitTabSession`, `CloseTabSession`, `GetSessionInitMode`,
`ReadFile`, `ReadNotebook`, `NotebookUseContext`, `SaveFile`, `SaveNotebook`,
`PickSaveFile`, `PickOpenFile`, `PickNotebookFile`, `SaveBinaryFile`,
`PickSaveExportFile`, `GetSessionParameters`, `GetSessionVariables`,
`GetCurrentUser`, `GetCurrentRegion`, `GetSnowsightURL`, `Disconnect`.

## Gotchas

- `runQuery` captures `activeTabId` at call time into `runTabId`. All
  subsequent state updates use `runTabId` so results from background queries
  land in the correct tab even if the user switches tabs mid-execution.
- The multi-statement statement index emitted by Go is relative to the
  selection (not the full buffer). `QueryPage` offsets it by
  `selectionBaseStmtIdxRef` to drive editor highlighting correctly for partial
  runs.
- Pending queries (initiated while disconnected) are stored in
  `pendingQueryRef` and re-run once `isConnected` transitions to `true`.
- File-backed tab content is cleared in `queryStore` persistence to avoid
  localStorage quota exhaustion. `QueryPage` re-reads from disk on mount.
