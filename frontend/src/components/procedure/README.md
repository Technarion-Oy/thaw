# frontend/src/components/procedure

> Modal for interactively calling a Snowflake stored procedure with typed parameter inputs.

## Responsibility

Fetches a procedure's parameter list from the backend, renders an appropriate input
control for each parameter (boolean Select or text Input), builds a live `CALL` statement
preview, and executes it in a new SQL tab on submit.

## Files

| File | Purpose |
|------|---------|
| `CallProcedureModal.tsx` | Full call-procedure modal: parameter loading, type-based input rendering, CALL statement preview, and new-tab execution. |

## Patterns & integration

**IPC calls:**
- `GetProcedureParams(db, schema, name, rawArgs)` — returns a `Param[]` list (name + dataType) from `DESCRIBE PROCEDURE`
- `IsBoolean(dataType)` / `IsNumeric(dataType)` / `NeedsQuotes(dataType)` — backend datatype predicates; all three are called in parallel via `Promise.all` for every parameter after the params are loaded
- `BuildCallStatement(db, schema, name, args)` — builds the `CALL db.schema.name(...)` string with correct quoting; called via `useEffect` whenever param values change

**Execution:** On submit, `executeInNewTab(preview)` from `queryStore` opens the generated `CALL` statement in a new SQL editor tab and immediately executes it. The modal closes before the tab is activated.

**Input rendering:** Parameter type info (fetched from the backend) drives the control:
- `isBoolean` → `<Select>` with TRUE/FALSE options
- otherwise → `<Input>` with a placeholder that reflects whether the value needs quotes (`needsQuotes`) or is numeric

## Gotchas

- All three type predicates (`IsBoolean`, `IsNumeric`, `NeedsQuotes`) are backend round-trips called in a `Promise.all` for every parameter. For procedures with many parameters this can produce a burst of IPC calls. Results are not cached between modal opens.
- If `GetProcedureParams` fails, `params` is set to `[]` (empty array), which renders the "no parameters" message instead of an error state.
