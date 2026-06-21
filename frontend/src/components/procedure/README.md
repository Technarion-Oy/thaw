# frontend/src/components/procedure

> Modals for creating, inspecting, and interactively calling Snowflake stored procedures.

## Responsibility

Create a stored procedure (with a live `CREATE PROCEDURE` preview), inspect and edit an
existing procedure's properties, and interactively call a procedure with typed parameter
inputs.

## Files

| File | Purpose |
|------|---------|
| `CallProcedureModal.tsx` | Full call-procedure modal: parameter loading, type-based input rendering, CALL statement preview, and new-tab execution. |
| `CreateProcedureModal.tsx` | Create-procedure modal: name + OR REPLACE / IF NOT EXISTS, case control, an arguments editor (name + a backend-derived `DataTypeAutoComplete` type field; the RETURNS-TABLE column editor uses it too), a Language select (SQL/PYTHON/JAVA/JAVASCRIPT/SCALA), a RETURNS-TABLE switch (scalar return type vs. a table-columns editor; the scalar return type and the table column types both use `DataTypeAutoComplete`), an Advanced collapse with the handler-only fields gated to Python/Java/Scala and their placeholder/help following the selected runtime (SECURE, RUNTIME_VERSION, PACKAGES, IMPORTS, HANDLER, null handling, volatility, EXECUTE AS), a comment, and a Monaco-backed procedure body. Submission is gated on a name, a body, and — for handler languages — RUNTIME_VERSION + HANDLER. Builds the DDL via `BuildCreateProcedureSql` and runs it via `ExecDDL`. |
| `ProcedurePropertiesModal.tsx` | Procedure properties modal: an editable Comment row and a SECURE toggle (via `AlterProcedure` `SET`/`UNSET COMMENT` and `SET`/`UNSET SECURE`), plus a generic table of the remaining `SHOW PROCEDURES` rows. Procedures have no lifecycle and no DDL/defining-query fetch. |

## Create / Properties patterns

**`CreateProcedureModal`** follows the shared create-modal convention: `CreateModalShell`
chrome, `NameWithReplaceOptions` + `ObjectNameCaseControl` for the name, `useSqlPreview`
for the live preview (built backend-side by `procedure.BuildCreateProcedureSql`), and
`useCreateSubmit` for submit plumbing. The form state is a plain object cast to the
generated `procedure.ProcedureConfig` only at the IPC boundary (`cfg as any`), because the
generated class carries a `convertValues` method that a literal can't satisfy. `canSubmit`
requires a non-empty name and body. Packages / imports use `Select mode="tags"`.

**`ProcedurePropertiesModal`** mirrors the materialized-view properties modal but drops the
lifecycle controls and the DDL fetch: it loads `GetObjectProperties(db, schema, "PROCEDURE",
name)` and edits via `AlterProcedure(db, schema, name, clause)`.

## Patterns & integration

**IPC calls:**
- `GetProcedureParams(db, schema, name, rawArgs)` — returns a `Param[]` list (name + dataType) from `DESCRIBE PROCEDURE`
- `IsBoolean(dataType)` / `IsNumeric(dataType)` / `NeedsQuotes(dataType)` — backend datatype predicates; all three are called in parallel via `Promise.all` for every parameter after the params are loaded
- `BuildCallStatement(db, schema, name, args)` — builds the `CALL db.schema.name(...)` string with correct quoting; called via `useEffect` whenever param values change

**Execution:** On submit, `executeInNewTab(preview)` from `queryStore` opens the generated `CALL` statement in a new SQL editor tab and immediately executes it. The modal closes before the tab is activated.

**Insert mode:** When the optional `onInsert?: (sql: string) => void` prop is supplied, the primary button becomes **Insert** and hands the built `CALL` statement back to the caller instead of executing it — used by the Alert builder (`components/alert/CreateAlertModal.tsx`) to drop a `CALL` into the condition editor without duplicating the parameter UI or the `BuildCallStatement` logic.

**Input rendering:** Parameter type info (fetched from the backend) drives the control:
- `isBoolean` → `<Select>` with TRUE/FALSE options
- otherwise → `<Input>` with a placeholder that reflects whether the value needs quotes (`needsQuotes`) or is numeric

## Gotchas

- All three type predicates (`IsBoolean`, `IsNumeric`, `NeedsQuotes`) are backend round-trips called in a `Promise.all` for every parameter. For procedures with many parameters this can produce a burst of IPC calls. Results are not cached between modal opens.
- If `GetProcedureParams` fails, `params` is set to `[]` (empty array), which renders the "no parameters" message instead of an error state.
