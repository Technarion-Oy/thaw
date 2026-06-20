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
  `('a', 'b')` single-quoted-literal list grammar (a thin delegator to the
  shared `snowflake.FormatStringLitList`), exposed over IPC via
  `App.FormatPackagesPolicyList` so the properties modal builds its
  `ALTER … SET <list> = (…)` clause through the same serializer the builder uses.
- **`ParseList(raw)`** — the read counterpart: tokenizes a `DESCRIBE PACKAGES
  POLICY` allow/block-list cell into its package-spec entries (exposed via
  `App.ParsePackagesPolicyList`). It deliberately does **not** use the general
  `snowflake.ParseSqlList` SQL tokenizer, which discards operator tokens and
  would split a bare `numpy==1.26.4` into `numpy`/`1.26.4`. Package specs never
  contain commas, so it splits on top-level commas/newlines (after stripping one
  optional surrounding `[ ]`/`( )` layer) and strips one optional quote layer per
  element — preserving the `==`/`>=`/`<=`/`<`/`>` version operators whether or not
  Snowflake quotes the entries. Robust to `('a', 'b')`, `["a","b"]`,
  `[a==1, b]`, and bare comma lists alike.

## ALTER / DESCRIBE

`ALTER PACKAGES POLICY` (`SET`/`UNSET ALLOWLIST | BLOCKLIST |
ADDITIONAL_CREATION_BLOCKLIST | COMMENT`) is issued as a free-form statement by
`App.AlterPackagesPolicy` (`internal/app/packagespolicy.go`). **There is no
`RENAME` and no `SET/UNSET TAG`** for packages policies. The current list values
are read via the `DESCRIBE PACKAGES POLICY` enrichment in `internal/objects`
(`SHOW PACKAGES POLICIES` reports only metadata, not the allowlist/blocklist).

`GET_DDL` does **not** support packages policies — `GET_DDL('POLICY', …)` fails
with *"Cannot initialize Snowflake Metadata. Dictionary unavailable"* and there
is no `'PACKAGES POLICY'` object type — so (like image repositories and services)
there is no DDL export, View Definition, or object comparison for this kind. The
full configuration is reconstructable from `DESCRIBE` and is surfaced in the
properties modal instead.
