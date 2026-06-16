# components/hybridtable

UI for Snowflake **HYBRID TABLE** objects (Unistore / HTAP tables with an
enforced primary key and secondary indexes) in the object browser.

## Components

- **`CreateHybridTableModal.tsx`** — `CREATE HYBRID TABLE` builder. Name +
  case-sensitivity, mutually-exclusive `OR REPLACE` / `IF NOT EXISTS`, a column
  editor (name, type, `NOT NULL`, `DEFAULT`, and a **PK** checkbox per column),
  a collapsible **secondary indexes** editor (name + comma-separated columns +
  optional `INCLUDE` columns), and a comment. A hybrid table requires a primary
  key, so submit is disabled until at least one column is flagged PK. Live SQL
  preview via `BuildCreateHybridTableSql`; runs through `ExecDDL`.
- **`HybridTablePropertiesModal.tsx`** — `GetObjectProperties("HYBRID TABLE", …)`
  (SHOW HYBRID TABLES) for the overview (owner, rows, bytes) + inline-editable
  **comment** via `AlterHybridTable` (`ALTER TABLE … SET`/`UNSET COMMENT`). An
  **Indexes & Primary Key** section lists `SHOW INDEXES IN TABLE` output
  (the PK surfaces here as a unique index) and supports adding
  (`CreateHybridTableIndex` → `CREATE INDEX`) and dropping
  (`DropHybridTableIndex` → `DROP INDEX`) secondary indexes.

## Wiring

Registered in `components/layout/Sidebar.tsx` (kind `HYBRID TABLE`): Create-Object
→ Tables & Views submenu, type-node "Create Hybrid Table…", object-node
"Properties…", plus DROP / RENAME. Icon + colour live in
`components/sidebar/objectIcons.tsx` (`TableOutlined`, `--icon-hybridtable`).
Hybrid tables use the plain `TABLE` grammar for `ALTER`/`DROP`/`RENAME`, support
`GET_DDL` (via the `TABLE` type), and are queryable, so View Definition /
comparison / rename / Select Top 1000 are all available.
