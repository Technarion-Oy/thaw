# internal/view

> SQL builder for Snowflake VIEW objects.

## Responsibility

Builds the `CREATE VIEW` DDL from a structured config. The simple alterations
(`SET`/`UNSET SECURE`, `SET`/`UNSET COMMENT`, `RENAME TO`) are issued as free-form
`ALTER VIEW <fqn> <clause>` statements directly from `internal/app/view.go`
(`App.AlterView`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `ViewConfig`, `BuildCreateViewSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `ViewConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `Secure`, `Recursive`, `IfNotExists`, `CopyGrants`, comment, `Columns` (optional explicit column list), `Tags` (`[]snowflake.TagPair`), and the defining `Query` |
| `snowflake.TagPair` / `snowflake.TagClause` | Shared tag type and `TAG (...)` clause builder (in `internal/snowflake`) |
| `BuildCreateViewSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] [SECURE] [RECURSIVE] VIEW [IF NOT EXISTS] <fqn> [(cols)] [COPY GRANTS] [COMMENT='…'] [TAG (…)] AS <query>;` — optional clauses emitted only when set, in documented order |

## Patterns & integration

- Required field left empty (`Query`) emits an obvious placeholder so the live
  SQL preview reads as a completable template.
- `App.BuildCreateViewSql` (in `internal/app/builders.go`) is the thin IPC
  delegator; `App.AlterView` (in `internal/app/view.go`) runs the alter clauses.
- The `body` is assembled in the order Snowflake's parser requires:
  `SECURE RECURSIVE VIEW`.

## Gotchas

- The defining `Query` is appended verbatim after `AS`; any trailing semicolons
  are trimmed so the builder controls statement termination.
- `Columns` is a free-form comma-separated list emitted as `(...)` right after the
  view name; the builder does not validate it against the query's projection —
  mismatches surface at execution time.
