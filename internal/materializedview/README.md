# internal/materializedview

> SQL builder for Snowflake MATERIALIZED VIEW objects.

## Responsibility

Builds the `CREATE MATERIALIZED VIEW` DDL from a structured config. The lifecycle
commands (`SUSPEND`, `RESUME`, `SUSPEND/RESUME RECLUSTER`, `CLUSTER BY`, `DROP
CLUSTERING KEY`, `SET`/`UNSET`, `RENAME TO`) are simple enough that they are
issued as free-form `ALTER MATERIALIZED VIEW <fqn> <clause>` statements directly
from `internal/app/materializedview.go` (`App.AlterMaterializedView`) without a
dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `MaterializedViewConfig`, `BuildCreateMaterializedViewSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `MaterializedViewConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `Secure`, `IfNotExists`, `CopyGrants`, comment, `ClusterBy`, `Tags` (`[]snowflake.TagPair`), and the defining `Query` |
| `snowflake.TagPair` / `snowflake.TagClause` | Shared tag type and `TAG (...)` clause builder (in `internal/snowflake`) |
| `BuildCreateMaterializedViewSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] [SECURE] MATERIALIZED VIEW [IF NOT EXISTS] <fqn> [COPY GRANTS] [COMMENT='…'] [CLUSTER BY (…)] [TAG (…)] AS <query>;` — optional clauses emitted only when set, in documented order |

## Patterns & integration

- Required field left empty (`Query`) emits an obvious placeholder so the live
  SQL preview reads as a completable template.
- `App.BuildCreateMaterializedViewSql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterMaterializedView` (in `internal/app/materializedview.go`)
  runs the lifecycle clauses.
- Discovery: `Client.ListExtendedObjects` runs `SHOW MATERIALIZED VIEWS IN SCHEMA`
  with the fixed kind `"MATERIALIZED VIEW"`. Materialized views can also surface in
  `SHOW OBJECTS`, so `dedupeMaterializedViews` in `ListObjects` drops any
  `(schema, name)` collision to avoid duplicate tree entries.
- DDL export: materialized views are retrieved via the `GET_DDL` object type
  `VIEW` (Snowflake has no separate `MATERIALIZED_VIEW` type — `TABLE` and `VIEW`
  are interchangeable in `GET_DDL`). `buildGetDDLQuery` normalizes the kind
  `"MATERIALIZED VIEW"` to `VIEW`.

## Gotchas

- Materialized views have **no manual `REFRESH`** command — Snowflake maintains
  them automatically. `SUSPEND` halts use and maintenance; `RESUME` restores it.
- The defining `Query` is appended verbatim after `AS`; any trailing semicolons
  are trimmed so the builder controls statement termination. Snowflake forbids
  `ORDER BY` / `HAVING` and most joins/aggregations in a materialized view's
  query — those errors surface at execution time, not in the builder.
