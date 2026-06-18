# internal/sessionpolicy

Builds SQL for Snowflake **SESSION POLICY** objects and backs the object-browser
create flow.

## What a session policy is

A session policy is a schema-level governance object that controls session
behavior: the **idle timeout** (separately for programmatic clients and for the
Snowsight UI), the **maximum session lifespan** (likewise split into a client
and a UI value), and which **secondary roles** may be activated in a session. A
policy is attached to the account or to individual users via
`ALTER ACCOUNT … SET SESSION POLICY` / `ALTER USER … SET SESSION POLICY`.

## Types & builders

- **`SessionPolicyConfig`** — the structured create config. Each numeric timeout
  parameter is a `*int` so the builder can distinguish "leave at the Snowflake
  default" (`nil`) from "set to N" (e.g. `0`, which disables lifespan
  enforcement). The secondary-role controls are `[]string` slices (each entry is
  either the special token `ALL` → the quoted literal `'ALL'`, or a role
  identifier). Identifier-casing, `OR REPLACE`, and `IF NOT EXISTS` follow the
  usual create-modal conventions.
- **`BuildCreateSessionPolicySql(db, schema, cfg)`** — emits
  `CREATE [OR REPLACE] SESSION POLICY [IF NOT EXISTS] <fqn>` followed by only the
  parameters the caller set, then `COMMENT`. `OR REPLACE` and `IF NOT EXISTS` are
  mutually exclusive (`OR REPLACE` wins). A blank name becomes a
  `session_policy_name` placeholder so the live preview stays a valid template.

The secondary-role list values are rendered by **`snowflake.FormatSecondaryRoles`**
(in `internal/snowflake/identifiers.go`) — a general helper for the
`( 'ALL' | <role>, … )` grammar shared by session/authentication policies and
`ALTER USER … DEFAULT_SECONDARY_ROLES`. It emits `'ALL'` for the special token and
role identifiers bare when valid unquoted names (so `analyst` resolves to
`ANALYST`), double-quoting only when needed — e.g. `('ALL')`, `(R1, R2)`,
`("my role")`, or `()`.

## Parameters (range / Snowflake default)

| Parameter | Range | Default |
|-----------|-------|---------|
| `SESSION_IDLE_TIMEOUT_MINS` | 5–1440 | 240 |
| `SESSION_UI_IDLE_TIMEOUT_MINS` | 5–1440 | 240 |
| `SESSION_MAX_LIFESPAN_MINS` | 0–43200 | 0 (no limit) |
| `SESSION_UI_MAX_LIFESPAN_MINS` | 0–43200 | 0 (no limit) |
| `ALLOWED_SECONDARY_ROLES` | `('ALL')` or role list | `('ALL')` |
| `BLOCKED_SECONDARY_ROLES` | role list | `()` |

## ALTER / DESCRIBE / references

`ALTER SESSION POLICY` (RENAME, `SET`/`UNSET` each parameter, `SET`/`UNSET
COMMENT`, `SET`/`UNSET TAG`) is issued as a free-form statement by
`App.AlterSessionPolicy` (`internal/app/sessionpolicy.go`). The current parameter
values are read with `App.DescribeSessionPolicy` (`DESCRIBE SESSION POLICY`
returns a single row whose columns include the timeout values and
`allowed_secondary_roles` — `SHOW SESSION POLICIES` reports only the comment and
metadata). The users/account the policy is attached to are read with
`App.GetSessionPolicyReferences` (`POLICY_REFERENCES` filtered to
`POLICY_KIND = 'SESSION_POLICY'`).

DDL export uses `GET_DDL('POLICY', …)`, the generic policy object type
(`internal/snowflake/client.go`).
