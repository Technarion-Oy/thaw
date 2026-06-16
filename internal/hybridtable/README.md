# internal/hybridtable

Builds SQL for Snowflake **HYBRID TABLE** objects — schema-level tables backing
Unistore / HTAP workloads. Hybrid tables serve low-latency single-row operations
(point lookups, DML) alongside analytical queries, enforce a `PRIMARY KEY`, and
support secondary indexes.

## What it does

`BuildCreateHybridTableSql(db, schema, cfg)` renders a `CREATE HYBRID TABLE`
statement from a `HybridTableConfig`:

```sql
CREATE HYBRID TABLE [IF NOT EXISTS] <fqn> (
    <col> <type> [NOT NULL] [DEFAULT <expr>],
    ...,
    PRIMARY KEY (<col> [, ...]),
    [INDEX <name> (<col> [, ...]) [INCLUDE (<col> [, ...])], ...]
  )
  [COMMENT = '<string>'];
```

Every hybrid table **must** declare a primary key, so the builder always emits a
`PRIMARY KEY (...)` clause derived from the columns flagged `PrimaryKey`
(single or composite). When no column is flagged, a `PRIMARY KEY (<column>)`
placeholder is emitted so the live preview reads as a template rather than
invalid SQL.

`BuildCreateIndexSql` / `BuildDropIndexSql` render the `CREATE INDEX` /
`DROP INDEX` statements used to add or remove secondary indexes **after** the
table exists (the properties panel calls these via `App.CreateHybridTableIndex`
/ `App.DropHybridTableIndex`).

`index.go` holds the column-eligibility rules (`IsIndexableType` /
`IsIncludableType`, built on `snowflake.BaseType`) and `EligibleIndexColumns`,
which partitions a `[]IndexColumn` into the names valid as index key columns vs.
`INCLUDE` columns. The index editors call this via `App.HybridIndexColumnOptions`
so the dropdown filtering shares one source of truth with the builder.

## Types & builders

- `HybridTableConfig` — name + case sensitivity, `IfNotExists`, `Columns`,
  `Indexes`, `Comment`. (No `OrReplace` — hybrid tables don't support it.)
- `HybridColumn` — `<name> <type>` plus `NotNull`, `PrimaryKey`, and `Default`.
  Columns flagged `PrimaryKey` are collected into one out-of-line `PRIMARY KEY`
  and forced to `NOT NULL` (Snowflake requires PK columns to be `NOT NULL`).
- `HybridIndex` — `Name`, `Columns` (key columns), `Include` (non-key INCLUDE
  columns). Used both inline (in `CREATE HYBRID TABLE`) and by `BuildCreateIndexSql`.
- `IndexColumn` / `IndexColumnOptions` — input/output of `EligibleIndexColumns`.
- `BuildCreateHybridTableSql`, `BuildCreateIndexSql`, `BuildDropIndexSql`.
- `IsIndexableType`, `IsIncludableType`, `EligibleIndexColumns` (`index.go`).

Identifier-list quoting reuses the shared `snowflake.QuoteIdentList` helper
rather than a package-local copy.

## Gotchas

- Hybrid tables do **not** support `OR REPLACE`, `TRANSIENT`, `CLUSTER BY`,
  `DATA_RETENTION_TIME_IN_DAYS`, `CHANGE_TRACKING`, or `COPY GRANTS` — the
  builder offers only `IF NOT EXISTS`. (The SQL editor's `validateCreateHybridTable`
  in `internal/sqleditor/patterns.go` flags all of these, so emitting any would
  surface a diagnostic.)
- The primary key is **always** emitted (placeholder when none is selected) — a
  hybrid table cannot be created without one — and every PK column is forced to
  `NOT NULL`.
- There is **no** `ALTER HYBRID TABLE` / `DROP HYBRID TABLE` statement — hybrid
  tables are altered, renamed, and dropped through the plain `TABLE` grammar.
  `COMMENT` `SET`/`UNSET` and `RENAME TO` are issued as free-form `ALTER TABLE`
  via `App.AlterHybridTable` in `internal/app/hybridtable.go`; the Sidebar drops
  them with `DROP TABLE`.
- `GET_DDL` has **no** `HYBRID_TABLE` object type — hybrid tables are retrieved
  via the `'TABLE'` type. The `"HYBRID TABLE"` → `TABLE` normalization lives in
  `internal/snowflake` (`buildGetDDLQuery`), not here.
- `DROP INDEX` addresses the index dot-qualified, **table first**:
  `DROP INDEX [IF EXISTS] <db>.<schema>.<table>.<index>` (not an `ON` clause).
- Hybrid tables surface in `SHOW OBJECTS` / `SHOW TABLES` (with `is_hybrid = Y`);
  they are listed separately via `SHOW HYBRID TABLES` and de-duplicated in
  `internal/snowflake` (`dedupeHybridTables`).
