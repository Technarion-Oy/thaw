# internal/authenticationpolicy

Builds SQL for Snowflake **AUTHENTICATION POLICY** objects and backs the
object-browser create flow.

## What an authentication policy is

An authentication policy is a schema-level governance object that restricts how
users (or the entire account) may authenticate. It controls which
**authentication methods** are permitted (`PASSWORD`, `SAML`, `OAUTH`,
`KEYPAIR`, `PROGRAMMATIC_ACCESS_TOKEN`, `WORKLOAD_IDENTITY`), which **client
types** may connect (`SNOWFLAKE_UI`, `DRIVERS`, `SNOWFLAKE_CLI`, `SNOWSQL`),
which **security integrations** are allowed, and whether multi-factor
authentication enrollment is required (`MFA_ENROLLMENT`). A policy is attached
to the account or to individual users via
`ALTER ACCOUNT … SET AUTHENTICATION POLICY` / `ALTER USER … SET AUTHENTICATION POLICY`.

## Types & builders

- **`AuthenticationPolicyConfig`** — the structured create config. The
  list-valued parameters (`AuthenticationMethods`, `ClientTypes`,
  `SecurityIntegrations`) are `[]string` slices of bare tokens which the builder
  renders as single-quoted string literals; `MFAEnrollment` is a single
  enumerated keyword. The four nested property bags (`MFAPolicy`, `PATPolicy`,
  `WorkloadIdentityPolicy`, `ClientPolicy`) are embedded too, so they can be set
  at creation as well as via ALTER. Identifier-casing, `OR REPLACE`, and
  `IF NOT EXISTS` follow the usual create-modal conventions.
- **`BuildCreateAuthenticationPolicySql(db, schema, cfg)`** — emits
  `CREATE [OR REPLACE] AUTHENTICATION POLICY [IF NOT EXISTS] <fqn>` followed by
  only the parameters the caller set, then `COMMENT`. The nested bags are
  serialized through the same `Build<Bag>Value` functions the ALTER path uses;
  an empty bag builds to `()` and is omitted (so it inherits the default), just
  like a blank list. `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive
  (`OR REPLACE` wins). A blank name becomes an `authentication_policy_name`
  placeholder so the live preview stays a valid template.
- **`FormatStringList(tokens)`** — renders a token slice into the
  `('A', 'B')` single-quoted-literal list grammar shared by the list
  parameters; exposed so the app layer / properties modal builds
  `ALTER … SET <list> = (…)` clauses through the same serializer.

### Editor metadata (`params.go`)

- **`ListParams()`** — returns the `ListParamMeta` descriptors (ALTER keyword,
  label, allowed-value enumeration, free-form flag) for the three top-level list
  parameters, and **`MFAEnrollmentOptions()`** returns the `MFA_ENROLLMENT`
  values. These keep the grammar's allowed values in Go so the properties modal
  renders its editors from one source of truth (exposed via
  `App.AuthenticationPolicyListParams` / `App.AuthenticationPolicyMFAEnrollmentOptions`).
  The modal parses DESCRIBE list/scalar cells back through the general
  `App.ParseSqlList` / `App.NormalizeSqlScalar` helpers (over `internal/snowflake`),
  so no SQL parsing lives in TypeScript.
- **`ClientPolicyDrivers()`** — the driver/client tokens selectable in a
  `CLIENT_POLICY` bag, filtered from the general `snowflake.ClientDrivers()`
  catalog to the version-governed subset (CLI clients like SnowSQL / Snowflake CLI
  are dropped as inapplicable). Exposed via `App.AuthenticationPolicyClientDrivers`
  so the bag editor's driver picker draws from the shared catalog instead of a
  hard-coded frontend list.
- **`ClientPolicyDriverVersions(info)`** / **`DriverVersionHint`** — maps those
  drivers to Snowflake's minimum-supported / recommended versions from
  `SYSTEM$CLIENT_VERSION_INFO()` (via `snowflake.MatchClientVersions`), so the bag
  editor can suggest a version instead of the user looking it up. Exposed via
  `App.AuthenticationPolicyClientDriverVersions` (which runs the query) — drivers
  the function doesn't report are omitted, and a failure degrades to manual entry.

### Nested property bags (`policies.go`)

The four nested parameters — `MFA_POLICY`, `PAT_POLICY`,
`WORKLOAD_IDENTITY_POLICY`, `CLIENT_POLICY` — are each a parenthesized list of
sub-properties with their own grammar. Each is modeled as a struct
(`MFAPolicy`, `PATPolicy`, `WorkloadIdentityPolicy`, `ClientPolicy` /
`ClientPolicyEntry`) with a matching **`Build<Bag>Value(p)`** serializer (emits
the `( … )` value for an `ALTER … SET <BAG> = <value>` clause — only the
sub-properties the caller set; `*int`/`*bool` distinguish unset from `0`/`false`)
and a tolerant **`Parse<Bag>(raw)`** reader. `DESCRIBE AUTHENTICATION POLICY`
renders these bags in Snowflake's structured-object notation — `{KEY=VALUE,
KEY={NESTED=VALUE}}`, e.g. `CLIENT_POLICY` → `{GO_DRIVER={MINIMUM_VERSION=3.14.1}}`
([reference](https://docs.snowflake.com/en/sql-reference/sql/desc-authentication-policy)),
**not** JSON — so the parsers run that grammar through `parseDescribeBag` (strict
JSON is accepted as a fallback) and never error: an unrecognized/empty value
yields a zero struct (the editor starts blank). The `structScanner` also accepts
the parenthesized SQL form (`( KEY = VALUE … )` → object, `('A', 'B')` → list) so
a paren-style DESCRIBE rendering still pre-fills the editor rather than blanking
it (which would risk a Set wiping the bag). `BuildPATPolicyValue` range-checks the
expiry day counts against the documented 1–365 bound as defense-in-depth (the
exported IPC method can't rely on the UI's input clamps), and
`BuildClientPolicyValue` drops a repeated driver (first-wins) so the bag can't
carry a duplicate key (the editor also blocks duplicates). All of this lives in Go
(exposed via `App.Build<Bag>Value` /
`App.Parse<Bag>`) so the properties modal carries no SQL-serialization or
DESCRIBE-parsing logic. `UNSET DCM PROJECT` (detach from a Declarative Change
Management project) is issued as a plain `AlterAuthenticationPolicy` clause.

## Parameters (allowed values / Snowflake default)

| Parameter | Allowed values | Default |
|-----------|----------------|---------|
| `AUTHENTICATION_METHODS` | `ALL`, `SAML`, `PASSWORD`, `OAUTH`, `KEYPAIR`, `PROGRAMMATIC_ACCESS_TOKEN`, `WORKLOAD_IDENTITY` | `('ALL')` |
| `CLIENT_TYPES` | `ALL`, `SNOWFLAKE_UI`, `DRIVERS`, `SNOWFLAKE_CLI`, `SNOWSQL` | `('ALL')` |
| `SECURITY_INTEGRATIONS` | `ALL` or integration names | `('ALL')` |
| `MFA_ENROLLMENT` | `REQUIRED`, `REQUIRED_PASSWORD_ONLY`, `OPTIONAL` | `OPTIONAL` |
| `COMMENT` | string literal | — |

## ALTER / DESCRIBE / references

`ALTER AUTHENTICATION POLICY` (RENAME, `SET`/`UNSET` each parameter,
`SET`/`UNSET COMMENT`) is issued as a free-form statement by
`App.AlterAuthenticationPolicy` (`internal/app/authenticationpolicy.go`). The
current parameter values are read with `App.DescribeAuthenticationPolicy`
(`DESCRIBE AUTHENTICATION POLICY` returns one row per property with the columns
`property` / `value` — `SHOW AUTHENTICATION POLICIES` reports only the comment
and metadata). The users/account the policy is attached to are read with
`App.GetAuthenticationPolicyReferences` (`POLICY_REFERENCES` filtered to
`POLICY_KIND = 'AUTHENTICATION_POLICY'`).

DDL export uses `GET_DDL('POLICY', …)`, the generic policy object type
(`internal/snowflake/client.go`).
