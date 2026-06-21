# frontend/src/components/function

> UI for Snowflake user-defined **FUNCTION** (UDF) objects: creating them, editing
> their properties, and calling them.

## Responsibility

Covers the full UDF lifecycle in the object browser — a create modal that builds a
`CREATE FUNCTION` statement, a properties modal for inline comment / SECURE edits,
and a select modal that calls an existing function with typed parameter inputs and
a generated `SELECT` preview (mirrors `CallProcedureModal`, but functions return
values and use `SELECT` rather than `CALL`).

## Files

| File | Purpose |
|------|---------|
| `CreateFunctionModal.tsx` | Builds a `CREATE FUNCTION` statement: name + `OR REPLACE` / `IF NOT EXISTS`, case control, an argument editor (name + a backend-derived `DataTypeAutoComplete` type field), a Language select (SQL / Python / Java / JavaScript / Scala), a "Returns TABLE" switch toggling between a scalar return-type field and a table-columns editor (both the scalar return type and the table column types use `DataTypeAutoComplete`), an Advanced section gated to handler languages (`SECURE`, null handling, volatility, `RUNTIME_VERSION`, `PACKAGES`, `IMPORTS`, `HANDLER`; the per-language placeholder/help follow the selected runtime), a comment, and the function body via `MonacoSqlField`. Live SQL preview; submission gated on a name, a body, and — for Python/Java/Scala — `RUNTIME_VERSION` + `HANDLER`. SQL is built by `BuildCreateFunctionSql` (delegating to `internal/udf`); executed via `ExecDDL`. |
| `FunctionPropertiesModal.tsx` | Properties view (`GetRoutineProperties`, kind `FUNCTION`, passing the `args` signature so the correct overload's row is shown — `SHOW FUNCTIONS` returns one row per overload): an editable **Settings** section (inline-editable comment and a `SECURE` toggle, both via `AlterFunction` targeting the same `args` signature) plus a generic **Properties** table of the remaining `SHOW FUNCTIONS` rows. Functions have no lifecycle (no suspend/resume) and the modal does not fetch the defining query (GET_DDL needs the argument signature, handled elsewhere). |
| `SelectFunctionModal.tsx` | Full select-function modal: parameter loading, type-based input rendering, SELECT statement preview, and new-tab execution. |

## Patterns & integration

**IPC calls:**
- `GetFunctionInfo(db, schema, name, rawArgs)` — returns param list (name + dataType)
- `IsBoolean(dataType)` / `IsNumeric(dataType)` / `NeedsQuotes(dataType)` — backend datatype predicates fetched in `Promise.all` for all parameters after load
- `BuildFunctionSelectStatement(db, schema, name, args)` — constructs the `SELECT` call with correct quoting; called on every param value change

**Execution:** `executeInNewTab(preview)` from `queryStore` opens the generated SELECT in a new tab and runs it immediately. The modal closes on submit.

**Input rendering:** Same logic as `CallProcedureModal`: `isBoolean` → Select (TRUE/FALSE), otherwise → Input with placeholder reflecting quoting behaviour.

**Stores used:** `queryStore` (`executeInNewTab`).
