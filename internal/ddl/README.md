# internal/ddl

> Snowflake DDL per-object metadata extractor and parallel git-export pipeline.

## Responsibility

This package has two jobs:

1. **Parsing** — extract structured metadata (object kind, database, schema, name, argument signature) from each CREATE statement. Statement splitting is handled by `internal/sqlutil`.
2. **Exporting** — drive a concurrent, multi-database DDL export to disk, determining the correct on-disk file path for each object and handling file-path collisions.

A separate `account.go` sub-pipeline handles account-level objects (roles, warehouses) that live outside any database.

## Key files

| File | Purpose |
|------|---------|
| `parser.go` | Helper functions (`isIdentRune`, `runesEqual`) used by tests |
| `object.go` | `Object`, `Kind` constants, `Parse(sql) Object`, `FilePath()`, `FilePathFor(template, db, naming)`, `parseArgSig`/`parseArgSigFull`, `nameTracker` (collision resolver) |
| `naming.go` | `OverloadNaming` strategies (`argtypes` / `signature` / `grouped`), `overloadSuffix`, and `planFiles` — the deterministic object→path planner (collision numbering + overload grouping) |
| `exporter.go` | `ExportDatabases(ctx, dbs, fetch, opts, progress)` — parallel export pipeline; `ExportOptions` (path template, overload naming, object-type/schema filters, skip-existing, dollar-quoted bodies, concurrency), `ExportResult`, `ProgressFunc`, `FetchDDL` |
| `account.go` | `ExportAccountObjects(ctx, client, outputDir)` — exports roles and warehouses to `_account/roles/` and `_account/warehouses/` |
| `doc.go` | Package doc + `thaw:domain` annotation |

## Key types & functions

### Object metadata
```go
type Object struct {
    Kind     Kind    // TABLE, VIEW, FUNCTION, PROCEDURE, SEQUENCE, STAGE, STREAM,
                    // TASK, FILE FORMAT, PIPE, SCHEMA, DATABASE, UNKNOWN
    Database string
    Schema   string
    Name     string
    ArgSig   string // e.g. "FLOAT_VARCHAR" for overloaded functions/procedures
    ArgSigFull string // same, size qualifiers kept: "VARCHAR_256", "NUMBER_38_0"
    SQL      string // full DDL text without trailing semicolon
}
```

`ArgSig` (size-stripped) is also the overload key `internal/migration` diffs on — do not change its shape. `ArgSigFull` exists only for file naming.

`Parse(sql string) Object` — classifies the statement over the `internal/sqltok` significant-token stream: `CREATE`, an optional `OR REPLACE`, any number of modifier keywords (`createModifiers`: TRANSIENT, SECURE, MATERIALIZED, …), then the object-type keyword (`createKinds`, plus the two-word `FILE FORMAT`). The name is read with `sqltok.ReadIdentParts` (up to three dot-separated parts, quoted or unquoted) after an optional `IF NOT EXISTS`.

This replaced an anchored `^\s*create…` regex that could not see through comments, so `-- header\nCREATE TABLE t …` and `CREATE /* mod */ TABLE t …` both fell through to `KindUnknown` — header comments are normal in the user-authored migration scripts that `internal/migration` feeds to `Parse`, and those statements silently dropped out of kind-based handling.

`(o *Object).FilePath() string` — returns the relative path using the default layout:
```
_database.sql
schemas/<SCHEMA>.sql
<SCHEMA>/tables/<TABLE>.sql
<SCHEMA>/functions/<NAME>__<ARGSIG>.sql
…
```

`(o *Object).FilePathFor(template, database string, naming OverloadNaming) string` — same but applies a user-configured path template with placeholders `{database}`, `{schema}`, `{object_type}`, `{object_name}`. `DefaultExportPathTemplate = "{database}/{schema}/{object_type}/{object_name}.sql"`. The result is a *candidate* path — two overloads can still map to the same one; `planFiles` resolves that. The placeholder list is mirrored on the frontend in `frontend/src/components/export/pathTemplate.ts` (insert tags + preview in both export dialogs) — add a placeholder there too when adding one here.

`nameTracker` — mutex-protected collision resolver; first occurrence keeps the plain path, subsequent ones get `_2`, `_3`, … suffixes. Used by `planFiles` as the uniqueness registry.

### Overload naming & file planning (`naming.go`)

`OverloadNaming` picks how overloaded FUNCTION / PROCEDURE definitions land on disk (empty/unknown → `DefaultOverloadNaming`, so no caller validates):

| Value | `FOO(X VARCHAR(16))` + `FOO(X VARCHAR(256))` |
|---|---|
| `OverloadNamingArgTypes` (`"argtypes"`, default) | `FOO__VARCHAR.sql` + `FOO__VARCHAR_2.sql` — size qualifiers dropped, so these two collide |
| `OverloadNamingSignature` (`"signature"`) | `FOO__VARCHAR_16.sql` + `FOO__VARCHAR_256.sql` — qualifiers folded into the name |
| `OverloadNamingGrouped` (`"grouped"`) | one `FOO.sql` holding both `CREATE` statements |

`planFiles(objs, template, database, naming) []filePlan` maps the *whole* object set onto paths — `exportOne` parses and filters everything first, then calls it once. Every result depends only on the set of objects, never on the order Snowflake returned the statements in, so re-exporting produces byte-identical files and Git diffs stay meaningful:

1. Objects are grouped by candidate path; groups are ordered by path.
2. All candidate paths are reserved up front, so a group forced onto a numeric suffix skips names a real object already owns (`FOO_2__VARCHAR.sql` → the collided overload takes `FOO__VARCHAR_2.sql`).
3. Within a group members are ordered by `overloadKey` (full signature, then size-stripped signature, then SQL text): the first keeps the plain path, the rest take the next free `_2`, `_3`, … slot.

Under `grouped`, a group made purely of overloads of one name (`areOverloadsOfOneName`: same kind, database, schema, and name) shares a single file — `filePlan.content()` writes each statement semicolon-terminated, separated by a blank line, in the same signature order. Collisions between *unrelated* objects (a template without `{schema}` flattening two schemas) always fall through to numeric suffixes instead of being merged.

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
- `opts.ObjectTypes []Kind` / `opts.Schemas []string` (both empty = all) filter parsed statements before writing — **post-fetch filters**; `GET_DDL('DATABASE', …)` always returns the whole database. `KindDatabase`/`KindSchema` anchors are always written. Schema entries are matched case-insensitively and may be bare (`"PUBLIC"` — matches in every exported database) or qualified (`"DB1.PUBLIC"` — matches only in that database).
- `opts.SkipExisting` leaves already-existing files untouched (counted in `Skipped` alongside unparsable statements). Paths are planned *before* this check, so name assignment never depends on what happens to be on disk.
- `opts.OverloadNaming` selects the overload layout above (empty = `argtypes`).
- `opts.DollarQuoteBodies` rewrites FUNCTION / PROCEDURE bodies from the single-quoted string literal `GET_DDL` returns into the dollar-quoted (`$$…$$`) form via `snowflake.DollarQuoteBody`. It runs per object right after `Parse`, *before* `planFiles`, so the rewritten text is what the overload sort key (`overloadKey`, which includes `SQL`) and every path decision see. The helper is a no-op for anything it cannot convert faithfully, so the flag never produces a half-rewritten statement.

### Account-level export
`ExportAccountObjects(ctx, client, outputDir)` calls `client.ListRoles`/`client.GetRoleDDL` and `client.ListWarehouses`/`client.GetWarehouseDDL`, writing results under `outputDir/_account/{roles,warehouses}/`.

## Patterns & integration

- `internal/app` (specifically `ExportDatabaseDDL` / `ExportAllDatabasesDDL`) constructs the `FetchDDL` closure using `client.GetCompleteDatabaseDDL` and calls `ExportDatabases`.
- The package has no dependency on `internal/app` or Wails — it is independently unit-testable (`parser_test.go`, `object_test.go`).
- `sanitize(name)` normalises names to `[A-Za-z0-9_-]` for safe use as file/directory components.

## Gotchas

- `Parse` returns `Kind == KindUnknown` for any non-CREATE statement (e.g. comments, grants, USE). Callers must filter on `Kind != KindUnknown` before writing files.
- Overloaded functions/procedures with identical sanitized argument signatures produce the same candidate path from `FilePath()` / `FilePathFor()`. Only `planFiles` resolves that — call it with the full object set rather than resolving paths one statement at a time, or the numeric suffixes become order-dependent again (the bug that made re-exports churn unrelated files).
- `ExportDatabases` writes files with `os.MkdirAll` + `os.WriteFile` in goroutines; disk errors are collected in `ExportResult.Errors`, not returned as a top-level error.
