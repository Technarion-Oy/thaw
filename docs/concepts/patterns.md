# Key patterns

Reusable patterns used throughout Thaw. Per-module specifics live in each folder's `README.md`; this page is the cross-cutting reference.

## Adding a Go → frontend IPC method

1. Add a public method on `*App` (receiver `a *App`) to the `internal/app/<domain>.go` file matching its domain (query methods → `query.go`, object listing → `objects.go`). All files in `internal/app/` are `package app`, so methods bind regardless of which file they live in. Keep `app.go` for the `App` struct, lifecycle, and session management; `run.go`/`menu.go` hold the Wails wiring and native menu.
2. Run `wails generate module` to regenerate `frontend/wailsjs/`.
3. Import from `"../../../wailsjs/go/app/App"` in the component.

## Thin-delegator pattern

`*App` methods should be **thin delegators**: a nil-check, one call into a domain package, and a return. All real logic — SQL string building, `snowflake.QueryResult` parsing, validation, key generation, multi-step orchestration — lives in `internal/<domain>` packages so it is independently unit-testable without a live connection or a bound `App`.

```go
// internal/app/warehouse.go — thin delegator
func (a *App) GetWarehouseMeteringHistory(wh, start, end string) ([]warehouse.WarehouseMeteringRow, error) {
    if a.client == nil { return nil, apperrors.ErrNotConnected }
    return warehouse.GetMeteringHistory(a.ctx, a.client, wh, start, end)
}
```

Conventions for the domain package:

- High-level funcs take `(ctx context.Context, client *snowflake.Client, …)` and return **domain types** (e.g. `warehouse.WarehouseMeteringRow`, `backup.BackupRow`, `table.TableSettings`, `keypair.KeyPairResult`, `queryhistory.QueryHistoryRow`, `objects.ColumnComment`). Types live in their domain package, **not** in `internal/app`.
- Pure helpers are split out for testing: `Build*Sql(...) (string, error)` builders and `Parse*(res *snowflake.QueryResult) […]` parsers. `App` calls `client.Execute()` / `ExecDDL()`.
  - **Validated-wrapper variant** (`queryhistory`): when a builder embeds a value that must be pre-validated (e.g. a `SESSION_ID` embedded unquoted as a bare integer), keep the builder **unexported** (`buildQueryHistorySql`) and route all external access through the exported `Get*` wrapper, which validates first and returns an error. The builder treats the precondition as an invariant — an invalid value reaching it is a programmer error and **panics**, rather than silently producing a wrong query.
- Shared result-parsing helpers live in `internal/snowflake/result.go` (`ColIdx`, `CellString/CellFloat/CellInt64/CellBool`, `PropertyPair{Key,Value}`, `ResultToPairs`, `SessionParam/SessionVar`, `QuoteSessionParamValue`).
- **Exceptions that stay in `internal/app`** (coupled to `App` state / Wails event emission / goroutine orchestration): `StartQuery`, `WaitForQueryResult`, `CancelQuery`, `RunExplain`, the shell PTY methods, `ExportDatabaseDDL`/`ExportAllDatabasesDDL`, `GetSessionContext`, and the editor-prefs/session-config backfill getters.
- Frontend model refs use the new TS namespaces (`backup.`, `objects.`, `warehouse.`, `table.`, `keypair.`, `queryhistory.`, and `snowflake.` for `PropertyPair`/`SessionParam`/`SessionVar`); only `app.AppInfo` remains in the `app` namespace.

## Emitting events Go → frontend

```go
wailsruntime.EventsEmit(a.ctx, "event:name", payload)
```
```ts
const cleanup = EventsOn("event:name", (data) => { ... });
// call cleanup() on unmount
```

## Zustand stores

- `connectionStore` — active connection, role, warehouse, database
- `queryStore` — SQL tabs, results, selected SQL, active query
- `objectStore` — sidebar tree (databases, schemas, objects); also an instant cache for the search cascade
- `themeStore` — light/dark/system + editor font/size
- `sessionStore` — persisted session state
- `panelLayoutStore` — persisted panel sizes
- `diffStore` — DDL diff comparisons
- `gitStore` — git repo state
- `notebookToolbarStore` — bridges `NotebookTab` kernel state/callbacks to the unified Toolbar
- `gridStore` — results grid selection range, search, column formatting, conditional formatting (a **singleton** — see [gotchas](gotchas.md))
- `featureFlagsStore` — feature flags, loaded on startup, reloaded after the modal saves
- `mcpStore` — MCP session state

## Adding a feature flag

Flags live in `internal/config/config.go` (`FeatureFlags`) and surface via **View → Enabled Features…**. All flags default to enabled — the `Initialized` sentinel prevents Go's zero-value `false` from silently disabling features on a fresh install.

1. **`internal/config/config.go`** — add a `bool` field to `FeatureFlags`, set it `true` in `DefaultFeatureFlags()`, bump `flagsVersion`, and add migration logic to `MigrateFlags()`.
2. **`wails generate module`** — regenerates `frontend/wailsjs/go/models.ts`.
3. **`FeatureFlagsModal.tsx`** — add a `<FlagRow>` (with `locked={locked.myNewFeature}`) in the right category.
4. **In the gated component** — read the flag from `featureFlagsStore` and pass `disabled` + `disabledReason` to `menuItem` (Sidebar), or conditionally render/disable your UI. When a flag is `false`, the feature must be HIDDEN or DISABLED.

Also update `FEATURES.md` (and its "Feasible Optional Features" section if toggleable).

### IT-admin enforced policies

Admins can lock flags per platform — macOS managed plist (`Disable<Feature> = true`), Windows Group Policy registry (`Disable<Feature> = 1`), Linux `/etc/thaw/features.json`. Locked flags show a lock icon and can't be changed. Logic in `internal/config/adminconfig.go`; IPC `GetAdminLockedFlags()`.

## Unified Toolbar

`<Toolbar />` (`frontend/src/components/toolbar/Toolbar.tsx`) renders execution controls, action buttons, session selectors (role/warehouse/database/schema), and connection info. Context additions go through the `contextSlot` prop; the notebook adds kernel status via `NotebookToolbarSlot` and Deploy via `primaryAction`. The Toolbar reads session state directly from `connectionStore`/`sessionStore` — no prop drilling.

## Sidebar tree node-key formats

Key prefixes identify node types:

- **Columns**: `col:DB:SCHEMA:TABLE:COLUMN` — leaf nodes under `obj:` TABLE/VIEW nodes; carry props for the context menu. **All column DDL is built in the backend `internal/column` package** — the Sidebar and `AddColumnModal` only collect config and call the builder, never construct SQL inline. Datatype/collation reference data also comes from the backend (`internal/snowflake/collations.go`). Altering actions are gated behind the `columnManagement` flag.
- **Git Repos**: `obj:DB:SCHEMA:GIT REPOSITORY:NAME` → `gitbranches:`/`gittags:`/`gitcommits:` → `gitdir:…` → `gitfile:…`
- **Stages**: `obj:DB:SCHEMA:STAGE:NAME` → `stagedir:…` → `stagefile:…`
- **DBT Projects**: `obj:DB:SCHEMA:DBT PROJECT:NAME` → `dbtversion:…` → `dbtdir:…` → `dbtfile:…`

Loading state lives in the shared `loadingGitNodes` Set (namespaced keys). `buildEntryNodes(...)` is the shared stage/DBT helper; `emptyChildNode` is the empty-state placeholder.

## Object-listing cache (backend)

`Client` (`internal/snowflake/client.go`) has a per-schema 30 s TTL cache for `ListObjects`/`ListBasicObjects`, keyed by `"DB\x00SCHEMA"` (full) and `"basic\x00DB\x00SCHEMA"` (basic). `getObjectCache` returns `slices.Clone()` to avoid corrupting the backing array. `ClearObjectCache()` / `ClearObjectCacheForDatabase(db)` are IPC methods. The sidebar search cascade is three-tier: `objectStore` (instant) → Go TTL cache → `ListBasicObjects` fallback.

## Monaco SQL editor

- Main component: `frontend/src/components/editor/SqlEditor.tsx`; pure helpers in `sqlEditorUtils.ts`.
- `getQualifiedIdent(model, pos)` extracts dotted identifiers; `getStatementLineRanges(sql)` mirrors the Go splitter.
- Module-level caches: `hoverDDLCache` (60 s TTL), `fetchedSchemaObjects` Set.
- **Never register completion/hover providers inside render** — use module-level disposable refs.
- SQL analysis (diagnostics, JOIN suggestions, autocomplete context, ref resolution) goes through the backend `sqleditor.Service` (imported from `wailsjs/go/sqleditor/Service`). The frontend resolves nothing inline; `GetAutocompleteContextFull` returns resolved refs, in-editor CREATE TABLE columns, and context-detection flags in one round-trip. Only `node-sql-parser`-dependent checks (`validateWithParser`, `validateBareColumnRefs`) stay in `sqlDiagnostics.ts`.

## Cross-tab search & replace

`CrossTabSearch` (`components/editor/CrossTabSearch.tsx`) renders between the TabBar and editor, triggered by `⌘⇧H`, gated behind `crossTabSearch`. It searches all tabs (and notebook cells via parsed Jupyter JSON), navigates via the `thaw:scroll-to-line` event (waiting for `thaw:editor-ready` with a 500 ms fallback), and routes replaces on the active non-notebook tab through `editor.executeEdits()` so Monaco's undo stack works.

## File system watcher

`internal/filesystem/watcher.go` (`Watcher`) installs a single recursive watch (`rjeczalik/notify`: FSEvents/macOS, `ReadDirectoryChangesW`/Windows, inotify/Linux) over the whole tree, filters out hidden dirs per-event, and debounces 200 ms per directory. The recursive watch avoids the per-directory file-descriptor exhaustion that broke opening large trees (e.g. a `venv`) on macOS (issue #485). Write events on existing files are emitted as well, so external edits propagate to open editor tabs. `StartFileWatcher(dir)`/`StopFileWatcher()` IPC emit `"fs:changed"` events. `FileBrowser.tsx` starts/stops on `exportDir` change (regardless of whether the panel is expanded — open tabs need events too) and incrementally refreshes the tree; in-app mutations mark dirs in a `selfChangedDirs` Set (500 ms) to suppress double-refresh. `QueryPage.tsx` separately listens for `"fs:changed"` and re-reads any open file-backed tab in the changed directory via `queryStore.refreshFileTab` (clean tabs adopt the new disk content; tabs with unsaved edits are left untouched, VSCode-style) or `orphanFileTab` if the file is gone. Gated behind `fileWatcher`.

## Snowflake CLI profile management

`internal/sfconfig/writer.go` (`SaveProfile`, `DeleteProfile`, `CloneProfile`, `RenameProfile`, `SetDefaultProfile`) manipulates `~/.snowflake/config.toml` at the text level, preserving comments, blank lines, and unknown keys. `atomicWriteFile` writes a temp file then renames at `0600`. The `ConnectModal.tsx` profile UI is gated behind `snowflakeCLIProfileManager`.

## MCP server

`internal/mcp/` hosts read-only MCP servers (official Go MCP SDK) exposing the active connection to external AI clients over SSE/HTTP on `localhost`. `Manager` owns multiple labeled `session`s (ports auto-assign from 9100); each session runs its own dedicated `*snowflake.Client`. Sessions are user-started only and stopped via `StopAll()` in both `App.shutdown()` and `App.Disconnect()`. **`internal/mcp` must not import `internal/app`** (cycle). Gated behind `mcpServer`. Two middlewares wrap the SSE handler (`security.go`): `loopbackGuard` (DNS-rebinding defense) and `tokenGuard` (per-session crypto-random token required on the session-creating `GET`, via `Authorization: Bearer` or `?token=`; message `POST`s are authorized by the SDK's `sessionid`). The token is surfaced only via `Manager.AuthenticatedURL` (used by `GetMCPSessionConfig`), never in `SessionInfo`.

## EXPLAIN precompilation gate

The MCP server's `execute_snowflake_sql` tool uses a three-layer gate (`internal/mcp/gate.go`) to validate every SQL statement before execution:

1. **Single-statement check** — `SplitStatements(sql)` must return exactly 1 statement. Multi-statement SQL is rejected before any Snowflake round-trip.
2. **USE statement check** — `isUSEStatement(sql)` rejects `USE ROLE/WAREHOUSE/DATABASE/SCHEMA` and `USE SECONDARY ROLES`. Context-switching is exposed only through dedicated trusted tools (`use_role`, `use_warehouse`, etc.) that can be individually omitted via session pinning.
3. **EXPLAIN plan validation** — `EXPLAIN USING TABULAR <stmt>` is sent to Snowflake. Every operation in the returned plan must be in the `readOnlyOps` allow-list (default-deny). Any unknown operation (including future Snowflake additions) is rejected.

The gate returns a `GateVerdict` struct with `Allowed`, `Operations`, `Rejected`, and `Reason` fields, providing structured feedback to the AI client on why a statement was rejected.

**Key design decisions**: The gate accepts a `queryRunner` interface (not `*snowflake.Client` directly) so unit tests can use a fake implementation with canned results. The `readOnlyOps` map is intentionally conservative — it is better to over-reject than to let a mutation through. The gate is defense-in-depth; the real security boundary is the Snowflake role's grants.

## Mode-gated tool registration

The MCP server uses mode-gated registration (`internal/mcp/server.go`) to control which tools are exposed based on the session's execution mode:

```go
func buildServer(client, mode, cfg) *mcpsdk.Server {
    registerTools(srv, client)           // always: schema browsing
    registerDiagTools(srv, client)       // always: diagnostics
    if mode == "readonly" || mode == "explain_only" {
        registerSQLTools(srv, client, mode, cfg)  // gated: SQL execution
    }
}
```

Within `registerSQLTools`, individual context-switching tools are further gated by `SessionConfig` pinning — `use_role` is omitted when `PinnedRole` is true, `use_warehouse` when `PinnedWarehouse` is true. This ensures the AI client cannot switch to a different role or warehouse when the session is pinned.

## Code snippets cascading menu

Implemented via Monaco's internal `MenuRegistry` + `CommandsRegistry` (a one-time module IIFE), not per-editor patching. Snippets respect `editorPrefsRef` at insertion time (`applyPrefsToSnippet`). Definitions live in `snowflakeSnippets.ts`; `SNIPPET_CATEGORIES` drives the submenu. **Do not use `instanceof SubmenuAction` from an external import** — use `MenuRegistry` and let Monaco build the action internally.
