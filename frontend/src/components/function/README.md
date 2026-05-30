# frontend/src/components/function

> Modal for calling a Snowflake function or UDF with typed parameter inputs and a generated SELECT preview.

## Responsibility

Fetches a function's parameter list, renders type-appropriate input controls for each
parameter, generates a `SELECT db.schema.name(...)` statement via the backend, and
executes it in a new SQL tab on submit. Mirrors `CallProcedureModal` but for functions
(which return scalar values and use `SELECT` rather than `CALL`).

## Files

| File | Purpose |
|------|---------|
| `SelectFunctionModal.tsx` | Full select-function modal: parameter loading, type-based input rendering, SELECT statement preview, and new-tab execution. |

## Patterns & integration

**IPC calls:**
- `GetFunctionInfo(db, schema, name, rawArgs)` — returns param list (name + dataType)
- `IsBoolean(dataType)` / `IsNumeric(dataType)` / `NeedsQuotes(dataType)` — backend datatype predicates fetched in `Promise.all` for all parameters after load
- `BuildFunctionSelectStatement(db, schema, name, args)` — constructs the `SELECT` call with correct quoting; called on every param value change

**Execution:** `executeInNewTab(preview)` from `queryStore` opens the generated SELECT in a new tab and runs it immediately. The modal closes on submit.

**Input rendering:** Same logic as `CallProcedureModal`: `isBoolean` → Select (TRUE/FALSE), otherwise → Input with placeholder reflecting quoting behaviour.

**Stores used:** `queryStore` (`executeInNewTab`).
