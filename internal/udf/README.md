# internal/function

Builds SQL for Snowflake user-defined **FUNCTION** (UDF) objects. Unlike an
EXTERNAL FUNCTION (which proxies an HTTPS call to code running outside Snowflake),
a regular UDF carries its own body: a SQL expression, or handler code in Python,
Java, JavaScript, or Scala that Snowflake runs inside the warehouse.

## What it does

`BuildCreateFunctionSql(db, schema, cfg)` renders a `CREATE FUNCTION` statement
from a `FunctionConfig`:

```
CREATE [OR REPLACE] [SECURE] FUNCTION [IF NOT EXISTS] <fqn> ( [ <arg> <type> [, ...] ] )
  RETURNS { <result_type> | TABLE ( <col> <type> [, ...] ) }
  [ LANGUAGE <language> ]
  [ { CALLED ON NULL INPUT | RETURNS NULL ON NULL INPUT } ]
  [ { VOLATILE | IMMUTABLE } ]
  [ RUNTIME_VERSION = '<version>' ]
  [ PACKAGES = ( '<pkg>', ... ) ]
  [ IMPORTS = ( '<stage_path>', ... ) ]
  [ HANDLER = '<handler>' ]
  [ COMMENT = '<string>' ]
  AS $$ <body> $$;
```

## Types & builders

- `FunctionConfig` — name + case sensitivity, `OrReplace`, `Secure`,
  `IfNotExists`, `Args` (`[]FuncArg`), `ReturnType` / `ReturnsTable` +
  `TableColumns`, `Language`, `NullHandling`, `Volatility`, `RuntimeVersion`,
  `Packages`, `Imports`, `Handler`, `Comment`, `Body`.
- `FuncArg` — a single `{Name, DataType}` parameter (also reused for the
  `RETURNS TABLE` column list).
- `BuildCreateFunctionSql` — the only exported builder.

## Gotchas

- **`LANGUAGE` is omitted for SQL functions** — SQL is Snowflake's default. The
  builder uppercases the language and emits `LANGUAGE <lang>` only when it is set
  and not `SQL`.
- The body is wrapped identically (`AS $$ … $$`) regardless of language: for SQL
  functions it is the returned expression, for the other languages it is the
  handler source code.
- Empty fields fall back to placeholders (`function_name`, `RETURNS VARIANT`,
  `<function_body>`) so the live preview stays readable; the Create modal gates
  submission on the real name and body being present.
- Functions share the **regular `FUNCTION` management commands** — there is no
  function-specific ALTER variant. Mutations (`SET`/`UNSET COMMENT`,
  `SET`/`UNSET SECURE`, `RENAME TO`) are issued as free-form
  `ALTER FUNCTION <fqn> <clause>` via `App.AlterFunction`. Because functions are
  overloaded by argument types, commands that must resolve a specific overload
  (DROP, GET_DDL) need the argument **signature** appended to the identifier.
