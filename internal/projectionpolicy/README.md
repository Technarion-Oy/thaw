# internal/projectionpolicy

Builds SQL for Snowflake **PROJECTION POLICY** objects and backs the
object-browser create flow.

## What a projection policy is

A projection policy is a schema-level governance object that controls whether a
protected **column** can appear in query output — i.e. whether it can be
**projected** via `SELECT`. Unlike a masking policy, which transforms a column's
values, a projection policy prevents the column from being selected at all. It is
attached to a column via `ALTER TABLE … MODIFY COLUMN … SET PROJECTION POLICY` /
`ALTER VIEW … MODIFY COLUMN … SET PROJECTION POLICY`.

Like aggregation policies, a projection policy takes **no arguments**: its
signature is always the empty `()` and it always `RETURNS PROJECTION_CONSTRAINT`.
The only authored parts are the **body** expression and an optional comment.

## Types & builders

- **`ProjectionPolicyConfig`** — the structured create config: `Name`,
  identifier casing, `OrReplace`, `IfNotExists`, `Body`, `Comment`.
- **`BuildCreateProjectionPolicySql(db, schema, cfg)`** — emits
  `CREATE [OR REPLACE] PROJECTION POLICY [IF NOT EXISTS] <fqn> AS ()
  RETURNS PROJECTION_CONSTRAINT -> <body> [COMMENT = '…']`. `OR REPLACE` and
  `IF NOT EXISTS` are mutually exclusive (`OR REPLACE` wins). A blank name
  becomes a `projection_policy_name` placeholder and a blank body becomes
  `PROJECTION_CONSTRAINT(ALLOW => true)` so the live preview stays a valid
  template.

## Body expression

The body is an SQL expression returning a projection constraint:

- `PROJECTION_CONSTRAINT(ALLOW => true)` — permit the column to be projected.
- `PROJECTION_CONSTRAINT(ALLOW => false)` — prevent the column from being
  projected.
- Conditional logic, e.g.
  `CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN PROJECTION_CONSTRAINT(ALLOW => true)
  ELSE PROJECTION_CONSTRAINT(ALLOW => false) END`.

The body is raw SQL (not a string literal) and is emitted verbatim.

## ALTER / DESCRIBE / references

`ALTER PROJECTION POLICY` (RENAME, `SET BODY -> …`, `SET`/`UNSET COMMENT`,
`SET`/`UNSET TAG`) is issued as a free-form statement by
`App.AlterProjectionPolicy` (`internal/app/projectionpolicy.go`). The current
body is read via the `DESCRIBE PROJECTION POLICY` enrichment in
`internal/objects` (`SHOW PROJECTION POLICIES` reports only metadata, not the
body). The columns the policy is attached to are read with
`App.GetProjectionPolicyReferences` (`POLICY_REFERENCES` filtered to
`POLICY_KIND = 'PROJECTION_POLICY'`).

DDL export uses `GET_DDL('POLICY', …)`, the generic policy object type
(`internal/snowflake/client.go`).
