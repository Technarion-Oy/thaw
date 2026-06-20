# internal/packagespolicy

Builds SQL for Snowflake **PACKAGES POLICY** objects and backs the
object-browser create flow.

## What a packages policy is

A packages policy is a schema-level governance object that controls which
third-party packages a UDF or stored procedure may import. It is important for
security governance — it lets an account restrict code to a vetted set of
packages. A policy defines:

- **`LANGUAGE`** — the language the policy applies to. **Required**; `PYTHON` is
  the only value Snowflake currently supports, so the builder always emits
  `LANGUAGE PYTHON`.
- **`ALLOWLIST`** — package specifications that are permitted. Default `('*')`
  (all packages allowed).
- **`BLOCKLIST`** — package specifications that are forbidden. Default `()`
  (none blocked). The blocklist takes precedence over the allowlist.
- **`ADDITIONAL_CREATION_BLOCKLIST`** — package specifications blocked only at
  object-creation time (not at run time). Default `()`.

Each list entry is a package specification: a bare name (`numpy`), or a name
with a version specifier using `==`, `<=`, `>=`, `<`, or `>` (`numpy==1.26.4`,
`pandas>=2.0`), or the wildcard `*`.

A packages policy is attached to the account with
`ALTER ACCOUNT … SET PACKAGES POLICY`.

## Types & builders

- **`PackagesPolicyConfig`** — the structured create config: `Name`, identifier
  casing, `OrReplace`, `IfNotExists`, `Allowlist`, `Blocklist`,
  `AdditionalCreationBlocklist`, `Comment`.
- **`BuildCreatePackagesPolicySql(db, schema, cfg)`** — emits
  `CREATE [OR REPLACE] PACKAGES POLICY [IF NOT EXISTS] <fqn> LANGUAGE PYTHON
  [ALLOWLIST = (…)] [BLOCKLIST = (…)] [ADDITIONAL_CREATION_BLOCKLIST = (…)]
  [COMMENT = '…']`. `OR REPLACE` and `IF NOT EXISTS` are mutually exclusive
  (`OR REPLACE` wins). Only the lists the caller set are emitted; the rest
  inherit Snowflake's defaults. A blank name becomes a `packages_policy_name`
  placeholder so the live preview stays a valid template.
- **`FormatStringList(tokens)`** — renders a token slice into the
  `('a', 'b')` single-quoted-literal list grammar (exposed over IPC via
  `App.FormatPackagesPolicyList` so the properties modal builds its
  `ALTER … SET <list> = (…)` clause through the same serializer).

## ALTER / DESCRIBE

`ALTER PACKAGES POLICY` (`SET`/`UNSET ALLOWLIST | BLOCKLIST |
ADDITIONAL_CREATION_BLOCKLIST | COMMENT`) is issued as a free-form statement by
`App.AlterPackagesPolicy` (`internal/app/packagespolicy.go`). **There is no
`RENAME` and no `SET/UNSET TAG`** for packages policies. The current list values
are read via the `DESCRIBE PACKAGES POLICY` enrichment in `internal/objects`
(`SHOW PACKAGES POLICIES` reports only metadata, not the allowlist/blocklist).

DDL export uses `GET_DDL('POLICY', …)`, the generic policy object type
(`internal/snowflake/client.go`).
