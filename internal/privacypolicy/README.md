# internal/privacypolicy

> SQL builder for Snowflake PRIVACY POLICY objects.

## Responsibility

Builds the `CREATE PRIVACY POLICY` DDL from a structured config. Privacy policies
enforce **differential-privacy** guarantees on query results, limiting the
information that can be extracted about individual records by constraining a
privacy budget. Like join policies, and unlike masking or row access policies, a
privacy policy has a **fixed signature**: it takes no arguments and always
`RETURNS PRIVACY_BUDGET`, with a body that calls either `NO_PRIVACY_POLICY()`
(unrestricted access) or
`PRIVACY_BUDGET(BUDGET_NAME => '…', [BUDGET_LIMIT => …], [MAX_BUDGET_PER_AGGREGATE => …], [BUDGET_WINDOW => '…'])`
(an enforced privacy budget). The lifecycle / edit commands (`RENAME TO`,
`SET BODY`, `SET`/`UNSET COMMENT`, `SET`/`UNSET TAG`) are simple enough that they
are issued as free-form `ALTER PRIVACY POLICY <fqn> <clause>` statements directly
from `internal/app/privacypolicy.go` (`App.AlterPrivacyPolicy`) without a
dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `PrivacyPolicyConfig`, `BuildCreatePrivacyPolicySql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `PrivacyPolicyConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `IfNotExists`, `Body` (the `PRIVACY_BUDGET(...)` / `NO_PRIVACY_POLICY()` expression), `Comment` |
| `BuildCreatePrivacyPolicySql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] PRIVACY POLICY [IF NOT EXISTS] <fqn> AS () RETURNS PRIVACY_BUDGET -> <body> [COMMENT='…'];` |

## Patterns & integration

- Blank required parts get placeholders so the live SQL preview reads as a
  completable template: an empty name emits `privacy_policy_name` and an empty
  body emits `PRIVACY_BUDGET(BUDGET_NAME => 'privacy_budget')`.
- Unlike masking / row access policies, a privacy policy has a fixed
  argument-less signature returning `PRIVACY_BUDGET`, so there is no signature or
  return-type field.
- `App.BuildCreatePrivacyPolicySql` (in `internal/app/builders.go`) is the thin
  IPC delegator; `App.AlterPrivacyPolicy` (in `internal/app/privacypolicy.go`)
  runs the edit clauses and `App.GetPrivacyPolicyReferences` lists the objects a
  policy is applied to.
- Discovery: `Client.ListExtendedObjects` runs `SHOW PRIVACY POLICIES IN SCHEMA`
  with the fixed kind `"PRIVACY POLICY"`. Privacy policies are not surfaced by
  `SHOW OBJECTS`, so — unlike dynamic / external tables and materialized views —
  no dedupe pass is needed.
- DDL export: privacy policies are retrieved via the `GET_DDL` object type
  `POLICY` (which covers all policy kinds including privacy), so `buildGetDDLQuery`
  maps the SHOW kind `"PRIVACY POLICY"` to `POLICY`.
- Properties panel: `internal/objects` runs `SHOW PRIVACY POLICIES LIKE …` for
  the `PRIVACY POLICY` kind and enriches it via `DESCRIBE PRIVACY POLICY`
  (signature, return type, body — none of which SHOW reports); the panel also
  surfaces the policy references.

## Gotchas

- **The body must call `NO_PRIVACY_POLICY` or `PRIVACY_BUDGET`** — Snowflake
  requires the body to be one of those two function calls; if it uses a `CASE`
  block it must include an `ELSE` clause. The builder defaults the body to
  `PRIVACY_BUDGET(BUDGET_NAME => 'privacy_budget')` when left blank.
- **Privacy budget parameters** — `PRIVACY_BUDGET` accepts `BUDGET_NAME`
  (required, auto-creates the named budget), and optionally `BUDGET_LIMIT`
  (default 233.0), `MAX_BUDGET_PER_AGGREGATE` (default 0.5), and `BUDGET_WINDOW`
  (`Daily`/`Weekly`/`Monthly`/`Yearly`/`Never`, default `Weekly`).
- **References require ACCOUNT_USAGE** — `App.GetPrivacyPolicyReferences` queries
  `SNOWFLAKE.ACCOUNT_USAGE.POLICY_REFERENCES` (filtered to
  `POLICY_KIND = 'PRIVACY_POLICY'`), which needs governance privileges and has
  propagation latency (newly-applied policies may take time to appear).
- **Preview feature** — privacy policies require Enterprise Edition (or higher)
  and were a preview feature; accounts without the feature will reject the DDL.
