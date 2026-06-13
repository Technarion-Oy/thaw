# internal/externaltable

> SQL builder for Snowflake EXTERNAL TABLE objects.

## Responsibility

Builds the `CREATE EXTERNAL TABLE` DDL from a structured config. The lifecycle
commands (`REFRESH`, `SET AUTO_REFRESH`, `SET`/`UNSET`, `RENAME TO`) are simple
enough that they are issued as free-form `ALTER EXTERNAL TABLE <fqn> <clause>`
statements directly from `internal/app/externaltable.go`
(`App.AlterExternalTable`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `ExternalTableConfig`, `ExternalTableColumn`, `TagPair`, `BuildCreateExternalTableSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `ExternalTableConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Columns` (`[]ExternalTableColumn`), `Location`, `RefreshOnCreate`, `AutoRefresh`, `Pattern`, `FileFormatName`, `FileFormatType`, `AwsSnsTopic`, `CopyGrants`, comment, `Tags` (`[]TagPair`) |
| `ExternalTableColumn` | One column derived from the staged file: `Name`, `Type`, `Expression` (emitted as `AS (<expr>)`), and `Partition` (whether to include it in `PARTITION BY`) |
| `TagPair` | `Name` / `Value` for the table-level `TAG (...)` clause |
| `BuildCreateExternalTableSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] EXTERNAL TABLE [IF NOT EXISTS] <fqn> [( <cols> )] [PARTITION BY (…)] LOCATION=… [REFRESH_ON_CREATE=…] [AUTO_REFRESH=…] [PATTERN='…'] FILE_FORMAT=(…) [AWS_SNS_TOPIC='…'] [COPY GRANTS] [COMMENT='…'] [TAG (…)];` — optional clauses emitted only when set, in the order Snowflake documents them |

## Patterns & integration

- Every external-table column is derived from the file via an expression, so each
  column renders as `<name> <type> AS (<expr>)`. `PARTITION BY` is derived from
  the columns flagged `Partition` (in declared order); the partition columns must
  also appear in the column list, matching Snowflake's grammar.
- `FILE_FORMAT` prefers a named format (`FORMAT_NAME = '<name>'`) over an inline
  `TYPE`; when a `TYPE` is set it is used directly, and when neither is supplied
  the clause emits a completable `FORMAT_NAME = '<file_format>'` placeholder (the
  UI always supplies a concrete `TYPE` in inline-type mode, so an empty type
  means "named format, not yet chosen").
- Required fields left empty emit obvious placeholders so the live SQL preview
  reads as a completable template — `LOCATION = @<stage>/<path>` and the
  `external_table_name` token.
- `App.BuildCreateExternalTableSql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterExternalTable` (in `internal/app/externaltable.go`)
  runs the lifecycle clauses.
- Discovery: `Client.ListExtendedObjects` runs `SHOW EXTERNAL TABLES IN SCHEMA`
  with the fixed kind `"EXTERNAL TABLE"`. `showInSchema` skips `is_external=Y`
  rows on the generic-kind path, and `dedupeExternalTables` drops any remaining
  `(schema, name)` collision, to avoid duplicate tree entries.
- DDL export: `buildGetDDLQuery` normalizes the kind `"EXTERNAL TABLE"` to the
  `GET_DDL` object type `EXTERNAL_TABLE`.

## Gotchas

The builder does not parse or validate column expressions, the location, or the
pattern — Snowflake reports those errors at execution time. Column-level
constraints, Delta Lake / Iceberg table formats, and policy attachments are out
of scope for the visual builder and are left to raw SQL.
