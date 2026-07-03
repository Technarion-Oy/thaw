# frontend/src/utils

> Shared, framework-agnostic helper modules used across the frontend.

## Responsibility

Provides pure utility functions and small shared data structures that do not belong to any single component or store. Modules here have no React dependencies and import only from Wails runtime bindings or third-party libraries.

## Files

| File | Purpose |
|------|---------|
| `taskHierarchy.ts` | Parses the `predecessors` column returned by `SHOW TASKS` — handles the several non-standard string formats Snowflake uses (bare string, JSON array, bracket-delimited quoted list). Exports `parsePredecessors(raw)` and `extractName(ref)`. |
| `timezones.ts` | Static lookup table of every Snowflake-supported IANA timezone name with its standard UTC offset, typed as `TzOption[]`. Used by the cron-schedule timezone selector in task modals. |
| `monacoClipboard.ts` | `patchMonacoClipboard(editor)` routes the **code buffer's** copy / cut / paste through Wails' `ClipboardGetText` / `ClipboardSetText` native APIs (required because `navigator.clipboard` is blocked inside WKWebView on macOS). Its capture-phase Cmd/Ctrl+V/C/X handler acts **only** when the editor's text input is focused (Monaco's public `codeEditor.hasTextFocus()`); for find/replace/rename fields it lets the event bubble (no `stopPropagation`) to the global handler in `App.tsx`. Called from every editor's `onMount`. |
| `monacoTooltipFix.ts` | `registerFindWidgetTooltipFix(monaco.editor)` — registers a one-time global `onDidCreateEditor` hook that patches Monaco's base-layer hover service (`_createHover`) so find-widget button tooltips render below their target instead of the default above, avoiding the tab-bar clip (issue #593). Called from `ensureMonacoSetup`, so it's decoupled from the clipboard wiring and covers every editor Monaco creates. Warns once and stops if `_createHover` is unavailable (e.g. a Monaco version bump). |
| `sqlFormatter.ts` | Two-pass SQL formatter: structural pass via `sql-formatter` (Snowflake dialect) for indentation, line breaks, CTE layout, and operator/comma placement; then a casing pass via the `sqleditor.Service.ApplySqlCasing` IPC method for keyword, identifier, and function casing. Exports `EditorPrefs`, `DEFAULT_EDITOR_PREFS`, and `formatSQL(sql, prefs)`. |
| `gridMeasure.ts` | Canvas-based text measurement and column auto-sizing. `measureText(text, font)` caches `CanvasRenderingContext2D` instances by font string (up to 10 entries). `computeColumnWidths(columns, rows, opts)` samples the first N rows and returns pixel-clamped widths for each column. Used by `ResultGrid` and `PipeCopyHistoryModal`. |
| `sqlFormatter.test.ts` | Vitest unit tests for `sqlFormatter.ts`, covering keyword/identifier/function casing, indent style, comma position, operator position, Snowflake `::` cast and `:` variant path operators, CTEs, string literal and comment passthrough, and idempotency. Mocks `ApplySqlCasing` inline to avoid a live Wails runtime. |
| `formatBytes.ts` | `formatBytes(bytes)` → human-readable size string with binary (1024) units and one decimal place (e.g. `1.5 KB`). Used by the stage browsers (`StageBrowserModal`, the External Table location picker). Components needing different rounding/zero-handling (e.g. `ExplainModal`, `ObjectSummariesModal`) keep their own variants by design. |
| `objectDdl.ts` | `DDL_UNSUPPORTED_KINDS` set + `kindSupportsDdl(kind)` — the object kinds Snowflake's `GET_DDL` cannot render (IMAGE REPOSITORY, SERVICE, GATEWAY, PACKAGES POLICY, MODEL, MODEL MONITOR, DATASET, CORTEX SEARCH SERVICE, EXTERNAL AGENT, MCP SERVER). Mirror of the guard in `internal/snowflake/client.go GetObjectDDL`; every frontend DDL entry point (`Sidebar` hover, `SqlEditor` cmd/ctrl-hover) checks it before firing a doomed `GET_DDL`. Keep in sync with the Go list. |
| `modalDragResize.ts` | Side-effect module (imported once in `main.tsx`) that installs a single document-level `mousedown`/`mousemove`/`mouseup` delegate to make **every** Ant Design modal draggable — no per-Modal props. The handle is the `.ant-modal-header` plus the content's top padding band (so the very top edge grabs), minus interactive controls; the drag is applied as inline `left`/`top` on the `.ant-modal` box (not `transform`, which would break `position: fixed` descendants such as the in-modal context menus), clamped so the header stays on-screen. A lost mouseup (`e.buttons === 0` / window `blur`) is recovered, and a `window` `resize` listener re-clamps any moved modal that a shrinking window would strand off-screen. Width resize is pure CSS (`resize: horizontal` on `.ant-modal`, in `styles/global.css`) — this module owns only the drag. Because Thaw's modals unmount on close, position and width reset to default on reopen. Issue #572. |

## Patterns & integration

- **No React imports** — all modules are plain TypeScript; they can be imported from any component, store, or other utility.
- `sqlFormatter.ts` is async because `ApplySqlCasing` is an IPC call; callers must `await formatSQL(...)`.
- `monacoClipboard.ts` patches via Monaco's internal `_commandService` and a capture-phase `keydown` listener — call it once after `editor.onDidMount`, not on every render.
- `gridMeasure.ts` keeps a module-level canvas cache. The cache is cleared when it reaches 10 font entries to prevent unbounded growth.
- `timezones.ts` exports a plain `const` array; there is no lazy loading — the full list (~600 entries) is bundled.

## Gotchas

- `monacoClipboard.ts` patches both child editors when given a `DiffEditor`; apply it only once — re-patching wraps `executeCommand` again and breaks the chain.
- `sqlFormatter.ts` silently falls back to the original SQL if either the structural pass or the casing IPC call throws (e.g. on dollar-quoted `$…$` bodies that `sql-formatter` cannot parse).
- The canvas cache in `gridMeasure.ts` is module-level and shared across all callers; never call it from a worker thread.
