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
| `StreamConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `CopyGrants`, `SourceType`, `Source`, `TimeTravelMode`/`TimeTravelKind`/`TimeTravelValue`, `AppendOnly`, `ShowInitialRows`, `InsertOnly`, comment |
| `BuildCreateStreamSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] STREAM [IF NOT EXISTS] <fqn> [COPY GRANTS] ON <SourceType> <source> [{AT\|BEFORE} (<kind> => <value>)] [APPEND_ONLY = TRUE] [SHOW_INITIAL_ROWS = TRUE] [INSERT_ONLY = TRUE] [COMMENT='…'];` — optional clauses emitted only when set, in documented order |

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

- Each source type has its own grammar (see Snowflake's CREATE STREAM reference).
  The builder gates the optional clauses by `SourceType` — a clause is dropped for
  a source type that rejects it, even if the config sets it:
  - **`AT`/`BEFORE` Time Travel** — `TABLE`, `EXTERNAL TABLE`, `VIEW`.
  - **`APPEND_ONLY` / `SHOW_INITIAL_ROWS`** — `TABLE`, `VIEW` only (**not**
    `DYNAMIC TABLE`, despite it also being a row-change source).
  - **`INSERT_ONLY`** — `EXTERNAL TABLE` only (where Snowflake actually requires it).
  - **`STAGE` / `DYNAMIC TABLE`** — take none of the above, only `COPY GRANTS` /
    `COMMENT`.
- Time Travel value quoting is kind-specific: `TIMESTAMP` (a SQL expression) and
  `OFFSET` (a signed number) are emitted verbatim; `STATEMENT` (a query id) and
  `STREAM` (a stream name) are quoted as string literals.
- `SHOW_INITIAL_ROWS` is only meaningful on creation and is ignored by Snowflake
  unless the stream is append-only on a table; the builder emits it whenever set.
