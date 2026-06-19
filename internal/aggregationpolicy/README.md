# internal/aggregationpolicy

Builds SQL for Snowflake **AGGREGATION POLICY** objects and backs the
object-browser create flow.

## What an aggregation policy is

An aggregation policy is a schema-level governance object that enforces queries
on a protected table or view to **aggregate** their results to a minimum group
size, so individual records cannot be identified. It is attached to a table or
view via `ALTER TABLE … SET AGGREGATION POLICY` / `ALTER VIEW … SET AGGREGATION
POLICY`.

Unlike masking and row-access policies, an aggregation policy takes **no
arguments**: its signature is always the empty `()` and it always
`RETURNS AGGREGATION_CONSTRAINT`. The only authored parts are the **body**
expression and an optional comment.

## Types & builders

- **`AggregationPolicyConfig`** — the structured create config: `Name`,
  identifier casing, `OrReplace`, `IfNotExists`, `Body`, `Comment`.
- **`BuildCreateAggregationPolicySql(db, schema, cfg)`** — emits
  `CREATE [OR REPLACE] AGGREGATION POLICY [IF NOT EXISTS] <fqn> AS ()
  RETURNS AGGREGATION_CONSTRAINT -> <body> [COMMENT = '…']`. `OR REPLACE` and
  `IF NOT EXISTS` are mutually exclusive (`OR REPLACE` wins). A blank name
  becomes an `aggregation_policy_name` placeholder and a blank body becomes
  `AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5)` so the live preview stays a valid
  template.

## Body expression

The body is an SQL expression returning an aggregation constraint:

- `AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => <n>)` — require grouping with at
  least `n` records per group.
- `NO_AGGREGATION_CONSTRAINT()` — no aggregation restriction.
- Conditional logic, e.g.
  `CASE WHEN CURRENT_ROLE() = 'ADMIN' THEN NO_AGGREGATION_CONSTRAINT() ELSE
  AGGREGATION_CONSTRAINT(MIN_GROUP_SIZE => 5) END`.

The body is raw SQL (not a string literal) and is emitted verbatim.

## ALTER / DESCRIBE / references

`ALTER AGGREGATION POLICY` (RENAME, `SET BODY -> …`, `SET`/`UNSET COMMENT`,
`SET`/`UNSET TAG`) is issued as a free-form statement by
`App.AlterAggregationPolicy` (`internal/app/aggregationpolicy.go`). The current
body is read via the `DESCRIBE AGGREGATION POLICY` enrichment in
`internal/objects` (`SHOW AGGREGATION POLICIES` reports only metadata, not the
body). The tables/views the policy is attached to are read with
`App.GetAggregationPolicyReferences` (`POLICY_REFERENCES` filtered to
`POLICY_KIND = 'AGGREGATION_POLICY'`).

DDL export uses `GET_DDL('POLICY', …)`, the generic policy object type
(`internal/snowflake/client.go`).
