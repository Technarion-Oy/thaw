# components/packagespolicy

Object-browser UI for Snowflake **PACKAGES POLICY** objects — a schema-level
governance object that controls which third-party packages (currently Python)
UDFs and stored procedures may import.

## Components

- **`CreatePackagesPolicyModal`** — create flow. Name + identifier casing +
  `OR REPLACE` / `IF NOT EXISTS` (via `NameWithReplaceOptions`), a read-only
  `LANGUAGE PYTHON` (the only supported language), and three tag editors for the
  `ALLOWLIST`, `BLOCKLIST`, and `ADDITIONAL_CREATION_BLOCKLIST` package-spec
  lists, plus a comment. A live SQL preview (`BuildCreatePackagesPolicySql`) is
  shown and executed via `ExecDDL`. Each list entry is a package specification: a
  bare name, a name with a version specifier (`==`, `<=`, `>=`, `<`, `>`), or the
  wildcard `*`. An empty list is omitted so the policy inherits Snowflake's
  default.
- **`PackagesPolicyPropertiesModal`** — properties / edit flow. Loads the policy
  via `GetObjectProperties` (which enriches `SHOW PACKAGES POLICIES` with the
  `DESCRIBE` language and allow/block lists). Shows the language, three editable
  list rows (Set via `AlterPackagesPolicy` + `FormatPackagesPolicyList`, Unset to
  restore the default), an editable comment, and the raw SHOW properties.

## Notes

- Packages policies have **no `RENAME` and no `TAG`** support — the Sidebar omits
  the Rename item for this kind and the properties modal exposes only the
  allow/block lists and comment.
- The list serialization (`('a', 'b')`) is built in Go
  (`App.FormatPackagesPolicyList`) and the DESCRIBE list cells are tokenized by
  the package-spec-aware backend parser (`App.ParsePackagesPolicyList`, which
  preserves version specifiers like `numpy==1.26.4`), so this folder carries no
  SQL quoting or parsing logic of its own.
