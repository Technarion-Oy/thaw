# internal/ddl

> Snowflake DDL splitter, per-object metadata extractor, and parallel git-export pipeline.

## Responsibility

This package has two jobs:

1. **Parsing** — split a raw DDL string (as returned by `GET_DDL`) into individual SQL statements and extract structured metadata (object kind, database, schema, name, argument signature) from each CREATE statement.
2. **Exporting** — drive a concurrent, multi-database DDL export to disk, determining the correct on-disk file path for each object and handling file-path collisions.

A separate `account.go` sub-pipeline handles account-level objects (roles, warehouses) that live outside any database.

## Key files

| File | Purpose |
|------|---------|
| `parser.go` | `Split(src) []string` — byte-level SQL statement splitter; handles all Snowflake quoting and comment styles |
| `object.go` | `Object`, `Kind` constants, `Parse(sql) Object`, `FilePath()`, `FilePathFor(template, db)`, `nameTracker` (collision resolver) |
| `exporter.go` | `ExportDatabases(ctx, dbs, fetch, opts, progress)` — parallel export pipeline; `ExportOptions`, `ExportResult`, `ProgressFunc`, `FetchDDL` |
| `account.go` | `ExportAccountObjects(ctx, client, outputDir)` — exports roles and warehouses to `_account/roles/` and `_account/warehouses/` |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Key types & functions

### Splitting
`Split(src string) []string` in `parser.go` tokenises at the byte level and flushes on each unquoted `;`. It correctly skips semicolons inside:
- line comments (`-- … \n`)
- block comments (`/* … */`)
- single-quoted strings (`'…'` with `''` escaping)
- double-quoted identifiers (`"…"` with `""` escaping)
- dollar-quoted bodies (`$$…$$` or `$tag$…$tag$`)

SIMD-accelerated `strings.Index`/`strings.IndexByte` make large procedure bodies very fast to skip.

### Object metadata
```go
type Object struct {
    Kind     Kind    // TABLE, VIEW, FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM,
                    // TASK, FILE FORMAT, PIPE, SCHEMA, DATABASE, UNKNOWN
    Database string
    Schema   string
    Name     string
    ArgSig   string // e.g. "FLOAT_VARCHAR" for overloaded functions/procedures
    SQL      string // full DDL text without trailing semicolon
}
```

`Parse(sql string) Object` — regex `createRE` matches the CREATE preamble, then `extractIdent` tokenises the qualified name (up to three dot-separated parts) handling both quoted and unquoted identifiers.

`(o *Object).FilePath() string` — returns the relative path using the default layout:
```
_database.sql
schemas/<SCHEMA>.sql
<SCHEMA>/tables/<TABLE>.sql
<SCHEMA>/functions/<NAME>__<ARGSIG>.sql
…
```

`(o *Object).FilePathFor(template, database string) string` — same but applies a user-configured path template with placeholders `{database}`, `{schema}`, `{object_type}`, `{object_name}`. `DefaultExportPathTemplate = "{database}/{schema}/{object_type}/{object_name}.sql"`.

`nameTracker` — mutex-protected collision resolver; first occurrence keeps the plain path, subsequent ones get `_2`, `_3`, … suffixes.

### Export pipeline
```go
func ExportDatabases(
    ctx context.Context,
    databases []string,
    fetch FetchDDL,         // func(ctx, database) (ddlString, error)
    opts ExportOptions,
    progress ProgressFunc,  // called goroutine-safely after each DB
) []ExportResult
```

- Up to `opts.DBConcurrency` (default `min(16, NumCPU*4)`) databases are fetched from Snowflake in parallel via a channel semaphore.
- For each database, up to `opts.FileConcurrency` (default `NumCPU*4`) goroutines write `.sql` files in parallel.
- `ExportResult{Database, Files, Skipped, Errors}` is returned per database and reported to `progress`.

### Account-level export
`ExportAccountObjects(ctx, client, outputDir)` calls `client.ListRoles`/`client.GetRoleDDL` and `client.ListWarehouses`/`client.GetWarehouseDDL`, writing results under `outputDir/_account/{roles,warehouses}/`.

## Patterns & integration

- `internal/app` (specifically `ExportDatabaseDDL` / `ExportAllDatabasesDDL`) constructs the `FetchDDL` closure using `client.GetCompleteDatabaseDDL` and calls `ExportDatabases`.
- The package has no dependency on `internal/app` or Wails — it is independently unit-testable (`parser_test.go`, `object_test.go`).
- `sanitize(name)` normalises names to `[A-Za-z0-9_-]` for safe use as file/directory components.

## Gotchas

- `Parse` returns `Kind == KindUnknown` for any non-CREATE statement (e.g. comments, grants, USE). Callers must filter on `Kind != KindUnknown` before writing files.
- Overloaded functions/procedures with identical sanitized argument signatures produce the same `FilePath()`; `nameTracker` resolves this but relies on deterministic call order — callers should process statements in the order they appear in the DDL string.
- `ExportDatabases` writes files with `os.MkdirAll` + `os.WriteFile` in goroutines; disk errors are collected in `ExportResult.Errors`, not returned as a top-level error.
