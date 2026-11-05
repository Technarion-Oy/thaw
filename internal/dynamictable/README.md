# internal/dynamictable

> SQL builder for Snowflake DYNAMIC TABLE objects.

## Responsibility

Builds the `CREATE DYNAMIC TABLE` DDL from a structured config. The lifecycle
commands (`SUSPEND`, `RESUME`, `REFRESH [COPY SESSION]`, `SWAP WITH`, clustering,
search optimization, `SET`/`UNSET` properties, policy attach/detach, column
`ALTER COLUMN …` actions, `RENAME TO`) are issued as free-form
`ALTER DYNAMIC TABLE <fqn> <clause>` statements through `App.AlterDynamicTable`
(`internal/app/dynamictable.go`). The clause text is built on the frontend in
`frontend/src/components/dynamictable/dynamicTableClauses.ts` (validated
identifiers, unit-tested), so no Go builder or extra IPC is needed.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `DynamicTableConfig`, `BuildCreateDynamicTableSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `history.go` | `RefreshHistoryRow`, `BuildRefreshHistorySql`, `GetDynamicTableRefreshHistory` |
| `history_test.go` | Unit tests for the refresh-history query and row parser |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `DynamicTableConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Transient`, `TargetLag`, `Scheduler`, `Warehouse`, `InitializationWarehouse`, `RefreshMode`, `Initialize`, `ClusterBy`, `DataRetentionTimeInDays`, `MaxDataExtensionTimeInDays`, comment, `CopyGrants`, `RequireUser`, `RowTimestamp`, `Tags` (`[]snowflake.TagPair`), and the defining `Query` |
| `snowflake.TagPair` / `snowflake.TagClause` | Shared tag type and `TAG (...)` clause builder (in `internal/snowflake`) — used by every object builder that supports tags |
| `BuildCreateDynamicTableSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] [TRANSIENT] DYNAMIC TABLE [IF NOT EXISTS] <fqn> TARGET_LAG=… [SCHEDULER=…] WAREHOUSE=… [INITIALIZATION_WAREHOUSE=…] [REFRESH_MODE=…] [INITIALIZE=…] [CLUSTER BY (…)] [DATA_RETENTION_TIME_IN_DAYS=…] [MAX_DATA_EXTENSION_TIME_IN_DAYS=…] [COMMENT='…'] [COPY GRANTS] [TAG (…)] [REQUIRE USER] [ROW_TIMESTAMP=…] AS <query>;` — optional clauses emitted only when set, in documented order |

| `RefreshHistoryRow` | One `DYNAMIC_TABLE_REFRESH_HISTORY` row: state, state code/message, refresh action/trigger, data timestamp, refresh start/end, rows inserted/deleted, target lag (s), query ID — all strings (RFC3339 timestamps, `""` for NULL) |
| `BuildRefreshHistorySql(db, schema, name, days)` | `SELECT … FROM TABLE("db".INFORMATION_SCHEMA.DYNAMIC_TABLE_REFRESH_HISTORY(NAME => '"db"."schema"."name"', DATA_TIMESTAMP_START => DATEADD('day', -days, …), RESULT_LIMIT => 10000)) ORDER BY DATA_TIMESTAMP DESC`; `days` clamped to 1..7 |
| `GetDynamicTableRefreshHistory(ctx, client, db, schema, name, days)` | Runs the query and parses rows by column name |

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

- `DYNAMIC_TABLE_REFRESH_HISTORY` only retains refreshes whose `DATA_TIMESTAMP` is within
  **7 days** (not 14), hence the clamp. Inserted/deleted row counts are not top-level columns —
  they live in the `STATISTICS` object and are flattened in the SELECT
  (`STATISTICS:numInsertedRows::NUMBER AS NUMROWS_INSERTED`); failed refreshes leave them NULL.
- The function needs MONITOR (or OPERATE / ownership) on the table; the Snowflake error is
  returned unchanged for the UI to show.

The defining `Query` is appended verbatim after `AS`; any trailing semicolons are
trimmed so the builder controls statement termination. The builder does not parse
or validate the query — Snowflake reports defining-query errors at execution time.
