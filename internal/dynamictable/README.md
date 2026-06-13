# internal/dynamictable

> SQL builder for Snowflake DYNAMIC TABLE objects.

## Responsibility

Builds the `CREATE DYNAMIC TABLE` DDL from a structured config. The lifecycle
commands (`SUSPEND`, `RESUME`, `REFRESH`, `SET`/`UNSET`, `RENAME TO`) are simple
enough that they are issued as free-form `ALTER DYNAMIC TABLE <fqn> <clause>`
statements directly from `internal/app/dynamictable.go` (`App.AlterDynamicTable`)
without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `DynamicTableConfig`, `BuildCreateDynamicTableSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `DynamicTableConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Transient`, `TargetLag`, `Scheduler`, `Warehouse`, `InitializationWarehouse`, `RefreshMode`, `Initialize`, `ClusterBy`, `DataRetentionTimeInDays`, `MaxDataExtensionTimeInDays`, comment, `CopyGrants`, `RequireUser`, `RowTimestamp`, `Tags` (`[]snowflake.TagPair`), and the defining `Query` |
| `snowflake.TagPair` / `snowflake.TagClause` | Shared tag type and `TAG (...)` clause builder (in `internal/snowflake`) — used by every object builder that supports tags |
| `BuildCreateDynamicTableSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] [TRANSIENT] DYNAMIC TABLE [IF NOT EXISTS] <fqn> TARGET_LAG=… [SCHEDULER=…] WAREHOUSE=… [INITIALIZATION_WAREHOUSE=…] [REFRESH_MODE=…] [INITIALIZE=…] [CLUSTER BY (…)] [DATA_RETENTION_TIME_IN_DAYS=…] [MAX_DATA_EXTENSION_TIME_IN_DAYS=…] [COMMENT='…'] [COPY GRANTS] [TAG (…)] [REQUIRE USER] [ROW_TIMESTAMP=…] AS <query>;` — optional clauses emitted only when set, in documented order |

## Patterns & integration

- `TARGET_LAG` is rendered as a string literal (e.g. `'1 minute'`) **except** the
  `DOWNSTREAM` keyword, which must be emitted bare. `targetLagClause` handles this.
- Required fields left empty (`TargetLag`, `Warehouse`, `Query`) emit obvious
  placeholders so the live SQL preview reads as a completable template.
- `App.BuildCreateDynamicTableSql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterDynamicTable` (in `internal/app/dynamictable.go`) runs
  the lifecycle clauses.
- Discovery: `Client.ListExtendedObjects` runs `SHOW DYNAMIC TABLES IN SCHEMA`
  with the fixed kind `"DYNAMIC TABLE"`. Because dynamic tables also surface in
  `SHOW OBJECTS`/`SHOW TABLES` with `is_dynamic=Y`, `showInSchema` skips those
  rows on the generic-kind path to avoid duplicate tree entries.
- DDL export: `buildGetDDLQuery` normalizes the kind `"DYNAMIC TABLE"` to the
  `GET_DDL` object type `DYNAMIC_TABLE`.

## Gotchas

The defining `Query` is appended verbatim after `AS`; any trailing semicolons are
trimmed so the builder controls statement termination. The builder does not parse
or validate the query — Snowflake reports defining-query errors at execution time.
