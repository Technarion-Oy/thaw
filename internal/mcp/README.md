# internal/mcp

> Model Context Protocol (MCP) servers that expose the active Snowflake connection to external AI clients over a local SSE/HTTP transport, with configurable execution modes and an EXPLAIN precompilation safety gate.

## Responsibility

Hosts one or more MCP servers, each bound to its own dedicated `*snowflake.Client`, on `localhost`. A `Manager` owns the set of running sessions; each session runs an `http.Server` serving the Go MCP SDK's SSE handler and registers schema-browsing, SQL diagnostics, editor context, and (optionally) SQL execution tools. Sessions are started and stopped only on explicit user action (View → MCP Sessions); none start automatically.

`internal/mcp` must **not** import `internal/app` — the dependency is one-way (`App` holds a `*mcp.Manager`). All Snowflake access goes through the `*snowflake.Client` handed to each session, mirroring the isolated per-tab session pattern.

## Key files

| File | Purpose |
|---|---|
| `manager.go` | `Manager` (multi-session registry), `SessionInfo`/`SessionConfig` types, execution mode constants, port allocation, `Start`/`Stop`/`UpdateMode`/`List`/`StopAll`, `EditorContext()` accessor |
| `session.go` | Per-session `http.Server` + SSE lifecycle (`start`/`stop`/`updateMode`/`info`); serves on the held loopback listener and owns/closes its `*snowflake.Client`. `updateMode` mutates the existing server's tool registry via `RemoveTools`/`AddTool` — the SDK sends `tools/list_changed` notifications so connected clients update seamlessly. If the serve goroutine exits unexpectedly it closes the client and self-removes from the `Manager` (`removeIfPresent`) so no dead row or leaked connection lingers |
| `security.go` | `loopbackGuard` middleware (rejects non-loopback `Host`/cross-origin `Origin` — DNS-rebinding defense), `tokenGuard` middleware (per-session token auth on the SSE GET), and `newSessionToken` (crypto-random token) |
| `server.go` | `buildServer(client, mode, cfg, editorCtx, emit)` — constructs the MCP server and registers tools based on execution mode; `modeSpecificToolNames` lists tools that `updateMode` removes/re-registers on mode switch |
| `tools.go` | Tool input structs + `registerTools` (schema-browsing tools); `jsonResult`/`textResult` content helpers |
| `schema_tools.go` | `registerSchemaTools` — extended schema discovery tools (`get_schema_foreign_keys`, `get_database_ddl`, `get_er_model`, `search_objects`, `get_all_data_types`, `validate_data_type`, `list_dropped_tables`, `list_dropped_schemas`, `get_data_retention`); always registered in all modes |
| `account_tools.go` | `registerAccountTools` — account & infrastructure tools (`list_roles`, `list_available_roles`, `get_role_ddl`, `list_warehouses`, `get_warehouse_ddl`, `list_integrations`, `list_secrets`, `list_file_formats`); always registered in all modes |
| `diag_tools.go` | `registerDiagTools` — SQL diagnostics & validation tools (`validate_sql`, `suggest_join_conditions`, `format_sql`, `get_snowflake_keywords`); type-conversion helpers for sqleditor ↔ snowflake types |
| `profile_tools.go` | `registerProfileTools` — query profiling tools (`explain_query`, `get_explain_diagnostics`); wraps `queryprofile.RunExplain` and `queryprofile.GetExplainDiagnostics`; always registered in all modes |
| `lineage_tools.go` | `registerLineageTools` — object lineage and cross-dependency tools (`get_object_lineage`, `get_schema_cross_deps`, `get_database_cross_deps`); wraps `Client.GetObjectDependencies`, `Client.GetSchemaCrossDeps`, `Client.GetDatabaseCrossDeps`; always registered in all modes |
| `context.go` | `EditorContextStore` — concurrency-safe in-memory store for per-tab editor SQL and result summaries; `ResultSummary` and `QueryHistoryEntry` types |
| `editor_tools.go` | `registerEditorTools` — editor context tools (`get_current_editor_sql`, `get_query_results_summary`, `get_query_history`); bridges frontend editor state to MCP clients |
| `tab_tools.go` | `registerTabTools` — tab-delivery tool (`open_sql_tab`); formats SQL with user prefs, runs diagnostics, emits `mcp:open-sql-tab` Wails event. Registered when `emit` is non-nil. `OpenSqlTabPayload` type, `loadEditorPrefs` helper |
| `gate.go` | EXPLAIN precompilation gate: `queryRunner` interface, `CheckGate` (3-layer validation), `checkExplainPlan`, `readOnlyOps` allow-list, `extractOperations`, `isUSEStatement` |
| `sql_tools.go` | `registerSQLTools` — SQL execution tool (`execute_snowflake_sql`) with EXPLAIN-gated pipeline (`executeSQLPipeline`), LIMIT injection (`injectLimit`), and trusted context-switching tools (`use_role`, `use_warehouse`, `use_database`, `use_schema`); only registered in `readonly`/`explain_only` modes |
| `gate_test.go` | Unit tests for the EXPLAIN gate and `checkExplainPlan` |
| `sql_tools_test.go` | Unit tests for `injectLimit` and the full `executeSQLPipeline` (EXPLAIN error rejection, LIMIT injection, row cap, CTE+DELETE detection, etc.) |
| `context_test.go` | Unit tests for `EditorContextStore` (set/get, remove, concurrent access) |
| `tab_tools_test.go` | Unit tests for `open_sql_tab` tool (nil-emit graceful degradation, registration, empty SQL rejection, emit payload shape) |
| `editor_tools_test.go` | Unit tests for editor context tools (empty store, content return, mode-gating, nil client handling) |
| `account_tools_test.go` | Unit tests for account tools (registration in all modes, empty kind/name/schema validation) |
| `schema_tools_test.go` | Unit tests for schema tools (registration, validate_data_type valid/invalid, get_data_retention input validation, search_objects empty pattern, get_all_data_types, mode coverage) |
| `profile_tools_test.go` | Unit tests for profiling tools (registration in all modes, nil client, empty SQL validation) |
| `lineage_tools_test.go` | Unit tests for lineage tools (registration in all modes, nil client, missing fields, invalid kind validation) |
| `mcp_test.go` | SSE round-trip test (external client lists tools), port-allocation test, diagnostics tool tests, mode-gating tests |
| `doc.go` | Package doc + `thaw:domain: MCP Server` annotation |

## Key types & functions

### `Manager`

| Function | Behaviour |
|---|---|
| `NewManager(emit)` | Empty registry with initialized `EditorContextStore`. `emit` is an optional Wails event emitter for tab-delivery tools; pass `nil` in tests. Safe for concurrent use. |
| `EditorContext()` | Returns the shared `*EditorContextStore`; MCP tools read, frontend pushes state via App IPC. |
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

The server exposes 36 tools in metadata mode and up to 42 tools in readonly/explain_only modes (with `EditorContextStore` and `emit` provided):

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

**Editor context tools** (`editor_tools.go`, registered when `EditorContextStore` is non-nil):

| Tool | Mode gate | Data source |
|---|---|---|
| `get_current_editor_sql` | All modes | `EditorContextStore.ActiveEditorSQL()` |
| `get_query_results_summary` | readonly, explain_only only (NOT metadata) | `EditorContextStore.QueryResultSummary()` |
| `get_query_history` | All modes | `queryhistory.GetQueryHistory()` via session's `*snowflake.Client` |

`get_query_results_summary` is suppressed in metadata mode because it exposes actual data rows. `get_query_history` uses the MCP session's own Snowflake client to query `INFORMATION_SCHEMA.QUERY_HISTORY`.

**SQL execution tools** (readonly/explain_only only, `sql_tools.go`):

| Tool | Purpose | Pinning |
|---|---|---|
| `execute_snowflake_sql` | Execute a single read-only SQL statement through the EXPLAIN gate | Always registered |
| `use_role` | Switch the active Snowflake role | Omitted when `PinnedRole` |
| `use_warehouse` | Switch the active Snowflake warehouse | Omitted when `PinnedWarehouse` |
| `use_database` | Switch the active Snowflake database | Always registered |
| `use_schema` | Switch the active Snowflake schema | Always registered |

**Tab-delivery tools** (`tab_tools.go`, registered when `emit` is non-nil):

| Tool | Purpose | Event |
|---|---|---|
| `open_sql_tab` | Format SQL with user prefs, run diagnostics, open a new editor tab | Emits `mcp:open-sql-tab` Wails event with `{title, sql, markers}` |

`open_sql_tab` completes the Phase 1 MCP round-trip: the AI validates and formats SQL, then delivers it into a new editor tab. The user sees diagnostics inline and must manually run the query (human-in-the-loop preserved). The event emitter callback is injected into `Manager` at construction (`NewManager(emit)`) and threaded through `buildServer` to `registerTabTools`. The emitter pattern follows the established `migration.NewService` approach — `internal/mcp` cannot import `internal/app`, so the Wails runtime is accessed via a closure wired in `App.startup()`.

**Diagnostics vs. EXPLAIN gate**: The diagnostics tools serve the *editor/notebook delivery path* — the AI writes SQL, validates it, then places it in front of the human for review. The EXPLAIN gate validates SQL immediately before execution in the `execute_snowflake_sql` tool.

### Editor context bridge

Editor SQL and query results live in the frontend Zustand `queryStore`, while MCP tools run in `internal/mcp/` which cannot import `internal/app`. The bridge works as follows:

1. **`EditorContextStore`** (`context.go`) — a `sync.RWMutex`-protected in-memory store owned by `Manager`, initialized in `NewManager()`.
2. **App IPC methods** (`internal/app/editorcontext.go`) — four thin delegators (`UpdateEditorContext`, `UpdateEditorTabSQL`, `UpdateQueryResult`, `RemoveEditorTab`) that write into `Manager.EditorContext()`.
3. **Frontend sync hook** (`frontend/src/hooks/useEditorContextSync.ts`) — a React hook mounted once in `QueryPage.tsx` that subscribes to `queryStore` and pushes state changes to the backend via IPC (debounced SQL updates, immediate tab switch and result notifications, tab removal cleanup).

## Patterns & integration

The `*App` delegators in `internal/app/mcp.go` (`StartMCPSession`, `StopMCPSession`, `UpdateMCPSessionMode`, `ListMCPSessions`, `GetMCPSessionConfig`) open a fresh `*snowflake.Client` from `App.connectParams` and hand it to `Manager.Start`. `StartMCPSession` enforces the admin-lockable `mcpServer` feature flag via the **effective** flags (`App.GetFeatureFlags()`, which applies IT-admin overrides) so an admin lock cannot be bypassed through the native menu. Sessions are **not persisted** — they exist only for the lifetime of the process and are not restored on the next launch. Frontend surface: `MCPSessionsModal.tsx`, `MCPIndicator.tsx`, and `mcpStore.ts`.

Each session opens its **own** `snowflake.NewClient` (a separate Snowflake session, independent of the UI tab sessions). With interactive authenticators (e.g. `externalbrowser`) starting a session may therefore trigger a fresh auth prompt, and every running session consumes one additional Snowflake session.

A session's SSE endpoint is `http://127.0.0.1:<port>/sse`; `GetMCPSessionConfig` formats the standard client config block `{"mcpServers": {"thaw-<label>": {"url": "..."}}}`, where the URL carries the per-session token (`?token=…`). `SessionInfo.URL` is the token-free endpoint (for display); the token is surfaced only through `Manager.AuthenticatedURL` (used by `GetMCPSessionConfig`) so it is not broadcast in every `List()` snapshot. Both URLs use `127.0.0.1` (not `localhost`) to match the listener's bind address.

On teardown (`stop`/`StopAll`, fired by `Disconnect` and app `shutdown`), `http.Shutdown` runs with a 5s deadline and the client is then closed unconditionally. SSE connections are long-lived/hijacked and are not awaited by `Shutdown`, so a tool call in flight at teardown can hit a closed client and error out — this is expected on teardown.

## Security

The listener binds only the loopback interface (`127.0.0.1`) and the `loopbackGuard` middleware (`security.go`) rejects any request whose `Host` header is not loopback or whose `Origin` header is cross-origin — this defends against DNS-rebinding attacks where a malicious web page the user has open targets `http://localhost:<port>/sse`.

Each session also has a **per-session auth token** (`tokenGuard`, `security.go`). The token (32 crypto-random bytes, base64url) is required to open the session-creating SSE `GET`, presented either as `Authorization: Bearer <token>` or a `?token=…` query parameter. The follow-up message `POST`s are **not** separately token-checked: the go-sdk builds the message endpoint via `req.URL.Parse("?sessionid=…")`, which replaces the query string and so drops the token, but the `sessionid` it issues is crypto-random and delivered only over the authenticated `GET` stream — a process that cannot pass the `GET` token never learns a valid `sessionid`, so it can neither open a session nor post into one. This closes the local-process gap from [#350](https://github.com/Technarion-Oy/thaw/issues/350).

The token defends against other **non-admin** local processes/users only. A local administrator (or `SYSTEM`) can read the app's process memory, read files regardless of ACL, and capture loopback traffic, so they are outside the boundary this token can enforce. For SQL execution modes, the EXPLAIN precompilation gate provides defense-in-depth, but the real security boundary is the Snowflake role's grants — always use a scoped read-only role for sessions that can execute SQL. Sessions must be started explicitly and should be stopped when not in use; the copied client configuration embeds the token and must be treated as a secret.

## Gotchas

The Go MCP SDK's generic `AddTool[In, Out]` infers an output JSON schema from `Out` and **panics at registration** if that schema's type is not `"object"`. Tools that return arrays, strings, or slices of structs therefore declare `Out` as `any` (the SDK then omits the output schema) and return `nil` structured output, delivering the payload as text content via `jsonResult`/`textResult`. Never give an MCP tool a concrete non-struct `Out` type.
