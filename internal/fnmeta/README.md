# internal/fnmeta

> Three-tier SQLite cache for Snowflake function metadata powering SQL editor autocomplete and hover tooltips.

## Responsibility

Maintains a local `fn_metadata.db` SQLite database that stores function names, signatures,
descriptions, and types (BUILTIN or UDF). The editor reads from this cache for instant
completions without round-tripping Snowflake on every keystroke. An embedded JSON fallback
ensures built-in function completions work offline, before any connection is established.
A background sync from the live connection adds user-defined functions (UDFs).

## Key files

| File | Purpose |
|---|---|
| `fnmeta.go` | `Store` (SQLite wrapper): `Open`, `Close`, `Search`, `Lookup`, `GetAllNames`, `Upsert`, `LoadFallback`, schema migration via `PRAGMA user_version`. |
| `sync.go` | `SyncFromSnowflake`: fetches UDFs via `SHOW USER FUNCTIONS` and upserts them. `fetchBuiltins` is present but intentionally unused (see Gotchas). |
| `fallback.go` | `//go:embed snowflake_builtin_fallback.json` — offline baseline catalog of Snowflake built-in functions. |

## Key types & functions

| Symbol | Description |
|---|---|
| `FunctionMeta` | `{ functionName, functionSignature, description, functionType }`. |
| `Store` | Wraps `*sql.DB`; opened at a caller-supplied directory path. |
| `Store.Search(prefix)` | Returns up to 50 functions whose name starts with `prefix` (case-insensitive), ordered shortest-first. |
| `Store.Lookup(name)` | Returns all overloads for an exact function name. |
| `Store.GetAllNames()` | Returns every distinct `(name, type)` pair — used to build Monaco syntax-highlight decorations. |
| `Store.Upsert(metas)` | Bulk insert-or-update in a single transaction. |
| `Store.LoadFallback()` | Decodes the embedded JSON and upserts it; idempotent via `ON CONFLICT`. |
| `SyncFromSnowflake(ctx, client, store)` | Fetches live UDFs and upserts them; best-effort (errors silently ignored). |

## Patterns & integration

- `App` opens the `Store` at startup (`internal/app/app.go`) and calls `LoadFallback()` immediately so completions work before connecting.
- `SyncFromSnowflake` is called in a background goroutine after a successful Snowflake connection.
- Uses `modernc.org/sqlite` (pure-Go, no CGO) so the binary remains self-contained.
- Schema migrations are handled via `PRAGMA user_version`; bumping `schemaVersion` drops and recreates the table, triggering a re-seed from the fallback on next `LoadFallback`.

## Gotchas

- Built-in functions are **not** synced from the live `SHOW FUNCTIONS` output. Snowflake returns type-only signatures (e.g. `ABS(NUMBER)`) with no parameter names, which would produce duplicates alongside the embedded catalog that has proper named signatures. Only UDFs (which Snowflake does include parameter names for) are synced live.
- `Store.Search` uses a `LIKE prefix%` query, so the caller should upper-case the prefix to match the stored upper-case `function_name` values.
