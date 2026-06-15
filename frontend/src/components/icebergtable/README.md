# components/icebergtable

UI for Snowflake **ICEBERG TABLE** objects (Apache Iceberg open-format tables on
an external volume) in the object browser.

## Components

- **`CreateIcebergTableModal.tsx`** — `CREATE ICEBERG TABLE` builder. Name +
  case-sensitivity, mutually-exclusive `OR REPLACE` / `IF NOT EXISTS`, and a
  **table type** selector that switches between Snowflake's variants (each has
  different required fields, so the form shows only the relevant ones):
  - *Snowflake-managed* (`CATALOG = 'SNOWFLAKE'`) — a column list (name + type)
    and a **base location**.
  - *External Iceberg catalog (REST / AWS Glue)* — **catalog table name** (req)
    + optional **catalog namespace** / `AUTO_REFRESH`.
  - *Delta Lake files* — **base location** (req, the dir with `_delta_log/`) +
    optional `AUTO_REFRESH`; the catalog integration must be
    `OBJECT_STORE`/`TABLE_FORMAT=DELTA`.
  - *Iceberg files in object storage* — **metadata file path** (req); no
    `AUTO_REFRESH`.

  All external variants share an optional **catalog integration** (a searchable
  dropdown sourced from `ListIntegrations("CATALOG")` / `SHOW CATALOG
  INTEGRATIONS`, showing each integration's type) + a
  `REPLACE_INVALID_CHARACTERS` toggle. Every variant shares the optional
  **external volume** (a searchable dropdown sourced from `ListExternalVolumes` /
  `SHOW EXTERNAL VOLUMES`), tags (`TagInput`), and comment; cluster-by is
  Snowflake-managed only. Live SQL preview via `BuildCreateIcebergTableSql`; runs
  through `ExecDDL`.
- **`IcebergTablePropertiesModal.tsx`** — `GetObjectProperties("ICEBERG TABLE", …)`
  (SHOW ICEBERG TABLES). Surfaces the Iceberg highlights (**external volume**,
  **catalog**, **base location**, **catalog table name**), a **Refresh** button
  (`ALTER ICEBERG TABLE … REFRESH`, re-syncs an externally-managed table's
  metadata), inline-editable **comment** via `AlterIcebergTable` `SET`/`UNSET`,
  and the remaining columns in a generic properties table.

## Wiring

Registered in `components/layout/Sidebar.tsx` (kind `ICEBERG TABLE`): Create-Object
→ Tables & Views submenu, type-node "Create Iceberg Table…", object-node
"Properties…" + "Refresh…", plus DROP / RENAME. Icon + colour live in
`components/sidebar/objectIcons.tsx` (`GoldOutlined`, `--icon-icebergtable`).
Iceberg tables support `GET_DDL` (via the `TABLE` type) and are queryable, so View
Definition / comparison / rename / Select Top 1000 are all available.
