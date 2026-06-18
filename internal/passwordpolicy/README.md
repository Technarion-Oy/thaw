# internal/passwordpolicy

Builds SQL for Snowflake **PASSWORD POLICY** objects and backs the object-browser
create flow.

## What a password policy is

A password policy is a schema-level governance object that defines the rules
Snowflake enforces when a user sets or changes their password: complexity
(minimum length, maximum length, required upper/lower/numeric/special
characters), age (minimum/maximum days), retry/lockout (max retries, lockout
time), and reuse history. A policy is attached to the account or to individual
users via `ALTER ACCOUNT … SET PASSWORD POLICY` / `ALTER USER … SET PASSWORD
POLICY`.

## Types & builders

- **`PasswordPolicyConfig`** — the structured create config. Every numeric
  parameter is a `*int` so the builder can distinguish "leave at the Snowflake
  default" (`nil`) from "set to N" (e.g. `0`, which is a meaningful value for
  `PASSWORD_MIN_SPECIAL_CHARS`). Identifier-casing, `OR REPLACE`, and
  `IF NOT EXISTS` follow the usual create-modal conventions.
- **`BuildCreatePasswordPolicySql(db, schema, cfg)`** — emits
  `CREATE [OR REPLACE] PASSWORD POLICY [IF NOT EXISTS] <fqn>` followed by only
  the parameters the caller set, then `COMMENT`. `OR REPLACE` and
  `IF NOT EXISTS` are mutually exclusive (`OR REPLACE` wins). A blank name
  becomes a `password_policy_name` placeholder so the live preview stays a valid
  template.

## Parameters (range / Snowflake default)

| Parameter | Range | Default |
|-----------|-------|---------|
| `PASSWORD_MIN_LENGTH` | 8–256 | 14 |
| `PASSWORD_MAX_LENGTH` | 8–256 | 256 |
| `PASSWORD_MIN_UPPER_CASE_CHARS` | 0–256 | 1 |
| `PASSWORD_MIN_LOWER_CASE_CHARS` | 0–256 | 1 |
| `PASSWORD_MIN_NUMERIC_CHARS` | 0–256 | 1 |
| `PASSWORD_MIN_SPECIAL_CHARS` | 0–256 | 0 |
| `PASSWORD_MIN_AGE_DAYS` | 0–999 | 0 |
| `PASSWORD_MAX_AGE_DAYS` | 0–999 | 90 |
| `PASSWORD_MAX_RETRIES` | 1–10 | 5 |
| `PASSWORD_LOCKOUT_TIME_MINS` | 1–999 | 15 |
| `PASSWORD_HISTORY` | 0–24 | 5 |

## ALTER / DESCRIBE / references

`ALTER PASSWORD POLICY` (RENAME, `SET`/`UNSET` each parameter, `SET`/`UNSET
COMMENT`, `SET`/`UNSET TAG`) is issued as a free-form statement by
`App.AlterPasswordPolicy` (`internal/app/passwordpolicy.go`). The current
parameter values and their defaults are read with `App.DescribePasswordPolicy`
(`DESCRIBE PASSWORD POLICY` returns one `property/value/default/description` row
per parameter). The users/account the policy is attached to are read with
`App.GetPasswordPolicyReferences` (`POLICY_REFERENCES` filtered to
`POLICY_KIND = 'PASSWORD_POLICY'`).

DDL export uses `GET_DDL('POLICY', …)`, the generic policy object type
(`internal/snowflake/client.go`).
