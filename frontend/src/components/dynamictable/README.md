# frontend/src/components/dynamictable

> Modals for managing Snowflake Dynamic Table objects: create, view/edit properties, and refresh history.

## Responsibility

Provides the create and properties UI for Snowflake Dynamic Table objects.
`CreateDynamicTableModal` follows the standard debounced SQL preview pattern.
`DynamicTablePropertiesModal` shows `SHOW DYNAMIC TABLES` metadata, the rendered
defining query, inline-editable settings, and lifecycle actions.
`DynamicTableRefreshHistoryModal` lists recent refreshes. The remaining
operations (Refresh [Copy Session] / Suspend / Resume / Swap With / Clustering ▸ /
Search Optimization ▸ / Drop / Rename, plus the column-node Comment / Policies ▸ /
Tag items) are driven from the sidebar context menu in `layout/Sidebar.tsx` via
one generic confirm-or-prompt helper (`alterAction`, shared with base tables) that calls `App.AlterDynamicTable`.

## Files

| File | Purpose |
|------|---------|
| `CreateDynamicTableModal.tsx` | `CREATE DYNAMIC TABLE` form — name/options (OR REPLACE / IF NOT EXISTS / TRANSIENT), a Target Lag composer (number + unit, or DOWNSTREAM), Warehouse (from `ListWarehouses`), comment, and an embedded Monaco editor for the defining query with an **Insert from table** database→schema→table picker that generates the same `SELECT "col", … FROM "db"."schema"."table"` snippet as dragging a table from the object store (falls back to `SELECT *`); an **Advanced options** `Collapse` covers every other table-level clause (Refresh Mode incl. ADAPTIVE/CUSTOM_INCREMENTAL, Initialize, Scheduler, Initialization Warehouse, Cluster By, Data Retention / Max Data Extension days, Row Timestamp, COPY GRANTS, REQUIRE USER, and table-level Tags). Uses `BuildCreateDynamicTableSql` for live SQL preview. |
| `DynamicTablePropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "DYNAMIC TABLE", name)` plus (best-effort) `GetObjectParameters("TABLE", [db, schema, name])` for the parameter-backed rows; renders header Refresh/Suspend/Resume buttons plus **Refresh History** (opens `DynamicTableRefreshHistoryModal`). **Settings** uses the shared `../shared/PropertyRows` (`EditRow` / `SelectRow` / `PickerRow`): Target Lag, Warehouse (a local `SelectEditRow` sourced from `ListWarehouses` — `WAREHOUSE` can't be unset), Initialization Warehouse, Scheduler, Cluster By (`LINEAR(…)` stripped for editing; empty → `DROP CLUSTERING KEY`), Data Retention / Max Data Extension days, Default DDL Collation, Log Level, Frozen Where (read from `immutable_where`), Execute As User, Row Timestamp, Comment, Tags, and a Contact set/unset row. A **Governance** section (`../common/GovernanceRows`, shared with the base-table `PropertiesModal`) of `ActionRow`s attaches/detaches row access, aggregation, and storage lifecycle policies — current attachments aren't in SHOW, so these are editors, not value rows. Then the remaining `SHOW DYNAMIC TABLES` properties and the rendered defining query (`text` column). |
| `dynamicTableClauses.ts` | Pure clause builders for every `ALTER DYNAMIC TABLE` action (clustering, search optimization, `EXECUTE AS USER`, contacts, row access / aggregation / storage lifecycle policies, column comment / masking / projection / tag). User-typed identifiers are **validated, not quoted** (`sqlIdent`: bare or `"quoted"` parts, ≤ 3) so Snowflake's case rules apply and only an identifier can be spliced; expressions (`CLUSTER BY`, `FROZEN WHERE`, search-optimization `ON` targets) pass through as written. Has no Wails imports so `dynamicTableClauses.test.ts` runs under vitest. |
| `DynamicTableRefreshHistoryModal.tsx` | Opened from the sidebar's **View Refresh History…** item or the Properties header. Calls `GetDynamicTableRefreshHistory(db, schema, name, days)` with a Last 24 Hours / Last 7 Days selector (the function's retention is 7 days) and a Refresh button; shows state tags (FAILED / UPSTREAM_FAILED rows carry a `STATE_CODE`/`STATE_MESSAGE` tooltip and the `task-row-failed` highlight), action, trigger, data timestamp, start/end, duration, rows inserted/deleted, target lag, and a copyable query ID. Snowflake errors (e.g. missing MONITOR privilege) are shown verbatim. |

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
- `GetDynamicTableRefreshHistory(db, schema, name, days)` — refresh history rows (`dynamictable.RefreshHistoryRow`)

**`dynamictable.DynamicTableConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`, `orReplace`, `ifNotExists`, `transient`, `targetLag`, `scheduler`, `warehouse`, `initializationWarehouse`, `refreshMode`, `initialize`, `clusterBy`, `dataRetentionTimeInDays`, `maxDataExtensionTimeInDays`, `comment`, `copyGrants`, `requireUser`, `rowTimestamp`, `tags` (`{name, value}[]`), `query`. The Create modal keeps form state in a local `DTConfig` (the generated class carries a `convertValues` method — see Gotchas — that a plain literal can't satisfy) and casts to the generated type only at the IPC boundary.

**Target Lag:** the create form composes `targetLag` from an interval (number + `seconds|minutes|hours|days`) or the bare `DOWNSTREAM` keyword; the Go builder and the properties modal's `targetLagAssign` each render the correct quoted-literal-vs-keyword form.

**Shared components:** `ObjectNameCaseControl` for case-sensitivity; inline SQL preview block. **Stores used:** `themeStore` (Monaco editor theme).

## Gotchas

- `BuildCreateDynamicTableSql` runs on every `cfg` change without an explicit debounce; rapid typing in the query editor generates frequent IPC calls (same tradeoff as `CreatePipeModal`).
- Refresh-history rows are keyed by `queryId + refreshStartTime + dataTimestamp`, never `dataTimestamp` alone — retries share a `DATA_TIMESTAMP`.
- The properties panel reads the defining query from the `text` column of `SHOW DYNAMIC TABLES`; `target_lag`, `warehouse`, `comment`, and `text` are excluded from the generic Properties table because they are surfaced in dedicated sections.
