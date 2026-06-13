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
| `CreateExternalTableModal.tsx` | `CREATE EXTERNAL TABLE` form — name/options (OR REPLACE / IF NOT EXISTS), a **Location** field with a database→schema→stage picker (the stage list is `ListStages` filtered to `EXTERNAL`-type stages, since external tables can only reference an external stage) and an inline **stage browser** (breadcrumb + navigable directory listing backed by `ListStageEntries`/`LIST @stage`); navigating folders keeps the editable `@"db"."schema"."stage"/path` Location text field in sync, and the field can also be typed/edited directly. A **File Format** chooser (inline `TYPE` select or a named `FILE FORMAT` object, sourced from `ListObjects`), an editable **Columns** list (name / type / `AS` expression / partition flag → `PARTITION BY`), and an **Advanced options** `Collapse` (Refresh On Create, Auto Refresh, Pattern, AWS SNS Topic, COPY GRANTS, table-level Tags) plus a comment. Uses `BuildCreateExternalTableSql` for live SQL preview. |
| `ExternalTablePropertiesModal.tsx` | Loads `GetObjectProperties(db, schema, "EXTERNAL TABLE", name)`; renders a header **Refresh** button and a validity tag, inline-editable **Auto Refresh** (`Select` TRUE/FALSE) and **Comment** (via `AlterExternalTable … SET/UNSET`), and the remaining `SHOW EXTERNAL TABLES` properties (location, file format, last refreshed, notification channel, pattern, …). |

## Patterns & integration

**IPC calls:**
- `BuildCreateExternalTableSql(db, schema, cfg)` — live SQL preview (direct `useEffect` dependency, no explicit debounce timer)
- `ExecDDL(sql)` — executes the CREATE DDL on submit; the statement is rebuilt fresh via `BuildCreateExternalTableSql(db, schema, cfg)` at submit time rather than reusing the debounced `preview` state (which lags a keystroke behind)
- `GetQuotedIdentifiersIgnoreCase()` — feeds `ObjectNameCaseControl`
- `ListDatabases()` / `ListSchemas(db)` — feed the cascading database/schema selects
- `ListStages(db, schema)` — stage picker options, filtered to `type === "EXTERNAL"`
- `ListObjects(db, schema)` — file-format picker options (filtered to `FILE FORMAT`)
- `ListStageEntries(db, schema, stage, dirPath)` — directory-aware listing for the inline stage browser; navigating folders composes the `@stage/path` Location
- `GetObjectProperties(db, schema, "EXTERNAL TABLE", name)` — properties panel data
- `AlterExternalTable(db, schema, name, clause)` — `REFRESH` / `SET AUTO_REFRESH = …` / `SET … ` / `UNSET …`

**`externaltable.ExternalTableConfig` type** from `wailsjs/go/models`: `name`, `caseSensitive`, `orReplace`, `ifNotExists`, `columns` (`{name, type, expression, partition}[]`), `location`, `refreshOnCreate`, `autoRefresh`, `pattern`, `fileFormatName`, `fileFormatType`, `awsSnsTopic`, `copyGrants`, `comment`, `tags` (`{name, value}[]`). The Create modal keeps form state in a local `ETConfig` (the generated class carries a `convertValues` method — see Gotchas — that a plain literal can't satisfy) and casts to the generated type only at the IPC boundary.

**Shared components:** `ObjectNameCaseControl` for case-sensitivity; inline SQL preview block.

## Gotchas

- `BuildCreateExternalTableSql` runs on every `cfg` change without an explicit debounce for the live preview; rapid typing generates frequent IPC calls (same tradeoff as `CreatePipeModal` / `CreateDynamicTableModal`). Submitting rebuilds the statement from the current `cfg` rather than trusting that preview state.
- External-table columns must each carry an `AS (<expr>)` transformation referencing the staged data (e.g. `value:c1::varchar`, `metadata$filename`); the modal does not validate the general case — Snowflake reports errors at execution time. As a targeted guard, a **partition**-flagged column with an empty expression shows an inline warning, since the builder would otherwise emit `AS (value)`, which Snowflake rejects in `PARTITION BY`.
- `auto_refresh` is normalized from the assorted representations `SHOW EXTERNAL TABLES` may return (`true`/`false`, `Y`/`N`) into `TRUE`/`FALSE` for the Select editor. Because that column is not exposed on every Snowflake edition, the properties modal falls back to inferring the state from a non-empty `notification_channel` when `auto_refresh` is absent. `comment` and `auto_refresh` are excluded from the generic Properties table because they are surfaced in the editable Settings section (`notification_channel` is kept visible so the inferred state is auditable).
