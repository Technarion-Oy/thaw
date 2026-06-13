# frontend/src/components/dynamictable

> Modals for managing Snowflake Dynamic Table objects: create and view/edit properties.

## Responsibility

Provides the create and properties UI for Snowflake Dynamic Table objects.
`CreateDynamicTableModal` follows the standard debounced SQL preview pattern.
`DynamicTablePropertiesModal` shows `SHOW DYNAMIC TABLES` metadata, the rendered
defining query, inline-editable settings, and lifecycle actions. The remaining
operations (Refresh / Suspend / Resume / Drop / Rename) are driven from the
sidebar context menu in `layout/Sidebar.tsx` via `App.AlterDynamicTable`.

## Files

| File | Purpose |
|------|---------|
| `CreateDynamicTableModal.tsx` | `CREATE DYNAMIC TABLE` form — name/options (OR REPLACE / IF NOT EXISTS / TRANSIENT), a Target Lag composer (number + unit, or DOWNSTREAM), Warehouse (from `ListWarehouses`), comment, and an embedded Monaco editor for the defining query with an **Insert from table** database→schema→table picker that generates the same `SELECT "col", … FROM "db"."schema"."table"` snippet as dragging a table from the object store (falls back to `SELECT *`); an **Advanced options** `Collapse` covers every other table-level clause (Refresh Mode incl. ADAPTIVE/CUSTOM_INCREMENTAL, Initialize, Scheduler, Initialization Warehouse, Cluster By, Data Retention / Max Data Extension days, Row Timestamp, COPY GRANTS, REQUIRE USER, and table-level Tags). Uses `BuildCreateDynamicTableSql` for live SQL preview. |
| `DynamicTablePropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "DYNAMIC TABLE", name)`; renders header Refresh/Suspend/Resume buttons, inline-editable Target Lag (text) / Warehouse (a `Select` sourced from `ListWarehouses` so the saved identifier is always canonically cased) / Comment (via `AlterDynamicTable … SET/UNSET`), the remaining `SHOW DYNAMIC TABLES` properties, and the rendered defining query (`text` column). |

## Patterns & integration

**IPC calls:**
- `BuildCreateDynamicTableSql(db, schema, cfg)` — live SQL preview (direct `useEffect` dependency, no explicit debounce timer)
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `ListWarehouses()` — populates the Warehouse select
- `ListDatabases()` / `ListSchemas(db)` / `ListObjects(db, schema)` — feed the **Insert from table** cascading picker (objects filtered to `TABLE`/`VIEW`/`DYNAMIC TABLE`, since a dynamic table is a valid source for another)
- `GetTableColumns(db, schema, table)` — column list for the inserted `SELECT` (mirrors the SQL-editor drag-and-drop); inserts at the cursor via the captured Monaco editor ref, or replaces the body when it's empty/placeholder
- `GetObjectProperties(db, schema, "DYNAMIC TABLE", name)` — properties panel data
- `AlterDynamicTable(db, schema, name, clause)` — `SUSPEND` / `RESUME` / `REFRESH` / `SET …` / `UNSET …`

**`dynamictable.DynamicTableConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`, `orReplace`, `ifNotExists`, `transient`, `targetLag`, `scheduler`, `warehouse`, `initializationWarehouse`, `refreshMode`, `initialize`, `clusterBy`, `dataRetentionTimeInDays`, `maxDataExtensionTimeInDays`, `comment`, `copyGrants`, `requireUser`, `rowTimestamp`, `tags` (`{name, value}[]`), `query`. The Create modal keeps form state in a local `DTConfig` (the generated class carries a `convertValues` method — see Gotchas — that a plain literal can't satisfy) and casts to the generated type only at the IPC boundary.

**Target Lag:** the create form composes `targetLag` from an interval (number + `seconds|minutes|hours|days`) or the bare `DOWNSTREAM` keyword; the Go builder and the properties modal's `targetLagAssign` each render the correct quoted-literal-vs-keyword form.

**Shared components:** `ObjectNameCaseControl` for case-sensitivity; inline SQL preview block. **Stores used:** `themeStore` (Monaco editor theme).

## Gotchas

- `BuildCreateDynamicTableSql` runs on every `cfg` change without an explicit debounce; rapid typing in the query editor generates frequent IPC calls (same tradeoff as `CreatePipeModal`).
- The properties panel reads the defining query from the `text` column of `SHOW DYNAMIC TABLES`; `target_lag`, `warehouse`, `comment`, and `text` are excluded from the generic Properties table because they are surfaced in dedicated sections.
