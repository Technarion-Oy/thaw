# components/hybridtable

UI for Snowflake **HYBRID TABLE** objects (Unistore / HTAP tables with an
enforced primary key and secondary indexes) in the object browser.

## Components

- **`CreateHybridTableModal.tsx`** — `CREATE HYBRID TABLE` builder. Name +
  case-sensitivity, an `IF NOT EXISTS` toggle (hybrid tables don't support
  `OR REPLACE`), a column editor (name, type, `NOT NULL`, `DEFAULT`, and a
  **PK** checkbox per column), a collapsible **secondary indexes** editor
  (name + key columns + optional `INCLUDE` columns, both **multi-select
  dropdowns** populated from the form's eligible columns), and a comment. A
  hybrid table requires a primary key, so submit is disabled until at least one
  column is flagged PK. Live SQL preview via `BuildCreateHybridTableSql`; runs
  through `ExecDDL`.
- **`HybridTablePropertiesModal.tsx`** — `GetObjectProperties("HYBRID TABLE", …)`
  (SHOW HYBRID TABLES) for the overview (owner, rows, bytes) + inline-editable
  **comment** via `AlterHybridTable` (`ALTER TABLE … SET`/`UNSET COMMENT`). An
  **Indexes & Primary Key** section lists `SHOW INDEXES IN TABLE` output
  (the PK surfaces here as a unique index) and supports adding
  (`CreateHybridTableIndex` → `CREATE INDEX`, with key / `INCLUDE` columns picked
  from dropdowns fetched via `GetTableColumnsWithTypes`, plus a case-sensitive
  name toggle) and dropping (`DropHybridTableIndex` → `DROP INDEX`) secondary
  indexes. Drop is offered only on non-unique indexes — the primary key surfaces
  as the lone UNIQUE index and cannot be dropped directly.
Both index editors get their eligible key / `INCLUDE` column lists from the
backend via `HybridIndexColumnOptions` (the datatype rules — semi-structured /
geospatial / VECTOR / `TIMESTAMP_TZ` barred from keys; semi-structured /
geospatial barred from `INCLUDE` — live in `internal/hybridtable`, built on
`snowflake.BaseType`), so the create modal and the properties dialog share one
source of truth with the SQL builder.

## Wiring

Registered in `components/layout/Sidebar.tsx` (kind `HYBRID TABLE`): Create-Object
→ Tables & Views submenu, type-node "Create Hybrid Table…", object-node
"Properties…", plus DROP / RENAME. Icon + colour live in
`components/sidebar/objectIcons.tsx` (`MergeCellsOutlined`, `--icon-hybridtable`).
Hybrid tables use the plain `TABLE` grammar for `ALTER`/`DROP`/`RENAME`, support
`GET_DDL` (via the `TABLE` type), and are queryable, so View Definition /
comparison / rename / Select Top 1000 are all available.
