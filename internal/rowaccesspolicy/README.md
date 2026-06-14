# internal/rowaccesspolicy

> SQL builder for Snowflake ROW ACCESS POLICY objects.

## Responsibility

Builds the `CREATE ROW ACCESS POLICY` DDL from a structured config. Row access
policies are part of Snowflake's row-level security framework: a policy takes a
signature (the columns it evaluates), always returns a `BOOLEAN`, and a body
expression that decides — typically from the querying role — whether a given row
is visible. The lifecycle / edit commands (`RENAME TO`, `SET BODY`,
`SET`/`UNSET COMMENT`, `SET`/`UNSET TAG`) are simple enough that they are issued
as free-form `ALTER ROW ACCESS POLICY <fqn> <clause>` statements directly from
`internal/app/rowaccesspolicy.go` (`App.AlterRowAccessPolicy`) without a
dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `RowAccessArg`, `RowAccessPolicyConfig`, `BuildCreateRowAccessPolicySql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `RowAccessArg` | One signature entry: parameter `Name` + SQL `Type`. Each arg is a column the body may reference; when the policy is attached, each maps to a table/view column |
| `RowAccessPolicyConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Args` (signature), `Body` (boolean expression), `Comment` |
| `BuildCreateRowAccessPolicySql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] ROW ACCESS POLICY [IF NOT EXISTS] <fqn> AS (<arg> <type>, …) RETURNS BOOLEAN -> <body> [COMMENT='…'];` |

## Patterns & integration

- Blank required parts get placeholders so the live SQL preview reads as a
  completable template: an empty name emits `row_access_policy_name`, an empty
  signature emits `(val VARCHAR)`, and an empty body emits `TRUE`.
- Unlike masking policies, a row access policy **always** returns `BOOLEAN` and
  has no `EXEMPT_OTHER_POLICIES` option, so there is no return-type field.
- `App.BuildCreateRowAccessPolicySql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterRowAccessPolicy` (in
  `internal/app/rowaccesspolicy.go`) runs the edit clauses and
  `App.GetRowAccessPolicyReferences` lists the objects a policy is applied to.
- Discovery: `Client.ListExtendedObjects` runs
  `SHOW ROW ACCESS POLICIES IN SCHEMA` with the fixed kind
  `"ROW ACCESS POLICY"`. Row access policies are not surfaced by `SHOW OBJECTS`,
  so — unlike dynamic / external tables and materialized views — no dedupe pass
  is needed.
- DDL export: row access policies are retrieved via the `GET_DDL` object type
  `POLICY` (which covers all policy kinds), so `buildGetDDLQuery` maps the SHOW
  kind `"ROW ACCESS POLICY"` to `POLICY`.
- Properties panel: `internal/objects` runs `SHOW ROW ACCESS POLICIES LIKE …`
  for the `ROW ACCESS POLICY` kind; the panel also surfaces the policy body and
  its references.

## Gotchas

- **The body must be a boolean expression** — Snowflake rejects a row access
  policy whose body does not evaluate to `BOOLEAN`. The builder defaults the body
  to `TRUE` when left blank.
- **References require ACCOUNT_USAGE** — `App.GetRowAccessPolicyReferences`
  queries `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES` (filtered to
  `POLICY_KIND = 'ROW_ACCESS_POLICY'`), which needs governance privileges and
  has propagation latency (newly-applied policies may take time to appear).
