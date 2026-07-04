# frontend/src/components/pipe

> Modals for managing Snowflake Pipe objects: create, view properties/status, refresh, and browse COPY history.

## Responsibility

Provides the full lifecycle UI for Snowflake Pipe objects. `CreatePipeModal` follows the
standard debounced SQL preview pattern. The remaining modals display read-only operational
data (status, copy history) or trigger management actions (refresh).

## Files

| File | Purpose |
|------|---------|
| `CreatePipeModal.tsx` | `CREATE PIPE` form with auto-ingest toggle, notification integrations, AWS SNS topic, and an embedded Monaco editor for the `COPY INTO` statement body. Uses `BuildCreatePipeSql` for live SQL preview. |
| `PipePropertiesModal.tsx` | Displays SHOW PIPES properties for an existing pipe; the Settings section adds an inline-editable Comment and the shared `TagsRow` editor (`SET`/`UNSET TAG` via `AlterPipe`). |
| `PipeStatusModal.tsx` | Shows live pipe status from `SYSTEM$PIPE_STATUS`, formatted as execution state, pending files, error count, and last ingested time. |
| `PipeCopyHistoryModal.tsx` | Paginated table of `COPY_HISTORY` records for a pipe: file name, row count, status, load time. |
| `RefreshPipeModal.tsx` | Confirms and triggers `ALTER PIPE … REFRESH` (optionally with a prefix filter and error-on-execute flag). |

## Patterns & integration

**IPC calls:**
- `BuildCreatePipeSql(db, schema, cfg)` — debounced SQL preview (no explicit timer; direct `useEffect` dependency)
- `ExecDDL(preview)` — executes the CREATE PIPE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `ListNotificationIntegrations()` — populates the error integration and integration selects

**`pipe.PipeConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`, `orReplace`, `ifNotExists`, `autoIngest`, `errorIntegration`, `awsSnsTopic`, `integration`, `comment`, `copyStatement`.

**Monaco in-modal:** `CreatePipeModal` embeds a 120px Monaco editor (SQL language) for the `COPY INTO` statement body. `patchMonacoClipboard` is applied on mount to handle WKWebView clipboard restrictions.

**Shared components:** `ObjectNameCaseControl` for case-sensitivity; inline SQL preview block.

**Stores used:** `themeStore` (Monaco editor theme: `vs-dark` / `vs`).

## Gotchas

- `BuildCreatePipeSql` is called on every `cfg` state change without an explicit debounce timer (unlike the dbtproject modals which use a 200 ms debounce ref). Rapid typing in the COPY INTO editor generates frequent IPC calls.
- The `copyStatement` field defaults to `"COPY INTO my_table\n  FROM @my_stage"` so the SQL preview is always non-empty on first open.
