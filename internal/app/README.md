# internal/app

> The Wails-bound `App` struct: composition root, lifecycle, and all frontend
> IPC methods for the Thaw application.

## Responsibility

`package app` is the single entry point that the Wails runtime binds to the
frontend. It owns:

- The shared `*snowflake.Client` (the "main" connection used by non-query IPC).
- A `sync.Map` of per-tab `*tabSession` values, each with its own isolated
  Snowflake client and two-phase query state.
- All exported IPC methods callable from `frontend/wailsjs/go/app/App.ts`.

Real business logic (SQL building, result parsing, validation) lives in
`internal/<domain>` packages. Most `*App` methods are **thin delegators**:
nil-check → delegate → return.

## Key files

| File | Contents |
|------|----------|
| `app.go` | `App` struct definition, `NewApp`, `startup`/`shutdown` lifecycle, `Connect`/`CancelConnect`/`Disconnect`/`IsConnected`, tab-session management (`getOrInitTabSession`, `InitTabSession`, `CloseTabSession`, `evictIfNeeded`, `evictIdleSessions`, `runIdleEvictionLoop`, `applySessionConfig`), `GetAppInfo`. |
| `run.go` | `Run(assets embed.FS)` — the sole exported entry point called by `main.go`. Initialises crash reporting, restores window state, calls `buildMenu`, calls `wails.Run`. Also registers `sqleditor.NewService()` in the `Bind` array. |
| `menu.go` | `buildMenu(*App)` — constructs the native macOS/Windows menu bar. All menu actions emit `menu:*` Wails events; no direct state mutation. |
| `doc.go` | Package doc comment and `// thaw:domain: Core IPC & App Lifecycle` annotation. |
| `query.go` | `ExecuteQuery`, `StartQuery`, `WaitForQueryResult`, `CancelQuery`, `RunExplain`, `GetQueryHistory`, `GetQueryOperatorStats`. These methods contain non-delegator orchestration (goroutines, Wails event emission, `sync.WaitGroup`). |
| `session.go` | `GetSessionContext`, `GetTabSessionID`, `GetQuotedIdentifiersIgnoreCase`, `UseRole`/`UseWarehouse`/`UseDatabase`/`UseSchema`, session-parameter getters/setters. |
| `objects.go` | `ListDatabases`, `ListSchemas`, `ListObjects`, `ListBasicObjects`, `ClearObjectCache`, `ClearObjectCacheForDatabase`, `DropDatabase`, `DropSchema`, `GetObjectDDL`, and related object-management methods. |
| `warehouse.go` | Warehouse IPC: `GetWarehouseDDL`, `AlterWarehouseProperty`/`Suspend`/`Resume`/`AbortAllQueries`/`Rename`, `GetWarehouseParameters`, `GetWarehouseMeteringHistory`. Delegates to `internal/warehouse`. |
| `table.go` | Table settings queries and column DDL builders; delegates to `internal/table` and `internal/column`. |
| `backup.go` | Backup set/policy CRUD; delegates to `internal/backup`. |
| `builders.go` | Miscellaneous SQL-builder IPC methods (key-pair generation via `internal/keypair`, query-history builder via `internal/queryhistory`, etc.). |
| `stage.go` | Stage listing, file management, and `ExecuteStageFile`; delegates to `internal/snowflake`. |
| `dbtproject.go` | DBT PROJECT create/alter/execute builders; delegates to `internal/dbtproject`. |
| `pipe.go` | Pipe SQL builders and COPY_HISTORY; delegates to `internal/pipe`. |
| `dynamictable.go` | `AlterDynamicTable` (free-form `ALTER DYNAMIC TABLE … <clause>` for SUSPEND/RESUME/REFRESH/SET/UNSET); CREATE builder in `builders.go` delegates to `internal/dynamictable`. |
| `externaltable.go` | `AlterExternalTable` (free-form `ALTER EXTERNAL TABLE … <clause>` for REFRESH / SET AUTO_REFRESH; the grammar has no SET/UNSET COMMENT or RENAME — comments go through `COMMENT ON TABLE`); CREATE builder in `builders.go` delegates to `internal/externaltable`. |
| `icebergtable.go` | `AlterIcebergTable` (free-form `ALTER ICEBERG TABLE … <clause>` for REFRESH / SET/UNSET COMMENT / RENAME TO); CREATE builder in `builders.go` delegates to `internal/icebergtable`. |
| `hybridtable.go` | `AlterHybridTable` (free-form `ALTER TABLE … <clause>` for SET/UNSET COMMENT / RENAME TO — hybrid tables have no dedicated ALTER/DROP HYBRID TABLE statement), `ListHybridTableIndexes` (`SHOW INDEXES IN TABLE`), `CreateHybridTableIndex` / `DropHybridTableIndex` (`CREATE INDEX` / `DROP INDEX`), and `HybridIndexColumnOptions` (pure helper partitioning columns into index key- vs INCLUDE-eligible per Snowflake's datatype rules); CREATE builder in `builders.go` delegates to `internal/hybridtable`. |
| `eventtable.go` | `AlterEventTable` (free-form `ALTER TABLE … <clause>` for SET/UNSET COMMENT / CHANGE_TRACKING / DATA_RETENTION_TIME_IN_DAYS / MAX_DATA_EXTENSION_TIME_IN_DAYS, ADD/DROP SEARCH OPTIMIZATION, RENAME TO — event tables share the standard TABLE grammar and have no dedicated ALTER/DROP EVENT TABLE statement) and `GetEventTableParameters` (`SHOW PARAMETERS IN TABLE` — supplies the configurable parameter values that `SHOW EVENT TABLES` omits); CREATE builder in `builders.go` delegates to `internal/eventtable`. |
| `externalfunction.go` | `AlterExternalFunction` (free-form `ALTER FUNCTION <fqn>(<args>) <clause>` for SET/UNSET COMMENT / SET SECURE / SET API_INTEGRATION / etc. — external functions share the regular FUNCTION grammar and have no dedicated ALTER/DROP EXTERNAL FUNCTION statement) and `DescribeExternalFunction` (`DESCRIBE FUNCTION <fqn>(<args>)` — supplies the API integration, URL, headers, translators, and compression that `SHOW EXTERNAL FUNCTIONS` omits); CREATE builder in `builders.go` delegates to `internal/externalfunction`. Both require the argument signature to resolve the overload. Also `ListUserFunctions` (`SHOW USER FUNCTIONS IN DATABASE <db>`, filtered to scalar UDFs — table and external functions are excluded) which populates the request/response translator pickers, and `GetExternalFunctionOptions` (connection-free; returns the static compression / null-handling / volatility / context-header choice lists from `internal/externalfunction`) which populates the builder's dropdowns. |
| `datametricfunction.go` | `AlterDataMetricFunction` (free-form `ALTER FUNCTION <fqn>(<args>) <clause>` for SET/UNSET COMMENT / SET/UNSET SECURE / RENAME TO / SET/UNSET TAG — data metric functions share the regular FUNCTION grammar and have no dedicated ALTER/DROP DATA METRIC FUNCTION statement) and `DescribeDataMetricFunction` (`DESCRIBE FUNCTION <fqn>(<args>)` — supplies the body expression that `SHOW DATA METRIC FUNCTIONS` omits). Both require the TABLE argument signature (e.g. `TABLE(NUMBER)`) to resolve the overload. Also `GetDataMetricFunctionReferences` (the tables/views a DMF is scheduled against, from `SNOWFLAKE.ACCOUNT_USAGE.DATA_METRIC_FUNCTION_REFERENCES` — governance priv + latency, loaded on demand) and `GetDataMetricFunctionTags` (the tags applied to the DMF, via the no-latency `INFORMATION_SCHEMA.TAG_REFERENCES('<fqn>','FUNCTION')` table function, for the properties tag editor). CREATE builder in `builders.go` delegates to `internal/datametricfunction`. |
| `materializedview.go` | `AlterMaterializedView` (free-form `ALTER MATERIALIZED VIEW … <clause>` for SUSPEND/RESUME/RECLUSTER/CLUSTER BY/SET/UNSET; materialized views have no manual REFRESH); CREATE builder in `builders.go` delegates to `internal/materializedview`. |
| `alert.go` | `AlterAlert` (free-form `ALTER ALERT … <clause>` for RESUME/SUSPEND/SET/UNSET/MODIFY CONDITION/MODIFY ACTION; alerts have no RENAME) and `ExecuteAlert` (the standalone `EXECUTE ALERT <fqn>` statement, not an ALTER clause); CREATE builder in `builders.go` delegates to `internal/alert`. |
| `tag.go` | `AlterTag` (free-form `ALTER TAG … <clause>` for RENAME/SET/UNSET COMMENT/ADD/DROP/UNSET ALLOWED_VALUES/SET/UNSET MASKING POLICY) and `GetTagReferences` (lists where the tag is applied via `SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES`); CREATE builder in `builders.go` delegates to `internal/tag`. |
| `maskingpolicy.go` | `AlterMaskingPolicy` (free-form `ALTER MASKING POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG) and `GetMaskingPolicyReferences` (lists the columns the policy is applied to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'MASKING_POLICY'`); CREATE builder in `builders.go` delegates to `internal/maskingpolicy`. |
| `rowaccesspolicy.go` | `AlterRowAccessPolicy` (free-form `ALTER ROW ACCESS POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG) and `GetRowAccessPolicyReferences` (lists the tables/views the policy is applied to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'ROW_ACCESS_POLICY'`); CREATE builder in `builders.go` delegates to `internal/rowaccesspolicy`. |
| `passwordpolicy.go` | `AlterPasswordPolicy` (free-form `ALTER PASSWORD POLICY … <clause>` for RENAME/SET/UNSET each `PASSWORD_*` parameter/SET/UNSET COMMENT/SET/UNSET TAG), `DescribePasswordPolicy` (`DESCRIBE PASSWORD POLICY` — one `property/value/default` row per parameter, which `SHOW PASSWORD POLICIES` omits) and `GetPasswordPolicyReferences` (lists the users/account the policy is attached to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'PASSWORD_POLICY'`); CREATE builder in `builders.go` delegates to `internal/passwordpolicy`. |
| `networkrule.go` | `AlterNetworkRule` (free-form `ALTER NETWORK RULE … <clause>` for SET/UNSET VALUE_LIST and SET/UNSET COMMENT — TYPE/MODE are immutable and there is no RENAME); CREATE builder in `builders.go` delegates to `internal/networkrule`. |
| `imagerepository.go` | `AlterImageRepository` (free-form `ALTER IMAGE REPOSITORY … <clause>` for SET/UNSET COMMENT — the only mutable property; no RENAME) and `ListImagesInRepository` (`SHOW IMAGES IN IMAGE REPOSITORY`); CREATE builder in `builders.go` delegates to `internal/imagerepository`. |
| `service.go` | `AlterService` (free-form `ALTER SERVICE … <clause>` for SUSPEND/RESUME and SET/UNSET of MIN_INSTANCES/MAX_INSTANCES/AUTO_RESUME/QUERY_WAREHOUSE/COMMENT; no RENAME), `ListServiceEndpoints` (`SHOW ENDPOINTS IN SERVICE`), `GetServiceContainers` (`SHOW SERVICE CONTAINERS IN SERVICE`), and `GetServiceLogs` (`SYSTEM$GET_SERVICE_LOGS`); CREATE builder in `builders.go` delegates to `internal/service`. The compute-pool picker is backed by `ListComputePools` (`SHOW COMPUTE POOLS`) in `session.go`. |
| `streamlit.go` | `AlterStreamlit` (free-form `ALTER STREAMLIT … <clause>` for RENAME TO and SET/UNSET of MAIN_FILE/QUERY_WAREHOUSE/TITLE/COMMENT/EXTERNAL_ACCESS_INTEGRATIONS); CREATE builder in `builders.go` delegates to `internal/streamlit`. |
| `git.go` | Git repository browsing, filtering, and config persistence; delegates to `internal/gitrepo`. |
| `filesystem.go` | File read/write/rename/delete, `StartFileWatcher`/`StopFileWatcher`, reveal in Finder; delegates to `internal/filesystem`. |
| `profiles.go` | Snowflake CLI profile CRUD (save, delete, clone, rename, set default); delegates to `internal/sfconfig`. |
| `ddlexport.go` | `ExportDatabaseDDL`, `ExportAllDatabasesDDL`, `ExportAccountObjectsDDL`, `GetERDiagramData`. Contain goroutine orchestration and `ddl:progress` event emission — not thin delegators. |
| `querylog.go` | `GetQueryLogEntries`, `ClearQueryLog`, `IsQueryLogEnabled`, `SetQueryLogEnabled`, `PickQueryLogExportFile`. Thin delegators to `a.queryLog` (`internal/querylog`). |
| `config.go` | `GetFeatureFlags`/`SaveFeatureFlags`/`GetAdminLockedFlags`, `GetEditorPrefs`/`SaveEditorPrefs`, `GetGitConfig`/`SaveGitConfig`, `GetSessionConfig`/`SaveSessionConfig`/`GetSessionInitMode`. |
| `ai.go` | `ListAIModels`, `TestAIModel`, `GetAISuggestion`, `GetAIEdit`, `GetAIExplain`, `GetEditorPrefs` back-fill; delegates to `internal/ai`. |
| `shell.go` | Embedded terminal (PTY): `GetAvailableShells`, `StartShell`, `StopShell`, `WriteShell`, `ResizeShell`. Contains PTY goroutine; emits `shell:data` events. |
| `users.go` | User/role management IPC: `ListUsers`, `ListRoles`, `CreateUser`, `DropUser`, `AlterUserProperty`, `GetUserRSAKeyPair`; delegates to `internal/keypair`. |
| `tasks.go` | Task graph queries and run-history IPC; delegates to `internal/tasks`. |
| `integrations.go` | Security/API integration and secrets listing; delegates to `internal/snowflake` and `internal/integrations`. |
| `erdesigner.go` | ER designer state sync IPC: `UpdateERDesignerState`, `ClearERDesignerState`; pushes designer table state into `mcp.Manager.ERDesignerState()` for MCP tool access. |
| `notebook_native.go` | Snowflake Notebooks CRUD IPC; delegates to `internal/snowflake`. |
| `migration.go` | Schema-migration IPC (`ScanMigrationSource`, `AnalyzeMigration`, `CreateMigrationSnapshot`, `ExecuteMigration`); delegates to `a.migrationSvc` (`internal/migration`). |
| `snowpark.go` | Snowpark/Jupyter environment check, setup, kernel lifecycle; delegates to `a.snowparkSvc` (`internal/snowpark`). |

## Key types & functions

### `App` struct (`app.go`)

The single Wails-bound struct. Fields of note:

| Field | Purpose |
|-------|---------|
| `client *snowflake.Client` | Shared client for non-query IPC calls. |
| `connectParams *snowflake.ConnectParams` | Stored after `Connect` for tab-session creation. |
| `tabSessions sync.Map` | `tabId → *tabSession` per-tab isolated connections. |
| `evictedContexts sync.Map` | Caches role/wh/db/schema for LRU-evicted tabs so they can be restored transparently. |
| `migrationSvc *migration.Service` | Delegatee for schema-migration methods. |
| `snowparkSvc *snowpark.Service` | Delegatee for Snowpark/Jupyter methods. |
| `ptmx *os.File`, `ptyCmd *exec.Cmd` | Embedded terminal state. |
| `fsWatcher *filesystem.Watcher` | Active FS watcher, if any. |

### `tabSession` struct (`app.go`)

Per-tab state:

| Field | Purpose |
|-------|---------|
| `client *snowflake.Client` | Isolated connection for this tab. |
| `lastUsed atomic.Int64` | UnixNano timestamp for LRU eviction. |
| `inUse atomic.Int32` | Prevents eviction during in-flight non-query RPCs. |
| `queryID`, `queryDone`, `queryResult`, `queryErr` | Two-phase query execution state set by `StartQuery`. |

### Top-level functions

| Function | File | Notes |
|----------|------|-------|
| `Run(assets embed.FS) error` | `run.go` | Called from `main.go`. Wails entry point. |
| `NewApp() *App` | `app.go` | Constructs an empty `App`; called inside `Run`. |
| `buildMenu(app *App) *menu.Menu` | `menu.go` | Constructs the native menu; called inside `Run`. |

## Patterns & integration

### Thin-delegator pattern

The canonical IPC method shape (from `warehouse.go`):

```go
func (a *App) GetWarehouseMeteringHistory(wh, startDate, endDate string) ([]warehouse.WarehouseMeteringRow, error) {
    if a.client == nil {
        return nil, apperrors.ErrNotConnected
    }
    return warehouse.GetMeteringHistory(a.ctx, a.client, wh, startDate, endDate)
}
```

The nil-check uses `apperrors.ErrNotConnected` (from `internal/apperrors`). All
real logic — SQL building, `snowflake.QueryResult` parsing — lives in the
domain package.

### Exceptions (non-delegator methods that stay in `internal/app`)

These methods contain goroutine orchestration, Wails event emission, or are
tightly coupled to `App`-internal state and therefore cannot move to a domain
package:

- `StartQuery` / `WaitForQueryResult` / `CancelQuery` — per-tab query channels,
  goroutines, `wailsruntime.EventsEmit`.
- `RunExplain` — uses a pinned connection from the tab session.
- `ExportDatabaseDDL` / `ExportAllDatabasesDDL` — goroutine + `ddl:progress` events.
- Shell PTY methods (`StartShell`, `WriteShell`, `ResizeShell`, `StopShell`).
- `GetSessionContext` — fast-path evicted-context cache lookup.
- `GetEditorPrefs` / `GetSessionConfig` — back-fill defaults from `App`-held
  config; not pure delegation.

### IPC flow

```
Frontend wailsjs/go/app/App.ts
   ↓  Wails runtime
*App method in internal/app/<domain>.go
   ↓  nil-check
domain-package func(ctx, *snowflake.Client, …)
   ↓
internal/snowflake.Client + result types in the domain package
```

`sqleditor.Service` is bound alongside `App` (see `run.go`); its methods are
imported from `wailsjs/go/sqleditor/Service`.

### Wails events emitted from `internal/app`

| Event | Emitted in | Payload |
|-------|-----------|---------|
| `menu:*` | `menu.go` | varies by item |
| `ddl:progress` | `ddlexport.go` | `DDLProgressPayload{Done, Total, Result}` |
| `shell:data` | `shell.go` | base64-encoded PTY output |
| `migration:*` | via `migrationSvc` callback in `app.go` startup | varies |

## Gotchas

- **All files are `package app`.** Wails binds the whole method set regardless
  of which `.go` file a method lives in. New domain files just need to be placed
  in `internal/app/` with `package app`.
- **`wails generate module`** must be run after changing any public method
  signature or adding/removing methods, to keep `frontend/wailsjs/` in sync.
- **Never edit `frontend/wailsjs/` by hand** — it is fully overwritten by the
  generator.
- **`app.go` only**: the `App` struct definition, lifecycle (`startup`/`shutdown`),
  `Connect`/`Disconnect`, and tab-session machinery must all stay in `app.go`.
  Domain-specific IPC goes in the matching `<domain>.go` file.
- **`a.client` vs tab sessions**: `a.client` is the shared connection used for
  IPC calls that are not tab-scoped (DDL, object listing, etc.). Tab-scoped
  query execution always goes through `getOrInitTabSession(tabId)`.
- **MCP sessions** — `internal/app/mcp.go` (described in CLAUDE.md) is present
  on the `feat/mcp-server-foundation` branch but may not yet exist on `main`.
  Check `Glob` before assuming its presence.
