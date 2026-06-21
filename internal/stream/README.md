# internal/stream

> SQL builder for Snowflake STREAM objects.

## Responsibility

Builds the `CREATE STREAM` DDL from a structured config. A stream tracks change
data (CDC) on a source object — `TABLE`, `VIEW`, `EXTERNAL TABLE`, `STAGE`, or
`DYNAMIC TABLE`. The lifecycle commands (`SET`/`UNSET COMMENT`, `RENAME TO`) are
simple enough that they are issued as free-form `ALTER STREAM <fqn> <clause>`
statements directly from `internal/app/stream.go` (`App.AlterStream`) without a
dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `StreamConfig`, `BuildCreateStreamSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `StreamConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `CopyGrants`, `SourceType`, `Source`, `AppendOnly`, `ShowInitialRows`, `InsertOnly`, comment |
| `BuildCreateStreamSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] STREAM [IF NOT EXISTS] <fqn> [COPY GRANTS] ON <SourceType> <source> [APPEND_ONLY = TRUE] [SHOW_INITIAL_ROWS = TRUE] [INSERT_ONLY = TRUE] [COMMENT='…'];` — optional clauses emitted only when set, in documented order |

## Patterns & integration

- A required field left empty (`Source`) emits an obvious placeholder
  (`<source_object>`) so the live SQL preview reads as a completable template.
- `SourceType` defaults to `TABLE` when empty.
- The source identifier is **qualified only when bare**: a `Source` that already
  contains a `.` is treated as fully qualified and emitted verbatim; a bare name
  is qualified with the active `db`/`schema` via `snowflake.Qualify`.
- `App.AlterStream` (in `internal/app/stream.go`) runs the `SET`/`UNSET COMMENT`
  and `RENAME TO` clauses.

## Gotchas

- `APPEND_ONLY` and `INSERT_ONLY` are mutually constrained by source type
  (`INSERT_ONLY` is for external tables / directory tables on stages); those
  errors surface at execution time, not in the builder.
- `SHOW_INITIAL_ROWS` is only meaningful on creation and is ignored by Snowflake
  unless the stream is append-only on a table; the builder emits it whenever set.
