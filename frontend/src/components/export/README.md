# frontend/src/components/export

> DDL export panel and table data import/export modals.

## Responsibility

Covers two distinct concerns:

1. **DDL export** (`ExportPanel`, `ExportPathFormatModal`): exports all or selected Snowflake database schemas as `.sql` files to a local directory, streaming progress via `ddl:progress` Wails events.
2. **Table data export/import** (`ExportTableModal`, `ImportTableModal`): unloads a table to local files (CSV/JSON/Parquet) or loads local files into an existing or new table using Snowflake's COPY INTO mechanism.

## Files

| File | Purpose |
|---|---|
| `ExportPanel.tsx` | Sidebar panel for DDL export. Shows a directory picker (backed by `gitStore.exportDir`), a scrollable database checklist (loaded from `ListExportableDatabases`), Export / Cancel buttons, a live `Progress` bar fed by `ddl:progress` events, and a collapsible per-database results summary after completion. Dispatches `thaw:export-complete` custom event when done so other components can react. |
| `ExportPathFormatModal.tsx` | Configures the DDL export file path template (e.g. `{database}/{schema}/{object_type}/{object_name}.sql`). Shows clickable variable tags and a live preview substituted with example values. Persists via `gitStore.saveExportPathTemplate(tmpl)` (atomic, field-scoped `SaveGitExportPathTemplate` IPC — not the whole-struct `saveConfig`). |
| `ExportTableModal.tsx` | Exports a single table's data via `ExportTableData` IPC. Format: CSV (delimiter, header, null string), JSON, or Parquet (Snappy/None/Zstd). Output directory picked via `PickDirectory`. Can also be opened at schema level with an empty `table` prop, in which case it shows a table selector populated by `ListObjects`. Shows a success state with row count and file list after completion. |
| `ImportTableModal.tsx` | Imports local files into a Snowflake table via `ImportTableData` IPC. Supports CSV, JSON, AVRO, ORC, and Parquet; auto-detects format from file extension. Multi-file selection via `PickDataFilesByFormat`. Inline CSV/JSON file preview (first 64 KB via `ReadFileHead`, parsed/raw toggle, tabbed for multiple files). Format options via `FileFormatFields` (reused component) or a named file format from `ListFileFormats`. Supports CREATE TABLE from data (schema inference) and TRUNCATE + load (overwrite). |

## Patterns & integration

- **IPC**: `ExportAllDatabasesDDL`, `CancelExport`, `ListExportableDatabases`, `RevealInFinder`, `ExportTableData`, `ImportTableData`, `PickDataFilesByFormat`, `ReadFileHead`, `ListFileFormats`, `ListObjects`, `PickDirectory` — all from `wailsjs/go/app/App`.
- **Events**: `ExportPanel` subscribes to `ddl:progress` Wails events to stream per-object export progress; the subscription is established immediately before calling `ExportAllDatabasesDDL` and torn down in the `finally` block.
- **Export directory**: `ExportPanel` reads `exportDir` from `gitStore` (shared with the Git panel and `FileBrowser`). The directory picker calls `gitStore.pickExportDir()`.
- **`thaw:export-complete`**: dispatched as a DOM `CustomEvent` after export finishes; `FileBrowser` (and potentially other components) listen for it to refresh the file tree.
- **Platform label**: `ExportPanel` uses `platformUtil.getPlatformOS()` / `revealLabel()` to show the correct OS-specific label ("Reveal in Finder" / "Show in Explorer" / "Show in File Manager") on the reveal button.

## Gotchas

- Passing `table=""` to `ExportTableModal` switches it into schema-level mode (shows a table selector); passing a non-empty string locks it to that table.
- `ImportTableModal` loads file previews lazily via `ReadFileHead(path, 65536)` and caches them in a `fileHeads` map keyed by path. The `pendingLoads` ref prevents duplicate IPC calls for the same path.
- The `ddl:progress` listener is set up inside `exportSelected()` (not in `useEffect`) and is explicitly cleaned up with `off()` in the `finally` block — do not move it to a mount-time effect or it will miss the initial events.
- `CancelExport()` is a fire-and-forget IPC call; the backend signals completion by resolving the `ExportAllDatabasesDDL` promise.
