# components/networkrule

> Modals for creating and managing Snowflake NETWORK RULE objects.

## Components

| File | Purpose |
|---|---|
| `CreateNetworkRuleModal.tsx` | Create form with a live `CREATE NETWORK RULE` SQL preview. Fields: name, OR REPLACE (network rules have **no** `IF NOT EXISTS`), a **Type** select (IPV4 / IPV6 / AWSVPCEID / AZURELINKID / GCPPSCID / HOST_PORT / PRIVATE_HOST_PORT / COMPUTE_POOL), a **Mode** select (INGRESS / EGRESS / INTERNAL_STAGE / SNOWFLAKE_MANAGED_STORAGE_VOLUME), a repeatable **Value list** (rows of identifiers, with a per-type placeholder), and a comment. |
| `NetworkRulePropertiesModal.tsx` | `SHOW`/`DESCRIBE NETWORK RULE` metadata: a read-only **Definition** (type + mode), an editable **Value list** (removable `Tag`s + add input + clear all), an inline-editable **Comment**, and the generic property rows. |

## Integration

- Both delegate to IPC: `BuildCreateNetworkRuleSql` / `ExecDDL` (create) and
  `GetObjectProperties` / `AlterNetworkRule` (properties + edits).
- `AlterNetworkRule(db, schema, name, clause)` runs free-form
  `ALTER NETWORK RULE … <clause>` for `SET`/`UNSET VALUE_LIST` and
  `SET`/`UNSET COMMENT`.
- **`SET VALUE_LIST` replaces the whole list** (it is not additive), so every
  add/remove in the properties modal resends the full list; an empty list maps
  to `UNSET VALUE_LIST`.
- The **value list** itself comes from `DESCRIBE NETWORK RULE` (appended by
  `internal/objects` `GetObjectProperties`) because `SHOW NETWORK RULES` reports
  only a count (`entries_in_valuelist`). It is parsed from a comma-separated
  string.
- Wired into the object tree from `components/layout/Sidebar.tsx` under the
  **Network Rules** group (kind `"NETWORK RULE"`). Network rules are not queryable
  tables, so there is no **Select Top 1000 Rows**; `ALTER NETWORK RULE` has no
  `RENAME TO` (and TYPE/MODE are immutable), so **Rename** is not offered.
- The form shape mirrors the Wails-generated `networkrule.NetworkRuleConfig`
  (with a nested `valueList` array); a plain object literal is cast `as any` only
  at the IPC boundary.

## Gotchas

- **Type and Mode are immutable** — Snowflake rejects changing them after
  creation; the create modal labels both as fixed, and the properties panel shows
  them read-only. Changing them means `CREATE OR REPLACE`.
- **Type/Mode combinations are constrained** by Snowflake (e.g. `HOST_PORT`
  requires `MODE = EGRESS`); the modal nudges defaults but does not enforce them —
  Snowflake validates on execute and surfaces any error.
