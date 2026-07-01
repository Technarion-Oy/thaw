# frontend/src/utils

> Shared, framework-agnostic helper modules used across the frontend.

## Responsibility

Provides pure utility functions and small shared data structures that do not belong to any single component or store. Modules here have no React dependencies and import only from Wails runtime bindings or third-party libraries.

## Files

| File | Purpose |
|------|---------|
| `taskHierarchy.ts` | Parses the `predecessors` column returned by `SHOW TASKS` — handles the several non-standard string formats Snowflake uses (bare string, JSON array, bracket-delimited quoted list). Exports `parsePredecessors(raw)` and `extractName(ref)`. |
| `timezones.ts` | Static lookup table of every Snowflake-supported IANA timezone name with its standard UTC offset, typed as `TzOption[]`. Used by the cron-schedule timezone selector in task modals. |
| `fieldClipboard.ts` | Shared native `<input>`/`<textarea>` clipboard helpers used by both the app-wide Cmd/Ctrl+V/C/X handler (`App.tsx`) and `monacoClipboard.ts`. `spliceFieldValue` writes through the native value setter so React-controlled inputs fire `onChange`, then dispatches `input` so non-React listeners (Monaco's find widget) also react; `fieldSelectionText` reads the current selection; `isMonacoCodeSurface` is the single source of truth for "is this Monaco's own code-editing surface (`.inputarea`)" vs. an ordinary editable field (find/replace/rename) that merely lives inside `.monaco-editor`. |
| `monacoClipboard.ts` | Patches a Monaco `IStandaloneCodeEditor` (or diff editor) so the **code buffer's** copy / cut / paste route through Wails' `ClipboardGetText` / `ClipboardSetText` native APIs (required because `navigator.clipboard` is blocked inside WKWebView on macOS). The capture-phase Cmd/Ctrl+V/C/X handler acts **only** when the focused element is the code surface (`isMonacoCodeSurface`); for find/replace/rename fields it lets the event bubble to the global handler in `App.tsx`, so every Monaco mount is covered even when it never calls `patchMonacoClipboard` (notebook cells, the read-only diff view). The find-widget tooltip fix (`forceHoverTooltipsBelow`) lives in `components/editor/monacoSetup.ts`, wired globally via `onDidCreateEditor`. |
| `sqlFormatter.ts` | Two-pass SQL formatter: structural pass via `sql-formatter` (Snowflake dialect) for indentation, line breaks, CTE layout, and operator/comma placement; then a casing pass via the `sqleditor.Service.ApplySqlCasing` IPC method for keyword, identifier, and function casing. Exports `EditorPrefs`, `DEFAULT_EDITOR_PREFS`, and `formatSQL(sql, prefs)`. |
| `gridMeasure.ts` | Canvas-based text measurement and column auto-sizing. `measureText(text, font)` caches `CanvasRenderingContext2D` instances by font string (up to 10 entries). `computeColumnWidths(columns, rows, opts)` samples the first N rows and returns pixel-clamped widths for each column. Used by `ResultGrid` and `PipeCopyHistoryModal`. |
| `sqlFormatter.test.ts` | Vitest unit tests for `sqlFormatter.ts`, covering keyword/identifier/function casing, indent style, comma position, operator position, Snowflake `::` cast and `:` variant path operators, CTEs, string literal and comment passthrough, and idempotency. Mocks `ApplySqlCasing` inline to avoid a live Wails runtime. |
| `formatBytes.ts` | `formatBytes(bytes)` → human-readable size string with binary (1024) units and one decimal place (e.g. `1.5 KB`). Used by the stage browsers (`StageBrowserModal`, the External Table location picker). Components needing different rounding/zero-handling (e.g. `ExplainModal`, `ObjectSummariesModal`) keep their own variants by design. |

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
