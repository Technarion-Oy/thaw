# internal/networkrule

> SQL builder for Snowflake NETWORK RULE objects.

## Responsibility

Builds the `CREATE NETWORK RULE` DDL from a structured config. A network rule
groups a set of network identifiers — selected by its `TYPE` (IP addresses /
CIDR ranges, VPC or private-endpoint IDs, or `host:port` destinations) — and a
`MODE` that declares how the rule is used (ingress to Snowflake, egress to
external destinations, or internal-stage access). Network rules are referenced
by network policies and external-access integrations.

`TYPE` and `MODE` are fixed at creation and network rules cannot be renamed; only
`VALUE_LIST` and `COMMENT` can be changed, so the edit commands (`SET`/`UNSET
VALUE_LIST`, `SET`/`UNSET COMMENT`) are issued as free-form `ALTER NETWORK RULE
<fqn> <clause>` statements directly from `internal/app/networkrule.go`
(`App.AlterNetworkRule`) without a dedicated builder.

## Key files

| File | Purpose |
|---|---|
| `sql.go` | `NetworkRuleConfig`, `BuildCreateNetworkRuleSql` |
| `sql_test.go` | Unit tests for the SQL builder |
| `doc.go` | Package doc + `thaw:domain: Object Browser & Administration` annotation |

## Key types & functions

| Type / Function | Purpose |
|---|---|
| `NetworkRuleConfig` | CREATE parameters: name, case sensitivity, `OrReplace`, `Type`, `Mode`, `ValueList`, `Comment` |
| `BuildCreateNetworkRuleSql(db, schema, cfg)` | Emits `CREATE [OR REPLACE] NETWORK RULE <fqn> TYPE = <type> VALUE_LIST = ('…', …) MODE = <mode> [COMMENT='…'];` |

## Patterns & integration

- Blank required parts get placeholders so the live SQL preview reads as a
  completable template: an empty name emits `network_rule_name`, an empty type
  defaults to `IPV4`, an empty mode defaults to `INGRESS`, and an empty value
  list renders as `()` (a valid, later-populatable rule).
- There is **no `IF NOT EXISTS`** form for `CREATE NETWORK RULE`, so only
  `OrReplace` is modelled.
- `App.BuildCreateNetworkRuleSql` (in `internal/app/builders.go`) is the thin IPC
  delegator; `App.AlterNetworkRule` (in `internal/app/networkrule.go`) runs the
  edit clauses.
- Discovery: `Client.ListExtendedObjects` runs `SHOW NETWORK RULES IN SCHEMA`
  with the fixed kind `"NETWORK RULE"`. Network rules are not surfaced by
  `SHOW OBJECTS`, so — like masking policies, tags, and alerts — no dedupe pass
  is needed.
- DDL export: `buildGetDDLQuery` maps the SHOW kind `"NETWORK RULE"` to the
  `GET_DDL` object type `NETWORK_RULE`.
- Properties panel: `internal/objects` runs `SHOW NETWORK RULES LIKE …` for the
  `NETWORK RULE` kind and appends the `value_list` from `DESCRIBE NETWORK RULE`
  (which `SHOW` reports only as a count, `entries_in_valuelist`).

## Gotchas

- **`TYPE` and `MODE` are immutable** — Snowflake rejects any attempt to change
  them after creation, and there is no `RENAME TO`. To change them, recreate the
  rule (`CREATE OR REPLACE`). Only `VALUE_LIST` and `COMMENT` are editable in
  place.
- **`TYPE`/`MODE` combinations are constrained** — e.g. `HOST_PORT` /
  `PRIVATE_HOST_PORT` require `MODE = EGRESS`; `COMPUTE_POOL` requires
  `MODE = INGRESS`; `INTERNAL_STAGE` / `SNOWFLAKE_MANAGED_STORAGE_VOLUME` require
  `TYPE = AWSVPCEID`. The builder does not enforce these (Snowflake validates at
  execution); the create modal nudges sensible defaults.
