# frontend/src/components/externaltable

> Modals for managing Snowflake External Table objects: create and view/edit properties.

## Responsibility

Provides the create and properties UI for Snowflake External Table objects.
`CreateExternalTableModal` follows the standard debounced SQL preview pattern.
`ExternalTablePropertiesModal` shows `SHOW EXTERNAL TABLES` metadata,
inline-editable settings, and a Refresh action. The remaining operations
(Refresh / Drop / Rename) are driven from the sidebar context menu in
`layout/Sidebar.tsx` via `App.AlterExternalTable`.

## Files

| File | Purpose |
|------|---------|
| `CreateExternalTableModal.tsx` | `CREATE EXTERNAL TABLE` form — name/options (OR REPLACE / IF NOT EXISTS), a **Location** field with a database→schema→stage picker (+ path) that composes `@"db"."schema"."stage"/path`, a **File Format** chooser (inline `TYPE` select or a named `FILE FORMAT` object, sourced from `ListObjects`), an editable **Columns** list (name / type / `AS` expression / partition flag → `PARTITION BY`), and an **Advanced options** `Collapse` (Refresh On Create, Auto Refresh, Pattern, AWS SNS Topic, COPY GRANTS, table-level Tags) plus a comment. Uses `BuildCreateExternalTableSql` for live SQL preview. |
| `ExternalTablePropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "EXTERNAL TABLE", name)`; renders a header **Refresh** button and a validity tag, inline-editable **Auto Refresh** (`Select` TRUE/FALSE) and **Comment** (via `AlterExternalTable … SET/UNSET`), and the remaining `SHOW EXTERNAL TABLES` properties (location, file format, last refreshed, notification channel, pattern, …). |

## Patterns & integration

**IPC calls:**
- `BuildCreateExternalTableSql(db, schema, cfg)` — live SQL preview (direct `useEffect` dependency, no explicit debounce timer)
- `ExecDDL(preview)` — executes the CREATE DDL on submit
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `ListDatabases()` / `ListSchemas(db)` / `ListObjects(db, schema)` — feed the cascading stage / file-format pickers (filtered to `STAGE` and `FILE FORMAT`)
- `GetObjectProperties(db, schema, "EXTERNAL TABLE", name)` — properties panel data
- `AlterExternalTable(db, schema, name, clause)` — `REFRESH` / `SET AUTO_REFRESH = …` / `SET … ` / `UNSET …`

**`externaltable.ExternalTableConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`, `orReplace`, `ifNotExists`, `columns` (`{name, type, expression, partition}[]`), `location`, `refreshOnCreate`, `autoRefresh`, `pattern`, `fileFormatName`, `fileFormatType`, `awsSnsTopic`, `copyGrants`, `comment`, `tags` (`{name, value}[]`). The Create modal keeps form state in a local `ETConfig` (the generated class carries a `convertValues` method — see Gotchas — that a plain literal can't satisfy) and casts to the generated type only at the IPC boundary.

**Shared components:** `ObjectNameCaseControl` for case-sensitivity; inline SQL preview block.

## Gotchas

- `BuildCreateExternalTableSql` runs on every `cfg` change without an explicit debounce; rapid typing generates frequent IPC calls (same tradeoff as `CreatePipeModal` / `CreateDynamicTableModal`).
- External-table columns must each carry an `AS (<expr>)` transformation referencing the staged data (e.g. `value:c1::varchar`, `metadata$filename`); the modal does not validate these — Snowflake reports errors at execution time.
- `auto_refresh` is normalized from the assorted representations `SHOW EXTERNAL TABLES` may return (`true`/`false`, `Y`/`N`) into `TRUE`/`FALSE` for the Select editor; `comment` and `auto_refresh` are excluded from the generic Properties table because they are surfaced in the editable Settings section.
