# frontend/src/components/export

> DDL export panel and table data import/export modals.

## Responsibility

Covers two distinct concerns:

1. **DDL export** (`ExportOptionsModal`, `ExportPathFormatModal`): exports all or selected Snowflake database schemas as `.sql` files to a local directory, streaming progress via `ddl:progress` Wails events. Opened via **Tools → Export Database DDL…** (`menu:export-ddl` event, rendered from `QueryPage`); there is no sidebar export panel.
2. **Table data export/import** (`ExportTableModal`, `ImportTableModal`): unloads a table to local files (CSV/JSON/Parquet) or loads local files into an existing or new table using Snowflake's COPY INTO mechanism.
3. **Resultset Excel export** (`ExcelExportModal`): picks which resultsets from a tab's in-memory result history to export into a single multi-sheet `.xlsx` workbook.

## Files

| File | Purpose |
|---|---|
| `ExportOptionsModal.tsx` | The full DDL-export dialog (Tools → Export Database DDL…). Output-directory picker (backed by `gitStore.exportDir` / `pickExportDir`), database multi-select dropdown (from `ListExportableDatabases`; empty = all databases), schema `tags`-mode dropdown (suggestions are qualified `DB.SCHEMA` values from `ListUserSchemas` across the databases the export covers — selected ones or all; free-typed bare names match in every database; empty = all schemas), object-type checkboxes (all checked by default; full selection is sent as an empty filter), per-export path template override (prefilled from `gitStore.exportPathTemplate`), overwrite-vs-skip radio for existing files, and a warehouse `Select` (from `sessionStore.warehouses` / `loadWarehouses()`; empty = session warehouse). Runs the export in-dialog: `ExportAllDatabasesDDL(exportDir, databases, opts as any)` (cast per the Wails request-class gotcha), live `Progress` bar from `ddl:progress` events, Cancel Export button (`CancelExport`), collapsible per-database results summary with a reveal-in-file-manager button, and a `thaw:export-complete` DOM event on finish. The dialog cannot be closed while an export is running. |
| `ExportPathFormatModal.tsx` | Configures the DDL export file path template (e.g. `{database}/{schema}/{object_type}/{object_name}.sql`). Shows clickable variable tags and a live preview substituted with example values. Persists via `gitStore.saveExportPathTemplate(tmpl)` (atomic, field-scoped `SaveGitExportPathTemplate` IPC — not the whole-struct `saveConfig`). |
| `ExportTableModal.tsx` | Exports a single table's data via `ExportTableData` IPC. Format: CSV (delimiter, header, null string), JSON, or Parquet (Snappy/None/Zstd). Output directory picked via `PickDirectory`. Can also be opened at schema level with an empty `table` prop, in which case it shows a table selector populated by `ListObjects`. Shows a success state with row count and file list after completion. |
| `ExcelExportModal.tsx` | Multi-select dialog for Excel resultset export. Opened from `QueryPage`'s "Export as Excel" button when the tab holds two or more results (a single result exports directly, no dialog). Lists the tab's `resultHistory` entries (most-recent-first, all selected by default) as a checkbox list with a **Select all** master toggle; each row previews the derived sheet name and row count. On export it returns the selected `HistoryEntry` ids to `QueryPage`, which builds the workbook — one sheet per resultset via `XLSX.utils.book_append_sheet` — using `deriveSheetName` (`frontend/src/utils/excelSheetName.ts`) for 31-char-capped, invalid-char-stripped, de-duplicated names. No new IPC — reuses the existing `PickSaveExportFile` + `SaveBinaryFile` save path. |
| `ImportTableModal.tsx` | Imports local files into a Snowflake table via `ImportTableData` IPC. Supports CSV, JSON, AVRO, ORC, and Parquet; auto-detects format from file extension. Multi-file selection via `PickDataFilesByFormat`. Inline CSV/JSON file preview (first 64 KB via `ReadFileHead`, parsed/raw toggle, tabbed for multiple files). Format options via `FileFormatFields` (reused component) or a named file format from `ListFileFormats`. Supports CREATE TABLE from data (schema inference) and TRUNCATE + load (overwrite). |

## Patterns & integration

- **IPC**: `ExportAllDatabasesDDL`, `CancelExport`, `ListExportableDatabases`, `ListUserSchemas`, `RevealInFinder`, `ExportTableData`, `ImportTableData`, `PickDataFilesByFormat`, `ReadFileHead`, `ListFileFormats`, `ListObjects`, `PickDirectory` — all from `wailsjs/go/app/App`.
- **Events**: `ExportOptionsModal` subscribes to `ddl:progress` Wails events to stream per-database export progress; the subscription is established immediately before calling `ExportAllDatabasesDDL` and torn down in the `finally` block.
- **Export directory**: `ExportOptionsModal` reads `exportDir` from `gitStore` (shared with the Git panel and `FileBrowser`). The directory picker calls `gitStore.pickExportDir()`.
- **`thaw:export-complete`**: dispatched as a DOM `CustomEvent` after export finishes; `FileBrowser` (and potentially other components) listen for it to refresh the file tree.
- **Platform label**: `ExportOptionsModal` uses `platformUtil.getPlatformOS()` / `revealLabel()` to show the correct OS-specific label ("Reveal in Finder" / "Show in Explorer" / "Show in File Manager") on the reveal button.

## Gotchas

- Passing `table=""` to `ExportTableModal` switches it into schema-level mode (shows a table selector); passing a non-empty string locks it to that table.
- `ImportTableModal` loads file previews lazily via `ReadFileHead(path, 65536)` and caches them in a `fileHeads` map keyed by path. The `pendingLoads` ref prevents duplicate IPC calls for the same path.
- The `ddl:progress` listener is set up inside `runExport()` (not in `useEffect`) and is explicitly cleaned up with `off()` in the `finally` block — do not move it to a mount-time effect or it will miss the initial events.
- `CancelExport()` is a fire-and-forget IPC call; the backend signals completion by resolving the `ExportAllDatabasesDDL` promise.
