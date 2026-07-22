# internal/users

> ALTER USER per-property SQL builder for the user administration UI.

## Responsibility

Builds and executes `ALTER USER` statements, mirroring the `internal/warehouse`
property-builder pattern: the frontend's `UserPropertiesModal` edits one property
(or fires one action) at a time, and each save routes through a builder here so
all validation and quoting lives here, never inline in a component. Two flavours:
the single-property `SET / UNSET` path (`BuildAlterUserPropertySQL`) and the
non-property one-shot **action** variants (reset password, rename, abort queries,
MFA removal, policy attach/detach, tag set/unset, delegated authorization).

## Files

| File | Purpose |
|---|---|
| `users.go` | `BuildAlterUserPropertySQL(name, property, value)` — validates and renders one property change; `AlterProperty` executes it via the Snowflake client. |
| `users_test.go` | Table test covering quoting, escaping, UNSET-on-empty, enum/integer validation, and the `DEFAULT_SECONDARY_ROLES` / `TYPE` / `PASSWORD` special forms. |
| `actions.go` | The non-property `ALTER USER` action builders — `BuildResetPasswordSQL`, `BuildRenameUserSQL`, `BuildAbortAllQueriesSQL`, `BuildRemoveMfaMethodSQL`, `BuildSet/UnsetPolicySQL`, `BuildSet/UnsetTagsSQL`, `BuildAdd/RemoveDelegatedAuthSQL` — each with a matching `Ctx`-taking executor. `TagPair` is the `<name> = '<value>'` assignment struct. `renderQualifiedName` renders a free-hand, possibly-qualified tag/policy name (quote-aware split, folded bare parts). |
| `actions_test.go` | Table tests for every action builder: quoting, folding, enum/kind validation, the FORCE suffix, multi-tag SET, and the role-vs-AUTHORIZATIONS branch of REMOVE DELEGATED. |

## Property semantics

- **Strings** (`loginName`, `displayName`, `firstName`, `middleName`, `lastName`, `email`, `comment`) — quoted string literals; empty value → `UNSET`.
- **Identifiers** — `defaultWarehouse` / `defaultRole` are double-quoted exactly (their values are canonical-case names from SHOW via the UI's Selects); `networkPolicy` is typed free-hand and rendered via `QuoteOrBare` so bare names fold. `defaultNamespace` (`DB` or `DB.SCHEMA`) splits quote-aware via `snowflake.SplitQualifiedName` (the shared `sqltok`-based qualified-name splitter, capped at 2 parts; `splitNamespace` here is a thin alias) — explicitly-quoted parts keep exact case, bare parts stay bare, and a quoted part containing a literal dot (`"MY.DB".PUB`) stays one part. Empty → `UNSET`.
- **Integers** (`daysToExpiry`, `minsToUnlock`, `minsToBypassMfa`) — validated non-negative; empty → `UNSET`.
- **Booleans** (`disabled`, `mustChangePassword`) — `TRUE`/`FALSE` only; no UNSET. MFA is deliberately not managed here (`DISABLE_MFA` is a legacy Duo-era property with contested support); use `minsToUnlock`/`minsToBypassMfa` or `ALTER USER … REMOVE MFA METHOD` via the SQL editor.
- **Enums** — `type` (`PERSON`/`SERVICE`/`LEGACY_SERVICE`, empty → `UNSET`); `defaultSecondaryRoles` (`ALL` → `('ALL')`, `NONE` → `()`, empty → `UNSET`).
- **`password`** — set-only, never trimmed (spaces are legal), empty is an error.
- **RSA public keys** (`rsaPublicKey` → `RSA_PUBLIC_KEY`, `rsaPublicKey2` → `RSA_PUBLIC_KEY_2`) — the two key-pair-auth slots. The value is the stripped base64 payload; all whitespace/newlines are stripped so a copy-pasted multi-line key works, then it must match a strict base64 charset (`^[A-Za-z0-9+/]+=*$`). Anything else is **rejected** — a full PEM (`-----BEGIN/-----END-----` lines) gets a dedicated message, other non-base64 input a generic one. Because the field is fed by a free-form paste UI, this charset gate (not an assumption that "base64 has no backslashes") is what makes `QuoteStringLit` safe: no value that passes can contain a `'` or `\` to break out of the single-quoted literal. Empty → `UNSET` (removes the key, locking out that private key). This is the single builder for key registration; `internal/keypair` only *generates* keys.

## Action semantics (`actions.go`)

- **`BuildResetPasswordSQL`** — `RESET PASSWORD` (generates a single-use reset URL; does *not* take a new password — use the `password` property for that). Its executor `ResetPassword` returns Snowflake's status row (the reset URL string) instead of discarding it, so the UI can show the link copyably.
- **`BuildRenameUserSQL`** — `RENAME TO <new_name>`; the target is rendered via `renderQualifiedName` (single part), so a bare name folds and a name with a space must be typed quoted (same SQL-syntax model as `defaultNamespace`).
- **`BuildAbortAllQueriesSQL`** — `ABORT ALL QUERIES`.
- **`BuildRemoveMfaMethodSQL`** — `REMOVE MFA METHOD <method>`; `method` is a validated enum (`PASSKEY` / `TOTP` / `DUO`).
- **`BuildSet/UnsetPolicySQL`** — `SET { AUTHENTICATION | PASSWORD | SESSION } POLICY <name> [ FORCE ]` / `UNSET … POLICY`; kind is validated, policy name is a `renderQualifiedName` (up to 3 parts), `force` appends the `FORCE` keyword.
- **`BuildSet/UnsetTagsSQL`** — `SET TAG <n> = '<v>' [ , … ]` / `UNSET TAG <n> [ , … ]`; at least one tag required, names via `renderQualifiedName`, values via `QuoteTextLit`. `TagPair` carries one assignment.
- **`BuildAdd/RemoveDelegatedAuthSQL`** — `ADD DELEGATED AUTHORIZATION OF ROLE <r> TO SECURITY INTEGRATION <i>` / `REMOVE DELEGATED { AUTHORIZATION OF ROLE <r> | AUTHORIZATIONS } FROM SECURITY INTEGRATION <i>`; an **empty role** on remove selects the all-`AUTHORIZATIONS` form. Role/integration are picker-sourced canonical-case names, so they are `QuoteIdent`-wrapped exactly (like `asIdent` for `defaultRole`/`defaultWarehouse`) rather than run through the free-hand fold-or-quote path — a quoted mixed-case role/integration keeps its case.

## Gotchas

- `ListUsers` and `GetUserDDL` still live on the Snowflake client
  (`internal/snowflake/client.go`), not here — this package only owns the
  ALTER property path.
- Enum/integer validation delegates to the shared
  `snowflake.ValidateEnumValue` / `snowflake.ValidateNonNegativeInt` helpers
  (also used by `internal/warehouse`).
- `MINS_TO_BYPASS_MFA` is rejected by Snowflake for users without MFA enrolled;
  the error surfaces to the UI rather than being pre-checked.
