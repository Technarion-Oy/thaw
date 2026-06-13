# frontend/src/components/materializedview

> Modals for managing Snowflake Materialized View objects: create and view/edit properties.

## Responsibility

Provides the create and properties UI for Snowflake Materialized View objects.
`CreateMaterializedViewModal` follows the standard debounced SQL preview pattern.
`MaterializedViewPropertiesModal` shows `SHOW MATERIALIZED VIEWS` metadata, the
rendered defining query, inline-editable settings, and lifecycle actions. The
remaining operations (Suspend / Resume / Drop / Rename) are driven from the
sidebar context menu in `layout/Sidebar.tsx` via `App.AlterMaterializedView`.

## Files

| File | Purpose |
|------|---------|
| `CreateMaterializedViewModal.tsx` | `CREATE MATERIALIZED VIEW` form — name/options (OR REPLACE / IF NOT EXISTS), comment, and an embedded Monaco editor for the defining query with an **Insert from table** database→schema→table picker that generates the same `SELECT "col", … FROM "db"."schema"."table"` snippet as dragging a table from the object store (falls back to `SELECT *`); an **Advanced options** `Collapse` covers Cluster By, SECURE, COPY GRANTS, and view-level Tags. Uses `BuildCreateMaterializedViewSql` for live SQL preview. |
| `MaterializedViewPropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "MATERIALIZED VIEW", name)`; renders a Valid/Invalid status tag (+ behind-by lag), header Suspend/Resume buttons, inline-editable Comment and a SECURE toggle (via `AlterMaterializedView … SET/UNSET`), the remaining `SHOW MATERIALIZED VIEWS` properties, and the rendered defining query (`text` column). |

## Patterns & integration

**IPC calls:**
- `BuildCreateMaterializedViewSql(db, schema, cfg)` — live SQL preview (direct `useEffect` dependency, no explicit debounce timer)
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `ListDatabases()` / `ListSchemas(db)` / `ListObjects(db, schema)` — feed the **Insert from table** cascading picker (objects filtered to `TABLE`/`VIEW`)
- `GetTableColumns(db, schema, table)` — column list for the inserted `SELECT` (mirrors the SQL-editor drag-and-drop); inserts at the cursor via the captured Monaco editor ref, or replaces the body when it's empty/placeholder
- `GetObjectProperties(db, schema, "MATERIALIZED VIEW", name)` — properties panel data
- `AlterMaterializedView(db, schema, name, clause)` — `SUSPEND` / `RESUME` / `SET SECURE` / `UNSET SECURE` / `SET COMMENT …` / `UNSET COMMENT`

**`materializedview.MaterializedViewConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`, `orReplace`, `secure`, `ifNotExists`, `copyGrants`, `comment`, `clusterBy`, `tags` (`{name, value}[]`), `query`. The Create modal keeps form state in a local `MVConfig` (the generated class carries a `convertValues` method — see Gotchas — that a plain literal can't satisfy) and casts to the generated type only at the IPC boundary.

**Shared components:** `ObjectNameCaseControl` for case-sensitivity; inline SQL preview block. **Stores used:** `themeStore` (Monaco editor theme).

## Gotchas

- Materialized views have **no manual REFRESH** — Snowflake maintains them
  automatically; only Suspend/Resume (use + maintenance) are offered.
- `BuildCreateMaterializedViewSql` runs on every `cfg` change without an explicit
  debounce; rapid typing in the query editor generates frequent IPC calls (same
  tradeoff as `CreateDynamicTableModal`).
- **Create** stays disabled until the defining query is edited away from the seeded
  `DEFAULT_QUERY` placeholder — submitting the untouched template would `CREATE …
  AS SELECT * FROM my_source_table` and fail server-side.
- The properties panel reads the defining query from the `text` column of `SHOW
  MATERIALIZED VIEWS`; `comment`, `is_secure`, and `text` are excluded from the
  generic Properties table because they are surfaced in dedicated sections.
