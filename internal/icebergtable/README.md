# internal/icebergtable

Builds SQL for Snowflake **ICEBERG TABLE** objects — schema-level tables stored
in the open Apache Iceberg table format on an external volume (cloud object
storage), interoperable with engines such as Spark and Trino.

## What it does

`BuildCreateIcebergTableSql(db, schema, cfg)` renders a `CREATE ICEBERG TABLE`
statement from an `IcebergTableConfig`. `CREATE ICEBERG TABLE` has several
variants whose **required attributes differ**; `cfg.TableType` selects which one
is emitted (only that variant's clauses are produced). `EXTERNAL_VOLUME` and
`CATALOG` are optional for every variant (a schema/database/account default is
used when omitted).

| `TableType` | Required | Optional extras | Columns |
|---|---|---|---|
| `snowflake` (default) | `BASE_LOCATION` (+ columns) | `CLUSTER BY` | declared |
| `external_catalog` (Iceberg REST / AWS Glue) | `CATALOG_TABLE_NAME` | `CATALOG_NAMESPACE`, `AUTO_REFRESH`, `REPLACE_INVALID_CHARACTERS` | inferred |
| `delta` (Delta Lake files) | `BASE_LOCATION` | `AUTO_REFRESH`, `REPLACE_INVALID_CHARACTERS` | inferred |
| `iceberg_files` (object storage) | `METADATA_FILE_PATH` | `REPLACE_INVALID_CHARACTERS` | inferred |

For `snowflake`, `CATALOG = 'SNOWFLAKE'` is always emitted. The `external_catalog`
variant covers both Iceberg REST and AWS Glue — their table DDL is identical (the
difference is the catalog integration type). `delta` requires a catalog
integration configured with `CATALOG_SOURCE = OBJECT_STORE` and
`TABLE_FORMAT = DELTA`. `iceberg_files` is the only variant **without**
`AUTO_REFRESH`.

## Types & builders

- `IcebergTableConfig` — name + case sensitivity, `OrReplace`/`IfNotExists`
  (mutually exclusive), `TableType`, `Columns` (Snowflake-managed),
  `ExternalVolume`, `Catalog`, `BaseLocation`, `CatalogTableName`,
  `CatalogNamespace`, `MetadataFilePath`, `ReplaceInvalidCharacters`,
  `AutoRefresh`, `ClusterBy`, `Comment`, `Tags`.
- `IcebergColumn` — `<name> <type>` for the Snowflake-managed column list.
- `BuildCreateIcebergTableSql` — the only exported builder.
- `TableTypeSnowflake` / `TableTypeExternalCatalog` / `TableTypeDelta` /
  `TableTypeIcebergFiles` — the `TableType` values.

## Gotchas

- `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive; when both are set the
  builder drops `IF NOT EXISTS`.
- `EXTERNAL_VOLUME` and `CATALOG` are optional for **every** variant — a default
  may be set on the database, schema, or account — so each is emitted only when
  set (except the Snowflake-managed `CATALOG = 'SNOWFLAKE'`, always emitted).
- Only the Snowflake-managed variant carries a column list; the others infer
  columns from the existing files/catalog metadata.
- The required locator clause is variant-specific: `BASE_LOCATION` (Snowflake-managed
  + Delta), `CATALOG_TABLE_NAME` (external catalog), or `METADATA_FILE_PATH`
  (Iceberg files). It is the one field emitted with a `<placeholder>` when empty.
- Mutations (`COMMENT` `SET`/`UNSET`, manual `REFRESH`, and `RENAME TO`) are
  issued as free-form `ALTER ICEBERG TABLE` statements via `App.AlterIcebergTable`
  in `internal/app/icebergtable.go`, not built here. `REFRESH` re-syncs an
  externally-managed table's metadata; it is a no-op concept for the
  Snowflake-managed path.
- `GET_DDL` has **no** `ICEBERG_TABLE` object type — Iceberg tables are retrieved
  via the `'TABLE'` type. The `"ICEBERG TABLE"` → `TABLE` normalization lives in
  `internal/snowflake` (`buildGetDDLQuery`), not here.
- Iceberg tables surface in `SHOW OBJECTS` (with `is_iceberg = Y`); they are
  listed separately via `SHOW ICEBERG TABLES` and de-duplicated in
  `internal/snowflake` (`dedupeIcebergTables`).
