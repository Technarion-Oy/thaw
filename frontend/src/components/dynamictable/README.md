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
| `CreateDynamicTableModal.tsx` | `CREATE DYNAMIC TABLE` form — name/options, Target Lag, Warehouse (from `ListWarehouses`), Refresh Mode, Initialize, Cluster By, comment, and an embedded Monaco editor for the defining query. Uses `BuildCreateDynamicTableSql` for live SQL preview. |
| `DynamicTablePropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "DYNAMIC TABLE", name)`; renders header Refresh/Suspend/Resume buttons, inline-editable Target Lag / Warehouse / Comment (via `AlterDynamicTable … SET/UNSET`), the remaining `SHOW DYNAMIC TABLES` properties, and the rendered defining query (`text` column). |

## Patterns & integration

**IPC calls:**
- `BuildCreateDynamicTableSql(db, schema, cfg)` — live SQL preview (direct `useEffect` dependency, no explicit debounce timer)
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `ListWarehouses()` — populates the Warehouse select
- `GetObjectProperties(db, schema, "DYNAMIC TABLE", name)` — properties panel data
- `AlterDynamicTable(db, schema, name, clause)` — `SUSPEND` / `RESUME` / `REFRESH` / `SET …` / `UNSET …`

**`dynamictable.DynamicTableConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`, `orReplace`, `ifNotExists`, `transient`, `targetLag`, `warehouse`, `refreshMode`, `initialize`, `clusterBy`, `comment`, `query`.

**Target Lag:** accepts a duration string (`1 minute`) or the bare `DOWNSTREAM` keyword; `targetLagAssign` in the properties modal renders the correct `SET TARGET_LAG` clause (quoted literal vs. bare keyword).

**Shared components:** `ObjectNameCaseControl` for case-sensitivity; inline SQL preview block. **Stores used:** `themeStore` (Monaco editor theme).

## Gotchas

- `BuildCreateDynamicTableSql` runs on every `cfg` change without an explicit debounce; rapid typing in the query editor generates frequent IPC calls (same tradeoff as `CreatePipeModal`).
- The properties panel reads the defining query from the `text` column of `SHOW DYNAMIC TABLES`; `target_lag`, `warehouse`, `comment`, and `text` are excluded from the generic Properties table because they are surfaced in dedicated sections.
