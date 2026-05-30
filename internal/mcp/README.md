# internal/mcp

> Read-only Model Context Protocol (MCP) servers that expose the active Snowflake connection to external AI clients over a local SSE/HTTP transport.

## Responsibility

Hosts one or more MCP servers, each bound to its own dedicated `*snowflake.Client`, on `localhost`. A `Manager` owns the set of running sessions; each session runs an `http.Server` serving the Go MCP SDK's SSE handler and registers a fixed set of read-only schema-browsing tools. Sessions are started and stopped only on explicit user action (View → MCP Sessions); none start automatically.

`internal/mcp` must **not** import `internal/app` — the dependency is one-way (`App` holds a `*mcp.Manager`). All Snowflake access goes through the `*snowflake.Client` handed to each session, mirroring the isolated per-tab session pattern.

## Key files

| File | Purpose |
|---|---|
| `manager.go` | `Manager` (multi-session registry), `SessionInfo` type, port allocation, `Start`/`Stop`/`List`/`StopAll` |
| `session.go` | Per-session `http.Server` + SSE lifecycle (`start`/`stop`/`info`); owns and closes its `*snowflake.Client` |
| `server.go` | `buildServer(client, mode)` — constructs the MCP server and registers tools |
| `tools.go` | Tool input structs + `registerTools`; `jsonResult`/`textResult` content helpers |
| `mcp_test.go` | SSE round-trip test (external client lists tools) + port-allocation test |
| `doc.go` | Package doc + `thaw:domain: MCP Server` annotation |

## Key types & functions

### `Manager`

| Function | Behaviour |
|---|---|
| `NewManager()` | Empty registry. Safe for concurrent use. |
| `Start(label, connLabel, mode, port, client)` | Starts a session under a unique `label`; `port == 0` auto-assigns from `9100`. Takes ownership of `client`. |
| `Stop(label)` | Stops and removes the named session, closing its connection. |
| `List()` | Snapshot of all sessions (`[]SessionInfo`) sorted by label. |
| `StopAll()` | Stops every session; called on app `shutdown` and `Disconnect`. |

Ports auto-assign sequentially from `basePort` (`9100`) up to `basePort+1000`; `portFree` validates the loopback bind before use. An explicit duplicate or unavailable port is rejected.

### Execution mode

`ExecutionModeMetadata` (`"metadata"`) is the only supported mode in the foundation milestone — sessions are read-only metadata browsers.

### Tools (registered in `tools.go`)

`get_session_context`, `list_databases`, `list_schemas`, `list_objects`, `describe_table`, `get_ddl`, `get_table_foreign_keys`. Each delegates to the session's `*snowflake.Client` and returns its payload as indented-JSON text content (`get_ddl` returns raw text).

## Patterns & integration

The `*App` delegators in `internal/app/mcp.go` (`StartMCPSession`, `StopMCPSession`, `ListMCPSessions`, `GetMCPSessionConfig`) open a fresh `*snowflake.Client` from `App.connectParams` and hand it to `Manager.Start`. `StartMCPSession` enforces the admin-lockable `mcpServer` feature flag via the **effective** flags (`App.GetFeatureFlags()`, which applies IT-admin overrides) so an admin lock cannot be bypassed through the native menu. Sessions are **not persisted** — they exist only for the lifetime of the process and are not restored on the next launch. Frontend surface: `MCPSessionsModal.tsx`, `MCPIndicator.tsx`, and `mcpStore.ts`.

A session's SSE endpoint is `http://localhost:<port>/sse`; `GetMCPSessionConfig` formats the standard client config block `{"mcpServers": {"thaw-<label>": {"url": "..."}}}`.

## Security

The SSE endpoint has **no authentication token** — any local process that can reach `localhost:<port>` (default range from `9100`) can call the read-only metadata tools and read schema metadata for the connected account. The Go MCP SDK's SSE handler binds only to the loopback interface and validates the `Host` header against loopback, which mitigates DNS-rebinding attacks from a browser, but it does **not** prevent other local processes on the same machine from connecting. Sessions are read-only (metadata browsing only) and must be started explicitly; stop them when not in use.

## Gotchas

The Go MCP SDK's generic `AddTool[In, Out]` infers an output JSON schema from `Out` and **panics at registration** if that schema's type is not `"object"`. Tools that return arrays, strings, or slices of structs therefore declare `Out` as `any` (the SDK then omits the output schema) and return `nil` structured output, delivering the payload as text content via `jsonResult`/`textResult`. Never give an MCP tool a concrete non-struct `Out` type.
