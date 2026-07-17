# internal/mcp

> Model Context Protocol (MCP) servers that expose the active Snowflake connection to external AI clients over a local SSE/HTTP transport, with configurable execution modes and an EXPLAIN precompilation safety gate.

## Responsibility

Hosts one or more MCP servers, each bound to its own dedicated `*snowflake.Client`, on `localhost`. A `Manager` owns the set of running sessions; each session runs an `http.Server` serving the Go MCP SDK's SSE handler and registers schema-browsing, SQL diagnostics, editor context, and (optionally) SQL execution tools. Sessions are started and stopped only on explicit user action (Tools → MCP Sessions); none start automatically.

`internal/mcp` must **not** import `internal/app` — the dependency is one-way (`App` holds a `*mcp.Manager`). All Snowflake access goes through the `*snowflake.Client` handed to each session, mirroring the isolated per-tab session pattern.

## Key files

| File | Purpose |
|---|---|
| `manager.go` | `Manager` (multi-session registry), `SessionInfo`/`SessionConfig` types, execution mode constants, port allocation, `Start`/`Stop`/`UpdateMode`/`List`/`StopAll`, `EditorContext()`/`ERDesignerState()` accessors |
| `session.go` | Per-session `http.Server` + SSE lifecycle (`start`/`stop`/`updateMode`/`info`); serves on the held loopback listener and owns/closes its `*snowflake.Client`. `updateMode` mutates the existing server's tool registry via `RemoveTools`/`AddTool` — the SDK sends `tools/list_changed` notifications so connected clients update seamlessly. If the serve goroutine exits unexpectedly it closes the client and self-removes from the `Manager` (`removeIfPresent`) so no dead row or leaked connection lingers |
| `security.go` | `loopbackGuard` middleware (rejects non-loopback `Host`/cross-origin `Origin` — DNS-rebinding defense), `tokenGuard` middleware (per-session token auth on the SSE GET, with rate-limited logging of rejected requests via `authFailureLogger`), and `newSessionToken` (crypto-random token) |
| `server.go` | `buildServer(client, mode, cfg, editorCtx, emit, fnStore, nb)` — constructs the MCP server and registers tools based on execution mode; `modeSpecificToolNames` lists tools that `updateMode` removes/re-registers on mode switch |
| `tools.go` | Tool input structs + `registerTools` (schema-browsing tools); `jsonResult`/`textResult` content helpers |
| `schema_tools.go` | `registerSchemaTools` — extended schema discovery tools (`get_schema_foreign_keys`, `get_database_ddl`, `get_er_model`, `search_objects`, `get_all_data_types`, `validate_data_type`, `list_dropped_tables`, `list_dropped_schemas`, `get_data_retention`); always registered in all modes |
| `account_tools.go` | `registerAccountTools` — account & infrastructure tools (`list_roles`, `list_available_roles`, `get_role_ddl`, `list_warehouses`, `get_warehouse_ddl`, `list_integrations`, `list_secrets`, `list_file_formats`); always registered in all modes |
| `diag_tools.go` | `registerDiagTools` — SQL diagnostics & validation tools (`validate_sql`, `suggest_join_conditions`, `format_sql`, `get_snowflake_keywords`); type-conversion helpers for sqleditor ↔ snowflake types |
| `profile_tools.go` | `registerProfileTools` — query profiling tools (`explain_query`, `get_explain_diagnostics`); wraps `queryprofile.RunExplain` and `queryprofile.GetExplainDiagnostics`; always registered in all modes |
| `lineage_tools.go` | `registerLineageTools` — object lineage and cross-dependency tools (`get_object_lineage`, `get_schema_cross_deps`, `get_database_cross_deps`); wraps `Client.GetObjectDependencies`, `Client.GetSchemaCrossDeps`, `Client.GetDatabaseCrossDeps`; always registered in all modes |
| `workspace_tools.go` | `registerWorkspaceTools(srv, workspaceRoot)` — local filesystem and git read-only tools (`git_status`, `git_list_branches`, `git_get_head_file`, `git_diff_lines`, `list_directory`, `read_file`, `search_files`); delegates to `gitrepo`, `filesystem`, and `sqleditor` packages; no Snowflake client needed; **only registered when `WorkspaceRoot` is set** in `SessionConfig`; all path inputs are validated against the workspace root using `filesystem.ValidateInsideOrEqual` (symlink-resolving defense-in-depth), except `git_get_head_file` which uses `filesystem.ValidatePathOrAncestorInsideOrEqual` so it can return HEAD content for files deleted from the working tree |
| `context.go` | `EditorContextStore` — concurrency-safe in-memory store for per-tab editor SQL and result summaries; `ResultSummary` and `QueryHistoryEntry` types |
| `er_designer_state.go` | `ERDesignerStateStore` — concurrency-safe in-memory cache for the open ER designer's state (database, tables, columns); `ERDesignerState`, `ERDesignerTableOut`, `ERDesignerColumnOut` types. Frontend pushes state via IPC; `get_er_designer_state` reads from it |
| `editor_tools.go` | `registerEditorTools` — editor context tools (`get_current_editor_sql`, `get_query_results_summary`, `get_query_history`); bridges frontend editor state to MCP clients |
| `tab_tools.go` | `registerTabTools` — tab-delivery tool (`open_sql_tab`); formats SQL with user prefs, runs diagnostics, emits `mcp:open-sql-tab` Wails event. Registered when `emit` is non-nil. `OpenSqlTabPayload` type, `loadEditorPrefs` helper |
| `notebook_tools.go` | `registerNotebookTools` — notebook/Snowpark tools (`read_notebook`, `get_notebook_completions`, `check_python_syntax`, `open_notebook_tab`); `NotebookBackend` interface (dependency-injected from `App` via adapter), MCP-local type duplicates (`NotebookCompletion`, `NotebookSyntaxError`), `OpenNotebookTabPayload` type, `buildNbformat` helper. `read_notebook` is workspace-gated; kernel tools are backend-gated; `open_notebook_tab` is emit-gated |
| `er_tools.go` | `registerERDesignerTools` — ER designer delivery tool (`open_er_designer`); fetches live ER data, merges AI-generated tables via `mergeAITables`, emits `mcp:open-er-designer` Wails event. Emit-gated. `registerERDesignerStateTools` — ER designer state tools (`get_er_designer_state`, `modify_er_designer`); registered in `session.start()` after `buildServer()` to avoid changing the buildServer signature. `modify_er_designer` computes a change-delta summary (`describeERDelta`) for the tool result and keeps the state cache consistent by applying the merge itself (`mergeAITablesIntoState`). `OpenERDesignerPayload`, `ModifyERDesignerPayload` types, input types `openERDesignerInput`, `modifyERDesignerInput`, `erDesignerTableIn`, `erDesignerColumnIn` |
| `pipeline_tools.go` | `registerPipelineTools` — task graph, stage, and pipe inspection tools (`list_tasks`, `get_task_run_history`, `get_task_dependencies`, `list_stage_files`, `preview_stage_file`, `get_pipe_status`, `get_pipe_copy_history`, `open_task_graph`); delegates to `tasks`, `stage`, `fileformat`, `pipe` packages. `preview_stage_file` is mode-gated (readonly/explain_only). `open_task_graph` is emit-gated. `OpenTaskGraphPayload` type |
| `function_tools.go` | `registerFunctionTools` — function/procedure metadata and invocation builder tools (`search_functions`, `get_function_tooltip`, `get_procedure_params`, `get_function_info`, `build_call_statement`, `build_function_select`); delegates to `fnmeta.Store`, `snowflake.Client`, and `procedure` packages; always registered in all modes |
| `builder_tools.go` | `registerBuilderTools` — pure DDL builder tools (`build_create_stage_sql`, `build_alter_stage_sql`, `build_create_file_format_sql`, `build_create_pipe_sql`, `build_refresh_pipe_sql`, `build_create_secret_sql`, `build_storage_integration_sql`, `build_api_integration_sql`, `build_catalog_integration_sql`, `build_external_access_integration_sql`, `build_notification_integration_sql`, `build_security_integration_sql`); delegates to `stage`, `fileformat`, `pipe`, `secret`, `integrations` packages; no Snowflake client needed; always registered in all modes |
| `migration_tools.go` | `registerMigrationTools` — migration diff/script engine and dbt project scaffolder tools (`scan_migration_source`, `analyze_migration`, `generate_migration_script`, `generate_dbt_project`); delegates to `migration` and `dbt` packages; `scan_migration_source` and `generate_dbt_project` are workspace-gated; `analyze_migration` and `generate_migration_script` are always registered |
| `gate.go` | EXPLAIN precompilation gate: `queryRunner` interface, `CheckGate` (3-layer validation), `checkExplainPlan`, `readOnlyOps` allow-list, `extractOperations`, `isUSEStatement` |
| `sql_tools.go` | `registerSQLTools` — SQL execution tool (`execute_snowflake_sql`) with EXPLAIN-gated pipeline (`executeSQLPipeline`), LIMIT injection (`injectLimit`), and trusted context-switching tools (`use_role`, `use_warehouse`, `use_database`, `use_schema`); only registered in `readonly`/`explain_only` modes |
| `gate_test.go` | Unit tests for the EXPLAIN gate and `checkExplainPlan` |
| `sql_tools_test.go` | Unit tests for `injectLimit` and the full `executeSQLPipeline` (EXPLAIN error rejection, LIMIT injection, row cap, CTE+DELETE detection, etc.) |
| `context_test.go` | Unit tests for `EditorContextStore` (set/get, remove, concurrent access) |
| `tab_tools_test.go` | Unit tests for `open_sql_tab` tool (nil-emit graceful degradation, registration, empty SQL rejection, emit payload shape) |
| `notebook_tools_test.go` | Unit tests for notebook tools (registration gating for nil backend/workspace/emit, `open_notebook_tab` event payload, default title, truncation, cell kind validation, `buildNbformat` mapping with outputs/execution_count, kernel tool input validation, `read_notebook` sandbox and extension check) |
| `editor_tools_test.go` | Unit tests for editor context tools (empty store, content return, mode-gating, nil client handling) |
| `er_tools_test.go` | Unit tests for ER designer tools (`mergeAITables` / `mergeAITablesIntoState` merge logic, `describeERDelta` change-delta summaries, emit-gating for `open_er_designer`); `ERDesignerStateStore` unit tests (set/get, clear, concurrent access); registration gating for `get_er_designer_state` and `modify_er_designer`; tool behavior tests (designer open/closed, event emission, input validation, delta result + immediate cache consistency) |
| `pipeline_tools_test.go` | Unit tests for pipeline tools (registration in all modes, mode-gating for `preview_stage_file`, emit-gating for `open_task_graph`, nil client, input validation) |
| `account_tools_test.go` | Unit tests for account tools (registration in all modes, empty kind/name/schema validation) |
| `schema_tools_test.go` | Unit tests for schema tools (registration, validate_data_type valid/invalid, get_data_retention input validation, search_objects empty pattern, get_all_data_types, mode coverage) |
| `profile_tools_test.go` | Unit tests for profiling tools (registration in all modes, nil client, empty SQL validation) |
| `lineage_tools_test.go` | Unit tests for lineage tools (registration in all modes, nil client, missing fields, invalid kind validation) |
| `workspace_tools_test.go` | Unit tests for workspace tools (registration when `WorkspaceRoot` is set, absence when empty, input validation, path-escape sandbox tests, functional tests with temp directories) |
| `function_tools_test.go` | Unit tests for function/procedure tools (registration in all modes, nil fnStore, empty inputs, nil client, success paths for `build_call_statement` and `build_function_select`) |
| `builder_tools_test.go` | Unit tests for DDL builder tools (registration in all modes, empty db/schema validation, success paths for stage, pipe, secret, and all six integration builders) |
| `migration_tools_test.go` | Unit tests for migration/dbt tools (registration in all modes, workspace-gating, input validation, sandbox path tests, success paths for scan and script generation) |
| `mcp_test.go` | SSE round-trip test (external client lists tools), port-allocation test, diagnostics tool tests, mode-gating tests |
| `doc.go` | Package doc + `thaw:domain: MCP Server` annotation |

## Key types & functions

### `Manager`

| Function | Behaviour |
|---|---|
| `NewManager(emit)` | Empty registry with initialized `EditorContextStore` and `ERDesignerStateStore`. `emit` is an optional Wails event emitter for tab-delivery tools; pass `nil` in tests. Safe for concurrent use. |
| `EditorContext()` | Returns the shared `*EditorContextStore`; MCP tools read, frontend pushes state via App IPC. |
| `ERDesignerState()` | Returns the shared `*ERDesignerStateStore`; `get_er_designer_state` reads, frontend pushes state via App IPC. |
| `SetFnStore(store)` | Sets the function metadata store (`*fnmeta.Store`) on the manager. New sessions started after this call will expose function/procedure lookup tools. Called from `App.startup` after the fnStore is opened. |
| `SetNotebookBackend(nb)` | Sets the notebook/Snowpark backend (`NotebookBackend`) on the manager. New sessions started after this call will expose notebook tools (`get_notebook_completions`, `check_python_syntax`). Called from `App.startup` after the `snowparkSvc` is created. |
| `Start(label, connLabel, mode, port, client, cfg)` | Starts a session under a unique `label`; `port == 0` auto-assigns from `9100`. Takes ownership of `client`. Applies `SessionConfig` (role/warehouse pinning, secondary roles). |
| `UpdateMode(ctx, label, newMode)` | Changes the execution mode of a running session by mutating the server's tool registry in place. The SDK sends `tools/list_changed` notifications to connected clients automatically. Re-applies session config (role/warehouse pinning) when switching to a non-metadata mode. |
| `Stop(label)` | Stops and removes the named session, closing its connection. |
| `List()` | Snapshot of all sessions (`[]SessionInfo`) sorted by label. |
| `StopAll()` | Stops every session; called on app `shutdown` and `Disconnect`. |

Ports auto-assign sequentially from `basePort` (`9100`) up to `basePort+1000`. `allocatePortLocked` binds and returns the *held* `net.Listener` that `session.start` serves on, so the port is never released between the availability check and the real bind (no TOCTOU). An explicit duplicate or unavailable port is rejected.

### `EditorContextStore`

A `sync.RWMutex`-protected map of `tabID → {sql, result}` plus the active tab ID. The frontend pushes state into this store via four IPC methods in `internal/app/editorcontext.go`; MCP tool handlers read from it.

| Method | Purpose |
|---|---|
| `SetActiveTab(tabID, sql)` | Sets active tab + its SQL |
| `SetTabSQL(tabID, sql)` | Updates SQL for a specific tab |
| `SetTabResult(tabID, *ResultSummary)` | Stores latest result summary |
| `RemoveTab(tabID)` | Cleanup on tab close |
| `ActiveEditorSQL() (string, bool)` | Read by `get_current_editor_sql` |
| `QueryResultSummary(tabID) *ResultSummary` | Read by `get_query_results_summary`; empty tabID = active tab |

### Execution modes

| Mode | Constant | SQL tools | Behaviour |
|---|---|---|---|
| **Metadata Only** | `ExecutionModeMetadata` (`"metadata"`) | No | Schema browsing and diagnostics only. No SQL execution. |
| **Read-Only SQL** | `ExecutionModeReadonly` (`"readonly"`) | Yes | SQL execution via `execute_snowflake_sql`. Every statement passes through the EXPLAIN precompilation gate. Only read-only operations are allowed. |
| **Explain Only** | `ExecutionModeExplainOnly` (`"explain_only"`) | Yes | Same gate validation as readonly, but returns only the EXPLAIN plan metadata — the statement is never actually executed. |

### Session configuration

`SessionConfig` controls optional per-session settings applied at startup:

| Field | Effect |
|---|---|
| `Role` / `PinnedRole` | Runs `USE ROLE <role>` at session start. When `PinnedRole` is true, the `use_role` tool is not registered, preventing the AI client from switching roles. |
| `Warehouse` / `PinnedWarehouse` | Runs `USE WAREHOUSE <warehouse>` at session start. When `PinnedWarehouse` is true, the `use_warehouse` tool is not registered. |
| `SecondaryRoles` | When set to `"none"`, runs `USE SECONDARY ROLES NONE` at session start to restrict the session to only its primary role's grants. |
| `WorkspaceRoot` | The directory that workspace tools are sandboxed to (populated from the app's cached export directory). When empty, workspace tools are not registered at all. All path inputs are validated against this root using `filesystem.ValidateInsideOrEqual` (symlink-resolving, case-aware). |

### SQL execution pipeline

**Principle: no raw SQL reaches Snowflake without passing through EXPLAIN USING TABULAR first.** Snowflake's own query planner determines whether a statement is read-only — not fragile keyword heuristics. The pipeline (`executeSQLPipeline` in `sql_tools.go`) runs every statement through these steps:

1. **Empty/whitespace check** — reject blank input.
2. **Single-statement check** — `SplitStatements(sql)` must return exactly 1 statement. Multi-statement SQL is rejected.
3. **USE statement check** — `isUSEStatement(sql)` rejects USE statements with a descriptive error (use the dedicated context-switching tools instead). This is a best-effort early check; the EXPLAIN gate is the authoritative backstop.
4. **EXPLAIN USING TABULAR gate** — the statement is sent to Snowflake's `EXPLAIN USING TABULAR`. If EXPLAIN itself errors (e.g. on `SHOW`, `DESCRIBE`, `LIST`, DDL, or any unsupported statement type), the statement is rejected as "not supported". If the plan contains non-read-only operations, the statement is rejected. This catches cases like `WITH target AS (...) DELETE FROM t` where a keyword classifier would be fooled by the leading `WITH`.
5. **explain_only mode** — return the gate verdict (plan metadata) without executing.
6. **readonly mode** — wrap the query with `injectLimit` (`SELECT * FROM (<query>) AS _mcp_limit LIMIT 100`) to prevent full-table scans, execute, and cap at `maxMCPResultRows` (1000).

Metadata needs (listing databases, describing tables, etc.) are served by the dedicated schema-browsing tools (`list_databases`, `list_schemas`, `list_objects`, `describe_table`, `get_ddl`, `get_table_foreign_keys`) which use safe Go methods internally — not raw SQL passthrough.

**`CheckGate` backwards-compatibility**: The original `CheckGate` function is preserved unchanged (it still runs layers 1–3 plus EXPLAIN). The pipeline delegates to `CheckGate`, which internally calls the extracted `checkExplainPlan`.

**Caveats**: The pipeline is not a substitute for a scoped read-only Snowflake role. It fails safe by over-rejecting (any statement EXPLAIN can't handle or any unknown operation is denied). The real security boundary is the Snowflake role's grants — the pipeline provides an additional defense layer.

### Tools

The server exposes 60 tools in the baseline metadata mode (no workspace, no emit, no editorCtx, no fnStore, no nb). Additional tools are registered when optional dependencies are provided: workspace tools (+8), emit-gated tools (+4), editor context tools (+2–3), notebook backend tools (+2), and SQL execution tools (+6–7 in readonly/explain_only modes):

**Schema-browsing tools** (always registered, `tools.go`): `get_session_context`, `list_databases`, `list_schemas`, `list_objects`, `describe_table`, `get_ddl`, `get_table_foreign_keys`.

**Extended schema discovery tools** (always registered, `schema_tools.go`):

| Tool | Description |
|---|---|
| `get_schema_foreign_keys` | Bulk FK listing for an entire schema (cheaper than per-table queries) |
| `get_database_ddl` | Complete DDL export for a database (all schemas and objects) |
| `get_er_model` | ER diagram data: tables with columns, PKs, nullability, and FK relationships |
| `search_objects` | Cross-schema object and column name search using SQL ILIKE patterns |
| `get_all_data_types` | Complete list of supported Snowflake data types with parameter syntax hints |
| `validate_data_type` | Validate a data type string and return the normalised form |
| `list_dropped_tables` | List tables dropped in a schema (available for time-travel undrop) |
| `list_dropped_schemas` | List schemas dropped in a database (available for time-travel undrop) |
| `get_data_retention` | Return data retention period (days) at database, schema, or table level |

**Account & infrastructure tools** (always registered, `account_tools.go`):

| Tool | Description |
|---|---|
| `list_roles` | List all roles visible to the current session |
| `list_available_roles` | List roles available (grantable) to the current user |
| `get_role_ddl` | Return the CREATE ROLE DDL for a role, including granted privileges |
| `list_warehouses` | List all warehouses accessible to the current session |
| `get_warehouse_ddl` | Return the CREATE WAREHOUSE DDL for a warehouse |
| `list_integrations` | List integrations of a given kind (API, NOTIFICATION, SECURITY, STORAGE, CATALOG, EXTERNAL ACCESS) |
| `list_secrets` | List all secrets visible in the account (name, database, schema) |
| `list_file_formats` | List file formats defined in a schema |

**SQL diagnostics tools** (always registered, `diag_tools.go`): `validate_sql`, `suggest_join_conditions`, `format_sql`, `get_snowflake_keywords`.

**Query profiling tools** (always registered, `profile_tools.go`):

| Tool | Description |
|---|---|
| `explain_query` | Full EXPLAIN plan tree (partitions, bytes, operations) plus performance diagnostics (full scans, cartesian joins, row explosion) |
| `get_explain_diagnostics` | Diagnostics only (lighter than `explain_query` when you only need the warnings, not the full plan tree) |

**Object lineage tools** (always registered, `lineage_tools.go`):

| Tool | Description |
|---|---|
| `get_object_lineage` | Recursive dependency tree for a VIEW, PROCEDURE, or FUNCTION (upstream impact analysis) |
| `get_schema_cross_deps` | Cross-schema references from views in a schema |
| `get_database_cross_deps` | Combined cross-schema references across multiple schemas in a database (deduplicated) |

**Pipeline tools** (`pipeline_tools.go`; task, stage, and pipe inspection):

| Tool | Mode gate | Description |
|---|---|---|
| `list_tasks` | All modes | List tasks in a schema with state, predecessors, and last run status |
| `get_task_run_history` | All modes | Run history for a task (or root task graph); configurable day range (default 7, max 30) |
| `get_task_dependencies` | All modes | Topological order and child status for a root task's dependency graph |
| `list_stage_files` | All modes | List files in a Snowflake stage with optional regex filter |
| `preview_stage_file` | readonly, explain_only only (NOT metadata) | Preview up to 50 rows from a stage file with configurable file format |
| `get_pipe_status` | All modes | Snowpipe status via `SYSTEM$PIPE_STATUS` (execution state, pending files, notification channel) |
| `get_pipe_copy_history` | All modes | Snowpipe copy history from `INFORMATION_SCHEMA` with optional time/status/file filters |
| `open_task_graph` | All modes (emit-gated) | Open the task graph visualization in Thaw; emits `mcp:open-task-graph` Wails event |

`preview_stage_file` is suppressed in metadata mode because it reads actual file data. `open_task_graph` is only registered when `emit` is non-nil (i.e. running inside the app, not in tests). The emit pattern matches `open_sql_tab` — panic recovery around the Wails event emitter prevents a torn-down context from crashing the MCP server goroutine.

**Function metadata tools** (always registered, `function_tools.go`):

| Tool | Description |
|---|---|
| `search_functions` | Search the local function metadata cache by name prefix |
| `get_function_tooltip` | Look up function metadata (signature, description, type) by exact name |
| `get_procedure_params` | Retrieve parameter metadata for a stored procedure from its Snowflake DDL |
| `get_function_info` | Retrieve parameter and return-type metadata for a user-defined function from its DDL |
| `build_call_statement` | Generate a syntactically correct CALL statement for a stored procedure (pure builder, no execution) |
| `build_function_select` | Generate a SELECT statement to invoke a UDF (scalar or table function form; pure builder, no execution) |

`search_functions` and `get_function_tooltip` use the local `fnmeta.Store` (populated from the fallback bundle and optional Snowflake sync). `get_procedure_params` and `get_function_info` fetch DDL from the live Snowflake connection. `build_call_statement` and `build_function_select` are pure SQL generators — no client needed.

**DDL builder tools** (always registered, `builder_tools.go`):

| Tool | Description |
|---|---|
| `build_create_stage_sql` | Generate CREATE STAGE DDL |
| `build_alter_stage_sql` | Generate ALTER STAGE DDL |
| `build_create_file_format_sql` | Generate CREATE FILE FORMAT DDL |
| `build_create_pipe_sql` | Generate CREATE PIPE DDL with COPY INTO definition |
| `build_refresh_pipe_sql` | Generate ALTER PIPE ... REFRESH with optional prefix/modifiedAfter filters |
| `build_create_secret_sql` | Generate CREATE SECRET DDL |
| `build_storage_integration_sql` | Generate CREATE STORAGE INTEGRATION DDL |
| `build_api_integration_sql` | Generate CREATE API INTEGRATION DDL |
| `build_catalog_integration_sql` | Generate CREATE CATALOG INTEGRATION DDL |
| `build_external_access_integration_sql` | Generate CREATE EXTERNAL ACCESS INTEGRATION DDL |
| `build_notification_integration_sql` | Generate CREATE NOTIFICATION INTEGRATION DDL |
| `build_security_integration_sql` | Generate CREATE SECURITY INTEGRATION DDL |

All builder tools are pure SQL generators — no Snowflake client required, no SQL execution. They delegate to the existing domain builder packages (`stage`, `fileformat`, `pipe`, `secret`, `integrations`) and return the generated DDL string.

**Migration & dbt tools** (`migration_tools.go`):

| Tool | Gating | Description |
|---|---|---|
| `scan_migration_source` | Workspace-gated | Scan a local directory for `.sql` files and return the DDL objects found (delegates to `migration.ScanSource`) |
| `analyze_migration` | Always registered (nil-client check at call time) | Compare local DDL objects against a live Snowflake database and return a diff for each object (delegates to `migration.Analyze`) |
| `generate_migration_script` | Always registered (pure function) | Generate a human-readable SQL migration script from diff items (delegates to `migration.GenerateScript`) |
| `generate_dbt_project` | Workspace-gated (nil-client check at call time) | Scaffold a dbt project pre-wired to the active Snowflake connection (delegates to `dbt.CreateProject`) |

`scan_migration_source` and `generate_dbt_project` are only registered when `WorkspaceRoot` is set in `SessionConfig`, matching the `registerWorkspaceTools` pattern. Both validate path inputs against the workspace root using `filesystem.ValidateInsideOrEqual`. `analyze_migration` and `generate_migration_script` are always registered (like builder tools); `analyze_migration` checks for a nil client at call time. All tools use a no-op emit callback since MCP does not stream progress events.

**Workspace tools** (registered when `WorkspaceRoot` is set, `workspace_tools.go`; sandboxed to the configured workspace root):

| Tool | Description |
|---|---|
| `git_status` | Git status for a directory: branch, modified/added/deleted files, remote info, ahead count |
| `git_list_branches` | List all local and remote branches in a git repository |
| `git_get_head_file` | Content of a file as it exists in the HEAD commit |
| `git_diff_lines` | Line-level diff between HEAD and current content (added, modified, deleted line numbers) |
| `list_directory` | Direct children of a directory with name, path, size, and type |
| `read_file` | Read file content (up to 50 KB) |
| `search_files` | Recursive text/regex search across files in a directory |

All workspace tools that accept a directory or file path validate the input against `WorkspaceRoot` using `filesystem.ValidateInsideOrEqual` before delegating to the underlying implementation. This prevents an MCP client from reading files outside the workspace (e.g. `~/.ssh/id_rsa`). The `git_diff_lines` tool is exempt as it operates on line arrays, not file paths. `git_get_head_file` uses `filesystem.ValidatePathOrAncestorInsideOrEqual`, which validates via the nearest existing ancestor instead of requiring the path to exist — so it still returns HEAD content for a file deleted from the working tree, while keeping the containment check.

**Editor context tools** (`editor_tools.go`, registered when `EditorContextStore` is non-nil):

| Tool | Mode gate | Data source |
|---|---|---|
| `get_current_editor_sql` | All modes | `EditorContextStore.ActiveEditorSQL()` |
| `get_query_results_summary` | readonly, explain_only only (NOT metadata) | `EditorContextStore.QueryResultSummary()` |
| `get_query_history` | All modes | `queryhistory.GetQueryHistory()` via session's `*snowflake.Client` |

`get_query_results_summary` is suppressed in metadata mode because it exposes actual data rows. `get_query_history` uses the MCP session's own Snowflake client to query `INFORMATION_SCHEMA.QUERY_HISTORY`; it resolves the session user via `GetCurrentUserCached` (`SELECT CURRENT_USER()`, cached on the client for the connection's lifetime so it survives mode-switch tool re-registration) and passes it as the explicit `USER_NAME` filter, since user-scoped history now requires a non-empty user.

**SQL execution tools** (readonly/explain_only only, `sql_tools.go`):

| Tool | Purpose | Pinning |
|---|---|---|
| `execute_snowflake_sql` | Execute a single read-only SQL statement through the EXPLAIN gate | Always registered |
| `use_role` | Switch the active Snowflake role | Omitted when `PinnedRole` |
| `use_warehouse` | Switch the active Snowflake warehouse | Omitted when `PinnedWarehouse` |
| `use_database` | Switch the active Snowflake database | Always registered |
| `use_schema` | Switch the active Snowflake schema | Always registered |

**Notebook/Snowpark tools** (`notebook_tools.go`):

| Tool | Gating | Description |
|---|---|---|
| `read_notebook` | Workspace-gated | Read a Jupyter notebook (.ipynb) file from the workspace (up to 5 MB, `.ipynb` extension required); returns raw JSON |
| `get_notebook_completions` | Backend-gated | Get Python intellisense completions from the running kernel at a cursor position |
| `check_python_syntax` | Backend-gated | Validate Python syntax and check for common errors using the running kernel |
| `open_notebook_tab` | Emit-gated | Open a new notebook tab in Thaw with pre-filled cells (python, markdown, sql); builds nbformat v4 JSON; emits `mcp:open-notebook-tab` Wails event |

`read_notebook` is only registered when `WorkspaceRoot` is set; path inputs are validated against the workspace root and must have a `.ipynb` extension. File size is checked via `os.Stat` before reading to prevent OOM on large files. Kernel-dependent tools (`get_notebook_completions`, `check_python_syntax`) are only registered when the `NotebookBackend` is non-nil (set via `SetNotebookBackend`). `open_notebook_tab` is only registered when `emit` is non-nil. The `NotebookBackend` interface is implemented by `notebookBackendAdapter` in `internal/app/mcp_notebook.go`, which delegates to `snowpark.Service` and maps snowpark types to MCP-local duplicates.

**Tab-delivery tools** (`tab_tools.go`, registered when `emit` is non-nil):

| Tool | Purpose | Event |
|---|---|---|
| `open_sql_tab` | Format SQL with user prefs, run diagnostics, open a new editor tab | Emits `mcp:open-sql-tab` Wails event with `{title, sql, markers}` |

`open_sql_tab` completes the Phase 1 MCP round-trip: the AI validates and formats SQL, then delivers it into a new editor tab. The user sees diagnostics inline and must manually run the query (human-in-the-loop preserved). The event emitter callback is injected into `Manager` at construction (`NewManager(emit)`) and threaded through `buildServer` to `registerTabTools`. The emitter pattern follows the established `migration.NewService` approach — `internal/mcp` cannot import `internal/app`, so the Wails runtime is accessed via a closure wired in `App.startup()`.

**ER designer tools** (`er_tools.go`):

| Tool | Gating | Purpose | Event |
|---|---|---|---|
| `open_er_designer` | Emit-gated | Fetch live ER data, merge AI-generated tables, open the ER designer | Emits `mcp:open-er-designer` Wails event with `{database, merged, baseline}` |
| `get_er_designer_state` | erState-gated | Read current tables/columns/FKs from the open designer | None (reads from `ERDesignerStateStore` cache) |
| `modify_er_designer` | Emit+erState-gated | Push AI table modifications into the open designer; returns a change-delta summary | Emits `mcp:modify-er-designer` Wails event with `{tables}`; updates the `ERDesignerStateStore` cache in place |

`open_er_designer` enables the "AI scaffolds, human refines" workflow: the AI generates tables from natural language and delivers them onto the interactive ER canvas. The `mergeAITables` function merges AI tables into the live schema — matching tables (by uppercase `SCHEMA.NAME`) are replaced, new tables are appended, untouched live tables are preserved. The frontend receives both the merged data (for display) and the baseline (original live schema for diff SQL generation). The user reviews the visual model and the generated SQL diff before applying.

`get_er_designer_state` and `modify_er_designer` are registered via `registerERDesignerStateTools`, called from `session.start()` after `buildServer()` to avoid changing the `buildServer` signature (and touching ~60 test call sites). `get_er_designer_state` reads from the `ERDesignerStateStore` cache (pushed by the frontend via IPC). `modify_er_designer` validates inputs, checks the designer is open, and emits a `mcp:modify-er-designer` event — the frontend runs the merge against its React state, preserving UUIDs and canvas positions, and highlights the changed tables on the canvas (the latest change only).

`modify_er_designer` also (a) computes a **change delta** — added tables and per-table column additions/removals/type-PK-nullability-FK modifications — via `describeERDelta`, and returns it in the tool result so the LLM can self-correct without re-reading the whole model; and (b) applies the same merge the frontend will apply (`mergeAITablesIntoState`) and writes it straight into the `ERDesignerStateStore`, so a `get_er_designer_state` call inside the frontend's 300 ms debounce window reflects the change instead of returning pre-modification data. The delta is derived from the snapshot taken **before** the merge (not a follow-up state read), so it never races the debounced push-back. The frontend's later, authoritative push overwrites the cache with an equivalent result.

**Diagnostics vs. EXPLAIN gate**: The diagnostics tools serve the *editor/notebook delivery path* — the AI writes SQL, validates it, then places it in front of the human for review. The EXPLAIN gate validates SQL immediately before execution in the `execute_snowflake_sql` tool.

### Editor context bridge

Editor SQL and query results live in the frontend Zustand `queryStore`, while MCP tools run in `internal/mcp/` which cannot import `internal/app`. The bridge works as follows:

1. **`EditorContextStore`** (`context.go`) — a `sync.RWMutex`-protected in-memory store owned by `Manager`, initialized in `NewManager()`.
2. **App IPC methods** (`internal/app/editorcontext.go`) — four thin delegators (`UpdateEditorContext`, `UpdateEditorTabSQL`, `UpdateQueryResult`, `RemoveEditorTab`) that write into `Manager.EditorContext()`.
3. **Frontend sync hook** (`frontend/src/hooks/useEditorContextSync.ts`) — a React hook mounted once in `QueryPage.tsx` that subscribes to `queryStore` and pushes state changes to the backend via IPC (debounced SQL updates, immediate tab switch and result notifications, tab removal cleanup).

### ER designer state bridge

The ER designer's table state lives in `ERDesigner.tsx` (React `useState`), while the `get_er_designer_state` and `modify_er_designer` MCP tools run in `internal/mcp/`. The bridge follows the same pattern as the editor context bridge:

1. **`ERDesignerStateStore`** (`er_designer_state.go`) — a `sync.RWMutex`-protected cache owned by `Manager`, initialized in `NewManager()`. Holds `ERDesignerState` (database + `[]ERDesignerTableOut`), or nil when the designer is closed.
2. **App IPC methods** (`internal/app/erdesigner.go`) — two thin delegators (`UpdateERDesignerState`, `ClearERDesignerState`) that write into `Manager.ERDesignerState()`.
3. **Frontend sync** (`ERDesigner.tsx`) — pushes state on mount, on debounced (300ms) `tables` changes, and clears on unmount. Listens for `mcp:modify-er-designer` Wails events and merges AI tables via `mergeAITablesIntoDesigner`.

## Patterns & integration

The `*App` delegators in `internal/app/mcp.go` (`StartMCPSession`, `StopMCPSession`, `UpdateMCPSessionMode`, `ListMCPSessions`, `GetMCPSessionConfig`) open a fresh `*snowflake.Client` from `App.connectParams` and hand it to `Manager.Start`. `StartMCPSession` enforces the admin-lockable `mcpServer` feature flag via the **effective** flags (`App.GetFeatureFlags()`, which applies IT-admin overrides) so an admin lock cannot be bypassed through the native menu. Sessions are **not persisted** — they exist only for the lifetime of the process and are not restored on the next launch. Frontend surface: `MCPSessionsModal.tsx`, `MCPIndicator.tsx`, and `mcpStore.ts`.

Each session opens its **own** `snowflake.NewClient` (a separate Snowflake session, independent of the UI tab sessions). With interactive authenticators (e.g. `externalbrowser`) starting a session may therefore trigger a fresh auth prompt, and every running session consumes one additional Snowflake session.

A session's SSE endpoint is `http://127.0.0.1:<port>/sse`; `GetMCPSessionConfig` formats the standard client config block `{"mcpServers": {"thaw-<label>": {"type": "sse", "url": "…/sse", "headers": {"Authorization": "Bearer <token>"}}}}`. The per-session token is carried in an `Authorization: Bearer` header, **not** in the URL query string, so the token stays out of local proxy logs, process listings (`ps aux`), and shell history. The `url` in the config is the token-free endpoint. `SessionInfo.URL` is likewise token-free (for display). The token itself is surfaced only through `Manager.SessionEndpoint` (used by `GetMCPSessionConfig`, returns the token-free URL + token as separate values) and `Manager.SessionToken` (used to persist the token to config), never in `SessionInfo`, so it is not broadcast in every `List()` snapshot. The `tokenGuard` middleware accepts both the header and a `?token=…` query parameter, so a URL-only client can append the token to the endpoint as a fallback. All URLs use `127.0.0.1` (not `localhost`) to match the listener's bind address.

On teardown (`stop`/`StopAll`, fired by `Disconnect` and app `shutdown`), `http.Shutdown` runs with a 5s deadline and the client is then closed unconditionally. SSE connections are long-lived/hijacked and are not awaited by `Shutdown`, so a tool call in flight at teardown can hit a closed client and error out — this is expected on teardown.

## Security

The listener binds only the loopback interface (`127.0.0.1`) and the `loopbackGuard` middleware (`security.go`) rejects any request whose `Host` header is not loopback or whose `Origin` header is cross-origin — this defends against DNS-rebinding attacks where a malicious web page the user has open targets `http://localhost:<port>/sse`.

Each session also has a **per-session auth token** (`tokenGuard`, `security.go`). The token (32 crypto-random bytes, base64url) is required to open the session-creating SSE `GET`, presented either as `Authorization: Bearer <token>` (preferred; what generated configs use) or a `?token=…` query parameter (fallback for URL-only clients). The follow-up message `POST`s are **not** separately token-checked: the go-sdk builds the message endpoint via `req.URL.Parse("?sessionid=…")`, which replaces the query string and so drops the token, but the `sessionid` it issues is crypto-random and delivered only over the authenticated `GET` stream — a process that cannot pass the `GET` token never learns a valid `sessionid`, so it can neither open a session nor post into one. This closes the local-process gap from [#350](https://github.com/Technarion-Oy/thaw/issues/350).

Rejected (unauthenticated) `GET`s are logged for observability via a per-session `authFailureLogger` (`security.go`). Logging is **rate-limited** to at most one line per 10 s per session — a co-resident process probing the loopback port surfaces as `mcp: rejected unauthenticated session request` warnings (with an `attempts` count folding in suppressed failures) without flooding the log. No rate *limiting* of the requests themselves is applied: the 256-bit token entropy makes brute-force infeasible regardless of attempt rate, so the logging is purely for visibility.

The token defends against other **non-admin** local processes/users only. A local administrator (or `SYSTEM`) can read the app's process memory, read files regardless of ACL, and capture loopback traffic, so they are outside the boundary this token can enforce. For SQL execution modes, the EXPLAIN precompilation gate provides defense-in-depth, but the real security boundary is the Snowflake role's grants — always use a scoped read-only role for sessions that can execute SQL. Sessions must be started explicitly and should be stopped when not in use; the copied client configuration embeds the token and must be treated as a secret.

## Gotchas

The Go MCP SDK's generic `AddTool[In, Out]` infers an output JSON schema from `Out` and **panics at registration** if that schema's type is not `"object"`. Tools that return arrays, strings, or slices of structs therefore declare `Out` as `any` (the SDK then omits the output schema) and return `nil` structured output, delivering the payload as text content via `jsonResult`/`textResult`. Never give an MCP tool a concrete non-struct `Out` type.
