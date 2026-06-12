# frontend/src/components/results

> Virtualised query results grid with column filtering, conditional formatting, search, charting, and query plan modals.

## Responsibility

Renders the results of a Snowflake query as a virtualised TanStack Table v8 grid. Provides
per-column formatting, conditional colour rules, in-grid search, a status bar with selection
statistics, quick charting, EXPLAIN output, and the Query Profile (operator stats). Integrates
with the `gridStore` singleton for shared selection/search/formatting state.

## Files

| File | Purpose |
|------|---------|
| `ResultGrid.tsx` | Main grid component. Uses `@tanstack/react-table` + `@tanstack/react-virtual` for row virtualisation. Manages cell selection (range), right-click context menu (copy cell/row/CSV/JSON), column pinning, column sorting, **drag-to-reorder columns** (TanStack `columnOrder` state — view-only, gated by the `columnReorder` flag), per-column filtering via `ColumnFilterDropdown`, per-column formatting via `DataTypeFormatModal`, conditional formatting via `ConditionalFormattingModal`, and quick-chart via `QuickChartModal`. Exports `ResultGridHandle` (imperative `scrollToRow` and `scrollToCell` — the latter minimally scrolls so a cell is fully visible, accounting for the row-number gutter and pinned columns). Accepts a `standalone` prop. |
| `StatusBar.tsx` | Reads `selectionRange` and `tableRows` from `gridStore`. Shows count, sum, avg, min, max over the selected numeric cells. |
| `CellDetailPanel.tsx` | Side panel on the right edge of the results area showing the full content of the selected cell (the selection anchor — `selectionRange.startRow/startCol`). Opens only for cell-originated selections (`gridStore.selectionOrigin === "cell"`) — row-gutter, column-header, and select-all gestures don't trigger it. Column name, row number, scrollable/selectable monospace text, JSON pretty-printing with a Raw/Formatted toggle, copy via `ClipboardSetText`. Huge values are capped at `DISPLAY_CAP` (500 k chars) with a "show all" affordance, and JSON detection is skipped above `JSON_DETECT_CAP` (1 M chars); copy always uses the full raw value. Closes via ✕ or Escape and reopens when a different cell is selected. When it opens (or switches cells) it calls `onVisibleCellChange`, which QueryPage wires to `ResultGridHandle.scrollToCell` so the selected cell isn't covered by the panel. Width is persisted in `panelLayoutStore.cellDetailWidth` (drag the left edge to resize). Gated behind the `cellDetailPanel` feature flag; needs `multiCellCopy` for cell selection. |
| `cellDetailUtils.ts` | Import-free pure helpers, unit-tested in `cellDetailUtils.test.ts`: `prettyPrintJson`, `truncateForDisplay`, `reconcileDismissedKey` (the panel's dismissal state machine), and `computeCellScrollLeft` (the horizontal scroll-into-view math used by `ResultGridHandle.scrollToCell`). |
| `columnOrderUtils.ts` | Import-free pure helpers for column reordering / visual⇄original translation, unit-tested in `columnOrderUtils.test.ts`: `defaultColumnOrder`, `reorderColumnOrder` (splice-based move before/after a target ID), `visualToOriginalIndex` (translate a visual column position to its original SELECT index), and `columnIdFor`. |
| `GridSearch.tsx` | In-grid text search panel. Reads/writes `searchTerm`, `searchMatches`, `currentMatchIndex` in `gridStore`. Debounces search recomputation. Calls `onScrollToRow` to virtualise-scroll to the match. |
| `ColumnFilterDropdown.tsx` | Per-column filter popover. Supports value checklist (up to 500 distinct values) and conditional operators (`contains`, `startsWith`, `endsWith`, `equals`, `gt`, `lt`, `gte`, `lte`). Exports `ColumnFilterValue` type and `columnFilterFn` (TanStack `FilterFn`). |
| `ConditionalFormattingModal.tsx` | Modal for adding/removing per-column colour-scale rules. Reads/writes `conditionalRules` in `gridStore`. Preset colour palettes (Green→Red, Blue→Red, etc.). |
| `DataTypeFormatModal.tsx` | Modal for per-column display formatting. Auto-detects column type (number/datetime/string) from sample values. Reads/writes `formatConfigs` in `gridStore`. Exports `applyFormat(value, config)` used by `ResultGrid`. |
| `QuickChartModal.tsx` | Modal rendering bar, line, or scatter charts (Recharts) over the selected grid range. |
| `ExplainModal.tsx` | Modal that calls `RunExplain` IPC, then renders a flattened EXPLAIN plan as an Ant Design Table with step/operation/cost/rows/bytes columns. |
| `QueryProfileModal.tsx` | Modal that calls `GetQueryOperatorStats` IPC. Shows operator-level stats (type, execution time, rows, bytes, spilling) with `liveRefresh` auto-poll every 3 s while a query is still running. |
| `QueryLogPane.tsx` | Session-scoped query log panel. Subscribes to `querylog:entry` and `querylog:update` Wails events for live updates. Calls `GetQueryLogEntries` and `ClearQueryLog` IPC. Provides source/status filtering, text search, and copy-to-clipboard formatting for debugging and issue reporting. Gated behind the `queryLog` feature flag. |

## Patterns & integration

**IPC calls:**
- `ExplainModal.tsx` — `RunExplain` from `wailsjs/go/app/App`.
- `QueryProfileModal.tsx` — `GetQueryOperatorStats` from `wailsjs/go/app/App`.
- `QueryLogPane.tsx` — `GetQueryLogEntries`, `ClearQueryLog` from `wailsjs/go/app/App`; `ClipboardSetText` from `wailsjs/runtime/runtime`.
- `ResultGrid.tsx` — `ClipboardSetText` from `wailsjs/runtime/runtime` (WKWebView clipboard workaround).
- `CellDetailPanel.tsx` — `ClipboardSetText` from `wailsjs/runtime/runtime` (copy button).

**Stores used:**
- `gridStore` — `selectionRange` (column bounds are **visual** positions), `columnVisualOrder`
  (visual→original index map, set by `ResultGrid`, read by `StatusBar`/`CellDetailPanel`),
  `tableRows`, `searchTerm`, `searchMatches`, `currentMatchIndex`,
  `conditionalRules`, `formatConfigs`, `nextMatch`, `prevMatch`, `setSearchTerm`,
  `setSearchMatches`, `setTableRows`, `setColumnVisualOrder`, `resetGrid`, `resetNavigation`,
  `setConditionalRules`, `clearConditionalRules`, `setFormatConfig`.
- `themeStore` — dark/light theming for grid cell colours.
- `featureFlagsStore` — gating of optional result features.
- `panelLayoutStore` — `cellDetailWidth`/`setCellDetailWidth` (persisted width of `CellDetailPanel`).

**`standalone` prop on `ResultGrid`:** When `true`, the component skips all writes to `gridStore`
(`setTableRows`, `resetGrid`, `resetNavigation`). Use this for embedded grids in notebook SQL
cells to prevent contaminating the main query tab's `StatusBar` and `GridSearch` state.

**Column IDs:** TanStack column IDs follow the pattern `{colIndex}_{COLUMN_NAME}` (e.g. `3_COL_NAME`).
Conditional rules and format configs are keyed by this ID. The helper `colIdxFromId` extracts the
0-based index. This means rules keyed on `0_ID` can match across tabs with identically-named
columns — a known limitation of the singleton `gridStore`.

**Column reordering:** Dragging a column header (the hover grip handle) reorders columns via
TanStack's `columnOrder` state — a list of the stable `{colIndex}_{NAME}` IDs. It is **view-only**:
`result.columns`/`result.rows` are never touched, so sort, filter, format, and conditional rules
(all keyed off the stable column ID) follow each column to its new position. Reordering is confined
to the unpinned (center) region — pinned headers are neither draggable nor drop targets, so
pinned-left/right groups keep their edges. The order lives in local component state (like
pinning/sizing): it resets to SELECT order on a column schema change but is preserved across a
re-run of the same query. The header context menu offers **Move Column Left/Right** (a keyboard- and
screenreader-reachable alternative to dragging) and **Reset Column Order**. Pure reorder logic lives
in `columnOrderUtils.ts` (unit-tested). Gated behind the `columnReorder` feature flag.

**Visual vs. original column indices:** Range selection (and therefore copy, the StatusBar
aggregations, Quick Chart, and the Cell Detail Panel) is tracked in **visual** column positions
(left-to-right on screen), because columns can be reordered *and* pinned — visual order ≠ SELECT
order. `ResultGrid` builds a `visualToOriginal` map from `table.getVisibleLeafColumns()` and
publishes it to `gridStore.columnVisualOrder`. Selection handlers convert original→visual via
`originalToVisual`; every data read converts back with `visualToOriginalIndex(map, visualPos)` so
highlight, aggregation, and copy cover exactly the columns the user swept, and copy emits in visual
order. When no reorder/pinning is active the map is the identity, so behaviour is unchanged.

**Column width measurement:** `computeColumnWidths` and `measureText` from `../../utils/gridMeasure`
are called after data loads to auto-size columns based on header and sample cell content.

## Gotchas

- **`gridStore` is a singleton** — formatting rules, search state, selection range, conditional
  rules, and `tableRows` are shared across tabs. They reset when switching tabs or running a query
  in another tab. In side-by-side compare mode, both `ResultGrid` instances call `setTableRows()`
  and the last to render wins (affecting `StatusBar`/`GridSearch`).
- **Notebook cells** must always pass `standalone={true}` to `ResultGrid` to avoid contaminating
  main tab state.
- **Clipboard** — `navigator.clipboard` is blocked in WKWebView; `ResultGrid` uses
  `ClipboardSetText` from the Wails runtime for all copy operations.
- **`ColumnFilterDropdown`** caps the distinct-value checklist at 500 entries (`truncated` prop
  signals this to the user); filtering beyond 500 unique values falls back to the conditional
  operator mode.
- **`QueryProfileModal`** with `liveRefresh` polls every 3 s; ensure the modal is closed (or
  `liveRefresh` set to `false`) when the query completes to stop the interval.
