# internal/snowflake

> Snowflake driver wrapper — connection management, query execution, DDL retrieval, object listing, and data lineage.

## Responsibility

This package is the single point of contact between Thaw and Snowflake. It wraps the `gosnowflake v2` driver with a session-state-aware connection pool, provides typed query helpers, maintains a per-schema object TTL cache, and exposes high-level methods for every operation the rest of the app needs (listing objects, fetching DDL, running queries, switching roles/warehouses, exporting table data, etc.).

No business logic belongs here — callers pass SQL strings or high-level parameters; this package handles connection pooling, result parsing, and caching.

## Key files

| File | Purpose |
|------|---------|
| `client.go` | `Client` struct, `NewClient`, `Execute`, `QuerySingle`, `ListObjects`, object cache, `Use*` methods, DDL fetchers, and all other `*Client` methods |
| `result.go` | Shared result-parsing helpers: `ColIdx`, `CellString/Float/Int64/Bool`, `PropertyPair`, `ResultToPairs` |
| `session.go` | `SessionParam`, `SessionVar`, `GetSessionParameters`, `GetSessionVariables`, `QuoteSessionParamValue` |
| `identifiers.go` | `NeedsQuoting`, `QuoteIdent`, `QuoteStringLit`, `EscapeStringLit`, `EscapeLikePattern`, `QuoteOrBare`, `ReservedKeywords`, `GetQuotedIdentifiersIgnoreCase` |
| `tags.go` | `TagPair` (shared `{Name, Value}` tag DTO) and `TagClause` — the single `TAG (name = 'value', ...)` clause builder reused by every object CREATE builder that supports tags (dynamic/external tables, materialized views, alerts, git repositories). Callers whose grammar uses `WITH TAG (...)` prepend `WITH ` to a non-empty result. |
| `collations.go` | `CollationOption`, `CollationLocale`, `CollationSpecifier`; `Collations()`, `CollationLocales()`, `CollationSpecifiers()` — single source of truth for the collation registry surfaced in the UI |
| `helpers.go` | `IsBoolean`, `IsNumeric`, `NeedsQuotes` — data-type predicate helpers used by column DDL builders |
| `lineage.go` | `DependencyNode`, `SchemaRef`, `GetObjectDependencies`, `GetSchemaCrossDeps` — recursive DDL-parsing dependency tree (capped at depth 8 by `maxDependencyDepth`) |
| `explain.go` | `ExplainFormat`, `Explain`, `ExplainOnConn` — format-parameterised EXPLAIN execution helpers |
| `datatypes.go` | Snowflake data type normalisation and validation |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Key types & functions

### Connection
- `ConnectParams` — all fields for opening a session (account, user, password, authenticator, Okta URL, key-pair path, TOTP passcode, OAuth/PAT token + token-file path, OAuth2 client ID/secret/URLs/scope, Workload Identity Federation provider/Entra-resource/impersonation-path, forward-proxy host/port/user/password/protocol + no-proxy list, etc.)
- `NewClient(ctx, ConnectParams) (*Client, error)` — opens the pool, pings, resolves the actual server role. Maps the `authenticator` string to a gosnowflake `AuthType` (`snowflake`, `username_password_mfa`, `externalbrowser`, `okta`, `snowflake_jwt`, `oauth`, `programmatic_access_token`, `oauth_authorization_code`, `oauth_client_credentials`, `workload_identity`), applies a 3-minute login timeout for the interactive flows (MFA, browser, Okta, OAuth authorization-code), and rejects invalid combinations (Token + TokenFilePath together; Azure WIF + impersonation path) before the handshake. When `ProxyHost` is set it forwards the discrete proxy fields onto `sf.Config` (the driver defaults `ProxyProtocol` to `http` and honors `HTTP_PROXY`/`HTTPS_PROXY`/`NO_PROXY` as a fallback when no explicit proxy is configured)
- `sessionConnector` — internal `driver.Connector` that applies the stored role/warehouse/database/schema to every newly created pool connection, flushing idle connections when context changes

### Query execution
- `Execute(ctx, sql, onProgress...)` — uses `sqlutil.Split` to split on semicolons, applies `normalizePutGet` to each statement, runs them sequentially on a pinned `*sql.Conn`; single-statement path uses async mode via `sf.WithQueryIDChan`; PUT/GET use a sync context
- `QuerySingle(ctx, sql)` — always synchronous, bypasses multi-statement logic
- `ExecDDL(ctx, sql)` — fire-and-forget DDL (no result set)
- `CancelSnowflakeQuery(ctx, queryID)` — calls `SYSTEM$CANCEL_QUERY`
- `ExplainFormat` (`string`) — `ExplainJSON` or `ExplainTabular`
- `Explain(ctx, query, format)` — runs `EXPLAIN USING <format> <query>` via `QuerySingle`; validates format before SQL construction
- `ExplainOnConn(ctx, conn, query, format)` — same on a pinned `*sql.Conn` via `queryOnConn` (no session sync); validates format

### Result types
- `QueryResult{Columns, Rows, RowsAffected, QueryID, Truncated}` — capped at `maxQueryRows = 50_000`
- `SnowflakeObject{Name, Kind, Schema, Arguments, RowCount, Predecessors, Finalize}`

### Result-parsing helpers (result.go)
- `ColIdx(cols, names...)` — finds a column index by name (case-insensitive, multiple alternatives)
- `CellString/CellFloat/CellInt64/CellBool(v any)` — convert raw `interface{}` cells
- `PropertyPair{Key, Value}` / `ResultToPairs(res)` — project first result row as key/value pairs

### Object listing & cache
- `ListObjects(ctx, db, schema)` — `ListBasicObjects` + `ListExtendedObjects`; result is cached per-schema (30 s TTL, key `"DB\x00SCHEMA"`)
- `ListBasicObjects(ctx, db, schema)` — tables, views, sequences from a single `SHOW OBJECTS` query; cached with key `"basic\x00DB\x00SCHEMA"`. Rows with `is_dynamic=Y` (or `is_external=Y`) are skipped here — dynamic tables and external tables are surfaced by `ListExtendedObjects` instead (kinds `"DYNAMIC TABLE"` / `"EXTERNAL TABLE"`) to avoid duplicate tree entries; `dedupeDynamicTables`/`dedupeExternalTables`/`dedupeMaterializedViews` in `ListObjects` are the belt-and-suspenders fallback (drop by `(schema, name)`) for editions where `SHOW OBJECTS` still surfaces them. Materialized views have no `is_*` skip column, so the dedupe-by-name pass is their sole guard
- `ListExtendedObjects(ctx, db, schema)` — runs the per-type SHOW commands not covered by `SHOW OBJECTS` (DYNAMIC TABLE, EXTERNAL TABLE, MATERIALIZED VIEW, ALERT, TAG, MASKING POLICY, ROW ACCESS POLICY, NETWORK RULE, IMAGE REPOSITORY, SERVICE, PROCEDURE, FUNCTION, TASK, STREAM, STAGE, FILE FORMAT, PIPE, NOTEBOOK, SECRET, GIT REPOSITORY, DBT PROJECT) concurrently; failures per-type are silently skipped. Alerts, tags, masking policies, row access policies, network rules, image repositories, and services are not surfaced by `SHOW OBJECTS`, so — unlike dynamic / external tables and materialized views — they need no dedupe pass. Masking and row access policies map to the `GET_DDL` object type `POLICY` (which covers all policy kinds), so `buildGetDDLQuery` normalizes the SHOW kinds `"MASKING POLICY"` and `"ROW ACCESS POLICY"` → `POLICY`; likewise `"NETWORK RULE"` → `NETWORK_RULE`. Image repositories and services are **not** supported by `GET_DDL`, so there is no `buildGetDDLQuery` mapping (and no DDL export) for them
- `getObjectCache(key)` — returns `slices.Clone()` of the cached slice (prevents append corruption)
- `ClearObjectCache()` / `ClearObjectCacheForDatabase(db)` — IPC-exposed cache invalidation
- `ClearObjectCacheForSchema(db, schema)` — internal use only, not exposed as IPC
- `ListStages(ctx, db, schema)` — `SHOW STAGES IN SCHEMA` → `[]StageSummary{Name, Type, URL}`; the `Type` column distinguishes `INTERNAL`/`EXTERNAL` so callers can filter (e.g. external tables may only reference an `EXTERNAL` stage)
- `ListStageEntries(ctx, db, schema, stage, dirPath)` — directory-aware listing via `LIST @stage/dirPath` (internal or external stages)

### Session management
- `UseRole/UseWarehouse/UseDatabase/UseSchema` — execute the USE statement then call `refreshConnectorState`, which flushes idle connections on role/warehouse/database changes
- `GetSessionContext` / `GetCachedSessionContext` — live and in-memory snapshots of `{Role, Warehouse, Database, Schema}`
- `SetPoolLimits(maxOpen, maxIdle)` — tab sessions use smaller limits (e.g. 4/1) vs. shared client default of 8/8

## Patterns & integration

- Domain packages (`warehouse`, `backup`, `table`, etc.) receive a `*Client` and call `Execute`/`ExecDDL`; they never construct the pool themselves.
- `internal/app` holds a `*Client` as the "shared" connection; tab sessions are separate `*Client` instances managed by `internal/app/app.go`.
- `internal/mcp` creates its own dedicated `*Client` per MCP session (mirrors tab-session isolation).
- Result-parsing helpers in `result.go` are used by every domain package that parses SHOW/DESCRIBE output.

## Gotchas

- **gosnowflake logs ALL query errors at ERROR level** even when the caller catches them. Never call `GetObjectDDL` speculatively with a guessed object kind — always resolve the kind first to avoid noisy error logs.
- **`sf.WithQueryIDChan`** — the driver writes the query ID to the channel _and then closes it_. Never call `close(qidChan)` manually; that panics. Drain with `case qid := <-ch:`.
- **Multi-statement `Execute`** uses an inner `execCtx` (`context.WithCancel(context.Background())`) so async-mode flags from the outer context don't apply. The outer `qidChan` never fires for multi-statement scripts; per-statement IDs are delivered through fresh per-statement channels.
- **PUT/GET commands** are incompatible with async mode. `Execute` detects them and substitutes a plain synchronous context before calling `QuerySingle`.
- **`ServerSessionKeepAlive: true`** keeps each pool connection's Snowflake session alive for up to 4 hours, which is essential for multi-statement scripts but counts against the Snowflake session quota.
