# internal/pipe

> SQL builders for Snowflake PIPE objects and COPY_HISTORY queries, plus a DDL parser for extracting the COPY INTO target table.

## Responsibility

Owns three distinct concerns that are tied together by the pipe object lifecycle:

1. **SQL builders** — `CREATE PIPE` and `ALTER PIPE ... REFRESH` DDL.
2. **DDL parser** — extracts the `COPY INTO` target table (as fully-qualified identifier parts with quoting metadata) from a pipe's GET_DDL text.
3. **Copy history** — builds and executes the `INFORMATION_SCHEMA.copy_history` table-function query, using the parsed target table name as the required `TABLE_NAME` argument.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `PipeConfig`, `RefreshPipeConfig`, `BuildCreatePipeSql`, `BuildRefreshPipeSql` |
| `parse.go` | `FQNPart`, `ParseCopyIntoTargetParts`, `ParseCopyIntoTarget`, internal identifier parsers |
| `copyhistory.go` | `GetCopyHistory(ctx, client, ...)` — fetches and filters copy history rows |
| `sql_test.go` | Unit tests for the SQL builders |
| `parse_test.go` | Unit tests for the DDL parser |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

### Config types

| Type | Purpose |
|---|---|
| `PipeConfig` | CREATE PIPE parameters: name, `AutoIngest`, `ErrorIntegration`, `AwsSnsTopic`, `Integration`, comment, `CopyStatement` |
| `RefreshPipeConfig` | ALTER PIPE REFRESH parameters: optional `Prefix` path and `ModifiedAfter` ISO timestamp |

### SQL builders

| Function | Emits |
|---|---|
| `BuildCreatePipeSql(db, schema, cfg)` | `CREATE [OR REPLACE] PIPE [IF NOT EXISTS] <fqn> [AUTO_INGEST=TRUE] [...] AS <COPY INTO ...>;` |
| `BuildRefreshPipeSql(db, schema, name, cfg)` | `ALTER PIPE <fqn> REFRESH [PREFIX='...'] [MODIFIED_AFTER='...'];` |

### DDL parser

| Function | Description |
|---|---|
| `FQNPart` | `Value string`, `Quoted bool` — one identifier component from a dot-separated FQN |
| `ParseCopyIntoTargetParts(ddl)` | Returns up to three `FQNPart` values (db, schema, table) parsed from the pipe DDL's `COPY INTO` clause, preserving quoting status |
| `ParseCopyIntoTarget(ddl)` | Convenience wrapper; returns `(db, schema, table string, err error)` without quoting metadata |

### Copy history

| Function | Description |
|---|---|
| `GetCopyHistory(ctx, client, db, schema, name, startTime, status, fileName)` | Fetches pipe DDL → parses COPY INTO target → queries `INFORMATION_SCHEMA.copy_history(TABLE_NAME => ..., START_TIME => ...)` with optional `STATUS`, `FILE_NAME`, and pipe identity filters; returns `*snowflake.QueryResult` |

## Patterns & integration

`*App` in `internal/app/pipe.go`:
- `App.AlterPipe(db, schema, name, clause)` — executes a free-form `ALTER PIPE <fqn> <clause>` directly (no builder needed for simple clauses like `RESUME` / `PAUSE`)
- `App.GetPipeStatus(db, schema, name)` — runs `SYSTEM$PIPE_STATUS('<fqn>')`, returns JSON string
- `App.GetPipeCopyHistory(db, schema, name, startTime, status, fileName)` → `pipe.GetCopyHistory`

`BuildCreatePipeSql` validates the `CopyStatement` via `validateCopyStatement` before embedding it: exactly one statement, must start with `COPY INTO`. An empty `CopyStatement` emits the placeholder `COPY INTO <table> FROM @<stage>` for the live preview.

`ParseCopyIntoTargetParts` preserves the `Quoted` flag per identifier part so that `GetCopyHistory` can uppercase unquoted parts (Snowflake's `GET_DDL` may return them in any case) before passing them to `copy_history`. Quoted identifiers are left as-is to preserve case-sensitive names.

## Gotchas

`GetCopyHistory` fetches the pipe DDL via `client.GetObjectDDL` on every call — there is no caching. This adds one extra round-trip per copy-history request, which is necessary because the `copy_history` table function requires the target table name rather than the pipe name.

`copy_history` filters by `pipe_catalog_name`, `pipe_schema_name`, and `pipe_name` separately (not a single FQN column) because Snowflake does not expose a combined pipe FQN column in that function's output.

`validateCopyStatement` uses `sqlutil.Split` to detect multi-statement input and rejects it with an error, preventing users from accidentally embedding multiple statements in the pipe definition.
