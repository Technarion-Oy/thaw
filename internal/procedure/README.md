# internal/procedure

> SQL statement builders for invoking Snowflake stored procedures and user-defined functions.

## Responsibility

Constructs `CALL` statements for stored procedures and `SELECT` statements for
scalar/table UDFs. Handles argument value formatting — quoting string literals,
passing numeric and boolean values bare, and substituting `NULL` for empty inputs.
No network I/O; all functions are pure builders.

## Key files

| File | Purpose |
|------|---------|
| `doc.go` | Package doc + `thaw:domain` annotation (Object Browser & Administration) |
| `sql.go` | `Argument` type, `BuildCallStatement`, `BuildFunctionSelectStatement`, and private helpers |
| `sql_test.go` | Unit tests covering no-args, mixed types, SQL injection escaping, numeric scale |

## Key types & functions

### `Argument`
```go
type Argument struct {
    Name     string `json:"name"`
    DataType string `json:"dataType"`
    Value    string `json:"value"`
}
```
Carries one parameter's metadata and the user-supplied value. Sent from the
frontend as JSON and decoded by the `*App` IPC method before being passed to the
builders.

### `BuildCallStatement(db, schema, name string, args []Argument) string`
Produces `CALL "DB"."SCHEMA"."PROC"(arg1, arg2, ...);`.
- All identifiers are double-quoted with internal `"` doubled.
- Argument values are formatted via `formatValue`: numeric and boolean types are
  interpolated bare; strings are single-quoted with `'` doubled; empty value becomes `NULL`.

### `BuildFunctionSelectStatement(db, schema, name string, args []Argument, isTableFunction bool) string`
- Scalar UDF: `SELECT "DB"."SCHEMA"."FN"(args) AS result LIMIT 1000;`
- Table UDF: `SELECT * FROM TABLE("DB"."SCHEMA"."FN"(args)) LIMIT 1000;`

### Private helpers
- `formatValue(arg Argument) string` — type-aware value formatter (delegates to
  `snowflake.IsBoolean` / `snowflake.IsNumeric` for type classification).
- `escapeIdent(s string) string` — doubles double-quotes for SQL identifier escaping.

## Patterns & integration (thin-delegator)

`internal/app` (e.g. `internal/app/objects.go`) calls these builders, executes the
resulting SQL via `client.Execute`, and returns the `QueryResult` to the frontend.
No live connection or `App` state is required to call the builders, so they are
fully unit-testable.

The builders rely on `snowflake.IsBoolean` and `snowflake.IsNumeric` (from
`internal/snowflake`) for datatype classification rather than hardcoding regex
patterns in this package.

## Gotchas

- `BuildFunctionSelectStatement` appends `LIMIT 1000` for both scalar and table
  functions. This is intentional for safe preview queries but would need to be
  lifted for production data-export use cases.
- `formatValue` matches boolean type strings case-insensitively via
  `snowflake.IsBoolean`; numeric types via `snowflake.IsNumeric`. Composite types
  like `NUMBER(38,0)` or `DECIMAL(10,2)` are handled correctly by those helpers.
- Empty `Name` on `Argument` is not an error — only the `Value` field is used at
  call time; `Name` is informational metadata for the UI.
