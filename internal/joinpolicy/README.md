# internal/joinpolicy

> SQL builder for Snowflake JOIN POLICY objects.

## Responsibility

Builds the `CREATE JOIN POLICY` DDL from a structured config. Join policies are a
governance primitive that restrict which tables and views may be joined
together, preventing unauthorized correlation across datasets. Unlike masking or
row access policies, a join policy has a **fixed signature**: it takes no
arguments and always `RETURNS JOIN_CONSTRAINT`, with a body of the form
`JOIN_CONSTRAINT(JOIN_REQUIRED => <boolean_expression>)`. The lifecycle / edit
commands (`RENAME TO`, `SET BODY`, `SET`/`UNSET COMMENT`, `SET`/`UNSET TAG`) are
simple enough that they are issued as free-form
`ALTER JOIN POLICY <fqn> <clause>` statements directly from
`internal/app/joinpolicy.go` (`App.AlterJoinPolicy`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `JoinPolicyConfig`, `BuildCreateJoinPolicySql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `JoinPolicyConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Body` (the `JOIN_CONSTRAINT(...)` expression), `Comment` |
| `BuildCreateJoinPolicySql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] JOIN POLICY [IF NOT EXISTS] <fqn> AS () RETURNS JOIN_CONSTRAINT -> <body> [COMMENT='…'];` |

## Patterns & integration

- Blank required parts get placeholders so the live SQL preview reads as a
  completable template: an empty name emits `join_policy_name` and an empty body
  emits `JOIN_CONSTRAINT(JOIN_REQUIRED => TRUE)`.
- Unlike masking / row access policies, a join policy has a fixed argument-less
  signature returning `JOIN_CONSTRAINT`, so there is no signature or return-type
  field.
- `App.BuildCreateJoinPolicySql` (in `internal/app/builders.go`) is the thin IPC
  delegator; `App.AlterJoinPolicy` (in `internal/app/joinpolicy.go`) runs the
  edit clauses and `App.GetJoinPolicyReferences` lists the objects a policy is
  applied to.
- Discovery: `Client.ListExtendedObjects` runs `SHOW JOIN POLICIES IN SCHEMA`
  with the fixed kind `"JOIN POLICY"`. Join policies are not surfaced by
  `SHOW OBJECTS`, so — unlike dynamic / external tables and materialized views —
  no dedupe pass is needed.
- DDL export: join policies are retrieved via the `GET_DDL` object type `POLICY`
  (which covers all policy kinds including join), so `buildGetDDLQuery` maps the
  SHOW kind `"JOIN POLICY"` to `POLICY`.
- Properties panel: `internal/objects` runs `SHOW JOIN POLICIES LIKE …` for the
  `JOIN POLICY` kind and enriches it via `DESCRIBE JOIN POLICY` (signature,
  return type, body — none of which SHOW reports); the panel also surfaces the
  policy references.

## Gotchas

- **The body must call `JOIN_CONSTRAINT`** — Snowflake requires the body to be a
  `JOIN_CONSTRAINT(JOIN_REQUIRED => <boolean_expression>)` expression; it cannot
  reference user-defined functions, tables, or views. The builder defaults the
  body to `JOIN_CONSTRAINT(JOIN_REQUIRED => TRUE)` when left blank.
- **References require ACCOUNT_USAGE** — `App.GetJoinPolicyReferences` queries
  `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES` (filtered to
  `POLICY_KIND = 'JOIN_POLICY'`), which needs governance privileges and has
  propagation latency (newly-applied policies may take time to appear).
- **Preview feature** — join policies require Enterprise Edition (or higher) and
  were a preview feature; accounts without the feature will reject the DDL.
