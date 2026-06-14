# internal/maskingpolicy

> SQL builder for Snowflake MASKING POLICY objects.

## Responsibility

Builds the `CREATE MASKING POLICY` DDL from a structured config. Masking
policies are part of Snowflake's column-level governance framework: a policy
takes a signature (the column type to mask plus any conditional columns),
returns a type that must match the first argument, and a body expression that
decides — typically from the querying role — whether to return the value or a
masked substitute. The lifecycle / edit commands (`RENAME TO`, `SET BODY`,
`SET`/`UNSET COMMENT`, `SET`/`UNSET TAG`) are simple enough that they are issued
as free-form `ALTER MASKING POLICY <fqn> <clause>` statements directly from
`internal/app/maskingpolicy.go` (`App.AlterMaskingPolicy`) without a dedicated
builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `MaskingArg`, `MaskingPolicyConfig`, `BuildCreateMaskingPolicySql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `MaskingArg` | One signature entry: parameter `Name` + SQL `Type`. The first arg is the masked column; the rest are conditional columns the body may reference |
| `MaskingPolicyConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Args` (signature), `ReturnType` (must match the first arg's type), `Body` (masking expression), `Comment`, `ExemptOtherPolicies` |
| `BuildCreateMaskingPolicySql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] MASKING POLICY [IF NOT EXISTS] <fqn> AS (<arg> <type>, …) RETURNS <type> -> <body> [COMMENT='…'] [EXEMPT_OTHER_POLICIES = TRUE];` |

## Patterns & integration

- Blank required parts get placeholders so the live SQL preview reads as a
  completable template: an empty name emits `masking_policy_name`, an empty
  signature emits `(val VARCHAR)`, an empty return type defaults to the first
  arg's type (or `VARCHAR`), and an empty body emits `'***MASKED***'`.
- `EXEMPT_OTHER_POLICIES` is only emitted when `true` (FALSE is the default).
- `App.BuildCreateMaskingPolicySql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterMaskingPolicy` (in `internal/app/maskingpolicy.go`)
  runs the edit clauses and `App.GetMaskingPolicyReferences` lists the columns a
  policy is applied to.
- Discovery: `Client.ListExtendedObjects` runs `SHOW MASKING POLICIES IN SCHEMA`
  with the fixed kind `"MASKING POLICY"`. Masking policies are not surfaced by
  `SHOW OBJECTS`, so — unlike dynamic / external tables and materialized views —
  no dedupe pass is needed.
- DDL export: masking policies are retrieved via the `GET_DDL` object type
  `POLICY` (which covers all policy kinds), so `buildGetDDLQuery` maps the SHOW
  kind `"MASKING POLICY"` to `POLICY`.
- Properties panel: `internal/objects` runs `SHOW MASKING POLICIES LIKE …` for
  the `MASKING POLICY` kind; the panel also surfaces the policy body and its
  references.

## Gotchas

- **The return type must match the first argument's type** — Snowflake rejects a
  policy whose `RETURNS` type differs from the masked column's type. The builder
  defaults `RETURNS` to the first signature entry's type when left blank, but the
  caller is responsible for keeping them consistent when both are supplied.
- **References require ACCOUNT_USAGE** — `App.GetMaskingPolicyReferences` queries
  `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES`, which needs governance privileges
  and has propagation latency (newly-applied policies may take time to appear).
