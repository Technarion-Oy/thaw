# internal/eventtable

Builds SQL for Snowflake **EVENT TABLE** objects — special schema-level tables
with a fixed, predefined schema that capture telemetry data (logs, traces, and
metrics) emitted by UDFs, stored procedures, and Snowpark Container Services.
Event tables are central to debugging and observability.

## What it does

`BuildCreateEventTableSql(db, schema, cfg)` renders a `CREATE EVENT TABLE`
statement from an `EventTableConfig`. Because event tables have a **fixed column
layout**, the statement carries no column list and no `CLUSTER BY` clause — only
the supported table-level properties:

```
CREATE [OR REPLACE] EVENT TABLE [IF NOT EXISTS] <fqn>
  [DATA_RETENTION_TIME_IN_DAYS = <int>]
  [MAX_DATA_EXTENSION_TIME_IN_DAYS = <int>]
  [CHANGE_TRACKING = { TRUE | FALSE }]
  [DEFAULT_DDL_COLLATION = '<spec>']
  [COPY GRANTS]
  [COMMENT = '<string>']
  [TAG ( <name> = '<value>', ... )];
```

## Types & builders

- `EventTableConfig` — name + case sensitivity, `OrReplace`/`IfNotExists`
  (mutually exclusive), `DataRetentionTimeInDays`, `MaxDataExtensionTimeInDays`,
  `ChangeTracking`, `DefaultDdlCollation`, `CopyGrants`, `Comment`, `Tags`.
- `BuildCreateEventTableSql` — the only exported builder.

## Gotchas

- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive; when both are set the
  builder drops `IF NOT EXISTS`. The SQL editor's `validateCreateEventTable`
  flags the same conflict, plus column definitions, `CLUSTER BY`, and
  `TRANSIENT`, none of which this builder ever emits.
- Event tables share the **standard `TABLE` management commands** — there is no
  `ALTER`/`DROP EVENT TABLE`. Mutations (`COMMENT` `SET`/`UNSET`, retention,
  change tracking, etc.) and `RENAME TO` are issued as free-form `ALTER TABLE`
  statements via `App.AlterEventTable` in `internal/app/eventtable.go`; the
  Sidebar drops them with `DROP TABLE`.
- Event tables are **not** expected to be returned by `SHOW OBJECTS`; they are
  listed via `SHOW EVENT TABLES` in `internal/snowflake` (`ListExtendedObjects`).
  A `dedupeEventTables` pass still runs in `ListObjects` for consistency with the
  other extended table-like kinds — a cheap belt-and-suspenders that drops any
  `(schema, name)` collision should an edition ever surface an event table as a
  plain `TABLE`.
- `GET_DDL` exposes a dedicated `EVENT_TABLE` object type. The
  `"EVENT TABLE"` → `EVENT_TABLE` normalization lives in `internal/snowflake`
  (`buildGetDDLQuery`), not here.
