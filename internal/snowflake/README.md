# internal/snowflake

> Snowflake driver wrapper — connection management, query execution, DDL retrieval, object listing, and data lineage.

## Responsibility

This package is the single point of contact between Thaw and Snowflake. It wraps the `gosnowflake v2` driver with a session-state-aware connection pool, provides typed query helpers, maintains a per-schema object TTL cache, and exposes high-level methods for every operation the rest of the app needs (listing objects, fetching DDL, running queries, switching roles/warehouses, exporting table data, etc.).

No business logic belongs here — callers pass SQL strings or high-level parameters; this package handles connection pooling, result parsing, and caching.

## Key files

| File | Purpose |
|------|---------|
| `client.go` | `Client` struct, `NewClient`, `Execute`, `QuerySingle`, `SplitStatements`, `ListObjects`, object cache, `Use*` methods, DDL fetchers, and all other `*Client` methods |
| `result.go` | Shared result-parsing helpers: `ColIdx`, `CellString/Float/Int64/Bool`, `PropertyPair`, `ResultToPairs` |
| `session.go` | `SessionParam`, `SessionVar`, `GetSessionParameters`, `GetSessionVariables`, `QuoteSessionParamValue` |
| `identifiers.go` | `NeedsQuoting`, `QuoteIdent`, `QuoteStringLit`, `EscapeLikePattern`, `QuoteOrBare`, `ReservedKeywords`, `GetQuotedIdentifiersIgnoreCase` |
| `collations.go` | `CollationOption`, `CollationLocale`, `CollationSpecifier`; `Collations()`, `CollationLocales()`, `CollationSpecifiers()` — single source of truth for the collation registry surfaced in the UI |
| `helpers.go` | `IsBoolean`, `IsNumeric`, `NeedsQuotes` — data-type predicate helpers used by column DDL builders |
| `lineage.go` | `DependencyNode`, `SchemaRef`, `GetObjectDependencies`, `GetSchemaCrossDeps` — recursive DDL-parsing dependency tree (capped at depth 8 by `maxDependencyDepth`) |
| `datatypes.go` | Snowflake data type normalisation and validation |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Key types & functions

### Connection
- `ConnectParams` — all fields for opening a session (account, user, password, authenticator, Okta URL, key-pair path, TOTP passcode, etc.)
- `NewClient(ctx, ConnectParams) (*Client, error)` — opens the pool, pings, resolves the actual server role
- `sessionConnector` — internal `driver.Connector` that applies the stored role/warehouse/database/schema to every newly created pool connection, flushing idle connections when context changes

### Query execution
- `Execute(ctx, sql, onProgress...)` — splits on semicolons, runs statements sequentially on a pinned `*sql.Conn`; single-statement path uses async mode via `sf.WithQueryIDChan`; PUT/GET use a sync context
- `QuerySingle(ctx, sql)` — always synchronous, bypasses multi-statement logic
- `ExecDDL(ctx, sql)` — fire-and-forget DDL (no result set)
- `CancelSnowflakeQuery(ctx, queryID)` — calls `SYSTEM$CANCEL_QUERY`
- `SplitStatements(sql) []string` — exported wrapper around `splitStatements`; handles `--`, `/* */`, `'…'`, `"…"`, `$tag$…$tag$`

### Result types
- `QueryResult{Columns, Rows, RowsAffected, QueryID, Truncated}` — capped at `maxQueryRows = 50_000`
- `SnowflakeObject{Name, Kind, Schema, Arguments, RowCount, Predecessors, Finalize}`

### Result-parsing helpers (result.go)
- `ColIdx(cols, names...)` — finds a column index by name (case-insensitive, multiple alternatives)
- `CellString/CellFloat/CellInt64/CellBool(v any)` — convert raw `interface{}` cells
- `PropertyPair{Key, Value}` / `ResultToPairs(res)` — project first result row as key/value pairs

### Object listing & cache
- `ListObjects(ctx, db, schema)` — `ListBasicObjects` + `ListExtendedObjects`; result is cached per-schema (30 s TTL, key `"DB\x00SCHEMA"`)
- `ListBasicObjects(ctx, db, schema)` — tables, views, sequences from a single SHOW query; cached with key `"basic\x00DB\x00SCHEMA"`
- `getObjectCache(key)` — returns `slices.Clone()` of the cached slice (prevents append corruption)
- `ClearObjectCache()` / `ClearObjectCacheForDatabase(db)` — IPC-exposed cache invalidation
- `ClearObjectCacheForSchema(db, schema)` — internal use only, not exposed as IPC

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
