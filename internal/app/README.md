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
| `session.go` | `GetSessionContext`, `GetTabSessionID`, `GetQuotedIdentifiersIgnoreCase`, `UseRole`/`UseWarehouse`/`UseDatabase`/`UseSchema`, session-parameter getters/setters, `GetClientVersionInfo` (general `SELECT SYSTEM$CLIENT_VERSION_INFO()` → supported/recommended client & driver versions, reusable by any feature). |
| `objects.go` | `ListDatabases`, `ListSchemas`, `ListObjects`, `ListBasicObjects`, `ClearObjectCache`, `ClearObjectCacheForDatabase`, `DropDatabase`, `DropSchema`, `GetObjectDDL`, and related object-management methods. |
| `warehouse.go` | Warehouse IPC: `GetWarehouseDDL`, `AlterWarehouseProperty`/`Suspend`/`Resume`/`AbortAllQueries`/`Rename`, `GetWarehouseParameters`, `GetWarehouseMeteringHistory`. Delegates to `internal/warehouse`. |
| `table.go` | Table settings queries and column DDL builders; delegates to `internal/table` and `internal/column`. |
| `backup.go` | Backup set/policy CRUD; delegates to `internal/backup`. |
| `builders.go` | Miscellaneous SQL-builder IPC methods (key-pair generation via `internal/keypair`, query-history builder via `internal/queryhistory`, etc.). |
| `sqlformat.go` | General connection-free SQL string-formatting delegators over `internal/snowflake`: `ParseSqlList` (parse a DESCRIBE list cell into value tokens), `NormalizeSqlScalar` (strip wrapping brackets/quotes from a DESCRIBE scalar), `QuoteSqlText` (single-quote a free-text literal), `ReconcileAllExclusiveList` (collapse a mixed `('ALL', X)` selection to the kind chosen last). Used by the policy property modals so the frontend keeps no SQL-quoting/parsing logic. |
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
| `view.go` | `AlterView` (free-form `ALTER VIEW … <clause>` for SET/UNSET SECURE, SET/UNSET COMMENT, RENAME TO); CREATE builder in `builders.go` delegates to `internal/view`. |
| `sequence.go` | `AlterSequence` (free-form `ALTER SEQUENCE … <clause>` for SET INCREMENT, SET/UNSET COMMENT, RENAME TO — note `START WITH` is not alterable); CREATE builder in `builders.go` delegates to `internal/sequence`. |
| `stream.go` | `AlterStream` (free-form `ALTER STREAM … <clause>` for SET/UNSET COMMENT, RENAME TO — the source and `APPEND_ONLY`/`INSERT_ONLY` modes are fixed at create time); CREATE builder in `builders.go` delegates to `internal/stream`. |
| `function.go` | `AlterFunction` (free-form `ALTER FUNCTION <fqn>(<args>) <clause>` for SET/UNSET SECURE, SET/UNSET COMMENT, RENAME TO); CREATE builder in `builders.go` delegates to `internal/udf` (the Go package is named `udf`, not `function`, because Wails derives a TS namespace from the package name and `function` is a TS reserved word). |
| `procedure.go` | `AlterProcedure` (free-form `ALTER PROCEDURE <fqn>(<args>) <clause>` for SET/UNSET SECURE, SET/UNSET COMMENT, EXECUTE AS, RENAME TO); CREATE builder in `builders.go` delegates to `internal/procedure` (which also holds `BuildCallStatement` / `BuildFunctionSelectStatement`). |
| `alert.go` | `AlterAlert` (free-form `ALTER ALERT … <clause>` for RESUME/SUSPEND/SET/UNSET/MODIFY CONDITION/MODIFY ACTION; alerts have no RENAME) and `ExecuteAlert` (the standalone `EXECUTE ALERT <fqn>` statement, not an ALTER clause); CREATE builder in `builders.go` delegates to `internal/alert`. |
| `tag.go` | `AlterTag` (free-form `ALTER TAG … <clause>` for RENAME/SET/UNSET COMMENT/ADD/DROP/UNSET ALLOWED_VALUES/SET/UNSET MASKING POLICY) and `GetTagReferences` (lists where the tag is applied via `SNOWFLAKE.ACCOUNT_USAGE.TAG_REFERENCES`); CREATE builder in `builders.go` delegates to `internal/tag`. |
| `maskingpolicy.go` | `AlterMaskingPolicy` (free-form `ALTER MASKING POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG) and `GetMaskingPolicyReferences` (lists the columns the policy is applied to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'MASKING_POLICY'`); CREATE builder in `builders.go` delegates to `internal/maskingpolicy`. |
| `rowaccesspolicy.go` | `AlterRowAccessPolicy` (free-form `ALTER ROW ACCESS POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG) and `GetRowAccessPolicyReferences` (lists the tables/views the policy is applied to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'ROW_ACCESS_POLICY'`); CREATE builder in `builders.go` delegates to `internal/rowaccesspolicy`. |
| `joinpolicy.go` | `AlterJoinPolicy` (free-form `ALTER JOIN POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG), `GetJoinPolicyTags` (current tags via `INFORMATION_SCHEMA.TAG_REFERENCES`, object domain `JOIN POLICY`), and `GetJoinPolicyReferences` (lists the tables/views the policy is applied to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'JOIN_POLICY'`); CREATE builder in `builders.go` delegates to `internal/joinpolicy`. |
| `privacypolicy.go` | `AlterPrivacyPolicy` (free-form `ALTER PRIVACY POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG), `GetPrivacyPolicyTags` (current tags via `INFORMATION_SCHEMA.TAG_REFERENCES`, object domain `PRIVACY POLICY`), and `GetPrivacyPolicyReferences` (lists the tables/views the policy is applied to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'PRIVACY_POLICY'`); CREATE builder in `builders.go` delegates to `internal/privacypolicy`. |
| `storagelifecyclepolicy.go` | `AlterStorageLifecyclePolicy` (free-form `ALTER STORAGE LIFECYCLE POLICY … <clause>` for RENAME/SET BODY/SET ARCHIVE_TIER/SET/UNSET ARCHIVE_FOR_DAYS/SET/UNSET COMMENT/SET/UNSET TAG), `GetStorageLifecyclePolicyTags` (current tags via `INFORMATION_SCHEMA.TAG_REFERENCES`, object domain `STORAGE LIFECYCLE POLICY`), and `GetStorageLifecyclePolicyReferences` (lists the tables the policy is applied to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'STORAGE_LIFECYCLE_POLICY'`); CREATE builder in `builders.go` delegates to `internal/storagelifecyclepolicy`. |
| `passwordpolicy.go` | `AlterPasswordPolicy` (free-form `ALTER PASSWORD POLICY … <clause>` for RENAME/SET/UNSET each `PASSWORD_*` parameter/SET/UNSET COMMENT/SET/UNSET TAG), `DescribePasswordPolicy` (`DESCRIBE PASSWORD POLICY` — one `property/value/default` row per parameter, which `SHOW PASSWORD POLICIES` omits) and `GetPasswordPolicyReferences` (lists the users/account the policy is attached to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'PASSWORD_POLICY'`); CREATE builder in `builders.go` delegates to `internal/passwordpolicy`. |
| `sessionpolicy.go` | `AlterSessionPolicy` (free-form `ALTER SESSION POLICY … <clause>` for RENAME/SET/UNSET each `SESSION_*` timeout/`ALLOWED_SECONDARY_ROLES`/`BLOCKED_SECONDARY_ROLES`/SET/UNSET COMMENT/SET/UNSET TAG), `DescribeSessionPolicy` (`DESCRIBE SESSION POLICY` — a single row whose columns carry the timeout values and `allowed_secondary_roles`, which `SHOW SESSION POLICIES` omits), `ParseSecondaryRoles` / `FormatSecondaryRoles` / `ReconcileSecondaryRoles` (pure helpers delegating to the matching `snowflake.*` functions, so the create / properties modals parse a DESCRIBE secondary-role cell, serialize a list into the `( 'ALL' | <role>, … )` SQL grammar, and enforce the `ALL`-vs-role-list exclusivity via one shared Go implementation rather than re-deriving any of it in TypeScript) and `GetSessionPolicyReferences` (lists the users/account the policy is attached to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'SESSION_POLICY'`); CREATE builder in `builders.go` delegates to `internal/sessionpolicy`. |
| `aggregationpolicy.go` | `AlterAggregationPolicy` (free-form `ALTER AGGREGATION POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG) and `GetAggregationPolicyReferences` (lists the tables/views the policy is attached to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'AGGREGATION_POLICY'`); the body is read back via the `DESCRIBE AGGREGATION POLICY` enrichment in `internal/objects`; CREATE builder in `builders.go` delegates to `internal/aggregationpolicy`. |
| `projectionpolicy.go` | `AlterProjectionPolicy` (free-form `ALTER PROJECTION POLICY … <clause>` for RENAME/SET BODY/SET/UNSET COMMENT/SET/UNSET TAG) and `GetProjectionPolicyReferences` (lists the columns the policy is attached to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'PROJECTION_POLICY'`); the body is read back via the `DESCRIBE PROJECTION POLICY` enrichment in `internal/objects`; CREATE builder in `builders.go` delegates to `internal/projectionpolicy`. |
| `authenticationpolicy.go` | `AlterAuthenticationPolicy` (free-form `ALTER AUTHENTICATION POLICY … <clause>` for RENAME/SET/UNSET each list parameter (`AUTHENTICATION_METHODS`/`CLIENT_TYPES`/`SECURITY_INTEGRATIONS`)/`MFA_ENROLLMENT`/SET/UNSET COMMENT), `DescribeAuthenticationPolicy` (`DESCRIBE AUTHENTICATION POLICY` — one row per property with `property`/`value` columns, which `SHOW AUTHENTICATION POLICIES` omits; projected to `[]snowflake.PropertyPair` via `snowflake.ResultPropertyValueRows` so the column indexing stays in Go, not the modal), `FormatAuthPolicyList` (pure helper delegating to `authenticationpolicy.FormatStringList`, so the create / properties modals serialize a token list into the `('A', 'B')` SQL grammar via one shared Go implementation), `AuthenticationPolicyListParams` / `AuthenticationPolicyMFAEnrollmentOptions` / `AuthenticationPolicyBagOptions` / `AuthenticationPolicyClientDrivers` (pure metadata — the list-parameter editor descriptors (keyword/label/allowed values/free-form flag), the `MFA_ENROLLMENT` options, the nested-bag enum sets (MFA methods/enforce, PAT network-policy-evaluation/require-role, workload providers), and the `CLIENT_POLICY` driver picker (the version-governed subset of the shared `snowflake.ClientDrivers` catalog), so the modal renders the editors from the Go grammar rather than hardcoding allowed values in TypeScript), `AuthenticationPolicyClientDriverVersions` (runs `SYSTEM$CLIENT_VERSION_INFO()` and returns each CLIENT_POLICY driver's minimum-supported / recommended versions so the editor suggests them; the general `GetClientVersionInfo` lives in `session.go`), the nested-property-bag converters `Build{MFA,PAT,WorkloadIdentity,Client}PolicyValue` / `Parse{MFAPolicy,PATPolicy,WorkloadIdentityPolicy,ClientPolicy}` (pure delegators so the modal serializes / pre-fills `MFA_POLICY`/`PAT_POLICY`/`WORKLOAD_IDENTITY_POLICY`/`CLIENT_POLICY` entirely in Go — the modal feeds a `Build…Value` result into `SET <BAG> = …`; `UNSET DCM PROJECT` is a plain clause) and `GetAuthenticationPolicyReferences` (lists the users/account the policy is attached to via `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, `POLICY_KIND = 'AUTHENTICATION_POLICY'`); the list/scalar DESCRIBE values are parsed and the comment quoted via the general `App.{ParseSqlList,NormalizeSqlScalar,QuoteSqlText}` helpers in `sqlformat.go`; CREATE builder in `builders.go` delegates to `internal/authenticationpolicy`. |
| `packagespolicy.go` | `AlterPackagesPolicy` (free-form `ALTER PACKAGES POLICY … <clause>` for SET/UNSET each list parameter (`ALLOWLIST`/`BLOCKLIST`/`ADDITIONAL_CREATION_BLOCKLIST`)/SET/UNSET COMMENT — packages policies have **no RENAME and no TAG**), `FormatPackagesPolicyList` (pure helper delegating to `packagespolicy.FormatStringList`, so the create / properties modals serialize a token list into the `('A', 'B')` SQL grammar via one shared Go implementation) and `ParsePackagesPolicyList` (pure helper delegating through `packagespolicy.ParseList` to the general `snowflake.ParseSqlListVerbatim` — the tokenizer-driven list parser that reconstructs each element's verbatim text, rather than the general `App.ParseSqlList` whose tokenizer drops operator tokens, so a package **version specifier** like `numpy==1.26.4` survives whether or not Snowflake quotes the entries; the generic parser would split it on the `==`); the list values are read back via the `DESCRIBE PACKAGES POLICY` enrichment in `internal/objects` (`SHOW PACKAGES POLICIES` reports only metadata), the comment quoted via `App.QuoteSqlText` in `sqlformat.go`; CREATE builder in `builders.go` delegates to `internal/packagespolicy`. |
| `networkrule.go` | `AlterNetworkRule` (free-form `ALTER NETWORK RULE … <clause>` for SET/UNSET VALUE_LIST and SET/UNSET COMMENT — TYPE/MODE are immutable and there is no RENAME); CREATE builder in `builders.go` delegates to `internal/networkrule`. |
| `imagerepository.go` | `AlterImageRepository` (free-form `ALTER IMAGE REPOSITORY … <clause>` for SET/UNSET COMMENT — the only mutable property; no RENAME) and `ListImagesInRepository` (`SHOW IMAGES IN IMAGE REPOSITORY`); CREATE builder in `builders.go` delegates to `internal/imagerepository`. |
| `model.go` | `AlterModel` (free-form `ALTER MODEL … <clause>` covering the full grammar: SET COMMENT/DEFAULT_VERSION, UNSET COMMENT, SET/UNSET TAG, RENAME TO, per-version `VERSION … SET/UNSET ALIAS`, `MODIFY VERSION … SET COMMENT/METADATA`, `ADD VERSION … FROM MODEL/stage`, `DROP VERSION`), `ListModelVersions` (`SHOW VERSIONS IN MODEL`), `ListModels` (`SHOW MODELS IN ACCOUNT` → quoted FQNs, backs the source-model picker), and `GetModelTags` (`INFORMATION_SCHEMA.TAG_REFERENCES` object domain MODEL — the immediate-consistency tag editor source); CREATE builder in `builders.go` delegates to `internal/model`. GET_DDL is not supported for models. |
| `modelmonitor.go` | `AlterModelMonitor` (free-form `ALTER MODEL MONITOR … <clause>` covering the full mutable surface: `SUSPEND` / `RESUME`, `SET BASELINE` / `REFRESH_INTERVAL` / `WAREHOUSE`, `ADD` / `DROP segment_column` — no RENAME / COMMENT / TAG); CREATE builder in `builders.go` delegates to `internal/modelmonitor`. GET_DDL is not supported for model monitors. |
| `service.go` | `AlterService` (free-form `ALTER SERVICE … <clause>` for SUSPEND/RESUME and SET/UNSET of MIN_INSTANCES/MAX_INSTANCES/AUTO_RESUME/QUERY_WAREHOUSE/COMMENT; no RENAME), `ListServiceEndpoints` (`SHOW ENDPOINTS IN SERVICE`), `GetServiceContainers` (`SHOW SERVICE CONTAINERS IN SERVICE`), and `GetServiceLogs` (`SYSTEM$GET_SERVICE_LOGS`); CREATE builder in `builders.go` delegates to `internal/service`. The compute-pool picker is backed by `ListComputePools` (`SHOW COMPUTE POOLS`) in `session.go`. |
| `cortexsearchservice.go` | `AlterCortexSearchService` (free-form `ALTER CORTEX SEARCH SERVICE … <clause>` covering the full grammar: SUSPEND/RESUME `[INDEXING|SERVING]`, REFRESH, SET/UNSET of TARGET_LAG/WAREHOUSE/ATTRIBUTES/PRIMARY KEY/AUTO_SUSPEND/FULL_INDEX_BUILD_INTERVAL_DAYS/REQUEST_LOGGING/COMMENT, SET/UNSET TAG, ADD/DROP SCORING PROFILE; no RENAME), `FormatCortexSearchAttributes` (joins a column list for the SET ATTRIBUTES / SET PRIMARY KEY clauses), and `GetCortexSearchServiceTags` (`INFORMATION_SCHEMA.TAG_REFERENCES` object domain CORTEX SEARCH SERVICE — the immediate-consistency tag editor source); CREATE builder in `builders.go` delegates to `internal/cortexsearchservice`. GET_DDL is not supported for cortex search services. |
| `streamlit.go` | `AlterStreamlit` (free-form `ALTER STREAMLIT … <clause>` for RENAME TO and SET/UNSET of MAIN_FILE/QUERY_WAREHOUSE/TITLE/COMMENT/EXTERNAL_ACCESS_INTEGRATIONS); CREATE builder in `builders.go` delegates to `internal/streamlit`. |
| `agent.go` | `AlterAgent` (free-form `ALTER AGENT … <clause>` covering the full grammar: `SET COMMENT`, `SET PROFILE`, `MODIFY LIVE VERSION SET SPECIFICATION = $THAW$…$THAW$`; agents have no RENAME/UNSET/TAG) and `DescribeAgent` (`DESCRIBE AGENT` → the `agent_spec` column the live-spec editor reads, which `SHOW AGENTS` omits); CREATE builder in `builders.go` delegates to `internal/agent`. GET_DDL works via the `CORTEX_AGENT` object type. |
| `externalagent.go` | `AlterExternalAgent` (free-form `ALTER EXTERNAL AGENT … <clause>` covering the full grammar: `SET COMMENT`, `ADD VERSION <name>`; external agents have no RENAME/UNSET/TAG); CREATE builder in `builders.go` delegates to `internal/externalagent`. GET_DDL is not supported for external agents. |
| `mcpserver.go` | `DescribeMCPServer` (`DESCRIBE MCP SERVER` → the `server_spec` column the read-only properties viewer reads, which `SHOW MCP SERVERS` omits); CREATE builder in `builders.go` delegates to `internal/mcpserver`. Snowflake has **no `ALTER MCP SERVER`** (recreate with CREATE OR REPLACE), so there is no mutation method, and GET_DDL is not supported for MCP servers. |
| `semanticview.go` | `AlterSemanticView` (free-form `ALTER SEMANTIC VIEW` — rename / set-unset comment / set-unset tag; the definition body is changed via CREATE OR REPLACE, not ALTER), `DescribeSemanticView` (`DESCRIBE SEMANTIC VIEW` → one row per logical table / relationship / dimension / fact / metric property), `ListSemanticDimensions` / `ListSemanticFacts` / `ListSemanticMetrics` (the `SHOW SEMANTIC …` commands), `ListSemanticDimensionsForMetric` (`SHOW SEMANTIC DIMENSIONS … FOR METRIC`), and `GetSemanticViewTags` (immediate-consistency `INFORMATION_SCHEMA.TAG_REFERENCES`). CREATE builder in `builders.go` delegates to `internal/semanticview`. GET_DDL **is** supported (object_type `'SEMANTIC VIEW'`). |
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
